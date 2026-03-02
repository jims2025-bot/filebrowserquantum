package iptcindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
)

const Filename = "iptcindex.json"

// IPTCEntry holds metadata presence/values for a single image file.
type IPTCEntry struct {
	// HasNotes is true when any of the following are non-empty:
	//   XMP:Instructions, Photoshop:Instructions,
	//   IPTC:Caption-Abstract, IPTC:Description, IPTC:By-line
	HasNotes bool `json:"hasNotes"`
	// DateTaken is the EXIF:DateTimeOriginal value (e.g. "2023:07:04 14:32:00"), empty if absent.
	DateTaken string `json:"dateTaken"`
}

// IPTCIndex is the structure stored in each folder's iptcindex.json.
// ALL image files in the folder are included, even if HasNotes=false and DateTaken="".
type IPTCIndex struct {
	GeneratedAt time.Time            `json:"generatedAt"`
	Files       map[string]IPTCEntry `json:"files"` // keyed by filename
}

// noteKeys are the ExifTool group:field keys we check for HasNotes.
var noteKeys = []string{
	"XMP:Instructions",
	"Photoshop:Instructions",
	"IPTC:Caption-Abstract",
	"IPTC:Description",
	"IPTC:By-line",
}

// dateTakenKeys are the keys we check for DateTaken (first non-empty wins).
var dateTakenKeys = []string{
	"EXIF:DateTimeOriginal",
	"EXIF:CreateDate",
}

// ReadIndex reads an existing iptcindex.json from a real folder path.
// Returns nil (no error) if the file does not exist.
func ReadIndex(realFolderPath string) (*IPTCIndex, error) {
	indexPath := filepath.Join(realFolderPath, Filename)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var idx IPTCIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// ScanFolder reads IPTC/EXIF metadata for all image files in a single folder
// and writes iptcindex.json. It uses the background ExifTool bridge.
func ScanFolder(sourceName, folderPath string) error {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return fmt.Errorf("iptcindex: source '%s' not found", sourceName)
	}

	realPath, _, err := idx.GetRealPath(folderPath)
	if err != nil {
		return fmt.Errorf("iptcindex: cannot resolve path '%s': %w", folderPath, err)
	}

	// Collect image file paths in this folder (non-recursive).
	dirInfo, exists := idx.GetReducedMetadata(folderPath, true)
	if !exists {
		return nil // empty / unknown folder — skip silently
	}

	var imagePaths []string
	var imageNames []string
	for _, f := range dirInfo.Files {
		if iteminfo.IsImage(f.Name) {
			imagePaths = append(imagePaths, filepath.Join(realPath, f.Name))
			imageNames = append(imageNames, f.Name)
		}
	}

	if len(imagePaths) == 0 {
		return nil // nothing to scan in this folder
	}

	scanStart := time.Now()

	result := IPTCIndex{
		GeneratedAt: time.Now(),
		Files:       make(map[string]IPTCEntry, len(imageNames)),
	}

	bridge, err := files.GetExifToolBridgeBackground()
	if err != nil {
		return fmt.Errorf("iptcindex: cannot get ExifTool bridge: %w", err)
	}

	metaArray, err := bridge.GetMetadataBulk(imagePaths)
	if err != nil {
		return fmt.Errorf("iptcindex: GetMetadataBulk failed for '%s': %w", folderPath, err)
	}

	withNotes := 0
	withDate := 0
	for i, meta := range metaArray {
		if i >= len(imageNames) {
			break
		}
		name := imageNames[i]

		// Check HasNotes
		hasNotes := false
		for _, key := range noteKeys {
			if val, ok := meta[key]; ok {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					hasNotes = true
					break
				}
			}
		}
		if hasNotes {
			withNotes++
		}

		// Check DateTaken
		dateTaken := ""
		for _, key := range dateTakenKeys {
			if val, ok := meta[key]; ok {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					dateTaken = strings.TrimSpace(s)
					break
				}
			}
		}
		if dateTaken != "" {
			withDate++
		}

		result.Files[name] = IPTCEntry{
			HasNotes:  hasNotes,
			DateTaken: dateTaken,
		}
	}

	logger.Info(fmt.Sprintf("IPTCIndex [DONE ]: %s — %d images, %d with notes, %d with date — took %s",
		folderPath, len(imagePaths), withNotes, withDate, time.Since(scanStart).Round(time.Millisecond)))

	// Write iptcindex.json atomically
	outPath := filepath.Join(realPath, Filename)
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	tmpPath := outPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// ScanRecursive walks the folder tree rooted at rootPath and calls ScanFolder
// on each directory (children first, then parent).
func ScanRecursive(sourceName, rootPath string) error {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return fmt.Errorf("iptcindex: source '%s' not found", sourceName)
	}

	dirInfo, exists := idx.GetReducedMetadata(rootPath, true)
	if exists {
		for _, sub := range dirInfo.Folders {
			nextPath := filepath.ToSlash(filepath.Join(rootPath, sub.Name))
			if err := ScanRecursive(sourceName, nextPath); err != nil {
				logger.Error(fmt.Sprintf("IPTCIndex: ScanRecursive error at '%s': %v", nextPath, err))
			}
		}
	}

	return ScanFolder(sourceName, rootPath)
}

// ScanAllSources runs ScanRecursive only within user-scoped paths for every known index.
// Called by the jobs scheduler. Mirrors the scope-filtering logic of heatmap.ScanAllSources.
func ScanAllSources(store *storage.Storage) {
	logger.Info("IPTCIndex: ScanAllSources starting")
	allStart := time.Now()

	allIndexes := indexing.GetIndexes()
	pathToSource := make(map[string]string)
	for name, idx := range allIndexes {
		pathToSource[filepath.ToSlash(filepath.Clean(idx.Source.Path))] = name
	}

	allUsers, err := store.Users.Gets()
	if err != nil {
		logger.Error("IPTCIndex: ScanAllSources failed to retrieve users: " + err.Error())
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
			logger.Info(fmt.Sprintf("IPTCIndex: Scanning source '%s' scope '%s'", sourceName, path))
			srcStart := time.Now()
			if err := ScanRecursive(sourceName, path); err != nil {
				logger.Error(fmt.Sprintf("IPTCIndex: Source '%s' scope '%s' failed: %v", sourceName, path, err))
			} else {
				logger.Info(fmt.Sprintf("IPTCIndex: Source '%s' scope '%s' complete — took %s",
					sourceName, path, time.Since(srcStart).Round(time.Second)))
			}
		}
	}

	logger.Info(fmt.Sprintf("IPTCIndex: ScanAllSources complete — total time %s", time.Since(allStart).Round(time.Second)))
}
