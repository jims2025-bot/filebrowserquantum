package http

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
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
		// CRITICAL FIX: Bypass cache for folder inspection to ensure we get the latest IDs.
		// The http-layer cache might be stale if the manager updated heatmap.json recently.
		// Since folder heatmaps are small, this performance hit is negligible.
		logger.Debug("Inspect: Bypassing cache for folder heatmap: " + scopePath)
		data, err = heatmap.GetFolderHeatmap(realSource, scopePath)
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

	// Helper to robustly get ID (Moved up for shared use)
	getOrGenID := func(c *heatmap.Cluster) string {
		if c.ID != "" {
			return c.ID
		}
		h := fnv.New64a()
		h.Write([]byte(c.Path))
		h.Write([]byte(fmt.Sprintf("%.6f,%.6f", c.Lat, c.Lon)))
		return fmt.Sprintf("gen-%x", h.Sum64())
	}

	var matchPoints []heatmap.ClusterPoint

	// CRITICAL: Direct Path Inspection Mode
	if path != "" && targetClusterID == "" && (targetLat == 0 && targetLon == 0) {
		// Use manual loading to bypass any cache
		leafData, err := heatmap.GetFolderHeatmap(realSource, path)
		if err == nil {
			// Add warning message
			warnMsg := "Warning: Showing all files in folder (Cluster ID context missing)"
			matchPoints = append(matchPoints, heatmap.ClusterPoint{
				Type:      "file",
				Path:      filepath.ToSlash(filepath.Join(path, warnMsg)),
				Count:     1,
				ID:        "warning-all-files",
				PreviewID: "",
				Source:    realSource,
			})

			// Flatten ALL clusters in this folder
			for i, c := range leafData.Clusters {
				cID := getOrGenID(&c)
				logger.Debug(fmt.Sprintf("Direct Path: Cluster %d Pts=%d Path=%s ID=%s", i, len(c.Points), c.Path, cID))

				if len(c.Points) > 0 {
					// INJECT ID and FILTER heatmap.json
					for _, p := range c.Points {
						if filepath.Base(p.Path) == "heatmap.json" {
							continue
						}
						// Propagate Parent Cluster ID
						p.ID = cID
						matchPoints = append(matchPoints, p)
					}
				} else if c.Count > 0 {
					// Fallback for empty clusters
					itemType := "file"
					if len(c.Points) == 0 && c.Count > 0 && filepath.Ext(c.Path) == "" {
						itemType = "folder"
					}
					matchPoints = append(matchPoints, heatmap.ClusterPoint{
						ID:        cID,
						Lat:       c.Lat,
						Lon:       c.Lon,
						Count:     c.Count,
						Path:      c.Path,
						PreviewID: c.PreviewID,
						Type:      itemType,
						Source:    realSource,
					})
				}
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(matchPoints)
			return http.StatusOK, nil
		}

		// If failed, continue to standard logic? Or 404?
		logger.Error("Inspect: Direct Path load failed: " + err.Error())
		// Don't return 404 yet, maybe spatial search finds it?
		// But at 0,0 unlikely.
	}

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
					// c.Path is the path to the item (file or folder).
					// If it has an extension, it's a file -> take Dir.
					// If it has no extension, it's a folder -> use as is.
					// We do NOT use PreviewID because it might be just a filename (e.g. "img.jpg").
					leafFolder := c.Path
					if filepath.Ext(leafFolder) != "" {
						leafFolder = filepath.Dir(leafFolder)
					}
					leafFolder = filepath.ToSlash(leafFolder)

					logger.Debug(fmt.Sprintf("Inspect: Found Target Cluster. Path='%s' Leaf='%s'", c.Path, leafFolder))

					// DRILL DOWN LOGIC:
					// If the request came from "Drill Down" (cluster_id is set), we always want to return the contents.

					// Use c.Source if available (it should be), otherwise fallback to realSource
					targetSource := c.Source
					if targetSource == "" {
						targetSource = realSource
					}

					// Fix for Path Duplication during Hydration:
					// The c.Path usually contains the Source Name (Virtual Path).
					// GetFolderHeatmap -> GetRealPath joins SourceRoot + Path.
					// If we pass "/SOURCE/Folder", it becomes "ROOT/SOURCE/SOURCE/Folder" -> 404.
					// We must strip the Source prefix if it exists in the path.
					prefix := "/" + targetSource
					prefixUpper := strings.ToUpper(prefix)
					leafUpper := strings.ToUpper(leafFolder)

					// Store original leaf for fallback logic
					originalLeaf := leafFolder

					// Check if path starts with /SOURCE/ or is exactly /SOURCE
					if strings.HasPrefix(leafUpper, prefixUpper+"/") || leafUpper == prefixUpper {
						leafFolder = leafFolder[len(prefix):]
						if leafFolder == "" {
							leafFolder = "/"
						}
						logger.Debug(fmt.Sprintf("Inspect: Stripped Source Prefix: '%s' -> '%s' (Source: %s)", originalLeaf, leafFolder, targetSource))
					} else {
						// Also check without leading slash just in case
						prefixNoSlash := targetSource
						if strings.HasPrefix(leafUpper, strings.ToUpper(prefixNoSlash)+"/") {
							leafFolder = leafFolder[len(prefixNoSlash):]
							if !strings.HasPrefix(leafFolder, "/") {
								leafFolder = "/" + leafFolder
							}
							logger.Debug(fmt.Sprintf("Inspect: Stripped Source Prefix (NoSlash): '%s' -> '%s'", originalLeaf, leafFolder))
						}
					}

					// Normalize paths for comparison (trim trailing slashes) (Optional debug info)
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

					// Use c.Source if available (it should be), otherwise fallback to realSource
					// targetSource is already defined and calculated above for stripping logic.
					// Just ensure it's not empty if we fell through?
					if targetSource == "" {
						targetSource = realSource
					}

					logger.Debug(fmt.Sprintf("Inspect: Hydrating aggregated cluster %s from leaf %s (Source: %s)", c.ID, leafFolder, targetSource))

					// Helper function to load data
					loadData := func(src, path string) (heatmap.HeatmapData, error) {
						ck := fmt.Sprintf("user:%v:%s", d.user.ID, src)
						return getCachedHeatmap(ck, path, func() (heatmap.HeatmapData, error) {
							return heatmap.GetFolderHeatmap(src, path)
						})
					}
					// Helper to search for match in loaded clusters
					findMatch := func(clusters []heatmap.Cluster, targetID string, targetPath string) ([]heatmap.ClusterPoint, bool, string) {
						// 1. Try Exact ID Match first
						for _, lc := range clusters {
							if lc.ID == targetID {
								return lc.Points, true, "ID Match"
							}
						}
						// 2. Try Path/Content Match
						// If the target path (preview image) is in the cluster, that's our cluster!
						// The targetPath comes from the cluster we clicked on.
						if targetPath != "" {
							for _, lc := range clusters {
								// Check the cluster's main path first
								if lc.Path == targetPath {
									return lc.Points, true, "Path Match (Cluster)"
								}
								// Check points if hydrated (should be for leaf)
								for _, p := range lc.Points {
									if p.Path == targetPath {
										return lc.Points, true, "Path Match (Point)"
									}
								}
							}
						}
						return nil, false, ""
					}

					// Attempt 1: Smart Stripped Path (calculated above)
					logger.Info(fmt.Sprintf("Inspect: Attempt 1 - Loading data for Source='%s' Path='%s'", targetSource, leafFolder))
					leafData, err := loadData(targetSource, leafFolder)

					foundMatch := false
					if err == nil {
						logger.Info(fmt.Sprintf("Inspect: Attempt 1 - Loaded %d clusters. Searching for ID %s or Path %s", len(leafData.Clusters), c.ID, c.Path))
						var method string
						matchPoints, foundMatch, method = findMatch(leafData.Clusters, c.ID, c.Path)
						if foundMatch {
							logger.Info(fmt.Sprintf("Inspect: Hydrated %d points via %s (Attempt 1)", len(matchPoints), method))
						} else {
							logger.Info(fmt.Sprintf("Inspect: Attempt 1 - No match found for ID %s", c.ID))
						}
					} else {
						logger.Info(fmt.Sprintf("Inspect: Attempt 1 - Load Failed: %v", err))
					}

					// Attempt 2: Blind Strip (Fallback)
					if !foundMatch {
						cleanLeaf := strings.Trim(strings.ReplaceAll(originalLeaf, "\\", "/"), "/")
						segments := strings.Split(cleanLeaf, "/")
						if len(segments) > 1 {
							blindPath := "/" + strings.Join(segments[1:], "/")
							logger.Info(fmt.Sprintf("Inspect: Attempt 2 - Blind Strip: '%s' -> '%s'", originalLeaf, blindPath))

							leafData2, err2 := loadData(targetSource, blindPath)
							if err2 == nil {
								logger.Info(fmt.Sprintf("Inspect: Attempt 2 - Loaded %d clusters", len(leafData2.Clusters)))
								var method string
								matchPoints, foundMatch, method = findMatch(leafData2.Clusters, c.ID, c.Path)
								if foundMatch {
									logger.Info(fmt.Sprintf("Inspect: Hydrated %d points via %s (Attempt 2)", len(matchPoints), method))
								}
							} else {
								logger.Info(fmt.Sprintf("Inspect: Attempt 2 - Load Failed: %v", err2))
							}
						} else {
							logger.Info("Inspect: Attempt 2 - Skipped (not enough segments)")
						}
					}

					// Attempt 3: Raw Relative Path
					if !foundMatch {
						rawRel := strings.TrimLeft(filepath.ToSlash(originalLeaf), "/")
						logger.Info(fmt.Sprintf("Inspect: Attempt 3 - Raw Relative: '%s' -> '%s'", originalLeaf, rawRel))
						leafData3, err3 := loadData(targetSource, rawRel)
						if err3 == nil {
							logger.Info(fmt.Sprintf("Inspect: Attempt 3 - Loaded %d clusters", len(leafData3.Clusters)))
							var method string
							matchPoints, foundMatch, method = findMatch(leafData3.Clusters, c.ID, c.Path)
							if foundMatch {
								logger.Info(fmt.Sprintf("Inspect: Hydrated %d points via %s (Attempt 3)", len(matchPoints), method))
							}
						} else {
							logger.Info(fmt.Sprintf("Inspect: Attempt 3 - Load Failed: %v", err3))
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
					// Debug: Why?
					if len(c.Points) == 0 {
						logger.Debug("Inspect: Original cluster points empty.")
					}
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
					// Make the error message appear INSIDE the folder by using the folder path as prefix
					errorMsg := "Leaf cluster not found - trigger manual scan to rebuild"
					errorPath := filepath.ToSlash(filepath.Join(c.Path, errorMsg))

					matchPoints = append(matchPoints, heatmap.ClusterPoint{
						Type:      "file",
						Path:      errorPath,
						Count:     1, // Set to 1 to ensure visibility in counts
						ID:        "error-msg",
						PreviewID: "",
						Lat:       c.Lat,
						Lon:       c.Lon,
						Source:    c.Source,
					})
				} else {
					// CRITICAL: Inject the Cluster ID into all found points so the frontend
					// can preserve context for further drill-downs.
					// Also filter out "heatmap.json" just in case.

					var cleanPoints []heatmap.ClusterPoint
					for _, p := range matchPoints {
						if filepath.Base(p.Path) == "heatmap.json" {
							continue
						}
						// Always override/set the ID to the Cluster ID (c.ID)
						p.ID = c.ID
						cleanPoints = append(cleanPoints, p)
					}
					matchPoints = cleanPoints
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

			// Check Centroid distance OR Bounding Box inclusion
			inBounds := targetLat >= c.Min[0]-radius && targetLat <= c.Max[0]+radius &&
				targetLon >= c.Min[1]-radius && targetLon <= c.Max[1]+radius

			isMatch := false
			if distSq < radius*radius || inBounds {
				isMatch = true
			}

			if isMatch {
				cID := getOrGenID(&c) // CRITICAL: Use Polyfill

				// DEBUG COMPREHENSIVE
				logger.Debug(fmt.Sprintf("Inspect: Match Cluster. Path='%s' Count=%d CID='%s' Points=%d", c.Path, c.Count, cID, len(c.Points)))

				if len(c.Points) > 0 {
					// INJECT ID and FILTER heatmap.json
					for i, p := range c.Points {
						if filepath.Base(p.Path) == "heatmap.json" {
							continue
						}
						// Propagate Parent Cluster ID
						p.ID = cID
						matchPoints = append(matchPoints, p)

						if i == 0 {
							logger.Debug(fmt.Sprintf("Inspect: Injected ID '%s' into point 0. P.Path='%s'", cID, p.Path))
						}
					}
				} else if c.Count > 0 {
					// Aggregated Cluster (Folder Node) OR Drill-Down Candidate
					// Attempt to hydrate from source leaf if possible.
					hydrated := false

					// "Drill Down" - Attempt to load the actual images from the child folder
					if c.Path != "" {
						targetPath := c.Path

						// CRITICAL: Bypass cache for hydration!
						logger.Debug(fmt.Sprintf("Inspect: Spatial Hydration - Bypassing cache for %s", targetPath))
						leafData, err := heatmap.GetFolderHeatmap(c.Source, targetPath)

						if (err != nil || len(leafData.Clusters) == 0) && filepath.Ext(targetPath) != "" {
							targetPath = filepath.Dir(targetPath)
							leafData, err = heatmap.GetFolderHeatmap(c.Source, targetPath)
						}

						if err == nil {
							for _, lc := range leafData.Clusters {
								// ID-based match (use Polyfill)
								lcID := getOrGenID(&lc)
								// logger.Debug(fmt.Sprintf("Inspect: Checking leaf cluster. ID='%s' vs Target='%s'", lcID, cID))

								// RELAXED MATCHING: If IDs don't match (due to regen), try Path match?
								// But generated ID relies on Lat/Lon/Path. Should be stable.
								if lcID == cID {
									if len(lc.Points) > 0 {
										// INJECT ID here too
										for idx, lp := range lc.Points {
											if filepath.Base(lp.Path) == "heatmap.json" {
												continue
											}
											lp.ID = cID
											matchPoints = append(matchPoints, lp)
											if idx == 0 {
												logger.Debug(fmt.Sprintf("Inspect: Hydrated Point 0 ID set to '%s'", cID))
											}
										}
										hydrated = true
										logger.Debug(fmt.Sprintf("Inspect: Hydrated %d points for ID %s from leaf", len(lc.Points), cID))
									}
									break
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

						logger.Debug(fmt.Sprintf("Inspect: Returning fallback node for %s (Path: %s) Type: %s", cID, c.Path, itemType))
						matchPoints = append(matchPoints, heatmap.ClusterPoint{
							Type:      itemType,
							Path:      c.Path,
							Count:     c.Count,
							ID:        cID,
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

	// sanitize the points to remove heatmap.json which is not an image
	var cleanedPoints []heatmap.ClusterPoint
	for _, p := range matchPoints {
		if filepath.Base(p.Path) == "heatmap.json" {
			continue
		}
		// ensure ID is set. If p.ID is missing, try to set it if we have context?
		// But in Spatial search, p comes from matchPoints.
		// matchPoints comes from c.Points.
		// c.Points might have empty IDs.
		// Wait, in the loop above we appended c.Points... we didn't set ID there!
		// We need to set it inside the loops.
		// BUT iterating here is safer/easier.
		// Except we lost "c" here.

		// So we must fix the Logic ABOVE.
		// Reverting this block and applying fix to the loops above properly.
		cleanedPoints = append(cleanedPoints, p)
	}
	matchPoints = cleanedPoints

	// Debug ID presence
	if len(matchPoints) > 0 {
		logger.Debug(fmt.Sprintf("Inspect: Preparing to return %d matchPoints. Point 0 ID='%s' Type='%s' Path='%s'", len(matchPoints), matchPoints[0].ID, matchPoints[0].Type, matchPoints[0].Path))
	} else {
		logger.Debug("Inspect: MatchPoints is empty before trimming.")
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

		// logger.Debug(fmt.Sprintf("Inspect: Final path[%d]: '%s' (source: '%s') Count=%d Type='%s'", i, newP.Path, newP.Source, newP.Count, newP.Type))
	}

	if len(finalPoints) > 0 {
		// Debug: Serialize first point to seeing exactly what goes on wire
		jsonBytes, _ := json.Marshal(finalPoints[0])
		logger.Debug(fmt.Sprintf("Inspect: First Point JSON: %s", string(jsonBytes)))
	}

	logger.Debug(fmt.Sprintf("Inspect: Returning %d points to frontend", len(finalPoints)))
	return renderJSON(w, r, finalPoints)
}
