package connect

import (
	"context"
	"fmt"

	"github.com/giti/giti/services/catalog/internal/model"
)

// stubConn advertises a store type in the admin wizard but defers actual
// connectivity to a component not yet wired (a live DB, or the Rust render
// service). Validate returns a clear message so the UI can surface it.
type stubConn struct{ reason string }

func (s stubConn) Validate(ctx context.Context, st model.Store) error {
	return fmt.Errorf("%s", s.reason)
}
func (s stubConn) Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error) {
	return nil, fmt.Errorf("%s", s.reason)
}

func init() {
	register("SQLServer", stubConn{reason: "MS SQL Server connector requires a reachable server; configure host/port/database/user"})
	registerMeta(StoreTypeMeta{
		Type: "SQLServer", Kind: "datastore", Category: "Vector", Label: "Microsoft SQL Server",
		Params: []ParamField{
			{Key: "host", Label: "Host", Type: "text", Required: true},
			{Key: "port", Label: "Port", Type: "number", Default: "1433"},
			{Key: "database", Label: "Database", Type: "text", Required: true},
			{Key: "user", Label: "User", Type: "text", Required: true},
			{Key: "passwd", Label: "Password", Type: "password"},
			{Key: "schema", Label: "Schema", Type: "text", Default: "dbo"},
		},
	})

	register("WMS", stubConn{reason: "cascade WMS proxy is served by the wms render service"})
	registerMeta(StoreTypeMeta{Type: "WMS", Kind: "coveragestore", Category: "Cascade", Label: "Cascaded WMS",
		Params: []ParamField{{Key: "url", Label: "Remote WMS capabilities URL", Type: "text", Required: true}}})

	register("WMTS", stubConn{reason: "cascade WMTS proxy is served by the tiles service"})
	registerMeta(StoreTypeMeta{Type: "WMTS", Kind: "coveragestore", Category: "Cascade", Label: "Cascaded WMTS",
		Params: []ParamField{{Key: "url", Label: "Remote WMTS capabilities URL", Type: "text", Required: true}}})
}
