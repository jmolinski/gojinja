package lexer

import (
	"fmt"
	"github.com/gojinja/gojinja/src/utils/maps"
	"github.com/gojinja/gojinja/src/utils/set"
	"os"
	"regexp"
	"sort"
	"strings"
)

const TOKEN_ADD = "add"
const TOKEN_ASSIGN = "assign"
const TOKEN_COLON = "colon"
const TOKEN_COMMA = "comma"
const TOKEN_DIV = "div"
const TOKEN_DOT = "dot"
const TOKEN_EQ = "eq"
const TOKEN_FLOORDIV = "floordiv"
const TOKEN_GT = "gt"
const TOKEN_GTEQ = "gteq"
const TOKEN_LBRACE = "lbrace"
const TOKEN_LBRACKET = "lbracket"
const TOKEN_LPAREN = "lparen"
const TOKEN_LT = "lt"
const TOKEN_LTEQ = "lteq"
const TOKEN_MOD = "mod"
const TOKEN_MUL = "mul"
const TOKEN_NE = "ne"
const TOKEN_PIPE = "pipe"
const TOKEN_POW = "pow"
const TOKEN_RBRACE = "rbrace"
const TOKEN_RBRACKET = "rbracket"
const TOKEN_RPAREN = "rparen"
const TOKEN_SEMICOLON = "semicolon"
const TOKEN_SUB = "sub"
const TOKEN_TILDE = "tilde"
const TOKEN_WHITESPACE = "whitespace"
const TOKEN_FLOAT = "float"
const TOKEN_INTEGER = "integer"
const TOKEN_NAME = "name"
const TOKEN_STRING = "string"
const TOKEN_OPERATOR = "operator"
const TOKEN_BLOCK_BEGIN = "block_begin"
const TOKEN_BLOCK_END = "block_end"
const TOKEN_VARIABLE_BEGIN = "variable_begin"
const TOKEN_VARIABLE_END = "variable_end"
const TOKEN_RAW_BEGIN = "raw_begin"
const TOKEN_RAW_END = "raw_end"
const TOKEN_COMMENT_BEGIN = "comment_begin"
const TOKEN_COMMENT_END = "comment_end"
const TOKEN_COMMENT = "comment"
const TOKEN_LINESTATEMENT_BEGIN = "linestatement_begin"
const TOKEN_LINESTATEMENT_END = "linestatement_end"
const TOKEN_LINECOMMENT_BEGIN = "linecomment_begin"
const TOKEN_LINECOMMENT_END = "linecomment_end"
const TOKEN_LINECOMMENT = "linecomment"
const TOKEN_DATA = "data"
const TOKEN_INITIAL = "initial"
const TOKEN_EOF = "eof"

var operators = map[string]string{
	"+":  TOKEN_ADD,
	"-":  TOKEN_SUB,
	"/":  TOKEN_DIV,
	"//": TOKEN_FLOORDIV,
	"*":  TOKEN_MUL,
	"%":  TOKEN_MOD,
	"**": TOKEN_POW,
	"~":  TOKEN_TILDE,
	"[":  TOKEN_LBRACKET,
	"]":  TOKEN_RBRACKET,
	"(":  TOKEN_LPAREN,
	")":  TOKEN_RPAREN,
	"{":  TOKEN_LBRACE,
	"}":  TOKEN_RBRACE,
	"==": TOKEN_EQ,
	"!=": TOKEN_NE,
	">":  TOKEN_GT,
	">=": TOKEN_GTEQ,
	"<":  TOKEN_LT,
	"<=": TOKEN_LTEQ,
	"=":  TOKEN_ASSIGN,
	".":  TOKEN_DOT,
	":":  TOKEN_COLON,
	"|":  TOKEN_PIPE,
	",":  TOKEN_COMMA,
	";":  TOKEN_SEMICOLON,
}

var reverseOperators = rev(operators)

func rev(ops map[string]string) map[string]string {
	rev := make(map[string]string)
	for k, v := range ops {
		rev[v] = k
	}
	if len(rev) != len(ops) {
		_, _ = fmt.Fprintln(os.Stderr, "operators dropped")
		os.Exit(1)
	}
	return rev
}

var operatorRe = getOperatorRe(operators)

func getOperatorRe(ops map[string]string) *regexp.Regexp {
	els := maps.Keys(ops)
	sort.Slice(els, func(i int, j int) bool {
		return len(els[i]) > len(els[j])
	})
	for i, el := range els {
		els[i] = regexp.QuoteMeta(el)
	}
	pat := strings.Join(els, "|")
	return regexp.MustCompile(pat)
}

var ignoredTokens = set.FrozenFromElems(
	TOKEN_COMMENT_BEGIN,
	TOKEN_COMMENT,
	TOKEN_COMMENT_END,
	TOKEN_WHITESPACE,
	TOKEN_LINECOMMENT_BEGIN,
	TOKEN_LINECOMMENT_END,
	TOKEN_LINECOMMENT,
)

var ignoreIfEmpty = set.FrozenFromElems(
	TOKEN_WHITESPACE,
	TOKEN_DATA,
	TOKEN_COMMENT,
	TOKEN_LINECOMMENT,
)

func describeTokenType(tokenType string) string {
	if op, ok := reverseOperators[tokenType]; ok {
		return op
	}

	switch tokenType {
	case TOKEN_COMMENT_BEGIN:
		return "begin of comment"
	case TOKEN_COMMENT_END:
		return "end of comment"
	case TOKEN_COMMENT:
		return "comment"
	case TOKEN_LINECOMMENT:
		return "comment"
	case TOKEN_BLOCK_BEGIN:
		return "begin of statement block"
	case TOKEN_BLOCK_END:
		return "end of statement block"
	case TOKEN_VARIABLE_BEGIN:
		return "begin of print statement"
	case TOKEN_VARIABLE_END:
		return "end of print statement"
	case TOKEN_LINESTATEMENT_BEGIN:
		return "begin of line statement"
	case TOKEN_LINESTATEMENT_END:
		return "end of line statement"
	case TOKEN_DATA:
		return "template data / text"
	case TOKEN_EOF:
		return "end of template"
	default:
		return tokenType
	}
}

func DescribeToken(token Token) string {
	if token.type_ == TOKEN_NAME {
		return fmt.Sprint(token.value)
	}

	return describeTokenType(token.type_)
}

func DescribeTokenExpr(expr string) string {
	if strings.Contains(expr, ":") {
		res := strings.SplitN(expr, ":", 1)
		if res[0] == TOKEN_NAME {
			return res[1]
		}
		return describeTokenType(res[0])
	}
	return describeTokenType(expr)
}

type Token struct {
	lineno int
	type_  string
	value  any
}

func (t Token) String() string {
	return DescribeToken(t)
}

func (t Token) Test(expr string) bool {
	if t.type_ == expr {
		return true
	}
	if strings.Contains(expr, ":") {
		res := strings.SplitN(expr, ":", 1)
		return res[0] == t.type_ && res[1] == t.value
	}
	return false
}

func (t Token) TestAny(exprs ...string) bool {
	for _, expr := range exprs {
		if t.Test(expr) {
			return true
		}
	}
	return false
}
