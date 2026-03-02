package http

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
	"github.com/jims2025-bot/filebrowserquantum/backend/preview"
)

type FileCache interface {
	Store(ctx context.Context, key string, value []byte) error
	Load(ctx context.Context, key string) ([]byte, bool, error)
	Delete(ctx context.Context, key string) error
}

// previewHandler handles the preview request for images.
// @Summary Get image preview
// @Description Returns a preview image based on the requested path and size.
// @Tags Resources
// @Accept json
// @Produce json
// @Param path query string true "File path of the image to preview"
// @Param size query string false "Preview size ('small' or 'large'). Default is based on server config."
// @Success 200 {file} file "Preview image content"
// @Failure 202 {object} map[string]string "Download permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File not found"
// @Failure 415 {object} map[string]string "Unsupported file type for preview"
// @Failure 500 {object} map[string]string "Internal server error"
// @Failure 501 {object} map[string]string "Preview generation not implemented"
// @Router /api/preview [get]
func previewHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	logger.Debug(fmt.Sprintf("[previewHandler] ENTRY: Method=%s, Path=%s, Remote=%s", r.Method, r.URL.Path, r.RemoteAddr))
	if config.Server.DisablePreviews {
		return http.StatusNotImplemented, fmt.Errorf("preview is disabled")
	}
	encodedPath := r.URL.Query().Get("path")
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	// NEW: Check for REST-style path in URL wildcard {path...} or manual extraction
	restPath := r.PathValue("path")
	if restPath == "" {
		// Fallback for standard prefix matching /preview/
		if strings.HasPrefix(r.URL.Path, "/preview/") {
			restPath = strings.TrimPrefix(r.URL.Path, "/preview/")
		}
	}

	if restPath != "" && path == "" {
		if !strings.HasPrefix(restPath, "/") {
			restPath = "/" + restPath
		}
		path = restPath
	}
	source := r.URL.Query().Get("source")
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	} else {
		var err error
		// decode url encoded source name
		source, err = url.QueryUnescape(source)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
		}
	}

	logger.Debug(fmt.Sprintf("Preview: Incoming request - path='%s', source='%s'", path, source))

	if path == "" {
		return http.StatusBadRequest, fmt.Errorf("invalid request path")
	}
	// Parse scope index and resolve path handling cross-scope permissions
	scopePath, realSource, err := ResolveScopePath(d.user, source, path)
	if err != nil {
		logger.Error(fmt.Sprintf("Preview: ResolveScopePath failed - path='%s', source='%s', error='%v'", path, source, err))
		return http.StatusForbidden, err
	}
	source = realSource

	logger.Debug(fmt.Sprintf("Preview: Resolved - scopePath='%s', realSource='%s'", scopePath, realSource))

	fileInfo, err := files.FileInfoFaster(iteminfo.FileOptions{
		Path:   scopePath,
		Modify: d.user.Permissions.Modify,
		Source: source,
		Expand: true,
	})

	if err != nil {
		logger.Error("Preview: FileInfoFaster failed for path " + scopePath + ": " + err.Error())
		return errToStatus(err), err
	}
	d.fileInfo = fileInfo

	val, err := previewHelperFunc(w, r, d)
	if err != nil {
		logger.Error("Preview: previewHelperFunc failed for " + d.fileInfo.RealPath + ": " + err.Error())
	}
	return val, err
}

func rawFileHandler(w http.ResponseWriter, r *http.Request, file iteminfo.ExtendedFileInfo) (int, error) {
	idx := indexing.GetIndex(file.Source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source not found: %s", file.Source)
	}
	realPath, _, _ := idx.GetRealPath(file.Path)
	fd, err := os.Open(realPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	setContentDisposition(w, r, file.Name)

	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, file.Name, file.ModTime, fd)
	return 0, nil
}

func previewHelperFunc(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	previewSize := r.URL.Query().Get("size")
	if previewSize != "small" && previewSize != "thumb" {
		previewSize = "large"
	}
	if d.fileInfo.Type == "directory" {
		return http.StatusBadRequest, fmt.Errorf("can't create preview for directory")
	}
	setContentDisposition(w, r, d.fileInfo.Name)
	isImage := strings.HasPrefix(d.fileInfo.Type, "image")
	if config.Server.DisableResize && isImage {
		return rawFileHandler(w, r, d.fileInfo)
	}
	if !preview.AvailablePreview(d.fileInfo) {
		if isImage {
			return rawFileHandler(w, r, d.fileInfo)
		}
		return http.StatusNotImplemented, fmt.Errorf("can't create preview for %s type", d.fileInfo.Type)
	}
	seekPercentage := 0
	percentage := r.URL.Query().Get("atPercentage")
	if percentage != "" {
		var err error
		// convert string to int
		seekPercentage, err = strconv.Atoi(percentage)
		if err != nil {
			seekPercentage = 10
		}
		if seekPercentage < 0 || seekPercentage > 100 {
			seekPercentage = 10
		}
	}

	officeUrl := ""
	if d.fileInfo.OnlyOfficeId != "" {
		pathUrl := fmt.Sprintf("/api/raw?files=%s::%s", d.fileInfo.Source, d.fileInfo.Path)
		pathUrl = pathUrl + "&auth=" + d.token
		if settings.Config.Server.InternalUrl != "" {
			officeUrl = config.Server.InternalUrl + pathUrl
		} else {
			scheme := "http"
			if r.TLS != nil {
				scheme = "https"
			}
			officeUrl = scheme + "://" + r.Host + pathUrl
		}
	}

	boxParam := r.URL.Query().Get("box")

	previewImg, err := preview.GetPreviewForFile(d.fileInfo, previewSize, officeUrl, seekPercentage, boxParam)
	if err != nil {
		// Log as error (warning level not available in this logger)
		logger.Error("Preview generation failed for " + d.fileInfo.RealPath + ": " + err.Error())
		return http.StatusUnprocessableEntity, err // 422 instead of 500
	}
	w.Header().Set("Cache-Control", "private")
	http.ServeContent(w, r, d.fileInfo.RealPath, d.fileInfo.ModTime, bytes.NewReader(previewImg))
	return 0, nil
}
