package http

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jims2025-bot/filebrowserquantum/backend/common/settings"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

// To run: go test -v backend/http/debug_test.go
func TestHydrationDebug(t *testing.T) {
	// Mock Setup
	// We need to see what GetFolderHeatmap does with these inputs.

	// User Scenario from Logs
	sourceName := "PHOTOS"
	sourcePath := "E:/PERSONAL/PHOTOS"

	// Create mock index
	testSource := settings.Source{
		Name: sourceName,
		Path: sourcePath,
	}
	indexing.Initialize(testSource, true)

	// Inputs from user logs
	// Path has "PHOTOCOLLECTIONS" prefix but source is "PHOTOS"
	clusterPath := "/PHOTOCOLLECTIONS/POWELL-COLLECTION/SteveCarlaPowellCollection/L35-Powell/IMG_20250912_0005.jpg"
	targetSource := "PHOTOS" // from URL/Client

	fmt.Printf("\n--- Debug Scenario ---\nSource: %s\nPath: %s\n", targetSource, clusterPath)

	// 1. Leaf Folder Calculation
	leafFolder := clusterPath
	if filepath.Ext(leafFolder) != "" {
		leafFolder = filepath.Dir(leafFolder)
	}
	leafFolder = filepath.ToSlash(leafFolder)

	fmt.Printf("Initial LeafFolder: %s\n", leafFolder)

	// 2. Mock Logic from handleInspect
	// Smart Strip Logic
	prefix := "/" + targetSource
	prefixUpper := strings.ToUpper(prefix)
	leafUpper := strings.ToUpper(leafFolder)

	originalLeaf := leafFolder

	if strings.HasPrefix(leafUpper, prefixUpper+"/") || leafUpper == prefixUpper {
		leafFolder = leafFolder[len(prefix):]
		if leafFolder == "" {
			leafFolder = "/"
		}
		fmt.Printf("Smart Strip Applied: %s\n", leafFolder)
	} else {
		// Log why it didn't apply
		fmt.Printf("Smart Strip Skipped: Prefix '%s' not found in '%s'\n", prefixUpper, leafUpper)
	}

	// 3. Blind Strip Logic (Simulation)
	cleanLeaf := strings.Trim(strings.ReplaceAll(originalLeaf, "\\", "/"), "/")
	segments := strings.Split(cleanLeaf, "/")
	if len(segments) > 1 {
		blindPath := "/" + strings.Join(segments[1:], "/")
		fmt.Printf("Blind Strip Result: %s\n", blindPath)

		// TEST: what does GetRealPath do with this?
		idx := indexing.GetIndex(targetSource)
		realPath, _, err := idx.GetRealPath(blindPath)
		fmt.Printf("GetRealPath('%s') -> '%s' (Err: %v)\n", blindPath, realPath, err)
	}
}
