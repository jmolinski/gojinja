package nodes

import "github.com/gojinja/gojinja/src/utils/slices"

type Node interface {
	GetLineno() int
	SetCtx(ctx string)
}

type ExprWithName interface {
	GetName() string
	Expr
}

type NodeCommon struct {
	Lineno int
}

func (n *NodeCommon) GetLineno() int {
	return n.Lineno
}

type Template struct {
	Body []Node
	NodeCommon
}

func (t *Template) SetCtx(ctx string) {
	for _, n := range t.Body {
		n.SetCtx(ctx)
	}
}

type Stmt interface{}

type Expr interface {
	CanAssign() bool
	Node
}

type Output struct {
	Nodes []Node
	NodeCommon
}

func (o *Output) SetCtx(ctx string) {
	for _, n := range o.Nodes {
		n.SetCtx(ctx)
	}
}

type Extends struct {
	Template Expr
	NodeCommon
}

func (e *Extends) SetCtx(ctx string) {
	e.Template.SetCtx(ctx)
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

func (m *Macro) SetCtx(ctx string) {
	for _, n := range m.Body {
		n.SetCtx(ctx)
	}
	for _, n := range m.Args {
		n.SetCtx(ctx)
	}
	for _, n := range m.Defaults {
		n.SetCtx(ctx)
	}
}

type EvalContextModifier struct {
	Options []Keyword
	NodeCommon
}

func (e *EvalContextModifier) SetCtx(ctx string) {
	for _, n := range e.Options {
		n.SetCtx(ctx)
	}
}

type ScopedEvalContextModifier struct {
	Body []Node
	EvalContextModifier
}

func (s *ScopedEvalContextModifier) SetCtx(ctx string) {
	for _, n := range s.Body {
		n.SetCtx(ctx)
	}
}

type Scope struct {
	Body []Node
	NodeCommon
}

func (s *Scope) SetCtx(ctx string) {
	for _, n := range s.Body {
		n.SetCtx(ctx)
	}
}

type FilterBlock struct {
	Body   []Node
	Filter Node
	NodeCommon
}

func (f *FilterBlock) SetCtx(ctx string) {
	for _, n := range f.Body {
		n.SetCtx(ctx)
	}
	f.Filter.SetCtx(ctx)
}

// TemplateData represents a constant template string.
type TemplateData struct {
	Data string
	NodeCommon
}

func (t *TemplateData) SetCtx(string) {}

type Tuple struct {
	Items []Node
	Ctx   string
	NodeCommon
}

func (t *Tuple) SetCtx(ctx string) {
	for _, n := range t.Items {
		n.SetCtx(ctx)
	}
	t.Ctx = ctx
}

type Const struct {
	Value any
	NodeCommon
}

func (c *Const) SetCtx(string) {}

type Name struct {
	Name string
	Ctx  string
	NodeCommon
}

func (n *Name) SetCtx(ctx string) {
	n.Ctx = ctx
}

func (n Name) CanAssign() bool {
	return !slices.Contains([]string{"true", "false", "none", "True", "False", "None"}, n.Name)
}

func (n Name) GetName() string {
	return n.Name
}

type NSRef struct {
	Name string
	Attr string
	NodeCommon
}

func (n NSRef) SetCtx(string) {}

func (n NSRef) CanAssign() bool {
	return true
}

func (n NSRef) GetName() string {
	return n.Name
}

type CondExpr struct {
	Test  Node
	Expr1 Node
	Expr2 Node
	NodeCommon
}

func (c *CondExpr) SetCtx(ctx string) {
	c.Test.SetCtx(ctx)
	c.Expr1.SetCtx(ctx)
	c.Expr2.SetCtx(ctx)
}

type Operand struct {
	Op   string
	Expr Node
	NodeCommon
}

func (o *Operand) SetCtx(ctx string) {
	o.Expr.SetCtx(ctx)
}

type Compare struct {
	Expr Node
	Ops  []Operand
	NodeCommon
}

func (c *Compare) SetCtx(ctx string) {
	c.Expr.SetCtx(ctx)
	for _, n := range c.Ops {
		n.SetCtx(ctx)
	}
}

type BinOp struct {
	Left  Node
	Right Node
	Op    string // same as lexer.TokenAdd etc. + "and", "or"
	NodeCommon
}

func (b *BinOp) SetCtx(ctx string) {
	b.Left.SetCtx(ctx)
	b.Right.SetCtx(ctx)
}

type Concat struct {
	Nodes []Node
	NodeCommon
}

func (c *Concat) SetCtx(ctx string) {
	for _, n := range c.Nodes {
		n.SetCtx(ctx)
	}
}

type UnaryOp struct {
	Node Node
	Op   string // same as lexer.TokenAdd etc. + "not"
	NodeCommon
}

func (u *UnaryOp) SetCtx(ctx string) {
	u.Node.SetCtx(ctx)
}

type Getattr struct {
	Node Node
	Attr string
	Ctx  string
	NodeCommon
}

func (g *Getattr) SetCtx(ctx string) {
	g.Node.SetCtx(ctx)
}

type Getitem struct {
	Node Node
	Arg  Node
	Ctx  string
	NodeCommon
}

func (g *Getitem) SetCtx(ctx string) {
	g.Node.SetCtx(ctx)
}

type Slice struct {
	Start *Node
	Stop  *Node
	Step  *Node
	NodeCommon
}

func (s *Slice) SetCtx(ctx string) {
	if s.Start != nil {
		(*s.Start).SetCtx(ctx)
	}
	if s.Stop != nil {
		(*s.Stop).SetCtx(ctx)
	}
	if s.Step != nil {
		(*s.Step).SetCtx(ctx)
	}
}

type Call struct {
	Node      Node
	Args      []Node
	Kwargs    []Node
	DynArgs   *Node
	DynKwargs *Node
	NodeCommon
}

func (c *Call) SetCtx(ctx string) {
	c.Node.SetCtx(ctx)
	for _, n := range c.Args {
		n.SetCtx(ctx)
	}
	for _, n := range c.Kwargs {
		n.SetCtx(ctx)
	}
	if c.DynArgs != nil {
		(*c.DynArgs).SetCtx(ctx)
	}
	if c.DynKwargs != nil {
		(*c.DynKwargs).SetCtx(ctx)
	}
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

func (f *Filter) SetCtx(ctx string) {
	f.Node.SetCtx(ctx)
	for _, n := range f.Args {
		n.SetCtx(ctx)
	}
	for _, n := range f.Kwargs {
		n.SetCtx(ctx)
	}
	if f.DynArgs != nil {
		(*f.DynArgs).SetCtx(ctx)
	}
	if f.DynKwargs != nil {
		(*f.DynKwargs).SetCtx(ctx)
	}
}

type Keyword struct {
	Key   string
	Value Node
	NodeCommon
}

func (k *Keyword) SetCtx(ctx string) {
	k.Value.SetCtx(ctx)
}

type If struct {
	Test Node
	Body []Node
	Elif []If
	Else []Node
	NodeCommon
}

func (i *If) SetCtx(ctx string) {
	i.Test.SetCtx(ctx)
	for _, n := range i.Body {
		n.SetCtx(ctx)
	}
	for _, n := range i.Else {
		n.SetCtx(ctx)
	}
	for _, n := range i.Elif {
		n.SetCtx(ctx)
	}
}

// Assert all types of nodes implement Node interface.
var _ Node = &Template{}
var _ Stmt = &Output{}
var _ Node = &TemplateData{}
var _ Node = &Tuple{}
var _ Node = &Const{}
var _ Node = &Name{}
var _ Node = &NSRef{}
var _ Node = &CondExpr{}
var _ Node = &Extends{}
var _ Node = &Macro{}
var _ Node = &Scope{}
var _ Node = &FilterBlock{}
var _ Node = &Keyword{}
var _ Node = &BinOp{}
var _ Node = &UnaryOp{}
var _ Node = &Output{}
var _ Node = &Compare{}
var _ Node = &Concat{}
var _ Node = &Operand{}
var _ Node = &Getattr{}
var _ Node = &Getitem{}
var _ Node = &Slice{}
var _ Node = &Call{}
var _ Node = &Filter{}
var _ Node = &If{}
var _ Node = &ScopedEvalContextModifier{}
var _ Node = &EvalContextModifier{}

var _ ExprWithName = &Name{}
var _ ExprWithName = &NSRef{}
