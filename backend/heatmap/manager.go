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
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
)

type ScanProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

var (
	// ActiveScans tracks which paths are currently being scanned to prevent duplicates.
	// Key: "sourceName|path", Value: *ScanProgress
	activeScans sync.Map
)

// ScanSafe triggers a recursive scan for a given path if one is not already running.
// It is safe to call continuously; it will reject requests for paths already being processed.
func ScanSafe(sourceName, path string) error {
	key := sourceName + "|" + path
	progress := &ScanProgress{Current: 0, Total: 0, Message: "Starting scan..."}
	if _, loaded := activeScans.LoadOrStore(key, progress); loaded {
		return fmt.Errorf("scan already in progress for %s", path)
	}
	defer activeScans.Delete(key)

	logger.Info("Heatmap: Manually started safe scan for " + sourceName + " " + path)

	// Pre-count files for progress reporting
	idx := indexing.GetIndex(sourceName)
	if idx != nil {
		progress.Message = "Counting files..."
		progress.Total = CountFilesRecursive(idx, path)
		logger.Info(fmt.Sprintf("Heatmap: Found %d files to scan in %s", progress.Total, path))
	}

	ScanRecursive(sourceName, path, progress)
	return nil
}

// GetScanProgress returns the progress of a running scan, if any.
func GetScanProgress(sourceName, path string) *ScanProgress {
	key := sourceName + "|" + path
	if val, ok := activeScans.Load(key); ok {
		return val.(*ScanProgress)
	}
	return nil
}

// IsScanning checks if a scan is currently in progress for the given path.
func IsScanning(sourceName, path string) bool {
	key := sourceName + "|" + path
	_, loaded := activeScans.Load(key)
	return loaded
}

func StartJob(store *storage.Storage) {
	go func() {
		// Wait for initial index to populate (large repos take time)
		time.Sleep(30 * time.Second)
		for {
			logger.Info("Starting Heatmap Generation Job")

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
						ScanRecursive(sourceName, path, nil)
					}
				}
			}

			logger.Info("Finished Heatmap Generation Job")
			// Run every 24 hours
			time.Sleep(24 * time.Hour)
		}
	}()
}

// GetGlobalHeatmap aggregates all clusters from folders accessible to the user.
func GetGlobalHeatmap(user *users.User) (GlobalHeatmap, error) {
	var allClusters []Cluster

	// If user has restricted scopes, iterate them.
	// If user is admin and has no scopes, they might see everything, but let's check.
	// We'll iterate all indexes and filter by scope.

	allIndexes := indexing.GetIndexes()

	for _, idx := range allIndexes {
		validPaths := []string{}

		// If user has scopes for this source, use them.
		hasSourceScope := false
		for _, s := range user.Scopes {
			// Strict name match OR path match
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

		// If admin and no specific scope for this source (or globally), maybe allow root?
		// Existing behavior: Admin with no scopes usually means access to all defined sources.
		// If user has NO scopes defined in `user.Scopes`, and is Admin, we add "/" for all sources.
		if user.Permissions.Admin && len(user.Scopes) == 0 {
			validPaths = append(validPaths, "/")
		} else if !user.Permissions.Admin && !hasSourceScope {
			// Non-admin, no scope for this source -> no access
			continue
		}

		if len(validPaths) == 0 {
			continue
		}

		for _, rootPath := range validPaths {
			var scopeClusters []Cluster
			walkHeatmap(idx, idx.Source.Name, rootPath, &scopeClusters)

			// Trim the scope path from the cluster paths so they are relative to the user's view
			for i := range scopeClusters {
				if rootPath != "/" {
					// Handle case-insensitive prefix trimming if on Windows?
					// For now, strict trim.
					// Ensure we check for slash consistency
					scopeClusters[i].Path = strings.TrimPrefix(scopeClusters[i].Path, rootPath)
					if !strings.HasPrefix(scopeClusters[i].Path, "/") && scopeClusters[i].Path != "" {
						scopeClusters[i].Path = "/" + scopeClusters[i].Path
					}

					// Update Children Points too
					for j := range scopeClusters[i].Points {
						scopeClusters[i].Points[j].Path = strings.TrimPrefix(scopeClusters[i].Points[j].Path, rootPath)
						if !strings.HasPrefix(scopeClusters[i].Points[j].Path, "/") && scopeClusters[i].Points[j].Path != "" {
							scopeClusters[i].Points[j].Path = "/" + scopeClusters[i].Points[j].Path
						}
					}
				}
			}
			allClusters = append(allClusters, scopeClusters...)
		}
	}

	return GlobalHeatmap{Clusters: allClusters}, nil
}

// GetFolderHeatmap recursively aggregates clusters for a specific folder.
func GetFolderHeatmap(sourceName, path string) (HeatmapData, error) {
	logger.Info("Heatmap: GetFolderHeatmap for Source=" + sourceName + " Path=" + path)
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		// Try resolving alias (e.g., PHOTOS:0 -> PHOTOS)
		// This is common if the frontend uses indexed source names
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

	var allClusters []Cluster
	walkHeatmap(idx, sourceName, path, &allClusters)

	// Create a synthetic HeatmapData
	return HeatmapData{
		GeneratedAt: time.Now(),
		Clusters:    allClusters,
		TotalImages: len(allClusters), // Approximate
	}, nil
}

func walkHeatmap(idx *indexing.Index, sourceName, currentPath string, collection *[]Cluster) {
	// Check if this folder has heatmap.json
	realPath, _, err := idx.GetRealPath(currentPath)
	if err == nil {
		heatmapPath := filepath.Join(realPath, HeatmapFilename)
		if _, err := os.Stat(heatmapPath); err == nil {
			// Found heatmap
			bytes, err := os.ReadFile(heatmapPath)
			if err == nil {
				var data HeatmapData
				if err := json.Unmarshal(bytes, &data); err == nil {
					// logger.Info("Heatmap: Found " + strconv.Itoa(len(data.Clusters)) + " clusters in " + heatmapPath)

					// Fix paths and inject source
					for i := range data.Clusters {
						data.Clusters[i].Source = sourceName

						// Rebase path to ensure it matches the current location (handles folder moves/renames)
						// Helper to fix a single path string
						fixPath := func(originalPath string) string {
							if originalPath == "" {
								return ""
							}
							return filepath.ToSlash(filepath.Join(currentPath, filepath.Base(originalPath)))
						}

						if data.Clusters[i].Path != "" {
							data.Clusters[i].Path = fixPath(data.Clusters[i].Path)
						}

						// Rebase children points
						for j := range data.Clusters[i].Points {
							data.Clusters[i].Points[j].Path = fixPath(data.Clusters[i].Points[j].Path)
							data.Clusters[i].Points[j].Source = sourceName
						}
					}
					*collection = append(*collection, data.Clusters...)
				} else {
					logger.Error("Heatmap: Failed to unmarshal " + heatmapPath + ": " + err.Error())
				}
			} else {
				logger.Error("Heatmap: Failed to read " + heatmapPath + ": " + err.Error())
			}
		} else {
			// Logger might be too noisy if we log every missing file
			// logger.Info("Heatmap: No heatmap.json in " + realPath)
		}
	} else {
		logger.Error("Heatmap: Failed to get real path for " + currentPath + ": " + err.Error())
	}

	// Recurse
	dirInfo, exists := idx.GetReducedMetadata(currentPath, true)
	if exists {
		for _, sub := range dirInfo.Folders {
			// IMPORTANT: index paths typically use forward slash, but filepath.Join uses OS separator.
			// We must ensure internal paths are slash-separated for index lookups.
			nextPath := filepath.ToSlash(filepath.Join(currentPath, sub.Name))
			walkHeatmap(idx, sourceName, nextPath, collection)
		}
	} else {
		// logger.Info("Heatmap: No reduced metadata for " + currentPath)
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

// Recursive Scanner Wrapper
func ScanRecursive(sourceName, rootPath string, progress *ScanProgress) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return
	}

	// Internal recursive function
	var walk func(path string)
	walk = func(path string) {
		if progress != nil {
			progress.Message = "Scanning " + filepath.Base(path)
		}

		// Scan current folder
		_, _ = ScanFolder(sourceName, path, progress)

		// Scan subfolders
		dirInfo, exists := idx.GetReducedMetadata(path, true)
		if exists {
			for _, sub := range dirInfo.Folders {
				// Ensure forward slashes for index compatibility
				nextPath := filepath.ToSlash(filepath.Join(path, sub.Name))
				walk(nextPath)
			}
		}
	}

	walk(rootPath)
}
