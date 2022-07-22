package defaults

const BlockStartString = "{%"
const BlockEndString = "%}"
const VariableStartString = "{{"
const VariableEndString = "}}"
const CommentStartString = "{#"
const CommentEndString = "#}"
const TrimBlocks = false
const LStripBlocks = false
const NewlineSequence = "\n"
const KeepTrailingNewline = false

var LineStatementPrefix *string = nil
var LineCommentPrefix *string = nil

var DefaultNamespace = map[string]any{
	// TODO fill
}
var DefaultPolicies = map[string]any{
	"compiler.ascii_str":   true,
	"urlize.rel":           "noopener",
	"urlize.target":        nil,
	"urlize.extra_schemes": nil,
	"truncate.leeway":      5,
	"json.dumps_function":  nil,
	"json.dumps_kwargs":    map[string]any{"sort_keys": true},
	"ext.i18n.trimmed":     false,
}
