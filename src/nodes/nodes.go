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

type Const struct {
	Value any
	NodeCommon
}

type Name struct {
	Name string
	Ctx  string
	NodeCommon
}

type CondExpr struct {
	Test  Node
	Expr1 Node
	Expr2 Node
	NodeCommon
}

type Or struct {
	Left  Node
	Right Node
	NodeCommon
}

type And struct {
	Left  Node
	Right Node
	NodeCommon
}

type Not struct {
	Node Node
	NodeCommon
}

type Operand struct {
	Op   string
	Expr Node
	NodeCommon
}

type Compare struct {
	Expr Node
	Ops  []Operand
	NodeCommon
}

type Add struct {
	Left  Node
	Right Node
	NodeCommon
}

type Sub struct {
	Left  Node
	Right Node
	NodeCommon
}

type Mul struct {
	Left  Node
	Right Node
	NodeCommon
}

type Div struct {
	Left  Node
	Right Node
	NodeCommon
}

type FloorDiv struct {
	Left  Node
	Right Node
	NodeCommon
}

type Mod struct {
	Left  Node
	Right Node
	NodeCommon
}

type Pow struct {
	Left  Node
	Right Node
	NodeCommon
}

type Concat struct {
	Nodes []Node
	NodeCommon
}

type Neg struct {
	Node Node
	NodeCommon
}

type Pos struct {
	Node Node
	NodeCommon
}

type Getattr struct {
	Node Node
	Attr string
	Ctx  string
	NodeCommon
}

type Getitem struct {
	Node Node
	Arg  Node
	Ctx  string
	NodeCommon
}

type Slice struct {
	Start *Node
	Stop  *Node
	Step  *Node
	NodeCommon
}

type Call struct {
	Node      Node
	Args      []Node
	Kwargs    []Node
	DynArgs   *Node
	DynKwargs *Node
	NodeCommon
}

type Filter struct {
	Node      Node
	Name      string
	Args      []Node
	Kwargs    []Node
	DynArgs   *Node
	DynKwargs *Node
	NodeCommon
}

type Keyword struct {
	Key   string
	Value Node
	NodeCommon
}

type If struct {
	Test Node
	Body []Node
	Elif []If
	Else []Node
	NodeCommon
}

// Assert all types of nodes implement Node interface.
var _ Node = &Template{}
var _ Stmt = &Output{}
var _ Node = &TemplateData{}
var _ Node = &Tuple{}
var _ Node = &Const{}
var _ Node = &Name{}
var _ Node = &CondExpr{}
