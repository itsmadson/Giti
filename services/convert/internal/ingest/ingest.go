// Package ingest copies uploaded spatial files and publishes them as layers
// via the catalog REST API.
package ingest

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type Result struct {
	Workspace, Store, Layer, StoredPath string
}

// DetectType maps a filename extension to a Giti store type.
func DetectType(filename string) (string, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".shp":
		return "Shapefile", nil
	case ".gpkg":
		return "GeoPackage", nil
	case ".geojson", ".json":
		return "GeoJSON", nil
	case ".csv":
		return "CSV", nil
	case ".tif", ".tiff":
		return "GeoTIFF", nil
	default:
		return "", fmt.Errorf("unsupported file type %q", filepath.Ext(filename))
	}
}

func base(filename string) string {
	return strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
}

// Import copies data into dataDir/{workspace}/{filename}, then registers a
// store + featuretype via the catalog REST API (auto-publishing a layer).
func Import(ctx context.Context, catalogURL, dataDir, workspace, filename string,
	data []byte, progress func(string)) (Result, error) {

	storeType, err := DetectType(filename)
	if err != nil {
		return Result{}, err
	}
	name := base(filename)

	progress("storing file")
	dir := filepath.Join(dataDir, workspace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, err
	}
	stored := filepath.Join(dir, filepath.Base(filename))
	if err := os.WriteFile(stored, data, 0o644); err != nil {
		return Result{}, err
	}

	c := &http.Client{}

	progress("creating workspace")
	post(ctx, c, catalogURL+"/rest/workspaces", "application/xml",
		"<workspace><name>"+workspace+"</name></workspace>")

	progress("creating store")
	storeBody := fmt.Sprintf(
		`<dataStore><name>%s</name><type>%s</type><enabled>true</enabled>`+
			`<connectionParameters><entry key="url">file://%s</entry></connectionParameters></dataStore>`,
		name, storeType, stored)
	if _, err := post(ctx, c, catalogURL+"/rest/workspaces/"+workspace+"/datastores",
		"application/xml", storeBody); err != nil {
		return Result{}, err
	}

	progress("publishing layer")
	ftBody := "<featureType><name>" + name + "</name><enabled>true</enabled></featureType>"
	if _, err := post(ctx, c,
		catalogURL+"/rest/workspaces/"+workspace+"/datastores/"+name+"/featuretypes",
		"application/xml", ftBody); err != nil {
		return Result{}, err
	}

	progress("done")
	return Result{Workspace: workspace, Store: name, Layer: name, StoredPath: stored}, nil
}

// post sends a request; 200/201/409 are treated as success.
func post(ctx context.Context, c *http.Client, url, ct, body string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", ct)
	resp, err := c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusConflict {
		return resp.StatusCode, fmt.Errorf("catalog %s returned %d", url, resp.StatusCode)
	}
	return resp.StatusCode, nil
}
