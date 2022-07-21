package encoding

func Encode(b string, encoding string) ([]byte, error) {
	// TODO support other formats that utf-8
	return []byte(b), nil
}

func Decode(b []byte, encoding string) (string, error) {
	// TODO support other formats that utf-8
	return string(b), nil
}
