package parser

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
	"github.com/gojinja/gojinja/src/extensions"
	"github.com/gojinja/gojinja/src/lexer"
	"github.com/gojinja/gojinja/src/nodes"
	"github.com/gojinja/gojinja/src/utils/set"
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

func makeMathNode(left, right nodes.Node, op string, lineno int) nodes.Node {
	if op == "add" {
		return &nodes.Add{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else if op == "sub" {
		return &nodes.Sub{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else if op == "mul" {
		return &nodes.Mul{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else if op == "div" {
		return &nodes.Div{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else if op == "floordiv" {
		return &nodes.FloorDiv{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else if op == "mod" {
		return &nodes.Mod{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else {
		panic("unknown operator")
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
	dataBuffer := make([]nodes.Node, 0)
	addData := func(node nodes.Node) {
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
				NodeCommon: nodes.NodeCommon{Lineno: lineno},
			})
			dataBuffer = make([]nodes.Node, 0)
		}
	}

	for p.stream.Bool() {
		token := p.stream.Current()
		if token.Type == lexer.TokenData {
			if token.Value != "" {
				// type assert is safe, because token.Type == lexer.TokenData
				addData(&nodes.TemplateData{
					Data:       token.Value.(string),
					NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
				})
			}
			p.stream.Next()
		} else if token.Type == lexer.TokenVariableBegin {
			p.stream.Next()
			tuple, err := p.parseTuple(false, true, nil, false)
			if err != nil {
				return nil, err
			}
			addData(tuple)
			if _, err := p.stream.Expect(lexer.TokenVariableEnd); err != nil {
				return nil, err
			}
		} else if token.Type == lexer.TokenBlockBegin {
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
		} else {
			return nil, p.fail("internal parsing error", nil)
		}
	}

	flushData()
	return body, nil
}

func (p *parser) parseTuple(simplified bool, withCondexpr bool, extraEndRules []string, explicitParentheses bool) (nodes.Node, error) {
	lineno := p.stream.Current().Lineno
	var parse func() (nodes.Node, error)
	if simplified {
		parse = p.parsePrimary
	} else {
		parse = func() (nodes.Node, error) {
			return p.parseExpression(withCondexpr)
		}
	}

	var args []nodes.Node
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
		Items:      args,
		Ctx:        "load",
		NodeCommon: nodes.NodeCommon{Lineno: lineno},
	}, nil
}

func (p *parser) parsePrimary() (nodes.Node, error) {
	token := p.stream.Current()
	var node nodes.Node
	if token.Type == lexer.TokenName {
		if token.Value == "true" || token.Value == "false" || token.Value == "True" || token.Value == "False" {
			node = &nodes.Const{
				Value:      token.Value == "true" || token.Value == "True",
				NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
			}
		} else if token.Value == "none" || token.Value == "None" {
			node = &nodes.Const{
				Value:      nil,
				NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
			}
		} else {
			node = &nodes.Name{
				Name:       token.Value.(string),
				Ctx:        "load",
				NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
			}
		}
		p.stream.Next()
		return node, nil
	} else if token.Type == lexer.TokenString {
		p.stream.Next()
		buf := []string{token.Value.(string)}
		lineno := token.Lineno
		for p.stream.Current().Type == lexer.TokenString {
			buf = append(buf, p.stream.Current().Value.(string))
			p.stream.Next()
		}
		return &nodes.Const{
			Value:      strings.Join(buf, ""),
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}, nil
	} else if token.Type == lexer.TokenInteger || token.Type == lexer.TokenFloat {
		p.stream.Next()
		return &nodes.Const{
			Value:      token.Value,
			NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
		}, nil
	} else if token.Type == lexer.TokenLParen {
		p.stream.Next()
		node, err := p.parseTuple(false, true, nil, true)
		if err != nil {
			return nil, err
		}
		if _, err := p.stream.Expect(lexer.TokenRParen); err != nil {
			return nil, err
		}
		return node, nil
	} else if token.Type == lexer.TokenLBracket {
		return p.parseList()
	} else if token.Type == lexer.TokenLBrace {
		return p.parseDict()
	} else {
		return nil, p.fail(fmt.Sprintf("unexpected %q", lexer.DescribeToken(token)), &token.Lineno)
	}
}

func (p *parser) parseExpression(withCondexpr bool) (nodes.Node, error) {
	if withCondexpr {
		return p.parseCondexpr()
	}
	return p.parseOr()
}

func (p *parser) parseCondexpr() (nodes.Node, error) {
	lineno := p.stream.Current().Lineno
	expr1, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	var expr3 nodes.Node

	for p.stream.SkipIf("name:if") {
		expr2, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.stream.SkipIf("name:else") {
			expr3, err = p.parseCondexpr()
			if err != nil {
				return nil, err
			}
		} else {
			expr3 = nil
		}
		expr1 = &nodes.CondExpr{
			Test:       expr2,
			Expr1:      expr1,
			Expr2:      expr3,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
		lineno = p.stream.Current().Lineno
	}

	return expr1, nil
}

func (p *parser) parseOr() (nodes.Node, error) {
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
		left = &nodes.Or{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseAnd() (nodes.Node, error) {
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
		left = &nodes.And{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseNot() (nodes.Node, error) {
	if p.stream.Current().Test("name:not") {
		lineno := p.stream.Next().Lineno
		n, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &nodes.Not{
			Node:       n,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}, nil
	}
	return p.parseCompare()
}

func (p *parser) parseCompare() (nodes.Node, error) {
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
			Op:         tokenType,
			Expr:       e,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
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
		NodeCommon: nodes.NodeCommon{Lineno: lineno},
	}, nil
}

func (p *parser) parseMath1() (nodes.Node, error) {
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
		left = makeMathNode(left, right, currentType, lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseMath2() (nodes.Node, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parsePow()
	if err != nil {
		return nil, err
	}

	for p.stream.Current().Type == lexer.TokenMul || p.stream.Current().Type == lexer.TokenDiv || p.stream.Current().Type == lexer.TokenFloordiv || p.stream.Current().Type == lexer.TokenMod {
		currentType := p.stream.Current().Type
		p.stream.Next()
		right, err := p.parsePow()
		if err != nil {
			return nil, err
		}
		left = makeMathNode(left, right, currentType, lineno)
		lineno = p.stream.Current().Lineno
	}
	return left, nil

}

func (p *parser) parseConcat() (nodes.Node, error) {
	lineno := p.stream.Current().Lineno
	left, err := p.parseMath2()
	if err != nil {
		return nil, err
	}
	args := []nodes.Node{left}

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
		NodeCommon: nodes.NodeCommon{Lineno: lineno},
	}, nil
}

func (p *parser) parsePow() (nodes.Node, error) {
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
		left = &nodes.Pow{
			Left:       left,
			Right:      right,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
		lineno = p.stream.Current().Lineno
	}
	return left, nil
}

func (p *parser) parseUnary(withFilter bool) (nodes.Node, error) {
	//         token_type = self.stream.current.type
	//        lineno = self.stream.current.lineno
	//        node: nodes.Expr
	//
	//        if token_type == "sub":
	//            next(self.stream)
	//            node = nodes.Neg(self.parse_unary(False), lineno=lineno)
	//        elif token_type == "add":
	//            next(self.stream)
	//            node = nodes.Pos(self.parse_unary(False), lineno=lineno)
	//        else:
	//            node = self.parse_primary()
	//        node = self.parse_postfix(node)
	//        if with_filter:
	//            node = self.parse_filter_expr(node)
	//        return node

	lineno := p.stream.Current().Lineno
	tokenType := p.stream.Current().Type
	var node nodes.Node
	var err error

	if tokenType == lexer.TokenSub || tokenType == lexer.TokenAdd {
		p.stream.Next()
		node, err = p.parseUnary(false)
		if err != nil {
			return nil, err
		}
	}
	if tokenType == lexer.TokenSub {
		node = &nodes.Neg{
			Node:       node,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else if tokenType == lexer.TokenAdd {
		node = &nodes.Pos{
			Node:       node,
			NodeCommon: nodes.NodeCommon{Lineno: lineno},
		}
	} else {
		node, err = p.parsePrimary()
		if err != nil {
			return nil, err
		}
	}

	node, err = p.parsePostfix(node)
	if err != nil {
		return nil, err
	}

	if withFilter {
		node, err = p.parseFilterExpr(node)
	}

	return node, err
}

func (p *parser) parsePostfix(node nodes.Node) (nodes.Node, error) {
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

func (p *parser) parseFilterExpr(node nodes.Node) (nodes.Node, error) {
	var err error

	for {
		tokenType := p.stream.Current().Type
		if tokenType == lexer.TokenPipe {
			node, err = p.parseFilter(node, false)
			if err != nil {
				return nil, err
			}
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

func (p *parser) parseSubscript(node nodes.Node) (nodes.Node, error) {
	token := p.stream.Next()
	var arg nodes.Node

	if token.Type == lexer.TokenDot {
		attrToken := p.stream.Current()
		p.stream.Next()
		if attrToken.Type == lexer.TokenName {
			return &nodes.Getattr{
				Node:       node,
				Attr:       attrToken.Value.(string),
				Ctx:        "load",
				NodeCommon: nodes.NodeCommon{Lineno: attrToken.Lineno},
			}, nil
		} else if attrToken.Type != lexer.TokenInteger {
			return nil, p.fail(fmt.Sprintf("expected name or number, got %s", attrToken.Type), &attrToken.Lineno)
		}
		arg = &nodes.Const{
			Value:      attrToken.Value,
			NodeCommon: nodes.NodeCommon{Lineno: attrToken.Lineno},
		}
		return &nodes.Getitem{
			Node:       node,
			Arg:        arg,
			Ctx:        "load",
			NodeCommon: nodes.NodeCommon{Lineno: attrToken.Lineno},
		}, nil
	} else if token.Type == lexer.TokenLBracket {
		var args []nodes.Node
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
				Items:      args,
				Ctx:        "load",
				NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
			}
		}

		return &nodes.Getitem{
			Node:       node,
			Arg:        arg,
			Ctx:        "load",
			NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
		}, nil
	}

	return nil, p.fail("expected subscript expression", &token.Lineno)
}

func (p *parser) parseSubscribed() (nodes.Node, error) {
	lineno := p.stream.Current().Lineno
	var args []*nodes.Node

	if p.stream.Current().Type == lexer.TokenColon {
		p.stream.Next()
		args = []*nodes.Node{nil}
	} else {
		node, err := p.parseExpression(true)
		if err != nil {
			return nil, err
		}
		if p.stream.Current().Type != lexer.TokenColon {
			return node, nil
		}
		p.stream.Next()
		args = []*nodes.Node{&node}
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

	var start, stop, step *nodes.Node
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
		NodeCommon: nodes.NodeCommon{Lineno: lineno},
	}, nil
}

func (p *parser) parseCall(node nodes.Node) (nodes.Node, error) {
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
		NodeCommon: nodes.NodeCommon{Lineno: lineno},
	}, nil
}

func (p *parser) parseCallArgs() ([]nodes.Node, []nodes.Node, *nodes.Node, *nodes.Node, error) {
	token, err := p.stream.Expect(lexer.TokenLParen)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	var args []nodes.Node
	var kwargs []nodes.Node
	var dynArgs *nodes.Node
	var dynKwargs *nodes.Node
	requireComma := false

	ensure := func(expr bool) error {
		if !expr {
			return p.fail("invalid syntax for function call expression", &token.Lineno)
		}
		return nil
	}

	for p.stream.Current().Type != lexer.TokenRParen {
		if requireComma {
			if _, err := p.stream.Expect(lexer.TokenComma); err != nil {
				return nil, nil, nil, nil, err
			}
			// support for trailing comma
			if p.stream.Current().Type == lexer.TokenRParen {
				break
			}
		}

		if p.stream.Current().Type == lexer.TokenMul {
			if err := ensure(dynArgs == nil && dynKwargs == nil); err != nil {
				return nil, nil, nil, nil, err
			}
			p.stream.Next()
			e, err := p.parseExpression(true)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			dynArgs = &e
		} else if p.stream.Current().Type == lexer.TokenPow {
			if err := ensure(dynKwargs == nil); err != nil {
				return nil, nil, nil, nil, err
			}
			p.stream.Next()
			e, err := p.parseExpression(true)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			dynKwargs = &e
		} else {
			if p.stream.Current().Type == lexer.TokenName && p.stream.Look().Type == lexer.TokenAssign {
				// Parsing a kwarg
				if err := ensure(dynKwargs == nil); err != nil {
					return nil, nil, nil, nil, err
				}
				key := p.stream.Current().Value
				p.stream.Skip(2)
				value, err := p.parseExpression(true)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				kwargs = append(kwargs, &nodes.Keyword{
					Key:        key.(string),
					Value:      value,
					NodeCommon: nodes.NodeCommon{Lineno: value.GetLineno()},
				})
			} else {
				// Parsing an arg
				if err := ensure(dynArgs == nil && dynKwargs == nil && len(kwargs) == 0); err != nil {
					return nil, nil, nil, nil, err
				}
				arg, err := p.parseExpression(true)
				if err != nil {
					return nil, nil, nil, nil, err
				}
				args = append(args, arg)
			}
		}

		requireComma = true
	}

	if _, err := p.stream.Expect(lexer.TokenRParen); err != nil {
		return nil, nil, nil, nil, err
	}
	return args, kwargs, dynArgs, dynKwargs, nil
}

func (p *parser) parseFilter(node nodes.Node, startInline bool) (nodes.Node, error) {
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

		var args []nodes.Node
		var kwargs []nodes.Node
		var dynArgs *nodes.Node
		var dynKwargs *nodes.Node
		if p.stream.Current().Type == lexer.TokenLParen {
			args, kwargs, dynArgs, dynKwargs, err = p.parseCallArgs()
			if err != nil {
				return nil, err
			}
		} else {
			args = []nodes.Node{}
			kwargs = []nodes.Node{}
		}

		node = &nodes.Filter{
			Node:       node,
			Name:       name,
			Args:       args,
			Kwargs:     kwargs,
			DynArgs:    dynArgs,
			DynKwargs:  dynKwargs,
			NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
		}

		startInline = false
	}
	return node, nil
}

func (p *parser) parseTest(node nodes.Node) (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseList() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) parseDict() (nodes.Node, error) {
	// TODO
	panic("not implemented")
}

func (p *parser) isTupleEnd(extraEndRules []string) bool {
	current := p.stream.Current()
	if current.Type == lexer.TokenVariableEnd || current.Type == lexer.TokenBlockEnd || current.Type == lexer.TokenRParen {
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
		return p.parseFilterBlock()
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
		NodeCommon: nodes.NodeCommon{Lineno: tok.Lineno},
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
				NodeCommon: nodes.NodeCommon{Lineno: token.Lineno},
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

func (p *parser) parseFilterBlock() ([]nodes.Node, error) {
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
	endTokenStackSlice := endTokenStack.AsSlice()
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

	if p.tagStack.Len() > 0 {
		lastTag := *p.tagStack.Peek()
		messages = append(messages, fmt.Sprintf("The innermost block that needs to be closed is %q.", lastTag))
	}

	return p.fail(strings.Join(messages, " "), lineno)
}
