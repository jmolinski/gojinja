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

type Stmt interface{}

type Expr interface {
}

type Output struct {
	Nodes []Node
	NodeCommon
}

type Extends struct {
	Template Expr
	NodeCommon
}

type MacroCall struct {
	Args     []Name
	Defaults []Expr
}

type Macro struct {
	Name string
	Body []Node
	MacroCall
	NodeCommon
}

type EvalContextModifier struct {
	Options []Keyword
	NodeCommon
}

type ScopedEvalContextModifier struct {
	Body []Node
	EvalContextModifier
}

type Scope struct {
	Body []Node
	NodeCommon
}

type FilterBlock struct {
	Body   []Node
	Filter Node
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

type BinOp struct {
	Left  Node
	Right Node
	Op    string // same as lexer.TokenAdd etc. + "and", "or"
	NodeCommon
}

type Concat struct {
	Nodes []Node
	NodeCommon
}

type UnaryOp struct {
	Node Node
	Op   string // same as lexer.TokenAdd etc. + "not"
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
var _ Node = &Extends{}
var _ Node = &Macro{}

var _ Node = &Scope{}
var _ Node = &FilterBlock{}
