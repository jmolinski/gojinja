package lexer

import "regexp"

var newlineRe = regexp.MustCompile(`(\r\n|\r|\n)`)
var whitespaceRe = regexp.MustCompile(`\s+`)
var stringRe = regexp.MustCompile(`(?s)('([^'\\]*(?:\\.[^'\\]*)*)'|"([^"\\]*(?:\\.[^"\\]*)*)")`)
var integerRe = regexp.MustCompile(`(?i)(0b(_?[0-1])+|0o(_?[0-7])+|0x(_?[\da-f])+|[1-9](_?\d)*|0(_?0)*)`)
var floatRe = regexp.MustCompile(`(?i)(?<!\.)(\d+_)*\d+((\.(\d+_)*\d+)?e[+\-]?(\d+_)*\d+|\.(\d+_)*\d+)`)

func countNewlines(value string) int {
	return len(newlineRe.FindAllString(value, -1))
}
