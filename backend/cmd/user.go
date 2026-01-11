package cmd

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/adapters/fs/fileutils"
	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
)

var createBackup = []bool{}

func validateUserInfo() {
	// update source info for users if names/sources/paths might have changed
	usersList, err := store.Users.Gets()
	if err != nil {
		logger.Fatalf("could not load users: %v", err)
	}
	for _, user := range usersList {
		updateUser := false
		if user.Username == "publicUser" {
			settings.ApplyUserDefaults(user)
			updateUser = true
		}
		if updateUserScopes(user) {
			updateUser = true
		}
		if updatePermissions(user) {
			updateUser = true
		}
		if updatePreviewSettings(user) {
			updateUser = true
		}
		if updateLoginType(user) {
			updateUser = true
		}
		if updateUser {
			if len(createBackup) == 1 {
				logger.Warning("Incompatible user settings detected, creating backup of database before converting.")
				err = fileutils.CopyFile(settings.Config.Server.Database, fmt.Sprintf("%s.bak", settings.Config.Server.Database))
				if err != nil {
					logger.Fatalf("Unable to create automatic backup of database due to error: %v", err)
				}
			}
			err := store.Users.Save(user, false, true)
			if err != nil {
				logger.Errorf("could not update user: %v", err)
			}
		}
	}
	if settings.Config.Auth.ResetAdminOnStart {
		logger.Info("Resetting admin user to default username and password.")
		adminUser, err := store.Users.Get(1)
		if err != nil {
			logger.Fatalf("could not load admin user: %v", err)
		}
		adminUser.Username = settings.Config.Auth.AdminUsername
		adminUser.Password = settings.Config.Auth.AdminPassword
		adminUser.Permissions.Admin = true
		err = store.Users.Save(adminUser, true, true)
		if err != nil {
			logger.Errorf("could not Save admin user: %v", err)
		}
	}
}

func updateUserScopes(user *users.User) bool {
	newScopes := []users.SourceScope{}
	seenSources := make(map[string]bool)

	// 1. Process existing user scopes (Preserve Alias, fix Casing)
	for _, s := range user.Scopes {
		canonicalName := s.Name

		// Try to match against live config to fix casing
		for path := range settings.Config.Server.SourceMap {
			if strings.EqualFold(s.Name, path) {
				canonicalName = path
				break
			}
		}

		newScopes = append(newScopes, users.SourceScope{
			Name:  canonicalName,
			Scope: s.Scope,
			Alias: s.Alias, // IMPORTANT: Preserve Alias
		})
		// Mark as seen using upper case for case-insensitive check later
		seenSources[strings.ToUpper(canonicalName)] = true
	}

	// 2. Add Default-Enabled sources if not present
	for _, src := range settings.Config.Server.Sources {
		realsource, ok := settings.Config.Server.NameToSource[src.Name]
		if !ok {
			continue
		}

		// Check if we already have this source (Case-Insensitive check via Upper Key)
		if !seenSources[strings.ToUpper(realsource.Path)] && realsource.Config.DefaultEnabled {
			// Add default
			newScopes = append(newScopes, users.SourceScope{
				Name:  realsource.Path,
				Scope: realsource.Config.DefaultUserScope,
				// Alias left empty for auto-added defaults
			})
		}
	}

	changed := !reflect.DeepEqual(user.Scopes, newScopes)
	user.Scopes = newScopes
	return changed
}

// func to convert legacy user with perm key to permissions
func updatePermissions(user *users.User) bool {
	updateUser := false
	// if any keys are true, set the permissions to true
	if user.Perm.Api {
		user.Permissions.Api = true
		user.Perm.Api = false
		updateUser = true
	}
	if user.Perm.Admin {
		user.Permissions.Admin = true
		user.Perm.Admin = false
		updateUser = true
	}
	if user.Perm.Modify {
		user.Permissions.Modify = true
		user.Perm.Modify = false
		updateUser = true
	}
	if user.Perm.Share {
		user.Permissions.Share = true
		user.Perm.Share = false
		updateUser = true
	}
	if updateUser {
		createBackup = append(createBackup, true)
	}
	return updateUser
}

func updateLoginType(user *users.User) bool {
	if user.LoginMethod == "" {
		user.LoginMethod = users.LoginMethodPassword
		return true
	}
	return false
}

func updatePreviewSettings(user *users.User) bool {
	// if user hasn't been updated yet
	if user.LoginMethod == "" {
		user.Preview.Image = true
		user.Preview.PopUp = true
		return true
	}
	return false
}
