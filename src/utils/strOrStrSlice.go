package utils

type StrOrSlice interface {
	string | []string
}

func ToStrSlice[S StrOrSlice](s S) (sP []string) {
	switch v := any(s).(type) {
	case string:
		sP = append(sP, v)
	case []string:
		sP = v
	}
	return
}
