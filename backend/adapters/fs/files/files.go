package files

import (
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
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/fileutils"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/errors"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/utils"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
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
	Name           string      `xml:"Name"`
	Type           string      `xml:"Type"`
	NameAssignType string      `xml:"NameAssignType"`
	DLYArea        area        `xml:"DLYArea"`  // ACDSee uses DLYArea
	ALGArea        area        `xml:"ALGArea"`  // Standard ALGArea
}



type area struct {
	H float64 `xml:"h"`
	W float64 `xml:"w"`
	X float64 `xml:"x"`
	Y float64 `xml:"y"`
}

var OnlyOfficeCache = cache.NewCache(48 * time.Hour)

// GetMetadata fetches EXIF, IPTC, and XMP metadata for a given file path using exiftool.
func GetMetadata(filePath string) (map[string]interface{}, error) {
	// Step 1: Get general metadata as JSON
	cmd := exec.Command("exiftool", "-j", "-EXIF:All", "-IPTC:All", "-s", "-G", filePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Error executing exiftool for %s: %v\nOutput: %s", filePath, err, string(output))
		return nil, fmt.Errorf("could not get general metadata: %w", err)
	}

	var metadataArray []map[string]interface{}
	if err := json.Unmarshal(output, &metadataArray); err != nil {
		logger.Errorf("Error unmarshaling exiftool JSON output for %s: %v", filePath, err)
		return nil, fmt.Errorf("could not parse JSON metadata output: %w", err)
	}

	if len(metadataArray) == 0 {
		return nil, fmt.Errorf("no metadata found for file")
	}

	fullMetadata := metadataArray[0]
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

	// Step 2: Get specific structured XMP metadata (e.g., face regions) as XML
	cmdXMP := exec.Command("exiftool", "-b", "-XMP", filePath)
	xmpOutput, err := cmdXMP.CombinedOutput()
	if err != nil {
		logger.Errorf("Error reading raw XMP for %s: %v\nOutput: %s", filePath, err, string(xmpOutput))
		// We don't return an error here, as we still have the general metadata.
		// We just log it and move on.
	} else {
		var xmpDataParsed xmp
		if err := xml.Unmarshal(xmpOutput, &xmpDataParsed); err != nil {
			logger.Errorf("Error parsing XMP XML for %s: %v", filePath, err)
		} else {
			// Process and normalize the region data
			processedRegions := make([]map[string]interface{}, 0)
			
			// Check if we have any regions to process
			if xmpDataParsed.RDF.Regions.Items != nil {
				for _, region := range xmpDataParsed.RDF.Regions.Items {
					processedRegion := map[string]interface{}{
						"Name":           region.Name,
						"Type":           region.Type,
						"NameAssignType": region.NameAssignType, // This can be empty
					}

					// Check for coordinates in either DLYArea (ACDSee) or ALGArea (standard)
					var areaData area
					hasValidArea := false
					
					// Prefer DLYArea (ACDSee format) - check if coordinates are valid
					if region.DLYArea.X > 0 && region.DLYArea.Y > 0 && 
					   region.DLYArea.W > 0 && region.DLYArea.H > 0 {
						areaData = region.DLYArea
						hasValidArea = true
						logger.Debugf("Using DLYArea coordinates for %s: X=%f, Y=%f, W=%f, H=%f", 
							region.Name, areaData.X, areaData.Y, areaData.W, areaData.H)
					} else if region.ALGArea.X > 0 && region.ALGArea.Y > 0 && 
					          region.ALGArea.W > 0 && region.ALGArea.H > 0 {
						// Fall back to ALGArea (standard format)
						areaData = region.ALGArea
						hasValidArea = true
						logger.Debugf("Using ALGArea coordinates for %s: X=%f, Y=%f, W=%f, H=%f", 
							region.Name, areaData.X, areaData.Y, areaData.W, areaData.H)
					}

					if hasValidArea {
						processedRegion["ALGArea"] = map[string]float64{
							"X": areaData.X,
							"Y": areaData.Y,
							"W": areaData.W,
							"H": areaData.H,
						}
						processedRegions = append(processedRegions, processedRegion)
					} else {
						logger.Debugf("Skipping region with invalid coordinates in %s: Name=%s", filePath, region.Name)
					}
				}
			}

			// Add the structured data to the xmpData map
			if len(processedRegions) > 0 {
				xmpData["Regions"] = processedRegions
				logger.Debugf("Found %d valid regions in XMP for %s", len(processedRegions), filePath)
			} else {
				logger.Debugf("No valid regions found in XMP for %s", filePath)
			}
		}
	}

	// Step 3: Combine and return
	return map[string]interface{}{
		"exif": exifData,
		"iptc": iptcData,
		"xmp":  xmpData,
	}, nil
}

func FileInfoFaster(opts iteminfo.FileOptions) (iteminfo.ExtendedFileInfo, error) {
	response := iteminfo.ExtendedFileInfo{}
	if opts.Source == "" {
		opts.Source = settings.Config.Server.DefaultSource.Name
	}
	index := indexing.GetIndex(opts.Source)
	if index == nil {
		return response, fmt.Errorf("could not get index: %v ", opts.Source)
	}
	realPath, isDir, err := index.GetRealPath(opts.Path)
	if err != nil {
		return response, err
	}
	opts.IsDir = isDir
	// TODO: whats the best way to save trips to disk here?
	// disabled using cache because its not clear if this is helping or hurting
	// check if the file exists in the index
	//info, exists := index.GetReducedMetadata(opts.Path, opts.IsDir)
	//if exists {
	//	err := RefreshFileInfo(opts)
	//	if err != nil {
	//		return info, err
	//	}
	//	if opts.Content {
	//		content := ""
	//		content, err = getContent(opts.Path)
	//		if err != nil {
	//			return info, err
	//		}
	//		info.Content = content
	//	}
	//	return info, nil
	//}
	err = index.RefreshFileInfo(opts)
	if err != nil {
		return response, err
	}
	info, exists := index.GetReducedMetadata(opts.Path, opts.IsDir)
	if !exists {
		return response, fmt.Errorf("could not get metadata for path: %v", opts.Path)
	}
	if opts.Content && strings.HasPrefix(info.Type, "text") {
		if info.Size < 20*1024*1024 { // 20 megabytes in bytes
			content, err := getContent(realPath)
			if err != nil {
				logger.Debugf("could not get content for file: "+info.Path, info.Name, err)
				return response, err
			}
			response.Content = content
		} else {
			logger.Debug("skipping large text file contents (20MB limit): "+info.Path, info.Name)
		}
	}
	response.FileInfo = *info
	response.RealPath = realPath
	response.Source = opts.Source
	if settings.Config.Integrations.OnlyOffice.Secret != "" && info.Type != "directory" && iteminfo.IsOnlyOffice(info.Name) {
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

// Checksum checksums a given File for a given User, using a specific
// algorithm. The checksums data is saved on File object.
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
	err := os.RemoveAll(absPath)
	if err != nil {
		return err
	}
	index := indexing.GetIndex(source)
	if index == nil {
		return fmt.Errorf("could not get index: %v ", source)
	}
	refreshConfig := iteminfo.FileOptions{Path: index.MakeIndexPath(absDirPath), IsDir: true}
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return err
	}
	return nil
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
	// refresh info for source and dest
	err = idxSrc.RefreshFileInfo(iteminfo.FileOptions{
		Path:  refreshSourceDir,
		IsDir: true,
	})
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	if refreshSourceDir == refreshDestDir {
		return nil
	}
	refreshConfig := iteminfo.FileOptions{Path: refreshDestDir, IsDir: true}
	err = idxDst.RefreshFileInfo(refreshConfig)
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
	// refresh info for source and dest
	err = index.RefreshFileInfo(refreshConfig)
	if err != nil {
		return fmt.Errorf("could not refresh index for source: %v", err)
	}
	refreshConfig.Path = refreshDestDir
	err = index.RefreshFileInfo(refreshConfig)
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
	// Ensure the parent directories exist
	err := os.MkdirAll(realPath, 0775)
	if err != nil {
		return err
	}
	err = idx.RefreshFileInfo(opts)
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
	// Create the directory and all necessary parents
	err := os.MkdirAll(parentDir, 0775)
	if err != nil {
		return err
	}

	// Open the file for writing (create if it doesn't exist, truncate if it does)
	file, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0775)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy the contents from the reader to the file
	_, err = io.Copy(file, in)
	if err != nil {
		return err
	}
	opts.Path = idx.MakeIndexPath(parentDir)
	opts.IsDir = true
	return idx.RefreshFileInfo(opts)
}

// getContent reads and returns the file content if it's UTF-8 readable.
func getContent(realPath string) (string, error) {
	// Read the entire file in one go. This is more efficient.
	content, err := os.ReadFile(realPath)
	if err != nil {
		return "", err
	}

	// Check if the file content is valid UTF-8.
	// This allows for all characters, including ASCII and emojis.
	if !utf8.Valid(content) {
		// File is not valid text (e.g., a binary image), so we reject it.
		return "", nil
	}

	// Handle the special case for an empty file, as in your original code.
	if len(content) == 0 {
		return "empty-file-x6OlSil", nil
	}

	// The file is valid, so return its string content.
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