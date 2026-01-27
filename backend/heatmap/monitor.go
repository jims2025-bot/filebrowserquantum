package heatmap

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
)

// monitorActiveScans periodically logs a summary of currently active folder scans
func monitorActiveScans() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		var activeList []string

		activeFolders.Range(func(key, value interface{}) bool {
			info := value.(activeFolderInfo)
			itemDuration := time.Since(info.StartTime).Round(time.Second)

			// Format: " - /path/to/folder (150 files) [4m30s]"
			activeList = append(activeList, fmt.Sprintf(" - %s (%d files) [%s]",
				info.Path, info.FileCount, itemDuration))
			return true
		})

		if len(activeList) > 0 {
			// Sort for consistent output
			sort.Strings(activeList)

			summary := fmt.Sprintf("Heatmap [SUMMARY]: %d active scans:\n%s",
				len(activeList), strings.Join(activeList, "\n"))
			logger.Info(summary)
		}
	}
}
