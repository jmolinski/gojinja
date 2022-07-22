package filters

type Filter func(args []any, kwargs map[string]any) any

var Default = map[string]Filter{
	// TODO fill
}
