package heatmap

import "time"

// Cluster represents a group of images or sub-clusters.
type Cluster struct {
	Lat   float64    `json:"lat"`
	Lon   float64    `json:"lon"`
	Count int        `json:"count"`
	Min   [2]float64 `json:"min"` // [Lat, Lon] min bounds
	Max   [2]float64 `json:"max"` // [Lat, Lon] max bounds
	// For leaf nodes (folder level)
	PreviewID string `json:"previewID,omitempty"`
	Path      string `json:"path,omitempty"`   // Path to the image or folder
	Source    string `json:"source,omitempty"` // Derived source name

	// Individual points contained in this cluster (for spiderfy)
	Points []ClusterPoint `json:"points,omitempty"`

	// Logic:
	// If Path is a folder, this is a folder aggregate.
	// If Path is a file, this is an image.
}

type ClusterPoint struct {
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Path      string  `json:"path"`
	PreviewID string  `json:"previewID"`
	Source    string  `json:"source,omitempty"`
}

// HeatmapData represents the persistent JSON data for a folder.
type HeatmapData struct {
	GeneratedAt time.Time `json:"generatedAt"`
	Clusters    []Cluster `json:"clusters"`
	TotalImages int       `json:"totalImages"`
}

// GlobalHeatmap represents the user-specific top-level view.
type GlobalHeatmap struct {
	Clusters []Cluster `json:"clusters"`
}
