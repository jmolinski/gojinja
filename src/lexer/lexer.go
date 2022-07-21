package lexer

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
	"github.com/gojinja/gojinja/src/utils"
	"github.com/gojinja/gojinja/src/utils/stack"
	"github.com/hashicorp/golang-lru"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var lexerCache *lru.Cache

func init() {
	var err error
	lexerCache, err = lru.New(50)

	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "%w", err)
		os.Exit(1)
	}
}

func toStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toCacheKey(env *EnvLexerInformation) string {
	els := []string{
		env.BlockStartString,
		env.BlockEndString,
		env.VariableStartString,
		env.VariableEndString,
		env.CommentStartString,
		env.CommentEndString,
		toStr(env.LineStatementPrefix),
		toStr(env.LineCommentPrefix),
		strconv.FormatBool(env.TrimBlocks),
		strconv.FormatBool(env.LStripBlocks),
		env.NewlineSequence,
		strconv.FormatBool(env.KeepTrailingNewline),
	}

	return strings.Join(els, "^%&*$!*")
}

func GetLexer(env *EnvLexerInformation) *Lexer {
	key := toCacheKey(env)
	if v, ok := lexerCache.Get(key); ok {
		return v.(*Lexer)
	}
	l := New(env)
	lexerCache.Add(key, l)
	return l
}

type rule struct {
	pattern *regexp.Regexp
	tokens  any
	command *string
}

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

type Lexer struct {
	LStripBlocks        bool
	NewlineSequence     string
	KeepTrailingNewline bool
	Rules               map[string][]rule
}

func New(env *EnvLexerInformation) *Lexer {
	tagRules := []rule{
		{whitespaceRe, TOKEN_WHITESPACE, nil},
		{floatRe, TOKEN_FLOAT, nil},
		{integerRe, TOKEN_INTEGER, nil},
		{utils.NameRe, TOKEN_NAME, nil},
		{stringRe, TOKEN_STRING, nil},
		{operatorRe, TOKEN_OPERATOR, nil},
	}

	rootTagRules := compileRules(env)
	blockStartRe := regexp.QuoteMeta(env.BlockStartString)
	blockEndRe := regexp.QuoteMeta(env.BlockEndString)
	commentEndRe := regexp.QuoteMeta(env.CommentEndString)
	variableEndRe := regexp.QuoteMeta(env.VariableEndString)

	blockSuffixRe := ""
	if env.TrimBlocks {
		blockSuffixRe = "\\n?"
	}

	rootRawRe := fmt.Sprintf(`(?P<raw_begin>%s(\-|\+|)\s*raw\s*(?:\-%s\s*|%s))`, blockStartRe, blockEndRe, blockEndRe)
	rootPartsReArr := make([]string, 0, len(rootTagRules)+1)
	rootPartsReArr = append(rootPartsReArr, rootRawRe)
	for _, r := range rootTagRules {
		rootPartsReArr = append(rootPartsReArr, fmt.Sprintf(`(?P<%s>%s(\-|\+|))`, r.name, r.pattern))
	}
	rootPartsRe := strings.Join(rootPartsReArr, "|")
	popCmd := "#pop"
	byGrpCmd := "#bygroup"

	return &Lexer{
		LStripBlocks:        env.LStripBlocks,
		NewlineSequence:     env.NewlineSequence,
		KeepTrailingNewline: env.KeepTrailingNewline,
		Rules: map[string][]rule{
			"root": {
				{
					c(fmt.Sprintf(`(.*?)(?:%s)`, rootPartsRe)),
					OptionalLStrip{data: []string{TOKEN_DATA, byGrpCmd}},
					&byGrpCmd,
				}, // directives
				{c(".+"), TOKEN_DATA, nil}, // data
			},
			TOKEN_COMMENT_BEGIN: {
				{
					c(fmt.Sprintf(`(.*?)((?:\+%s|\-%s\s*|%s%s))`, commentEndRe, commentEndRe, commentEndRe, blockSuffixRe)),
					[]string{TOKEN_COMMENT, TOKEN_COMMENT_END},
					&popCmd,
				},
				{c(`(.)`), "" /* TODO */, nil},
			},
			TOKEN_BLOCK_BEGIN: append([]rule{
				{
					c(fmt.Sprintf(`(?:\+%s|\-%s\s*|%s%s)`, blockEndRe, blockEndRe, blockEndRe, blockSuffixRe)),
					TOKEN_BLOCK_END,
					&popCmd,
				},
			}, tagRules...),
			TOKEN_VARIABLE_BEGIN: append([]rule{
				{
					c(fmt.Sprintf(`\-%s\s*|%s`, variableEndRe, variableEndRe)),
					TOKEN_VARIABLE_END,
					&popCmd,
				},
			}, tagRules...),
			TOKEN_RAW_BEGIN: append([]rule{
				{
					c(fmt.Sprintf(`(.*?)((?:%s(\-|\+|))\s*endraw\s*(?:\+%s|\-%s\s*|%s%s))`, blockStartRe, blockEndRe, blockEndRe, blockEndRe, blockSuffixRe)),
					OptionalLStrip{data: []string{TOKEN_DATA, TOKEN_RAW_END}},
					&popCmd,
				},
			}, tagRules...),
			TOKEN_LINESTATEMENT_BEGIN: append([]rule{
				{c(`\s*(\n|$)`), TOKEN_LINECOMMENT_END, &popCmd},
			}, tagRules...),
			TOKEN_LINECOMMENT_BEGIN: {
				{
					c(`(.*?)()(?=\n|$)`),
					[]string{TOKEN_COMMENT, TOKEN_COMMENT_END},
					&popCmd,
				},
			},
		},
	}
}

func (l Lexer) normalizeNewlines(value string) string {
	return newlineRe.ReplaceAllString(value, l.NewlineSequence)
}

func (l *Lexer) Tokenize(source string, name *string, filename *string, state *string) (*TokenStream, error) {
	stream, err := l.Tokeniter(source, name, filename, state)
	if err != nil {
		return nil, err
	}
	wrapped, err := l.Wrap(stream, name, filename)
	if err != nil {
		return nil, err
	}
	return NewTokenStream(wrapped, name, filename), nil
}

type tokenRaw struct {
	lineno   int
	token    string
	valueStr string
}

type OptionalLStrip struct{ data []string }

func (l *Lexer) Wrap(stream []tokenRaw, name *string, filename *string) ([]Token, error) {
	ret := make([]Token, 0, len(stream))
	for _, raw := range stream {
		if ignoredTokens.Has(raw.token) {
			continue
		}

		var value any = raw.valueStr
		token := raw.token
		switch raw.token {
		case TOKEN_LINESTATEMENT_BEGIN:
			token = TOKEN_BLOCK_BEGIN
		case TOKEN_LINESTATEMENT_END:
			token = TOKEN_BLOCK_END
		case TOKEN_RAW_BEGIN, TOKEN_RAW_END:
			continue
		case TOKEN_DATA:
			value = l.normalizeNewlines(raw.valueStr)
		case "keyword":
			token = raw.valueStr
		case TOKEN_NAME:
			if !utils.IsIdentifier(raw.valueStr) {
				return nil, errors.TemplateSyntaxError("Invalid character in identifier", raw.lineno, name, filename)
			}
		case TOKEN_STRING:
			// TODO improve when we have encoding logic
			//# try to unescape string
			//try:
			//	value = (
			//		self._normalize_newlines(value_str[1:-1])
			//	.encode("ascii", "backslashreplace")
			//	.decode("unicode-escape")
			//	)
			//except Exception as e:
			//	msg = str(e).split(":")[-1].strip()
			//	raise TemplateSyntaxError(msg, lineno, name, filename) from e
			value = l.normalizeNewlines(raw.valueStr[1 : len(raw.valueStr)-1])
		case TOKEN_INTEGER:
			v, err := strconv.ParseInt(strings.Replace(raw.valueStr, "_", "", -1), 0, 64)
			if err != nil {
				return nil, err
			}
			value = v
		case TOKEN_FLOAT:
			// TODO change to `ast.literal_eval`
			v, err := strconv.ParseFloat(strings.Replace(raw.valueStr, "_", "", -1), 64)
			if err != nil {
				return nil, err
			}
			value = v
		case TOKEN_OPERATOR:
			token = operators[raw.valueStr]
		}
		ret = append(ret, Token{raw.lineno, token, value})
	}
	return ret, nil
}

func (l *Lexer) Tokeniter(source string, name *string, filename *string, state *string) (ret []tokenRaw, err error) {
	lines := newlineRe.Split(source, -1)
	if !l.KeepTrailingNewline && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	source = strings.Join(lines, "\n")
	pos := 0
	lineno := 1
	st := stack.New[string]()

	if state != nil && *state != "root" {
		if *state != "variable" && *state != "block" {
			_, _ = fmt.Fprintf(os.Stderr, "invalid state")
			os.Exit(1)
		}
		st.Push(*state + "_begin")
	}

	stateTokens := l.Rules[*st.Peek()]
	sourceLength := len(source)
	balancingStack := stack.New[string]()
	newlinesStripped := 0
	lineStarting := true
	for {
		broke := false
		// tokenizer loop
		for _, sToks := range stateTokens {
			// if no match we try again with the next rule
			groups := sToks.pattern.FindStringSubmatch(source[:pos])
			if groups == nil {
				continue
			}

			// we only match blocks and variables if braces / parentheses
			// are balanced. continue parsing with the lower rule which
			// is the operator rule. do this only if the end tags look
			// like operators
			if balancingStack.Peek() != nil &&
				(sToks.tokens == TOKEN_VARIABLE_END ||
					sToks.tokens == TOKEN_BLOCK_END ||
					sToks.tokens == TOKEN_LINESTATEMENT_END) {
				continue
			}

			// tuples support more options
			if _, ok := sToks.tokens.(OptionalLStrip); ok {
				// Rule supports lstrip. Match will look like
				// text, block type, whitespace control, type, control, ...
				text := groups[0]
				// Skipping the text and first type, every other group is the
				// whitespace control for each type. One of the groups will be
				// -, +, or empty string instead of None.
				stripSign := ""
				for i := 2; i < len(groups); i += 2 {
					if groups[i] != "" {
						stripSign = groups[i]
						break
					}
				}
				if stripSign == "-" {
					// Strip all whitespace between the text and the tag.
					stripped := strings.TrimRightFunc(text, unicode.IsSpace)
					newlinesStripped = strings.Count(text[len(stripped):], "\n")
					groups = append([]string{stripped}, groups[1:]...)
				} else if stripSign != "+" && l.LStripBlocks {
					names := sToks.pattern.SubexpNames()
					variableExpression := false
					for i := 0; i < len(names); i++ {
						if names[i] == TOKEN_VARIABLE_BEGIN && groups[i] != "" {
							variableExpression = true
						}
					}
					if !variableExpression {
						// The start of text between the last newline and the tag.
						lPos := strings.LastIndex(text, "\n") + 1
						if lPos > 0 || lineStarting {
							// If there's only whitespace between the newline and the
							// tag, strip it.
							if fullmatch(whitespaceRe, text[lPos:]) {
								groups = append([]string{text[:lPos]}, groups[1:]...)
							}
						}
					}
				}
			}
			if toks, ok := toToks(sToks.tokens); ok {
				for idx, token := range toks {
					if token == "#bygroup" {
						// bygroup is a bit more complex, in that case we
						// yield for the current token the first named
						// group that matched
						names := sToks.pattern.SubexpNames()
						found := false
						for i := 0; i < len(names); i++ {
							if groups[i] != "" {
								ret = append(ret, tokenRaw{lineno, names[i], groups[i]})
								lineno += strings.Count(groups[i], "\n")
								found = true
								break
							}
						}
						if !found {
							return nil, fmt.Errorf("'%s' wanted to resolve the token dynamically but no group matched", sToks.pattern)
						}
					} else {
						// normal group
						data := groups[idx]
						if data != "" || !ignoredTokens.Has(token) {
							ret = append(ret, tokenRaw{lineno, token, data})
						}
						lineno += strings.Count(data, "\n") + newlinesStripped
						newlinesStripped = 0
					}
				}
			} else if failure, ok := sToks.tokens.(Failure); ok {
				return nil, failure.Error(lineno, filename)
			} else if toks, ok := sToks.tokens.(string); ok {
				// strings as token just are yielded as it.
				data := groups[0]
				// update brace / parentheses balance
				if toks == TOKEN_OPERATOR {
					switch data {
					case "{":
						balancingStack.Push("}")
					case "(":
						balancingStack.Push(")")
					case "[":
						balancingStack.Push("]")
					case "}", ")", "]":
						exOp := balancingStack.Pop()
						if exOp == nil {
							return nil, errors.TemplateSyntaxError(fmt.Sprintf("unexpected '%s'", data), lineno, name, filename)
						}
						if *exOp != data {
							return nil, errors.TemplateSyntaxError(fmt.Sprintf("unexpected '%s', expected '%s'", data, *exOp), lineno, name, filename)
						}
						// yield items
						if data != "" || !ignoreIfEmpty.Has(toks) {
							ret = append(ret, tokenRaw{lineno, toks, data})
						}
						lineno += strings.Count(data, "\n")
					}
				}
			} else {
				return nil, fmt.Errorf("unexpected type")
			}

			lineStarting = groups[0][len(groups[0])-1] == '\n'
			// fetch new position into new variable so that we can check
			// if there is a internal parsing error which would result
			// in an infinite loop
			idx := sToks.pattern.FindAllStringSubmatchIndex(source[pos:], -1)
			pos2 := idx[0][1]
			// handle state changes
			if sToks.command != nil {
				// remove the uppermost state
				if *sToks.command == "#pop" {
					st.Pop()
				} else if *sToks.command == "#bygroup" {
					// resolve the new state by group checking
					names := sToks.pattern.SubexpNames()
					found := false
					for i := 0; i < len(names); i++ {
						if groups[i] != "" {
							st.Push(names[i])
							found = true
							break
						}
					}
					if !found {
						return nil, fmt.Errorf("'%s' wanted to resolve the new state dynamically but no group matched", sToks.pattern)
					}
				} else {
					st.Push(*sToks.command)
				}

				stateTokens = l.Rules[*st.Peek()]
			} else if pos2 == pos {
				// we are still at the same position and no stack change.
				// this means a loop without break condition, avoid that and
				// raise error
				return nil, fmt.Errorf("'%s' yielded empty string without stack change", sToks.pattern)
			}
			// publish new function and start again
			pos = pos2
			broke = true
			break
		}
		if !broke {
			// if loop terminated without break we haven't found a single match
			// either we are at the end of the file or we have a problem
			if pos >= sourceLength {
				return
			}
			return nil, errors.TemplateSyntaxError(fmt.Sprintf("unexpected char '%s' at %d", string(source[pos]), pos), lineno, name, filename)
		}
	}
}

type Failure struct {
	msg string
}

func (f Failure) Error(lineno int, filename *string) error {
	// I do not undestand why filename is passed as name and not filename but that what jinja does.
	return errors.TemplateSyntaxError(f.msg, lineno, filename, nil)
}

func toToks(tokens any) ([]string, bool) {
	switch v := tokens.(type) {
	case []string:
		return v, true
	case OptionalLStrip:
		return v.data, true
	default:
		return nil, false
	}
}

func fullmatch(re *regexp.Regexp, text string) bool {
	l := len(text)
	for _, m := range re.FindAllString(text, -1) {
		if len(m) == l {
			return true
		}
	}
	return false
}

func c(x string) *regexp.Regexp {
	return regexp.MustCompile("(?ms)" + x)
}
