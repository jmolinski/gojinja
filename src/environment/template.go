package environment

type Class struct{}

type Template struct{}

type ITemplate interface{}

type UpToDate = func() bool

func (Class) FromSource(env Environment, source string, filename *string, globals map[string]any, upToData UpToDate) (ITemplate, error) {
	panic("TODO implemented")
}
