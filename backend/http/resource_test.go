package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/users"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
	"github.com/stretchr/testify/assert"
)

func TestResourceDeleteHandler_Exif(t *testing.T) {
	// 1. Setup Temp Dir and Mock File
	tempDir := t.TempDir()
	// Create a dummy file
	// We don't care about content, just availability for FileInfoFaster
	// Note: index might need to scan it, but we can bypass that if we rely on simple GetRealPath logic
	// provided by GetIndex(source) + FileInfoFaster internals.

	// 2. Setup Mock Settings
	// Ensure we reset settings or use a clean state if possible, but for this test we hack globals.
	testSource := settings.Source{
		Name: "test",
		Path: tempDir,
	}
	settings.Config.Server.NameToSource = map[string]settings.Source{
		"test": testSource,
	}
	settings.Config.Server.SourceMap = map[string]settings.Source{
		tempDir: testSource,
	}

	// 3. Initialize Indexing (Required for files.FileInfoFaster)
	// We need to make sure the index exists so GetIndex returns non-nil.
	indexing.Initialize(testSource, true)

	fmt.Printf("DEBUG: settings.Config.Server.NameToSource: %+v\n", settings.Config.Server.NameToSource)

	// Also explicitly set the config pointer for http package if it's nil, just in case
	if config == nil {
		config = &settings.Config
	}

	// Setup mock user with Modify permissions
	user := &users.User{
		Permissions: users.Permissions{
			Modify: true,
		},
		Scopes: []users.SourceScope{
			{
				Name:  "test",
				Scope: "/",
			},
		},
	}

	// Setup request context
	d := &requestContext{
		user: user,
	}

	// Case 1: action=exif (Allowed)
	// We expect this to execute. It will likely fail at RemoveExif because exiftool is missing
	// or file doesn't match, or fail at FileInfoFaster if not fully mocked.
	// KEY: We just want to ensure it is NOT 403.
	req := httptest.NewRequest("DELETE", "/api/resources?action=exif&path=/test.jpg&source=test", nil)
	w := httptest.NewRecorder()

	status, _ := resourceDeleteHandler(w, req, d)

	assert.NotEqual(t, http.StatusForbidden, status, "action=exif should not be forbidden. Got status: %d", status)

	// Case 2: No action (standard delete) (Forbidden)
	req2 := httptest.NewRequest("DELETE", "/api/resources?path=/test.jpg&source=test", nil)
	w2 := httptest.NewRecorder()

	status2, _ := resourceDeleteHandler(w2, req2, d)
	assert.Equal(t, http.StatusForbidden, status2, "Standard delete should be forbidden")
}
