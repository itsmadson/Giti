// Package connect holds store connectors: connection validation and
// resource introspection per store type.
package connect

import (
	"context"
	"fmt"
	"sort"

	"github.com/giti/giti/services/catalog/internal/model"
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

// ParamField describes one connection parameter for a store type.
type ParamField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Type     string `json:"type"` // text | number | password | select
	Default  string `json:"default,omitempty"`
	Required bool   `json:"required"`
}

// StoreTypeMeta is a store type advertised to the admin UI.
type StoreTypeMeta struct {
	Type     string       `json:"type"`     // e.g. "PostGIS"
	Kind     string       `json:"kind"`     // datastore | coveragestore
	Category string       `json:"category"` // Vector | Raster | Cascade
	Label    string       `json:"label"`
	Params   []ParamField `json:"params"`
}

// Described is implemented by connectors that expose UI metadata.
type Described interface{ Meta() StoreTypeMeta }

var metas []StoreTypeMeta

func registerMeta(m StoreTypeMeta) { metas = append(metas, m) }

// StoreTypes returns all advertised store types (sorted by category then label).
func StoreTypes() []StoreTypeMeta {
	out := make([]StoreTypeMeta, len(metas))
	copy(out, metas)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Label < out[j].Label
	})
	return out
}
