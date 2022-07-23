package lexer

import (
	"reflect"
	"testing"
)

type testLexer struct {
	input string
	res   []Token
}

var cases = []testLexer{
	{input: `{{ name }}`,
		res: []Token{
			{1, TokenVariableBegin, "{{"},
			{1, TokenName, "name"},
			{1, TokenVariableEnd, "}}"},
		},
	},
	{input: `{% if name != "OFF" %}
my name is {{ name }}
{% endif %}
{{ 5 + 1 }}`,
		res: []Token{
			{1, TokenBlockBegin, "{%"},
			{1, TokenName, "if"},
			{1, TokenName, "name"},
			{1, TokenNe, "!="},
			{1, TokenString, "OFF"},
			{1, TokenBlockEnd, "%}"},
			{1, TokenData, "\nmy name is "},
			{2, TokenVariableBegin, "{{"},
			{2, TokenName, "name"},
			{2, TokenVariableEnd, "}}"},
			{2, TokenData, "\n"},
			{3, TokenBlockBegin, "{%"},
			{3, TokenName, "endif"},
			{3, TokenBlockEnd, "%}"},
			{3, TokenData, "\n"},
			{4, TokenVariableBegin, "{{"},
			{4, TokenInteger, int64(5)},
			{4, TokenAdd, "+"},
			{4, TokenInteger, int64(1)},
			{4, TokenVariableEnd, "}}"},
		},
	},
}

func Test(t *testing.T) {
	for _, c := range cases {
		runCase(c, t)
	}
}

func runCase(c testLexer, t *testing.T) {
	info := DefaultEnvLexerInformation()
	l := GetLexer(info)
	s, err := l.Tokenize(c.input, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	i := 0
	for !s.Eos() {
		tok := s.Next()
		if i == len(c.res) {
			t.Fatal("unexpected token", tok)
		}
		if !reflect.DeepEqual(tok, c.res[i]) {
			t.Fatal("expected", c.res[i], "got", tok)
		}
		i++
	}
	if i < len(c.res) {
		t.Fatal("not all tokens have been produced")
	}
}

func TestCountNewlines(t *testing.T) {
	if CountNewlines("\nb\n\naaaaaa") != 3 {
		t.Fatal("expected 3 newlines")
	}
	if CountNewlines("\rb\r\raaaaaa") != 3 {
		t.Fatal("expected 3 newlines")
	}
	if CountNewlines("\r\nb\n\r\r\n\r\n\naaaaaa") != 6 {
		t.Fatal("expected 6 newlines")
	}
}

type unescapeStringTest struct {
	unescaped string
	escape    string
}

var unescapeStringCases = []unescapeStringTest{
	{"a\\\"b\\\"", "a\"b\""},
	{"a\\'b\\'", "a'b'"},
	{"a\\'b\\\"", "a'b\""},
	{"a\\\\'b\\\"", "a\\\\'b\""},
	{"a", "a"},
}

func TestUnescapeString(t *testing.T) {
	for _, c := range unescapeStringCases {
		res := unescapeString(c.unescaped)
		if res != unescapeString(c.escape) {
			t.Fatal("got: ", c.unescaped, "expected: ", c.escape)
		}
	}
}
