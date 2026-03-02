package http

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
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

	// Handle "ALL" or "ALL_SOURCES" pseudo-source for global aggregation
	if settings.IsVirtualSource(source) {
		// If we are looking for a specific path but using a virtual source,
		// we should try to resolve the actual physical source from the path segments.
		if realSrc, relPath, found := settings.GetSourceFromPath(path); found {
			source = realSrc
			path = relPath
			// Continue to standard resolution with the physical source to apply user scopes
		} else if source == "ALL" {
			// Only return the "ALL" pseudo-source if explicitly requested.
			return path, "ALL", nil
		}
	}

	// 1. Determine Initial Scope (Default Behavior)
	// If source is empty, try to find a default.
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	}

	userscope, realSource, err := settings.GetScopeFromSourceString(user.Scopes, source)
	if err != nil {
		// Fallback: If the user doesn't have the default source, use the first available scope.
		// This prevents 403 errors when the frontend initializes with an empty source.
		if len(user.Scopes) > 0 {
			firstScope := user.Scopes[0]
			// We try to resolve the real name if it's an alias
			if firstScope.Alias != "" {
				return ResolveScopePath(user, firstScope.Alias, path)
			}
			// Otherwise resolve by Name (which is Source Path)
			for name, src := range settings.Config.Server.NameToSource {
				if src.Path == firstScope.Name {
					return ResolveScopePath(user, name, path)
				}
			}
			// Absolute fallback
			return firstScope.Scope, firstScope.Name, nil
		}
		return "", "", err
	}

	// FIX: If the path starts with the Source Alias (e.g. /SHANTON-VA/Subfolder),
	// and the mapped Scope is /PHOTOCOLLECTIONS/SHANTONVA-COLLECTION,
	// we should strip the alias from the path to avoid duplication like:
	// /PHOTOCOLLECTIONS/SHANTONVA-COLLECTION/SHANTON-VA/Subfolder.
	if source != "" {
		cleanedPath := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
		cleanedSource := strings.Trim(strings.ReplaceAll(source, "\\", "/"), "/")

		if strings.HasPrefix(strings.ToLower(cleanedPath), strings.ToLower(cleanedSource)+"/") {
			path = cleanedPath[len(cleanedSource):]
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			logger.Debug(fmt.Sprintf("ResolveScopePath: Stripped source alias '%s' from path. New path: '%s'", source, path))
		} else if strings.EqualFold(cleanedPath, cleanedSource) {
			path = "/"
			logger.Debug(fmt.Sprintf("ResolveScopePath: Path equals source alias '%s'. New path: '/'", source))
		}
	}

	// FIX: If User is Admin, force Scope to Root "/" to avoid path conflicts.
	// FIX: If the input path is already Absolute (e.g. from Heatmap), use it directly.
	// Otherwise, join with the User Scope (e.g. from File Browser).
	// This prevents "Double Joining" (Scope + AbsPath) while allowing Relative paths to work.
	var joinedPath string

	// Check for Windows Absolute path (Drive Letter) or Unix Absolute
	isAbs := filepath.IsAbs(path)
	if !isAbs && runtime.GOOS == "windows" {
		// filepath.IsAbs on windows requires Drive Letter.
		if len(path) > 2 && path[1] == ':' {
			isAbs = true
		}
	}

	// FIX: For Admins, we also treat paths starting with "/" or "\\" as Absolute.
	// This handles Abstract File System paths (e.g. /Collection/File.jpg) passed from Heatmap
	// without joining them to the Admin's default scope (which would cause duplication like /Collection/Collection/File.jpg).
	// Normal users are restricted so we typically force join unless it's a Drive Letter (system absolute).
	if user.Permissions.Admin && (strings.HasPrefix(path, "/") || strings.HasPrefix(path, "\\")) {
		isAbs = true
	}

	// CRITICAL FIX: Check if path already contains the scope BEFORE joining
	// This prevents duplication when heatmap returns paths like:
	// "/BillPowellCollection/A01-BillPowell-1995/PHOTOCOLLECTIONS/POWELL-COLLECTION/..."
	// where userscope is "/BillPowellCollection/A01-BillPowell-1995"
	pathContainsScope := false
	cleanPath := strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	cleanScope := strings.Trim(strings.ReplaceAll(userscope, "\\", "/"), "/")

	if cleanScope != "" {
		// Check if path contains scope as a substring (case-insensitive)
		pathLower := strings.ToLower(cleanPath)
		scopeLower := strings.ToLower(cleanScope)

		// Check for exact prefix match first
		if strings.HasPrefix(pathLower, scopeLower) {
			pathContainsScope = true
			logger.Debug(fmt.Sprintf("ResolveScopePath: Path '%s' already starts with scope '%s'", path, userscope))
		} else if idx := strings.Index(pathLower, scopeLower); idx != -1 {
			// Path contains scope somewhere in the middle
			// Verify it's a valid segment match (surrounded by slashes or at boundaries)
			endIdx := idx + len(scopeLower)
			validStart := (idx == 0 || pathLower[idx-1] == '/')
			validEnd := (endIdx == len(pathLower) || pathLower[endIdx] == '/')

			if validStart && validEnd {
				pathContainsScope = true
				logger.Debug(fmt.Sprintf("ResolveScopePath: Path '%s' contains scope '%s' at position %d", path, userscope, idx))
			}
		}
	} else {
		// Empty scope (root) - all paths implicitly contain it
		pathContainsScope = true
	}

	if isAbs {
		joinedPath = filepath.Clean(path)
		logger.Debug(fmt.Sprintf("ResolveScopePath: Treating as absolute path: '%s'", joinedPath))
	} else if pathContainsScope {
		// Path already contains the scope - don't join, use as-is
		joinedPath = filepath.ToSlash(filepath.Clean(path))
		logger.Debug(fmt.Sprintf("ResolveScopePath: Path contains scope, using as-is: '%s'", joinedPath))
	} else {
		// Relative path: Join with Scope
		joinedPath = filepath.ToSlash(filepath.Join(userscope, path))
		logger.Debug(fmt.Sprintf("ResolveScopePath: Joining scope '%s' + path '%s' = '%s'", userscope, path, joinedPath))
	}

	// Use the real source name derived from aliases
	source = realSource

	// Clean inputs for reliable string comparison (for later checks)
	cleanPath = strings.Trim(strings.ReplaceAll(path, "\\", "/"), "/")
	cleanScope = strings.Trim(strings.ReplaceAll(userscope, "\\", "/"), "/")

	// 2. Check if Path already contains the Scope (Duplication Prevention)
	// If cleanScope is empty (Root Scope), it is implicitly a prefix of everything.
	pathContainsScope = false
	if cleanScope == "" {
		pathContainsScope = true
	} else {
		// Log inputs for debugging
		logger.Debug(fmt.Sprintf("Resolve: Path='%s' Scope='%s'", cleanPath, cleanScope))

		// Check prefix with case-insensitivity
		if len(cleanPath) >= len(cleanScope) && strings.EqualFold(cleanPath[:len(cleanScope)], cleanScope) {
			pathContainsScope = true
			logger.Debug("Resolve: Prefix Match Found")
		} else {
			// Aggressive Substring Check
			// ...

			// Try finding scope in path
			searchScope := "/" + cleanScope
			idx := strings.Index(strings.ToLower(cleanPath), strings.ToLower(searchScope))
			if idx != -1 {
				// Verify it's a segment match (next char is / or end of string)
				endIdx := idx + len(searchScope)
				if endIdx == len(cleanPath) || cleanPath[endIdx] == '/' {
					pathContainsScope = true
					logger.Debug("Resolve: Substring Match Found")
				}
			}
		}
	}

	if pathContainsScope {
		// The path is already "Absolute" relative to the Source Root (or contains the scope chain). Use it as is.
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
