package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

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
	err = facerec.ScanFile(diskPath, settings.Config.Integrations.FacialRecognition, store, true) // Manual scan is forced
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, map[string]string{"status": "scanned"})
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
	err = facerec.ScanFolder(diskPath, settings.Config.Integrations.FacialRecognition, store, true) // Manual scan is forced
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, map[string]string{"status": "scanned"})
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
	if entries, ok := facesData[filename]; ok {
		for i, e := range entries {
			// Basic box matching
			if len(e.Box) == 4 && len(req.Box) == 4 && e.Box[0] == req.Box[0] && e.Box[1] == req.Box[1] {
				facesData[filename][i].Name = req.NewName
				facesData[filename][i].Confidence = 1.0 // Manual approval!
				updated = true
				break
			}
		}
	}

	if updated {
		b, _ := json.MarshalIndent(facesData, "", "  ")
		os.WriteFile(facesFilePath, b, 0666)

		// Update people.db
		people.RemoveFaceFromIndex(req.OldName, diskPath)
		people.MapFaceToIndex(req.NewName, diskPath, 1.0)
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
