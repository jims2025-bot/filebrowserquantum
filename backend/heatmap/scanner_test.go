package heatmap

import (
	"testing"
)

func TestClusterPoints(t *testing.T) {
	// Setup 3 points:
	// A and B are very close (same location)
	// C is far away
	p1 := Cluster{
		ID:     "p1",
		Lat:    40.0,
		Lon:    -74.0,
		Count:  1,
		Points: []ClusterPoint{{ID: "pt1", Lat: 40.0, Lon: -74.0}},
		Min:    [2]float64{40, -74}, Max: [2]float64{40, -74},
	}
	p2 := Cluster{
		ID:     "p2",
		Lat:    40.00001, // Very close (within 0.0005 radius)
		Lon:    -74.0,
		Count:  1,
		Points: []ClusterPoint{{ID: "pt2", Lat: 40.00001, Lon: -74.0}},
		Min:    [2]float64{40.00001, -74}, Max: [2]float64{40.00001, -74},
	}
	p3 := Cluster{
		ID:     "p3",
		Lat:    41.0, // Far
		Lon:    -74.0,
		Count:  1,
		Points: []ClusterPoint{{ID: "pt3", Lat: 41.0, Lon: -74.0}},
		Min:    [2]float64{41, -74}, Max: [2]float64{41, -74},
	}

	points := []Cluster{p1, p2, p3}
	radius := 0.0005

	clusters := clusterPoints(points, radius)

	if len(clusters) != 2 {
		t.Errorf("Expected 2 clusters, got %d", len(clusters))
	}

	// Find the large cluster
	var largeCluster *Cluster
	for i := range clusters {
		if clusters[i].Count == 2 {
			largeCluster = &clusters[i]
			break
		}
	}

	if largeCluster == nil {
		t.Fatal("Expected to find a cluster with count 2")
	}

	if len(largeCluster.Points) != 2 {
		t.Errorf("Expected large cluster to have 2 points, got %d", len(largeCluster.Points))
	}
}
