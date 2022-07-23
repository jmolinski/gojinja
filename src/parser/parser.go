package parser

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
	"github.com/gojinja/gojinja/src/extensions"
	"github.com/gojinja/gojinja/src/lexer"
	"github.com/gojinja/gojinja/src/nodes"
	"github.com/gojinja/gojinja/src/utils/set"
	"github.com/gojinja/gojinja/src/utils/slices"
	"github.com/gojinja/gojinja/src/utils/stack"
	"strings"
)

var statementKeywords = set.FrozenFromElems(
	"for",
	"if",
	"block",
	"extends",
	"print",
	"macro",
	"include",
	"from",
	"import",
	"set",
	"with",
	"autoescape",
)

var compareOperators = set.FrozenFromElems(
	"eq", "ne", "lt", "lteq", "gt", "gteq",
)

func makeBinaryOpExpr(left, right nodes.Expr, op string, lineno int) nodes.Expr {
	return &nodes.BinExpr{
		Left:       left,
		Right:      right,
		Op:         op,
		ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
	}
}

type extensionParser = func(p extensions.IParser) ([]nodes.Node, error)

type parser struct {
	stream                *lexer.TokenStream
	name, filename, state *string
	closed                bool
	extensions            map[string]extensionParser
	lastIdentifier        int
	tagStack              *stack.Stack[string]
	endTokenStack         *stack.Stack[[]string]
}

var _ extensions.IParser = &parser{}

func NewParser(stream *lexer.TokenStream, extensions []extensions.IExtension, name, filename, state *string) *parser {
	taggedExtensions := make(map[string]extensionParser, 0)
	for _, extension := range extensions {
		for _, tag := range extension.Tags() {
			taggedExtensions[tag] = extension.Parse
		}
	}

	return &parser{
		stream:         stream,
		name:           name,
		filename:       filename,
		state:          state,
		closed:         false,
		extensions:     taggedExtensions,
		lastIdentifier: 0,
		tagStack:       stack.New[string](),
		endTokenStack:  stack.New[[]string](),
	}
}

// Parse parses the whole template into a `Template` node.
func (p *parser) Parse() (*nodes.Template, error) {
	body, err := p.subparse(nil)
	if err != nil {
		return nil, err
	}

	// TODO set environment
	return &nodes.Template{
		Body:       body,
		NodeCommon: nodes.NodeCommon{Lineno: 1},
	}, nil
}

func (p *parser) subparse(endTokens []string) ([]nodes.Node, error) {
	body := make([]nodes.Node, 0)
	dataBuffer := make([]nodes.Expr, 0)
	addData := func(node nodes.Expr) {
		dataBuffer = append(dataBuffer, node)
	}

	if endTokens != nil {
		p.endTokenStack.Push(endTokens)
		defer p.endTokenStack.Pop()
	}

	flushData := func() {
		if len(dataBuffer) > 0 {
			lineno := dataBuffer[0].GetLineno()
			body = append(body, &nodes.Output{
				Nodes:      dataBuffer,
				StmtCommon: nodes.StmtCommon{Lineno: lineno},
			})
			dataBuffer = make([]nodes.Expr, 0)
		}
	}

	for p.stream.Bool() {
		token := p.stream.Current()
		switch token.Type {
		case lexer.TokenData:
			if token.Value != "" {
				// type assert is safe, because token.Type == lexer.TokenData
				addData(&nodes.TemplateData{
					Data:          token.Value.(string),
					LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
				})
			}
			p.stream.Next()
		case lexer.TokenVariableBegin:
			p.stream.Next()
			tuple, err := p.parseTuple(false, true, nil, false)
			if err != nil {
				return nil, err
			}
			addData(tuple)
			if _, err := p.stream.Expect(lexer.TokenVariableEnd); err != nil {
				return nil, err
			}
		case lexer.TokenBlockBegin:
			flushData()
			p.stream.Next()
			if endTokens != nil && p.stream.Current().TestAny(endTokens...) {
				return body, nil
			}
			rvs, err := p.parseStatement()
			if err != nil {
				return nil, err
			}
			body = append(body, rvs...)
			if _, err := p.stream.Expect(lexer.TokenBlockEnd); err != nil {
				return nil, err
			}
		default:
			return nil, p.fail("internal parsing error", nil)
		}
	}

	flushData()
	return body, nil
}

func (p *parser) parseTuple(simplified bool, withCondexpr bool, extraEndRules []string, explicitParentheses bool) (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	var parse func() (nodes.Expr, error)
	if simplified {
		parse = p.parsePrimary
	} else {
		parse = func() (nodes.Expr, error) {
			return p.parseExpression(withCondexpr)
		}
	}

	var args []nodes.Expr
	var isTuple bool

	for {
		if len(args) > 0 {
			if _, err := p.stream.Expect(lexer.TokenComma); err != nil {
				return nil, err
			}
		}
		if p.isTupleEnd(extraEndRules) {
			break
		}

		arg, err := parse()
		if err != nil {
			return nil, err
		}
		args = append(args, arg)
		if p.stream.Current().Type == lexer.TokenComma {
			isTuple = true
		} else {
			break
		}
		lineno = p.stream.Current().Lineno
	}

	if !isTuple {
		if len(args) > 0 {
			return args[0], nil
		}
		// if we don't have explicit parentheses, an empty tuple is
		// not a valid expression.  This would mean nothing (literally
		// nothing) in the spot of an expression would be an empty
		// tuple.
		if !explicitParentheses {
			return nil, p.fail(fmt.Sprintf("Expected an expression, got %s", p.stream.Current()), nil)
		}
	}

	return &nodes.Tuple{
		Items:         args,
		Ctx:           "load",
		LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
	}, nil
}

func (p *parser) parsePrimary() (nodes.Expr, error) {
	token := p.stream.Current()
	var node nodes.Expr

	switch token.Type {
	case lexer.TokenName:
		switch token.Value {
		case "True", "False", "true", "false":
			node = &nodes.Const{
				Value:         token.Value == "true" || token.Value == "True",
				LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
			}
		case "None", "none":
			node = &nodes.Const{
				Value:         nil,
				LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
			}
		default:
			node = &nodes.Name{
				Name:       token.Value.(string),
				Ctx:        "load",
				ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
			}
		}
		p.stream.Next()
		return node, nil
	case lexer.TokenString:
		p.stream.Next()
		buf := []string{token.Value.(string)}
		for p.stream.Current().Type == lexer.TokenString {
			buf = append(buf, p.stream.Current().Value.(string))
			p.stream.Next()
		}
		return &nodes.Const{
			Value:         strings.Join(buf, ""),
			LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
		}, nil
	case lexer.TokenInteger, lexer.TokenFloat:
		p.stream.Next()
		return &nodes.Const{
			Value:         token.Value,
			LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
		}, nil
	case lexer.TokenLParen:
		p.stream.Next()
		node, err := p.parseTuple(false, true, nil, true)
		if err != nil {
			return nil, err
		}
		if _, err := p.stream.Expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
		return node, nil
	case lexer.TokenLBracket:
		return p.parseList()
	case lexer.TokenLBrace:
		return p.parseDict()
	default:
		return nil, p.fail(fmt.Sprintf("unexpected %q", lexer.DescribeToken(token)), &token.Lineno)
	}
}

func (p *parser) parseExpression(withCondexpr bool) (nodes.Expr, error) {
	if withCondexpr {
		return p.parseCondexpr()
	}
	return p.parseOr()
}

func (p *parser) parseCondexpr() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	expr1, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	var expr3 *nodes.Expr

	for p.stream.SkipIf("name:if") {
		expr2, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.stream.SkipIf("name:else") {
			expr3H, err := p.parseCondexpr()
			if err != nil {
				return nil, err
			}
			expr3 = &expr3H
		} else {
			expr3 = nil
		}
		expr1 = &nodes.CondExpr{
			Test:       expr2,
			Expr1:      expr1,
			Expr2:      expr3,
			ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
		}
		lineno = p.stream.Current().Lineno
	}

	return expr1, nil
}

func (p *parser) parseOr() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.stream.SkipIf("name:or") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = makeBinaryOpExpr(left, right, "or", lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseAnd() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.stream.SkipIf("name:and") {
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = makeBinaryOpExpr(left, right, "and", lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseNot() (nodes.Expr, error) {
	if p.stream.Current().Test("name:not") {
		lineno := p.stream.Next().Lineno
		n, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &nodes.UnaryExpr{
			Node:       n,
			Op:         "not",
			ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
		}, nil
	}
	return p.parseCompare()
}

func (p *parser) parseCompare() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	expr, err := p.parseMath1()
	if err != nil {
		return nil, err
	}
	var ops []nodes.Operand

	addOperand := func(tokenType string) error {
		e, err := p.parseMath1()
		if err != nil {
			return err
		}
		ops = append(ops, nodes.Operand{
			Op:           tokenType,
			Expr:         e,
			HelperCommon: nodes.HelperCommon{Lineno: lineno},
		})
		return nil
	}

	for {
		tokenType := p.stream.Current().Type
		if compareOperators.Has(tokenType) {
			p.stream.Next()
			if err := addOperand(tokenType); err != nil {
				return nil, err
			}
		} else if p.stream.SkipIf("name:in") {
			if err := addOperand("in"); err != nil {
				return nil, err
			}
		} else if p.stream.Current().Test("name:not") && p.stream.Look().Test("name:in") {
			p.stream.Skip(2)
			if err := addOperand("notin"); err != nil {
				return nil, err
			}
		} else {
			break
		}
		lineno = p.stream.Current().Lineno
	}

	if len(ops) == 0 {
		return expr, nil
	}
	return &nodes.Compare{
		Expr:       expr,
		Ops:        ops,
		ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
	}, nil
}

func (p *parser) parseMath1() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parseConcat()
	if err != nil {
		return nil, err
	}

	for p.stream.Current().Type == lexer.TokenAdd || p.stream.Current().Type == lexer.TokenSub {
		currentType := p.stream.Current().Type
		p.stream.Next()
		right, err := p.parseConcat()
		if err != nil {
			return nil, err
		}
		left = makeBinaryOpExpr(left, right, currentType, lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseMath2() (nodes.Expr, error) {
	// TODO it's almost identical as parseMath1
	lineno := p.stream.Current().Lineno
	left, err := p.parsePow()
	if err != nil {
		return nil, err
	}

	for slices.Contains([]string{lexer.TokenMul, lexer.TokenDiv, lexer.TokenFloordiv, lexer.TokenMod}, p.stream.Current().Type) {
		currentType := p.stream.Current().Type
		p.stream.Next()
		right, err := p.parsePow()
		if err != nil {
			return nil, err
		}
		left = makeBinaryOpExpr(left, right, currentType, lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil

}

func (p *parser) parseConcat() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parseMath2()
	if err != nil {
		return nil, err
	}
	args := []nodes.Expr{left}

	for p.stream.Current().Type == lexer.TokenTilde {
		p.stream.Next()
		right, err := p.parseMath2()
		if err != nil {
			return nil, err
		}
		args = append(args, right)
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return &nodes.Concat{
		Nodes:      args,
		ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
	}, nil
}

func (p *parser) parsePow() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parseUnary(true)
	if err != nil {
		return nil, err
	}

	for p.stream.Current().Type == lexer.TokenPow {
		p.stream.Next()
		right, err := p.parseUnary(true)
		if err != nil {
			return nil, err
		}
		left = makeBinaryOpExpr(left, right, lexer.TokenPow, lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseUnary(withFilter bool) (node nodes.Expr, err error) {
	lineno := p.stream.Current().Lineno
	tokenType := p.stream.Current().Type

	if tokenType == lexer.TokenSub || tokenType == lexer.TokenAdd {
		p.stream.Next()
		node, err = p.parseUnary(false)
		if err != nil {
			return
		}
		node = &nodes.UnaryExpr{
			Node:       node,
			Op:         tokenType,
			ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
		}
	} else {
		node, err = p.parsePrimary()
		if err != nil {
			return
		}
	}

	node, err = p.parsePostfix(node)
	if err != nil {
		return
	}

	if withFilter {
		node, err = p.parseFilterExpr(node)
	}

	return
}

func (p *parser) parsePostfix(node nodes.Expr) (nodes.Expr, error) {
	var err error

	for {
		tokenType := p.stream.Current().Type
		if tokenType == lexer.TokenDot || tokenType == lexer.TokenLBracket {
			node, err = p.parseSubscript(node)
			if err != nil {
				return nil, err
			}
		} else if tokenType == lexer.TokenLParen {
			node, err = p.parseCall(node)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}

	return node, nil
}

func (p *parser) parseFilterExpr(node nodes.Expr) (nodes.Expr, error) {
	var err error

	for {
		tokenType := p.stream.Current().Type
		if tokenType == lexer.TokenPipe {
			nP, err := p.parseFilter(&node, false)
			if err != nil {
				return nil, err
			}
			node = *nP
		} else if tokenType == lexer.TokenName && p.stream.Current().Value == "is" {
			node, err = p.parseTest(node)
			if err != nil {
				return nil, err
			}
		} else if tokenType == lexer.TokenLParen {
			node, err = p.parseCall(node)
			if err != nil {
				return nil, err
			}
		} else {
			break
		}
	}

	return node, nil
}

func (p *parser) parseSubscript(node nodes.Expr) (nodes.Expr, error) {
	token := p.stream.Next()
	var arg nodes.Expr

	if token.Type == lexer.TokenDot {
		attrToken := p.stream.Current()
		p.stream.Next()
		if attrToken.Type == lexer.TokenName {
			return &nodes.Getattr{
				Node:       node,
				Attr:       attrToken.Value.(string),
				Ctx:        "load",
				ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: attrToken.Lineno}},
			}, nil
		} else if attrToken.Type != lexer.TokenInteger {
			return nil, p.fail(fmt.Sprintf("expected name or number, got %s", attrToken.Type), &attrToken.Lineno)
		}
		arg = &nodes.Const{
			Value:         attrToken.Value,
			LiteralCommon: nodes.LiteralCommon{NodeCommon: nodes.NodeCommon{Lineno: attrToken.Lineno}},
		}
		return &nodes.Getitem{
			Node:       node,
			Arg:        arg,
			Ctx:        "load",
			ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: attrToken.Lineno}},
		}, nil
	} else if token.Type == lexer.TokenLBracket {
		var args []nodes.Expr
		for p.stream.Current().Type != lexer.TokenRBracket {
			if len(args) > 0 {
				if _, err := p.stream.Expect(lexer.TokenComma); err != nil {
					return nil, err
				}
			}
			arg, err := p.parseSubscribed()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		if _, err := p.stream.Expect(lexer.TokenRBracket); err != nil {
			return nil, err
		}

		if len(args) == 1 {
			arg = args[0]
		} else {
			arg = &nodes.Tuple{
				Items:         args,
				Ctx:           "load",
				LiteralCommon: nodes.LiteralCommon{nodes.NodeCommon{Lineno: token.Lineno}},
			}
		}

		return &nodes.Getitem{
			Node:       node,
			Arg:        arg,
			Ctx:        "load",
			ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
		}, nil
	}

	return nil, p.fail("expected subscript expression", &token.Lineno)
}

func (p *parser) parseSubscribed() (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	var args []*nodes.Expr

	if p.stream.Current().Type == lexer.TokenColon {
		p.stream.Next()
		args = []*nodes.Expr{nil}
	} else {
		node, err := p.parseExpression(true)
		if err != nil {
			return nil, err
		}
		if p.stream.Current().Type != lexer.TokenColon {
			return node, nil
		}
		p.stream.Next()
		args = []*nodes.Expr{&node}
	}

	if p.stream.Current().Type == lexer.TokenColon {
		args = append(args, nil)
	} else if p.stream.Current().Type != lexer.TokenRBracket && p.stream.Current().Type != lexer.TokenComma {
		arg, err := p.parseExpression(true)
		if err != nil {
			return nil, err
		}
		args = append(args, &arg)
	} else {
		args = append(args, nil)
	}

	if p.stream.Current().Type == lexer.TokenColon {
		p.stream.Next()
		if p.stream.Current().Type != lexer.TokenRBracket && p.stream.Current().Type != lexer.TokenComma {
			arg, err := p.parseExpression(true)
			if err != nil {
				return nil, err
			}
			args = append(args, &arg)
		} else {
			args = append(args, nil)
		}
	} else {
		args = append(args, nil)
	}

	var start, stop, step *nodes.Expr
	if len(args) > 0 {
		start = args[0]
	}
	if len(args) > 1 {
		stop = args[1]
	}
	if len(args) > 2 {
		step = args[2]
	}
	return &nodes.Slice{
		Start:      start,
		Stop:       stop,
		Step:       step,
		ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
	}, nil
}

func (p *parser) parseCall(node nodes.Expr) (nodes.Expr, error) {
	lineno := p.stream.Current().Lineno
	args, kwargs, dynArgs, dynKwargs, err := p.parseCallArgs()
	if err != nil {
		return nil, err
	}
	return &nodes.Call{
		Node:       node,
		Args:       args,
		Kwargs:     kwargs,
		DynArgs:    dynArgs,
		DynKwargs:  dynKwargs,
		ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: lineno}},
	}, nil
}

func (p *parser) parseCallArgs() (args []nodes.Expr, kwargs []nodes.Keyword, dynArgs *nodes.Expr, dynKwargs *nodes.Expr, err error) {
	var token *lexer.Token
	token, err = p.stream.Expect(lexer.TokenLParen)
	if err != nil {
		return
	}

	requireComma := false

	ensure := func(expr bool) error {
		if !expr {
			return p.fail("invalid syntax for function call expression", &token.Lineno)
		}
		return nil
	}

	for p.stream.Current().Type != lexer.TokenRParen {
		if requireComma {
			if _, err = p.stream.Expect(lexer.TokenComma); err != nil {
				return
			}
			// support for trailing comma
			if p.stream.Current().Type == lexer.TokenRParen {
				break
			}
		}

		var expr nodes.Expr
		if p.stream.Current().Type == lexer.TokenMul {
			if err = ensure(dynArgs == nil && dynKwargs == nil); err != nil {
				return
			}
			p.stream.Next()
			expr, err = p.parseExpression(true)
			if err != nil {
				return
			}
			dynArgs = &expr
		} else if p.stream.Current().Type == lexer.TokenPow {
			if err = ensure(dynKwargs == nil); err != nil {
				return
			}
			p.stream.Next()
			expr, err = p.parseExpression(true)
			if err != nil {
				return
			}
			dynKwargs = &expr
		} else {
			if p.stream.Current().Type == lexer.TokenName && p.stream.Look().Type == lexer.TokenAssign {
				// Parsing a kwarg
				if err = ensure(dynKwargs == nil); err != nil {
					return
				}
				key := p.stream.Current().Value
				p.stream.Skip(2)
				expr, err = p.parseExpression(true)
				if err != nil {
					return
				}
				kwargs = append(kwargs, nodes.Keyword{
					Key:          key.(string),
					Value:        expr,
					HelperCommon: nodes.HelperCommon{Lineno: expr.GetLineno()},
				})
			} else {
				// Parsing an arg
				if err = ensure(dynArgs == nil && dynKwargs == nil && len(kwargs) == 0); err != nil {
					return
				}
				expr, err = p.parseExpression(true)
				if err != nil {
					return
				}
				args = append(args, expr)
			}
		}

		requireComma = true
	}

	_, err = p.stream.Expect(lexer.TokenRParen)
	return
}

func (p *parser) parseFilter(node *nodes.Expr, startInline bool) (*nodes.Expr, error) {
	for p.stream.Current().Type == lexer.TokenPipe || startInline {
		if !startInline {
			p.stream.Next()
		}

		token, err := p.stream.Expect(lexer.TokenName)
		if err != nil {
			return nil, err
		}
		name := token.Value.(string)
		for p.stream.Current().Type == lexer.TokenDot {
			p.stream.Next()
			token, err = p.stream.Expect(lexer.TokenName)
			if err != nil {
				return nil, err
			}
			name += "." + token.Value.(string)
		}

		var args []nodes.Expr
		var kwargs []nodes.Keyword
		var dynArgs *nodes.Expr
		var dynKwargs *nodes.Expr
		if p.stream.Current().Type == lexer.TokenLParen {
			args, kwargs, dynArgs, dynKwargs, err = p.parseCallArgs()
			if err != nil {
				return nil, err
			}
		}

		var f nodes.Expr = &nodes.Filter{
			FilterTestCommon: nodes.FilterTestCommon{
				Node:       node,
				Name:       name,
				Args:       args,
				Kwargs:     kwargs,
				DynArgs:    dynArgs,
				DynKwargs:  dynKwargs,
				ExprCommon: nodes.ExprCommon{NodeCommon: nodes.NodeCommon{Lineno: token.Lineno}},
			},
		}
		node = &f

		startInline = false
	}
	return node, nil
}

func (p *parser) parseTest(node nodes.Node) (nodes.Expr, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseList() (nodes.Expr, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseDict() (nodes.Expr, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) isTupleEnd(extraEndRules []string) bool {
	current := p.stream.Current()
	if slices.Contains([]string{lexer.TokenVariableEnd, lexer.TokenBlockEnd, lexer.TokenRParen}, current.Type) {
		return true
	} else if extraEndRules != nil {
		return current.TestAny(extraEndRules...)
	}
	return false
}

func (p *parser) parseStatement() ([]nodes.Node, error) {
	token := p.stream.Current()
	if token.Type != lexer.TokenName {
		return nil, p.fail("tag name expected", &token.Lineno)
	}
	p.tagStack.Push(token.Value.(string))
	popTag := true
	defer func() {
		if popTag {
			p.tagStack.Pop()
		}
	}()

	wrapInSlice := func(f func() (nodes.Node, error)) ([]nodes.Node, error) {
		n, err := f()
		if err != nil {
			return nil, err
		}
		return []nodes.Node{n}, nil
	}

	if tokenValue, ok := token.Value.(string); ok {
		if statementKeywords.Has(tokenValue) {
			switch tokenValue {
			case "for":
				return wrapInSlice(p.parseFor)
			case "if":
				return wrapInSlice(p.parseIf)
			case "block":
				return wrapInSlice(p.parseBlock)
			case "extends":
				return wrapInSlice(p.parseExtends)
			case "print":
				return wrapInSlice(p.parsePrint)
			case "macro":
				return wrapInSlice(p.parseMacro)
			case "include":
				return wrapInSlice(p.parseInclude)
			case "from":
				return wrapInSlice(p.parseFrom)
			case "import":
				return wrapInSlice(p.parseImport)
			case "set":
				return wrapInSlice(p.parseSet)
			case "with":
				return wrapInSlice(p.parseWith)
			case "autoescape":
				return wrapInSlice(p.parseAutoescape)
			default:
				panic("unexpected statement keyword " + tokenValue)
			}
		}
	}

	if token.Value == "call" {
		return p.parseCallBlock()
	} else if token.Value == "filter" {
		b, err := p.parseFilterBlock()
		if err != nil {
			return nil, err
		}
		ret := make([]nodes.Node, 1)
		ret[0] = b
		return ret, nil
	} else if tokenValue, ok := token.Value.(string); ok {
		if ext, ok := p.extensions[tokenValue]; ok {
			return ext(p)
		}
	}

	// did not work out, remove the token we pushed by accident
	// from the stack so that the unknown tag fail function can
	// produce a proper error message.
	p.tagStack.Pop()
	popTag = false
	return nil, p.failUnknownTag(token.Value.(string), &token.Lineno)
}

func (p *parser) parseFor() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseIf() (nodes.Node, error) {
	tok, err := p.stream.Expect("name:if")
	if err != nil {
		return nil, err
	}
	node := &nodes.If{
		StmtCommon: nodes.StmtCommon{Lineno: tok.Lineno},
	}
	result := node

	for {
		node.Test, err = p.parseTuple(false, false, nil, false)
		if err != nil {
			return nil, err
		}
		node.Body, err = p.parseStatements([]string{"name:elif", "name:else", "name:endif"}, false)
		if err != nil {
			return nil, err
		}
		node.Elif = []nodes.If{}
		node.Else = []nodes.Node{}
		token := p.stream.Next()
		if token.Test("name:elif") {
			node = &nodes.If{
				StmtCommon: nodes.StmtCommon{Lineno: token.Lineno},
			}
			result.Elif = append(result.Elif, *node)
			continue
		} else if token.Test("name:else") {
			result.Else, err = p.parseStatements([]string{"name:endif"}, true)
			if err != nil {
				return nil, err
			}
		}
		break
	}

	return result, nil
}

func (p *parser) parseStatements(endTokens []string, dropNeedle bool) ([]nodes.Node, error) {
	p.stream.SkipIf(lexer.TokenColon)
	if _, err := p.stream.Expect(lexer.TokenBlockEnd); err != nil {
		return nil, err
	}
	result, err := p.subparse(endTokens)
	if err != nil {
		return nil, err
	}

	// we reached the end of the template too early, the subparser
	// does not check for this, so we do that now
	if p.stream.Current().Type == lexer.TokenEOF {
		return nil, p.failEOF(endTokens, nil)
	}

	if dropNeedle {
		p.stream.Next()
	}

	return result, nil
}

func (p *parser) parseBlock() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseExtends() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parsePrint() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseMacro() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseInclude() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseFrom() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseImport() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseSet() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseWith() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseAutoescape() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseCallBlock() ([]nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseFilterBlock() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) fail(msg string, lineno *int) error {
	var lineNumber int
	if lineno == nil {
		lineNumber = p.stream.Current().Lineno
	} else {
		lineNumber = *lineno
	}
	return errors.TemplateSyntaxError(msg, lineNumber, p.name, p.filename)
}

func (p *parser) failUnknownTag(name string, lineno *int) error {
	return p.failUtEof(&name, p.endTokenStack, lineno)
}

func (p *parser) failEOF(endTokens []string, lineno *int) error {
	if endTokens != nil {
		p.endTokenStack.Push(endTokens)
	}
	return p.failUtEof(nil, p.endTokenStack, lineno)
}

func (p *parser) failUtEof(name *string, endTokenStack *stack.Stack[[]string], lineno *int) error {
	endTokenStackSlice := endTokenStack.Iter()
	expected := set.New[string]()

	for _, exprs := range endTokenStackSlice {
		for _, expr := range exprs {
			expected.Add(lexer.DescribeTokenExpr(expr))
		}
	}

	var currentlyLooking *string
	if len(endTokenStackSlice) > 0 {
		lastEndToken := endTokenStackSlice[len(endTokenStackSlice)-1]
		var described []string
		for _, expr := range lastEndToken {
			described = append(described, fmt.Sprintf("%q", lexer.DescribeTokenExpr(expr)))
		}
		v := strings.Join(described, " or ")
		currentlyLooking = &v
	}

	var messages []string
	if name == nil {
		messages = append(messages, "Unexpected end of template.")
	} else {
		messages = append(messages, fmt.Sprintf("Encountered unknown tag %q.", *name))
	}

	if currentlyLooking != nil {
		if name != nil && expected.Has(*name) {
			messages = append(messages, "You probably made a nesting mistake. Jinja is expecting this tag, but currently looking for "+*currentlyLooking+".")
		} else {
			messages = append(messages, "Jinja was looking for the following tags: "+*currentlyLooking+".")
		}
	}

	if lastTag := p.tagStack.Peek(); lastTag != nil {
		messages = append(messages, fmt.Sprintf("The innermost block that needs to be closed is %q.", *lastTag))
	}

	return p.fail(strings.Join(messages, " "), lineno)
}
