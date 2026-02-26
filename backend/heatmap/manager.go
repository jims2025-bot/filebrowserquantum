// Avoid cyclic dependency by using an interface or just passing the Users store directly?
// Passing *storage.Storage causes import cycle if storage imports heatmap.
// Storage imports: users, share, auth, settings.
// Heatmap imports: users, indexing.
// So importing storage in heatmap is safe.

package heatmap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
)

// ScanIntervalDays defines how often the full heatmap scan runs (in days)
// Manual scans override this and percolate up immediately
const ScanIntervalDays = 7 // Weekly scan

// OverlayScanIntervalHours defines how often the overlay scan runs
const OverlayScanIntervalHours = 12

// SafeClusterCollector removed as we are using aggregation.

type ScanProgress struct {
	Current    int        `json:"current"`
	Total      int        `json:"total"`
	Message    string     `json:"message"`
	LastUpdate time.Time  `json:"-"`
	mu         sync.Mutex `json:"-"`
}

var (
	// ActiveScans tracks which paths are currently being scanned to prevent duplicates.
	// Key: "sourceName|path", Value: *ScanProgress
	activeScans sync.Map

	// MergeRadius defines the proximity threshold for merging duplicate/stacked clusters.
	// 0.0003 degrees is approx 33 meters.
	MergeRadius = 0.0003

	// Tracker for active folder scans (for 5-min summary)
	activeFolders sync.Map // map[string]activeFolderInfo
)

type activeFolderInfo struct {
	Path      string
	FileCount int
	StartTime time.Time
}

// ScanSafe triggers a recursive scan for a given path if one is not already running.
// It is safe to call continuously; it will reject requests for paths already being processed.
// isManualScan: if true, forces rebuild regardless of version/timestamp
func ScanSafe(sourceName, path string, isManualScan bool) error {
	key := sourceName + "|" + path
	// Mutex is zero-value initialized
	progress := &ScanProgress{Current: 0, Total: 0, Message: "Starting scan...", LastUpdate: time.Now()}
	if _, loaded := activeScans.LoadOrStore(key, progress); loaded {
		return fmt.Errorf("scan already in progress for %s", path)
	}
	defer activeScans.Delete(key)

	// logger.Info("Heatmap: Manually started safe scan for " + sourceName + " " + path)
	logger.Debug(fmt.Sprintf("Heatmap: ScanSafe START Source=%s Path=%s", sourceName, path))

	// Pre-count files for progress reporting
	idx := indexing.GetIndex(sourceName)
	if idx != nil {
		progress.Message = "Counting files..."
		progress.Total = CountFilesRecursive(idx, path)
		logger.Info(fmt.Sprintf("Heatmap: Found %d files to scan in %s", progress.Total, path))
	}

	// Limit concurrency
	sem := make(chan struct{}, 20)

	// CRITICAL: Purge old clusters from parents before starting manual scan.
	// This ensures that if the new scan produces different IDs or counts, we don't end up with "ghost" clusters
	// remaining in the parent aggregate files (since manual scan injects into existing tree).
	if isManualScan {
		PurgeClustersFromParents(sourceName, path)
	}
	_, err := ScanRecursive(sourceName, path, progress, sem, isManualScan)
	if err == nil {
		// Update parents
		go PercolateUp(sourceName, path)
		logger.Debug(fmt.Sprintf("Heatmap: ScanSafe FINISHED Source=%s Path=%s", sourceName, path))
	} else {
		logger.Error(fmt.Sprintf("Heatmap: ScanSafe FAILED Source=%s Path=%s Error=%v", sourceName, path, err))
	}
	return nil
}

// PurgeClustersFromParents removes all clusters belonging to the target scopePath
// from all parent heatmap.json files up to the root.
func PurgeClustersFromParents(sourceName, scopePath string) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return
	}

	// logger.Info(fmt.Sprintf("Heatmap: Purging old clusters for scope %s in source %s", scopePath, sourceName))

	currentPath := scopePath
	// We walk UP from the target folder.
	for {
		parent := filepath.Dir(currentPath)
		logger.Debug(fmt.Sprintf("Heatmap Check Purge: Current='%s' Parent='%s'", currentPath, parent))
		// Break if we hit root or top or weirdness
		if parent == currentPath || parent == "." {
			if currentPath == "/" {
				break
			}
			// On windows filepath.Dir("C:") is "C:"?
			// Using ToSlash/Clean helps.
		}
		parent = filepath.ToSlash(filepath.Clean(parent))

		// If we processed root "/" last time, break now?
		// If currentPath was "/", parent is "/". We broke above.

		// 1. Calculate the relative path key we are looking for.
		// If Parent is "/A" and Scope is "/A/B", we invoke for Parent "/A".
		// We want to remove clusters that are inside "B".
		// Relative path from "/A" to "/A/B" is "B".
		rel, err := filepath.Rel(parent, scopePath)
		if err != nil {
			logger.Error("Heatmap: Purge Rel failed: " + err.Error())
			break
		}
		cleanRel := filepath.ToSlash(rel)

		// 2. Load Parent Heatmap
		realPath, _, err := idx.GetRealPath(parent)
		if err == nil {
			heatmapPath := filepath.Join(realPath, HeatmapFilename)

			if info, err := os.Stat(heatmapPath); err == nil && !info.IsDir() {
				bytes, err := os.ReadFile(heatmapPath)
				if err == nil {
					var data HeatmapData
					if err := json.Unmarshal(bytes, &data); err == nil {
						originalCount := len(data.Clusters)
						var newClusters []Cluster

						for _, c := range data.Clusters {
							// Check if this cluster belongs to the purged folder.
							// c.Path is relative to parent.
							// If c.Path starts with "B/" or is "B", it's from that folder.
							cPath := filepath.ToSlash(c.Path)

							// Exact match (the folder itself) or Child match
							isTarget := false
							if cPath == cleanRel {
								isTarget = true
							} else if strings.HasPrefix(cPath, cleanRel+"/") {
								isTarget = true
							}

							if !isTarget {
								newClusters = append(newClusters, c)
							}
						}

						if len(newClusters) != originalCount {
							logger.Info(fmt.Sprintf("Heatmap: Purged %d clusters from parent %s (Target=%s)", originalCount-len(newClusters), parent, cleanRel))

							// Save
							data.Clusters = newClusters
							data.GeneratedAt = time.Now() // Mark updated
							if err := writeHeatmapFile(heatmapPath, data); err != nil {
								logger.Error("Heatmap: Purge Write failed: " + err.Error())
							}
						}
					}
				}
			}
		}

		if parent == "/" || parent == "." || parent == "" {
			break
		}
		currentPath = parent
	}
}
func GetScanProgress(sourceName, path string) *ScanProgress {
	key := sourceName + "|" + path
	if val, ok := activeScans.Load(key); ok {
		// Return a copy to avoid race conditions on read
		// But wait, ScanProgress has a Mutex field now. We cannot copy the mutex.
		// We should return a struct with just the data.
		// But existing callers expect *ScanProgress.
		// Let's rely on the fact that the JSON marshaller (in the handler) accesses fields.
		// If we return the pointer, we race.
		// FIX: The handler marshalling will race.
		// We need a thread-safe way to get the snapshot.
		sp := val.(*ScanProgress)
		sp.mu.Lock()
		defer sp.mu.Unlock()

		return &ScanProgress{
			Current:    sp.Current,
			Total:      sp.Total,
			Message:    sp.Message,
			LastUpdate: sp.LastUpdate,
		}
	}
	return nil
}

// IsScanning checks if a scan is currently in progress for the given path.
func IsScanning(sourceName, path string) bool {
	key := sourceName + "|" + path
	_, loaded := activeScans.Load(key)
	// logger.Debug(fmt.Sprintf("Heatmap: IsScanning Check Key='%s' Result=%v", key, loaded))
	return loaded
}

func StartJob(store *storage.Storage, onComplete func()) {
	go func() {
		// Start the active scan monitor
		go monitorActiveScans()

		// Wait for initial index to populate (large repos take time)
		// Changed: Run scan shortly after startup to catch version updates.
		logger.Info("Heatmap: Scan job scheduled. Starting in 1 minute.")
		time.Sleep(1 * time.Minute)

		for {
			logger.Info("Starting Heatmap Generation Job")
			logger.Info(fmt.Sprintf("Heatmap.json version = %d", HeatmapVersion))
			logger.Info("New heatmap version changes require update in backend/heatmap/types.go")

			// Build lookup maps to resolve index names from paths
			allIndexes := indexing.GetIndexes()
			pathToSource := make(map[string]string)
			for name, idx := range allIndexes {
				pathToSource[filepath.ToSlash(filepath.Clean(idx.Source.Path))] = name
			}

			// Fetch all users
			allUsers, err := store.Users.Gets()
			if err != nil {
				logger.Error("Heatmap: Failed to retrieve users: " + err.Error())
			} else {
				// Deduplicate scopes to scan
				locationsToScan := make(map[string]map[string]struct{})

				for _, u := range allUsers {
					if len(u.Scopes) == 0 {
						continue
					}

					// Skip admins to prevent scanning the entire root/drive
					if u.Permissions.Admin {
						continue
					}

					for _, s := range u.Scopes {
						var properSourceName string

						if _, ok := allIndexes[s.Name]; ok {
							properSourceName = s.Name
						} else {
							// Try resolving alias (e.g. PHOTOS:0 -> PHOTOS)
							if strings.Contains(s.Name, ":") {
								parts := strings.Split(s.Name, ":")
								if len(parts) > 0 {
									if _, ok := allIndexes[parts[0]]; ok {
										properSourceName = parts[0]
									}
								}
							}

							if properSourceName == "" {
								// Try resolving path to name
								clean := filepath.ToSlash(filepath.Clean(s.Name))
								if name, ok := pathToSource[clean]; ok {
									properSourceName = name
								}
							}
						}

						if properSourceName != "" {
							if locationsToScan[properSourceName] == nil {
								locationsToScan[properSourceName] = make(map[string]struct{})
							}
							scopePath := s.Scope
							if scopePath == "" {
								scopePath = "/"
							}
							locationsToScan[properSourceName][scopePath] = struct{}{}
						}
					}
				}

				// Execute scans
				for sourceName, paths := range locationsToScan {
					for path := range paths {
						logger.Info("Heatmap: Scanning scope " + sourceName + " " + path)
						// Limit concurrency
						sem := make(chan struct{}, 20)
						_, err := ScanRecursive(sourceName, path, nil, sem, false) // Automatic scan
						if err == nil {
							// Percolate clusters up to parent directories after scan completes
							PercolateUp(sourceName, path)
						} else {
							logger.Error(fmt.Sprintf("Heatmap: Automatic scan failed Source=%s Path=%s Error=%v", sourceName, path, err))
						}
					}
				}
			}

			logger.Info("Finished Heatmap Generation Job")

			// Custom callback (e.g. Integrity Scan)
			if onComplete != nil {
				onComplete()
			}

			// Run every ScanIntervalDays
			time.Sleep(time.Duration(ScanIntervalDays) * 24 * time.Hour)
		}
	}()
}

func StartOverlayJob(store *storage.Storage) {
	go func() {
		// Wait a bit on startup to let indexes load
		time.Sleep(1 * time.Minute)

		for {
			logger.Info("Starting Overlay Scan Job")

			// Get all users to find scopes
			allUsers, err := store.Users.Gets()
			if err != nil {
				logger.Error("Overlay: Failed to get users: " + err.Error())
			} else {
				// Deduplicate scopes
				locationsToScan := make(map[string]map[string]struct{})
				allIndexes := indexing.GetIndexes()

				logger.Info(fmt.Sprintf("Overlay: Found %d users and %d indexes", len(allUsers), len(allIndexes)))

				// Scan restricted to PHOTOCOLLECTIONS as requested
				for sourceName, idx := range allIndexes {
					if locationsToScan[sourceName] == nil {
						locationsToScan[sourceName] = make(map[string]struct{})
					}

					// 1. If Source IS "PHOTOCOLLECTIONS", scan root
					if strings.ToUpper(sourceName) == "PHOTOCOLLECTIONS" {
						locationsToScan[sourceName]["/"] = struct{}{}
						logger.Info("Overlay: Scheduled restricted scan for Source " + sourceName)
						continue
					}

					// 2. If "/PHOTOCOLLECTIONS" exists in this source, scan it
					if realPath, _, err := idx.GetRealPath("/PHOTOCOLLECTIONS"); err == nil {
						if info, err := os.Stat(realPath); err == nil && info.IsDir() {
							locationsToScan[sourceName]["/PHOTOCOLLECTIONS"] = struct{}{}
							logger.Info("Overlay: Scheduled restricted scan for Folder /PHOTOCOLLECTIONS in " + sourceName)
							continue
						}
					}

					// 3. Otherwise scan the root of the source
					// Relaxed restriction: If the user didn't have a PHOTOCOLLECTIONS folder, we were skipping them entirely.
					// Now we default to scanning the source root.
					locationsToScan[sourceName]["/"] = struct{}{}
					logger.Info("Overlay: Scheduled full scan for Source " + sourceName)
				}

				// Execute Scans
				for sourceName, paths := range locationsToScan {
					for path := range paths {
						logger.Info("Overlay: Scanning scope " + sourceName + " " + path)
						err := ScanOverlaysRecursive(sourceName, path)
						if err != nil {
							logger.Error("Overlay: Scan failed for " + path + ": " + err.Error())
						}
					}
				}
			}

			logger.Info("Finished Overlay Scan Job")
			time.Sleep(time.Duration(OverlayScanIntervalHours) * time.Hour)
		}
	}()
}

// GetGlobalHeatmap aggregates pre-computed heatmap.json files from all user scopes.
func GetGlobalHeatmap(user *users.User) (GlobalHeatmap, error) {
	var allClusters []Cluster

	// We'll iterate all indexes and filter by scope.
	rawIndexes := indexing.GetIndexes()
	visitedHeatmaps := make(map[string]bool)
	processedRoots := []string{}

	// Convert Map to Slice for sorting
	allIndexes := make([]*indexing.Index, 0, len(rawIndexes))
	for _, idx := range rawIndexes {
		allIndexes = append(allIndexes, idx)
	}

	// Sort indexes by path length (Shortest first) to prioritize parents
	for i := 0; i < len(allIndexes)-1; i++ {
		for j := 0; j < len(allIndexes)-i-1; j++ {
			if len(allIndexes[j].Source.Path) > len(allIndexes[j+1].Source.Path) {
				allIndexes[j], allIndexes[j+1] = allIndexes[j+1], allIndexes[j]
			}
		}
	}

	for _, idx := range allIndexes {
		validPaths := []string{}

		// Containment Deduplication:
		// If this source is a child of an already processed source, SKIP IT.
		sourceRoot := filepath.ToSlash(filepath.Clean(idx.Source.Path))
		isNested := false
		for _, parent := range processedRoots {
			rel, err := filepath.Rel(parent, sourceRoot)
			if err == nil && !strings.HasPrefix(rel, "..") && rel != "." {
				// It is a subdirectory (and not the same directory, handled by validPaths logic?
				// No, duplicate aliases are handled by visitedHeatmaps.
				// Nested directories (e.g. /Foo vs /Foo/Bar) are handled here.
				isNested = true
				break
			}
		}
		if isNested {
			continue
		}
		processedRoots = append(processedRoots, sourceRoot)

		// Scope resolution logic
		hasSourceScope := false
		for _, s := range user.Scopes {
			isMatch := (s.Name == idx.Source.Name)
			if !isMatch && strings.Contains(s.Name, ":") {
				parts := strings.Split(s.Name, ":")
				if len(parts) > 0 && parts[0] == idx.Source.Name {
					isMatch = true
				}
			}
			if !isMatch {
				p1 := filepath.ToSlash(filepath.Clean(s.Name))
				p2 := filepath.ToSlash(filepath.Clean(idx.Source.Path))
				if p1 == p2 {
					isMatch = true
				}
			}
			if isMatch {
				validPaths = append(validPaths, s.Scope)
				hasSourceScope = true
			}
		}

		// FIX: For Admin, always use Root "/" to ensure full visibility and avoid scope conflicts.
		// This matches the override in ResolveScopePath/HeatmapHandler.
		if user.Permissions.Admin {
			validPaths = []string{"/"}
		} else if !hasSourceScope {
			continue
		}

		if len(validPaths) == 0 {
			continue
		}

		for _, rootPath := range validPaths {
			// Read the pre-computed heatmap.json for this scope
			// We need a fresh index object to be safe or just use the one we have?
			// idx is from the loop. It works.

			realPath, _, err := idx.GetRealPath(rootPath)
			if err != nil {
				continue
			}
			heatmapPath := filepath.Join(realPath, HeatmapFilename)

			// Deduplication check (Case Insensitive for Windows safety)
			dedupKey := strings.ToLower(filepath.Clean(heatmapPath))
			if visitedHeatmaps[dedupKey] {
				continue
			}
			visitedHeatmaps[dedupKey] = true

			// Read file
			if _, err := os.Stat(heatmapPath); err == nil {
				bytes, err := os.ReadFile(heatmapPath)
				if err == nil {
					var data HeatmapData
					if err := json.Unmarshal(bytes, &data); err == nil {
						// Inject Source and prefix paths
						for i := range data.Clusters {
							data.Clusters[i].Source = idx.Source.Name

							// Helper to join paths
							join := func(p string) string {
								// Trust raw input if it looks absolute (Unix or Windows style)
								if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "\\") {
									return filepath.ToSlash(filepath.Clean(p))
								}

								cleanP := filepath.ToSlash(filepath.Clean(p))
								cleanRoot := filepath.ToSlash(filepath.Clean(rootPath))

								// If the path ALREADY starts with the root path, return it as is.
								if strings.HasPrefix(cleanP, cleanRoot) {
									return cleanP
								}

								// Check cleaned path for absolute indicator
								if strings.HasPrefix(cleanP, "/") {
									return cleanP
								}

								// Debug why we are joining
								res := filepath.ToSlash(filepath.Join(rootPath, p))
								if strings.Contains(res, "PHOTOCOLLECTIONS/POWELL-COLLECTION") {
									logger.Info(fmt.Sprintf("Manager Join: Root='%s' P='%s' CleanP='%s' -> Res='%s'", rootPath, p, cleanP, res))
								}

								// Otherwise, join them.
								return res
							}

							if data.Clusters[i].Path != "" {
								data.Clusters[i].Path = join(data.Clusters[i].Path)
							}

							// Points
							for j := range data.Clusters[i].Points {
								if data.Clusters[i].Points[j].Path != "" {
									data.Clusters[i].Points[j].Path = join(data.Clusters[i].Points[j].Path)
								}
								data.Clusters[i].Points[j].Source = idx.Source.Name
							}
						}

						allClusters = append(allClusters, data.Clusters...)
					}
				}
			}
		}
	}

	// Sort clusters to ensure deterministic output
	sort.Slice(allClusters, func(i, j int) bool {
		if allClusters[i].ID != "" && allClusters[j].ID != "" {
			return allClusters[i].ID < allClusters[j].ID
		}
		if allClusters[i].Path != allClusters[j].Path {
			return allClusters[i].Path < allClusters[j].Path
		}
		return allClusters[i].Lat < allClusters[j].Lat
	})

	// Optional: Merge identical location clusters here if needed.
	// For now, raw aggregation.

	return GlobalHeatmap{Clusters: allClusters}, nil
}

// GetFolderHeatmap reads the pre-computed heatmap.json for a specific folder.
func GetFolderHeatmap(sourceName, path string) (HeatmapData, error) {
	// logger.Info("Heatmap: GetFolderHeatmap for Source=" + sourceName + " Path=" + path)
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		// Try resolving alias (e.g., PHOTOS:0 -> PHOTOS)
		if strings.Contains(sourceName, ":") {
			parts := strings.Split(sourceName, ":")
			if len(parts) > 0 {
				idx = indexing.GetIndex(parts[0])
			}
		}
	}

	if idx == nil {
		logger.Error("Heatmap: Index not found for source: " + sourceName)
		return HeatmapData{}, nil
	}

	realPath, _, err := idx.GetRealPath(path)
	if err != nil {
		return HeatmapData{}, err
	}

	heatmapPath := filepath.Join(realPath, HeatmapFilename)
	// If file doesn't exist, return empty data
	if _, err := os.Stat(heatmapPath); err != nil {
		return HeatmapData{
			GeneratedAt: time.Now(),
			Clusters:    []Cluster{},
			TotalImages: 0,
		}, nil
	}

	bytes, err := os.ReadFile(heatmapPath)
	if err != nil {
		return HeatmapData{}, err
	}

	var data HeatmapData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return HeatmapData{}, err
	}

	// Inject Source/Paths
	for i := range data.Clusters {
		data.Clusters[i].Source = sourceName
		// Points
		for j := range data.Clusters[i].Points {
			data.Clusters[i].Points[j].Source = sourceName
		}
	}

	return data, nil
}

func ScanRecursive(sourceName, rootPath string, progress *ScanProgress, sem chan struct{}, isManualScan bool) ([]Cluster, error) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return nil, fmt.Errorf("index not found")
	}

	if progress != nil {
		progress.Message = "Scanning " + filepath.Base(rootPath)
	}

	// 1. Scan Subfolders (Parallel)
	dirInfo, exists := idx.GetReducedMetadata(rootPath, true)
	var wg sync.WaitGroup

	if exists {
		for _, sub := range dirInfo.Folders {
			wg.Add(1)
			go func(subName string) {
				defer wg.Done()

				// Ensure forward slashes
				nextPath := filepath.ToSlash(filepath.Join(rootPath, subName))

				// Recurse without holding local semaphore (aggregation inside will acquire it)
				_, _ = ScanRecursive(sourceName, nextPath, progress, sem, isManualScan)
			}(sub.Name)
		}
	}

	wg.Wait()

	// 2. Aggregate current level (Local + Children)
	sem <- struct{}{}
	defer func() { <-sem }()
	return AggregateLevel(sourceName, rootPath, progress, isManualScan)
}

// AggregateLevel scans local files and aggregates child heatmap.json files.
// isManualScan: if true, forces rebuild regardless of version/timestamp
func AggregateLevel(sourceName, rootPath string, progress *ScanProgress, isManualScan bool) ([]Cluster, error) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return nil, fmt.Errorf("index not found")
	}

	// Count files in this folder for logging
	fileCount := 0
	dirInfo, exists := idx.GetReducedMetadata(rootPath, true)
	if exists {
		for _, file := range dirInfo.Files {
			if iteminfo.IsImage(file.Name) {
				fileCount++
			}
		}
	}

	// Check if we should skip this folder (only for automatic scans)
	// Check BEFORE logging START so we don't spam logs for skipped folders
	if !isManualScan {
		realPath, _, err := idx.GetRealPath(rootPath)
		if err == nil {
			heatmapPath := filepath.Join(realPath, HeatmapFilename)
			if _, err := os.Stat(heatmapPath); err == nil {
				bytes, err := os.ReadFile(heatmapPath)
				if err == nil {
					var existingData HeatmapData
					if err := json.Unmarshal(bytes, &existingData); err == nil {
						// Check version and age
						if existingData.Version == HeatmapVersion {
							age := time.Since(existingData.GeneratedAt)
							if age < time.Duration(ScanIntervalDays)*24*time.Hour {
								// Also check: skip only if no child heatmap.json is newer than this one.
								// If a child was updated more recently, the parent needs to re-aggregate.
								childNewer := false
								if childDirInfo, childExists := idx.GetReducedMetadata(rootPath, true); childExists {
									for _, sub := range childDirInfo.Folders {
										childVirtualPath := filepath.ToSlash(filepath.Join(rootPath, sub.Name))
										childRealPath, _, cerr := idx.GetRealPath(childVirtualPath)
										if cerr == nil {
											childHeatmap := filepath.Join(childRealPath, HeatmapFilename)
											if info, serr := os.Stat(childHeatmap); serr == nil {
												if info.ModTime().After(existingData.GeneratedAt) {
													childNewer = true
													break
												}
											}
										}
									}
								}

								if !childNewer {
									logger.Info(fmt.Sprintf("Heatmap [SKIP ]: %s (%d files) - HeatmapJSON v%d - last run: %s",
										rootPath, fileCount, existingData.Version, existingData.GeneratedAt.Format(time.RFC3339)))
									return existingData.Clusters, nil
								}
								logger.Info(fmt.Sprintf("Heatmap [RE-AGG]: %s - child heatmap newer than parent, re-aggregating", rootPath))
							}
						}
					}
				}
			}
		}
	}

	// Enhanced logging: START
	startTime := time.Now()
	// Track valid GPS count for the DONE log
	var totalValidGPS int

	// Track active folder for 5-minute summary
	activeFolders.Store(rootPath, activeFolderInfo{
		Path:      rootPath,
		FileCount: fileCount,
		StartTime: startTime,
	})

	if fileCount > 0 {
		// logger.Info(fmt.Sprintf("Heatmap [START]: %s (%d files)", rootPath, fileCount))
	}

	// Defer a DONE log if not skipped
	defer func() {
		// Remove from active tracker
		activeFolders.Delete(rootPath)

		if fileCount > 0 {
			// duration := time.Since(startTime)
			// logger.Info(fmt.Sprintf("Heatmap [DONE ]: %s (%d files, %d w/GPS) - took %s",
			// 	rootPath, fileCount, totalValidGPS, duration.Round(time.Millisecond)))
		}
	}()

	// 1. Get Local Clusters
	// This rescans local files to ensure accuracy.
	localClusters, _ := GetLocalClusters(sourceName, rootPath, progress)

	// Update valid GPS count for logging
	for _, c := range localClusters {
		totalValidGPS += c.Count
	}

	// 2. Read Child Clusters
	var childClusters []Cluster
	dirInfo, exists = idx.GetReducedMetadata(rootPath, true)
	if exists {
		for _, sub := range dirInfo.Folders {
			subName := sub.Name
			nextPath := filepath.ToSlash(filepath.Join(rootPath, subName))

			// Read child heatmap.json
			realPath, _, err := idx.GetRealPath(nextPath)
			if err == nil {
				heatmapPath := filepath.Join(realPath, HeatmapFilename)
				if _, err := os.Stat(heatmapPath); err == nil {
					bytes, err := os.ReadFile(heatmapPath)
					if err == nil {
						var data HeatmapData
						if err := json.Unmarshal(bytes, &data); err == nil {
							// Strip points and adjust paths
							for i := range data.Clusters {
								data.Clusters[i].Points = nil

								// Absolute paths (from GetLocalClusters) are kept as-is; relative paths get the subfolder prepended.
								if data.Clusters[i].Path != "" {
									cleanPath := filepath.ToSlash(filepath.Clean(data.Clusters[i].Path))

									// Paths saved by GetLocalClusters are always absolute from source root (start with "/").
									// Keep them as-is. Only prepend subName for legacy relative paths.
									if strings.HasPrefix(cleanPath, "/") {
										logger.Debug(fmt.Sprintf("AggregateLevel: Path '%s' is absolute, keeping as-is", cleanPath))
										data.Clusters[i].Path = cleanPath
									} else {
										// Path is relative — prepend subfolder name
										data.Clusters[i].Path = filepath.ToSlash(filepath.Join(subName, cleanPath))
										logger.Debug(fmt.Sprintf("AggregateLevel: Joined subName '%s' + path '%s' = '%s'", subName, cleanPath, data.Clusters[i].Path))
									}
								}
							}
							childClusters = append(childClusters, data.Clusters...)
						}
					}
				}
			}
		}
	}

	// 3. Aggregate
	allClusters := append(localClusters, childClusters...)

	// 4. Save
	realPath, _, err := idx.GetRealPath(rootPath)
	if err == nil {

		totalCount := 0
		for _, c := range allClusters {
			totalCount += c.Count
		}

		data := HeatmapData{
			Version:     HeatmapVersion,
			GeneratedAt: time.Now(),
			Clusters:    allClusters,
			TotalImages: totalCount,
		}

		outPath := filepath.Join(realPath, HeatmapFilename)
		if err := writeHeatmapFile(outPath, data); err != nil {
			logger.Error("Heatmap: Failed to write " + outPath + ": " + err.Error())
		}
	}

	return allClusters, nil
}

// writeHeatmapFile ensures atomic writes by writing to a temp file and renaming it.
func writeHeatmapFile(path string, data HeatmapData) error {
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// Use a temp file in the same directory to ensure we are on the same filesystem for atomic rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, bytes, 0644); err != nil {
		return err
	}

	// Atomic replace
	// On Windows, os.Rename replaces the file if it exists (Go > 1.4), similar to POSIX.
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Cleanup on failure
		return err
	}
	return nil
}

// PercolateUp updates heatmap.json for all parents of the given path up to the source root.
func PercolateUp(sourceName, startPath string) {
	currentPath := startPath
	for {
		// Move up
		parent := filepath.Dir(currentPath)
		if parent == currentPath || parent == "." {
			// Hit root or internal representation limit
			// Check if startPath itself was root.
			// If currentPath is "/", parent is "/". Break.
			if currentPath == "/" {
				break
			}
			// If we are at root but loop continues?
			if parent == "." {
				// usually indexing paths are absolute-ish like "/" or "/foo".
				// filepath.Dir("/") is "/".
				break
			}
		}

		// If filepath.Dir returns "\", fix to "/"
		parent = filepath.ToSlash(parent)

		// Safety: Check if we are still within valid source scope?
		// AggregateLevel checks GetReducedMetadata.

		logger.Info("Heatmap: Percolating up to " + parent)
		// Force manual scan (true) to ensure we Re-Aggregate and don't skip based on cache age.
		// Percolation implies something changed below, so we must update.
		_, err := AggregateLevel(sourceName, parent, nil, true)
		if err != nil {
			logger.Error("Heatmap: PercolateUp failed for " + parent + ": " + err.Error())
			break
		}

		if parent == "/" {
			break
		}
		currentPath = parent
	}
}

// CountFilesRecursive counts total images in path and subfolders
func CountFilesRecursive(idx *indexing.Index, rootPath string) int {
	count := 0
	var walk func(path string)
	walk = func(path string) {
		dirInfo, exists := idx.GetReducedMetadata(path, true)
		if exists {
			for _, file := range dirInfo.Files {
				if iteminfo.IsImage(file.Name) {
					count++
				}
			}
			for _, sub := range dirInfo.Folders {
				nextPath := filepath.ToSlash(filepath.Join(path, sub.Name))
				walk(nextPath)
			}
		}
	}
	walk(rootPath)
	return count
}
