package http

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
)

// handleInspect returns the list of files for a specific cluster location
// URL format: /api/heatmap/inspect?lat=...&lon=...&source=...&path=...
func handleInspect(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// Check for explicit Cluster ID (Hidden PK) which can be single or comma-separated
	targetClusterIDParam := r.URL.Query().Get("cluster_id")
	var targetClusterIDs []string
	if targetClusterIDParam != "" {
		targetClusterIDs = strings.Split(targetClusterIDParam, ",")
	}

	// 1. Parse coordinates
	latStr := r.URL.Query().Get("lat")
	lonStr := r.URL.Query().Get("lon")

	// 4. Determine Search Mode: Bounding Box vs. Radius/ID
	minLatStr := r.URL.Query().Get("minLat")
	maxLatStr := r.URL.Query().Get("maxLat")
	minLonStr := r.URL.Query().Get("minLon")
	maxLonStr := r.URL.Query().Get("maxLon")

	// VALIDATION RELAXED: If we have Cluster IDs or Bounds, we can skip lat/lon
	// But we still need them defined as float64, defaulting to 0 if missing.
	if (latStr == "" || lonStr == "") && len(targetClusterIDs) == 0 && minLatStr == "" {
		return http.StatusBadRequest, fmt.Errorf("missing lat/lon parameter")
	}

	targetLat, _ := strconv.ParseFloat(latStr, 64)
	targetLon, _ := strconv.ParseFloat(lonStr, 64)

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
	var err error
	var radius float64

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

	if (path == "/" || path == "") && source == "" {
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

	// 5. Calculate Search Radius or Use Tile Bounds
	zoomStr := r.URL.Query().Get("zoom")
	zoom := 18
	if zoomStr != "" {
		z, err := strconv.Atoi(zoomStr)
		if err == nil {
			zoom = z
		}
	}

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

	// --- MODE A: Bounding Box Search (Box Selection Tool) ---
	if minLatStr != "" && maxLatStr != "" && minLonStr != "" && maxLonStr != "" {
		minLat, _ := strconv.ParseFloat(minLatStr, 64)
		maxLat, _ := strconv.ParseFloat(maxLatStr, 64)
		minLon, _ := strconv.ParseFloat(minLonStr, 64)
		maxLon, _ := strconv.ParseFloat(maxLonStr, 64)

		logger.Info(fmt.Sprintf("Inspect: Bounding Box Search: [%f, %f] to [%f, %f] at Zoom %d (Source: %s, Path: %s)", minLat, minLon, maxLat, maxLon, zoom, realSource, scopePath))

		boxBounds := heatmap.TileBounds{
			MinLat: minLat,
			MaxLat: maxLat,
			MinLon: minLon,
			MaxLon: maxLon,
		}

		// Map to track source+path and their index in matchPoints for aggregation
		// 0 means not visited, >0 means index + 1, -1 means already drilled down
		visitedPaths := make(map[string]int)

		// Recursive helper to collect all items in the box 
		var collectRecursive func(clusters []heatmap.Cluster, currentDepth int)
		collectRecursive = func(clusters []heatmap.Cluster, currentDepth int) {
			// Safety: Don't recurse too deep
			if currentDepth > 5 {
				return
			}

			for _, c := range clusters {
				// Check if cluster overlaps selection box 
				if !heatmap.ClusterOverlapsBounds(&c, boxBounds) {
					continue
				}

				cID := getOrGenID(&c)
				
				// Dedup/Aggregate based on source and path 
				pathKey := c.Source + "|" + c.Path
				if status, exists := visitedPaths[pathKey]; exists {
					// If it's a folder entry that hasn't been drilled into yet, add the count
					if status >= 0 && c.Count > 0 && len(c.Points) == 0 {
						matchPoints[status].Count += c.Count
					}
					continue
				}

				// If it has points, add them
				if len(c.Points) > 0 {
					// Mark as visited (storing the index of the first point or a dummy)
					visitedPaths[pathKey] = len(matchPoints)
					for _, p := range c.Points {
						if filepath.Base(p.Path) == "heatmap.json" {
							continue
						}
						p.ID = cID
						p.TotalImageCount = c.TotalImageCount
						matchPoints = append(matchPoints, p)
					}
				} else if c.Count > 0 && c.Path != "" {
					// It's a folder or aggregated cluster
					
					// DECISION: Should we drill down?
					// (Zoom check removed - always allow drill down while in depth bounds)
					if currentDepth < 4 {
						subData, err := heatmap.GetFolderHeatmap(c.Source, c.Path)
						if err == nil && len(subData.Clusters) > 0 {
							// Mark as drilled down before recursing
							visitedPaths[pathKey] = -1
							// Recurse into children of this folder
							collectRecursive(subData.Clusters, currentDepth+1)
						} else {
							// Fallback: If no sub-data found, treat as folder leaf
							visitedPaths[pathKey] = len(matchPoints)
							matchPoints = append(matchPoints, heatmap.ClusterPoint{
								Type: "folder", Path: c.Path, Count: c.Count, ID: cID, Source: c.Source, TotalImageCount: c.TotalImageCount,
							})
						}
					} else {
						// Otherwise, just return the folder itself
						visitedPaths[pathKey] = len(matchPoints)
						matchPoints = append(matchPoints, heatmap.ClusterPoint{
							Type: "folder", Path: c.Path, Count: c.Count, ID: cID, Source: c.Source, TotalImageCount: c.TotalImageCount,
						})
					}
				}
			}
		}

		// Use the already loaded 'data' from Step 3 as the entry point
		collectRecursive(data.Clusters, 0)
		
		logger.Debug(fmt.Sprintf("Inspect: Found %d total points in box after recursive search", len(matchPoints)))

		// Skip standard tile-based search if we used the box
		goto finishMatchPoints
	}

	// --- MODE B: Direct Path Inspection Mode ---
	if path != "" && len(targetClusterIDs) == 0 && (targetLat == 0 && targetLon == 0) {
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
						// Propagate Parent Cluster ID and TotalImageCount
						p.ID = cID
						p.TotalImageCount = c.TotalImageCount
						matchPoints = append(matchPoints, p)
					}
				} else if c.Count > 0 {
					// Fallback for empty clusters
					itemType := "file"
					if len(c.Points) == 0 && c.Count > 0 && filepath.Ext(c.Path) == "" {
						itemType = "folder"
					}
					matchPoints = append(matchPoints, heatmap.ClusterPoint{
						ID:              cID,
						Lat:             c.Lat,
						Lon:             c.Lon,
						Count:           c.Count,
						Path:            c.Path,
						PreviewID:       c.PreviewID,
						Type:            itemType,
						Source:          realSource,
						TotalImageCount: c.TotalImageCount,
					})
				}
			}
			goto finishMatchPoints
		}

		// If failed, continue to standard logic? Or 404?
		logger.Error("Inspect: Direct Path load failed: " + err.Error())
		// Don't return 404 yet, maybe spatial search finds it?
		// But at 0,0 unlikely.
	}

	// Define radius variable in outer scope for error reporting fallback

	if len(targetClusterIDs) > 0 {
		// EXACT MATCH MODE
		logger.Debug(fmt.Sprintf("Inspect: Searching for Cluster IDs %v in %d clusters", targetClusterIDs, len(data.Clusters)))

		for _, c := range data.Clusters {
			if slices.Contains(targetClusterIDs, c.ID) {
				logger.Debug(fmt.Sprintf("Inspect: ID Match Found: %s (Count=%d PointsInStruct=%d)", c.ID, c.Count, len(c.Points)))

				// "Drill Down" Inspection:
				// If this is an aggregated cluster (Points stripped), fetch from the source leaf folder.
				if len(c.Points) == 0 && c.Count > 0 && c.Path != "" {
					leafFolder := c.Path
					if filepath.Ext(leafFolder) != "" {
						leafFolder = filepath.Dir(leafFolder)
					}
					leafFolder = filepath.ToSlash(leafFolder)

					// Use c.Source if available (it should be), otherwise fallback to realSource
					targetSource := c.Source
					if targetSource == "" {
						targetSource = realSource
					}

					// Fix for Path Duplication during Hydration:
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
					} else {
						prefixNoSlash := targetSource
						if strings.HasPrefix(leafUpper, strings.ToUpper(prefixNoSlash)+"/") {
							leafFolder = leafFolder[len(prefixNoSlash):]
							if !strings.HasPrefix(leafFolder, "/") {
								leafFolder = "/" + leafFolder
							}
						}
					}

					// Helper function to load data - CACHE BYPASSED
					loadData := func(src, path string) (heatmap.HeatmapData, error) {
						logger.Debug(fmt.Sprintf("Inspect: Direct Load (No Cache) for %s : %s", src, path))
						return heatmap.GetFolderHeatmap(src, path)
					}

					// Helper to search for match in loaded clusters
					findMatch := func(clusters []heatmap.Cluster, targetID string, targetPath string) ([]heatmap.ClusterPoint, bool, string) {
						// 1. Try Exact ID Match first
						for _, lc := range clusters {
							if lc.ID == targetID {
								return lc.Points, true, "ID Match"
							}
						}

						// 2. Try Path/Content Match (Robust)
						if targetPath != "" {
							normTarget := filepath.ToSlash(targetPath)
							for _, lc := range clusters {
								normLC := filepath.ToSlash(lc.Path)

								// 2a. Exact Path
								if normLC == normTarget {
									return lc.Points, true, "Path Match (Cluster Exact)"
								}
								// 2b. Suffix Match (Cluster)
								// Check if one ends with the other to handle prefix differences
								if strings.HasSuffix(normLC, normTarget) || strings.HasSuffix(normTarget, normLC) {
									return lc.Points, true, "Path Match (Cluster Suffix)"
								}

								for _, p := range lc.Points {
									normP := filepath.ToSlash(p.Path)
									// 2c. Exact Point Path
									if normP == normTarget {
										return lc.Points, true, "Path Match (Point Exact)"
									}
									// 2d. Suffix Match (Point)
									if strings.HasSuffix(normP, normTarget) || strings.HasSuffix(normTarget, normP) {
										return lc.Points, true, "Path Match (Point Suffix)"
									}
								}
							}
						}
						return nil, false, ""
					}

					var currentPoints []heatmap.ClusterPoint

					// Attempt 1: Smart Stripped Path
					logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 1 - Loading leaf '%s' (Source: %s)", leafFolder, targetSource))
					leafData, err := loadData(targetSource, leafFolder)
					foundMatch := false
					if err == nil {
						logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 1 - Loaded %d clusters from leaf. Searching for %s", len(leafData.Clusters), c.ID))
						var method string
						currentPoints, foundMatch, method = findMatch(leafData.Clusters, c.ID, c.Path)
						if foundMatch {
							logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Hydrated %d points via %s (Attempt 1)", len(currentPoints), method))
						} else {
							logger.Debug("INSPECT_DEBUG: Attempt 1 - ID/Path Match FAILED.")
						}
					} else {
						logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 1 - Load Failed: %v", err))
					}

					var leafData2 heatmap.HeatmapData
					var leafData3 heatmap.HeatmapData

					// Attempt 2: Blind Strip (Fallback)
					if !foundMatch {
						cleanLeaf := strings.Trim(strings.ReplaceAll(originalLeaf, "\\", "/"), "/")
						segments := strings.Split(cleanLeaf, "/")
						if len(segments) > 1 {
							blindPath := "/" + strings.Join(segments[1:], "/")
							logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 2 - Blind Strip '%s' -> '%s'", originalLeaf, blindPath))
							var err2 error
							leafData2, err2 = loadData(targetSource, blindPath)
							if err2 == nil {
								logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 2 - Loaded %d clusters", len(leafData2.Clusters)))
								var method string
								currentPoints, foundMatch, method = findMatch(leafData2.Clusters, c.ID, c.Path)
								if foundMatch {
									logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Hydrated %d points via %s (Attempt 2)", len(currentPoints), method))
								}
							} else {
								logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 2 - Load Failed: %v", err2))
							}
						}
					}

					// Attempt 3: Raw Relative Path
					if !foundMatch {
						rawRel := strings.TrimLeft(filepath.ToSlash(originalLeaf), "/")
						logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 3 - Raw Relative '%s'", rawRel))
						var err3 error
						leafData3, err3 = loadData(targetSource, rawRel)
						if err3 == nil {
							logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 3 - Loaded %d clusters", len(leafData3.Clusters)))
							var method string
							currentPoints, foundMatch, method = findMatch(leafData3.Clusters, c.ID, c.Path)
							if foundMatch {
								logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Hydrated %d points via %s (Attempt 3)", len(currentPoints), method))
							}
						} else {
							logger.Debug(fmt.Sprintf("INSPECT_DEBUG: Attempt 3 - Load Failed: %v", err3))
						}
					}

					// Append found points
					if foundMatch {
												if len(currentPoints) == 0 {
							// If we matched a folder but have no points, return its children from the leaf
							var sourceClusters []heatmap.Cluster
							if len(leafData.Clusters) > 0 { sourceClusters = leafData.Clusters }
							if len(leafData2.Clusters) > 0 { sourceClusters = leafData2.Clusters }
							if len(leafData3.Clusters) > 0 { sourceClusters = leafData3.Clusters }

							if len(sourceClusters) > 0 {
								logger.Debug(fmt.Sprintf("Inspect: Hydrating folder cluster %s with %d child clusters", c.ID, len(sourceClusters)))
								for _, lc := range sourceClusters {
									lcID := getOrGenID(&lc)
									itemType := "file"
									if len(lc.Points) == 0 && lc.Count > 0 && filepath.Ext(lc.Path) == "" {
										itemType = "folder"
									}
									matchPoints = append(matchPoints, heatmap.ClusterPoint{
										Type:            itemType,
										Path:            lc.Path,
										Count:           lc.Count,
										ID:              lcID,
										Source:          lc.Source,
										TotalImageCount: lc.TotalImageCount,
										Lat:             lc.Lat,
										Lon:             lc.Lon,
										PreviewID:       lc.PreviewID,
									})
								}
							} else {
								// Fallback: Return folder itself if truly empty
								matchPoints = append(matchPoints, heatmap.ClusterPoint{
									Type:            "folder",
									Path:            c.Path,
									Count:           c.Count,
									ID:              c.ID,
									Source:          c.Source,
									TotalImageCount: c.TotalImageCount,
									Lat:             c.Lat,
									Lon:             c.Lon,
									PreviewID:       c.PreviewID,
								})
							}
						} else {
							for _, p := range currentPoints {
								if filepath.Base(p.Path) == "heatmap.json" {
									continue
								}
								p.ID = c.ID
								p.TotalImageCount = c.TotalImageCount
								matchPoints = append(matchPoints, p)
							}
						}
					} else {
						// Hydration failed to find the specific cluster ID inside its own leaf heatmap.
						// NEW LOGIC: If leafData was successfully loaded, return ALL its top-level clusters as "points".
						// This ensures that clicking a folder cluster expands it into its constituent folders/files.
						if err == nil && len(leafData.Clusters) > 0 {
							logger.Debug(fmt.Sprintf("Inspect: Cluster %s not found in leaf, expanding ALL %d leaf clusters", c.ID, len(leafData.Clusters)))
							for _, lc := range leafData.Clusters {
								lcID := getOrGenID(&lc)
								// Determine type (folder if it has sub-clusters or no extension)
								itemType := "file"
								if len(lc.Points) == 0 && lc.Count > 0 && filepath.Ext(lc.Path) == "" {
									itemType = "folder"
								}
								matchPoints = append(matchPoints, heatmap.ClusterPoint{
									Type:            itemType,
									Path:            lc.Path,
									Count:           lc.Count,
									ID:              lcID,
									Source:          lc.Source,
									TotalImageCount: lc.TotalImageCount,
									Lat:             lc.Lat,
									Lon:             lc.Lon,
								})
							}
						} else {
							// Final Fallback: Return the cluster itself as a folder entry
							logger.Error(fmt.Sprintf("Inspect: Failed to hydrate cluster %s - No match found and no leaf data", c.ID))
							matchPoints = append(matchPoints, heatmap.ClusterPoint{
								Type:            "folder",
								Path:            c.Path,
								Count:           c.Count,
								ID:              c.ID,
								Source:          c.Source,
								TotalImageCount: c.TotalImageCount,
							})
						}
					}

				} else {
					// Use existing points (already hydrated or leaf)
					currentPoints := c.Points
					logger.Debug(fmt.Sprintf("Inspect: Using existing points for %s (Count=%d)", c.ID, len(currentPoints)))

					// Post-hydration check
					if len(currentPoints) == 0 {
						logger.Info("Inspect: Match found but points empty. Falling back to Folder Node.")
						// ... fallback code ...
						currentPoints = append(currentPoints, heatmap.ClusterPoint{
							Type:            "folder",
							Path:            c.Path,
							Count:           c.Count,
							ID:              c.ID,
							Source:          c.Source,
							TotalImageCount: c.TotalImageCount,
						})
					}

					for _, p := range currentPoints {
						if filepath.Base(p.Path) == "heatmap.json" {
							continue
						}
						// Always override/set the ID to the Cluster ID (c.ID)
						p.ID = c.ID
						p.TotalImageCount = c.TotalImageCount
						matchPoints = append(matchPoints, p)
					}
					logger.Debug(fmt.Sprintf("Inspect: Appended %d points from cluster %s. Total matchPoints=%d", len(currentPoints), c.ID, len(matchPoints)))
				}
			}
		}
		logger.Debug(fmt.Sprintf("Inspect: Finished Loop. Total matches: %d", len(matchPoints)))
		if len(matchPoints) == 0 {
			logger.Info(fmt.Sprintf("Inspect: Cluster IDs %v not found in current view. Falling back to spatial search.", targetClusterIDs))
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
						p.TotalImageCount = c.TotalImageCount
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
							Type:            itemType,
							Path:            c.Path,
							Count:           c.Count,
							ID:              cID,
							PreviewID:       c.PreviewID,
							Lat:             c.Lat,
							Lon:             c.Lon,
							Source:          c.Source,
							TotalImageCount: c.TotalImageCount,
						})
					}
				}
			}
		}

	}

finishMatchPoints:
	if len(matchPoints) == 0 {
		logger.Info(fmt.Sprintf("Inspect: No matches found for IDs %v (Radius: %f). Returning empty.", targetClusterIDs, radius))
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("[]"))
		return http.StatusOK, nil
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
		
		// Fix for Multi-Scope Path Resolution:
		// If the original request used a scoped source name (e.g. PHOTOS:1), 
		// we must return that same source name so subsequent API calls (like /preview) 
		// resolve the relative path against the correct scope.
		if source != "" && newP.Source == realSource {
			newP.Source = source
		}

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
