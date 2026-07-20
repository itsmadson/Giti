package store

import (
	"context"
	"fmt"
	"strings"
)

// FeatureTypePatch holds optional featuretype fields (nil = leave unchanged).
type FeatureTypePatch struct {
	Title           *string   `json:"title"`
	Abstract        *string   `json:"abstract"`
	Keywords        *[]string `json:"keywords"`
	SRS             *string   `json:"srs"`
	DeclaredSRS     *string   `json:"declaredSrs"`
	SRSHandling     *string   `json:"srsHandling"`
	TimeColumn      *string   `json:"timeColumn"`
	ElevationColumn *string   `json:"elevationColumn"`
}

// LayerPatch holds optional layer/publishing fields.
type LayerPatch struct {
	DefaultStyle    *string   `json:"defaultStyle"`
	AlternateStyles *[]string `json:"alternateStyles"`
	Queryable       *bool     `json:"queryable"`
	Opaque          *bool     `json:"opaque"`
	Advertised      *bool     `json:"advertised"`
	Enabled         *bool     `json:"enabled"`
}

// PatchFeatureType applies a partial update to a featuretype resource.
func (s *Store) PatchFeatureType(ctx context.Context, ws, name string, p FeatureTypePatch) error {
	set := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		set = append(set, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if p.Title != nil {
		add("title", *p.Title)
	}
	if p.Abstract != nil {
		add("abstract", *p.Abstract)
	}
	if p.Keywords != nil {
		add("keywords", *p.Keywords)
	}
	if p.SRS != nil {
		add("srs", *p.SRS)
	}
	if p.DeclaredSRS != nil {
		add("declared_srs", *p.DeclaredSRS)
	}
	if p.SRSHandling != nil {
		add("srs_handling", *p.SRSHandling)
	}
	if p.TimeColumn != nil {
		add("time_column", *p.TimeColumn)
	}
	if p.ElevationColumn != nil {
		add("elevation_column", *p.ElevationColumn)
	}
	if len(set) == 0 {
		return nil
	}
	args = append(args, ws, name)
	q := fmt.Sprintf(`UPDATE resources SET %s WHERE workspace=$%d AND name=$%d AND kind='featuretype'`,
		strings.Join(set, ", "), len(args)-1, len(args))
	tag, err := s.db.Exec(ctx, q, args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PatchLayer applies a partial update to a layer's publishing config.
func (s *Store) PatchLayer(ctx context.Context, ws, name string, p LayerPatch) error {
	set := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		set = append(set, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if p.DefaultStyle != nil {
		add("default_style", *p.DefaultStyle)
	}
	if p.AlternateStyles != nil {
		add("alternate_styles", *p.AlternateStyles)
	}
	if p.Queryable != nil {
		add("queryable", *p.Queryable)
	}
	if p.Opaque != nil {
		add("opaque", *p.Opaque)
	}
	if p.Advertised != nil {
		add("advertised", *p.Advertised)
	}
	if p.Enabled != nil {
		add("enabled", *p.Enabled)
	}
	if len(set) == 0 {
		return nil
	}
	args = append(args, ws, name)
	q := fmt.Sprintf(`UPDATE layers SET %s WHERE workspace=$%d AND name=$%d`,
		strings.Join(set, ", "), len(args)-1, len(args))
	tag, err := s.db.Exec(ctx, q, args...)
	if err != nil {
		return mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SRSInfo is coordinate-reference-system metadata for the UI.
type SRSInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// GetSRSInfo resolves a CRS code (e.g. "4326" or "EPSG:4326") from spatial_ref_sys.
func (s *Store) GetSRSInfo(ctx context.Context, code string) (SRSInfo, error) {
	c := code
	if i := strings.LastIndex(c, ":"); i >= 0 {
		c = c[i+1:]
	}
	var srtext string
	err := s.db.QueryRow(ctx,
		`SELECT srtext FROM spatial_ref_sys WHERE srid=$1`, c).Scan(&srtext)
	if err != nil {
		return SRSInfo{Code: "EPSG:" + c}, mapErr(err)
	}
	name := srtext
	if strings.HasPrefix(srtext, "PROJCS[\"") || strings.HasPrefix(srtext, "GEOGCS[\"") {
		start := strings.Index(srtext, "\"") + 1
		end := strings.Index(srtext[start:], "\"")
		if end > 0 {
			name = srtext[start : start+end]
		}
	}
	return SRSInfo{Code: "EPSG:" + c, Name: name}, nil
}
