package filter

import (
	"fmt"
	"strconv"
	"strings"
)

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokString
	tokNumber
	tokPunct // ( ) ,
	tokOp    // = <> < <= > >=
)

type token struct {
	kind tokKind
	text string
	up   string // upper-cased text (for keyword/ident matching)
}

type lexer struct {
	src string
	pos int
}

func newLexer(s string) *lexer { return &lexer{src: s} }

var spatialOps = map[string]bool{
	"INTERSECTS": true, "WITHIN": true, "CONTAINS": true, "DISJOINT": true,
	"TOUCHES": true, "CROSSES": true, "OVERLAPS": true, "EQUALS": true,
}

var geomKeywords = map[string]bool{
	"POINT": true, "LINESTRING": true, "POLYGON": true, "MULTIPOINT": true,
	"MULTILINESTRING": true, "MULTIPOLYGON": true, "GEOMETRYCOLLECTION": true,
}

func (l *lexer) next() (token, error) {
	for l.pos < len(l.src) && (l.src[l.pos] == ' ' || l.src[l.pos] == '\t' ||
		l.src[l.pos] == '\n' || l.src[l.pos] == '\r') {
		l.pos++
	}
	if l.pos >= len(l.src) {
		return token{kind: tokEOF}, nil
	}
	c := l.src[l.pos]
	switch {
	case c == '(' || c == ')' || c == ',':
		l.pos++
		return token{kind: tokPunct, text: string(c)}, nil
	case c == '=':
		l.pos++
		return token{kind: tokOp, text: "="}, nil
	case c == '<':
		l.pos++
		if l.pos < len(l.src) && l.src[l.pos] == '>' {
			l.pos++
			return token{kind: tokOp, text: "<>"}, nil
		}
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.pos++
			return token{kind: tokOp, text: "<="}, nil
		}
		return token{kind: tokOp, text: "<"}, nil
	case c == '>':
		l.pos++
		if l.pos < len(l.src) && l.src[l.pos] == '=' {
			l.pos++
			return token{kind: tokOp, text: ">="}, nil
		}
		return token{kind: tokOp, text: ">"}, nil
	case c == '\'':
		return l.lexString()
	case c >= '0' && c <= '9' || c == '-' || c == '+' || c == '.':
		return l.lexNumber()
	case isIdentStart(c):
		return l.lexIdent()
	}
	return token{}, fmt.Errorf("unexpected character %q", c)
}

func (l *lexer) lexString() (token, error) {
	l.pos++ // opening quote
	var b strings.Builder
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\'' {
			if l.pos+1 < len(l.src) && l.src[l.pos+1] == '\'' {
				b.WriteByte('\'')
				l.pos += 2
				continue
			}
			l.pos++
			return token{kind: tokString, text: b.String()}, nil
		}
		b.WriteByte(c)
		l.pos++
	}
	return token{}, fmt.Errorf("unterminated string literal")
}

func (l *lexer) lexNumber() (token, error) {
	start := l.pos
	if l.src[l.pos] == '-' || l.src[l.pos] == '+' {
		l.pos++
	}
	digits := false
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c >= '0' && c <= '9' {
			digits = true
			l.pos++
		} else if c == '.' || c == 'e' || c == 'E' || c == '-' || c == '+' {
			l.pos++
		} else {
			break
		}
	}
	if !digits {
		return token{}, fmt.Errorf("invalid number %q", l.src[start:l.pos])
	}
	return token{kind: tokNumber, text: l.src[start:l.pos]}, nil
}

func (l *lexer) lexIdent() (token, error) {
	start := l.pos
	for l.pos < len(l.src) && isIdentPart(l.src[l.pos]) {
		l.pos++
	}
	t := l.src[start:l.pos]
	return token{kind: tokIdent, text: t, up: strings.ToUpper(t)}, nil
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == ':' || c == '.'
}

// cqlParser is a recursive-descent parser over the lexer.
type cqlParser struct {
	lex *lexer
	tok token
}

func (p *cqlParser) advance() error {
	t, err := p.lex.next()
	if err != nil {
		return err
	}
	p.tok = t
	return nil
}

// ParseCQL parses an ECQL filter string into an Expr.
func ParseCQL(s string) (Expr, error) {
	p := &cqlParser{lex: newLexer(s)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	if p.tok.kind == tokEOF {
		return nil, fmt.Errorf("empty filter")
	}
	e, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.tok.kind != tokEOF {
		return nil, fmt.Errorf("unexpected %q at end of filter", p.tok.text)
	}
	return e, nil
}

func (p *cqlParser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.tok.kind == tokIdent && p.tok.up == "OR" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Logic{Op: "OR", Exprs: []Expr{left, right}}
	}
	return left, nil
}

func (p *cqlParser) parseAnd() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.tok.kind == tokIdent && p.tok.up == "AND" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = Logic{Op: "AND", Exprs: []Expr{left, right}}
	}
	return left, nil
}

func (p *cqlParser) parseUnary() (Expr, error) {
	if p.tok.kind == tokIdent && p.tok.up == "NOT" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		e, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return Not{Expr: e}, nil
	}
	return p.parsePrimary()
}

func (p *cqlParser) parsePrimary() (Expr, error) {
	if p.tok.kind == tokPunct && p.tok.text == "(" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		e, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.tok.kind != tokPunct || p.tok.text != ")" {
			return nil, fmt.Errorf("expected )")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return e, nil
	}
	if p.tok.kind != tokIdent {
		return nil, fmt.Errorf("expected identifier, got %q", p.tok.text)
	}
	up := p.tok.up
	if up == "INCLUDE" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return IncludeAll{}, nil
	}
	if up == "EXCLUDE" {
		if err := p.advance(); err != nil {
			return nil, err
		}
		return Not{Expr: IncludeAll{}}, nil
	}
	if up == "BBOX" {
		return p.parseBBox()
	}
	if spatialOps[up] {
		return p.parseSpatial(up)
	}
	if up == "DWITHIN" || up == "BEYOND" {
		return p.parseDWithin(up == "BEYOND")
	}
	return p.parsePredicate()
}

func stripPrefix(name string) string {
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return name[i+1:]
	}
	return name
}

func (p *cqlParser) parsePredicate() (Expr, error) {
	propName := stripPrefix(p.tok.text)
	prop := Property{Name: propName}
	if err := p.advance(); err != nil {
		return nil, err
	}

	// comparison operators
	if p.tok.kind == tokOp {
		op := p.tok.text
		if err := p.advance(); err != nil {
			return nil, err
		}
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return Compare{Op: op, Left: prop, Right: val}, nil
	}

	if p.tok.kind != tokIdent {
		return nil, fmt.Errorf("expected operator after %q", propName)
	}
	negate := false
	if p.tok.up == "NOT" {
		negate = true
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokIdent {
			return nil, fmt.Errorf("expected LIKE/IN/BETWEEN after NOT")
		}
	}
	switch p.tok.up {
	case "LIKE", "ILIKE":
		ci := p.tok.up == "ILIKE"
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokString {
			return nil, fmt.Errorf("LIKE requires a string pattern")
		}
		pat := p.tok.text
		if err := p.advance(); err != nil {
			return nil, err
		}
		return Like{Prop: prop, Pattern: pat, CaseInsensitive: ci, Negate: negate}, nil
	case "IN":
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokPunct || p.tok.text != "(" {
			return nil, fmt.Errorf("IN requires (")
		}
		var vals []Expr
		for {
			if err := p.advance(); err != nil {
				return nil, err
			}
			v, err := p.parseValue()
			if err != nil {
				return nil, err
			}
			vals = append(vals, v)
			if p.tok.kind == tokPunct && p.tok.text == "," {
				continue
			}
			break
		}
		if p.tok.kind != tokPunct || p.tok.text != ")" {
			return nil, fmt.Errorf("IN requires )")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return In{Prop: prop, Values: vals, Negate: negate}, nil
	case "BETWEEN":
		if err := p.advance(); err != nil {
			return nil, err
		}
		lo, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		if p.tok.kind != tokIdent || p.tok.up != "AND" {
			return nil, fmt.Errorf("BETWEEN requires AND")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		hi, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return Between{Prop: prop, Lo: lo, Hi: hi, Negate: negate}, nil
	case "IS":
		if negate {
			return nil, fmt.Errorf("unexpected NOT before IS")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		isNeg := false
		if p.tok.kind == tokIdent && p.tok.up == "NOT" {
			isNeg = true
			if err := p.advance(); err != nil {
				return nil, err
			}
		}
		if p.tok.kind != tokIdent || p.tok.up != "NULL" {
			return nil, fmt.Errorf("expected NULL")
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return IsNull{Prop: prop, Negate: isNeg}, nil
	}
	return nil, fmt.Errorf("unexpected token %q", p.tok.text)
}

func (p *cqlParser) parseValue() (Expr, error) {
	switch p.tok.kind {
	case tokString:
		v := p.tok.text
		if err := p.advance(); err != nil {
			return nil, err
		}
		return Literal{Value: v}, nil
	case tokNumber:
		f, err := strconv.ParseFloat(p.tok.text, 64)
		if err != nil {
			return nil, err
		}
		if err := p.advance(); err != nil {
			return nil, err
		}
		return Literal{Value: f}, nil
	case tokIdent:
		up := p.tok.up
		if up == "TRUE" || up == "FALSE" {
			if err := p.advance(); err != nil {
				return nil, err
			}
			return Literal{Value: up == "TRUE"}, nil
		}
		name := stripPrefix(p.tok.text)
		if err := p.advance(); err != nil {
			return nil, err
		}
		return Property{Name: name}, nil
	}
	return nil, fmt.Errorf("expected value, got %q", p.tok.text)
}

func (p *cqlParser) expect(text string) error {
	if p.tok.text != text {
		return fmt.Errorf("expected %q, got %q", text, p.tok.text)
	}
	return p.advance()
}

func (p *cqlParser) parseNum() (float64, error) {
	if p.tok.kind != tokNumber {
		return 0, fmt.Errorf("expected number, got %q", p.tok.text)
	}
	f, err := strconv.ParseFloat(p.tok.text, 64)
	if err != nil {
		return 0, err
	}
	return f, p.advance()
}

func (p *cqlParser) parseBBox() (Expr, error) {
	if err := p.advance(); err != nil { // consume BBOX
		return nil, err
	}
	if err := p.expect("("); err != nil {
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, fmt.Errorf("BBOX requires a property")
	}
	b := BBox{Prop: stripPrefix(p.tok.text)}
	if err := p.advance(); err != nil {
		return nil, err
	}
	nums := []*float64{&b.MinX, &b.MinY, &b.MaxX, &b.MaxY}
	for _, np := range nums {
		if err := p.expect(","); err != nil {
			return nil, err
		}
		v, err := p.parseNum()
		if err != nil {
			return nil, err
		}
		*np = v
	}
	if p.tok.kind == tokPunct && p.tok.text == "," {
		if err := p.advance(); err != nil {
			return nil, err
		}
		if p.tok.kind != tokString {
			return nil, fmt.Errorf("BBOX srs must be a string")
		}
		b.SRS = p.tok.text
		if err := p.advance(); err != nil {
			return nil, err
		}
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	return b, nil
}

// parseWKT captures raw WKT text from a geometry keyword through balanced parens.
func (p *cqlParser) parseWKT() (string, error) {
	if p.tok.kind != tokIdent || !geomKeywords[p.tok.up] {
		return "", fmt.Errorf("expected WKT geometry, got %q", p.tok.text)
	}
	var b strings.Builder
	b.WriteString(p.tok.up)
	if err := p.advance(); err != nil {
		return "", err
	}
	if p.tok.kind != tokPunct || p.tok.text != "(" {
		return "", fmt.Errorf("WKT requires (")
	}
	depth := 0
	b.WriteString("(")
	depth++
	if err := p.advance(); err != nil {
		return "", err
	}
	first := true
	for depth > 0 {
		if p.tok.kind == tokEOF {
			return "", fmt.Errorf("unterminated WKT")
		}
		switch {
		case p.tok.kind == tokPunct && p.tok.text == "(":
			b.WriteString("(")
			depth++
			first = true
		case p.tok.kind == tokPunct && p.tok.text == ")":
			b.WriteString(")")
			depth--
		case p.tok.kind == tokPunct && p.tok.text == ",":
			b.WriteString(", ")
			first = true
		default:
			if !first {
				b.WriteString(" ")
			}
			b.WriteString(p.tok.text)
			first = false
		}
		if depth == 0 {
			break
		}
		if err := p.advance(); err != nil {
			return "", err
		}
	}
	if err := p.advance(); err != nil { // consume final )
		return "", err
	}
	return b.String(), nil
}

func (p *cqlParser) parseSpatial(op string) (Expr, error) {
	if err := p.advance(); err != nil { // consume op keyword
		return nil, err
	}
	if err := p.expect("("); err != nil {
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, fmt.Errorf("%s requires a property", op)
	}
	prop := stripPrefix(p.tok.text)
	if err := p.advance(); err != nil {
		return nil, err
	}
	if err := p.expect(","); err != nil {
		return nil, err
	}
	wkt, err := p.parseWKT()
	if err != nil {
		return nil, err
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	return Spatial{Op: op, Prop: prop, WKT: wkt}, nil
}

func (p *cqlParser) parseDWithin(beyond bool) (Expr, error) {
	if err := p.advance(); err != nil { // consume DWITHIN/BEYOND
		return nil, err
	}
	if err := p.expect("("); err != nil {
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, fmt.Errorf("DWITHIN requires a property")
	}
	prop := stripPrefix(p.tok.text)
	if err := p.advance(); err != nil {
		return nil, err
	}
	if err := p.expect(","); err != nil {
		return nil, err
	}
	wkt, err := p.parseWKT()
	if err != nil {
		return nil, err
	}
	if err := p.expect(","); err != nil {
		return nil, err
	}
	dist, err := p.parseNum()
	if err != nil {
		return nil, err
	}
	if err := p.expect(","); err != nil {
		return nil, err
	}
	if p.tok.kind != tokIdent {
		return nil, fmt.Errorf("DWITHIN requires distance units")
	}
	units := p.tok.text
	if err := p.advance(); err != nil {
		return nil, err
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	return DWithin{Prop: prop, WKT: wkt, Distance: dist, Units: units, Beyond: beyond}, nil
}
