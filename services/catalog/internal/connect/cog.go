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
	register("GeoTIFF", cogConn{})
	registerMeta(StoreTypeMeta{Type: "GeoTIFF", Kind: "coveragestore", Category: "Raster", Label: "GeoTIFF / COG",
		Params: []ParamField{{Key: "url", Label: "File path / URL", Type: "text", Required: true}}})
}

type cogConn struct{}

// Validate accepts classic TIFF and BigTIFF, either byte order.
func (cogConn) Validate(ctx context.Context, st model.Store) error {
	path := storePath(st)
	fd, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fd.Close()
	head := make([]byte, 4)
	if _, err := fd.Read(head); err != nil {
		return err
	}
	ok := bytes.Equal(head, []byte{'I', 'I', 42, 0}) || // little-endian
		bytes.Equal(head, []byte{'M', 'M', 0, 42}) || // big-endian
		bytes.Equal(head, []byte{'I', 'I', 43, 0}) || // BigTIFF LE
		bytes.Equal(head, []byte{'M', 'M', 0, 43}) // BigTIFF BE
	if !ok {
		return fmt.Errorf("%s: not a TIFF", path)
	}
	return nil
}

func (c cogConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := c.Validate(ctx, st); err != nil {
		return nil, err
	}
	path := storePath(st)
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return []ResourceInfo{{Name: name, GeometryType: "Raster", SRS: "EPSG:4326"}}, nil
}
