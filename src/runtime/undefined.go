package runtime

type Undefined struct{}

type IUndefined interface{}

func NewUndefined(hint *string, obj any, name *string, exc func(msg string) error) Undefined {
	// TODO
	return Undefined{}
}
