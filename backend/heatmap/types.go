package heatmap

import "time"

// HeatmapVersion is the current version of the heatmap.json format
// Increment this when the JSON structure changes to force re-scanning
// Version 5: Added TotalImageCount
// Version 6: Force full re-scan
const HeatmapVersion = 6

// Cluster represents a group of images or sub-clusters.
type Cluster struct {
	ID    string     `json:"id"`
	Lat   float64    `json:"lat"`
	Lon   float64    `json:"lon"`
	Count int        `json:"count"`
	Min   [2]float64 `json:"min"` // [Lat, Lon] min bounds
	Max   [2]float64 `json:"max"` // [Lat, Lon] max bounds
	// For leaf nodes (folder level)
	PreviewID       string `json:"previewID,omitempty"`
	Path            string `json:"path,omitempty"`   // Path to the image or folder
	Source          string `json:"source,omitempty"` // Derived source name
	TotalImageCount int    `json:"totalImageCount,omitempty"`

	// Individual points contained in this cluster (for spiderfy)
	Points []ClusterPoint `json:"points,omitempty"`

	// Logic:
	// If Path is a folder, this is a folder aggregate.
	// If Path is a file, this is an image.
}

type ClusterPoint struct {
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
	Path            string  `json:"path"`
	PreviewID       string  `json:"previewID"`
	Source          string  `json:"source,omitempty"`
	TotalImageCount int     `json:"totalImageCount,omitempty"`
	// Additional fields for inspection drill-down
	Type  string `json:"type,omitempty"` // "folder" or "image"
	Count int    `json:"count"`          // For folder count
	ID    string `json:"id"`             // Cluster ID to pass to next step
}

// HeatmapData represents the persistent JSON data for a folder.
type HeatmapData struct {
	Version     int       `json:"version"` // Format version for compatibility checking
	GeneratedAt time.Time `json:"generatedAt"`
	Clusters    []Cluster `json:"clusters"`
	TotalImages int       `json:"totalImages"`
}

// GlobalHeatmap represents the user-specific top-level view.
type GlobalHeatmap struct {
	Clusters []Cluster `json:"clusters"`
}
