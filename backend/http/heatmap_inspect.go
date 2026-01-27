package http

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
)

// handleInspect returns the list of files for a specific cluster location
// URL format: /api/heatmap/inspect?lat=...&lon=...&source=...&path=...
func handleInspect(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// 1. Parse coordinates
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")
	if latStr == "" || lonStr == "" {
		return http.StatusBadRequest, fmt.Errorf("missing lat/lon parameter")
	}

	targetLat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid lat parameter")
	}
	targetLon, err := strconv.ParseFloat(lonStr, 64)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid lon parameter")
	}

	// 2. Resolve Source and Path (Scope Logic)
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path := "/"
	if encodedPath != "" {
		p, err := url.QueryUnescape(encodedPath)
		if err == nil {
			path = p
		}
	}

	var scopePath, realSource string
	var userscope string

	if path != "/" || source != "" {
		scopePath, realSource, err = ResolveScopePath(d.user, source, path)
		if err != nil {
			return http.StatusForbidden, err
		}

		// Fix: If path is a file (e.g. inspecting a single image), scopePath will be the file path.
		// GetFolderHeatmap needs a DIRECTORY.
		// We optimistically check extension. If it has one, assume file and take Dir.
		if filepath.Ext(scopePath) != "" {
			scopePath = filepath.Dir(scopePath)
			// Normalize separators to slash
			scopePath = filepath.ToSlash(scopePath)
		}

		userscope, _, _ = settings.GetScopeFromSourceString(d.user.Scopes, source)
	} else {
		// Global view
		realSource = ""
		scopePath = ""
		userscope = "/"
	}

	// 3. Retrieve Data from Cache (or load it)
	var data heatmap.HeatmapData

	if path == "/" && source == "" {
		// Global heatmap
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
	} else {
		// Folder heatmap
		userSourceKey := fmt.Sprintf("user:%v:%s", d.user.ID, realSource)
		data, err = getCachedHeatmap(userSourceKey, scopePath, func() (heatmap.HeatmapData, error) {
			folderData, fErr := heatmap.GetFolderHeatmap(realSource, scopePath)
			if fErr != nil {
				return heatmap.HeatmapData{}, fErr
			}
			return folderData, nil
		})
	}

	if err != nil {
		logger.Error("Heatmap Inspect: Failed to get data: " + err.Error())
		return http.StatusInternalServerError, err
	}

	// 4. Calculate Search Radius or Use Tile Bounds
	zoomStr := r.URL.Query().Get("zoom")
	zoom := 18
	if zoomStr != "" {
		z, err := strconv.Atoi(zoomStr)
		if err == nil {
			zoom = z
		}
	}

	// Check for explicit Cluster ID (Hidden PK)
	targetClusterID := r.URL.Query().Get("cluster_id")

	var matchPoints []heatmap.ClusterPoint

	// Define radius variable in outer scope for error reporting fallback
	var radius float64

	if targetClusterID != "" {
		// EXACT MATCH MODE
		found := false
		logger.Debug(fmt.Sprintf("Inspect: Searching for Cluster ID %s in %d clusters", targetClusterID, len(data.Clusters)))

		for _, c := range data.Clusters {
			if c.ID == targetClusterID {
				// "Drill Down" Inspection:
				// If this is an aggregated cluster (Points stripped), fetch from the source leaf folder.
				if len(c.Points) == 0 && c.Count > 0 && c.Path != "" {
					leafFolder := filepath.ToSlash(filepath.Dir(c.Path))
					logger.Debug(fmt.Sprintf("Inspect: Found Target Cluster. Path='%s' Leaf='%s'", c.Path, leafFolder))

					// DRILL DOWN LOGIC:
					// If the requested scope is NOT the leaf folder (i.e. we are higher up),
					// return a "Folder Node" instead of the images.
					// This allows the user to see which folder this cluster belongs to.
					// Normalize paths for comparison (trim trailing slashes)
					cleanScope := strings.TrimRight(scopePath, "/")
					cleanLeaf := strings.TrimRight(leafFolder, "/")

					logger.Debug(fmt.Sprintf("Inspect: Scope='%s' CleanScope='%s' CleanLeaf='%s'", scopePath, cleanScope, cleanLeaf))

					// If we are at Global (empty cleanScope) or parent, return folder.
					// Only hydrate if we are specifically inspecting the leaf folder.
					// WAIT: The user wants IMMEDIATE drill down.
					// "The clusterid is ... and only has one image. Currently the inspection panel is showing every image in that folder."
					// So if we find the cluster ID, we SHOULD return the points, regardless of scope level?
					// The user complained about "It does not seem to be drilling down...".
					// So I will ALWAYS hydrate if ID matches.

					/*
						if cleanScope != cleanLeaf {
							logger.Debug(fmt.Sprintf("Inspect: returning folder node for %s (Leaf: %s)", c.ID, leafFolder))
							finalPoints := []heatmap.ClusterPoint{{
								Type:      "folder",
								Path:      leafFolder,
								Count:     c.Count,
								ID:        c.ID, // Pass same ID to allow drill-down
								PreviewID: c.PreviewID,
								Lat:       c.Lat,
								Lon:       c.Lon,
								Source:    realSource, // Ensure source is passed
							}}
							return renderJSON(w, r, finalPoints)
						}
					*/

					logger.Debug(fmt.Sprintf("Inspect: Hydrating aggregated cluster %s from leaf %s", c.ID, leafFolder))
					// Load leaf heatmap (bypass cache for simplicity or use logic)
					leafData, err := heatmap.GetFolderHeatmap(realSource, leafFolder)
					if err == nil {
						logger.Debug(fmt.Sprintf("Inspect: Loaded %d clusters from leaf", len(leafData.Clusters)))
						for _, lc := range leafData.Clusters {
							// ID matches are preserved during aggregation
							if lc.ID == c.ID {
								matchPoints = lc.Points
								logger.Debug(fmt.Sprintf("Inspect: Hydrated %d points from leaf via ID Match", len(matchPoints)))
								break
							}
						}
					} else {
						logger.Error("Inspect: Failed to load leaf data: " + err.Error())
					}
				} else {
					matchPoints = c.Points
					logger.Debug(fmt.Sprintf("Inspect: Using existing points (Count=%d)", len(matchPoints)))
				}

				// Post-hydration check
				if len(matchPoints) == 0 {
					logger.Info("Inspect: Match found but points empty. Falling back to Folder Node.")
					// If Hydration failed (e.g. leaf load error, or empty leaf), fall back to returning the folder information itself.
					// This ensures the frontend doesn't get a 404 and can at least show the folder.
					matchPoints = append(matchPoints, heatmap.ClusterPoint{
						Type:      "folder",
						Path:      c.Path, // Points to the file or folder
						Count:     c.Count,
						ID:        c.ID,
						PreviewID: c.PreviewID,
						Lat:       c.Lat,
						Lon:       c.Lon,
						Source:    c.Source,
					})
				}

				found = true
				logger.Debug(fmt.Sprintf("Inspect: Exact Match for Cluster ID=%s Count=%d", targetClusterID, len(matchPoints)))
				break
			}
		}
		if !found {
			// Fallback or Error?
			// If ID provided but not found, it might be stale data.
			// We could fallback to location search, but let's log it.
			logger.Info(fmt.Sprintf("Inspect: Cluster ID %s not found in current view. Falling back to spatial search.", targetClusterID))
		}
	}

	// Fallback to Spatial/Tile search if no ID matched
	if len(matchPoints) == 0 {
		// STRICT TILE INSPECTION MODE
		// Ensure we match the exact logic of the tile cache for consistent counts.
		// 1. Determine which tile (X, Y) contains the target Lat/Lon at the requested Zoom.
		tileX, tileY := heatmap.LatLon2Tile(targetLat, targetLon, zoom)

		// 2. Get the exact bounds of that tile.
		tileBounds := heatmap.Tile2LatLon(zoom, tileX, tileY)

		// 3. Filter all clusters to ONLY those within this tile.
		filteredClusters := heatmap.FilterClustersByBounds(data.Clusters, tileBounds)

		// 4. Now collect points from these clusters.

		logger.Debug(fmt.Sprintf("Inspect: Target=%f,%f Zoom=%d Tile=%d/%d Bounds=%v",
			targetLat, targetLon, zoom, tileX, tileY, tileBounds))

		pixelRadius := 120.0
		radius = pixelRadius * 360.0 / (256.0 * math.Pow(2, float64(zoom)))
		if radius < 0.0006 {
			radius = 0.0006
		}

		for _, c := range filteredClusters {

			// We are iterating ONLY clusters that are verified to be inside the tile.

			dLat := c.Lat - targetLat
			dLon := c.Lon - targetLon
			distSq := dLat*dLat + dLon*dLon

			// Use a generous matching radius within the tile to find the intended target.
			// Since we've already isolated the tile, we are less likely to pick up "next tile over" junk.
			// We can just take everything in the tile?
			// "It should match the number provided by the vector tile cache"
			// implies the sum of the badge count?
			// If the badge says "5", and the tile has 5 items total, then we want all 5.
			// If the tile has 10 items (two badges of 5), we only want one of them.

			// Use the radius check to pick the right cluster within the tile.
			isMatch := false

			// Check Centroid distance OR Bounding Box inclusion
			inBounds := targetLat >= c.Min[0]-radius && targetLat <= c.Max[0]+radius &&
				targetLon >= c.Min[1]-radius && targetLon <= c.Max[1]+radius

			if distSq < radius*radius || inBounds {
				isMatch = true
			}

			if isMatch {
				if len(c.Points) > 0 {
					matchPoints = append(matchPoints, c.Points...)
				} else if c.Count > 0 {
					// Aggregated Cluster (Folder Node) OR Drill-Down Candidate
					// Attempt to hydrate from source leaf if possible.
					hydrated := false

					// "Drill Down" - Attempt to load the actual images from the child folder
					// This allows clicking "Inspect" on a single-image cluster at high zoom (but still aggregated)
					// and seeing the image instead of a folder icon.
					if c.Path != "" {
						// Load leaf heatmap (bypass cache for simplicity, it's fast)
						// Note: c.Path in an aggregated cluster usually points to the folder containing the items.
						// or the item itself?
						// In Manager.go, aggregated clusters take the path of the child.
						targetPath := c.Path
						// If path is a file, take dir? We can try both or assume it's a folder path if it has children.
						// But c.Path for a file-cluster might be the file path.
						// heatmap.GetFolderHeatmap ignores filenames usually? No, it relies on it being a directory.

						// Try treating c.Path as the folder (safe default for aggregated nodes)
						leafData, err := heatmap.GetFolderHeatmap(c.Source, targetPath)

						// If that fails or returns 0 clusters, and it looks like a file path, try Dir?
						if (err != nil || len(leafData.Clusters) == 0) && filepath.Ext(targetPath) != "" {
							targetPath = filepath.Dir(targetPath)
							leafData, err = heatmap.GetFolderHeatmap(c.Source, targetPath)
						}

						if err == nil {
							for _, lc := range leafData.Clusters {
								// 1. Try Exact ID Match (Fastest)
								if lc.ID == c.ID {
									if len(lc.Points) > 0 {
										matchPoints = append(matchPoints, lc.Points...)
										hydrated = true
										logger.Debug(fmt.Sprintf("Inspect: Hydrated %d points for ID %s from leaf", len(lc.Points), c.ID))
									}
									break
								}
							}

							// 2. Fallback: Spatial Match in Leaf (Robustness for stale IDs)
							// If ID logic fails (e.g. heatmap regenerated with new IDs but top-level didn't update yet),
							// we search for the cluster at the EXACT same location in the leaf data.
							if !hydrated {
								// Tiny delta for "exact" location match (float precision safety)
								delta := 0.00001
								for _, lc := range leafData.Clusters {
									dLat := math.Abs(lc.Lat - c.Lat)
									dLon := math.Abs(lc.Lon - c.Lon)

									if dLat < delta && dLon < delta {
										// Found the cluster physically!
										if len(lc.Points) > 0 {
											matchPoints = append(matchPoints, lc.Points...)
											hydrated = true
											logger.Debug(fmt.Sprintf("Inspect: Hydrated %d points via Spatial Match (ID %s -> %s)", len(lc.Points), c.ID, lc.ID))
										}
										break
									}
								}
							}
						}
					}

					if !hydrated {
						// Fallback: Return as "Folder Point" so frontend lists it as a group
						// UNLESS it's a file (extension exists), in which case return as file so it groups by parent.
						itemType := "folder"
						if filepath.Ext(c.Path) != "" {
							itemType = "" // Empty type = file/image in frontend logic
						}

						logger.Debug(fmt.Sprintf("Inspect: Returning fallback node for %s (Path: %s) Type: %s", c.ID, c.Path, itemType))
						matchPoints = append(matchPoints, heatmap.ClusterPoint{
							Type:      itemType,
							Path:      c.Path,
							Count:     c.Count,
							ID:        c.ID,
							PreviewID: c.PreviewID,
							Lat:       c.Lat,
							Lon:       c.Lon,
							Source:    c.Source,
						})
					}
				}
			}
		}

	}

	if len(matchPoints) == 0 {
		return http.StatusNotFound, fmt.Errorf("no cluster found at this location (radius: %f)", radius)
	}

	logger.Debug(fmt.Sprintf("Inspect: Found %d points before trimming. UserScope='%s'", len(matchPoints), userscope))

	// 5. Filter/Sanitize Result
	finalPoints := make([]heatmap.ClusterPoint, len(matchPoints))
	for i, p := range matchPoints {
		newP := p
		originalPath := p.Path

		// Apply trimming if needed
		if userscope != "/" {
			// CONSERVATIVE Trimming: Only trim if path EXACTLY starts with scope
			// This prevents over-aggressive trimming that breaks paths
			if strings.HasPrefix(newP.Path, userscope) {
				newP.Path = strings.TrimPrefix(newP.Path, userscope)
				logger.Debug(fmt.Sprintf("Inspect: Trimmed path '%s' -> '%s' (exact prefix match)", originalPath, newP.Path))
			} else {
				// Case-insensitive prefix check as fallback
				pathLower := strings.ToLower(newP.Path)
				scopeLower := strings.ToLower(userscope)

				if strings.HasPrefix(pathLower, scopeLower) {
					// Preserve original case but trim the scope length
					newP.Path = newP.Path[len(userscope):]
					logger.Debug(fmt.Sprintf("Inspect: Trimmed path '%s' -> '%s' (case-insensitive prefix)", originalPath, newP.Path))
				} else {
					// NO SUBSTRING MATCHING - this was causing incorrect trimming
					// If scope is not a prefix, return the path as-is
					logger.Debug(fmt.Sprintf("Inspect: Path '%s' does not start with scope '%s', keeping as-is", originalPath, userscope))
				}
			}

			// Ensure leading slash
			if !strings.HasPrefix(newP.Path, "/") && newP.Path != "" {
				newP.Path = "/" + newP.Path
			}
		}

		finalPoints[i] = newP

		logger.Debug(fmt.Sprintf("Inspect: Final path[%d]: '%s' (source: '%s') Count=%d Type='%s'", i, newP.Path, newP.Source, newP.Count, newP.Type))
	}

	logger.Debug(fmt.Sprintf("Inspect: Returning %d points to frontend", len(finalPoints)))
	return renderJSON(w, r, finalPoints)
}
