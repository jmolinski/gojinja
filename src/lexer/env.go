package lexer

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
