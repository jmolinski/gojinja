package environment

import "github.com/gojinja/gojinja/src/lexer"

type Environment struct {
	TemplateClass Class
	*lexer.EnvLexerInformation
}
