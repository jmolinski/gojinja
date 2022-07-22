package defaults

const BlockStartString = "{%"
const BlockEndString = "%}"
const VariableStartString = "{{"
const VariableEndString = "}}"
const CommentStartString = "{#"
const CommentEndString = "#}"
const TrimBlocks = false
const LstripBlocks = false
const NewlineSequence = "\n"
const KeepTrailingNewline = false

var LineStatementPrefix *string = nil
var LineCommentPrefix *string = nil

var DefaultNamespace = map[string]func(...any) any{}
var DefaultPolicies = map[string]any{}
