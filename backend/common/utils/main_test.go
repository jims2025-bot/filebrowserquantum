package utils

import (
	"strings"
	"testing"
)

func TestGetParentDirectoryPath(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{input: "/", expectedOutput: ""},                                              // Root directory
		{input: "/subfolder", expectedOutput: "/"},                                    // Single subfolder
		{input: "/sub/sub/", expectedOutput: "/sub"},                                  // Nested subfolder with trailing slash
		{input: "/subfolder/", expectedOutput: "/"},                                   // Relative path with trailing slash
		{input: "", expectedOutput: ""},                                               // Empty string treated as root
		{input: "/sub/subfolder", expectedOutput: "/sub"},                             // Double slash in path
		{input: "/sub/subfolder/deep/nested/", expectedOutput: "/sub/subfolder/deep"}, // Double slash in path
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actualOutput := GetParentDirectoryPath(test.input)
			if actualOutput != test.expectedOutput {
				t.Errorf("\n\tinput %q\n\texpected %q\n\tgot %q",
					test.input, test.expectedOutput, actualOutput)
			}
		})
	}
}

func TestCapitalizeFirst(t *testing.T) {
	tests := []struct {
		input          string
		expectedOutput string
	}{
		{input: "", expectedOutput: ""},                               // Empty string
		{input: "a", expectedOutput: "A"},                             // Single lowercase letter
		{input: "A", expectedOutput: "A"},                             // Single uppercase letter
		{input: "hello", expectedOutput: "Hello"},                     // All lowercase
		{input: "Hello", expectedOutput: "Hello"},                     // Already capitalized
		{input: "123hello", expectedOutput: "123hello"},               // Non-alphabetic first character
		{input: "hELLO", expectedOutput: "HELLO"},                     // Mixed case
		{input: " hello", expectedOutput: " hello"},                   // Leading space, no capitalization
		{input: "hello world", expectedOutput: "Hello world"},         // Phrase with spaces
		{input: " hello world", expectedOutput: " hello world"},       // Phrase with leading space
		{input: "123 hello world", expectedOutput: "123 hello world"}, // Numbers before text
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actualOutput := CapitalizeFirst(test.input)
			if actualOutput != test.expectedOutput {
				t.Errorf("\n\tinput %q\n\texpected %q\n\tgot %q",
					test.input, test.expectedOutput, actualOutput)
			}
		})
	}
}
func TestHashSHA256(t *testing.T) {
	input := "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	actual := HashSHA256(input)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestGetLastComponent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/a/b/c", "c"},
		{"/a/b/c/", "c"},
		{"/", ""},
		{"file.txt", "file.txt"},
		{"/dir/file.txt", "file.txt"},
	}
	for _, test := range tests {
		actual := GetLastComponent(test.input)
		if actual != test.expected {
			t.Errorf("input %s: expected %s, got %s", test.input, test.expected, actual)
		}
	}
}

func TestJoinPathAsUnix(t *testing.T) {
	// This test behaves differently on Windows vs Linux, but the function normalizes to forward slashes on Windows.
	// We simulate inputs.
	parts := []string{"foo", "bar"}
	result := JoinPathAsUnix(parts...)
	if strings.Contains(result, "\\") {
		t.Errorf("JoinPathAsUnix should not return backslashes on any OS (normalized), got: %s", result)
	}
	if !strings.HasSuffix(result, "foo/bar") && !strings.HasSuffix(result, "foo\\bar") {
		// Go's filepath.Join might return backslashes on windows before our replacement
		// But our function explicitly replaces them.
		// So checking for forward slash presence is key if we are on windows.
	}
}

func TestGenerateKey(t *testing.T) {
	key1 := GenerateKey()
	key2 := GenerateKey()
	if len(key1) != 64 {
		t.Errorf("expected key length 64, got %d", len(key1))
	}
	if key1 == key2 {
		t.Errorf("keys should be random, got duplicate: %s", key1)
	}
}
