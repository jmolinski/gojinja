package lexer

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
)

type TokenStream struct {
	tokens   []Token
	name     *string
	filename *string
	closed   bool
	current  Token
	idx      int
}

func NewTokenStream(tokens []Token, name *string, filename *string) *TokenStream {
	ret := &TokenStream{
		tokens:   tokens,
		name:     name,
		filename: filename,
		closed:   false,
		current:  Token{1, TokenInitial, ""},
		idx:      0,
	}
	_ = ret.Next()
	return ret
}

func (ts *TokenStream) Next() Token {
	rv := ts.current

	if ts.current.type_ != TokenEof {
		if ts.idx < len(ts.tokens) {
			ts.current = ts.tokens[ts.idx]
			ts.idx += 1
		} else {
			ts.Close()
		}
	}

	return rv
}

func (ts *TokenStream) Close() {
	ts.current = Token{ts.current.lineno, TokenEof, ""}
	ts.closed = true
}

func (ts TokenStream) Bool() bool {
	return ts.current.type_ != TokenEof
}

func (ts TokenStream) Eos() bool {
	return !ts.Bool()
}

func (ts TokenStream) Look() Token {
	if ts.idx+1 < len(ts.tokens) {
		return ts.tokens[ts.idx+1]
	}
	return ts.current
}

func (ts *TokenStream) Skip(n int) {
	for i := 0; i < n; i++ {
		ts.Next()
	}
}

func (ts *TokenStream) NextIf(expr string) *Token {
	if ts.current.Test(expr) {
		t := ts.Next()
		return &t
	}
	return nil
}

func (ts *TokenStream) SkipIf(expr string) bool {
	return ts.NextIf(expr) != nil
}

func (ts *TokenStream) Expect(expr string) (*Token, error) {
	if !ts.current.Test(expr) {
		desc := DescribeTokenExpr(expr)

		if ts.current.type_ == TokenEof {
			return nil, errors.TemplateSyntaxError(
				fmt.Sprintf("unexpected end of template, expected '%s'.", desc),
				ts.current.lineno,
				ts.name,
				ts.filename,
			)
		}
		return nil, errors.TemplateSyntaxError(
			fmt.Sprintf("expected token '%s', got '%s'", desc, DescribeToken(ts.current)),
			ts.current.lineno,
			ts.name,
			ts.filename,
		)
	}
	next := ts.Next()
	return &next, nil
}
