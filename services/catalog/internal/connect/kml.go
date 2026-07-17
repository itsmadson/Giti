package connect

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/giti/giti/services/catalog/internal/model"
)

func init() {
	register("KML", kmlConn{})
	registerMeta(StoreTypeMeta{Type: "KML", Kind: "datastore", Category: "Vector", Label: "KML / KMZ",
		Params: []ParamField{{Key: "url", Label: "File path (.kml or .kmz)", Type: "text", Required: true}}})
}

// kmlConn validates KML/KMZ files (KMZ = zipped KML).
type kmlConn struct{}

func (k kmlConn) readHead(st model.Store) ([]byte, bool, error) {
	path := storePath(st)
	if strings.EqualFold(filepath.Ext(path), ".kmz") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, true, err
		}
		defer zr.Close()
		for _, f := range zr.File {
			if strings.EqualFold(filepath.Ext(f.Name), ".kml") {
				rc, err := f.Open()
				if err != nil {
					return nil, true, err
				}
				defer rc.Close()
				buf := make([]byte, 512)
				n, _ := rc.Read(buf)
				return buf[:n], true, nil
			}
		}
		return nil, true, fmt.Errorf("%s: no .kml inside KMZ", path)
	}
	fd, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer fd.Close()
	buf := make([]byte, 512)
	n, _ := fd.Read(buf)
	return buf[:n], false, nil
}

func (k kmlConn) Validate(ctx context.Context, st model.Store) error {
	head, _, err := k.readHead(st)
	if err != nil {
		return err
	}
	if !bytes.Contains(bytes.ToLower(head), []byte("<kml")) && !bytes.Contains(head, []byte("http://www.opengis.net/kml")) {
		return fmt.Errorf("%s: not a KML document", storePath(st))
	}
	return nil
}

func (k kmlConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	if err := k.Validate(ctx, st); err != nil {
		return nil, err
	}
	name := strings.TrimSuffix(filepath.Base(storePath(st)), filepath.Ext(storePath(st)))
	return []ResourceInfo{{Name: name, GeometryType: "Geometry", SRS: "EPSG:4326"}}, nil
}
