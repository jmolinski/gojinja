package lexer

import "github.com/gojinja/gojinja/src/defaults"

type EnvLexerInformation struct {
	BlockStartString    string
	BlockEndString      string
	VariableStartString string
	VariableEndString   string
	CommentStartString  string
	CommentEndString    string
	LineStatementPrefix *string
	LineCommentPrefix   *string
	TrimBlocks          bool
	LStripBlocks        bool
	NewlineSequence     string
	KeepTrailingNewline bool
}

func DefaultEnvLexerInformation() *EnvLexerInformation {
	return &EnvLexerInformation{
		BlockStartString:    defaults.BlockStartString,
		BlockEndString:      defaults.BlockEndString,
		VariableStartString: defaults.VariableStartString,
		VariableEndString:   defaults.VariableEndString,
		CommentStartString:  defaults.CommentStartString,
		CommentEndString:    defaults.CommentEndString,
		LineStatementPrefix: defaults.LineStatementPrefix,
		LineCommentPrefix:   defaults.LineCommentPrefix,
		TrimBlocks:          defaults.TrimBlocks,
		LStripBlocks:        defaults.LstripBlocks,
		NewlineSequence:     defaults.NewlineSequence,
		KeepTrailingNewline: defaults.KeepTrailingNewline,
	}
}
