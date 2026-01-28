package http

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Run: go test -v backend/http/simple_debug_test.go
func TestPathLogic(t *testing.T) {
	clusterPath := "/PHOTOCOLLECTIONS/POWELL-COLLECTION/SteveCarlaPowellCollection/L35-Powell/IMG_20250912_0005.jpg"
	targetSource := "PHOTOS"

	fmt.Printf("Input Path: %s\nTarget Source: %s\n", clusterPath, targetSource)

	// 1. Leaf Folder Calculation
	leafFolder := clusterPath
	if filepath.Ext(leafFolder) != "" {
		leafFolder = filepath.Dir(leafFolder)
	}
	leafFolder = filepath.ToSlash(leafFolder)

	fmt.Printf("Derived LeafFolder: %s\n", leafFolder)

	originalLeaf := leafFolder

	// 2. Smart Strip Logic
	prefix := "/" + targetSource
	prefixUpper := strings.ToUpper(prefix)
	leafUpper := strings.ToUpper(leafFolder)

	if strings.HasPrefix(leafUpper, prefixUpper+"/") || leafUpper == prefixUpper {
		leafFolder = leafFolder[len(prefix):]
		if leafFolder == "" {
			leafFolder = "/"
		}
		fmt.Printf("Smart Strip: '%s'\n", leafFolder)
	} else {
		fmt.Printf("Smart Strip Skipped (Prefix '%s' != Start of '%s')\n", prefixUpper, leafUpper)
	}

	// 3. Blind Strip Logic
	cleanLeaf := strings.Trim(strings.ReplaceAll(originalLeaf, "\\", "/"), "/")
	segments := strings.Split(cleanLeaf, "/")
	fmt.Printf("Segments: %v\n", segments)

	if len(segments) > 1 {
		blindPath := "/" + strings.Join(segments[1:], "/")
		fmt.Printf("Blind Strip Result: '%s'\n", blindPath)
	}
}
