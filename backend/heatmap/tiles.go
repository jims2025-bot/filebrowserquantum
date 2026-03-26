package heatmap

import (
	"math"
)

// TileBounds represents the geographic bounds of a map tile
type TileBounds struct {
	MinLat float64
	MaxLat float64
	MinLon float64
	MaxLon float64
}

// LatLon2Tile converts coordinates to tile coordinates
func LatLon2Tile(lat, lon float64, zoom int) (x, y int) {
	n := math.Pow(2, float64(zoom))
	x = int(math.Floor((lon + 180.0) / 360.0 * n))
	latRad := lat * math.Pi / 180.0
	y = int(math.Floor((1.0 - math.Log(math.Tan(latRad)+1.0/math.Cos(latRad))/math.Pi) / 2.0 * n))
	return
}

// Tile2LatLon converts tile coordinates to geographic bounds
// z = zoom level, x = tile X coordinate, y = tile Y coordinate
func Tile2LatLon(z, x, y int) TileBounds {
	n := math.Pow(2, float64(z))

	minLon := float64(x)/n*360.0 - 180.0
	maxLon := float64(x+1)/n*360.0 - 180.0

	minLat := lat2tile(float64(y+1), z, n)
	maxLat := lat2tile(float64(y), z, n)

	return TileBounds{
		MinLat: minLat,
		MaxLat: maxLat,
		MinLon: minLon,
		MaxLon: maxLon,
	}
}

// lat2tile converts tile Y coordinate to latitude
func lat2tile(y float64, z int, n float64) float64 {
	latRad := math.Atan(math.Sinh(math.Pi * (1 - 2*y/n)))
	return latRad * 180.0 / math.Pi
}

// ClusterInBounds checks if a cluster's center point falls within tile bounds
func ClusterInBounds(cluster *Cluster, bounds TileBounds) bool {
	return cluster.Lat >= bounds.MinLat &&
		cluster.Lat <= bounds.MaxLat &&
		cluster.Lon >= bounds.MinLon &&
		cluster.Lon <= bounds.MaxLon
}

// ClusterOverlapsBounds checks if any part of a cluster's geographic extent overlaps the bounds.
func ClusterOverlapsBounds(cluster *Cluster, bounds TileBounds) bool {
	// If cluster has no bounds (point-cluster), fallback to center check
	if cluster.Min[0] == 0 && cluster.Max[0] == 0 {
		return ClusterInBounds(cluster, bounds)
	}

	// Range A overlaps Range B if (A.min <= B.max) && (A.max >= B.min)
	overlapLat := (cluster.Min[0] <= bounds.MaxLat) && (cluster.Max[0] >= bounds.MinLat)
	overlapLon := (cluster.Min[1] <= bounds.MaxLon) && (cluster.Max[1] >= bounds.MinLon)

	return overlapLat && overlapLon
}

// FilterClustersByBounds returns only clusters within the tile bounds
func FilterClustersByBounds(clusters []Cluster, bounds TileBounds) []Cluster {
	filtered := make([]Cluster, 0)
	for _, cluster := range clusters {
		if ClusterInBounds(&cluster, bounds) {
			filtered = append(filtered, cluster)
		}
	}
	return filtered
}

// SimplifyClustersByZoom reduces cluster detail based on zoom level
// At low zoom levels, we aggregate more; at high zoom, we show individual photos
func SimplifyClustersByZoom(clusters []Cluster, zoom int) []Cluster {
	if zoom >= 13 {
		// High zoom (13+): return all details including individual points
		// This allows frontend to render fan markers and individual photos
		return clusters
	} else if zoom >= 9 {
		// Medium zoom (9-12): return clusters but limit points
		simplified := make([]Cluster, len(clusters))
		for i, cluster := range clusters {
			simplified[i] = cluster
			// Limit points to 10 per cluster at medium zoom
			if len(cluster.Points) > 10 {
				simplified[i].Points = cluster.Points[:10]
			}
		}
		return simplified
	} else {
		// Low zoom (1-8): return only cluster aggregates, no individual points
		simplified := make([]Cluster, len(clusters))
		for i, cluster := range clusters {
			simplified[i] = cluster
			simplified[i].Points = nil // Remove individual points
		}
		return simplified
	}
}
