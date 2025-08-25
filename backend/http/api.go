package http

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"log"

	"github.com/jims2025-bot/filebrowserquantum/backend/auth"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/files"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
)

// createApiKeyHandler creates an API key for the user.
// @Summary Create API key
// @Description Create an API key with specified name, duration, and permissions.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param name query string true "Name of the API key"
// @Param days query string true "Duration of the API key in days"
// @Param permissions query string true "Permissions for the API key (comma-separated)"
// @Success 200 {object} HttpResponse "Token created successfully, resoponse contains json object with token key"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 409 {object} map[string]string "Conflict"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/createApiKey [post]
func createApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	durationStr := r.URL.Query().Get("days")
	permissionsStr := r.URL.Query().Get("permissions")

	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to create api keys")
	}

	if name == "" {
		return http.StatusBadRequest, fmt.Errorf("api name must be valid")
	}
	if durationStr == "" {
		return http.StatusBadRequest, fmt.Errorf("api duration must be valid")
	}
	if permissionsStr == "" {
		return http.StatusBadRequest, fmt.Errorf("api permissions must be valid")
	}
	permissions := users.Permissions{
		Api:    strings.Contains(permissionsStr, "api") && d.user.Permissions.Api,
		Admin:  strings.Contains(permissionsStr, "admin") && d.user.Permissions.Admin,
		Modify: strings.Contains(permissionsStr, "modify") && d.user.Permissions.Modify,
		Share:  strings.Contains(permissionsStr, "share") && d.user.Permissions.Share,
	}

	durationInt, err := strconv.ParseInt(durationStr, 10, 64)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid duration value: %w", err)
	}
	duration := time.Duration(durationInt) * time.Hour * 24

	token, err := makeSignedTokenAPI(d.user, name, duration, permissions)
	if err != nil {
		if strings.Contains(err.Error(), "key already exists with same name") {
			return http.StatusConflict, err
		}
		return http.StatusInternalServerError, err
	}
	response := HttpResponse{
		Message: "here is your token!",
		Token:   token.Key,
	}
	return renderJSON(w, r, response)
}

// deleteApiKeyHandler deletes an API key for the user.
// @Summary Delete API key
// @Description Delete an API key with specified name.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param name query string true "Name of the API key to delete"
// @Success 200 {object} HttpResponse "API key deleted successfully"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/deleteApiKey [delete]
func deleteApiKeyHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	name := r.URL.Query().Get("name")
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to delete api keys")
	}

	keyInfo, ok := d.user.ApiKeys[name]
	if !ok {
		return http.StatusNotFound, fmt.Errorf("api key not found")
	}

	err := store.Users.DeleteApiKey(d.user.ID, name)
	if err != nil {
		return http.StatusNotFound, err
	}

	auth.RevokeAPIKey(keyInfo.Key)
	response := HttpResponse{
		Message: "successfully deleted api key from user",
	}
	return renderJSON(w, r, response)
}

type AuthTokenMin struct {
	Key         string            `json:"key"`
	Name        string            `json:"name"`
	Created     int64             `json:"created"`
	Expires     int64             `json:"expires"`
	Permissions users.Permissions `json:"Permissions"`
}

// listApiKeysHandler lists all API keys or retrieves details for a specific key.
// @Summary List API keys
// @Description List all API keys or retrieve details for a specific key.
// @Tags API Keys
// @Accept json
// @Produce json
// @Param name query string false "Name of the API to retrieve details"
// @Success 200 {object} AuthTokenMin "List of API keys or specific key details"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/listApiKeys [get]
func listApiKeysHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	key := r.URL.Query().Get("name")
	if !d.user.Permissions.Api {
		return http.StatusForbidden, fmt.Errorf("user does not have permission to list api keys")
	}

	if key != "" {
		keyInfo, ok := d.user.ApiKeys[key]
		if !ok {
			return http.StatusNotFound, fmt.Errorf("api key not found")
		}
		modifiedKey := AuthTokenMin{
			Key:         keyInfo.Key,
			Name:        key,
			Created:     keyInfo.Created,
			Expires:     keyInfo.Expires,
			Permissions: keyInfo.Permissions,
		}
		return renderJSON(w, r, modifiedKey)
	}

	modifiedList := map[string]AuthTokenMin{}
	for key, value := range d.user.ApiKeys {
		modifiedList[key] = AuthTokenMin{
			Key:         value.Key,
			Created:     value.Created,
			Expires:     value.Expires,
			Permissions: value.Permissions,
		}
	}

	return renderJSON(w, r, modifiedList)
}

// getMetadataHandler fetches metadata for a file.
// @Summary Get file metadata
// @Description Fetches EXIF, IPTC, and XMP metadata for a given file.
// @Tags Files
// @Accept json
// @Produce json
// @Param source query string true "Name of the storage source"
// @Param path query string true "Path to the file"
// @Success 200 {object} map[string]interface{} "File metadata"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/metadata [get]
func getMetadataHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	log.SetPrefix("[getMetadataHandler] ")

	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")

	log.Printf("Query parameters: source=%s, path=%s", source, path)

	if source == "" || path == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}

	userScope := "/"
	if d.user.Username != "publicUser" {
		var err error
		userScope, err = settings.GetScopeFromSourceName(d.user.Scopes, source)
		if err != nil && d.share == nil {
			return http.StatusForbidden, fmt.Errorf("source %s is not available for user %s", source, d.user.Username)
		}
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source '%s' not found", source)
	}

	scopedPath := filepath.Join(userScope, path)
	log.Printf("Resolving scoped path: %s", scopedPath)

	realPath, _, err := idx.GetRealPath(scopedPath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("file not found: %w", err)
	}

	log.Printf("Real path resolved: %s", realPath)

	metadata, err := files.GetMetadata(realPath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not get metadata: %w", err)
	}

	return renderJSON(w, r, metadata)
}

// resourceGetInstructionsHandler fetches the XMP Photoshop:Instructions field.
// @Summary Get XMP Instructions
// @Description Fetch the XMP-photoshop:Instructions metadata field for a file.
// @Tags Files
// @Accept json
// @Produce json
// @Param source query string true "Name of the storage source"
// @Param path query string true "Path to the file"
// @Success 200 {object} map[string]string "XMP Instructions"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources/instructions [get]
func resourceGetInstructionsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")

	if source == "" || path == "" {
		return http.StatusBadRequest, fmt.Errorf("source and path are required")
	}

	userScope := "/"
	if d.user.Username != "publicUser" {
		var err error
		userScope, err = settings.GetScopeFromSourceName(d.user.Scopes, source)
		if err != nil && d.share == nil {
			return http.StatusForbidden, fmt.Errorf("source %s is not available for user %s", source, d.user.Username)
		}
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source '%s' not found", source)
	}

	scopedPath := filepath.Join(userScope, path)
	realPath, _, err := idx.GetRealPath(scopedPath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("file not found: %w", err)
	}

	instructions, err := files.GetXMPInstructions(realPath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not read XMP Instructions: %w", err)
	}

	return renderJSON(w, r, map[string]string{"instructions": instructions})
}

// resourceInstructionsHandler writes a string into the XMP Photoshop:Instructions field.
// @Summary Write XMP Instructions
// @Description Write a new string into the XMP-photoshop:Instructions metadata field of a file.
// @Tags Files
// @Accept json
// @Produce json
// @Param source query string true "Name of the storage source"
// @Param path query string true "Path to the file"
// @Param instructions query string true "Instructions string to write"
// @Success 200 {object} HttpResponse "Instructions updated successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "Not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/resources/instructions [post]
func resourceInstructionsHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	source := r.URL.Query().Get("source")
	path := r.URL.Query().Get("path")
	instructions := r.URL.Query().Get("instructions")

	if source == "" || path == "" || instructions == "" {
		return http.StatusBadRequest, fmt.Errorf("source, path, and instructions are required")
	}

	userScope := "/"
	if d.user.Username != "publicUser" {
		var err error
		userScope, err = settings.GetScopeFromSourceName(d.user.Scopes, source)
		if err != nil && d.share == nil {
			return http.StatusForbidden, fmt.Errorf("source %s is not available for user %s", source, d.user.Username)
		}
	}

	idx := indexing.GetIndex(source)
	if idx == nil {
		return http.StatusNotFound, fmt.Errorf("source '%s' not found", source)
	}

	scopedPath := filepath.Join(userScope, path)
	realPath, _, err := idx.GetRealPath(scopedPath)
	if err != nil {
		return http.StatusNotFound, fmt.Errorf("file not found: %w", err)
	}

	err = files.WriteXMPInstructions(realPath, instructions)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("could not write XMP Instructions: %w", err)
	}

	response := HttpResponse{
		Message: "XMP Instructions updated successfully",
	}
	return renderJSON(w, r, response)
}
