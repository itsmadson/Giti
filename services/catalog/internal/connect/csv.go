package connect

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/giti/giti/services/catalog/internal/model"
)

func init() {
	register("CSV", csvConn{})
	registerMeta(StoreTypeMeta{
		Type: "CSV", Kind: "datastore", Category: "Vector", Label: "CSV (delimited text)",
		Params: []ParamField{
			{Key: "url", Label: "File path", Type: "text", Required: true},
			{Key: "latField", Label: "Latitude column", Type: "text", Default: "lat"},
			{Key: "lonField", Label: "Longitude column", Type: "text", Default: "lon"},
			{Key: "wktField", Label: "WKT column (optional)", Type: "text"},
		},
	})
}

// csvConn validates a delimited text file and derives point geometry from
// lat/lon (or WKT) columns.
type csvConn struct{}

func (c csvConn) header(st model.Store) ([]string, error) {
	path := storePath(st)
	fd, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	r := csv.NewReader(fd)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: cannot read header: %w", path, err)
	}
	return head, nil
}

func (c csvConn) Validate(ctx context.Context, st model.Store) error {
	head, err := c.header(st)
	if err != nil {
		return err
	}
	set := map[string]bool{}
	for _, h := range head {
		set[strings.ToLower(strings.TrimSpace(h))] = true
	}
	if st.Connection["wktField"] != "" {
		if !set[strings.ToLower(st.Connection["wktField"])] {
			return fmt.Errorf("wkt column %q not found", st.Connection["wktField"])
		}
		return nil
	}
	lat, lon := st.Connection["latField"], st.Connection["lonField"]
	if lat == "" {
		lat = "lat"
	}
	if lon == "" {
		lon = "lon"
	}
	if !set[strings.ToLower(lat)] || !set[strings.ToLower(lon)] {
		return fmt.Errorf("lat/lon columns %q/%q not found in header", lat, lon)
	}
	return nil
}

func (c csvConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := c.Validate(ctx, st); err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(storePath(st)), filepath.Ext(storePath(st)))
	geom := "Point"
	if st.Connection["wktField"] != "" {
		geom = "Geometry"
	}
	return []ResourceInfo{{Name: name, GeometryType: geom, SRS: "EPSG:4326"}}, nil
}
