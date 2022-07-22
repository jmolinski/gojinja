package environment

type Class struct{}

type Template struct{}

type ITemplate interface {
	IsUpToDate() bool
	Globals() map[string]any
}

type UpToDate = func() bool

func (Class) FromSource(env *Environment, source string, filename *string, globals map[string]any, upToData UpToDate) (ITemplate, error) {
	panic("TODO implemented")
}
