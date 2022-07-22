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

var DefaultNamespace = map[string]any{}
var DefaultPolicies = map[string]any{}
