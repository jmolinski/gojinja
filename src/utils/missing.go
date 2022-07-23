package utils

type missing struct{}

type Missing interface {
	Missing()
}

func (missing) Missing() {}

func GetMissing() Missing {
	return missing{}
}
