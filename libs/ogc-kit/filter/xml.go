package filter

import (
	"encoding/xml"
	"fmt"
	"strings"
)

// node is a decoded XML element (namespace-agnostic: keyed by local name).
type node struct {
	Name     string
	Attrs    map[string]string
	Children []*node
	Text     string
}

func decodeTree(body []byte) (*node, error) {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	var stack []*node
	var root *node
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("malformed filter xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &node{Name: t.Name.Local, Attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.Attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].Text += string(t)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("empty filter xml")
	}
	return root, nil
}

// elements returns child element nodes (skipping whitespace-only text).
func (n *node) elements() []*node { return n.Children }

var xmlCompareOps = map[string]string{
	"PropertyIsEqualTo":              "=",
	"PropertyIsNotEqualTo":           "<>",
	"PropertyIsLessThan":             "<",
	"PropertyIsLessThanOrEqualTo":    "<=",
	"PropertyIsGreaterThan":          ">",
	"PropertyIsGreaterThanOrEqualTo": ">=",
}

var xmlSpatialOps = map[string]string{
	"Intersects": "INTERSECTS", "Within": "WITHIN", "Contains": "CONTAINS",
	"Disjoint": "DISJOINT", "Touches": "TOUCHES", "Crosses": "CROSSES",
	"Overlaps": "OVERLAPS", "Equals": "EQUALS",
}

// ParseFilterXML parses OGC Filter 1.0/1.1 and 2.0 (fes) XML into an Expr.
func ParseFilterXML(body []byte) (Expr, error) {
	root, err := decodeTree(body)
	if err != nil {
		return nil, err
	}
	if root.Name != "Filter" {
		return nil, fmt.Errorf("root element is %q, want Filter", root.Name)
	}
	kids := root.elements()
	if len(kids) == 0 {
		return nil, fmt.Errorf("empty Filter")
	}
	return convert(kids[0])
}

func convert(n *node) (Expr, error) {
	if op, ok := xmlCompareOps[n.Name]; ok {
		return convertCompare(n, op)
	}
	if op, ok := xmlSpatialOps[n.Name]; ok {
		return convertSpatial(n, op)
	}
	switch n.Name {
	case "And", "Or":
		op := strings.ToUpper(n.Name)
		var exprs []Expr
		for _, c := range n.elements() {
			e, err := convert(c)
			if err != nil {
				return nil, err
			}
			exprs = append(exprs, e)
		}
		if len(exprs) < 2 {
			return nil, fmt.Errorf("%s requires 2+ operands", n.Name)
		}
		// left-fold to binary Logic nodes for AST stability
		acc := Logic{Op: op, Exprs: []Expr{exprs[0], exprs[1]}}
		for _, e := range exprs[2:] {
			acc = Logic{Op: op, Exprs: []Expr{acc, e}}
		}
		return acc, nil
	case "Not":
		kids := n.elements()
		if len(kids) != 1 {
			return nil, fmt.Errorf("Not requires 1 operand")
		}
		e, err := convert(kids[0])
		if err != nil {
			return nil, err
		}
		return Not{Expr: e}, nil
	case "PropertyIsLike":
		prop, err := propOf(n)
		if err != nil {
			return nil, err
		}
		return Like{Prop: prop, Pattern: literalText(n)}, nil
	case "PropertyIsNull":
		prop, err := propOf(n)
		if err != nil {
			return nil, err
		}
		return IsNull{Prop: prop}, nil
	case "PropertyIsBetween":
		prop, err := propOf(n)
		if err != nil {
			return nil, err
		}
		var lo, hi string
		for _, c := range n.elements() {
			switch c.Name {
			case "LowerBoundary":
				lo = firstLiteral(c)
			case "UpperBoundary":
				hi = firstLiteral(c)
			}
		}
		return Between{Prop: prop, Lo: Literal{Value: lo}, Hi: Literal{Value: hi}}, nil
	case "BBOX":
		return convertBBox(n)
	}
	return nil, fmt.Errorf("unsupported filter element %q", n.Name)
}

func propName(n *node) (string, bool) {
	if n.Name == "PropertyName" || n.Name == "ValueReference" {
		return stripPrefix(strings.TrimSpace(n.Text)), true
	}
	return "", false
}

func propOf(n *node) (Property, error) {
	for _, c := range n.elements() {
		if name, ok := propName(c); ok {
			return Property{Name: name}, nil
		}
	}
	return Property{}, fmt.Errorf("%s missing PropertyName", n.Name)
}

func literalText(n *node) string {
	for _, c := range n.elements() {
		if c.Name == "Literal" {
			return strings.TrimSpace(c.Text)
		}
	}
	return ""
}

func firstLiteral(n *node) string {
	for _, c := range n.elements() {
		if c.Name == "Literal" {
			return strings.TrimSpace(c.Text)
		}
	}
	return strings.TrimSpace(n.Text)
}

func convertCompare(n *node, op string) (Expr, error) {
	prop, err := propOf(n)
	if err != nil {
		return nil, err
	}
	return Compare{Op: op, Left: prop, Right: Literal{Value: literalText(n)}}, nil
}

func convertSpatial(n *node, op string) (Expr, error) {
	prop, err := propOf(n)
	if err != nil {
		return nil, err
	}
	for _, c := range n.elements() {
		if wkt, ok := gmlToWKT(c); ok {
			return Spatial{Op: op, Prop: prop.Name, WKT: wkt}, nil
		}
	}
	return nil, fmt.Errorf("%s missing geometry", n.Name)
}

func convertBBox(n *node) (Expr, error) {
	prop, err := propOf(n)
	if err != nil {
		return nil, err
	}
	for _, c := range n.elements() {
		if c.Name == "Envelope" || c.Name == "Box" {
			b := BBox{Prop: prop.Name, SRS: c.Attrs["srsName"]}
			var lower, upper string
			for _, gc := range c.elements() {
				switch gc.Name {
				case "lowerCorner":
					lower = strings.TrimSpace(gc.Text)
				case "upperCorner":
					upper = strings.TrimSpace(gc.Text)
				case "coordinates":
					// gml:Box uses coordinates "minx,miny maxx,maxy"
					pts := strings.Fields(strings.TrimSpace(gc.Text))
					if len(pts) == 2 {
						lower = strings.ReplaceAll(pts[0], ",", " ")
						upper = strings.ReplaceAll(pts[1], ",", " ")
					}
				}
			}
			lx, ly, ok1 := twoFloats(lower)
			ux, uy, ok2 := twoFloats(upper)
			if !ok1 || !ok2 {
				return nil, fmt.Errorf("invalid BBOX corners")
			}
			b.MinX, b.MinY, b.MaxX, b.MaxY = lx, ly, ux, uy
			return b, nil
		}
	}
	return nil, fmt.Errorf("BBOX missing Envelope")
}

// gmlToWKT converts a gml geometry node to WKT. Supports Point and Polygon.
func gmlToWKT(n *node) (string, bool) {
	switch n.Name {
	case "Point":
		if x, y, ok := pointCoords(n); ok {
			return fmt.Sprintf("POINT(%s %s)", trimFloat(x), trimFloat(y)), true
		}
	case "Polygon":
		if ring, ok := polygonRing(n); ok {
			return "POLYGON((" + ring + "))", true
		}
	case "Envelope":
		var lower, upper string
		for _, gc := range n.elements() {
			if gc.Name == "lowerCorner" {
				lower = strings.TrimSpace(gc.Text)
			}
			if gc.Name == "upperCorner" {
				upper = strings.TrimSpace(gc.Text)
			}
		}
		if lx, ly, ok1 := twoFloats(lower); ok1 {
			if ux, uy, ok2 := twoFloats(upper); ok2 {
				return fmt.Sprintf("POLYGON((%s %s, %s %s, %s %s, %s %s, %s %s))",
					trimFloat(lx), trimFloat(ly), trimFloat(ux), trimFloat(ly),
					trimFloat(ux), trimFloat(uy), trimFloat(lx), trimFloat(uy),
					trimFloat(lx), trimFloat(ly)), true
			}
		}
	}
	return "", false
}

func pointCoords(n *node) (float64, float64, bool) {
	for _, c := range n.elements() {
		switch c.Name {
		case "pos":
			return twoFloats(strings.TrimSpace(c.Text))
		case "coordinates":
			return twoFloats(strings.ReplaceAll(strings.TrimSpace(c.Text), ",", " "))
		}
	}
	return 0, 0, false
}

func polygonRing(n *node) (string, bool) {
	var text string
	// exterior/LinearRing/(posList|coordinates)
	var walk func(*node)
	walk = func(x *node) {
		if x.Name == "posList" || x.Name == "coordinates" {
			text = strings.TrimSpace(x.Text)
		}
		for _, c := range x.elements() {
			walk(c)
		}
	}
	walk(n)
	if text == "" {
		return "", false
	}
	// coordinates form "x,y x,y"; posList form "x y x y"
	if strings.Contains(text, ",") {
		pairs := strings.Fields(text)
		for i, p := range pairs {
			pairs[i] = strings.ReplaceAll(p, ",", " ")
		}
		return strings.Join(pairs, ", "), true
	}
	nums := strings.Fields(text)
	var pts []string
	for i := 0; i+1 < len(nums); i += 2 {
		pts = append(pts, nums[i]+" "+nums[i+1])
	}
	return strings.Join(pts, ", "), true
}

func twoFloats(s string) (float64, float64, bool) {
	f := strings.Fields(s)
	if len(f) != 2 {
		return 0, 0, false
	}
	var x, y float64
	if _, err := fmt.Sscan(f[0], &x); err != nil {
		return 0, 0, false
	}
	if _, err := fmt.Sscan(f[1], &y); err != nil {
		return 0, 0, false
	}
	return x, y, true
}

func trimFloat(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", f), "0"), ".")
}
