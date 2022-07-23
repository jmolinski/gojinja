package identifier

import (
	"unicode"
	"unicode/utf8"
)

func IsIdentifier(s string) bool {
	// https://docs.python.org/3/reference/lexical_analysis.html
	cps, ok := toCodePoints(s)
	if !ok || len(cps) == 0 {
		return false
	}
	if !isXStart(cps[0]) {
		return false
	}
	for _, cp := range cps {
		if !isXContinue(cp) {
			return false
		}
	}
	return true
}

func isXStart(r rune) bool {
	return unicode.In(r, unicode.Lu, unicode.Ll, unicode.Lt, unicode.Lm, unicode.Lo, unicode.Nl) ||
		r == '_' ||
		isOtherIdStart(r)
}

func isXContinue(r rune) bool {
	return isXStart(r) || unicode.In(r, unicode.Mn, unicode.Mc, unicode.Nd, unicode.Pc) || isOtherIdContinue(r)
}

func isOtherIdContinue(r rune) bool {
	// https://www.unicode.org/Public/13.0.0/ucd/PropList.txt
	switch r {
	case 0xB7, 0x387, 0x1369, 0x1370, 0x1371, 0x19DA:
		return true
	default:
		return false
	}
}

func isOtherIdStart(r rune) bool {
	// https://www.unicode.org/Public/13.0.0/ucd/PropList.txt
	switch r {
	case 0x1885, 0x1888, 0x2118, 0x212E, 0x309B, 0x309C:
		return true
	default:
		return false
	}
}

func toCodePoints(s string) ([]rune, bool) {
	ret := make([]rune, 0)
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if r == utf8.RuneError {
			return nil, false
		}
		ret = append(ret, r)
		s = s[size:]
	}
	return ret, true
}
