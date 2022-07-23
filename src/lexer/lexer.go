package lexer

import (
	"fmt"
	"github.com/gojinja/gojinja/src/errors"
	"github.com/gojinja/gojinja/src/utils/identifier"
	"github.com/gojinja/gojinja/src/utils/stack"
	"github.com/hashicorp/golang-lru"
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
		panic(err)
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

// Lexer is a struct that implements a lexer for a given environment. Automatically
// created by the environment class, usually you don't have to do that.
//
// Note that the lexer is not automatically bound to an environment.
// Multiple environments can share the same lexer.
type Lexer struct {
	lStripBlocks        bool
	newlineSequence     string
	keepTrailingNewline bool
	rules               map[string][]rule
}

func New(env *EnvLexerInformation) *Lexer {
	tagRules := []rule{
		{whitespaceRe, TokenWhitespace, nil},
		{floatRe, TokenFloat, nil},
		{integerRe, TokenInteger, nil},
		{identifier.NameRe, TokenName, nil},
		{stringRe, TokenString, nil},
		{operatorRe, TokenOperator, nil},
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
		lStripBlocks:        env.LStripBlocks,
		newlineSequence:     env.NewlineSequence,
		keepTrailingNewline: env.KeepTrailingNewline,
		rules: map[string][]rule{
			"root": {
				{
					c(fmt.Sprintf(`^(.*?)(?:%s)`, rootPartsRe)),
					OptionalLStrip{data: []string{TokenData, byGrpCmd}},
					&byGrpCmd,
				}, // directives
				{c("^.+"), TokenData, nil}, // data
			},
			TokenCommentBegin: {
				{
					c(fmt.Sprintf(`^(.*?)((?:\+%s|\-%s\s*|%s%s))`, commentEndRe, commentEndRe, commentEndRe, blockSuffixRe)),
					[]string{TokenComment, TokenCommentEnd},
					&popCmd,
				},
				{c(`^(.)`), Failure{"Missing end of comment tag"}, nil},
			},
			TokenBlockBegin: append([]rule{
				{
					c(fmt.Sprintf(`^(?:\+%s|\-%s\s*|%s%s)`, blockEndRe, blockEndRe, blockEndRe, blockSuffixRe)),
					TokenBlockEnd,
					&popCmd,
				},
			}, tagRules...),
			TokenVariableBegin: append([]rule{
				{
					c(fmt.Sprintf(`^\-%s\s*|^%s`, variableEndRe, variableEndRe)),
					TokenVariableEnd,
					&popCmd,
				},
			}, tagRules...),
			TokenRawBegin: append([]rule{
				{
					c(fmt.Sprintf(`^(.*?)((?:%s(\-|\+|))\s*endraw\s*(?:\+%s|\-%s\s*|%s%s))`, blockStartRe, blockEndRe, blockEndRe, blockEndRe, blockSuffixRe)),
					OptionalLStrip{data: []string{TokenData, TokenRawEnd}},
					&popCmd,
				},
			}, tagRules...),
			TokenLinestatementBegin: append([]rule{
				{c(`^\s*(\n|$)`), TokenLinestatementEnd, &popCmd},
			}, tagRules...),
			TokenLinecommentBegin: {
				{
					c(`^(.*?)()(?:\n|$)`),
					[]string{TokenLinecomment, TokenLinecommentEnd},
					&popCmd,
				},
			},
		},
	}
}

func (l Lexer) normalizeNewlines(value string) string {
	return newlineRe.ReplaceAllString(value, l.newlineSequence)
}

// Tokenize calls Tokeniter and wraps it using Wrap into a token stream.
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

// OptionalLStrip is used for marking a point in the state that can have lstrip applied.
type OptionalLStrip struct{ data []string }

// Wrap is called with the stream as returned by `tokenize` and wraps
// every token in a `Token` and converts the value.
func (l *Lexer) Wrap(stream []tokenRaw, name *string, filename *string) ([]Token, error) {
	ret := make([]Token, 0, len(stream))
	for _, raw := range stream {
		if ignoredTokens.Has(raw.token) {
			continue
		}

		var value any = raw.valueStr
		token := raw.token
		switch raw.token {
		case TokenLinestatementBegin:
			token = TokenBlockBegin
		case TokenLinestatementEnd:
			token = TokenBlockEnd
		case TokenRawBegin, TokenRawEnd:
			continue
		case TokenData:
			value = l.normalizeNewlines(raw.valueStr)
		case "keyword":
			token = raw.valueStr
		case TokenName:
			if !identifier.IsIdentifier(raw.valueStr) {
				return nil, errors.TemplateSyntaxError("Invalid character in identifier", raw.lineno, name, filename)
			}
		case TokenString:
			value = unescapeString(l.normalizeNewlines(raw.valueStr[1 : len(raw.valueStr)-1]))
		case TokenInteger:
			v, err := strconv.ParseInt(strings.Replace(raw.valueStr, "_", "", -1), 0, 64)
			if err != nil {
				return nil, err
			}
			value = v
		case TokenFloat:
			// TODO change to `ast.literal_eval`
			v, err := strconv.ParseFloat(strings.Replace(raw.valueStr, "_", "", -1), 64)
			if err != nil {
				return nil, err
			}
			value = v
		case TokenOperator:
			token = operators[raw.valueStr]
		}
		ret = append(ret, Token{raw.lineno, token, value})
	}
	return ret, nil
}

// Tokeniter tokenizes the text and returns the tokens.
// Use this method if you just want to tokenize a template.
func (l *Lexer) Tokeniter(source string, name *string, filename *string, state *string) (ret []tokenRaw, err error) {
	lines := newlineRe.Split(source, -1)
	if !l.keepTrailingNewline && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	source = strings.Join(lines, "\n")
	pos := 0
	lineno := 1
	st := stack.New[string]()
	st.Push("root")

	if state != nil && *state != "root" {
		if *state != "variable" && *state != "block" {
			return nil, fmt.Errorf("invalid state")
		}
		st.Push(*state + "_begin")
	}
	stateTokens := l.rules[*st.Peek()]
	sourceLength := len(source)
	balancingStack := stack.New[string]()
	newlinesStripped := 0
	lineStarting := true

	broke := true
	for broke {
		broke = false
		// tokenizer loop
		for _, sToks := range stateTokens {
			// if no match we try again with the next rule
			groups := sToks.pattern.FindStringSubmatch(source[pos:])
			if len(groups) == 0 {
				continue
			}
			grp := groups[0]
			groups = groups[1:] // Remove first element as it's not in python counterpart.

			// we only match blocks and variables if braces / parentheses
			// are balanced. continue parsing with the lower rule which
			// is the operator rule. do this only if the end tags look
			// like operators
			if balancingStack.Peek() != nil &&
				(sToks.tokens == TokenVariableEnd ||
					sToks.tokens == TokenBlockEnd ||
					sToks.tokens == TokenLinestatementEnd) {
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
				} else if stripSign != "+" && l.lStripBlocks {
					names := sToks.pattern.SubexpNames()[1:]
					variableExpression := false
					for i := 0; i < len(names); i++ {
						if names[i] == TokenVariableBegin && groups[i] != "" {
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
						names := sToks.pattern.SubexpNames()[1:]
						found := false
						for i := 0; i < len(names); i++ {
							if names[i] != "" && groups[i] != "" {
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
						if data != "" || !ignoreIfEmpty.Has(token) {
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
				data := grp
				// update brace / parentheses balance
				if toks == TokenOperator {
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
					}
				}

				// yield items
				if data != "" || !ignoreIfEmpty.Has(toks) {
					ret = append(ret, tokenRaw{lineno, toks, data})
				}
				lineno += strings.Count(data, "\n")
			} else {
				return nil, fmt.Errorf("unexpected type")
			}

			lineStarting = grp[len(grp)-1] == '\n'
			// fetch new position into new variable so that we can check
			// if there is a internal parsing error which would result
			// in an infinite loop
			idx := sToks.pattern.FindAllStringSubmatchIndex(source[pos:], -1)
			pos2 := pos + idx[0][1]
			// handle state changes
			if sToks.command != nil {
				// remove the uppermost state
				if *sToks.command == "#pop" {
					st.Pop()
				} else if *sToks.command == "#bygroup" {
					// resolve the new state by group checking
					names := sToks.pattern.SubexpNames()[1:]
					found := false
					for i := 0; i < len(names); i++ {
						if names[i] != "" && groups[i] != "" {
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

				stateTokens = l.rules[*st.Peek()]
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
	}

	// if loop terminated without break we haven't found a single match
	// either we are at the end of the file or we have a problem
	if pos >= sourceLength {
		return
	}
	return nil, errors.TemplateSyntaxError(fmt.Sprintf("unexpected char '%s' at %d", string(source[pos]), pos), lineno, name, filename)
}

// Failure is used by the `Lexer` to specify known errors.
type Failure struct {
	msg string
}

func (f Failure) Error(lineno int, filename *string) error {
	// I do not undestand why filename is passed as name and not filename but that what jinja does.
	return errors.TemplateSyntaxError(f.msg, lineno, filename, filename)
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

func unescapeString(s string) string {
	backslashBefore := false
	var builder strings.Builder
	for _, c := range s {
		if c == '\\' {
			if backslashBefore {
				builder.WriteString("\\\\")
				backslashBefore = false
			} else {
				backslashBefore = true
			}
			continue
		}

		if c == '"' || c == '\'' {
			backslashBefore = false
		}

		if backslashBefore {
			builder.WriteRune('\\')
			backslashBefore = false
		}
		builder.WriteRune(c)
	}
	if backslashBefore {
		builder.WriteRune('\\')
	}
	return builder.String()
}

func c(x string) *regexp.Regexp {
	return regexp.MustCompile("(?ms)" + x)
}
