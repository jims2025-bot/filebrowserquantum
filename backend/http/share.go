package http

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/errors"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/share"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
)

// shareListHandler returns a list of all share links.
// @Summary List share links
// @Description Returns a list of share links for the current user, or all links if the user is an admin.
// @Tags Shares
// @Accept json
// @Produce json
// @Success 200 {array} share.Link "List of share links"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/shares [get]
func shareListHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	var (
		s   []*share.Link
		err error
	)
	if d.user.Permissions.Admin {
		s, err = store.Share.All()
	} else {
		s, err = store.Share.FindByUserID(d.user.ID)
	}
	if err == errors.ErrNotExist {
		return renderJSON(w, r, []*share.Link{})
	}
	if err != nil {
		return http.StatusInternalServerError, err
	}

	sort.Slice(s, func(i, j int) bool {
		if s[i].UserID != s[j].UserID {
			return s[i].UserID < s[j].UserID
		}
		return s[i].Expire < s[j].Expire
	})
	return renderJSON(w, r, s)
}

// shareGetsHandler retrieves share links for a specific resource path.
// @Summary Get share links by path
// @Description Retrieves all share links associated with a specific resource path for the current user.
// @Tags Shares
// @Accept json
// @Produce json
// @Param path query string true "Resource path for which to retrieve share links"
// @Param source query string true "Source name for share links"
// @Success 200 {array} share.Link "List of share links for the specified path"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/share [get]
func shareGetHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	encodedPath := r.URL.Query().Get("path")
	source := r.URL.Query().Get("source")
	logger.Debugf("shareGetHandler: path=%s, source=%s, user=%s", encodedPath, source, d.user.Username)
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
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}

	// Resolve the source name (which might be an alias or Name:Index) to the real source path/name
	_, realSource, err := settings.GetScopeFromSourceString(d.user.Scopes, source)
	if err != nil {
		// FALLBACK: If user is Admin, try to resolve directly from NameToSource
		if d.user.Perm.Admin {
			// Clean source name (remove :Index suffix if present)
			cleanSourceName := source
			if strings.Contains(cleanSourceName, ":") {
				parts := strings.Split(cleanSourceName, ":")
				if len(parts) == 2 {
					cleanSourceName = parts[0]
				}
			}

			if src, ok := config.Server.NameToSource[cleanSourceName]; ok {
				realSource = src.Name
				err = nil
			} else {
				// Case-insensitive fallback for Admin
				for name, src := range config.Server.NameToSource {
					if strings.EqualFold(name, cleanSourceName) {
						realSource = src.Name
						err = nil
						break
					}
				}
			}
		}
	}

	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source name: %s", source)
	}

	// Get the real source object to get the Path
	sourceObj, ok := config.Server.NameToSource[realSource]
	if !ok {
		return http.StatusInternalServerError, fmt.Errorf("configured source not found for: %s", realSource)
	}

	s, err := store.Share.Gets(path, sourceObj.Path, d.user.ID)
	if err != nil && err != errors.ErrNotExist {
		logger.Debugf("shareGetHandler: error getting shares for path %s: %v", path, err)
		return http.StatusInternalServerError, fmt.Errorf("error getting share info from server")
	}

	if s == nil {
		s = []*share.Link{}
	}

	// Also retrieve Service Shares (which are stored as Users)
	allUsers, err := store.Users.Gets()
	if err == nil {
		for _, u := range allUsers {
			if strings.HasPrefix(u.Username, "_svc_share_") && len(u.Scopes) == 1 {
				scope := u.Scopes[0]
				// Match scope name and path
				if scope.Name == sourceObj.Path && scope.Scope == path {
					// Convert User to a pseudo-Link for the frontend
					s = append(s, &share.Link{
						Hash:      u.Username, // Use username as hash for service shares
						Path:      path,
						Source:    source,
						UserID:    u.ID,
						Expire:    u.Expiration,
						IsService: true,
					})
				}
			}
		}
	}

	return renderJSON(w, r, s)
}

// shareDeleteHandler deletes a specific share link by its hash.
// @Summary Delete a share link
// @Description Deletes a share link specified by its hash.
// @Tags Shares
// @Accept json
// @Produce json
// @Param hash path string true "Hash of the share link to delete"
// @Success 200 "Share link deleted successfully"
// @Failure 400 {object} map[string]string "Bad request - missing or invalid hash"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/shares/{hash} [delete]
func shareDeleteHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	hash := r.URL.Query().Get("hash")

	if hash == "" {
		return http.StatusBadRequest, nil
	}

	// Check if it's a Service Share (by username prefix)
	if strings.HasPrefix(hash, "_svc_share_") {
		// Find the user and delete them
		user, err := store.Users.Get(hash)
		if err != nil {
			if err == errors.ErrNotExist {
				return http.StatusNotFound, nil
			}
			return http.StatusInternalServerError, err
		}
		err = store.Users.Delete(user.ID)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		return http.StatusOK, nil
	}

	err := store.Share.Delete(hash)
	if err != nil {
		return errToStatus(err), err
	}

	return errToStatus(err), err
}

// sharePostHandler creates a new share link.
// @Summary Create a share link
// @Description Creates a new share link with an optional expiration time and password protection.
// @Tags Shares
// @Accept json
// @Produce json
// @Param body body share.CreateBody true "Share link creation parameters"
// @Param path path string true "Source Path of the files to share"
// @Param source path string true "Source name of the files to share"
// @Success 200 {object} share.Link "Created share link"
// @Failure 400 {object} map[string]string "Bad request - failed to decode body"
// @Failure 500 {object} map[string]string "Internal server error"
// @Router /api/shares [post]
func sharePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	logger.Debugf("sharePostHandler: user=%s, path=%s, source=%s", d.user.Username, r.URL.Query().Get("path"), r.URL.Query().Get("source"))
	var s *share.Link
	var body share.CreateBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}

	secure_hash, err := generateShortUUID()
	if err != nil {
		return http.StatusInternalServerError, err
	}

	var expire int64 = 0

	if body.Expires != "" {
		//nolint:govet
		num, err := strconv.Atoi(body.Expires)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		var add time.Duration
		switch body.Unit {
		case "seconds":
			add = time.Second * time.Duration(num)
		case "minutes":
			add = time.Minute * time.Duration(num)
		case "days":
			add = time.Hour * 24 * time.Duration(num)
		default:
			add = time.Hour * time.Duration(num)
		}

		expire = time.Now().Add(add).Unix()
	}

	hash, status, err := getSharePasswordHash(body)
	if err != nil {
		return status, err
	}
	stringHash := ""
	var token string
	if len(hash) > 0 {
		tokenBuffer := make([]byte, 24) //nolint:gomnd
		if _, err = rand.Read(tokenBuffer); err != nil {
			return http.StatusInternalServerError, err
		}
		token = base64.URLEncoding.EncodeToString(tokenBuffer)
		stringHash = string(hash)
	}
	encodedPath := r.URL.Query().Get("path")
	// Decode the URL-encoded path
	path, err := url.QueryUnescape(encodedPath)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
	}
	sourceName := r.URL.Query().Get("source")
	if sourceName == "" {
		sourceName = config.Server.DefaultSource.Name
	} else {
		var err error
		// decode url encoded source name
		sourceName, err = url.QueryUnescape(sourceName)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid source encoding: %v", err)
		}
	}
	// Resolve the source name (which might be an alias or Name:Index) to the real source path/name
	scopePath, realSourceName, err := settings.GetScopeFromSourceString(d.user.Scopes, sourceName)
	if err != nil {
		// FALLBACK: If user is Admin, try to resolve directly from NameToSource
		// Admins generally have access to all sources even if not explicitly in Scopes
		if d.user.Perm.Admin {
			// Clean source name (remove :Index suffix if present)
			cleanSourceName := sourceName
			if strings.Contains(cleanSourceName, ":") {
				parts := strings.Split(cleanSourceName, ":")
				if len(parts) == 2 {
					cleanSourceName = parts[0]
				}
			}

			if src, ok := config.Server.NameToSource[cleanSourceName]; ok {
				scopePath = "/"
				realSourceName = src.Name
				err = nil
			} else {
				// Case-insensitive fallback for Admin
				for name, src := range config.Server.NameToSource {
					if strings.EqualFold(name, cleanSourceName) {
						scopePath = "/"
						realSourceName = src.Name
						err = nil
						break
					}
				}
			}
		}
	}

	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("invalid source: %v", err)
	}

	source, ok := config.Server.NameToSource[realSourceName]
	if !ok {
		return http.StatusBadRequest, fmt.Errorf("source configuration not found for: %s", realSourceName)
	}

	// If using an alias/scope, rewrite the path to be absolute within the source
	if scopePath != "/" {
		// Ensure scopePath has leading slash and no trailing slash
		if !strings.HasPrefix(scopePath, "/") {
			scopePath = "/" + scopePath
		}
		scopePath = strings.TrimSuffix(scopePath, "/")

		// Ensure path has leading slash
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		path = scopePath + path
	}

	s = &share.Link{
		Path:         path,
		Hash:         secure_hash,
		Source:       source.Path, // path instead to persist accoss name change
		Expire:       expire,
		UserID:       d.user.ID,
		PasswordHash: stringHash,
		Token:        token,
	}

	if err := store.Share.Save(s); err != nil {
		logger.Debugf("sharePostHandler: error saving share: %v", err)
		return http.StatusInternalServerError, err
	}

	logger.Debugf("sharePostHandler: share created successfully: hash=%s", s.Hash)

	return renderJSON(w, r, s)
}

// serviceSharePostHandler creates a temporary service account for full-app sharing.
func serviceSharePostHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.user.Permissions.ManageServiceShares && !d.user.Permissions.Admin {
		logger.Debugf("serviceSharePostHandler: forbidden for user %s (missing ManageServiceShares/Admin)", d.user.Username)
		return http.StatusForbidden, fmt.Errorf("you do not have permission to create service shares")
	}

	var body share.CreateBody
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return http.StatusBadRequest, fmt.Errorf("failed to decode body: %w", err)
		}
		defer r.Body.Close()
	}

	// 1. Generate random username with prefix
	shortID, err := generateShortUUID()
	if err != nil {
		return http.StatusInternalServerError, err
	}
	username := strings.ToLower("_svc_share_" + shortID[:12])
	admissions := d.user.Permissions
	admissions.Admin = false // Ensure service account is never admin
	admissions.ManageServiceShares = false
	admissions.Share = false
	admissions.UpdateMap = true // Allow map interactions if shared in map mode? Or just follow user perms.
	// Actually we should probably copy most perms but NOT admin/share

	// Let's use clean username
	username = users.CleanUsername(username)

	// 2. Resolve Path and Source
	encodedPath := r.URL.Query().Get("path")
	path, _ := url.QueryUnescape(encodedPath)
	sourceName := r.URL.Query().Get("source")
	if sourceName == "" {
		sourceName = config.Server.DefaultSource.Name
	}

	// Resolve the real path and source name using the current user's context
	// This handles aliases and virtual paths correctly.
	resolvedPath, realSourceName, err := ResolveScopePath(d.user, sourceName, path)
	if err != nil {
		return http.StatusForbidden, fmt.Errorf("failed to resolve path: %w", err)
	}
	path = resolvedPath

	// Resolve the source Name back to its real Path for the scope key
	sourceKey := realSourceName
	if src, ok := settings.Config.Server.NameToSource[realSourceName]; ok {
		sourceKey = src.Path
	}

	// 3. Calculate Expiration
	var expire int64 = 0
	if body.Expires != "" {
		num, _ := strconv.Atoi(body.Expires)
		var add time.Duration
		switch body.Unit {
		case "seconds":
			add = time.Second * time.Duration(num)
		case "minutes":
			add = time.Minute * time.Duration(num)
		case "days":
			add = time.Hour * 24 * time.Duration(num)
		default:
			add = time.Hour * time.Duration(num)
		}
		expire = time.Now().Add(add).Unix()
	}

	// 4. Create the Service User
	var scopes []users.SourceScope
	if settings.IsVirtualSource(sourceName) {
		// Inherit and Scope: Access the SAME shared path across all visible sources
		for _, s := range d.user.Scopes {
			// We skip restricted scopes that don't match the path if we wanted to be even more strict,
			// but usually "ALL" means "Show me this path everywhere it exists".
			scopes = append(scopes, users.SourceScope{
				Name:  s.Name,
				Scope: path,
				Alias: s.Alias,
			})
		}
		// If user is admin but has no explicit scopes (unlikely in this app),
		// we might need to add all configured sources.
		// But in this codebase, d.user.Scopes is usually fully populated.
	} else {
		scopes = []users.SourceScope{
			{
				Name:  sourceKey,
				Scope: path,
			},
		}
	}

	user := users.User{
		Username:    username,
		LoginMethod: users.LoginMethodPassword,
		Scopes:      scopes,
		Permissions: users.Permissions{
			Share: false, // Service accounts shouldn't create more shares
		},
		Expiration: expire,
	}

	if body.Password != "" {
		user.Password = body.Password
	} else {
		// Random password for "link-only" style (but still using JWT)
		user.Password, _ = generateShortUUID()
	}

	err = storage.CreateUser(user, false)
	if err != nil {
		if strings.Contains(err.Error(), "minimum") || strings.Contains(err.Error(), "exists") {
			return http.StatusBadRequest, err
		}
		return http.StatusInternalServerError, err
	}

	// 5. Generate long-lived JWT for this user
	// We use 10 years if no expiration, or the actual expiration
	tokenDuration := time.Hour * 24 * 365 * 10
	if expire > 0 {
		tokenDuration = time.Until(time.Unix(expire, 0))
	}

	// get the created user for ID
	createdUser, err := store.Users.Get(username)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to retrieve created user: %w", err)
	}
	signed, err := makeSignedTokenAPI(createdUser, "SERVICE_SHARE_"+shortID, tokenDuration, createdUser.Permissions)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return renderJSON(w, r, map[string]string{
		"username": username,
		"token":    signed.Key,
	})
}

func getSharePasswordHash(body share.CreateBody) (data []byte, statuscode int, err error) {
	if body.Password == "" {
		return nil, 0, nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, http.StatusInternalServerError, fmt.Errorf("failed to hash password")
	}

	return hash, 0, nil
}

func generateShortUUID() (string, error) {
	// Generate 16 random bytes (128 bits of entropy)
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	// Encode the bytes to a URL-safe base64 string
	uuid := base64.RawURLEncoding.EncodeToString(bytes)

	// Trim the length to 22 characters for a shorter ID
	return uuid[:22], nil
}
