package store

import (
	"context"
	"encoding/json"
)

// GroupMember is one ordered entry (layer + optional style) of a layer group.
type GroupMember struct {
	Layer string `json:"layer"`
	Style string `json:"style"`
}

// GroupFull is the admin view of a layer group.
type GroupFull struct {
	Workspace string        `json:"workspace"`
	Name      string        `json:"name"`
	Title     string        `json:"title"`
	Abstract  string        `json:"abstract"`
	Mode      string        `json:"mode"`
	SRS       string        `json:"srs"`
	Bounds    []float64     `json:"bounds,omitempty"`
	Members   []GroupMember `json:"members"`
}

// SaveGroup upserts a layer group with ordered members.
func (s *Store) SaveGroup(ctx context.Context, g GroupFull) error {
	if g.Members == nil {
		g.Members = []GroupMember{}
	}
	members, _ := json.Marshal(g.Members)
	bounds, _ := json.Marshal(g.Bounds)
	names := make([]string, len(g.Members))
	for i, m := range g.Members {
		names[i] = m.Layer
	}
	if g.Mode == "" {
		g.Mode = "SINGLE"
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO layer_groups(workspace, name, mode, layers, title, abstract, srs, bounds, members)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (workspace, name) DO UPDATE SET
			mode=$3, layers=$4, title=$5, abstract=$6, srs=$7, bounds=$8, members=$9`,
		g.Workspace, g.Name, g.Mode, names, g.Title, g.Abstract, g.SRS, bounds, members)
	return mapErr(err)
}

// GetGroupFull reads a layer group with its ordered members.
func (s *Store) GetGroupFull(ctx context.Context, ws, name string) (GroupFull, error) {
	var g GroupFull
	var members, bounds []byte
	err := s.db.QueryRow(ctx, `
		SELECT workspace, name, mode, title, abstract, srs, bounds, members
		FROM layer_groups WHERE workspace=$1 AND name=$2`, ws, name).
		Scan(&g.Workspace, &g.Name, &g.Mode, &g.Title, &g.Abstract, &g.SRS, &bounds, &members)
	if err != nil {
		return g, mapErr(err)
	}
	json.Unmarshal(members, &g.Members)
	json.Unmarshal(bounds, &g.Bounds)
	if g.Members == nil {
		g.Members = []GroupMember{}
	}
	return g, nil
}

// ListGroupsFull lists all layer groups (all workspaces) for the admin UI.
func (s *Store) ListGroupsFull(ctx context.Context) ([]GroupFull, error) {
	rows, err := s.db.Query(ctx, `
		SELECT workspace, name, mode, title, abstract, srs, bounds, members
		FROM layer_groups ORDER BY workspace, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GroupFull{}
	for rows.Next() {
		var g GroupFull
		var members, bounds []byte
		if err := rows.Scan(&g.Workspace, &g.Name, &g.Mode, &g.Title, &g.Abstract, &g.SRS, &bounds, &members); err != nil {
			return nil, err
		}
		json.Unmarshal(members, &g.Members)
		json.Unmarshal(bounds, &g.Bounds)
		if g.Members == nil {
			g.Members = []GroupMember{}
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// ComputeGroupBounds unions the EPSG:4326 bboxes of a group's member layers.
func (s *Store) ComputeGroupBounds(ctx context.Context, ws, name string) ([]float64, error) {
	g, err := s.GetGroupFull(ctx, ws, name)
	if err != nil {
		return nil, err
	}
	var b []float64
	for _, m := range g.Members {
		mws, mname := splitLayerRef(m.Layer, ws)
		d, err := s.GetLayerDetail(ctx, mws, mname)
		if err != nil || len(d.Bbox) != 4 {
			continue
		}
		if b == nil {
			b = append([]float64{}, d.Bbox...)
			continue
		}
		if d.Bbox[0] < b[0] {
			b[0] = d.Bbox[0]
		}
		if d.Bbox[1] < b[1] {
			b[1] = d.Bbox[1]
		}
		if d.Bbox[2] > b[2] {
			b[2] = d.Bbox[2]
		}
		if d.Bbox[3] > b[3] {
			b[3] = d.Bbox[3]
		}
	}
	return b, nil
}

// splitLayerRef parses "ws:name" or bare "name" (defaulting to defWs).
func splitLayerRef(ref, defWs string) (string, string) {
	for i := 0; i < len(ref); i++ {
		if ref[i] == ':' {
			return ref[:i], ref[i+1:]
		}
	}
	return defWs, ref
}
