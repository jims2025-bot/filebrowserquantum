package indexing

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"

	"github.com/shirou/gopsutil/v3/disk"
)

var DiskUsageCache = cache.NewCache(30 * time.Second)

// UpdateFileMetadata updates the FileInfo for the specified directory in the index.
func (idx *Index) UpdateMetadata(info *iteminfo.FileInfo) bool {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.Directories[info.Path] = info
	return true
}

// GetMetadataInfo retrieves the FileInfo from the specified file or directory in the index.
func (idx *Index) GetReducedMetadata(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	checkDir := idx.MakeIndexPath(target)
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target))
	}
	if checkDir == "" {
		checkDir = "/"
	}
	dir, exists := idx.Directories[checkDir]
	if !exists {
		// Case-insensitive fallback for Windows compatibility
		for path, d := range idx.Directories {
			if strings.EqualFold(path, checkDir) {
				dir = d
				exists = true
				break
			}
		}
		if !exists {
			return nil, false
		}
	}

	if isDir {
		return dir, true
	}
	// handle file
	if checkDir == "/" {
		checkDir = ""
	}
	baseName := filepath.Base(target)
	for _, item := range dir.Files {
		if item.Name == baseName {
			return &iteminfo.FileInfo{
				Path:     checkDir + "/" + item.Name,
				ItemInfo: item,
			}, true
		}
	}
	return nil, false

}

// raw directory info retrieval -- does not work on files, only returns a directory
func (idx *Index) GetMetadataInfo(target string, isDir bool) (*iteminfo.FileInfo, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	checkDir := idx.MakeIndexPath(target)
	if !isDir {
		checkDir = idx.MakeIndexPath(filepath.Dir(target))
	}
	if checkDir == "" {
		checkDir = "/"
	}
	dir, exists := idx.Directories[checkDir]
	if !exists {
		// Case-insensitive fallback for Windows compatibility
		for path, d := range idx.Directories {
			if strings.EqualFold(path, checkDir) {
				dir = d
				exists = true
				break
			}
		}
	}
	return dir, exists
}

func (idx *Index) RemoveDirectory(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.NumDeleted++
	delete(idx.Directories, path)
}

func GetIndex(name string) *Index {
	indexesMutex.Lock()
	defer indexesMutex.Unlock()

	// 1. Direct match in indices map (fast path)
	if index, ok := indexes[name]; ok {
		return index
	}

	// 2. Virtual sources
	if settings.IsVirtualSource(name) {
		return nil
	}

	// 3. Try lookup via settings NameToSource (robust to name mismatches)
	if src, ok := settings.Config.Server.NameToSource[name]; ok {
		if idx, ok := indexes[src.Name]; ok {
			return idx
		}
	}

	// 4. Try lookup via settings SourceMap (resolves indices by their physical path)
	if src, ok := settings.Config.Server.SourceMap[name]; ok {
		if idx, ok := indexes[src.Name]; ok {
			return idx
		}
	}

	logger.Errorf("index %s not found (tried direct, name, and path)", name)
	return nil
}

func GetIndexInfo(sourceName string) (ReducedIndex, error) {
	idx, ok := indexes[sourceName]
	if !ok {
		return ReducedIndex{}, fmt.Errorf("index %s not found", sourceName)
	}
	sourcePath := idx.Path
	cacheKey := "usageCache-" + sourceName
	_, ok = DiskUsageCache.Get(cacheKey).(bool)
	if !ok {
		usage, err := disk.Usage(sourcePath)
		if err != nil {
			logger.Errorf("error getting disk usage for %s: %v", sourcePath, err)
			idx.SetStatus(UNAVAILABLE)
			return ReducedIndex{}, fmt.Errorf("error getting disk usage for %s: %v", sourcePath, err)
		}
		latestUsage := DiskUsage{
			Total: usage.Total,
			Used:  usage.Used,
		}
		idx.SetUsage(latestUsage)
		DiskUsageCache.Set(cacheKey, true)
	}
	return idx.ReducedIndex, nil
}
