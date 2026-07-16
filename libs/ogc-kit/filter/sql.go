package filter

import (
	"fmt"
	"regexp"
	"strings"
)

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// quoteIdent validates and double-quotes a column name. Names that don't
// match the identifier pattern are rejected (injection defense).
func quoteIdent(name string) (string, error) {
	if !identRe.MatchString(name) {
		return "", fmt.Errorf("invalid identifier %q", name)
	}
	return `"` + name + `"`, nil
}

// argctr tracks the running bind-parameter index.
type argctr struct {
	n    int
	args []any
}

func (a *argctr) placeholder(v any) string {
	a.args = append(a.args, v)
	p := fmt.Sprintf("$%d", a.n)
	a.n++
	return p
}

// ToSQL compiles an Expr to a parameterized WHERE fragment for PostGIS.
// startArg is the first placeholder index (e.g. 1 -> $1).
func ToSQL(e Expr, startArg int) (string, []any, error) {
	ac := &argctr{n: startArg}
	sql, err := emit(e, ac)
	if err != nil {
		return "", nil, err
	}
	return sql, ac.args, nil
}

func litValue(e Expr) (any, error) {
	l, ok := e.(Literal)
	if !ok {
		return nil, fmt.Errorf("expected literal value")
	}
	return l.Value, nil
}

func emit(e Expr, ac *argctr) (string, error) {
	switch n := e.(type) {
	case IncludeAll:
		return "TRUE", nil
	case Property:
		return quoteIdent(n.Name)
	case Compare:
		left, err := emit(n.Left, ac)
		if err != nil {
			return "", err
		}
		v, err := litValue(n.Right)
		if err != nil {
			// property-to-property comparison
			right, rerr := emit(n.Right, ac)
			if rerr != nil {
				return "", err
			}
			return fmt.Sprintf("%s %s %s", left, n.Op, right), nil
		}
		return fmt.Sprintf("%s %s %s", left, n.Op, ac.placeholder(v)), nil
	case Logic:
		parts := make([]string, 0, len(n.Exprs))
		for _, sub := range n.Exprs {
			s, err := emit(sub, ac)
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return "(" + strings.Join(parts, " "+n.Op+" ") + ")", nil
	case Not:
		s, err := emit(n.Expr, ac)
		if err != nil {
			return "", err
		}
		return "NOT (" + s + ")", nil
	case Like:
		col, err := emit(n.Prop, ac)
		if err != nil {
			return "", err
		}
		op := "LIKE"
		if n.CaseInsensitive {
			op = "ILIKE"
		}
		if n.Negate {
			op = "NOT " + op
		}
		return fmt.Sprintf("%s %s %s", col, op, ac.placeholder(n.Pattern)), nil
	case In:
		col, err := emit(n.Prop, ac)
		if err != nil {
			return "", err
		}
		phs := make([]string, 0, len(n.Values))
		for _, v := range n.Values {
			lv, err := litValue(v)
			if err != nil {
				return "", err
			}
			phs = append(phs, ac.placeholder(lv))
		}
		op := "IN"
		if n.Negate {
			op = "NOT IN"
		}
		return fmt.Sprintf("%s %s (%s)", col, op, strings.Join(phs, ", ")), nil
	case Between:
		col, err := emit(n.Prop, ac)
		if err != nil {
			return "", err
		}
		lo, err := litValue(n.Lo)
		if err != nil {
			return "", err
		}
		hi, err := litValue(n.Hi)
		if err != nil {
			return "", err
		}
		op := "BETWEEN"
		if n.Negate {
			op = "NOT BETWEEN"
		}
		return fmt.Sprintf("%s %s %s AND %s", col, op,
			ac.placeholder(lo), ac.placeholder(hi)), nil
	case IsNull:
		col, err := emit(n.Prop, ac)
		if err != nil {
			return "", err
		}
		if n.Negate {
			return col + " IS NOT NULL", nil
		}
		return col + " IS NULL", nil
	case BBox:
		col, err := quoteIdent(n.Prop)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s && ST_MakeEnvelope(%s, %s, %s, %s)", col,
			ac.placeholder(n.MinX), ac.placeholder(n.MinY),
			ac.placeholder(n.MaxX), ac.placeholder(n.MaxY)), nil
	case Spatial:
		col, err := quoteIdent(n.Prop)
		if err != nil {
			return "", err
		}
		fn := "ST_" + strings.Title(strings.ToLower(n.Op)) //nolint:staticcheck
		return fmt.Sprintf("%s(%s, ST_GeomFromText(%s))", fn, col,
			ac.placeholder(n.WKT)), nil
	case DWithin:
		col, err := quoteIdent(n.Prop)
		if err != nil {
			return "", err
		}
		expr := fmt.Sprintf("ST_DWithin(%s, ST_GeomFromText(%s), %s)", col,
			ac.placeholder(n.WKT), ac.placeholder(n.Distance))
		if n.Beyond {
			return "NOT " + expr, nil
		}
		return expr, nil
	}
	return "", fmt.Errorf("cannot compile expression %T", e)
}
