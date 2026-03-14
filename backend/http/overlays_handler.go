package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

type OverlayResponse struct {
	Overlays []heatmap.MapOverlay `json:"overlays"`
	Path     string               `json:"path"`
}

// HandleGetOverlays serves the mapoverlays.json for a specific scope/path.
// Query Params:
//   - source: Source Name
//   - path: Folder Path (defaults to root)
//   - mode: "all" (Global/Root) vs "context" (Current Folder).
//     Actually, since mapoverlays.json ALREADY aggregates children,
//     "all" = get file at Root "/" (or source root).
//     Backend just needs to serve the file at the requested target path.
func HandleGetOverlays(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	sourceName := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	mode := r.URL.Query().Get("mode")

	if sourceName == "" {
		return http.StatusBadRequest, fmt.Errorf("source is required")
	}

	// Handle pseudo-sources: Aggregate overlays across all visible scopes
	if settings.IsVirtualSource(sourceName) {
		forceScan := r.URL.Query().Get("scan") == "true"
		allOverlays := []heatmap.MapOverlay{}
		visited := make(map[string]bool)

		type sourceInfo struct {
			Name  string
			Alias string
		}
		var sources []sourceInfo

		if d.user.Permissions.Admin {
			for name := range indexing.GetIndexes() {
				sources = append(sources, sourceInfo{Name: name, Alias: name})
			}
		} else {
			for _, s := range d.user.Scopes {
				sources = append(sources, sourceInfo{Name: s.Name, Alias: s.Alias})
			}
		}

		for _, s := range sources {
			idx := indexing.GetIndex(s.Name)
			if idx == nil {
				continue
			}

			// Determine target path for this scope
			target := path
			if target == "" {
				target = "/"
			}

			// Resolve relative to THIS specific scope (to handle nested shares/permissions correctly)
			// But for "ALL" aggregation, we usually want the same path across all.
			// RESOLUTION: Use the absolute path if possible.
			// Resolve target path for this specific scope/source
			resolvedTarget, _, err := ResolveScopePath(d.user, s.Name, path)
			if err != nil {
				continue
			}

			if forceScan {
				logger.Info(fmt.Sprintf("[Overlays] Force scanning virtual sub-source %s at path %s (original: %s)", s.Name, resolvedTarget, path))
				heatmap.AggregateOverlayLevel(idx.Name, resolvedTarget)
			}

			realPath, _, err := idx.GetRealPath(resolvedTarget)
			if err != nil {
				// logger.Debug(fmt.Sprintf("[Overlays] Target path %s not found in scope %s", resolvedTarget, s.Name))
				continue
			}

			overlayPath := filepath.Join(realPath, heatmap.MapOverlaysFilename)
			dedupKey := strings.ToLower(filepath.Clean(overlayPath))
			if visited[dedupKey] {
				continue
			}
			visited[dedupKey] = true

			if _, err := os.Stat(overlayPath); err == nil {
				bytes, err := os.ReadFile(overlayPath)
				if err == nil {
					var data heatmap.OverlayData
					if err := json.Unmarshal(bytes, &data); err == nil {
						logger.Info(fmt.Sprintf("[Overlays] Found %d overlays in %s for scope %s", len(data.Overlays), overlayPath, s.Name))
						for _, ov := range data.Overlays {
							// Inject source alias so the frontend knows which source to use for /api/raw
							ov.Source = s.Alias
							if ov.Source == "" {
								ov.Source = s.Name
							}
							allOverlays = append(allOverlays, ov)
						}
					}
				}
			}
		}

		return renderJSON(w, r, OverlayResponse{
			Overlays: allOverlays,
			Path:     path,
		})
	}

	var idx *indexing.Index

	// Optimization: Resolve Alias First to avoid "Index Not Found" logs
	if strings.Contains(sourceName, ":") {
		parts := strings.Split(sourceName, ":")
		if len(parts) > 0 {
			idx = indexing.GetIndex(parts[0])
		}
	}

	// Fallback/Normal Lookup
	if idx == nil {
		idx = indexing.GetIndex(sourceName)
	}

	// Retry with URL decoding if needed
	if idx == nil {
		if decoded, err := url.QueryUnescape(sourceName); err == nil && decoded != sourceName {
			if strings.Contains(decoded, ":") {
				parts := strings.Split(decoded, ":")
				if len(parts) > 0 {
					idx = indexing.GetIndex(parts[0])
				}
				if idx == nil {
					idx = indexing.GetIndex(decoded)
				}
			}
		}
	}

	// Fix for Initial Load where Source = Scope Alias (e.g. SHANTON-VA)
	// If direct index lookup failed, check if the user has a scope with this alias.
	if idx == nil && d.user != nil {
		for _, s := range d.user.Scopes {
			// Check if the requested source matches the Scope Alias
			if strings.EqualFold(s.Alias, sourceName) || strings.EqualFold(s.Name, sourceName) {
				// Use the underlying Source Name (which is usually the path or ID) for the index lookup
				// But wait, s.Name in settings.go is likely the Source Path/ID.
				// Let's resolve the Index by s.Name
				potentialIdx := indexing.GetIndex(s.Name)
				if potentialIdx != nil {
					idx = potentialIdx
					// logger.Info("HandleGetOverlays: Resolved source match via User Scope: " + sourceName + " -> " + s.Name)
					break
				}
			}
		}
	}

	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("index not found")
	}

	// Determine effective path based on mode
	targetPath := path
	if targetPath == "" {
		targetPath = "/"
	}

	// Fix: Resolve path relative to User Scope for "Current Folder" mode
	if mode != "all" && d.user != nil {
		resolvedPath, _, err := ResolveScopePath(d.user, sourceName, targetPath)
		if err == nil {
			targetPath = resolvedPath
		} else {
			logger.Error("HandleGetOverlays: ResolveScopePath failed: " + err.Error())
		}
	}

	if mode == "all" {
		targetPath = "/"

		// Use the User's Scope Root if available
		// This ensures that "All Overlays" respects the user's view (e.g. starting at /PHOTOCOLLECTIONS)
		// rather than always showing the physical root of the Source.
		if d.user != nil {
			userScope, _, err := settings.GetScopeFromSourceString(d.user.Scopes, sourceName)
			if err == nil && userScope != "" {
				targetPath = userScope
			}
		}
	}

	// Manual Scan Trigger (Debugging)
	if r.URL.Query().Get("scan") == "true" {
		logger.Info("HandleGetOverlays: Force scanning " + targetPath)
		_, err := heatmap.AggregateOverlayLevel(idx.Name, targetPath)
		if err != nil {
			logger.Error("HandleGetOverlays: Scan failed: " + err.Error())
		}
	}

	realPath, _, err := idx.GetRealPath(targetPath)
	if err != nil {
		// If path doesn't exist, we can't give overlays
		return http.StatusNotFound, fmt.Errorf("path not found")
	}

	overlayPath := filepath.Join(realPath, heatmap.MapOverlaysFilename)

	// If file doesn't exist, return empty list (not error)
	if _, err := os.Stat(overlayPath); err != nil {
		return renderJSON(w, r, OverlayResponse{
			Overlays: []heatmap.MapOverlay{},
			Path:     targetPath,
		})
	}

	bytes, err := os.ReadFile(overlayPath)
	if err != nil {
		logger.Error("HandleGetOverlays: ReadFile failed: " + err.Error())
		return http.StatusInternalServerError, fmt.Errorf("failed to read overlays")
	}

	// We can serve the raw file, but let's wrap it or parse/validate
	// The file contains { generated_at, overlays: [] }
	// We might want to just return the list.
	var data heatmap.OverlayData
	if err := json.Unmarshal(bytes, &data); err != nil {
		logger.Error("HandleGetOverlays: Unmarshal failed: " + err.Error())
		return http.StatusInternalServerError, fmt.Errorf("corrupt overlay data")
	}

	response := OverlayResponse{
		Overlays: data.Overlays,
		Path:     targetPath,
	}

	return renderJSON(w, r, response)
}

// HandleUpdateOverlayMetadata updates the persistent metadata for an overlay.
// PUT /api/heatmap/overlay/metadata
func HandleUpdateOverlayMetadata(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.ManageOverlays && !d.user.Permissions.Admin {
		return http.StatusForbidden, nil
	}

	var req struct {
		Source      string `json:"source"`
		Path        string `json:"path"` // Relative to source root
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}

	if req.Source == "" || req.Path == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}

	// Resolve source and path to handle aliases or virtual sources (e.g. ALL_SOURCES)
	resolvedPath, realSource, err := ResolveScopePath(d.user, req.Source, req.Path)
	if err != nil {
		logger.Error(fmt.Sprintf("[Overlays] ResolveScopePath failed: %v", err))
		return http.StatusInternalServerError, err
	}

	logger.Info(fmt.Sprintf("[Overlays] Updating metadata for %s in source %s (resolved: %s, %s)", req.Path, req.Source, realSource, resolvedPath))

	err = heatmap.SaveOverlayMetadata(realSource, resolvedPath, req.Name, req.Description)
	if err != nil {
		logger.Error(fmt.Sprintf("[Overlays] Failed to save metadata: %v", err))
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
