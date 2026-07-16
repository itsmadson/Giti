// Package filter parses OGC filters (CQL text and Filter XML) into a shared
// AST and compiles them to parameterized SQL.
package filter

type Expr interface{ isExpr() }

type Property struct{ Name string }
type Literal struct{ Value any }
type Compare struct {
	Op          string
	Left, Right Expr
}
type Logic struct {
	Op    string
	Exprs []Expr
}
type Not struct{ Expr Expr }
type Like struct {
	Prop            Expr
	Pattern         string
	CaseInsensitive bool
	Negate          bool
}
type In struct {
	Prop   Expr
	Values []Expr
	Negate bool
}
type Between struct {
	Prop, Lo, Hi Expr
	Negate       bool
}
type IsNull struct {
	Prop   Expr
	Negate bool
}
type BBox struct {
	Prop                   string
	MinX, MinY, MaxX, MaxY float64
	SRS                    string
}
type Spatial struct {
	Op   string
	Prop string
	WKT  string
}
type DWithin struct {
	Prop     string
	WKT      string
	Distance float64
	Units    string
	Beyond   bool
}
type IncludeAll struct{}

func (Property) isExpr()   {}
func (Literal) isExpr()    {}
func (Compare) isExpr()    {}
func (Logic) isExpr()      {}
func (Not) isExpr()        {}
func (Like) isExpr()       {}
func (In) isExpr()         {}
func (Between) isExpr()    {}
func (IsNull) isExpr()     {}
func (BBox) isExpr()       {}
func (Spatial) isExpr()    {}
func (DWithin) isExpr()    {}
func (IncludeAll) isExpr() {}
