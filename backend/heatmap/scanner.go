package heatmap

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
)

const HeatmapFilename = "heatmap.json"
const ClusterRadius = 0.0005 // Approx 50 meters, adjust as needed

// ScanFolder scans a specific folder for images, extracts GPS, clusters them, and saves metadata.
// It returns the Clusters found in this folder (not including subfolders).
// Recursive: If true, it also triggers scans for subfolders, but the return value is still just *this* folder's direct clusters?
// No, the requirement says "Each folder and sub-folder image location and counts should percolate up".
// So "Top-level heatmap aggregates folder clusters".
// This means for a folder F, we need to know the clusters of its subfolders too if we are at the top level?
// Actually, "Folder's heatmap data Consists of one or more spatial clusters... Folder clusters are used to generate markers".
// The top level map shows "Folder clusters".
// So for a folder P with child C, P's heatmap shows P's images (clustered) AND C's clusters (as markers?).
// No, "each folder may contain up to 5000 images". "A folder's heatmap data consists of one or more spatial clusters".
// "Top-level heatmap aggregates folder clusters from all accessible folders".
// So the scanner should simply generate clusters for the *images* in the current folder.
// And store that in heatmap.json.
// The aggregation logic (GlobalHeatmap) will read these heatmap.json files.

// GetLocalClusters scans a specific folder for images, extracts GPS and clusters them.
// It returns the Clusters found in this folder (local images only).
// It does NOT write to disk. That is handled by the Recursive Orchestrator.
func GetLocalClusters(sourceName, folderPath string, progress *ScanProgress) ([]Cluster, error) {
	// DEBUG LOG
	// logger.Info(fmt.Sprintf("[DebugScan] Scanning Source=%s Folder=%s", sourceName, folderPath))

	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		logger.Error("[DebugScan] Index not found for " + sourceName)
		return nil, nil
	}

	realPath, _, err := idx.GetRealPath(folderPath)
	if err != nil {
		logger.Error("[DebugScan] GetRealPath failed: " + err.Error())
		return nil, err
	}
	// logger.Info("[DebugScan] Real path resolved to: " + realPath)

	// 1. Find images in this folder
	// Force index refresh
	_, refreshErr := idx.RefreshFileInfo(iteminfo.FileOptions{
		Path:  folderPath,
		IsDir: true,
	})
	if refreshErr != nil {
		// logger.Error("[DebugScan] RefreshFileInfo failed: " + refreshErr.Error())
	}

	dirInfo, exists := idx.GetReducedMetadata(folderPath, true)
	if !exists {
		// logger.Error("[DebugScan] GetReducedMetadata returned FALSE (not found) for " + folderPath)
		return nil, nil
	}

	var points []Cluster

	// Helper for ID generation
	genID := func() string {
		return fmt.Sprintf("%d-%d", time.Now().UnixNano(), len(points))
	}

	// Process files
	for _, file := range dirInfo.Files {
		if !iteminfo.IsImage(file.Name) {
			continue
		}

		// Update Progress
		if progress != nil {
			progress.Current++
		}

		fullPath := filepath.Join(realPath, file.Name)
		lat, lon, err := files.GetGPS(fullPath)
		if err != nil {
			// No GPS or error
			continue
		}

		points = append(points, Cluster{
			ID:        genID(),
			Lat:       lat,
			Lon:       lon,
			Count:     1,
			Min:       [2]float64{lat, lon},
			Max:       [2]float64{lat, lon},
			PreviewID: file.Name,
			Path:      filepath.ToSlash(filepath.Join(folderPath, file.Name)),
			Points: []ClusterPoint{
				{
					Lat:       lat,
					Lon:       lon,
					Path:      filepath.ToSlash(filepath.Join(folderPath, file.Name)),
					PreviewID: file.Name,
					ID:        genID(), // Assign ID to point
				},
			},
		})
	}

	// logger.Info(fmt.Sprintf("[DebugScan] Found %d valid GPS points in %s", len(points), folderPath))

	// validGPSCount := len(points)
	totalImageCount := 0
	for _, file := range dirInfo.Files {
		if iteminfo.IsImage(file.Name) {
			totalImageCount++
		}
	}

	if totalImageCount > 0 {
		// logger.Info(fmt.Sprintf("Heatmap: scanned %s - Total Images: %d, Valid GPS: %d", folderPath, totalImageCount, validGPSCount))
	} else {
		// logger.Debug(fmt.Sprintf("Heatmap: scanned %s - No images found", folderPath))
	}

	// 2. Cluster the points (local only)
	clusters := clusterPoints(points, ClusterRadius)

	return clusters, nil
}

// Simple greedy clustering
func clusterPoints(points []Cluster, radius float64) []Cluster {
	var clusters []Cluster

	for _, p := range points {
		added := false
		for i := range clusters {
			c := &clusters[i]
			// Check distance (simple Euclidean for now, accurate enough for small "folder" scale clustering usually,
			// but for global we might need HAversine. Let's use simple for speed on 5000 items)
			// Actually, lat/lon degrees vary. But for "nearby" logic it might suffice.
			// Better: use crude distance.
			if distance(p.Lat, p.Lon, c.Lat, c.Lon) < radius {
				// Merge
				c.Lat = (c.Lat*float64(c.Count) + p.Lat) / float64(c.Count+1)
				c.Lon = (c.Lon*float64(c.Count) + p.Lon) / float64(c.Count+1)
				c.Count += p.Count
				c.Min[0] = math.Min(c.Min[0], p.Lat)
				c.Min[1] = math.Min(c.Min[1], p.Lon)
				c.Max[0] = math.Max(c.Max[0], p.Lat)
				c.Max[1] = math.Max(c.Max[1], p.Lon)

				// Keep the ID of the larger cluster or generate new?
				// Ideally stable ID if possible, but merging makes it a new entity.
				// Re-using c.ID is fine for stability of the "anchor".

				// Accumulate points
				if len(c.Points) > 0 || len(p.Points) > 0 {
					// Update IDs of merged points to match the parent cluster ID
					// This ensures consistency
					for j := range p.Points {
						p.Points[j].ID = c.ID
					}
					c.Points = append(c.Points, p.Points...)
				}

				// PreviewID usually keeps the first one or logic to pick "best"
				added = true
				break
			}
		}
		if !added {
			clusters = append(clusters, p)
		}
	}
	return clusters
}

func distance(lat1, lon1, lat2, lon2 float64) float64 {
	return math.Sqrt(math.Pow(lat1-lat2, 2) + math.Pow(lon1-lon2, 2))
}
