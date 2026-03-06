package facerec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/people"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
)

type ScanMonitor struct {
	mu          sync.Mutex
	activeScans map[string]bool
}

var monitor = &ScanMonitor{
	activeScans: make(map[string]bool),
}

func GetScanStatus() map[string]interface{} {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()

	active := len(monitor.activeScans) > 0
	return map[string]interface{}{
		"isScanning":  active,
		"activeCount": len(monitor.activeScans),
	}
}

func addScan(diskPath string) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.activeScans[diskPath] = true
}

func removeScan(diskPath string) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	delete(monitor.activeScans, diskPath)
}

// FaceBox represents bounding box coordinates [y1, x2, y2, x1]
type FaceBox []int

// FaceArea represents the percentage-based XMP Region coordinates
type FaceArea struct {
	X float64 `json:"X"`
	Y float64 `json:"Y"`
	W float64 `json:"W"`
	H float64 `json:"H"`
}

// FaceEntry represents a single face prediction.
type FaceEntry struct {
	Box        FaceBox   `json:"box"`
	Name       string    `json:"name"`
	Confidence float64   `json:"confidence"`
	Source     string    `json:"source"`
	Embedding  []float64 `json:"embedding,omitempty"`
	Area       *FaceArea `json:"Area,omitempty"`
}

// FacesFile maps filenames to lists of FaceEntries
type FacesFile map[string][]FaceEntry

// pythonResponse is the expected shape from the FastAPI `analyze` endpoint
type pythonResponse struct {
	Faces []FaceEntry `json:"faces"`
}

func AnalyzeFile(serverAddress string, imagePath string) ([]FaceEntry, error) {
	// Prepare multipart form data
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}
	writer.Close()

	url := fmt.Sprintf("%s/analyze", serverAddress)
	req, err := http.NewRequest("POST", url, body)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("python ML server returned %d", resp.StatusCode)
	}

	var pyr pythonResponse
	if err := json.NewDecoder(resp.Body).Decode(&pyr); err != nil {
		return nil, err
	}

	// tag ML sources
	for i := range pyr.Faces {
		pyr.Faces[i].Source = "ml"
	}

	return pyr.Faces, nil
}

// LearnFace sends an image and a box to the Python server to get an embedding
func LearnFace(serverAddress string, imagePath string, box FaceBox) ([]float64, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(imagePath))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}

	boxBytes, _ := json.Marshal(box)
	writer.WriteField("box", string(boxBytes))
	writer.Close()

	url := fmt.Sprintf("%s/learn", serverAddress)
	req, err := http.NewRequest("POST", url, body)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("python ML server (learn) returned %d", resp.StatusCode)
	}

	var res struct {
		Embedding []float64 `json:"embedding"`
		Success   bool      `json:"success"`
		Error     string    `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}
	if !res.Success {
		return nil, fmt.Errorf(res.Error)
	}
	return res.Embedding, nil
}

func CosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func CalculateIOU(boxA, boxB FaceBox) float64 {
	if len(boxA) < 4 || len(boxB) < 4 {
		return 0
	}
	y1A, x2A, y2A, x1A := boxA[0], boxA[1], boxA[2], boxA[3]
	y1B, x2B, y2B, x1B := boxB[0], boxB[1], boxB[2], boxB[3]

	y1I := maxInt(y1A, y1B)
	x1I := maxInt(x1A, x1B)
	y2I := minInt(y2A, y2B)
	x2I := minInt(x2A, x2B)

	if y1I >= y2I || x1I >= x2I {
		return 0
	}

	intersectionArea := float64((y2I - y1I) * (x2I - x1I))
	areaA := float64((y2A - y1A) * (x2A - x1A))
	areaB := float64((y2B - y1B) * (x2B - x1B))

	return intersectionArea / (areaA + areaB - intersectionArea)
}

// ExtractACDSeeRegions interrogates ExifTool to see if ACDSee wrote 'mwg-rs' Regions directly into the JPG
func ExtractACDSeeRegions(imagePath string) ([]FaceEntry, error) {
	bridge, err := files.GetExifToolBridgeBackground()
	if err != nil {
		return nil, err
	}

	args := []string{"-m", "-j", "-struct", "-XMP:RegionInfo", "-XMP:RegionInfoACDSee", "-XMP:Regions", "-ImageWidth", "-ImageHeight", "-Orientation#", imagePath}
	output, err := bridge.Execute(args)
	if err != nil {
		return nil, err
	}

	var metaArray []map[string]interface{}
	if err := json.Unmarshal(output, &metaArray); err != nil || len(metaArray) == 0 {
		return nil, err
	}
	exifData := metaArray[0]

	var acdseeFaces []FaceEntry
	var regions []interface{}

	checkContainer := func(key string) {
		if container, ok := exifData[key].(map[string]interface{}); ok {
			if rl, ok := container["RegionList"].([]interface{}); ok {
				regions = append(regions, rl...)
			} else if rlMap, ok := container["RegionList"].(map[string]interface{}); ok {
				regions = append(regions, rlMap)
			}
		} else if containerArr, ok := exifData[key].([]interface{}); ok {
			regions = append(regions, containerArr...)
		}
	}

	checkContainer("RegionInfo")
	checkContainer("RegionInfoACDSee")
	checkContainer("Regions")

	imgW := 0.0
	imgH := 0.0
	if w, ok := exifData["ImageWidth"].(float64); ok {
		imgW = w
	} else if w, ok := exifData["ImageWidth"].(int); ok {
		imgW = float64(w)
	}
	if h, ok := exifData["ImageHeight"].(float64); ok {
		imgH = h
	} else if h, ok := exifData["ImageHeight"].(int); ok {
		imgH = float64(h)
	}

	// Override ExifTool dimensions with true raw decoded bounds (accounts for JPEG MCU padding)
	file, err := os.Open(imagePath)
	if err == nil {
		defer file.Close()
		config, _, err := image.DecodeConfig(file)
		if err == nil && config.Width > 0 && config.Height > 0 {
			imgW = float64(config.Width)
			imgH = float64(config.Height)
		}
	}

	orientation := 1
	if orient, ok := exifData["Orientation"].(float64); ok {
		orientation = int(orient)
	} else if orient, ok := exifData["Orientation"].(int); ok {
		orientation = orient
	}

	for _, regAny := range regions {
		reg, ok := regAny.(map[string]interface{})
		if !ok {
			continue
		}

		name, _ := reg["Name"].(string)
		if name == "" {
			continue
		}

		nameAssignType, _ := reg["NameAssignType"].(string)

		confidence := 0.85
		if strings.ToLower(nameAssignType) == "manual" {
			confidence = 1.0
		}

		entry := FaceEntry{
			Box:        []int{0, 0, 0, 0},
			Name:       name,
			Confidence: confidence,
			Source:     "acdsee",
		}

		// Parse Area to allow frontend to render it
		areaRaw := reg["Area"]
		if areaRaw == nil {
			areaRaw = reg["ALGArea"]
		}
		if areaRaw == nil {
			areaRaw = reg["DLYArea"]
		}

		if areaMap, ok := areaRaw.(map[string]interface{}); ok {
			var fa FaceArea
			parseCoord := func(val interface{}) float64 {
				if f, ok := val.(float64); ok {
					return f
				}
				if s, ok := val.(string); ok {
					if f, err := strconv.ParseFloat(s, 64); err == nil {
						return f
					}
				}
				return 0
			}

			fa.X = parseCoord(areaMap["X"])
			if fa.X == 0 {
				fa.X = parseCoord(areaMap["x"])
			}
			if fa.X == 0 {
				fa.X = parseCoord(areaMap["stArea:x"])
			}

			fa.Y = parseCoord(areaMap["Y"])
			if fa.Y == 0 {
				fa.Y = parseCoord(areaMap["y"])
			}
			if fa.Y == 0 {
				fa.Y = parseCoord(areaMap["stArea:y"])
			}

			fa.W = parseCoord(areaMap["W"])
			if fa.W == 0 {
				fa.W = parseCoord(areaMap["w"])
			}
			if fa.W == 0 {
				fa.W = parseCoord(areaMap["stArea:w"])
			}

			fa.H = parseCoord(areaMap["H"])
			if fa.H == 0 {
				fa.H = parseCoord(areaMap["h"])
			}
			if fa.H == 0 {
				fa.H = parseCoord(areaMap["stArea:h"])
			}

			if fa.X != 0 || fa.Y != 0 {
				entry.Area = &fa
				// Normalize box to pixels if we have dimensions
				if imgW > 0 && imgH > 0 {
					vx, vy, vw, vh := fa.X, fa.Y, fa.W, fa.H
					rx, ry, rw, rh := vx, vy, vw, vh

					// MWG standard says coordinates are relative to the 'viewed' image.
					// We must convert them to 'stored' (raw) pixel coordinates.
					// Note: 6 and 8 swap width/height.
					switch orientation {
					case 3: // 180
						rx = 1 - vx
						ry = 1 - vy
					case 6: // 90 CW (Stored as Landscape, viewed as Portrait)
						// Viewed (vx, vy) -> Raw (rx, ry)
						rx = vy
						ry = 1 - vx
						rw = vh
						rh = vw
					case 8: // 270 CW (Stored as Landscape, viewed as Portrait)
						rx = 1 - vy
						ry = vx
						rw = vh
						rh = vw
					case 2: // Flip H
						rx = 1 - vx
					case 4: // Flip V
						ry = 1 - vy
					case 5: // Transpose
						rx = vy
						ry = vx
						rw = vh
						rh = vw
					case 7: // Transverse
						rx = 1 - vy
						ry = 1 - vx
						rw = vh
						rh = vw
					case 1:
						fallthrough
					default:
						// Already in raw space
					}

					log.Printf("[FacialRec] Orientation %d: Viewed(%.3f,%.3f) -> Raw(%.3f,%.3f) for %s", orientation, vx, vy, rx, ry, name)

					x1 := (rx - rw/2) * imgW
					y1 := (ry - rh/2) * imgH
					x2 := (rx + rw/2) * imgW
					y2 := (ry + rh/2) * imgH
					entry.Box = []int{int(y1), int(x2), int(y2), int(x1)}
				}
			}
		}

		acdseeFaces = append(acdseeFaces, entry)
	}

	return acdseeFaces, nil
}

func ScanFolder(dirPath string, cfg settings.FacialRecognition, store *storage.Storage, force bool) error {
	dirPath = filepath.Clean(dirPath)
	addScan(dirPath)
	defer removeScan(dirPath)

	// Open or create `faces.json`
	facesFilePath := filepath.Join(dirPath, "faces.json")
	var facesData FacesFile

	jsonBytes, err := os.ReadFile(facesFilePath)
	if err == nil {
		json.Unmarshal(jsonBytes, &facesData)

		// Get folder dimensions once to validate boxes
		var firstW, firstH float64
		// Deduplicate and validate existing entries once at the start of the scan to clean up any legacy duplicates
		updatedJSON := false
		for fname, entries := range facesData {
			if len(entries) > 0 {
				fullPath := filepath.Join(dirPath, fname)
				orient, w, h := getImageExifInfo(fullPath)
				if w == 0 || h == 0 {
					w, h = firstW, firstH // Fallback to first seen if exif fails
				} else if firstW == 0 {
					firstW, firstH = w, h
				}

				validEntries := []FaceEntry{}
				for _, e := range entries {
					if isBoxValid(e.Box, orient, int(w), int(h)) {
						validEntries = append(validEntries, e)
					} else {
						updatedJSON = true
						log.Printf("[FacialRec] Purging invalid box for %s in %s: %v (Orient: %d, Image: %dx%d)", e.Name, fname, e.Box, orient, int(w), int(h))
					}
				}

				deduped := deduplicateFaces(validEntries)
				if len(deduped) < len(entries) {
					facesData[fname] = deduped
					updatedJSON = true
					log.Printf("[FacialRec] Cleaned up %d duplicate(s) for %s in %s", len(entries)-len(deduped), fname, facesFilePath)
				} else if len(validEntries) < len(entries) {
					facesData[fname] = validEntries
				}
			}
		}
		if updatedJSON {
			b, _ := json.MarshalIndent(facesData, "", "  ")
			os.WriteFile(facesFilePath, b, 0666)
		}

		// Fast-sync existing faces to people.db to ensure all boxes are populated
		for filename, entries := range facesData {
			fullPath := filepath.Join(dirPath, filename)
			
			// Group entries by person to avoid index/delete tug-of-war
			personHasVerified := make(map[string]bool)
			for _, f := range entries {
				if f.Name != "" && f.Name != "Unknown" && (f.Source == "ml" || f.Confidence >= 0.85) {
					personHasVerified[f.Name] = true
				}
			}

			seenPeople := make(map[string]bool)
			for _, f := range entries {
				if f.Name != "" && f.Name != "Unknown" {
					if seenPeople[f.Name] {
						continue // Only index one box per person per image (schema limitation)
					}
					boxStr := ""
					if len(f.Box) == 4 {
						boxStr = fmt.Sprintf("%d,%d,%d,%d", f.Box[0], f.Box[1], f.Box[2], f.Box[3])
					}
					conf := f.Confidence
					if conf == 0 {
						conf = 1.0
					}

					// Only allow ML faces or verified ACDSee faces into the global search index
					if f.Source == "ml" || conf >= 0.85 {
						people.MapFaceToIndex(f.Name, fullPath, conf, boxStr)
						seenPeople[f.Name] = true
					} else if !personHasVerified[f.Name] {
						// Only remove if we DON'T have a verified entry for this person in this image
						people.RemoveFaceFromIndex(f.Name, fullPath, "")
					}
				}
			}
		}
	}
	if facesData == nil {
		facesData = make(FacesFile)
	}

	// Read directory
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	changed := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if processSingleFile(dirPath, entry.Name(), facesData, cfg, force) {
			changed = true
		}
	}

	if changed {
		b, err := json.MarshalIndent(facesData, "", "  ")
		if err == nil {
			os.WriteFile(facesFilePath, b, 0666)
		}
	}

	return nil
}

// ScanFile triggers scanning for a single file ONLY
func ScanFile(filePath string, cfg settings.FacialRecognition, store *storage.Storage, force bool) error {
	filePath = filepath.Clean(filePath)
	addScan(filePath)
	defer removeScan(filePath)
	dirPath := filepath.Dir(filePath)
	fileName := filepath.Base(filePath)

	facesFilePath := filepath.Join(dirPath, "faces.json")
	var facesData FacesFile
	jsonBytes, err := os.ReadFile(facesFilePath)
	if err == nil {
		json.Unmarshal(jsonBytes, &facesData)
	} else {
		log.Printf("[FacialRec] faces.json not found or error reading: %v", err)
	}
	if facesData == nil {
		facesData = make(FacesFile)
	}

	log.Printf("[FacialRec] Processing file: %s (force=%v)", fileName, force)
	if processSingleFile(dirPath, fileName, facesData, cfg, force) {
		log.Printf("[FacialRec] File processed, writing faces.json: %s", facesFilePath)
		b, err := json.MarshalIndent(facesData, "", "  ")
		if err == nil {
			return os.WriteFile(facesFilePath, b, 0666)
		}
	} else {
		log.Printf("[FacialRec] processSingleFile returned false for %s", fileName)
	}
	return nil
}

func getFolderLimits(dirPath string) (bool, []string) {
	p := filepath.Join(dirPath, "folderdetails.json")
	var fl struct {
		RestrictFaces bool     `json:"restrictFaces"`
		People        []string `json:"people"`
	}
	if b, err := os.ReadFile(p); err == nil {
		json.Unmarshal(b, &fl)
	}
	return fl.RestrictFaces, fl.People
}

// getImageExifInfo reads EXIF orientation and raw stored dimensions via ExifTool.
func getImageExifInfo(imagePath string) (orientation int, rawW, rawH float64) {
	orientation = 1
	bridge, err := files.GetExifToolBridgeBackground()
	if err != nil {
		return
	}
	args := []string{"-m", "-j", "-ImageWidth", "-ImageHeight", "-Orientation#", imagePath}
	output, err := bridge.Execute(args)
	if err != nil {
		return
	}
	var metaArray []map[string]interface{}
	if err := json.Unmarshal(output, &metaArray); err != nil || len(metaArray) == 0 {
		return
	}
	exifData := metaArray[0]

	if w, ok := exifData["ImageWidth"].(float64); ok {
		rawW = w
	}
	if rawW == 0 {
		if w, ok := exifData["ImageWidth"].(int); ok {
			rawW = float64(w)
		}
	}
	if h, ok := exifData["ImageHeight"].(float64); ok {
		rawH = h
	}
	if rawH == 0 {
		if h, ok := exifData["ImageHeight"].(int); ok {
			rawH = float64(h)
		}
	}
	if orient, ok := exifData["Orientation"].(float64); ok {
		orientation = int(orient)
	} else if orient, ok := exifData["Orientation"].(int); ok {
		orientation = orient
	}

	// Override ExifTool dimensions with true raw decoded bounds (accounts for JPEG MCU padding)
	file, err := os.Open(imagePath)
	if err == nil {
		defer file.Close()
		config, _, err := image.DecodeConfig(file)
		if err == nil && config.Width > 0 && config.Height > 0 {
			rawW = float64(config.Width)
			rawH = float64(config.Height)
		}
	}

	return
}

// convertOrientedBoxToRaw transforms a face bounding box [y1, x2, y2, x1] from
// the oriented/display coordinate space (as returned by cv2.imdecode which auto-rotates)
// to raw stored pixel coordinates (as used by Go's image.Decode without auto-orientation).
func convertOrientedBoxToRaw(box FaceBox, orientation int, rawW, rawH int) FaceBox {
	if len(box) < 4 || orientation <= 1 {
		return box
	}
	// box = [y1, x2, y2, x1] in oriented space
	oy1, ox2, oy2, ox1 := box[0], box[1], box[2], box[3]

	// The oriented image dimensions depend on orientation:
	// For orientations 5,6,7,8 the displayed image has swapped width/height vs raw
	var ry1, rx2, ry2, rx1 int

	switch orientation {
	case 2: // Flip Horizontal
		rx1 = rawW - ox2
		rx2 = rawW - ox1
		ry1 = oy1
		ry2 = oy2
	case 3: // Rotate 180
		rx1 = rawW - ox2
		rx2 = rawW - ox1
		ry1 = rawH - oy2
		ry2 = rawH - oy1
	case 4: // Flip Vertical
		rx1 = ox1
		rx2 = ox2
		ry1 = rawH - oy2
		ry2 = rawH - oy1
	case 5: // Transpose (flip H + 270 CW)
		// Inverse: raw_x=disp_y, raw_y=disp_x
		rx1 = oy1
		rx2 = oy2
		ry1 = ox1
		ry2 = ox2
	case 6: // Rotate 90 CW
		// Inverse: raw_x=disp_y, raw_y=rawH-disp_x
		rx1 = oy1
		rx2 = oy2
		ry1 = rawH - ox2
		ry2 = rawH - ox1
	case 7: // Transverse (flip H + 90 CW)
		// Inverse: raw_x=rawW-disp_y, raw_y=rawH-disp_x
		rx1 = rawW - oy2
		rx2 = rawW - oy1
		ry1 = rawH - ox2
		ry2 = rawH - ox1
	case 8: // Rotate 270 CW (90 CCW)
		// Inverse: raw_x=rawW-disp_y, raw_y=disp_x
		rx1 = rawW - oy2
		rx2 = rawW - oy1
		ry1 = ox1
		ry2 = ox2
	default:
		return box
	}

	return FaceBox{ry1, rx2, ry2, rx1}
}

func processSingleFile(dirPath string, name string, facesData FacesFile, cfg settings.FacialRecognition, force bool) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return false
	}

	// Check if we already have ML data for this file
	existing, alreadyProcessed := facesData[name]
	hasML := false
	needsLearning := false
	if alreadyProcessed {
		for _, f := range existing {
			if f.Source == "ml" || f.Source == "ml_scanned" || f.Source == "deleted" {
				hasML = true
			}
			// If it's a high-confidence ACDSee face but HAS NO embedding, we still need to process/learn it
			if f.Source == "acdsee" && f.Confidence >= 0.85 && (f.Embedding == nil || len(f.Embedding) == 0) {
				needsLearning = true
			}
		}
	}

	if alreadyProcessed && hasML && !needsLearning && !force {
		log.Printf("[FacialRec] Skip %s: already has ML data and no pending ACDSee learning", name)
		return false // Fully processed
	}

	fullPath := filepath.Join(dirPath, name)
	log.Printf("[FacialRec] Scanning: %s", fullPath)

	// 1. ACDSee baseline
	var baseFaces []FaceEntry
	if alreadyProcessed && !force {
		baseFaces = existing
	} else {
		log.Printf("[FacialRec]   Extracting ACDSee regions for %s", fullPath)
		freshACDSee, _ := ExtractACDSeeRegions(fullPath)
		log.Printf("[FacialRec]   Found %d ACDSee regions", len(freshACDSee))

		// Preserve manual ML faces or deleted markers from existing data
		var preserved []FaceEntry
		if alreadyProcessed {
			orient, w, h := getImageExifInfo(fullPath)
			for _, f := range existing {
				if (f.Source == "deleted" || (f.Source == "ml" && f.Confidence >= 0.99)) && isBoxValid(f.Box, orient, int(w), int(h)) {
					preserved = append(preserved, f)
				}
			}
		}

		baseFaces = preserved
		for _, fresh := range freshACDSee {
			// Skip if this fresh ACDSee face overlaps with a face we manually deleted
			isDeleted := false
			for _, p := range preserved {
				if p.Source == "deleted" && CalculateIOU(fresh.Box, p.Box) > 0.40 {
					isDeleted = true
					break
				}
			}
			if !isDeleted {
				baseFaces = append(baseFaces, fresh)
			}
		}

		// Group entries by person to avoid index/delete tug-of-war
		personHasVerified := make(map[string]bool)
		for _, f := range baseFaces {
			if f.Name != "" && f.Name != "Unknown" && (f.Source == "ml" || f.Confidence >= 0.85) {
				personHasVerified[f.Name] = true
			}
		}

		seenPeople := make(map[string]bool)
		for i, f := range baseFaces {
			if f.Name != "" && f.Name != "Unknown" {
				if seenPeople[f.Name] {
					continue
				}
				boxStr := ""
				if len(f.Box) == 4 {
					boxStr = fmt.Sprintf("%d,%d,%d,%d", f.Box[0], f.Box[1], f.Box[2], f.Box[3])
				}

				// Only map verified ACDSee faces (confidence >= 0.85) to the global search index
				if f.Confidence >= 0.85 {
					people.MapFaceToIndex(f.Name, fullPath, f.Confidence, boxStr)
					seenPeople[f.Name] = true
				} else if !personHasVerified[f.Name] {
					// Only remove if we DON'T have a verified entry for this person in this image
					people.RemoveFaceFromIndex(f.Name, fullPath, "")
				}

				// "LEARN": Get embedding for the ACDSee box if it's manual
				if f.Source == "acdsee" && f.Confidence >= 0.85 && (f.Box != nil && len(f.Box) == 4 && f.Box[1] > 0) {
					// Check if we already have it in people.db to save time and Python server load
					hasEmb, _ := people.HasEmbedding(f.Name, fullPath)
					if hasEmb {
						log.Printf("[FacialRec]   Skip learning for %s: embedding already exists in DB", f.Name)
						continue
					}

					log.Printf("[FacialRec] Learning face for %s at box %v", f.Name, f.Box)
					emb, err := LearnFace(cfg.ServerAddress, fullPath, f.Box)
					if err == nil {
						log.Printf("[FacialRec] Successfully got embedding for %s (len=%d). Saving to DB.", f.Name, len(emb))
						err = people.SaveFaceEmbedding(f.Name, fullPath, emb)
						if err != nil {
							log.Printf("[FacialRec] ERROR saving embedding to DB: %v", err)
						}
						baseFaces[i].Embedding = emb
					} else {
						log.Printf("[FacialRec] ERROR learning face for %s: %v", f.Name, err)
					}
				}
			}
		}
	}

	// 2. Python ML overlay
	log.Printf("[FacialRec]   Analyzing via Python ML server for %s", fullPath)
	mlFaces, err := AnalyzeFile(cfg.ServerAddress, fullPath)
	if err != nil {
		log.Printf("[FacialRec]   Error analyzing %s via Python server: %v", name, err)
	} else {
		log.Printf("[FacialRec]   Found %d ML faces", len(mlFaces))
		// cv2.imdecode auto-rotates images, so ML boxes are in oriented/display space.
		// We need to:
		// 1. Compute oriented-space Area (for Preview.vue face overlays which use browser's auto-oriented image)
		// 2. Convert box to raw pixel space (for thumbnail cropping which decodes raw pixels)
		orientation, rawW, rawH := getImageExifInfo(fullPath)

		// Determine oriented display dimensions
		dispW, dispH := rawW, rawH
		if orientation >= 5 && orientation <= 8 {
			dispW, dispH = rawH, rawW // orientations 5-8 swap width/height
		}

		for i, mlf := range mlFaces {
			if len(mlf.Box) == 4 && dispW > 0 && dispH > 0 {
				// Compute oriented-space Area from the original oriented box BEFORE conversion
				oy1, ox2, oy2, ox1 := float64(mlf.Box[0]), float64(mlf.Box[1]), float64(mlf.Box[2]), float64(mlf.Box[3])
				ow := ox2 - ox1
				oh := oy2 - oy1
				ocx := ox1 + ow/2
				ocy := oy1 + oh/2
				mlFaces[i].Area = &FaceArea{
					X: ocx / dispW,
					Y: ocy / dispH,
					W: ow / dispW,
					H: oh / dispH,
				}

				// Now convert box to raw pixel space for thumbnail cropping
				if orientation > 1 {
					mlFaces[i].Box = convertOrientedBoxToRaw(mlf.Box, orientation, int(rawW), int(rawH))
					log.Printf("[FacialRec]   ML face %d: oriented %v -> raw %v (Area: %.3f,%.3f)", i, mlf.Box, mlFaces[i].Box, mlFaces[i].Area.X, mlFaces[i].Area.Y)
				}
			}
		}
		if orientation > 1 {
			log.Printf("[FacialRec] Image %s has orientation %d (raw %dx%d, display %dx%d), converted %d ML boxes", name, orientation, int(rawW), int(rawH), int(dispW), int(dispH), len(mlFaces))
		}
		// Identify ML faces from known embeddings
		allKnowns, err := people.GetAllEmbeddings()
		restricted, allowedPeople := getFolderLimits(dirPath)
		var knowns []people.PersonEmbedding

		if err != nil {
			log.Printf("[FacialRec] ERROR getting known embeddings from DB: %v", err)
		} else {
			if restricted && len(allowedPeople) > 0 {
				allowedMap := make(map[string]bool)
				for _, p := range allowedPeople {
					allowedMap[strings.ToLower(strings.TrimSpace(p))] = true
				}
				for _, k := range allKnowns {
					if allowedMap[strings.ToLower(k.Name)] {
						knowns = append(knowns, k)
					}
				}
				log.Printf("[FacialRec] Loaded %d known embeddings (restricted to %d allowed people).", len(knowns), len(allowedPeople))
			} else {
				knowns = allKnowns
				log.Printf("[FacialRec] Loaded %d known embeddings from DB for matching.", len(knowns))
			}
		}

		for i, mlf := range mlFaces {
			if len(mlf.Embedding) > 0 {
				bestMatch := ""
				bestScore := 0.0
				for _, k := range knowns {
					sim := CosineSimilarity(mlf.Embedding, k.Embedding)
					if sim > bestScore {
						bestScore = sim
						bestMatch = k.Name
					}
				}

				// Similarity threshold for recognition
				if bestScore > 0.82 {
					mlFaces[i].Name = bestMatch
					mlFaces[i].Confidence = bestScore
					log.Printf("[FacialRec] Recognized %s in %s (sim=%.2f)", bestMatch, name, bestScore)
				}
			}
		}

		// We no longer filter out ML faces that overlap with ACDSee faces because
		// the UI now has mutually exclusive toggles (ML vs ACDSee). Both should be saved.
		// HOWEVER, we should merge them to avoid literal duplicates in the JSON.
		baseFaces = deduplicateFaces(append(baseFaces, mlFaces...))

		// If ML ran but no ML faces survived IOU filtering, add a marker
		// so the weekly background scan knows not to re-process this image.
		if len(mlFaces) == 0 {
			hasMLMarker := false
			for _, f := range baseFaces {
				if f.Source == "ml_scanned" || f.Source == "deleted" {
					hasMLMarker = true
					break
				}
			}
			if !hasMLMarker {
				baseFaces = append(baseFaces, FaceEntry{
					Box:    FaceBox{},
					Source: "ml_scanned",
				})
				log.Printf("[FacialRec] No new ML faces for %s, added ml_scanned marker", name)
			}
		}

		// Map high-confidence ML faces to `people.db`
		for _, f := range baseFaces {
			if f.Source == "ml" && f.Name != "" && f.Name != "Unknown" && f.Confidence > 0.90 {
				boxStr := ""
				if len(f.Box) == 4 {
					boxStr = fmt.Sprintf("%d,%d,%d,%d", f.Box[0], f.Box[1], f.Box[2], f.Box[3])
				}
				// Prioritize indexing even if already processed above (Insert OR Replace)
				people.MapFaceToIndex(f.Name, fullPath, f.Confidence, boxStr)
				if len(f.Embedding) > 0 {
					people.SaveFaceEmbedding(f.Name, fullPath, f.Embedding)
				}
			}
		}
	}

	facesData[name] = baseFaces
	return true
}

// getFolderPeopleScope walks up from the directory, reading folderdetails.json
func getFolderPeopleScope(dir string) []string {
	return []string{}
}

// ScanAllSources triggers scanning on all valid sources
func ScanAllSources(set *settings.Settings, store *storage.Storage) {
	if !set.Integrations.FacialRecognition.Enabled {
		log.Println("[FacialRec] System disabled in global settings, skipping scheduled job.")
		return
	}

	log.Println("[START] Facial Recognition Background Scanner")
	start := time.Now()

	cfg := set.Integrations.FacialRecognition
	if cfg.ServerAddress == "" {
		cfg.ServerAddress = "http://localhost:8000"
	}

	// Helper to init the SQLite DB root level
	people.InitDB(set.Server.Database + "_people")

	for _, source := range set.Server.Sources {
		log.Printf("[FacialRec] Starting source: %s", source.Path)

		filepath.Walk(source.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				name := info.Name()
				// Skip common development directories and hidden folders to save massive amounts of time
				if name == "node_modules" || name == "venv" || name == ".venv" || name == ".git" || strings.HasPrefix(name, ".") {
					if path != source.Path { // Don't skip the root source path itself if it happens to be named this way
						return filepath.SkipDir
					}
				}
				ScanFolder(path, cfg, store, false)
			}
			return nil
		})
	}

	log.Printf("[DONE] Facial Recognition Scanner finished in %v", time.Since(start))
}

// deduplicateFaces merges overlapping boxes while preserving the best metadata.
func deduplicateFaces(faces []FaceEntry) []FaceEntry {
	if len(faces) < 2 {
		return faces
	}

	result := []FaceEntry{}
	merged := make(map[int]bool)

	for i := 0; i < len(faces); i++ {
		if merged[i] {
			continue
		}

		current := faces[i]
		for j := i + 1; j < len(faces); j++ {
			if merged[j] {
				continue
			}

			iou := CalculateIOU(current.Box, faces[j].Box)
			isMatch := false
			
			// 1. Basic IOU threshold for any overlapping faces
			if iou > 0.7 {
				isMatch = true
			}
			
			// 2. More aggressive threshold for identical names (catch stale boxes with slightly different coords)
			if current.Name != "" && current.Name != "Unknown" && current.Name == faces[j].Name && iou > 0.4 {
				isMatch = true
			}

			if isMatch {
				// Duplicate found! Merge faces[j] into current
				other := faces[j]

				// 1. Prioritize name (take non-unknown)
				if (current.Name == "" || current.Name == "Unknown") && other.Name != "" && other.Name != "Unknown" {
					current.Name = other.Name
				}
				// 2. Prioritize ML source for embedding/confidence
				if other.Source == "ml" && current.Source != "ml" {
					current.Source = "ml"
					current.Confidence = other.Confidence
					current.Embedding = other.Embedding
					if other.Area != nil {
						current.Area = other.Area
					}
				} else if current.Source == "ml" && other.Source == "ml" {
					// Both ML? Keep the higher confidence or more complete one
					if other.Confidence > current.Confidence {
						current.Confidence = other.Confidence
					}
					if len(other.Embedding) > 0 && len(current.Embedding) == 0 {
						current.Embedding = other.Embedding
					}
				}
				
				merged[j] = true
			}
		}
		result = append(result, current)
	}

	return result
}

// isBoxValid checks if the box is within reasonable bounds of the image dimensions.
func isBoxValid(box FaceBox, orientation, rawW, rawH int) bool {
	if len(box) != 4 || rawW <= 0 || rawH <= 0 {
		return true // Can't validate without dimensions
	}
	y1, x2, y2, x1 := box[0], box[1], box[2], box[3]
	
	// Allow for a small margin of error (e.g. 50px)
	margin := 50
	
	// Determine the oriented (runtime) bounds
	maxW, maxH := rawW, rawH
	if orientation >= 5 && orientation <= 8 {
		maxW, maxH = rawH, rawW // swapped width/height
	}

	if y1 < -margin || y2 > maxH+margin || x1 < -margin || x2 > maxW+margin {
		return false
	}
	
	// Sanity check for box size
	if y2 <= y1 || x2 <= x1 {
		return false
	}
	
	return true
}
