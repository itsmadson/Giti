// Package model holds catalog entities, shaped after GeoServer's
// configuration model so /rest compatibility is natural.
package model

type Workspace struct {
	Name         string `json:"name" xml:"name"`
	Isolated     bool   `json:"isolated,omitempty" xml:"isolated,omitempty"`
	NamespaceURI string `json:"-" xml:"-"` // exposed via /rest/namespaces later
}

type Store struct { // datastore or coveragestore
	Name        string            `json:"name" xml:"name"`
	Workspace   string            `json:"-" xml:"-"`
	Kind        string            `json:"-" xml:"-"` // "datastore" | "coveragestore"
	Type        string            `json:"type,omitempty" xml:"type,omitempty"` // PostGIS, Shapefile, GeoPackage, GeoJSON, GeoTIFF, GeoParquet
	Enabled     bool              `json:"enabled" xml:"enabled"`
	Description string            `json:"description,omitempty" xml:"description,omitempty"`
	Connection  map[string]string `json:"-" xml:"-"` // connectionParameters
}

type FeatureType struct {
	Name       string `json:"name" xml:"name"`
	NativeName string `json:"nativeName" xml:"nativeName"`
	Workspace  string `json:"-" xml:"-"`
	Store      string `json:"-" xml:"-"`
	Title      string `json:"title,omitempty" xml:"title,omitempty"`
	SRS        string `json:"srs,omitempty" xml:"srs,omitempty"`
	Enabled    bool   `json:"enabled" xml:"enabled"`
}

type Coverage struct {
	Name       string `json:"name" xml:"name"`
	NativeName string `json:"nativeName" xml:"nativeName"`
	Workspace  string `json:"-" xml:"-"`
	Store      string `json:"-" xml:"-"`
	Title      string `json:"title,omitempty" xml:"title,omitempty"`
	SRS        string `json:"srs,omitempty" xml:"srs,omitempty"`
	Enabled    bool   `json:"enabled" xml:"enabled"`
}

type Layer struct {
	Name         string `json:"name" xml:"name"`
	Workspace    string `json:"-" xml:"-"`
	Type         string `json:"type" xml:"type"` // VECTOR | RASTER
	ResourceName string `json:"-" xml:"-"`       // featuretype/coverage name
	DefaultStyle string `json:"-" xml:"-"`
	Enabled      bool   `json:"enabled,omitempty" xml:"enabled,omitempty"`
}

type Style struct {
	Name      string `json:"name" xml:"name"`
	Workspace string `json:"-" xml:"-"` // empty = global
	Format    string `json:"format,omitempty" xml:"format,omitempty"` // sld | mbstyle | geocss
	Filename  string `json:"filename,omitempty" xml:"filename,omitempty"`
	Body      string `json:"-" xml:"-"`
}

type LayerGroup struct {
	Name      string   `json:"name" xml:"name"`
	Workspace string   `json:"-" xml:"-"`
	Mode      string   `json:"mode" xml:"mode"` // SINGLE
	Layers    []string `json:"-" xml:"-"`
}
