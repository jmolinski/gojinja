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
	dataBuffer := make([]nodes.Node, 0) // TODO []nodes.Expr?
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
		return nil, nil
	}

	flushData()
	return body, nil
}

func (p *parser) parseTuple(simplified bool, withCondexpr bool, extraEndRules []string, explicitParentheses bool) (*nodes.Tuple, error) {
	// TODO
	return nil, nil
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

	if tokenValue, ok := token.Value.(string); ok {
		if statementKeywords.Has(tokenValue) {
			// TODO big switch
			//     f = getattr(self, f"parse_{self.stream.current.value}")
			//     return f()  # type: ignore
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

func (p *parser) parseCallBlock() ([]nodes.Node, error) {
	// TODO
	return nil, nil
}

func (p *parser) parseFilterBlock() ([]nodes.Node, error) {
	// TODO
	return nil, nil
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
