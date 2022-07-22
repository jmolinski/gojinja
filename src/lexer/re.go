package lexer

import (
	"regexp"
	"sort"
)

var newlineRe = regexp.MustCompile(`(\r\n|\r|\n)`)
var whitespaceRe = regexp.MustCompile(`^\s+`)
var stringRe = regexp.MustCompile(`(?s)^('([^'\\]*(?:\\.[^'\\]*)*)'|"([^"\\]*(?:\\.[^"\\]*)*)")`)
var integerRe = regexp.MustCompile(`(?i)^(0b(_?[0-1])+|0o(_?[0-7])+|0x(_?[\da-f])+|[1-9](_?\d)*|0(_?0)*)`)

// Had to change original regex not to include lookbehind
// float_re = re.compile(
//    r"""
//    (?<!\.)  # doesn't start with a .
//    (\d+_)*\d+  # digits, possibly _ separated
//    (
//        (\.(\d+_)*\d+)?  # optional fractional part
//        e[+\-]?(\d+_)*\d+  # exponent part
//    |
//        \.(\d+_)*\d+  # required fractional part
//    )
//    """,
//    re.IGNORECASE | re.VERBOSE,
//)
var floatRe = regexp.MustCompile(`(?i)^(\d+_)*\d+((\.(\d+_)*\d+)?e[+\-]?(\d+_)*\d+|\.(\d+_)*\d+)`)

type rulePair struct {
	name    string
	pattern string
}

type ruleWithLength struct {
	len     int
	tok     string
	pattern string
}

func compileRules(env *EnvLexerInformation) []rulePair {
	rules := []ruleWithLength{
		{len(env.CommentStartString),
			TokenCommentBegin,
			regexp.QuoteMeta(env.CommentStartString),
		},
		{len(env.BlockStartString),
			TokenBlockBegin,
			regexp.QuoteMeta(env.BlockStartString),
		},
		{len(env.VariableStartString),
			TokenVariableBegin,
			regexp.QuoteMeta(env.VariableStartString),
		},
	}

	if env.LineStatementPrefix != nil {
		rules = append(rules, ruleWithLength{
			len(*env.LineStatementPrefix),
			TokenLinestatementBegin,
			`^[ \t\v]*` + regexp.QuoteMeta(*env.LineStatementPrefix),
		})
	}

	if env.LineCommentPrefix != nil {
		rules = append(rules, ruleWithLength{
			len(*env.LineCommentPrefix),
			TokenLinecommentBegin,
			`(?:^|(?<=\S))[^\S\r\n]*` + regexp.QuoteMeta(*env.LineCommentPrefix),
		})
	}

	sort.Slice(rules, func(i int, j int) bool {
		r1 := rules[i]
		r2 := rules[j]
		if r1.len == r2.len {
			if r1.tok == r2.tok {
				return r1.pattern > r2.pattern
			}
			return r1.tok > r2.tok
		}
		return r1.len > r2.len
	})

	ret := make([]rulePair, 0, len(rules))
	for _, r := range rules {
		ret = append(ret, rulePair{r.tok, r.pattern})
	}

	return ret
}
