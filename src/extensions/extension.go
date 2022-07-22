package extensions

import "github.com/gojinja/gojinja/src/nodes"

type IParser interface {
	Parse() (*nodes.Template, error)
}

type IExtension interface {
	Tags() []string
	Parse(p IParser) ([]nodes.Node, error)
}
