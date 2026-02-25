package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

const folderDetailsFilename = "folderdetails.json"
const peopleListFilename = "PeopleList.json"

// FolderDetails is the structure of the folderdetails.json file.
type FolderDetails struct {
	CreateDate      string   `json:"createDate"`
	FolderAddedDate string   `json:"folderAddedDate"` // date folder was discovered/added
	FolderNotes     string   `json:"folderNotes"`
	People          []string `json:"people"`
	OldestDate      string   `json:"oldestDate"`
	MostRecentDate  string   `json:"mostRecentDate"`
	FileCount       int      `json:"fileCount"`
	FileNotes       []string `json:"fileNotes"`
}

// PeopleList is the global people registry stored as PeopleList.json at the source root.
type PeopleList struct {
	People []string `json:"people"`
}

// resolveFolderPath resolves the real filesystem folder path for a given source+path.
func resolveFolderPath(d *requestContext, source, path string) (string, error) {
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return "", err
	}
	idx := indexing.GetIndex(realSource)
	if idx == nil {
		return "", os.ErrNotExist
	}
	realPath, isDir, err := idx.GetRealPath(scopePath)
	if err != nil {
		return "", err
	}
	if !isDir {
		realPath = filepath.Dir(realPath)
	}
	return realPath, nil
}

// getFolderDetailsHandler handles GET /api/folderdetails
// Returns the contents of folderdetails.json for the requested folder, or an empty default.
func getFolderDetailsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	folderPath, err := resolveFolderPath(d, source, path)
	if err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, nil
		}
		return http.StatusForbidden, err
	}

	detailsFile := filepath.Join(folderPath, folderDetailsFilename)
	data, err := os.ReadFile(detailsFile)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty default — don't create the file on a read
			empty := FolderDetails{
				People:    []string{},
				FileNotes: []string{},
			}
			w.Header().Set("Content-Type", "application/json")
			return http.StatusOK, json.NewEncoder(w).Encode(empty)
		}
		return http.StatusInternalServerError, err
	}

	var details FolderDetails
	if err := json.Unmarshal(data, &details); err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	return http.StatusOK, json.NewEncoder(w).Encode(details)
}

// putFolderDetailsHandler handles PUT /api/folderdetails
// Writes (creates if necessary) folderdetails.json in the requested folder.
// Requires modify permission.
func putFolderDetailsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Modify {
		return http.StatusForbidden, nil
	}

	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	var incoming FolderDetails
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		return http.StatusBadRequest, err
	}

	folderPath, err := resolveFolderPath(d, source, path)
	if err != nil {
		if os.IsNotExist(err) {
			return http.StatusNotFound, nil
		}
		return http.StatusForbidden, err
	}

	detailsFile := filepath.Join(folderPath, folderDetailsFilename)

	// Preserve createDate from the existing file if it already exists.
	existingData, errRead := os.ReadFile(detailsFile)
	if errRead == nil {
		var existing FolderDetails
		if json.Unmarshal(existingData, &existing) == nil && existing.CreateDate != "" {
			incoming.CreateDate = existing.CreateDate
		}
		if json.Unmarshal(existingData, &existing) == nil && existing.FolderAddedDate != "" {
			incoming.FolderAddedDate = existing.FolderAddedDate
		}
	}

	// Set createDate on first creation.
	if incoming.CreateDate == "" {
		incoming.CreateDate = time.Now().UTC().Format(time.RFC3339)
	}
	if incoming.FolderAddedDate == "" {
		incoming.FolderAddedDate = time.Now().UTC().Format(time.RFC3339)
	}

	// Normalise People slice — remove blanks.
	cleaned := incoming.People[:0]
	for _, p := range incoming.People {
		if strings.TrimSpace(p) != "" {
			cleaned = append(cleaned, strings.TrimSpace(p))
		}
	}
	incoming.People = cleaned

	out, err := json.MarshalIndent(incoming, "", "  ")
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if err := os.WriteFile(detailsFile, out, 0644); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}

// getPeopleListHandler handles GET /api/peoplelist
// Returns the global PeopleList.json from the root of the requested source.
func getPeopleListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")

	_, realSource, err := ResolveScopePath(d.user, source, "/")
	if err != nil {
		return http.StatusForbidden, err
	}
	idx := indexing.GetIndex(realSource)
	if idx == nil {
		return http.StatusNotFound, nil
	}

	rootPath := idx.Source.Path
	listFile := filepath.Join(rootPath, peopleListFilename)

	data, err := os.ReadFile(listFile)
	if err != nil {
		if os.IsNotExist(err) {
			empty := PeopleList{People: []string{}}
			w.Header().Set("Content-Type", "application/json")
			return http.StatusOK, json.NewEncoder(w).Encode(empty)
		}
		return http.StatusInternalServerError, err
	}

	var list PeopleList
	if err := json.Unmarshal(data, &list); err != nil {
		return http.StatusInternalServerError, err
	}

	w.Header().Set("Content-Type", "application/json")
	return http.StatusOK, json.NewEncoder(w).Encode(list)
}

// putPeopleListHandler handles PUT /api/peoplelist
// Merges the supplied names into the global PeopleList.json (deduped, sorted).
// Requires modify permission.
func putPeopleListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.Modify {
		return http.StatusForbidden, nil
	}

	source := r.URL.Query().Get("source")

	var incoming PeopleList
	if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
		return http.StatusBadRequest, err
	}

	_, realSource, err := ResolveScopePath(d.user, source, "/")
	if err != nil {
		return http.StatusForbidden, err
	}
	idx := indexing.GetIndex(realSource)
	if idx == nil {
		return http.StatusNotFound, nil
	}

	rootPath := idx.Source.Path
	listFile := filepath.Join(rootPath, peopleListFilename)

	// Load existing list and merge.
	existing := PeopleList{People: []string{}}
	if data, err := os.ReadFile(listFile); err == nil {
		json.Unmarshal(data, &existing) //nolint:errcheck
	}

	seen := make(map[string]struct{})
	for _, p := range existing.People {
		t := strings.TrimSpace(p)
		if t != "" {
			seen[t] = struct{}{}
		}
	}
	for _, p := range incoming.People {
		t := strings.TrimSpace(p)
		if t != "" {
			seen[t] = struct{}{}
		}
	}

	merged := make([]string, 0, len(seen))
	for p := range seen {
		merged = append(merged, p)
	}
	sort.Strings(merged)

	out, err := json.MarshalIndent(PeopleList{People: merged}, "", "  ")
	if err != nil {
		return http.StatusInternalServerError, err
	}

	if err := os.WriteFile(listFile, out, 0644); err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
