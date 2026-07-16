// Package process defines WPS processes backed by PostGIS.
package process

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ParamKind int

const (
	KindGeometry ParamKind = iota
	KindDouble
	KindString
)

type Param struct {
	Name, Title string
	Kind        ParamKind
	Required    bool
}

type Process struct {
	Identifier, Title, Abstract string
	Inputs                      []Param
	Outputs                     []Param
	Run                         func(ctx context.Context, db *pgxpool.Pool, inputs map[string]string) (string, error)
}

var notImpl = func(context.Context, *pgxpool.Pool, map[string]string) (string, error) {
	return "", errors.New("not implemented")
}

func geom(name string, req bool) Param {
	return Param{Name: name, Title: name, Kind: KindGeometry, Required: req}
}
func dbl(name string, req bool) Param {
	return Param{Name: name, Title: name, Kind: KindDouble, Required: req}
}
func str(name string, req bool) Param {
	return Param{Name: name, Title: name, Kind: KindString, Required: req}
}

// Run funcs are wired to real implementations in geometry.go's init().
var (
	runBuffer       = notImpl
	runCentroid     = notImpl
	runArea         = notImpl
	runLength       = notImpl
	runReproject    = notImpl
	runIntersection = notImpl
	runUnion        = notImpl
	runSimplify     = notImpl
)

var registry = map[string]Process{
	"geoson:buffer": {Identifier: "geoson:buffer", Title: "Buffer", Abstract: "Buffer a geometry by a distance",
		Inputs: []Param{geom("geom", true), dbl("distance", true)}, Outputs: []Param{geom("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runBuffer(c, d, i) }},
	"geoson:centroid": {Identifier: "geoson:centroid", Title: "Centroid", Abstract: "Centroid of a geometry",
		Inputs: []Param{geom("geom", true)}, Outputs: []Param{geom("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runCentroid(c, d, i) }},
	"geoson:area": {Identifier: "geoson:area", Title: "Area", Abstract: "Area of a geometry",
		Inputs: []Param{geom("geom", true)}, Outputs: []Param{dbl("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runArea(c, d, i) }},
	"geoson:length": {Identifier: "geoson:length", Title: "Length", Abstract: "Length or perimeter of a geometry",
		Inputs: []Param{geom("geom", true)}, Outputs: []Param{dbl("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runLength(c, d, i) }},
	"geoson:reproject": {Identifier: "geoson:reproject", Title: "Reproject", Abstract: "Transform a geometry between SRIDs",
		Inputs: []Param{geom("geom", true), str("sourceSRID", true), str("targetSRID", true)}, Outputs: []Param{geom("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runReproject(c, d, i) }},
	"geoson:intersection": {Identifier: "geoson:intersection", Title: "Intersection", Abstract: "Intersection of two geometries",
		Inputs: []Param{geom("a", true), geom("b", true)}, Outputs: []Param{geom("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runIntersection(c, d, i) }},
	"geoson:union": {Identifier: "geoson:union", Title: "Union", Abstract: "Union of two geometries",
		Inputs: []Param{geom("a", true), geom("b", true)}, Outputs: []Param{geom("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runUnion(c, d, i) }},
	"geoson:simplify": {Identifier: "geoson:simplify", Title: "Simplify", Abstract: "Douglas-Peucker simplify",
		Inputs: []Param{geom("geom", true), dbl("tolerance", true)}, Outputs: []Param{geom("result", true)}, Run: func(c context.Context, d *pgxpool.Pool, i map[string]string) (string, error) { return runSimplify(c, d, i) }},
}

func Registry() map[string]Process { return registry }
func Get(id string) (Process, bool) { p, ok := registry[id]; return p, ok }
