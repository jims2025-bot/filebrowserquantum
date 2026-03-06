package http

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/utils"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

func setContentDisposition(w http.ResponseWriter, r *http.Request, fileName string) {
	if r.URL.Query().Get("inline") == "true" {
		w.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+url.PathEscape(fileName))
	} else {
		// As per RFC6266 section 4.3
		w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+url.PathEscape(fileName))
	}
}

// rawHandler serves the raw content of a file, multiple files, or directory in various formats.
// @Summary Get raw content of a file, multiple files, or directory
// @Description Returns the raw content of a file, multiple files, or a directory. Supports downloading files as archives in various formats.
// @Tags Resources
// @Accept json
// @Produce json
// @Param files query string true "a list of files in the following format 'source::filename' and separated by '||' with additional items in the list. (required)"
// @Param inline query bool false "If true, sets 'Content-Disposition' to 'inline'. Otherwise, defaults to 'attachment'."
// @Param algo query string false "Compression algorithm for archiving multiple files or directories. Options: 'zip' and 'tar.gz'. Default is 'zip'."
// @Success 200 {file} file "Raw file or directory content, or archive for multiple files"
// @Failure 202 {object} map[string]string "Modify permissions required"
// @Failure 400 {object} map[string]string "Invalid request path"
// @Failure 404 {object} map[string]string "File or directory not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/raw [get]
func rawHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	logger.Debug(fmt.Sprintf("[rawHandler] ENTRY: Method=%s, Path=%s, Remote=%s", r.Method, r.URL.Path, r.RemoteAddr))
	encodedFiles := r.URL.Query().Get("files")
	// Decode the URL-encoded files list (for standard requests)
	files, err := url.QueryUnescape(encodedFiles)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid files encoding: %v", err)
	}

	// NEW: Check for REST-style path in URL wildcard {path...} or manual extraction
	restPath := r.PathValue("path")
	if restPath == "" {
		// Fallback for standard prefix matching /raw/
		if strings.HasPrefix(r.URL.Path, "/raw/") {
			restPath = strings.TrimPrefix(r.URL.Path, "/raw/")
		}
	}

	if restPath != "" && files == "" {
		source := r.URL.Query().Get("source")
		if source == "" {
			source = config.Server.DefaultSource.Name
		}
		// Convert to virtual file list format: "source::/path"
		if !strings.HasPrefix(restPath, "/") {
			restPath = "/" + restPath
		}
		files = source + "::" + restPath
	}

	if files == "" {
		return http.StatusBadRequest, fmt.Errorf("no files or path provided")
	}

	fileList := strings.Split(files, "||")
	return rawFilesHandler(w, r, d, fileList)
}

func addFile(path string, d *requestContext, tarWriter *tar.Writer, zipWriter *zip.Writer, flatten bool) error {
	splitFile := strings.Split(path, "::")
	if len(splitFile) != 2 {
		return fmt.Errorf("invalid file in files requested: %v", splitFile)
	}
	source := splitFile[0]
	path = splitFile[1]
	var err error
	userScope := "/"
	var realSource string
	if d.user.Username != "publicUser" {
		userScope, realSource, err = settings.GetScopeFromSourceString(d.user.Scopes, source)
		if d.share == nil && err != nil {
			return fmt.Errorf("source %s is not available for user %s", source, d.user.Username)
		}
		source = realSource
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return fmt.Errorf("source %s is not available", source)
	}

	realPath, _, err := idx.GetRealPath(userScope, path)
	if err != nil {
		return fmt.Errorf("failed to get real path for %s: %v", path, err)
	}
	info, err := os.Stat(realPath)
	if err != nil {
		return err
	}

	// Get the base name of the top-level folder or file
	baseName := filepath.Base(realPath)

	if info.IsDir() {
		// Walk through directory contents
		return filepath.Walk(realPath, func(filePath string, fileInfo os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Calculate the relative path
			relPath, err := filepath.Rel(realPath, filePath) // Use realPath directly
			if err != nil {
				return err
			}

			// Normalize for tar: convert \ to /
			relPath = filepath.ToSlash(relPath)

			// Skip adding `.` (current directory)
			if relPath == "." {
				return nil
			}

			// Prepend base folder name unless flatten is true
			if !flatten {
				relPath = filepath.Join(baseName, relPath)
				relPath = filepath.ToSlash(relPath) // Ensure normalized separators
			}

			if fileInfo.IsDir() {
				if tarWriter != nil {
					header := &tar.Header{
						Name:     relPath + "/",
						Mode:     0755,
						Typeflag: tar.TypeDir,
						ModTime:  fileInfo.ModTime(),
					}
					return tarWriter.WriteHeader(header)
				}
				if zipWriter != nil {
					_, err := zipWriter.Create(relPath + "/")
					return err
				}
				return nil
			}
			return addSingleFile(filePath, relPath, zipWriter, tarWriter)
		})
	}

	// For a single file, use the base name as the archive path
	return addSingleFile(realPath, baseName, zipWriter, tarWriter)
}

func addSingleFile(realPath, archivePath string, zipWriter *zip.Writer, tarWriter *tar.Writer) error {
	file, err := os.Open(realPath)
	if err != nil {
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}

	if tarWriter != nil {
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(archivePath)
		if err = tarWriter.WriteHeader(header); err != nil {
			return err
		}
		_, err = io.Copy(tarWriter, file)
		return err
	}

	if zipWriter != nil {
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = archivePath
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, file)
		return err
	}

	return nil
}

func rawFilesHandler(w http.ResponseWriter, r *http.Request, d *requestContext, fileList []string) (int, error) {
	splitFile := strings.Split(fileList[0], "::")
	if len(splitFile) != 2 {
		return http.StatusBadRequest, fmt.Errorf("invalid file in files request: %v", fileList[0])
	}

	firstFileSource := splitFile[0]
	firstFilePath := splitFile[1]
	fileName := filepath.Base(firstFilePath)
	var err error
	userscope := "/"
	// userscope declared above
	if d.user.Username != "publicUser" {
		var realSource string
		var scopePath string
		var err error
		scopePath, realSource, err = ResolveScopePath(d.user, firstFileSource, firstFilePath)
		if err != nil {
			logger.Errorf("ResolveScopePath failed for source %s: %v", firstFileSource, err)
			return http.StatusForbidden, err
		}
		// Use Real Source Name
		firstFileSource = realSource
		// Use resolved absolute path
		firstFilePath = scopePath
	}

	logger.Debugf("RawFilesHandler: Original Source: %s, UserScope: %s", firstFileSource, userscope)

	idx := indexing.GetIndex(firstFileSource)
	if idx == nil {
		logger.Errorf("RawFilesHandler: Source index not found for %s", firstFileSource)
		return http.StatusInternalServerError, fmt.Errorf("source %s is not available", firstFileSource)
	}
	realPath, isDir, err := idx.GetRealPath(userscope, firstFilePath)
	if err != nil {
		logger.Errorf("RawFilesHandler: GetRealPath failed for scope %s, path %s: %v", userscope, firstFilePath, err)
		return http.StatusInternalServerError, err
	}
	logger.Debugf("RawFilesHandler: Resolved RealPath: %s", realPath)

	// PERFORMANCE/BUGFIX: Only compute archive size if we are actually building an archive (multiple files or a directory).
	// This avoids 500 errors in computeArchiveSize when requesting single sidecar files (like faces.json)
	// that are not indexed in the metadata store.
	estimatedSize := int64(0)
	if len(fileList) > 1 || isDir {
		var err2 error
		estimatedSize, err2 = computeArchiveSize(fileList, d)
		if err2 != nil {
			return http.StatusInternalServerError, err2
		}
	}

	// ** Single file download with Content-Length **
	if len(fileList) == 1 && !isDir {
		fd, err2 := os.Open(realPath)
		if err2 != nil {
			return http.StatusInternalServerError, err2
		}
		defer fd.Close()

		// Get file size
		fileInfo, err2 := fd.Stat()
		if err2 != nil {
			return http.StatusInternalServerError, err2
		}

		// Set headers
		setContentDisposition(w, r, fileName)
		w.Header().Set("Cache-Control", "private")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
		sizeInMB := estimatedSize / 1024 / 1024
		// if larger than 500 MB, log it
		if sizeInMB > 500 {
			logger.Debugf("User %v is downloading large (%d MB) file: %v", d.user.Username, sizeInMB, fileName)
		}
		// serve content allows for range requests.
		// video scrubbing, etc.
		http.ServeContent(w, r, fileName, fileInfo.ModTime(), fd)
		return 200, nil
	}

	if config.Server.MaxArchiveSizeGB > 0 {
		maxSize := config.Server.MaxArchiveSizeGB * 1024 * 1024 * 1024
		if estimatedSize > maxSize {
			return http.StatusRequestEntityTooLarge, fmt.Errorf("pre-archive combined size of files exceeds maximum limit of %d GB", config.Server.MaxArchiveSizeGB)
		}
	}
	// ** Archive (ZIP/TAR.GZ) handling **
	algo := r.URL.Query().Get("algo")
	var extension string
	switch algo {
	case "zip", "true", "":
		extension = ".zip"
	case "tar.gz":
		extension = ".tar.gz"
	default:
		return http.StatusInternalServerError, errors.New("format not implemented")
	}

	baseDirName := filepath.Base(filepath.Dir(firstFilePath))
	if baseDirName == "" || baseDirName == "/" {
		baseDirName = "download"
	}
	if len(fileList) == 1 && isDir {
		baseDirName = filepath.Base(realPath)
	}
	fileName = url.PathEscape(baseDirName + extension)

	archiveData := filepath.Join(config.Server.CacheDir, utils.InsecureRandomIdentifier(10))
	if extension == ".zip" {
		archiveData = archiveData + ".zip"
		err = createZip(d, archiveData, fileList...)
	} else {
		archiveData = archiveData + ".tar.gz"
		err = createTarGz(d, archiveData, fileList...)
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	// stream archive to response
	fd, err := os.Open(archiveData)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	defer fd.Close()

	// Get file size
	fileInfo, err := fd.Stat()
	if err != nil {
		os.Remove(archiveData) // Remove the file if stat fails
		return http.StatusInternalServerError, err
	}

	sizeInMB := fileInfo.Size() / 1024 / 1024
	if sizeInMB > 500 {
		logger.Debugf("User %v is downloading large (%d MB) file: %v", d.user.Username, sizeInMB, fileName)
	}

	// Set headers AFTER computing actual archive size
	w.Header().Set("Content-Disposition", "attachment; filename*=utf-8''"+fileName)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))
	w.Header().Set("Content-Type", "application/octet-stream")

	// Stream the file
	_, err = io.Copy(w, fd)
	os.Remove(archiveData) // Remove the file after streaming
	if err != nil {
		logger.Errorf("Failed to copy archive data to response: %v", err)
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

func computeArchiveSize(fileList []string, d *requestContext) (int64, error) {
	var estimatedSize int64
	for _, fname := range fileList {
		splitFile := strings.Split(fname, "::")
		if len(splitFile) != 2 {
			return http.StatusBadRequest, fmt.Errorf("invalid file in files request: %v", fileList[0])
		}
		source := splitFile[0]
		path := splitFile[1]
		var err error
		userScope := "/"
		if d.user.Username != "publicUser" {
			var realSource string
			var scopePath string
			scopePath, realSource, err = ResolveScopePath(d.user, source, path)
			if d.share == nil && err != nil {
				return 0, fmt.Errorf("source %s is not available for user %s", source, d.user.Username)
			}
			source = realSource
			// Use resolved path
			path = scopePath
		} else {
			// For public user, ensure path doesn't have double slash issues if strictly following previous logic
			path = utils.JoinPathAsUnix(userScope, path)
		}

		idx := indexing.GetIndex(source)
		if idx == nil {
			return 0, fmt.Errorf("source %s is not available", source)
		}
		// Path is now absolute scope path
		realPath, isDir, err := idx.GetRealPath("/", path)
		if err != nil {
			return 0, err
		}

		// Use logical path for metadata lookup to avoid casing issues with MakeIndexPath(realPath)
		indexPath := path
		info, ok := idx.GetReducedMetadata(indexPath, isDir)
		if !ok {
			// Check if we need to try with MakeIndexPath as fallback?
			// No, logical path should be robust if metadata exists.
			// But let's log specifically if this fails.
			logger.Errorf("computeArchiveSize: Metadata missing for path: %s, UserScope: %s, IndexPath: %s", path, userScope, indexPath)
			// Try MakeIndexPath as fallback just in case Symlinks usage makes logical path invalid
			indexPath = idx.MakeIndexPath(realPath)
			info, ok = idx.GetReducedMetadata(indexPath, isDir)
			if !ok {
				// Don't error out hard, just skip or assume 0 for unindexed but real files
				// This prevents 500 crashes on non-indexed sidecars.
				continue
			}
		}
		logger.Debugf("computeArchiveSize: Found metadata for %s. Size: %d", indexPath, info.Size)
		estimatedSize += info.Size
	}
	return estimatedSize, nil
}

func createZip(d *requestContext, tmpDirPath string, filenames ...string) error {
	file, err := os.Create(tmpDirPath)
	if err != nil {
		return err
	}
	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for _, fname := range filenames {
		err := addFile(fname, d, nil, zipWriter, false)
		if err != nil {
			logger.Errorf("Failed to add %s to ZIP: %v", fname, err)
			return err
		}
	}

	return nil
}

func createTarGz(d *requestContext, tmpDirPath string, filenames ...string) error {
	file, err := os.Create(tmpDirPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzWriter := gzip.NewWriter(file)
	defer gzWriter.Close()
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	for _, fname := range filenames {
		err := addFile(fname, d, tarWriter, nil, false)
		if err != nil {
			logger.Errorf("Failed to add %s to TAR.GZ: %v", fname, err)
			return err
		}
	}

	return nil
}
