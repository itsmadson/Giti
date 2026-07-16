package wfs

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/geoson/geoson/libs/ogc-kit/filter"
)

// xmlNode is a namespace-agnostic decoded element (keyed by local name).
type xmlNode struct {
	name     string
	attrs    map[string]string
	children []*xmlNode
	text     string
}

func decodeXMLTree(body []byte) (*xmlNode, error) {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	var stack []*xmlNode
	var root *xmlNode
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, fmt.Errorf("malformed xml: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			n := &xmlNode{name: t.Name.Local, attrs: map[string]string{}}
			for _, a := range t.Attr {
				n.attrs[a.Name.Local] = a.Value
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
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
				stack[len(stack)-1].text += string(t)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("empty xml")
	}
	return root, nil
}

// reserialize renders a node subtree back to XML (local names only).
func (n *xmlNode) reserialize() string {
	var b strings.Builder
	b.WriteString("<" + n.name)
	for k, v := range n.attrs {
		fmt.Fprintf(&b, " %s=%q", k, v)
	}
	b.WriteString(">")
	if len(n.children) == 0 {
		b.WriteString(n.text)
	}
	for _, c := range n.children {
		b.WriteString(c.reserialize())
	}
	b.WriteString("</" + n.name + ">")
	return b.String()
}

func (n *xmlNode) find(name string) *xmlNode {
	for _, c := range n.children {
		if c.name == name {
			return c
		}
	}
	return nil
}

// gmlNodeToWKT converts a geometry-property node (wrapping gml Point/LineString/
// Polygon) to WKT via reserialize + a small local converter.
func gmlNodeToWKT(prop *xmlNode) (string, bool) {
	for _, g := range prop.children {
		switch g.name {
		case "Point":
			if x, y, ok := twoCoord(g); ok {
				return fmt.Sprintf("POINT(%s %s)", x, y), true
			}
		case "LineString":
			if pts, ok := coordList(g); ok {
				return "LINESTRING(" + pts + ")", true
			}
		case "Polygon":
			if pts, ok := polyRing(g); ok {
				return "POLYGON((" + pts + "))", true
			}
		}
	}
	return "", false
}

func coordText(n *xmlNode) string {
	for _, c := range n.children {
		if c.name == "coordinates" || c.name == "pos" || c.name == "posList" {
			return strings.TrimSpace(c.text)
		}
		if t := coordText(c); t != "" {
			return t
		}
	}
	return ""
}

func twoCoord(n *xmlNode) (string, string, bool) {
	t := coordText(n)
	t = strings.ReplaceAll(t, ",", " ")
	f := strings.Fields(t)
	if len(f) < 2 {
		return "", "", false
	}
	return f[0], f[1], true
}

// coordList turns "x,y x,y" or "x y x y" into "x y, x y".
func coordList(n *xmlNode) (string, bool) {
	t := coordText(n)
	if t == "" {
		return "", false
	}
	if strings.Contains(t, ",") {
		pairs := strings.Fields(t)
		for i, p := range pairs {
			pairs[i] = strings.ReplaceAll(p, ",", " ")
		}
		return strings.Join(pairs, ", "), true
	}
	nums := strings.Fields(t)
	var pts []string
	for i := 0; i+1 < len(nums); i += 2 {
		pts = append(pts, nums[i]+" "+nums[i+1])
	}
	return strings.Join(pts, ", "), len(pts) > 0
}

func polyRing(n *xmlNode) (string, bool) {
	return coordList(n)
}

// txFilterWhere builds the WHERE clause for an Update/Delete op from its
// <Filter> child (or FeatureId/ResourceId), ANDing X-Geoson-CQL-Write.
func txFilterWhere(r *http.Request, op *xmlNode, startArg int) (string, []any, error) {
	var expr filter.Expr
	fnode := op.find("Filter")
	if fnode != nil {
		if e := featureIDFromFilter(fnode); e != nil {
			expr = e
		} else {
			parsed, err := filter.ParseFilterXML([]byte(fnode.reserialize()))
			if err != nil {
				return "", nil, err
			}
			expr = parsed
		}
	}
	if authCQL := r.Header.Get("X-Geoson-CQL-Write"); authCQL != "" {
		if e, err := filter.ParseCQL(authCQL); err == nil {
			expr = andExpr(expr, e)
		}
	}
	if expr == nil {
		return "", nil, nil
	}
	sql, args, err := filter.ToSQL(expr, startArg)
	if err != nil {
		return "", nil, err
	}
	return " WHERE " + sql, args, nil
}

// featureIDFromFilter handles <FeatureId fid="layer.3"/> and <ResourceId rid=.../>.
func featureIDFromFilter(fnode *xmlNode) filter.Expr {
	for _, c := range fnode.children {
		var raw string
		switch c.name {
		case "FeatureId":
			raw = c.attrs["fid"]
		case "ResourceId":
			raw = c.attrs["rid"]
		}
		if raw == "" {
			continue
		}
		id := raw
		if i := strings.LastIndexByte(raw, '.'); i >= 0 {
			id = raw[i+1:]
		}
		if v, err := strconv.ParseFloat(id, 64); err == nil {
			return filter.Compare{Op: "=", Left: filter.Property{Name: "id"}, Right: filter.Literal{Value: v}}
		}
		return filter.Compare{Op: "=", Left: filter.Property{Name: "id"}, Right: filter.Literal{Value: id}}
	}
	return nil
}
