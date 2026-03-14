package heatmap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

const MapOverlaysFilename = "mapoverlays.json"

type MapOverlay struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Path        string    `json:"path"`   // Relative to source root
	Source      string    `json:"source"` // Source name
	Type        string    `json:"type"`   // geojson, pmtiles
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
}

type OverlayData struct {
	GeneratedAt time.Time    `json:"generated_at"`
	Overlays    []MapOverlay `json:"overlays"`
}

// ScanLocalOverlays scans the given folder for .geojson files and returns them as MapOverlays.
// It reads the file content to extract "name" or "description" if available in the top-level JSON.
func ScanLocalOverlays(sourceName, folderPath string) ([]MapOverlay, error) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return nil, fmt.Errorf("index not found for %s", sourceName)
	}

	realPath, _, err := idx.GetRealPath(folderPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(realPath)
	if err != nil {
		return nil, err
	}

	var overlays []MapOverlay

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".geojson" || ext == ".pmtiles" || ext == ".pmtile" {
			logger.Info(fmt.Sprintf("[Overlays] Found map file: %s (ext: %s) in %s", entry.Name(), ext, realPath))
			info, err := entry.Info()
			if err != nil {
				continue
			}

			fullPath := filepath.Join(realPath, entry.Name())

			// Default values
			overlay := MapOverlay{
				Name:    entry.Name(),
				Path:    filepath.ToSlash(filepath.Join(folderPath, entry.Name())),
				Source:  sourceName,
				Type:    strings.TrimPrefix(ext, "."),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			}

			if ext == ".geojson" {
				// Try to read metadata from file
				bytes, err := os.ReadFile(fullPath)
				if err == nil {
					var meta struct {
						Name        string `json:"name"`
						Description string `json:"description"`
						Properties  struct {
							Name        string `json:"name"`
							Description string `json:"description"`
						} `json:"properties"`
					}
					if err := json.Unmarshal(bytes, &meta); err == nil {
						if meta.Name != "" {
							overlay.Name = meta.Name
						} else if meta.Properties.Name != "" {
							overlay.Name = meta.Properties.Name
						}

						if meta.Description != "" {
							overlay.Description = meta.Description
						} else if meta.Properties.Description != "" {
							overlay.Description = meta.Properties.Description
						}
					}
				}
			}

			// OVERRIDE with sidecar metadata if present (persistent edits)
			metaPath := fullPath + ".meta.json"
			if bytes, err := os.ReadFile(metaPath); err == nil {
				var sidecar struct {
					Name        string `json:"name"`
					Description string `json:"description"`
				}
				if err := json.Unmarshal(bytes, &sidecar); err == nil {
					if sidecar.Name != "" {
						overlay.Name = sidecar.Name
					}
					if sidecar.Description != "" {
						overlay.Description = sidecar.Description
					}
				}
			}

			overlays = append(overlays, overlay)
		}
	}

	return overlays, nil
}

// AggregateOverlayLevel aggregates local overlays and child overlays into mapoverlays.json
func AggregateOverlayLevel(sourceName, rootPath string) ([]MapOverlay, error) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return nil, fmt.Errorf("index not found")
	}

	// 1. Get Local Overlays
	localOverlays, err := ScanLocalOverlays(sourceName, rootPath)
	if err != nil {
		// Log but continue, maybe just empty
		// logger.Error("Overlay Scan Failed: " + err.Error())
	}

	// 2. Get Child Overlays
	var childOverlays []MapOverlay
	dirInfo, exists := idx.GetReducedMetadata(rootPath, true)
	if exists {
		for _, sub := range dirInfo.Folders {
			subName := sub.Name
			nextPath := filepath.ToSlash(filepath.Join(rootPath, subName))

			realPath, _, err := idx.GetRealPath(nextPath)
			if err == nil {
				overlayPath := filepath.Join(realPath, MapOverlaysFilename)
				if _, err := os.Stat(overlayPath); err == nil {
					bytes, err := os.ReadFile(overlayPath)
					if err == nil {
						var data OverlayData
						if err := json.Unmarshal(bytes, &data); err == nil {
							// Adjust paths if needed (usually they are absolute-ish from source root)
							// But ScanLocalOverlays constructs them as joined path.
							// So we just append.
							childOverlays = append(childOverlays, data.Overlays...)
						}
					}
				}
			}
		}
	}

	// 3. Aggregate
	allOverlays := append(localOverlays, childOverlays...)

	// Deduplicate?
	// If a child moves, we might have duplicates until next scan.
	// Map by Path to dedup
	uniqueMap := make(map[string]MapOverlay)
	for _, o := range allOverlays {
		uniqueMap[o.Path] = o
	}

	// Convert back to slice
	finalOverlays := make([]MapOverlay, 0, len(uniqueMap))
	for _, o := range uniqueMap {
		finalOverlays = append(finalOverlays, o)
	}

	// Sort by Path
	sort.Slice(finalOverlays, func(i, j int) bool {
		return finalOverlays[i].Path < finalOverlays[j].Path
	})

	// 4. Save
	realPath, _, err := idx.GetRealPath(rootPath)
	if err == nil {
		data := OverlayData{
			GeneratedAt: time.Now(),
			Overlays:    finalOverlays,
		}

		outPath := filepath.Join(realPath, MapOverlaysFilename)

		// Atomic write
		bytes, _ := json.Marshal(data)
		tmpPath := outPath + ".tmp"
		if err := os.WriteFile(tmpPath, bytes, 0644); err == nil {
			os.Rename(tmpPath, outPath)
		}
	}

	return finalOverlays, nil
}

// PercolateOverlaysUp updates mapoverlays.json for all parents of the given path
func PercolateOverlaysUp(sourceName, startPath string) {
	currentPath := startPath
	for {
		// Move up
		parent := filepath.Dir(currentPath)
		if parent == currentPath || parent == "." {
			if currentPath == "/" {
				break
			}
			if parent == "." {
				break
			}
		}
		parent = filepath.ToSlash(parent)

		// logger.Info("Overlay: Percolating up to " + parent)
		_, err := AggregateOverlayLevel(sourceName, parent)
		if err != nil {
			logger.Error("Overlay: PercolateUp failed for " + parent + ": " + err.Error())
			break
		}

		if parent == "/" {
			break
		}
		currentPath = parent
	}
}

// ScanOverlaysRecursive performs a full recursive scan for overlays
func ScanOverlaysRecursive(sourceName, rootPath string) error {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return fmt.Errorf("index not found")
	}

	// 1. Recurse first (Depth First, Post-Order Traversal)
	dirInfo, exists := idx.GetReducedMetadata(rootPath, true)
	if exists {
		for _, sub := range dirInfo.Folders {
			nextPath := filepath.ToSlash(filepath.Join(rootPath, sub.Name))
			_ = ScanOverlaysRecursive(sourceName, nextPath)
		}
	}

	// 2. Aggregate this level (after children are done)
	_, err := AggregateOverlayLevel(sourceName, rootPath)
	return err
}

// SaveOverlayMetadata saves persistent metadata to a sidecar file and triggers percolation.
func SaveOverlayMetadata(sourceName, overlayPath, name, description string) error {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return fmt.Errorf("index not found for source %s", sourceName)
	}

	realPath, _, err := idx.GetRealPath(overlayPath)
	if err != nil {
		return err
	}

	meta := struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}{
		Name:        name,
		Description: description,
	}

	bytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}

	metaPath := realPath + ".meta.json"
	logger.Info("[Overlays] Saving persistent metadata to " + metaPath)
	err = os.WriteFile(metaPath, bytes, 0644)
	if err != nil {
		return err
	}

	// Trigger percolation to update parent mapoverlays.json files
	folderPath := filepath.ToSlash(filepath.Dir(overlayPath))
	logger.Info("[Overlays] Triggering percolation for " + folderPath)
	PercolateOverlaysUp(sourceName, folderPath)

	return nil
}
