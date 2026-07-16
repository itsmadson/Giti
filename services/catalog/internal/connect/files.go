package connect

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/giti/giti/services/catalog/internal/model"
)

func init() {
	register("Shapefile", fileConn{ext: ".shp", magic: []byte{0x00, 0x00, 0x27, 0x0a}})
	register("GeoPackage", fileConn{ext: ".gpkg", magic: []byte("SQLite format 3\x00")})
	register("GeoJSON", fileConn{ext: ".geojson", jsonCheck: true})
}

// fileConn validates file-backed stores by existence + magic bytes.
// Deep schema introspection is done by wfs/wms at read time.
type fileConn struct {
	ext       string
	magic     []byte
	jsonCheck bool
}

func storePath(st model.Store) string {
	return strings.TrimPrefix(st.Connection["url"], "file://")
}

func (f fileConn) Validate(ctx context.Context, st model.Store) error {
	path := storePath(st)
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	head := make([]byte, 64)
	n, _ := fd.Read(head)
	head = head[:n]
	if f.jsonCheck {
		trimmed := bytes.TrimLeft(head, " \t\r\n")
		if len(trimmed) == 0 || trimmed[0] != '{' {
			return fmt.Errorf("%s: not a JSON document", path)
		}
		return nil
	}
	if len(head) < len(f.magic) || !bytes.Equal(head[:len(f.magic)], f.magic) {
		return fmt.Errorf("%s: bad magic for %s", path, f.ext)
	}
	return nil
}

func (f fileConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := f.Validate(ctx, st); err != nil {
		return nil, err
	}
	path := storePath(st)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return []ResourceInfo{{Name: name, SRS: "EPSG:4326"}}, nil
}
