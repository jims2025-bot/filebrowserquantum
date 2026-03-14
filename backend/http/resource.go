package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/errors"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/utils"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
	"github.com/jims2025-bot/filebrowserquantum/backend/preview"
)

// resourceGetHandler retrieves information about a resource.
// @Summary Get resource information
// @Description Returns metadata and optionally file contents for a specified resource path.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "Path to the resource"
// @Param source query string false "Source name for the desired source, default is used if not provided"
// @Param source query string false "Name for the desired source, default is used if not provided"
// @Param content query string false "Include file content if true"
// @Param checksum query string false "Optional checksum validation"
// @Success 200 {object} iteminfo.FileInfo "Resource metadata"
// @Failure 404 {object} map[string]string "Resource not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources [get]
func resourceGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	encodedPath := r.URL.Query().Get("path")
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
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	// Parse scope index and resolve path handling cross-scope permissions
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}

	// NEW: Handle Virtual Root Listing (ALL_SOURCES)
	// Only trigger aggregator view if this isn't a physical source.
	_, isPhysical := settings.Config.Server.NameToSource[realSource]
	if settings.IsVirtualSource(realSource) && !isPhysical && scopePath == "/" {
		virtualRoot := iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{
				ItemInfo: iteminfo.ItemInfo{
					Name:    realSource,
					Type:    "directory",
					ModTime: settings.Config.Server.ServerStart,
				},
				Files: []iteminfo.ItemInfo{}, // Ensure items is an empty slice, not nil
				Path:  "/",
			},
			Source: realSource,
		}
		// Populate Folders with user scopes to help navigation
		for _, s := range d.user.Scopes {
			name := s.Alias
			if name == "" {
				name = filepath.Base(s.Name)
			}
			virtualRoot.Folders = append(virtualRoot.Folders, iteminfo.ItemInfo{
				Name:    name,
				Type:    "directory",
				ModTime: settings.Config.Server.ServerStart,
			})
		}
		return renderJSON(w, r, virtualRoot)
	}

	// Restore userscope for path trimming logic later
	// We use the original source string because that's what the scopes are mapped to (or aliases)
	userscope, _, _ := settings.GetScopeFromSourceString(d.user.Scopes, source)

	source = realSource
	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Path:    scopePath,
		Modify:  d.user.Permissions.Modify,
		Source:  source,
		Expand:  true,
		Content: r.URL.Query().Get("content") == "true",
	})
	if err != nil {
		return errToStatus(err), err
	}
	if userscope != "/" {
		// Case-insensitive trimming for Windows compatibility
		if len(fileInfo.Path) >= len(userscope) && strings.EqualFold(fileInfo.Path[:len(userscope)], userscope) {
			fileInfo.Path = fileInfo.Path[len(userscope):]
		} else {
			fileInfo.Path = strings.TrimPrefix(fileInfo.Path, userscope)
		}
	}
	if fileInfo.Path == "" {
		fileInfo.Path = "/"
	}
	if fileInfo.Type == "directory" {
		return renderJSON(w, r, fileInfo)
	}
	if algo := r.URL.Query().Get("checksum"); algo != "" {
		idx := indexing.GetIndex(source)
		if idx == nil {
			return http.StatusNotFound, fmt.Errorf("source %s not found", source)
		}
		// Use scopePath (absolute) with root scope
		realPath, _, _ := idx.GetRealPath("/", scopePath)
		checksums, err := files.GetChecksum(realPath, algo)
		if err == errors.ErrInvalidOption {
			return http.StatusBadRequest, nil
		} else if err != nil {
			return http.StatusInternalServerError, err
		}
		fileInfo.Checksums = checksums
	}
	return renderJSON(w, r, fileInfo)

}

// resourceDeleteHandler deletes a resource at a specified path.
// resourceDeleteHandler deletes a resource at a specified path.
func resourceDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// SECURITY: File Deletion is explicitly DISABLED by user request.
	// EXCEPTION: Allow removing EXIF/GPS metadata if "action=exif" is specified.
	action := r.URL.Query().Get("action")
	if action == "exif" {
		if !d.user.Permissions.Admin {
			return http.StatusForbidden, fmt.Errorf("user is not allowed to modify file metadata")
		}

		path := r.URL.Query().Get("path")
		source := r.URL.Query().Get("source")
		if source == "" {
			source = config.Server.DefaultSource.Name
		} else {
			var err error
			source, err = url.QueryUnescape(source)
			if err != nil {
				return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
			}
		}

		encodedPath := r.URL.Query().Get("path")
		path, err := url.QueryUnescape(encodedPath)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
		}

		// Resolve scope path
		scopePath, realSource, err := ResolveScopePath(d.user, source, path)
		if err != nil {
			return http.StatusForbidden, err
		}
		source = realSource

		fileOpts := iteminfo.FileOptions{
			Path:   scopePath,
			Source: source,
			Modify: d.user.Permissions.Admin,
			Expand: false,
		}

		// Resolve Real Path via Indexing/Files Adapter
		fileInfo, err := files.FileInfoFaster(fileOpts)
		if err != nil {
			return errToStatus(err), err
		}

		// Perform EXIF Removal
		err = files.RemoveExif(fileInfo.RealPath)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		return http.StatusOK, nil
	}

	return http.StatusForbidden, fmt.Errorf("file deletion is disabled by administrator")
}

// resourcePostHandler creates or uploads a new resource.
func resourcePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = config.Server.DefaultSource.Name
	} else {
		var err error
		source, err = url.QueryUnescape(source)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
		}
	}
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}
	// Parse scope index and resolve path handling cross-scope permissions
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}
	source = realSource

	fileOpts := iteminfo.FileOptions{
		Path:   scopePath,
		Source: source,
		Modify: d.user.Permissions.Admin,
		Expand: false,
	}
	if strings.HasSuffix(path, "/") {
		err = files.WriteDirectory(fileOpts)
		if err != nil {
			return errToStatus(err), err
		}
		return http.StatusOK, nil
	}
	fileInfo, err := files.FileInfoFaster(fileOpts)
	if err == nil {
		if r.URL.Query().Get("override") != "true" {
			logger.Debugf("Resource already exists: %v", fileInfo.RealPath)
			return http.StatusConflict, nil
		}
		if !d.user.Permissions.Admin {
			return http.StatusForbidden, nil
		}
		preview.DelThumbs(r.Context(), fileInfo)
	}
	err = files.WriteFile(fileOpts, r.Body)
	if err != nil {
		return errToStatus(err), err
	}
	return http.StatusOK, nil
}

// resourcePutHandler updates an existing file resource.
func resourcePutHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	if source == "" {
		source = config.Server.DefaultSource.Name
	} else {
		var err error
		source, err = url.QueryUnescape(source)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
		}
	}
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	if strings.HasSuffix(path, "/") {
		return http.StatusMethodNotAllowed, nil
	}
	if strings.HasSuffix(path, "/") {
		return http.StatusMethodNotAllowed, nil
	}
	// Parse scope index and resolve path handling cross-scope permissions
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}
	source = realSource

	fileOpts := iteminfo.FileOptions{
		Path:   scopePath,
		Source: source,
		Modify: d.user.Permissions.Admin,
		Expand: false,
	}
	// Handle EXIF updates
	action := r.URL.Query().Get("action")
	if action == "exif" {
		// Read the JSON body: { "latitude": ..., "longitude": ... }
		var coords struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return http.StatusBadRequest, err
		}
		defer r.Body.Close()

		if err = json.Unmarshal(body, &coords); err != nil {
			return http.StatusBadRequest, err
		}

		// Resolve RealPath
		info, err := files.FileInfoFaster(fileOpts)
		if err != nil {
			return errToStatus(err), err
		}

		err = files.UpdateExif(info.RealPath, coords.Latitude, coords.Longitude)
		return errToStatus(err), err
	}

	err = files.WriteFile(fileOpts, r.Body)
	return errToStatus(err), err
}

// resourcePatchHandler performs a patch operation (move/rename) on a resource.
func resourcePatchHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	action := r.URL.Query().Get("action")
	if !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("user is not allowed to create or modify")
	}
	encodedFrom := r.URL.Query().Get("from")
	src, err := url.QueryUnescape(encodedFrom)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	dst := r.URL.Query().Get("destination")
	dst, err = url.QueryUnescape(dst)
	if err != nil {
		return errToStatus(err), err
	}
	splitSrc := strings.Split(src, "::")
	if len(splitSrc) <= 1 {
		return http.StatusBadRequest, fmt.Errorf("invalid source path: %v", src)
	}
	srcIndex := splitSrc[0]
	src = splitSrc[1]
	splitDst := strings.Split(dst, "::")
	if len(splitDst) <= 1 {
		return http.StatusBadRequest, fmt.Errorf("invalid destination path: %v", dst)
	}
	dstIndex := splitDst[0]
	dst = splitDst[1]
	if dst == "/" || src == "/" {
		return http.StatusForbidden, fmt.Errorf("forbidden: source or destination is attempting to modify root")
	}
	if dst == "/" || src == "/" {
		return http.StatusForbidden, fmt.Errorf("forbidden: source or destination is attempting to modify root")
	}
	var realSourceDst, realSourceSrc string
	userscopeDst, realSourceDst, err := settings.GetScopeFromSourceString(d.user.Scopes, dstIndex)
	if err != nil {
		return http.StatusForbidden, err
	}
	userscopeSrc, realSourceSrc, err := settings.GetScopeFromSourceString(d.user.Scopes, srcIndex)
	if err != nil {
		return http.StatusForbidden, err
	}
	// Use real source names
	dstIndex = realSourceDst
	srcIndex = realSourceSrc

	idx := indexing.GetIndex(dstIndex)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", dstIndex)
	}
	parentDir, _, err := idx.GetRealPath(userscopeDst, filepath.Dir(dst))
	if err != nil {
		logger.Debugf("Could not get real path for parent dir: %v %v %v", userscopeDst, filepath.Dir(dst), err)
		return http.StatusNotFound, err
	}
	realDest := parentDir + "/" + filepath.Base(dst)
	idx2 := indexing.GetIndex(srcIndex)
	if idx2 == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", srcIndex)
	}
	realSrc, isSrcDir, err := idx2.GetRealPath(userscopeSrc, src)
	if err != nil {
		return http.StatusNotFound, err
	}
	overwrite := r.URL.Query().Get("overwrite") == "true"
	rename := r.URL.Query().Get("rename") == "true"
	if rename {
		realDest = addVersionSuffix(realDest)
	}
	if overwrite && !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("forbidden: user does not have permission to overwrite file")
	}
	err = patchAction(r.Context(), action, realSrc, realDest, d, isSrcDir, srcIndex, dstIndex)
	if err != nil {
		logger.Debugf("Could not run patch action. src=%v dst=%v err=%v", realSrc, realDest, err)
	}
	return errToStatus(err), err
}

func addVersionSuffix(source string) string {
	counter := 1
	dir, name := path.Split(source)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for {
		if _, err := os.Stat(source); err != nil {
			break
		}
		renamed := fmt.Sprintf("%s(%d)%s", base, counter, ext)
		source = path.Join(dir, renamed)
		counter++
	}
	return source
}

func patchAction(ctx context.Context, action, src, dst string, d *requestContext, isSrcDir bool, srcIndex, destIndex string) error {
	switch action {
	case "copy":
		err := files.CopyResource(srcIndex, destIndex, src, dst)
		return err
	case "rename", "move":
		idx := indexing.GetIndex(srcIndex)
		srcPath := idx.MakeIndexPath(src)
		fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
			Path:       srcPath,
			Source:     srcIndex,
			IsDir:      isSrcDir,
			Modify:     d.user.Permissions.Admin,
			Expand:     false,
			ReadHeader: false,
		})
		if err != nil {
			return err
		}
		preview.DelThumbs(ctx, fileInfo)
		return files.MoveResource(srcIndex, destIndex, src, dst)
	default:
		return fmt.Errorf("unsupported action %s: %w", action, errors.ErrInvalidRequestParams)
	}
}

func inspectIndex(w http.ResponseWriter, r *http.Request) {
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = config.Server.DefaultSource.Name
	} else {
		source, _ = url.QueryUnescape(source)
	}
	path, _ := url.QueryUnescape(encodedPath)
	isNotDir := r.URL.Query().Get("isDir") == "false"
	index := indexing.GetIndex(source)
	if index == nil {
		http.Error(w, "source not found", http.StatusNotFound)
		return
	}
	info, _ := index.GetReducedMetadata(path, !isNotDir)
	renderJSON(w, r, info) // nolint:errcheck
}

func mockData(w http.ResponseWriter, r *http.Request) {
	d := r.URL.Query().Get("numDirs")
	f := r.URL.Query().Get("numFiles")
	NumDirs, err := strconv.Atoi(d)
	numFiles, err2 := strconv.Atoi(f)
	if err != nil || err2 != nil {
		return
	}
	mockDir := utils.CreateMockData(NumDirs, numFiles)
	renderJSON(w, r, mockDir) // nolint:errcheck
}

// resourceFixThumbnailsHandler recursively fixes thumbnail resolution for a folder.
func resourceFixThumbnailsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	// 1. Verify Admin (handled by middleware, but double-check if needed or rely on withAdmin)
	// Middleware 'withAdmin' should be used when routing.

	path := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if source == "" {
		source = config.Server.DefaultSource.Name
	} else {
		var err error
		source, err = url.QueryUnescape(source)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
		}
	}

	// 2. Resolve Path
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}
	source = realSource

	// 3. Get Real Path on Disk
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}
	realPath, _, err := idx.GetRealPath(scopePath)
	if err != nil {
		return http.StatusNotFound, err
	}

	// 4. Execute Fix
	err = files.FixThumbnailResolution(realPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
func resourceRebuildThumbnailsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.RebuildThumbnails && !d.user.Permissions.Admin {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to rebuild thumbnails")
	}

	encodedPath := r.URL.Query().Get("path")
	path, _ := url.QueryUnescape(encodedPath)
	source := r.URL.Query().Get("source")
	if source == "" {
		source = config.Server.DefaultSource.Name
	} else {
		source, _ = url.QueryUnescape(source)
	}

	// Resolve scope path
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		return http.StatusForbidden, err
	}
	source = realSource

	// Get Real Path via Indexing/Files Adapter
	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source %s not found", source)
	}

	realFolderPath, isDir, err := idx.GetRealPath(scopePath)
	if err != nil {
		return http.StatusNotFound, err
	}

	if !isDir {
		return http.StatusBadRequest, fmt.Errorf("rebuild thumbnails: path is not a directory")
	}

	// Walk the directory and clear thumbnails for each file
	err = filepath.Walk(realFolderPath, func(fpath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Create mock FileInfo for DelThumbs
		fileInfo := iteminfo.ExtendedFileInfo{
			FileInfo: iteminfo.FileInfo{
				ItemInfo: iteminfo.ItemInfo{
					ModTime: info.ModTime(),
					Name:    info.Name(),
				},
			},
			RealPath: fpath,
			Source:   source,
		}

		preview.DelThumbs(r.Context(), fileInfo)
		return nil
	})

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, map[string]string{"status": "success"})
}
