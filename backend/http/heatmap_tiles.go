package http

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
)

// getTileHandler returns heatmap data for a specific map tile
// URL format: /api/heatmap/tiles/{z}/{x}/{y}?source=PHOTOS&path=/2012
func getTileHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Parse tile coordinates from URL path
	// URL: /api/heatmap/tiles/{z}/{x}/{y}

	// Split by "/" and filter empty
	segments := strings.Split(r.URL.Path, "/")
	var parts []string
	for _, s := range segments {
		if s != "" {
			parts = append(parts, s)
		}
	}

	// Expected parts: [api, heatmap, tiles, z, x, y] or [heatmap, tiles, z, x, y] depending on router stripping
	// We need the LAST 3 parts.
	if len(parts) < 3 {
		logger.Error(fmt.Sprintf("Heatmap: Invalid tile path: %s (parsed: %v)", r.URL.Path, parts))
		return http.StatusBadRequest, fmt.Errorf("invalid tile path")
	}

	// Take the last 3 parts as z, x, y
	tileParts := parts[len(parts)-3:]

	var z, x, y int
	var err error

	z, err = strconv.Atoi(tileParts[0])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid zoom level: %v", err)
	}
	x, err = strconv.Atoi(tileParts[1])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid tile x: %v", err)
	}
	y, err = strconv.Atoi(tileParts[2])
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid tile y: %v", err)
	}

	// Validate tile coordinates
	// Validate tile coordinates
	maxTile := 1 << uint(z) // 2^z

	// Handle X Wrapping (Longitude)
	// If worldCopyJump is true, leaflet requests tiles like x=-1 or x=maxTile
	// We need to wrap them to 0..maxTile-1
	x = (x%maxTile + maxTile) % maxTile

	if y < 0 || y >= maxTile {
		// Y typically doesn't wrap for standard Mercator (it stops at poles approx 85deg)
		// But allow it to return empty instead of 400 if just out of bounds?
		// No, standard is 404 or empty.
		// Returning empty slice is better than error for map UX.
		return renderJSON(w, r, heatmap.HeatmapData{Clusters: []heatmap.Cluster{}})
	}

	// Get source and path from query params
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path := "/"
	if encodedPath != "" {
		path, err = url.QueryUnescape(encodedPath)
		if err != nil {
			return http.StatusBadRequest, err
		}
	}

	// Resolve scope and check permissions (same as folder heatmap)
	var scopePath, realSource string
	var userscope string

	if path != "/" || source != "" {
		scopePath, realSource, err = ResolveScopePath(d.user, source, path)
		if err != nil {
			return http.StatusForbidden, err
		}
		userscope, _, _ = settings.GetScopeFromSourceString(d.user.Scopes, source)
	} else {
		// Global view
		realSource = ""
		scopePath = ""
		userscope = "/"
	}

	// Get heatmap data based on path (with caching)
	var data heatmap.HeatmapData

	if (path == "/" && source == "") || (realSource == "" && settings.IsVirtualSource(source)) {
		// Global heatmap - Scope by User ID to prevent leaking data between users
		userKey := fmt.Sprintf("user:%v", d.user.ID)
		data, err = getCachedHeatmap(userKey, "global", func() (heatmap.HeatmapData, error) {
			globalData, gErr := heatmap.GetGlobalHeatmap(d.user)
			if gErr != nil {
				return heatmap.HeatmapData{}, gErr
			}
			return heatmap.HeatmapData{
				Clusters: globalData.Clusters,
			}, nil
		})
		if err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		// Folder heatmap (cached) - Scope by User ID + Source
		userSourceKey := fmt.Sprintf("user:%v:%s", d.user.ID, realSource)
		data, err = getCachedHeatmap(userSourceKey, scopePath, func() (heatmap.HeatmapData, error) {
			folderData, fErr := heatmap.GetFolderHeatmap(realSource, scopePath)
			if fErr != nil {
				return heatmap.HeatmapData{}, fErr
			}
			return folderData, nil
		})
		if err != nil {
			logger.Error("Failed to get folder heatmap for tile: " + err.Error())
			return http.StatusInternalServerError, err
		}
	}

	// Calculate tile bounds
	bounds := heatmap.Tile2LatLon(z, x, y)

	// Filter clusters to only those within tile bounds
	filteredClusters := heatmap.FilterClustersByBounds(data.Clusters, bounds)

	// Simplify clusters based on zoom level
	simplifiedClusters := heatmap.SimplifyClustersByZoom(filteredClusters, z)

	// Helper to round to 5 decimal places
	round5 := func(f float64) float64 {
		return float64(int(f*100000+0.5)) / 100000
	}

	logger.Debug(fmt.Sprintf("Tiles: Processing %d clusters for tile %d/%d/%d. Path='%s' UserScope='%s'", len(simplifiedClusters), z, x, y, path, userscope))

	// Create a fresh slice for the response to avoid mutating the cache
	// (SimplifyClustersByZoom might return the original slice at high zoom)
	responseClusters := make([]heatmap.Cluster, len(simplifiedClusters))

	for i, c := range simplifiedClusters {
		// Copy the cluster struct (Shallow copy is fine for non-slice fields, Points is slice but handled below)
		responseClusters[i] = c

		// DO NOT TRIM PATHS
		// Paths from heatmap.json are already correct absolute paths from source root
		logger.Debug(fmt.Sprintf("Tiles: Cluster[%d] path='%s' (keeping as-is)", i, c.Path))

		// Round coordinates to 5 decimal places for bandwidth optimization
		// This modifies the COPY in responseClusters, not the cache.
		responseClusters[i].Lat = round5(c.Lat)
		responseClusters[i].Lon = round5(c.Lon)

		// Optimization: Remove Points array to reduce JSON size.
		// Detailed file lists are mostly fetched via /api/heatmap/inspect.
		// BUT: For High Zoom (Fan View), frontend needs points.
		// Frontend Logic:
		// - Zoom < 15: Uses Badges (Points not needed)
		// - Zoom >= 15: Uses Fans (Points REQUIRED)

		if z < 15 {
			responseClusters[i].Points = nil
		}
		// If z >= 15, we keep points (passed through from SimplifyClustersByZoom)
	}

	logger.Debug(fmt.Sprintf("Tiles: Returning %d clusters for tile %d/%d/%d", len(responseClusters), z, x, y))

	// Return tile data
	tileData := map[string]interface{}{
		"clusters": responseClusters,
		"zoom":     z,
		"tile":     map[string]int{"x": x, "y": y, "z": z},
		"bounds":   bounds,
	}

	return renderJSON(w, r, tileData)
}
