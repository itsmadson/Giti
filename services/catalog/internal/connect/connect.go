// Package connect holds store connectors: connection validation and
// resource introspection per store type.
package connect

import (
	"context"
	"fmt"

	"github.com/geoson/geoson/services/catalog/internal/model"
)

type ResourceInfo struct {
	Name         string
	GeometryType string
	SRS          string
}

type Connector interface {
	Validate(ctx context.Context, st model.Store) error
	Introspect(ctx context.Context, st model.Store) ([]ResourceInfo, error)
}

var registry = map[string]Connector{}

func register(storeType string, c Connector) { registry[storeType] = c }

func ForType(storeType string) (Connector, error) {
	c, ok := registry[storeType]
	if !ok {
		return nil, fmt.Errorf("unsupported store type %q", storeType)
	}
	return c, nil
}
