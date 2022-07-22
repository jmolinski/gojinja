package nodes

type Node interface {
	GetLineno() int
}

type NodeCommon struct {
	Lineno int
}

func (n *NodeCommon) GetLineno() int {
	return n.Lineno
}

var _ Node = &NodeCommon{}

type Template struct {
	Body []Node
	NodeCommon
}

type Stmt interface {
}

type Expr interface {
}

type Output struct {
	Nodes []Node
	NodeCommon
}

// TemplateData represents a constant template string.
type TemplateData struct {
	Data string
	NodeCommon
}

type Tuple struct {
	Items []Node
	Ctx   string
	NodeCommon
}

// Assert all types of nodes implement Node interface.
var _ Node = &Template{}
var _ Stmt = &Output{}
var _ Node = &TemplateData{}
var _ Node = &Tuple{}
