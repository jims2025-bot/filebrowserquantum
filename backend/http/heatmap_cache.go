package http

import (
	"sync"
	"time"

	"github.com/jims2025-bot/filebrowserquantum/backend/heatmap"
)

// Tile cache to avoid reloading heatmap data for every tile request
type heatmapCache struct {
	mu   sync.RWMutex
	data map[string]*cachedHeatmap
}

type cachedHeatmap struct {
	data      heatmap.HeatmapData
	timestamp time.Time
}

var tileCache = &heatmapCache{
	data: make(map[string]*cachedHeatmap),
}

const cacheTTL = 5 * time.Minute

// getCachedHeatmap returns cached heatmap data or loads it if not cached
func getCachedHeatmap(source, path string, loadFunc func() (heatmap.HeatmapData, error)) (heatmap.HeatmapData, error) {
	key := source + ":" + path

	// Try to get from cache
	tileCache.mu.RLock()
	cached, exists := tileCache.data[key]
	tileCache.mu.RUnlock()

	if exists && time.Since(cached.timestamp) < cacheTTL {
		return cached.data, nil
	}

	// Load fresh data
	data, err := loadFunc()
	if err != nil {
		return heatmap.HeatmapData{}, err
	}

	// Store in cache
	tileCache.mu.Lock()
	tileCache.data[key] = &cachedHeatmap{
		data:      data,
		timestamp: time.Now(),
	}
	tileCache.mu.Unlock()

	return data, nil
}

// ClearHeatmapCache clears the tile cache for a specific source/path
func ClearHeatmapCache(source, path string) {
	key := source + ":" + path
	tileCache.mu.Lock()
	delete(tileCache.data, key)
	tileCache.mu.Unlock()
}
