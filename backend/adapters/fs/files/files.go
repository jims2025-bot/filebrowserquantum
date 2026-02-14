package files

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/fileutils"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/errors"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/utils"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
)

// Structs for parsing the relevant parts of XMP
type xmp struct {
	XMLName xml.Name `xml:"xmpmeta"`
	RDF     rdf      `xml:"RDF>Description"`
}

type rdf struct {
	Regions regions `xml:"Regions>RegionList>Bag"`
}

type regions struct {
	Items []rdfLi `xml:"li"`
}

type rdfLi struct {
	// Direct fields (legacy/simple format)
	Name           string `xml:"Name"`
	Type           string `xml:"Type"`
	NameAssignType string `xml:"NameAssignType"`
	DLYArea        area   `xml:"DLYArea"` // ACDSee uses DLYArea
	ALGArea        area   `xml:"ALGArea"` // Standard ALGArea

	// Nested Description (ACDSee standard format)
	Description *rdfLiDescription `xml:"Description"`
}

type rdfLiDescription struct {
	Name           string `xml:"Name,attr"`
	Type           string `xml:"Type,attr"`
	NameAssignType string `xml:"NameAssignType,attr"`
	DLYArea        area   `xml:"DLYArea"`
	ALGArea        area   `xml:"ALGArea"`
}

type area struct {
	// Element support
	H float64 `xml:"h"`
	W float64 `xml:"w"`
	X float64 `xml:"x"`
	Y float64 `xml:"y"`

	// Attribute support (generic and namespaced)
	HAttr float64 `xml:"h,attr"`
	WAttr float64 `xml:"w,attr"`
	XAttr float64 `xml:"x,attr"`
	YAttr float64 `xml:"y,attr"`
}

// Helper to get values regardless of format
func (a area) GetH() float64 {
	if a.H != 0 {
		return a.H
	}
	return a.HAttr
}
func (a area) GetW() float64 {
	if a.W != 0 {
		return a.W
	}
	return a.WAttr
}
func (a area) GetX() float64 {
	if a.X != 0 {
		return a.X
	}
	return a.XAttr
}
func (a area) GetY() float64 {
	if a.Y != 0 {
		return a.Y
	}
	return a.YAttr
}

var OnlyOfficeCache = cache.NewCache(48 * time.Hour)

const maxCachedFolders = 5

var (
	// Folder Metadata Cache
	folderMetadataCache = make(map[string]map[string]map[string]interface{})
	folderCacheOrder    []string
	cacheMu             sync.Mutex

	// Track active folder pre-fetches
	activePrefetches sync.Map // [folderPath]bool
)

func updateCache(folderPath string, data map[string]map[string]interface{}) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	// Add to cache or update
	folderMetadataCache[folderPath] = data

	// Update order for LRU
	for i, p := range folderCacheOrder {
		if p == folderPath {
			folderCacheOrder = append(folderCacheOrder[:i], folderCacheOrder[i+1:]...)
			break
		}
	}
	folderCacheOrder = append([]string{folderPath}, folderCacheOrder...)

	// Evict if over limit
	if len(folderCacheOrder) > maxCachedFolders {
		oldest := folderCacheOrder[len(folderCacheOrder)-1]
		delete(folderMetadataCache, oldest)
		folderCacheOrder = folderCacheOrder[:len(folderCacheOrder)-1]
	}
}

func getFromCache(folderPath, fileName string) (map[string]interface{}, bool) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if folder, ok := folderMetadataCache[folderPath]; ok {
		if meta, ok := folder[fileName]; ok {
			return meta, true
		}
	}
	return nil, false
}

func prefetchFolder(folderPath string) {
	// Don't start if already pre-fetching this folder
	if _, loaded := activePrefetches.LoadOrStore(folderPath, true); loaded {
		return
	}

	go func() {
		defer activePrefetches.Delete(folderPath)

		b, err := GetExifToolBridgeBackground()
		if err != nil {
			return
		}

		// Fetch all metadata for all images in the folder
		// We use the folder path directly with exiftool to get all files
		args := []string{"-m", "-j", "-G", "-EXIF:All", "-IPTC:All", "-XMP:All", folderPath}
		output, err := b.Execute(args)
		if err != nil {
			logger.Errorf("Background pre-fetch failed for %s: %v", folderPath, err)
			return
		}

		var metadataArray []map[string]interface{}
		if err := json.Unmarshal(output, &metadataArray); err != nil {
			logger.Errorf("Failed to parse pre-fetch JSON for %s: %v", folderPath, err)
			return
		}

		folderData := make(map[string]map[string]interface{})
		for _, m := range metadataArray {
			if sourceFile, ok := m["SourceFile"].(string); ok {
				fname := filepath.Base(sourceFile)
				folderData[fname] = m
			}
		}

		updateCache(folderPath, folderData)
		logger.Infof("Successfully pre-fetched %d files for folder: %s", len(folderData), folderPath)
	}()
}

// GetMetadata fetches EXIF, IPTC, and XMP metadata for a given file path.
// It uses a persistent ExifTool bridge and folder-level caching for speed.
func GetMetadata(filePath string) (map[string]interface{}, error) {
	folderPath := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	// 1. Check Cache
	if meta, ok := getFromCache(folderPath, fileName); ok {
		return formatMetadata(meta), nil
	}

	// 2. Trigger asynchronous pre-fetch for the whole folder
	prefetchFolder(folderPath)

	// 3. Fetch just this file immediately using the bridge
	b, err := GetExifToolBridge()
	if err != nil {
		return nil, err
	}

	args := []string{"-m", "-j", "-G", "-EXIF:All", "-IPTC:All", "-XMP:All", filePath}
	output, err := b.Execute(args)
	if err != nil {
		return nil, fmt.Errorf("exiftool extraction failed: %w", err)
	}

	var metadataArray []map[string]interface{}
	if err := json.Unmarshal(output, &metadataArray); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	if len(metadataArray) == 0 {
		return nil, fmt.Errorf("no metadata found for file")
	}

	formatted := formatMetadata(metadataArray[0])

	// Add IsPreFetching flag
	_, isPreFetching := activePrefetches.Load(folderPath)
	formatted["isPreFetching"] = isPreFetching

	return formatted, nil
}

func formatMetadata(fullMetadata map[string]interface{}) map[string]interface{} {
	exifData := make(map[string]interface{})
	iptcData := make(map[string]interface{})
	xmpData := make(map[string]interface{})

	for key, value := range fullMetadata {
		parts := strings.SplitN(key, ":", 2)
		if len(parts) != 2 {
			continue
		}
		group := parts[0]
		tag := parts[1]

		switch group {
		case "EXIF":
			exifData[tag] = value
		case "IPTC":
			iptcData[tag] = value
		case "XMP":
			xmpData[tag] = value
		}
	}

	// Process face regions if present in XMP
	if _, ok := fullMetadata["XMP:XMP"]; ok {
		// If we already have the raw XMP string from the bridge, we can parse regions
		// Note: The bridge command might not return the raw XML unless -b -XMP is used.
		// For now, let's see if we can extract regions from the JSON structure if exiftool parses them.
	}

	return map[string]interface{}{
		"exif": exifData,
		"iptc": iptcData,
		"xmp":  xmpData,
	}
}

// WriteXMPInstructions writes the given instructions string into both
// the XMP Photoshop:Instructions field and the IPTC SpecialInstructions field
// of the specified file using exiftool.
func WriteXMPInstructions(filePath, instructions string) error {
	log.Printf("WriteXMPInstructions called on %s with %s", filePath, instructions)

	// Invalidate cache for the folder
	cacheMu.Lock()
	delete(folderMetadataCache, filepath.Dir(filePath))
	cacheMu.Unlock()

	// User Request: Use specific command to fix IPTCDigest integrity issues
	// exiftool -m -overwrite_original \
	//   -IPTC:SpecialInstructions="YOUR TEXT" \
	//   -XMP-photoshop:Instructions<IPTC:SpecialInstructions \
	//   -IPTCDigest=new \
	//   "PATH/TO/FILE.jpg"

	cmd := exec.Command("exiftool",
		"-m",
		"-overwrite_original",
		fmt.Sprintf("-IPTC:SpecialInstructions=%s", instructions),
		"-XMP-photoshop:Instructions<IPTC:SpecialInstructions", // Copy value from IPTC to XMP
		"-IPTCDigest=new", // Regenerate digest to guarantee integrity
		filePath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Error writing XMP/IPTC Instructions to %s: %v\nOutput: %s", filePath, err, string(output))
		// Include the output in the returned error so the frontend can display it
		return fmt.Errorf("could not write XMP/IPTC Instructions: %s (err: %w)", string(output), err)
	}

	log.Printf("Exiftool output: %s", string(output))
	log.Printf("Successfully wrote XMP/IPTC Instructions to %s", filePath)
	return nil
}

// GetXMPInstructions retrieves the XMP Photoshop:Instructions field from the file
func GetXMPInstructions(filePath string) (string, error) {
	cmd := exec.Command("exiftool", "-s", "-s", "-s", "-XMP-photoshop:Instructions", filePath)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		logger.Errorf("Error reading XMP Instructions from %s: %v", filePath, err)
		return "", fmt.Errorf("could not read XMP Instructions: %w", err)
	}

	instructions := out.String()
	if instructions == "" {
		return "", nil // no field present
	}

	return instructions, nil
}

func FileInfoFaster(opts iteminfo.FileOptions) (iteminfo.ExtendedFileInfo, error) {
	response := iteminfo.ExtendedFileInfo{}
	if opts.Source == "ALL" {
		return response, fmt.Errorf("file listing is not supported for pseudo-source 'ALL'")
	}
	index := indexing.GetIndex(opts.Source)
	if index == nil {
		return response, fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, isDir, err := index.GetRealPath(opts.Path)
	if err != nil {
		return response, err
	}
	// Re-verify isDir to avoid stale cache issues from GetRealPath
	stat, statErr := os.Stat(realPath)
	if statErr == nil {
		isDir = stat.IsDir()
	}
	opts.IsDir = isDir

	// Canonicalize the path using the resolved real path to ensure consistency with index keys
	// This fixes issues where opts.Path has improper casing or duplication (e.g. /PHOTOS/PHOTOS/...)
	opts.Path = index.MakeIndexPath(realPath)

	// Capture filename if it's a file, because RefreshFileInfo resolves to the directory
	var originalBase string
	if !opts.IsDir {
		originalBase = filepath.Base(realPath)
	}

	correctedPath, err := index.RefreshFileInfo(opts)
	if err != nil {
		return response, err
	}

	// If it was a file, we need to append the filename back to the corrected directory path
	if !opts.IsDir {
		if strings.HasSuffix(correctedPath, "/") {
			correctedPath += originalBase
		} else {
			correctedPath += "/" + originalBase
		}
	}
	opts.Path = correctedPath

	// Re-get real path in case deduplication changed it
	realPath, isDir, err = index.GetRealPath(opts.Path)
	if err != nil {
		// Log but continue if possible, or return error?
		// If we can't get real path of the corrected path, something is weird.
		return response, err
	}
	opts.IsDir = isDir // Update opts.IsDir with the correct reality

	// info path lookup uses the canonical path now
	info, exists := index.GetReducedMetadata(opts.Path, opts.IsDir)
	if !exists {
		return response, errors.ErrNotExist
	}
	isText := strings.HasPrefix(info.Type, "text")
	isJSON := info.Type == "application/json" || info.Type == "application/geo+json"
	if opts.Content && (isText || isJSON) {
		if info.Size < 20*1024*1024 {
			content, err := getContent(realPath)
			if err != nil {
				logger.Debugf("could not get content for file: "+info.Path, info.Name, err)
				return response, err
			}
			response.Content = content
		}
	}
	response.FileInfo = *info
	response.RealPath = realPath
	response.Source = opts.Source
	if settings.Config.Integrations.OnlyOffice.Secret != "" &&
		info.Type != "directory" && iteminfo.IsOnlyOffice(info.Name) {
		response.OnlyOfficeId = generateOfficeId(realPath)
	}
	if strings.HasPrefix(info.Type, "video") {
		parentInfo, exists := index.GetReducedMetadata(filepath.Dir(info.Path), true)
		if exists {
			response.DetectSubtitles(parentInfo)
		}
	}
	return response, nil
}

func generateOfficeId(realPath string) string {
	key, ok := OnlyOfficeCache.Get(realPath).(string)
	if !ok {
		timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)
		documentKey := utils.HashSHA256(realPath + timestamp)
		OnlyOfficeCache.Set(realPath, documentKey)
		return documentKey
	}
	return key
}

func GetChecksum(fullPath, algo string) (map[string]string, error) {
	subs := map[string]string{}
	reader, err := os.Open(fullPath)
	if err != nil {
		return subs, err
	}
	defer reader.Close()

	hashFuncs := map[string]hash.Hash{
		"md5":    md5.New(),
		"sha1":   sha1.New(),
		"sha256": sha256.New(),
		"sha512": sha512.New(),
	}
	h, ok := hashFuncs[algo]
	if !ok {
		return subs, errors.ErrInvalidOption
	}
	_, err = io.Copy(h, reader)
	if err != nil {
		return subs, err
	}
	subs[algo] = hex.EncodeToString(h.Sum(nil))
	return subs, nil
}

func DeleteFiles(source, absPath string, absDirPath string) error {
	// SECURITY: File deletion disabled by administrator
	return fmt.Errorf("file deletion is disabled by administrator")
}

func MoveResource(sourceIndex, destIndex, realsrc, realdst string) error {
	err := fileutils.MoveFile(realsrc, realdst)
	if err != nil {
		return err
	}
	idxSrc := indexing.GetIndex(sourceIndex)
	if idxSrc == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	idxDst := indexing.GetIndex(destIndex)
	if idxDst == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshSourceDir := idxSrc.MakeIndexPath(filepath.Dir(realsrc))
	refreshDestDir := idxDst.MakeIndexPath(filepath.Dir(realdst))
	_, err = idxSrc.RefreshFileInfo(iteminfo.FileOptions{Path: refreshSourceDir, IsDir: true})
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	if refreshSourceDir == refreshDestDir {
		return nil
	}
	refreshConfig := iteminfo.FileOptions{Path: refreshDestDir, IsDir: true}
	_, err = idxDst.RefreshFileInfo(refreshConfig)
	if err != nil {
		return fmt.Errorf("could not refresh index for dest: %v", err)
	}
	return nil
}

func CopyResource(sourceIndex, destIndex, realsrc, realdst string) error {
	err := fileutils.CopyFile(realsrc, realdst)
	if err != nil {
		return err
	}
	idxSrc := indexing.GetIndex(sourceIndex)
	if idxSrc == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	idxDst := indexing.GetIndex(destIndex)
	if idxDst == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshSourceDir := idxSrc.MakeIndexPath(filepath.Dir(realsrc))
	refreshDestDir := idxDst.MakeIndexPath(filepath.Dir(realdst))
	index := indexing.GetIndex(sourceIndex)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", sourceIndex)
	}
	refreshConfig := iteminfo.FileOptions{Path: refreshSourceDir, IsDir: true}
	_, err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	refreshConfig.Path = refreshDestDir
	_, err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func WriteDirectory(opts iteminfo.FileOptions) error {
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, _, _ := idx.GetRealPath(opts.Path)
	err := os.MkdirAll(realPath, 0775)
	if err != nil {
		return err
	}
	_, err = idx.RefreshFileInfo(opts)
	if err != nil {
		return errors.ErrEmptyKey
	}
	return nil
}

func WriteFile(opts iteminfo.FileOptions, in io.Reader) error {
	idx := indexing.GetIndex(opts.Source)
	if idx == nil {
		return fmt.Errorf("could not get index: %v ", opts.Source)
	}
	dst, _, _ := idx.GetRealPath(opts.Path)
	parentDir := filepath.Dir(dst)
	err := os.MkdirAll(parentDir, 0775)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(file, in)
	if err != nil {
		return err
	}
	opts.Path = idx.MakeIndexPath(parentDir)
	opts.IsDir = true
	_, err = idx.RefreshFileInfo(opts)
	return err
}

func getContent(realPath string) (string, error) {
	content, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(content) {
		return "", nil
	}
	if len(content) == 0 {
		return "empty-file-x6OlSil", nil
	}
	return string(content), nil
}

func IsNamedPipe(mode os.FileMode) bool {
	return mode&os.ModeNamedPipe != 0
}

func IsSymlink(mode os.FileMode) bool {
	return mode&os.ModeSymlink != 0
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// FixThumbnailResolution recursively finds and updates thumbnail resolution in images
// using exiftool.
func FixThumbnailResolution(path string) error {
	logger.Infof("Starting recursive thumbnail fix for directory: %s", path)

	// User Request Command:
	// exiftool -m -overwrite_original -tagsfromfile @ ^
	//  "-IFD0:XResolution>IFD1:XResolution" ^
	//  "-IFD0:YResolution>IFD1:YResolution" ^
	//  "-IFD0:ResolutionUnit>IFD1:ResolutionUnit" ^
	//  -IPTCDigest=new ^
	//  -r "E:\PERSONAL\PHOTOS\PHOTOCOLLECTIONS\SHANTONGA-COLLECTION\2015\05-17-2015-ENGLAND"

	cmd := exec.Command("exiftool",
		"-m",
		"-overwrite_original",
		"-tagsfromfile", "@",
		"-IFD0:XResolution>IFD1:XResolution",
		"-IFD0:YResolution>IFD1:YResolution",
		"-IFD0:ResolutionUnit>IFD1:ResolutionUnit",
		"-IPTCDigest=new",
		"-r",
		path,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Exiftool failed to fix thumbnails: %v\nOutput: %s", err, string(output))
		return fmt.Errorf("exiftool failed: %s: %w", string(output), err)
	}

	logger.Infof("Successfully finished recursive thumbnail fix for: %s", path)
	logger.Debugf("Exiftool output: %s", string(output))
	return nil
}
