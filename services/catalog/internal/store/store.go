// Package store is the catalog repository over Postgres.
package store

import (
	"context"
	"errors"

	"github.com/giti/giti/services/catalog/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("already exists")
)

type Store struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Store { return &Store{db: db} }

func mapErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *Store) CreateWorkspace(ctx context.Context, w model.Workspace) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO workspaces(name, isolated, namespace_uri) VALUES($1,$2,$3)`,
		w.Name, w.Isolated, w.NamespaceURI)
	return mapErr(err)
}

func (s *Store) GetWorkspace(ctx context.Context, name string) (model.Workspace, error) {
	var w model.Workspace
	err := s.db.QueryRow(ctx,
		`SELECT name, isolated, namespace_uri FROM workspaces WHERE name=$1`, name,
	).Scan(&w.Name, &w.Isolated, &w.NamespaceURI)
	return w, mapErr(err)
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]model.Workspace, error) {
	rows, err := s.db.Query(ctx,
		`SELECT name, isolated, namespace_uri FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Workspace
	for rows.Next() {
		var w model.Workspace
		if err := rows.Scan(&w.Name, &w.Isolated, &w.NamespaceURI); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) UpdateWorkspace(ctx context.Context, name string, w model.Workspace) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE workspaces SET name=$2, isolated=$3 WHERE name=$1`, name, w.Name, w.Isolated)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteWorkspace(ctx context.Context, name string, recurse bool) error {
	if !recurse {
		var n int
		if err := s.db.QueryRow(ctx,
			`SELECT count(*) FROM stores WHERE workspace=$1`, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
	}
	tag, err := s.db.Exec(ctx, `DELETE FROM workspaces WHERE name=$1`, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateStore(ctx context.Context, st model.Store) error {
	if st.Connection == nil {
		st.Connection = map[string]string{}
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO stores(workspace, name, kind, type, enabled, description, connection)
		 VALUES($1,$2,$3,$4,$5,$6,$7)`,
		st.Workspace, st.Name, st.Kind, st.Type, st.Enabled, st.Description, st.Connection)
	return mapErr(err)
}

func (s *Store) GetStore(ctx context.Context, ws, name, kind string) (model.Store, error) {
	var st model.Store
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, kind, type, enabled, description, connection
		 FROM stores WHERE workspace=$1 AND name=$2 AND kind=$3`, ws, name, kind,
	).Scan(&st.Workspace, &st.Name, &st.Kind, &st.Type, &st.Enabled, &st.Description, &st.Connection)
	return st, mapErr(err)
}

func (s *Store) ListStores(ctx context.Context, ws, kind string) ([]model.Store, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, kind, type, enabled, description, connection
		 FROM stores WHERE workspace=$1 AND kind=$2 ORDER BY name`, ws, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Store
	for rows.Next() {
		var st model.Store
		if err := rows.Scan(&st.Workspace, &st.Name, &st.Kind, &st.Type,
			&st.Enabled, &st.Description, &st.Connection); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStore(ctx context.Context, ws, name string, st model.Store) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE stores SET type=$4, enabled=$5, description=$6, connection=$7
		 WHERE workspace=$1 AND name=$2 AND kind=$3`,
		ws, name, st.Kind, st.Type, st.Enabled, st.Description, st.Connection)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteStore(ctx context.Context, ws, name string, recurse bool) error {
	if !recurse {
		var n int
		if err := s.db.QueryRow(ctx,
			`SELECT count(*) FROM resources WHERE workspace=$1 AND store=$2`,
			ws, name).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return ErrConflict
		}
	}
	tag, err := s.db.Exec(ctx,
		`DELETE FROM stores WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateFeatureType(ctx context.Context, ft model.FeatureType) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO resources(workspace, store, name, kind, native_name, title, srs, enabled)
		 VALUES($1,$2,$3,'featuretype',$4,$5,$6,$7)`,
		ft.Workspace, ft.Store, ft.Name, ft.NativeName, ft.Title, ft.SRS, ft.Enabled)
	return mapErr(err)
}

func (s *Store) GetFeatureType(ctx context.Context, ws, st, name string) (model.FeatureType, error) {
	var ft model.FeatureType
	err := s.db.QueryRow(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='featuretype'`,
		ws, st, name,
	).Scan(&ft.Workspace, &ft.Store, &ft.Name, &ft.NativeName, &ft.Title, &ft.SRS, &ft.Enabled)
	return ft, mapErr(err)
}

func (s *Store) ListFeatureTypes(ctx context.Context, ws, st string) ([]model.FeatureType, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND kind='featuretype' ORDER BY name`, ws, st)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.FeatureType
	for rows.Next() {
		var ft model.FeatureType
		if err := rows.Scan(&ft.Workspace, &ft.Store, &ft.Name, &ft.NativeName,
			&ft.Title, &ft.SRS, &ft.Enabled); err != nil {
			return nil, err
		}
		out = append(out, ft)
	}
	return out, rows.Err()
}

func (s *Store) DeleteFeatureType(ctx context.Context, ws, st, name string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='featuretype'`,
		ws, st, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Coverages: identical shape over kind='coverage'.
func (s *Store) CreateCoverage(ctx context.Context, c model.Coverage) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO resources(workspace, store, name, kind, native_name, title, srs, enabled)
		 VALUES($1,$2,$3,'coverage',$4,$5,$6,$7)`,
		c.Workspace, c.Store, c.Name, c.NativeName, c.Title, c.SRS, c.Enabled)
	return mapErr(err)
}

func (s *Store) GetCoverage(ctx context.Context, ws, st, name string) (model.Coverage, error) {
	var c model.Coverage
	err := s.db.QueryRow(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='coverage'`,
		ws, st, name,
	).Scan(&c.Workspace, &c.Store, &c.Name, &c.NativeName, &c.Title, &c.SRS, &c.Enabled)
	return c, mapErr(err)
}

func (s *Store) ListCoverages(ctx context.Context, ws, st string) ([]model.Coverage, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, store, name, native_name, title, srs, enabled
		 FROM resources WHERE workspace=$1 AND store=$2 AND kind='coverage' ORDER BY name`, ws, st)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Coverage
	for rows.Next() {
		var c model.Coverage
		if err := rows.Scan(&c.Workspace, &c.Store, &c.Name, &c.NativeName,
			&c.Title, &c.SRS, &c.Enabled); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteCoverage(ctx context.Context, ws, st, name string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM resources WHERE workspace=$1 AND store=$2 AND name=$3 AND kind='coverage'`,
		ws, st, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateLayer(ctx context.Context, l model.Layer) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO layers(workspace, name, type, resource_name, default_style, enabled)
		 VALUES($1,$2,$3,$4,$5,$6)`,
		l.Workspace, l.Name, l.Type, l.ResourceName, l.DefaultStyle, l.Enabled)
	return mapErr(err)
}

func (s *Store) GetLayer(ctx context.Context, ws, name string) (model.Layer, error) {
	var l model.Layer
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, type, resource_name, default_style, enabled
		 FROM layers WHERE workspace=$1 AND name=$2`, ws, name,
	).Scan(&l.Workspace, &l.Name, &l.Type, &l.ResourceName, &l.DefaultStyle, &l.Enabled)
	return l, mapErr(err)
}

func (s *Store) ListLayers(ctx context.Context) ([]model.Layer, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, type, resource_name, default_style, enabled
		 FROM layers ORDER BY workspace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Layer
	for rows.Next() {
		var l model.Layer
		if err := rows.Scan(&l.Workspace, &l.Name, &l.Type, &l.ResourceName,
			&l.DefaultStyle, &l.Enabled); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func (s *Store) UpdateLayer(ctx context.Context, ws, name string, l model.Layer) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE layers SET default_style=$3, enabled=$4 WHERE workspace=$1 AND name=$2`,
		ws, name, l.DefaultStyle, l.Enabled)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteLayer(ctx context.Context, ws, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM layers WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateStyle(ctx context.Context, st model.Style) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO styles(workspace, name, format, filename, body) VALUES($1,$2,$3,$4,$5)`,
		st.Workspace, st.Name, st.Format, st.Filename, st.Body)
	return mapErr(err)
}

func (s *Store) GetStyle(ctx context.Context, ws, name string) (model.Style, error) {
	var st model.Style
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, format, filename, body FROM styles WHERE workspace=$1 AND name=$2`,
		ws, name,
	).Scan(&st.Workspace, &st.Name, &st.Format, &st.Filename, &st.Body)
	return st, mapErr(err)
}

func (s *Store) ListStyles(ctx context.Context, ws string) ([]model.Style, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, format, filename, body FROM styles WHERE workspace=$1 ORDER BY name`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Style
	for rows.Next() {
		var st model.Style
		if err := rows.Scan(&st.Workspace, &st.Name, &st.Format, &st.Filename, &st.Body); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) UpdateStyle(ctx context.Context, ws, name string, st model.Style) error {
	tag, err := s.db.Exec(ctx,
		`UPDATE styles SET format=$3, filename=$4, body=$5 WHERE workspace=$1 AND name=$2`,
		ws, name, st.Format, st.Filename, st.Body)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteStyle(ctx context.Context, ws, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM styles WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateLayerGroup(ctx context.Context, lg model.LayerGroup) error {
	if lg.Layers == nil {
		lg.Layers = []string{}
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO layer_groups(workspace, name, mode, layers) VALUES($1,$2,$3,$4)`,
		lg.Workspace, lg.Name, lg.Mode, lg.Layers)
	return mapErr(err)
}

func (s *Store) GetLayerGroup(ctx context.Context, ws, name string) (model.LayerGroup, error) {
	var lg model.LayerGroup
	err := s.db.QueryRow(ctx,
		`SELECT workspace, name, mode, layers FROM layer_groups WHERE workspace=$1 AND name=$2`,
		ws, name,
	).Scan(&lg.Workspace, &lg.Name, &lg.Mode, &lg.Layers)
	return lg, mapErr(err)
}

func (s *Store) ListLayerGroups(ctx context.Context, ws string) ([]model.LayerGroup, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, mode, layers FROM layer_groups WHERE workspace=$1 ORDER BY name`, ws)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.LayerGroup
	for rows.Next() {
		var lg model.LayerGroup
		if err := rows.Scan(&lg.Workspace, &lg.Name, &lg.Mode, &lg.Layers); err != nil {
			return nil, err
		}
		out = append(out, lg)
	}
	return out, rows.Err()
}

func (s *Store) DeleteLayerGroup(ctx context.Context, ws, name string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM layer_groups WHERE workspace=$1 AND name=$2`, ws, name)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListAllStores returns every store across all workspaces (for the admin UI).
func (s *Store) ListAllStores(ctx context.Context) ([]model.Store, error) {
	rows, err := s.db.Query(ctx,
		`SELECT workspace, name, kind, type, enabled, description, connection
		 FROM stores ORDER BY workspace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Store
	for rows.Next() {
		var st model.Store
		if err := rows.Scan(&st.Workspace, &st.Name, &st.Kind, &st.Type,
			&st.Enabled, &st.Description, &st.Connection); err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// LayerAttribute is one non-geometry column of a layer's table.
type LayerAttribute struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// LayerDetail is the full view of a published layer for the admin UI.
type LayerDetail struct {
	Workspace    string           `json:"workspace"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	SRS          string           `json:"srs"`
	Store        string           `json:"store"`
	Table        string           `json:"table"`
	GeomColumn   string           `json:"geomColumn"`
	GeomType     string           `json:"geomType"`
	DefaultStyle string           `json:"defaultStyle"`
	Attributes   []LayerAttribute `json:"attributes"`
	Bbox         []float64        `json:"bbox,omitempty"` // minx,miny,maxx,maxy (EPSG:4326)
	FeatureCount int64            `json:"featureCount"`
}

// GetLayerDetail resolves a layer and introspects its table (attributes, bbox,
// count). Assumes the table lives in this database (host=self stores).
func (s *Store) GetLayerDetail(ctx context.Context, ws, name string) (LayerDetail, error) {
	var d LayerDetail
	d.Workspace, d.Name = ws, name
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(l.type,'VECTOR'), r.srs, r.store, r.native_name, COALESCE(l.default_style,'')
		FROM resources r
		LEFT JOIN layers l ON l.workspace=r.workspace AND l.name=r.name
		WHERE r.workspace=$1 AND r.name=$2 AND r.kind='featuretype'`,
		ws, name).Scan(&d.Type, &d.SRS, &d.Store, &d.Table, &d.DefaultStyle)
	if err != nil {
		return d, mapErr(err)
	}

	// geometry column + type (best-effort; table may live in an external store)
	_ = s.db.QueryRow(ctx,
		`SELECT f_geometry_column, type FROM geometry_columns WHERE f_table_name=$1 LIMIT 1`,
		d.Table).Scan(&d.GeomColumn, &d.GeomType)

	if d.GeomColumn != "" {
		rows, err := s.db.Query(ctx,
			`SELECT column_name, data_type FROM information_schema.columns
			 WHERE table_name=$1 AND column_name <> $2 ORDER BY ordinal_position`,
			d.Table, d.GeomColumn)
		if err == nil {
			for rows.Next() {
				var a LayerAttribute
				if rows.Scan(&a.Name, &a.Type) == nil {
					d.Attributes = append(d.Attributes, a)
				}
			}
			rows.Close()
		}
		// bbox in EPSG:4326 + feature count (identifier is validated: from catalog)
		if validTable(d.Table) && validTable(d.GeomColumn) {
			var minx, miny, maxx, maxy *float64
			q := `SELECT ST_XMin(e), ST_YMin(e), ST_XMax(e), ST_YMax(e), n FROM (
				SELECT ST_Extent(ST_Transform("` + d.GeomColumn + `",4326)) e, count(*) n FROM "` + d.Table + `") s`
			if s.db.QueryRow(ctx, q).Scan(&minx, &miny, &maxx, &maxy, &d.FeatureCount) == nil &&
				minx != nil {
				d.Bbox = []float64{*minx, *miny, *maxx, *maxy}
			}
		}
	}
	return d, nil
}

func validTable(name string) bool {
	if name == "" {
		return false
	}
	for i, c := range name {
		if c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
			continue
		}
		if i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return false
	}
	return true
}
