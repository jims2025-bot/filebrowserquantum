package facerec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/people"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
)

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

	args := []string{"-m", "-j", "-struct", "-XMP:RegionInfo", "-XMP:RegionInfoACDSee", "-XMP:Regions", "-ImageWidth", "-ImageHeight", imagePath}
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
	if w, ok := exifData["ImageWidth"].(float64); ok {
		imgW = w
	}
	if imgW == 0 {
		if w, ok := exifData["ImageWidth"].(int); ok {
			imgW = float64(w)
		}
	}
	imgH := 0.0
	if h, ok := exifData["ImageHeight"].(float64); ok {
		imgH = h
	}
	if imgH == 0 {
		if h, ok := exifData["ImageHeight"].(int); ok {
			imgH = float64(h)
		}
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
					x1 := (fa.X - fa.W/2) * imgW
					y1 := (fa.Y - fa.H/2) * imgH
					x2 := (fa.X + fa.W/2) * imgW
					y2 := (fa.Y + fa.H/2) * imgH
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

	// Open or create `faces.json`
	facesFilePath := filepath.Join(dirPath, "faces.json")
	var facesData FacesFile

	jsonBytes, err := os.ReadFile(facesFilePath)
	if err == nil {
		json.Unmarshal(jsonBytes, &facesData)
		// Fast-sync existing faces to people.db to ensure all boxes are populated
		for filename, entries := range facesData {
			fullPath := filepath.Join(dirPath, filename)
			for _, f := range entries {
				if f.Name != "" && f.Name != "Unknown" {
					boxStr := ""
					if len(f.Box) == 4 {
						boxStr = fmt.Sprintf("%d,%d,%d,%d", f.Box[0], f.Box[1], f.Box[2], f.Box[3])
					}
					// Use 1.0 confidence for manual/unknown, or the actual confidence
					conf := f.Confidence
					if conf == 0 {
						conf = 1.0
					}
					people.MapFaceToIndex(f.Name, fullPath, conf, boxStr)
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

func processSingleFile(dirPath string, name string, facesData FacesFile, cfg settings.FacialRecognition, force bool) bool {
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		return false
	}

	// Check if we already have ML data for this file
	existing, alreadyProcessed := facesData[name]
	hasML := false
	if alreadyProcessed {
		for _, f := range existing {
			if f.Source == "ml" {
				hasML = true
				break
			}
		}
	}

	if alreadyProcessed && hasML && !force {
		log.Printf("[FacialRec] Skip %s: already has ML data", name)
		return false // Fully processed
	}

	fullPath := filepath.Join(dirPath, name)
	log.Printf("[FacialRec] Scanning: %s", fullPath)

	// 1. ACDSee baseline
	var baseFaces []FaceEntry
	if alreadyProcessed && !force {
		baseFaces = existing
	} else {
		baseFaces, _ = ExtractACDSeeRegions(fullPath)
		for i, f := range baseFaces {
			if f.Name != "" && f.Name != "Unknown" {
				boxStr := ""
				if len(f.Box) == 4 {
					boxStr = fmt.Sprintf("%d,%d,%d,%d", f.Box[0], f.Box[1], f.Box[2], f.Box[3])
				}
				people.MapFaceToIndex(f.Name, fullPath, 1.0, boxStr)

				// "LEARN": Get embedding for the ACDSee box if it's manual
				if f.Source == "acdsee" && f.Confidence >= 0.99 && (f.Box != nil && len(f.Box) == 4 && f.Box[1] > 0) {
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
				} else {
					log.Printf("[FacialRec] Skipping learning for %s (Confidence: %f, Box: %v)", f.Name, f.Confidence, f.Box)
				}
			}
		}
	}

	// 2. Python ML overlay
	mlFaces, err := AnalyzeFile(cfg.ServerAddress, fullPath)
	if err != nil {
		log.Printf("[FacialRec] Error analyzing %s via Python server: %v", name, err)
	} else {
		// Identify ML faces from known embeddings
		knowns, err := people.GetAllEmbeddings()
		if err != nil {
			log.Printf("[FacialRec] ERROR getting known embeddings from DB: %v", err)
		} else {
			log.Printf("[FacialRec] Loaded %d known embeddings from DB for matching.", len(knowns))
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

		// Filter out ML faces that heavily overlap with existing ACDSee faces
		var filteredMLFaces []FaceEntry
		for _, mlf := range mlFaces {
			isDuplicate := false
			for _, bf := range baseFaces {
				if bf.Source == "acdsee" && len(bf.Box) == 4 && len(mlf.Box) == 4 {
					iou := CalculateIOU(bf.Box, mlf.Box)
					if iou > 0.35 { // If more than 35% overlap, consider it the same face
						isDuplicate = true
						break
					}
				}
			}
			if !isDuplicate {
				filteredMLFaces = append(filteredMLFaces, mlf)
			}
		}

		baseFaces = append(baseFaces, filteredMLFaces...)

		// Map high-confidence ML faces to `people.db`
		for _, f := range filteredMLFaces {
			if f.Name != "" && f.Name != "Unknown" && f.Confidence > 0.90 {
				boxStr := ""
				if len(f.Box) == 4 {
					boxStr = fmt.Sprintf("%d,%d,%d,%d", f.Box[0], f.Box[1], f.Box[2], f.Box[3])
				}
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
