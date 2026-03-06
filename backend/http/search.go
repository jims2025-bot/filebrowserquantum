package http

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/people"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

// searchHandler handles search requests for files based on the provided query.
//
// This endpoint processes a search query, retrieves relevant file paths, and
// returns a JSON response with the search results. The search is performed
// against the file index, which is built from the root directory specified in
// the server's configuration. The results are filtered based on the user's scope.
//
// The handler expects the following headers in the request:
// - SessionId: A unique identifier for the user's session.
// - UserScope: The scope of the user, which influences the search context.
//
// The request URL should include a query parameter named `query` that specifies
// the search terms to use. The response will include an array of searchResponse objects
// containing the path, type, and dir status.
//
// Example request:
//
//	GET api/search?query=myfile
//
// Example response:
// [
//
//	{
//	    "path": "/path/to/myfile.txt",
//	    "type": "text"
//	},
//	{
//	    "path": "/path/to/mydir/",
//	    "type": "directory"
//	}
//
// ]
//
// @Summary Search Files
// @Description Searches for files matching the provided query. Returns file paths and metadata based on the user's session and scope.
// @Tags Search
// @Accept json
// @Produce json
// @Param query query string true "Search query"
// @Param scope query string false "path within user scope to search, for example '/first/second' to search within the second directory only"
// @Param SessionId header string false "User session ID, add unique value to prevent collisions"
// @Success 200 {array} indexing.SearchResult "List of search results"
// @Failure 400 {object} map[string]string "Bad Request"
// @Router /api/search [get]
func searchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	query := r.URL.Query().Get("query")

	// Intercept Facial Recognition searches
	personRegex := regexp.MustCompile(`(?i)(person|face):"([^"]+)"`)
	personMatches := personRegex.FindAllStringSubmatch(query, -1)

	if len(personMatches) > 0 {
		combinedResults := []indexing.SearchResult{}
		var seenPaths = make(map[string]bool)

		// Extract unique person names
		nameMap := make(map[string]bool)
		var personNames []string
		for _, pm := range personMatches {
			name := pm[2]
			if !nameMap[name] {
				nameMap[name] = true
				personNames = append(personNames, name)
			}
		}

		// Determine logic (default AND, switch to OR if " OR " found)
		logic := "AND"
		if strings.Contains(strings.ToUpper(query), " OR ") {
			logic = "OR"
		}

		// Pagination params
		limit := 1000 // default
		if l := r.URL.Query().Get("limit"); l != "" {
			fmt.Sscanf(l, "%d", &limit)
		}
		offset := 0
		if o := r.URL.Query().Get("offset"); o != "" {
			fmt.Sscanf(o, "%d", &offset)
		}
		// Context params
		scope := r.URL.Query().Get("scope")
		source := r.URL.Query().Get("source")
		pathPrefix := ""

		if scope != "" || source != "" {
			scopePath, realSource, err := ResolveScopePath(d.user, source, scope)
			if err == nil {
				idx := indexing.GetIndex(realSource)
				if idx != nil {
					diskPath, _, err := idx.GetRealPath("/", scopePath)
					if err == nil {
						pathPrefix = diskPath
					}
				}
			}
		}

		var matches []people.PersonMatch
		var err error
		if len(personNames) > 1 || logic == "OR" {
			matches, err = people.SearchImagesByPeople(personNames, logic, pathPrefix, limit, offset, d.user.ID)
		} else {
			matches, err = people.SearchImagesByPerson(personNames[0], pathPrefix, limit, offset, d.user.ID)
		}

		if err != nil {
			return http.StatusInternalServerError, err
		}

		for _, m := range matches {
			virtualPath, matchedSource := indexing.GetSourcePath(m.ImagePath)
			if matchedSource != "" {
				idx := indexing.GetIndex(matchedSource)
				if idx != nil {
					boxParam := ""
					if m.Box != "" {
						boxParam = "&box=" + url.QueryEscape(m.Box)
					}
					// URL encoding for Path and Source, preserving slashes
					encodedPath := (&url.URL{Path: virtualPath}).String()
					encodedSource := url.QueryEscape(matchedSource)
					thumbUrl := fmt.Sprintf("/api/preview%s?source=%s%s&size=thumb", encodedPath, encodedSource, boxParam)

					if !seenPaths[m.ImagePath] {
						combinedResults = append(combinedResults, indexing.SearchResult{
							Path:         virtualPath,
							Type:         "image",
							Size:         0,
							ThumbnailUrl: thumbUrl,
							Box:          m.Box,
						})
						seenPaths[m.ImagePath] = true
					}
				}
			}
		}

		return renderJSON(w, r, combinedResults)
	}

	source := r.URL.Query().Get("source")
	if source == "" {
		source = config.Server.DefaultSource.Name
	} else {
		var err error
		// decode url encoded source name
		source, err = url.QueryUnescape(source)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
		}
	}
	scope := r.URL.Query().Get("scope")
	unencodedScope, err := url.QueryUnescape(scope)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	searchScope := strings.TrimPrefix(unencodedScope, ".")
	// Retrieve the User-Agent and X-Auth headers from the request
	sessionId := r.Header.Get("SessionId")
	var response []indexing.SearchResult

	sourcesToSearch := []string{source}
	if settings.IsVirtualSource(source) {
		sourcesToSearch = []string{}
		for _, src := range settings.Config.Server.Sources {
			sourcesToSearch = append(sourcesToSearch, src.Name)
		}
	}

	for _, src := range sourcesToSearch {
		index := indexing.GetIndex(src)
		if index == nil {
			if !settings.IsVirtualSource(source) {
				return http.StatusBadRequest, fmt.Errorf("index not found for source %s", src)
			}
			continue
		}
		userscope, realSource, err := settings.GetScopeFromSourceName(d.user.Scopes, src)
		if err != nil {
			if !settings.IsVirtualSource(source) {
				return http.StatusForbidden, err
			}
			continue
		}

		combinedPath := index.MakeIndexPath(filepath.Join(userscope, searchScope))
		srcResponse := index.Search(query, combinedPath, sessionId)

		for i := range srcResponse {
			srcResponse[i].Path = strings.TrimPrefix(srcResponse[i].Path, userscope)
			if srcResponse[i].Path == "" {
				srcResponse[i].Path = "/"
			}

			// Map to virtual path under ALL_SOURCES
			if settings.IsVirtualSource(source) {
				srcPath := strings.TrimPrefix(srcResponse[i].Path, "/")
				if srcPath == "" {
					srcResponse[i].Path = "/" + realSource
				} else {
					srcResponse[i].Path = "/" + realSource + "/" + srcPath
				}
			}
		}
		response = append(response, srcResponse...)
	}

	// Set the Content-Type header to application/json
	w.Header().Set("Content-Type", "application/json")
	return renderJSON(w, r, response)
}
