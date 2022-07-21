package lexer

import (
	"testing"
)

const example = `
{% if name != "OFF" %}
my name is {{ name }}
{% endif %}
{{ 5 + 1 }}
`

func Test(t *testing.T) {
	info := &EnvLexerInformation{}
	l := GetLexer(info)
	s, err := l.Tokenize(example, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(s)
}
