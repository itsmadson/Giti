package store

import (
	"context"
	"encoding/json"
)

type Gridset struct {
	Name     string    `json:"name"`
	SRS      string    `json:"srs"`
	Extent   []float64 `json:"extent"`
	TileSize int       `json:"tileSize"`
	Levels   int       `json:"levels"`
}

type BlobStore struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Config    map[string]string `json:"config"`
	IsDefault bool              `json:"isDefault"`
}

type LayerCache struct {
	Workspace    string   `json:"workspace"`
	Layer        string   `json:"layer"`
	Enabled      bool     `json:"enabled"`
	MetatileX    int      `json:"metatileX"`
	MetatileY    int      `json:"metatileY"`
	Gutter       int      `json:"gutter"`
	Formats      []string `json:"formats"`
	ExpireServer int      `json:"expireServer"`
	ExpireClient int      `json:"expireClient"`
	Gridsets     []string `json:"gridsets"`
	BlobStore    string   `json:"blobStore"`
}

type DiskQuota struct {
	Policy   string `json:"policy"`
	MaxBytes int64  `json:"maxBytes"`
}

func (s *Store) ListGridsets(ctx context.Context) ([]Gridset, error) {
	rows, err := s.db.Query(ctx, `SELECT name, srs, extent, tile_size, levels FROM gridsets ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Gridset{}
	for rows.Next() {
		var g Gridset
		var ext []byte
		if err := rows.Scan(&g.Name, &g.SRS, &ext, &g.TileSize, &g.Levels); err != nil {
			return nil, err
		}
		json.Unmarshal(ext, &g.Extent)
		out = append(out, g)
	}
	return out, rows.Err()
}

func (s *Store) SaveGridset(ctx context.Context, g Gridset) error {
	ext, _ := json.Marshal(g.Extent)
	if g.TileSize == 0 {
		g.TileSize = 256
	}
	if g.Levels == 0 {
		g.Levels = 22
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO gridsets(name, srs, extent, tile_size, levels) VALUES($1,$2,$3,$4,$5)
		ON CONFLICT (name) DO UPDATE SET srs=$2, extent=$3, tile_size=$4, levels=$5`,
		g.Name, g.SRS, ext, g.TileSize, g.Levels)
	return mapErr(err)
}

func (s *Store) DeleteGridset(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM gridsets WHERE name=$1`, name)
	return err
}

func (s *Store) ListBlobStores(ctx context.Context) ([]BlobStore, error) {
	rows, err := s.db.Query(ctx, `SELECT name, type, config, is_default FROM blobstores ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BlobStore{}
	for rows.Next() {
		var b BlobStore
		var cfg []byte
		if err := rows.Scan(&b.Name, &b.Type, &cfg, &b.IsDefault); err != nil {
			return nil, err
		}
		json.Unmarshal(cfg, &b.Config)
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) SaveBlobStore(ctx context.Context, b BlobStore) error {
	cfg, _ := json.Marshal(b.Config)
	if b.Type == "" {
		b.Type = "file"
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO blobstores(name, type, config, is_default) VALUES($1,$2,$3,$4)
		ON CONFLICT (name) DO UPDATE SET type=$2, config=$3, is_default=$4`,
		b.Name, b.Type, cfg, b.IsDefault)
	return mapErr(err)
}

func (s *Store) DeleteBlobStore(ctx context.Context, name string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM blobstores WHERE name=$1`, name)
	return err
}

func (s *Store) GetQuota(ctx context.Context) (DiskQuota, error) {
	var q DiskQuota
	err := s.db.QueryRow(ctx, `SELECT policy, max_bytes FROM disk_quota WHERE id=1`).Scan(&q.Policy, &q.MaxBytes)
	return q, mapErr(err)
}

func (s *Store) SetQuota(ctx context.Context, q DiskQuota) error {
	_, err := s.db.Exec(ctx, `UPDATE disk_quota SET policy=$1, max_bytes=$2 WHERE id=1`, q.Policy, q.MaxBytes)
	return err
}

// GetLayerCache returns the cache config for a layer, defaulting if unset.
func (s *Store) GetLayerCache(ctx context.Context, ws, layer string) (LayerCache, error) {
	c := LayerCache{Workspace: ws, Layer: layer, Enabled: true, MetatileX: 4, MetatileY: 4,
		Formats: []string{"application/vnd.mapbox-vector-tile", "image/png"}, Gridsets: []string{"EPSG:3857"}}
	err := s.db.QueryRow(ctx, `
		SELECT enabled, metatile_x, metatile_y, gutter, formats, expire_server, expire_client, gridsets, blobstore
		FROM layer_cache WHERE workspace=$1 AND layer=$2`, ws, layer).
		Scan(&c.Enabled, &c.MetatileX, &c.MetatileY, &c.Gutter, &c.Formats, &c.ExpireServer, &c.ExpireClient, &c.Gridsets, &c.BlobStore)
	if err == ErrNotFound || (err != nil && err.Error() == "no rows in result set") {
		return c, nil
	}
	return c, mapErr(err)
}

func (s *Store) SaveLayerCache(ctx context.Context, c LayerCache) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO layer_cache(workspace, layer, enabled, metatile_x, metatile_y, gutter, formats, expire_server, expire_client, gridsets, blobstore)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (workspace, layer) DO UPDATE SET
			enabled=$3, metatile_x=$4, metatile_y=$5, gutter=$6, formats=$7,
			expire_server=$8, expire_client=$9, gridsets=$10, blobstore=$11`,
		c.Workspace, c.Layer, c.Enabled, c.MetatileX, c.MetatileY, c.Gutter, c.Formats,
		c.ExpireServer, c.ExpireClient, c.Gridsets, c.BlobStore)
	return mapErr(err)
}
