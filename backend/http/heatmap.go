package http

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
)

func getGlobalHeatmapHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	data, err := heatmap.GetGlobalHeatmap(d.user)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	return renderJSON(w, r, data)
}

func getFolderHeatmapHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Resolve Scope (handles decoding loop and cross-scope logic)
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}

	// We need userscope for trimming later
	// Use original source for scope lookup logic
	userscope, _, _ := settings.GetScopeFromSourceString(d.user.Scopes, source)

	// scopePath is already the full absolute path
	fullPath := scopePath

	// Use recursive aggregation for folder view too!
	// This ensures that if I view /2016, I see clusters from /2016/Trip
	data, err := heatmap.GetFolderHeatmap(realSource, fullPath)
	if err != nil {
		logger.Error("Failed to get folder heatmap: " + err.Error())
		return http.StatusInternalServerError, err
	}

	// Trim the user scope from the paths
	if userscope != "/" {
		for i := range data.Clusters {
			data.Clusters[i].Path = strings.TrimPrefix(data.Clusters[i].Path, userscope)
			if !strings.HasPrefix(data.Clusters[i].Path, "/") && data.Clusters[i].Path != "" {
				data.Clusters[i].Path = "/" + data.Clusters[i].Path
			}
		}
	}

	return renderJSON(w, r, data)
}

func regenerateHeatmapHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Resolve Scope (security check)
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}

	// Check if already scanning
	if heatmap.IsScanning(realSource, scopePath) {
		return http.StatusConflict, fmt.Errorf("scan already in progress for %s", path)
	}

	// Start scan asynchronously so frontend can poll for progress
	go func() {
		err := heatmap.ScanSafe(realSource, scopePath)
		if err != nil {
			logger.Error("Heatmap scan failed: " + err.Error())
		}
	}()

	return renderJSON(w, r, map[string]string{"message": "Heatmap regeneration started"})
}

func getHeatmapStatusHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	// Resolve Scope
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}

	isScanning := heatmap.IsScanning(realSource, scopePath)
	progress := heatmap.GetScanProgress(realSource, scopePath)

	resp := map[string]interface{}{
		"isScanning": isScanning,
		"progress":   progress,
	}
	return renderJSON(w, r, resp)
}
