package heatmap

import (
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

// ScanAllSources runs the full heatmap scan across all user scopes.
// It mirrors the inner loop of StartJob but is exported so the job scheduler
// can call it directly (manual or scheduled trigger).
func ScanAllSources(store *storage.Storage) {
	logger.Info("Heatmap: ScanAllSources starting")

	allIndexes := indexing.GetIndexes()
	pathToSource := make(map[string]string)
	for name, idx := range allIndexes {
		pathToSource[filepath.ToSlash(filepath.Clean(idx.Source.Path))] = name
	}

	allUsers, err := store.Users.Gets()
	if err != nil {
		logger.Error("Heatmap: ScanAllSources failed to retrieve users: " + err.Error())
		return
	}

	locationsToScan := make(map[string]map[string]struct{})

	for _, u := range allUsers {
		if len(u.Scopes) == 0 || u.Permissions.Admin {
			continue
		}
		for _, s := range u.Scopes {
			var properSourceName string
			if _, ok := allIndexes[s.Name]; ok {
				properSourceName = s.Name
			} else if strings.Contains(s.Name, ":") {
				parts := strings.Split(s.Name, ":")
				if len(parts) > 0 {
					if _, ok := allIndexes[parts[0]]; ok {
						properSourceName = parts[0]
					}
				}
			}
			if properSourceName == "" {
				clean := filepath.ToSlash(filepath.Clean(s.Name))
				if name, ok := pathToSource[clean]; ok {
					properSourceName = name
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

	for sourceName, paths := range locationsToScan {
		for path := range paths {
			logger.Info("Heatmap: Scanning scope " + sourceName + " " + path)
			sem := make(chan struct{}, 20)
			_, err := ScanRecursive(sourceName, path, nil, sem, false)
			if err == nil {
				PercolateUp(sourceName, path)
			} else {
				logger.Error("Heatmap: ScanAllSources failed Source=" + sourceName + " Path=" + path + ": " + err.Error())
			}
		}
	}

	logger.Info("Heatmap: ScanAllSources complete")
}
