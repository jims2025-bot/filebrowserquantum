package http

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/iptcindex"
)

// getIPTCIndexHandler returns the iptcindex.json for a folder.
// @Summary Get IPTC index for a folder
// @Description Returns per-file IPTC notes presence and DateTaken from the folder's iptcindex.json
// @Tags Files
// @Produce json
// @Param source query string true "Source name"
// @Param path   query string true "Folder path"
// @Success 200 {object} iptcindex.IPTCIndex
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/iptcindex [get]
func getIPTCIndexHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	encodedPath := r.URL.Query().Get("path")

	if source == "" || encodedPath == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}

	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %w", err)
	}

	// Resolve source alias (e.g. "PHOTOS:0" → "PHOTOS")
	if d.user.Username != "publicUser" {
		var realSource string
		path, realSource, err = ResolveScopePath(d.user, source, path)
		if err != nil {
			return http.StatusForbidden, fmt.Errorf("source %s not available: %w", source, err)
		}
		source = realSource
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		// Try stripping ":N" alias suffix
		if realSource, _, err2 := settings.GetScopeFromSourceName(d.user.Scopes, source); err2 == nil {
			idx = indexing.GetIndex(realSource)
			source = realSource
		}
		if idx == nil {
			return http.StatusNotFound, fmt.Errorf("source '%s' not found", source)
		}
	}

	realPath, _, err := idx.GetRealPath(path)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("path not found: %w", err)
	}

	data, err := iptcindex.ReadIndex(realPath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not read iptcindex: %w", err)
	}

	if data == nil {
		// Return empty index if file doesn't exist yet
		data = &iptcindex.IPTCIndex{
			Files: map[string]iptcindex.IPTCEntry{},
		}
	}

	return renderJSON(w, r, data)
}
