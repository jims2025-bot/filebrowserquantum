package http

import (
	"net/url"
	"strings"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/utils"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

// ResolveScopePath determines the effective file path given a user's context and requested path.
// It robustly handles:
// 1. Path duplication (if path passed from frontend already starts with the user's scope).
// 2. Cross-Scope permissions (identifying if the path belongs to a sibling scope authorized for the user).
// 3. Fallback to standard joining if no matches found.
// ResolveScopePath determines the effective file path given a user's context and requested path.
// Returns:
// - scopePath: The fully resolved, absolute virtual path.
// - realSource: The actual source name (resolved from aliases).
// - error: Any error encountered.
func ResolveScopePath(user *users.User, source string, path string) (string, string, error) {
	// Decode source just in case it's double/triple encoded (e.g. PHOTOS%253A1 -> PHOTOS:1)
	for i := 0; i < 3; i++ {
		if strings.Contains(source, "%") {
			if decoded, err := url.QueryUnescape(source); err == nil {
				source = decoded
			}
		}
	}
	// ...

	// 1. Determine Initial Scope (Default Behavior)
	// This validates the source and gets the primary scope for this user/source context.
	userscope, realSource, err := settings.GetScopeFromSourceString(user.Scopes, source)
	if err != nil {
		return "", "", err
	}
	// Use the real source name derived from aliases
	source = realSource

	// Clean inputs for reliable string comparison
	cleanPath := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	cleanScope := strings.Trim(strings.ReplaceAll(userscope, "\\", "/"), "/")

	// 2. Check if Path already contains the Scope (Duplication Prevention)
	// If cleanScope is empty (Root Scope), it is implicitly a prefix of everything.
	pathContainsScope := false
	if cleanScope == "" {
		pathContainsScope = true
	} else {
		// Check prefix with case-insensitivity
		if len(cleanPath) >= len(cleanScope) && strings.EqualFold(cleanPath[:len(cleanScope)], cleanScope) {
			pathContainsScope = true
		}
	}

	if pathContainsScope {
		// The path is already "Absolute" relative to the Source Root. Use it as is.
		scopePath := path
		if !strings.HasPrefix(scopePath, "/") {
			scopePath = "/" + scopePath
		}
		// fmt.Printf("DEBUG: ResolveScopePath - Path Duplication Detected/Handled. Using %s\n", scopePath)
		return scopePath, source, nil
	}

	// 3. Cross-Scope Permission Check
	// If we are here, path does NOT start with the default userscope.
	// Example: User is Scoped to /POWELL, but Path is /SHANTON/Image.jpg
	// We check if it starts with ANY OTHER valid scope for this user + source.

	idxSource := indexing.GetIndex(source)
	rootPath := ""
	if idxSource != nil {
		rootPath = idxSource.Source.Path
	}

	// fmt.Printf("DEBUG: ResolveScopePath - Checking Cross-Scope. Source=%s Root=%s Path=%s\n", source, rootPath, cleanPath)

	for _, s := range user.Scopes {
		// Verify if this Scope belongs to the requested Source.
		// We match against the Source Name (e.g. "PHOTOS") OR the Source Index Root Path (e.g. "E:/PHOTOS")
		matchesSource := (s.Name == source || s.Alias == source)
		if !matchesSource && rootPath != "" {
			// Fallback: Check if Scope Name matches Source Root Path (common in this codebase)
			cleanedName := strings.Trim(strings.ReplaceAll(s.Name, "\\", "/"), "/")
			cleanedRoot := strings.Trim(strings.ReplaceAll(rootPath, "\\", "/"), "/")
			matchesSource = strings.EqualFold(cleanedName, cleanedRoot)
		}

		if matchesSource {
			cleanCandidate := strings.Trim(strings.ReplaceAll(s.Scope, "\\", "/"), "/")

			// Check if this authorized scope acts as a prefix for the requested path.
			// ALLOW root candidate (empty string) to match everything.
			if len(cleanPath) >= len(cleanCandidate) && strings.EqualFold(cleanPath[:len(cleanCandidate)], cleanCandidate) {
				// fmt.Printf("DEBUG: ResolveScopePath - Cross-Scope Match found! Switching to scope %s for path %s\n", s.Scope, path)
				scopePath := path
				if !strings.HasPrefix(scopePath, "/") {
					scopePath = "/" + scopePath
				}
				return scopePath, source, nil
			}
		}
	}

	// 4. Default Fallback: Join Scope + Path
	// If no duplication and no cross-scope match, assume the path is relative to the current scope.
	return utils.JoinPathAsUnix(userscope, path), source, nil
}
