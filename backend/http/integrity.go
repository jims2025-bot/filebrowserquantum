package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/integrity"
)

type scanRequest struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

func integrityScanHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// 1. Verify Admin
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, nil
	}

	// 2. Parse Code
	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}

	logger.Info("Admin initiated integrity scan for: " + req.Source + " " + req.Path)

	// 3. Resolve Index
	idx := indexing.GetIndex(req.Source)
	if idx == nil {
		return http.StatusNotFound, nil
	}

	// 4. Run Scan (Synchronously is okay for single folder? It might take a few seconds)
	// User interface expects a start and done.
	// We can run it and return results, or run async.
	// Let's run synchronously so we can return "Scan Complete" with confidence.
	// If it takes too long (>30s), it might timeout, but single folder exiftool is usually fast.
	integrity.ScanFolder(idx, req.Path, nil)

	return http.StatusOK, nil
}

// getIntegrityIssuesHandler handles GET /api/integrity/issues
// Returns integrity issues for a file or folder
func getIntegrityIssuesHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, err
	}

	if source == "" || path == "" {
		return http.StatusBadRequest, nil
	}

	// Validate user has access to this source/path
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}

	idx := indexing.GetIndex(realSource)
	if idx == nil {
		return http.StatusNotFound, nil
	}

	realPath, isDir, err := idx.GetRealPath(scopePath)
	if err != nil {
		return http.StatusNotFound, err
	}

	// Determine the folder containing exif_issues.json
	var issuesFolder string
	if isDir {
		issuesFolder = realPath
	} else {
		issuesFolder = filepath.Dir(realPath)
	}

	issuesFile := filepath.Join(issuesFolder, integrity.IssueFilename)

	// Read exif_issues.json if it exists
	data, err := os.ReadFile(issuesFile)
	if err != nil {
		// If file doesn't exist, return empty response (no issues)
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			if isDir {
				w.Write([]byte("{}"))
			} else {
				w.Write([]byte("null"))
			}
			return http.StatusOK, nil
		}
		return http.StatusInternalServerError, err
	}

	var report integrity.IssueReport
	if err := json.Unmarshal(data, &report); err != nil {
		return http.StatusInternalServerError, err
	}

	if isDir {
		// Return map of filename -> issue details for all files in folder
		issuesMap := make(map[string]integrity.ExifResult)
		for _, problem := range report.Problems {
			filename := filepath.Base(problem.SourceFile)
			issuesMap[filename] = problem
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(issuesMap)
	} else {
		// Return issue details for specific file
		filename := filepath.Base(realPath)
		for _, problem := range report.Problems {
			if filepath.Base(problem.SourceFile) == filename {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(problem)
				return http.StatusOK, nil
			}
		}
		// File not in issues list
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("null"))
	}

	return http.StatusOK, nil
}
