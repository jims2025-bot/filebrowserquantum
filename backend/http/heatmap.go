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
		logger.Error("Heatmap: Invalid path (unescape failed): " + encodedPath)
		return http.StatusBadRequest, err
	}

	// Resolve Scope (handles decoding loop and cross-scope logic)
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}

	// Restore userscope for path trimming logic later
	// We use the original source string because that's what the scopes are mapped to (or aliases)
	userscope, _, _ := settings.GetScopeFromSourceString(d.user.Scopes, source)

	// FIX: For Admin, treat scope as Root to avoid trimming/matching issues with absolute paths
	if d.user.Permissions.Admin {
		userscope = "/"
	}

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
	// Trim the user scope from the paths
	if userscope != "/" {
		// Debug the trim operation
		if len(data.Clusters) > 0 {
			logger.Info(fmt.Sprintf("Heatmap: Trimming scope '%s' from path '%s'", userscope, data.Clusters[0].Path))
		}

		// Ensure we are comparing apples to apples (slashes)
		scope := strings.TrimRight(userscope, "/")
		for i := range data.Clusters {
			// Aggressive Trimming (Same as Tiles)
			if strings.HasPrefix(data.Clusters[i].Path, scope) {
				data.Clusters[i].Path = strings.TrimPrefix(data.Clusters[i].Path, scope)
			} else {
				// Substring match for nested scopes (Case Insensitive)
				cleanScope := strings.TrimPrefix(scope, "/")
				if idx := strings.Index(strings.ToLower(data.Clusters[i].Path), strings.ToLower(cleanScope)); idx != -1 {
					remainder := data.Clusters[i].Path[idx+len(cleanScope):]
					logger.Debug(fmt.Sprintf("Heatmap: Stripped parent from '%s' -> '%s'", data.Clusters[i].Path, remainder))
					data.Clusters[i].Path = remainder
				}
			}

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

	// FIX: If source is virtual, we must resolve the physical source from the path
	// because the heatmap scanner needs a physical index.
	if settings.IsVirtualSource(realSource) {
		if physicalSource, relPath, ok := settings.GetSourceFromPath(scopePath); ok {
			realSource = physicalSource
			scopePath = relPath
			logger.Debugf("Heatmap Regen: Resolved virtual source to physical: %s -> %s", physicalSource, relPath)
		} else {
			logger.Error(fmt.Sprintf("Heatmap Regen: Failed to resolve physical source for virtual path: %s", scopePath))
			return http.StatusBadRequest, fmt.Errorf("cannot regenerate heatmap for virtual root or unresolved path")
		}
	} else {
		logger.Debugf("Heatmap Regen: Physical source used directly: %s -> %s", realSource, scopePath)
	}

	// Check if already scanning
	if heatmap.IsScanning(realSource, scopePath) {
		return http.StatusConflict, fmt.Errorf("scan already in progress for %s", path)
	}

	// Start scan asynchronously so frontend can poll for progress
	go func() {
		logger.Debugf("Heatmap Regen: Starting ScanSafe async for %s %s", realSource, scopePath)
		err := heatmap.ScanSafe(realSource, scopePath, true) // Manual scan - force rebuild
		if err != nil {
			logger.Error("Heatmap scan failed: " + err.Error())
		} else {
			// CRITICAL FIX: Clear the cache so tiles update immediately with new Cluster IDs
			// The tile cache keys are formatted as "user:ID:SOURCE"
			userSourceKey := fmt.Sprintf("user:%v:%s", d.user.ID, realSource)
			ClearHeatmapCache(userSourceKey, scopePath)
			logger.Info("Heatmap regenerated and cache cleared for: " + scopePath)
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

	// FIX: If source is virtual, resolve physical source/path
	if settings.IsVirtualSource(realSource) {
		if physicalSource, relPath, ok := settings.GetSourceFromPath(scopePath); ok {
			realSource = physicalSource
			scopePath = relPath
		}
	}

	isScanning := heatmap.IsScanning(realSource, scopePath)
	progress := heatmap.GetScanProgress(realSource, scopePath)

	if isScanning {
		logger.Debugf("Heatmap Status: Scanning IS active for %s %s", realSource, scopePath)
	} else {
		// logger.Debugf("Heatmap Status: Scanning NOT active for %s %s", realSource, scopePath)
	}

	resp := map[string]interface{}{
		"isScanning": isScanning,
		"progress":   progress,
	}
	return renderJSON(w, r, resp)
}
