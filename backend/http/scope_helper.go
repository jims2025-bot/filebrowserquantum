package http

import (
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/utils"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

// GetBestScope finds the most appropriate scope for a user given a source and a requested path.
// It prioritizes scopes that are prefixes of the path.
func GetBestScope(user *users.User, source string, path string) (string, string, error) {
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	normPath := strings.ToLower(cleanPath)

	// 1. Resolve 'realSource' first
	_, realSource, err := settings.GetScopeFromSourceString(user.Scopes, source)
	if err != nil {
		// Fallback: If the user doesn't have the explicit source, use the first available scope as a hint.
		if len(user.Scopes) > 0 {
			realSource = user.Scopes[0].Name
		} else {
			return "", "", err
		}
	}

	idxSource := indexing.GetIndex(realSource)
	sourceDiskPath := ""
	if idxSource != nil {
		sourceDiskPath = idxSource.Source.Path
	}

	bestScope := ""
	firstMatch := ""

	// 2. Iterate all user scopes to find the longest matching prefix
	for _, s := range user.Scopes {
		// Is this scope for our target source? (Match by Name, Alias, or physical Path)
		matchesSource := (s.Name == realSource || s.Alias == source || (sourceDiskPath != "" && strings.EqualFold(s.Name, sourceDiskPath)))
		if !matchesSource {
			continue
		}

		candScope := filepath.ToSlash(filepath.Clean(s.Scope))
		if candScope == "" || candScope == "." {
			candScope = "/"
		}

		if firstMatch == "" {
			firstMatch = candScope
		}

		// Check if the path ALREADY starts with this scope (case-insensitive)
		normCand := strings.ToLower(candScope)
		if normCand == "/" || strings.HasPrefix(normPath, normCand+"/") || normPath == normCand {
			// Longest match wins
			if len(candScope) > len(bestScope) {
				bestScope = candScope
			}
		}
	}

	if bestScope != "" {
		return bestScope, realSource, nil
	}
	if firstMatch != "" {
		return firstMatch, realSource, nil
	}

	return "", "", err
}

func ResolveScopePath(user *users.User, source string, path string) (string, string, error) {
	// 1. Double/Triple decode source name (e.g. PHOTOS%253A1 -> PHOTOS:1)
	for i := 0; i < 3; i++ {
		if strings.Contains(source, "%") {
			if decoded, err := url.QueryUnescape(source); err == nil {
				source = decoded
			}
		}
	}

	// 2. Handle Virtual Aggregator Sources (ALL, ALL_SOURCES)
	// Only enter this block if the source isn't explicitly a physical source or alias in the config.
	_, _, errKnown := settings.GetScopeFromSourceString(user.Scopes, source)
	if settings.IsVirtualSource(source) && errKnown != nil {
		// ... (virtual source logic remains same)
		// A. Try resolving via source name prefix (e.g. /PHOTOS/folder)
		if realSrc, relPath, found := settings.GetSourceFromPath(path); found {
			source = realSrc
			path = relPath
			// Continue to standard resolution with the physical source
		} else {
			// B. Try matching against ALL user scopes to find a physical home
			cleanPath := filepath.ToSlash(filepath.Clean(path))
			for _, s := range user.Scopes {
				candScope := filepath.ToSlash(filepath.Clean(s.Scope))
				if candScope == "" || candScope == "." || candScope == "/" {
					continue
				}
				normPath := strings.ToLower(cleanPath)
				normCand := strings.ToLower(candScope)
				if strings.HasPrefix(normPath, normCand+"/") || normPath == normCand {
					// Found a matching scope! Use its real source.
					_, realSrc, _ := settings.GetScopeFromSourceString(user.Scopes, s.Name)
					return cleanPath, realSrc, nil // Return absolute path + real source
				}
			}

			// Virtual root or unresolvable path: Return pseudo-source
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			return path, source, nil
		}
	}

	// 3. Determine Initial Scope & Real Source Name
	if source == "" {
		source = settings.Config.Server.DefaultSource.Name
	}

	userScope, realSource, err := GetBestScope(user, source, path)
	if err != nil {
		return "", "", err
	}

	// 4. Input Path Sanitization
	// Strip source alias if explicitly included in path (e.g. /PHOTOS/2012 -> /2012)
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	cleanSource := strings.Trim(strings.ReplaceAll(source, "\\", "/"), "/")
	if strings.HasPrefix(strings.ToLower(cleanPath), "/"+strings.ToLower(cleanSource)+"/") {
		cleanPath = cleanPath[len(cleanSource)+1:]
	} else if strings.EqualFold(strings.Trim(cleanPath, "/"), cleanSource) {
		cleanPath = "/"
	}

	// 5. Handle Absolute Paths vs Root-Relative paths
	// If path starts with / and we're Admin, or if it explicitly includes the scope,
	// we avoid re-joining to prevent duplication.
	// SECURITY FIX: Restricted absolute path resolution to Administrators. 
	// In Linux, filepath.IsAbs("/") is true, which was allowing scoped users to reach the root.
	isExplicitAbsolute := user.Permissions.Admin && (filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, "/"))

	cleanScope := filepath.ToSlash(filepath.Clean(userScope))
	if cleanScope == "" || cleanScope == "." {
		cleanScope = "/"
	}

	// Check if the path already starts with the intended scope (case-insensitive)
	pathStartsWithScope := false
	if cleanScope != "/" {
		normPath := strings.ToLower(cleanPath)
		normScope := strings.ToLower(cleanScope)
		if strings.HasPrefix(normPath, normScope+"/") || normPath == normScope {
			pathStartsWithScope = true
		}
	} else {
		pathStartsWithScope = true
	}

	var resolvedPath string
	if isExplicitAbsolute || pathStartsWithScope {
		resolvedPath = cleanPath
	} else {
		// Cross-Scope Permission Check:
		// If path doesn't match default scope, check if it matches ANY authorized scope.
		idxSource := indexing.GetIndex(realSource)
		foundOtherScope := false
		if idxSource != nil {
			sourceDiskPath := idxSource.Source.Path
			for _, s := range user.Scopes {
				// Match scope to this specific source (by Name/Path or Alias)
				matchesSource := (s.Name == realSource || s.Alias == source || strings.EqualFold(s.Name, sourceDiskPath))
				if matchesSource {
					candScope := filepath.ToSlash(filepath.Clean(s.Scope))
					if candScope == "" || candScope == "." {
						candScope = "/"
					}
					normPath := strings.ToLower(cleanPath)
					normCand := strings.ToLower(candScope)
					if strings.HasPrefix(normPath, normCand+"/") || normPath == normCand {
						foundOtherScope = true
						break
					}
				}
			}
		}

		if foundOtherScope {
			resolvedPath = cleanPath
		} else {
			// Default Fallback: Assume path is relative to the user's default scope
			resolvedPath = utils.JoinPathAsUnix(cleanScope, cleanPath)
		}
	}

	// Ensure leading slash
	if !strings.HasPrefix(resolvedPath, "/") {
		resolvedPath = "/" + resolvedPath
	}

	return resolvedPath, realSource, nil
}
