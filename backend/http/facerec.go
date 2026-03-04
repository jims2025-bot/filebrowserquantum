package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/people"
	"github.com/jims2025-bot/filebrowserquantum/backend/facerec"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

// FaceMgmtReq represents the JSON payload to update or remove a face
type FaceMgmtReq struct {
	ImagePath string          `json:"imagePath"` // Absolute path to image
	OldName   string          `json:"oldName"`
	NewName   string          `json:"newName,omitempty"` // For update/renaming
	Box       facerec.FaceBox `json:"box"`
}

func faceScanFileHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.RunFaceScan {
		return http.StatusForbidden, nil
	}

	imagePath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if imagePath == "" {
		return http.StatusBadRequest, nil
	}
	logger.Debug(fmt.Sprintf("[FacialRec] ScanFile Request: path=%s, source=%s", imagePath, source))
	scopePath, realSource, err := ResolveScopePath(d.user, source, imagePath)
	if err != nil {
		logger.Error(fmt.Sprintf("[FacialRec] ResolveScopePath failed: %v", err))
		return http.StatusForbidden, err
	}
	index := indexing.GetIndex(realSource)
	if index == nil {
		logger.Error(fmt.Sprintf("[FacialRec] Index not found for source: %s", realSource))
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", realSource)
	}

	diskPath, _, err := index.GetRealPath("/", scopePath)
	if err != nil {
		logger.Error(fmt.Sprintf("[FacialRec] GetRealPath failed: %v", err))
		return http.StatusInternalServerError, fmt.Errorf("could not resolve absolute path: %v", err)
	}
	logger.Debug(fmt.Sprintf("[FacialRec] Resolved disk path: %s", diskPath))

	// Run the heavy file scan asynchronously so the frontend can poll it
	go func() {
		err := facerec.ScanFile(diskPath, settings.Config.Integrations.FacialRecognition, store, true) // Manual scan is forced
		if err != nil {
			logger.Errorf("[FacialRec] Async ScanFile failed for %s: %v", diskPath, err)
		}
	}()

	return renderJSON(w, r, map[string]string{"status": "scanning started in background"})
}

func faceScanFolderHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.RunFaceScan {
		return http.StatusForbidden, nil
	}

	folderPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	if folderPath == "" {
		return http.StatusBadRequest, nil
	}
	scopePath, realSource, err := ResolveScopePath(d.user, source, folderPath)
	if err != nil {
		return http.StatusForbidden, err
	}
	index := indexing.GetIndex(realSource)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", realSource)
	}

	diskPath, _, err := index.GetRealPath("/", scopePath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not resolve absolute path: %v", err)
	}
	// Run the heavy folder scan asynchronously so the frontend can poll it
	go func() {
		err := facerec.ScanFolder(diskPath, settings.Config.Integrations.FacialRecognition, store, true) // Manual scan is forced
		if err != nil {
			logger.Errorf("[FacialRec] Async ScanFolder failed for %s: %v", diskPath, err)
		}
	}()

	return renderJSON(w, r, map[string]string{"status": "scanning started in background"})
}

func faceScanStatusHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	status := facerec.GetScanStatus()
	return renderJSON(w, r, status)
}

func faceUpdateHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.ManageFaces {
		return http.StatusForbidden, nil
	}

	var req FaceMgmtReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}

	source := r.URL.Query().Get("source")
	scopePath, realSource, err := ResolveScopePath(d.user, source, req.ImagePath)
	if err != nil {
		return http.StatusForbidden, err
	}
	index := indexing.GetIndex(realSource)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", realSource)
	}

	diskPath, _, err := index.GetRealPath("/", scopePath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not resolve absolute path: %v", err)
	}

	dir := filepath.Dir(diskPath)
	filename := filepath.Base(diskPath)
	facesFilePath := filepath.Join(dir, "faces.json")

	var facesData facerec.FacesFile
	jsonBytes, err := os.ReadFile(facesFilePath)
	if err == nil {
		json.Unmarshal(jsonBytes, &facesData)
	}

	// Update the specific face
	updated := false
	var existingEmb []float64

	if entries, ok := facesData[filename]; ok {
		var mlFound bool
		var acdseeFound bool
		var acdseeTemplate facerec.FaceEntry

		for i, e := range entries {
			// Basic box matching
			if len(e.Box) == 4 && len(req.Box) == 4 && e.Box[0] == req.Box[0] && e.Box[1] == req.Box[1] {
				if e.Source == "ml" {
					facesData[filename][i].Name = req.NewName
					facesData[filename][i].Confidence = 1.0 // Manual approval!
					existingEmb = e.Embedding
					updated = true
					mlFound = true
				} else if e.Source == "acdsee" {
					acdseeFound = true
					acdseeTemplate = e
				}
			}
		}

		// If no ML face was found but an ACDSee face was, duplicate it to an ML face
		if !mlFound && acdseeFound {
			newFace := acdseeTemplate
			newFace.Source = "ml"
			newFace.Name = req.NewName
			newFace.Confidence = 1.0
			existingEmb = newFace.Embedding
			facesData[filename] = append(facesData[filename], newFace)
			updated = true
		}
	}

	if updated {
		b, _ := json.MarshalIndent(facesData, "", "  ")
		os.WriteFile(facesFilePath, b, 0666)

		// Update people.db index mapping
		boxStr := ""
		if len(req.Box) == 4 {
			boxStr = fmt.Sprintf("%d,%d,%d,%d", req.Box[0], req.Box[1], req.Box[2], req.Box[3])
		}
		people.RemoveFaceFromIndex(req.OldName, diskPath)
		people.MapFaceToIndex(req.NewName, diskPath, 1.0, boxStr)

		// Auto-append manually verified face to folderdetails.json
		folderDetailsPath := filepath.Join(dir, folderDetailsFilename)
		var fd FolderDetails
		if fdBytes, err := os.ReadFile(folderDetailsPath); err == nil {
			json.Unmarshal(fdBytes, &fd)
		} else {
			fd = FolderDetails{People: []string{}}
		}
		nameFound := false
		for _, p := range fd.People {
			if strings.EqualFold(p, req.NewName) {
				nameFound = true
				break
			}
		}
		if !nameFound && req.NewName != "Unknown" {
			fd.People = append(fd.People, req.NewName)
			fdOut, _ := json.MarshalIndent(fd, "", "  ")
			os.WriteFile(folderDetailsPath, fdOut, 0644)
		}

		// Train the ML model using this manual approval!
		if len(existingEmb) > 0 {
			// Embedding already acquired previously, just save it to DB
			people.SaveFaceEmbedding(req.NewName, diskPath, existingEmb)
		} else {
			// No embedding (e.g. native ACDSee face being renamed for the first time). Learn it!
			go func() {
				emb, err := facerec.LearnFace(settings.Config.Integrations.FacialRecognition.ServerAddress, diskPath, req.Box)
				if err == nil && len(emb) > 0 {
					people.SaveFaceEmbedding(req.NewName, diskPath, emb)

					// Update faces.json to ensure the embedding is cached permanently
					if jsonBytes, err := os.ReadFile(facesFilePath); err == nil {
						var asyncData facerec.FacesFile
						if err := json.Unmarshal(jsonBytes, &asyncData); err == nil {
							if entries, ok := asyncData[filename]; ok {
								for i, e := range entries {
									if len(e.Box) == 4 && len(req.Box) == 4 && e.Box[0] == req.Box[0] && e.Box[1] == req.Box[1] {
										asyncData[filename][i].Embedding = emb
										bAsync, _ := json.MarshalIndent(asyncData, "", "  ")
										os.WriteFile(facesFilePath, bAsync, 0666)
										break
									}
								}
							}
						}
					}
					logger.Debug(fmt.Sprintf("[FacialRec] Successfully learned and saved embedding for renamed face: %s", req.NewName))
				} else {
					logger.Error(fmt.Sprintf("[FacialRec] Failed to learn face during rename for %s: %v", req.NewName, err))
				}
			}()
		}
	}

	return renderJSON(w, r, facesData[filename])
}

func faceRemoveHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.ManageFaces {
		return http.StatusForbidden, nil
	}

	var req FaceMgmtReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}

	source := r.URL.Query().Get("source")
	scopePath, realSource, err := ResolveScopePath(d.user, source, req.ImagePath)
	if err != nil {
		return http.StatusForbidden, err
	}
	index := indexing.GetIndex(realSource)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", realSource)
	}
	diskPath, _, err := index.GetRealPath("/", scopePath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not resolve absolute path: %v", err)
	}

	dir := filepath.Dir(diskPath)
	filename := filepath.Base(diskPath)
	facesFilePath := filepath.Join(dir, "faces.json")

	var facesData facerec.FacesFile
	jsonBytes, err := os.ReadFile(facesFilePath)
	if err == nil {
		json.Unmarshal(jsonBytes, &facesData)
	}

	// Remove the specific face
	updated := false
	if entries, ok := facesData[filename]; ok {
		var newEntries []facerec.FaceEntry
		for _, e := range entries {
			if len(e.Box) == 4 && len(req.Box) == 4 && e.Box[0] == req.Box[0] && e.Box[1] == req.Box[1] {
				updated = true // Skip appending this one
				continue
			}
			newEntries = append(newEntries, e)
		}
		facesData[filename] = newEntries
	}

	if updated {
		b, _ := json.MarshalIndent(facesData, "", "  ")
		os.WriteFile(facesFilePath, b, 0666)

		// Remove from people.db
		people.RemoveFaceFromIndex(req.OldName, diskPath)
	}

	return renderJSON(w, r, map[string]string{"status": "removed"})
}

func faceUpdateAvatarHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.ManageFaces {
		return http.StatusForbidden, nil
	}

	var req struct {
		Name      string `json:"name"`
		ImagePath string `json:"imagePath"`
		Box       string `json:"box"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return http.StatusBadRequest, err
	}

	source := r.URL.Query().Get("source")
	scopePath, realSource, err := ResolveScopePath(d.user, source, req.ImagePath)
	if err != nil {
		return http.StatusForbidden, err
	}
	index := indexing.GetIndex(realSource)
	if index == nil {
		return http.StatusBadRequest, fmt.Errorf("index not found for source %s", realSource)
	}
	diskPath, _, err := index.GetRealPath("/", scopePath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not resolve absolute path: %v", err)
	}

	err = people.SetPersonAvatar(req.Name, diskPath, req.Box)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, map[string]string{"status": "avatar updated"})
}

// PersonAutocompleteResult represents a single dropdown suggestion.
type PersonAutocompleteResult struct {
	Name      string `json:"name"`
	AvatarUrl string `json:"avatarUrl"`
}

func faceAutocompleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !settings.Config.Integrations.FacialRecognition.Enabled {
		return renderJSON(w, r, []PersonAutocompleteResult{})
	}

	query := r.URL.Query().Get("q")
	scope := r.URL.Query().Get("scope")
	source := r.URL.Query().Get("source")

	pathPrefix := ""
	if scope != "" || source != "" {
		// Resolve scope to physical path
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

	if query == "" {
		summary, err := people.GetPeopleSummary(pathPrefix)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		// Resolve "RAW:" hints to virtual URLs
		for i, s := range summary {
			if strings.HasPrefix(s.AvatarUrl, "RAW:") {
				parts := strings.Split(strings.TrimPrefix(s.AvatarUrl, "RAW:"), "?")
				diskPath := parts[0]
				boxParam := ""
				if len(parts) > 1 {
					boxParam = parts[1]
				}

				// Find source
				for _, src := range settings.Config.Server.Sources {
					if strings.HasPrefix(diskPath, src.Path) {
						vPath := filepath.ToSlash(strings.TrimPrefix(diskPath, src.Path))
						if !strings.HasPrefix(vPath, "/") {
							vPath = "/" + vPath
						}

						ePath := (&url.URL{Path: vPath}).String()
						eSource := url.QueryEscape(src.Name)
						// boxParam is already "box=..." from Split.
						// Let's re-encode the coordinates part accurately.
						resolvedBox := ""
						if strings.HasPrefix(boxParam, "box=") {
							coords := strings.TrimPrefix(boxParam, "box=")
							resolvedBox = "&box=" + url.QueryEscape(coords)
						}
						summary[i].AvatarUrl = fmt.Sprintf("/api/preview%s?source=%s%s&size=thumb", ePath, eSource, resolvedBox)
						break
					}
				}
				if strings.HasPrefix(summary[i].AvatarUrl, "RAW:") {
					summary[i].AvatarUrl = "" // Failed to resolve
				}
			}
		}

		return renderJSON(w, r, summary)
	}

	// 1. Get raw database matches (returns matches with boxes)
	matches, err := people.SearchImagesByPerson(query, pathPrefix, 10, 0)
	if err != nil || len(matches) == 0 {
		return renderJSON(w, r, []PersonAutocompleteResult{})
	}

	var results []PersonAutocompleteResult
	seenNames := make(map[string]bool)

	// 2. Iterate through matches
	for _, m := range matches {
		diskPath := m.ImagePath
		dir := filepath.Dir(diskPath)
		filename := filepath.Base(diskPath)
		facesFilePath := filepath.Join(dir, "faces.json")

		var facesData facerec.FacesFile
		jsonBytes, err := os.ReadFile(facesFilePath)
		if err != nil {
			continue
		}
		json.Unmarshal(jsonBytes, &facesData)

		if entries, ok := facesData[filename]; ok {
			for _, e := range entries {
				if strings.Contains(strings.ToLower(e.Name), strings.ToLower(query)) {
					if !seenNames[e.Name] {
						seenNames[e.Name] = true

						vPath := ""
						srcName := ""
						for _, src := range settings.Config.Server.Sources {
							if strings.HasPrefix(diskPath, src.Path) {
								vPath = filepath.ToSlash(strings.TrimPrefix(diskPath, src.Path))
								if !strings.HasPrefix(vPath, "/") {
									vPath = "/" + vPath
								}
								srcName = src.Name
								break
							}
						}

						boxParam := ""
						if len(e.Box) == 4 {
							boxParam = fmt.Sprintf("%d,%d,%d,%d", e.Box[0], e.Box[1], e.Box[2], e.Box[3])
						}

						results = append(results, PersonAutocompleteResult{
							Name:      e.Name,
							AvatarUrl: fmt.Sprintf("/api/preview%s?source=%s&box=%s&size=thumb", (&url.URL{Path: vPath}).String(), url.QueryEscape(srcName), url.QueryEscape(boxParam)),
						})
					}
				}
			}
		}

		if len(results) >= 10 {
			break
		}
	}

	return renderJSON(w, r, results)
}
