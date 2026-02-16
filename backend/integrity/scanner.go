package integrity

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/jims2025-bot/filebrowserquantum/backend/database/storage"
	"github.com/jims2025-bot/filebrowserquantum/backend/indexing"
)

const (
	IssueFilename = "exif_issues.json"
)

// ExifResult matches the JSON output from ExifTool
type ExifResult struct {
	SourceFile string  `json:"SourceFile"`
	Error      string  `json:"Error,omitempty"`
	Warning    string  `json:"Warning,omitempty"`
	FileSize   float64 `json:"FileSize,omitempty"` // with -n flag
	Severity   string  `json:"Severity,omitempty"` // "minor" or "critical"
}

// IssueReport is the structure we write to exif_issues.json
type IssueReport struct {
	RunDate  string       `json:"run_date"`
	Problems []ExifResult `json:"problems,omitempty"`
	Message  string       `json:"message,omitempty"`
}

// RunScan triggers the integrity scan for all user scopes.
// It is designed to be called sequentially after the heatmap scan.
func RunScan(store *storage.Storage) {
	logger.Info("File Integrity Scan started")

	allIndexes := indexing.GetIndexes()
	pathToSource := make(map[string]string)
	for name, idx := range allIndexes {
		pathToSource[filepath.ToSlash(filepath.Clean(idx.Source.Path))] = name
	}

	users, err := store.Users.Gets()
	if err != nil {
		logger.Error("Integrity Scanner: Failed to get users: " + err.Error())
		return
	}

	// Deduplicate locations
	locationsToScan := make(map[string]map[string]struct{})

	for _, u := range users {
		if len(u.Scopes) == 0 {
			continue
		}

		// Match Heatmap logic: Skip admins to prevent scanning the entire root/drive if they have broad scopes.
		if u.Permissions.Admin {
			continue
		}

		for _, s := range u.Scopes {
			var properSourceName string
			if _, ok := allIndexes[s.Name]; ok {
				properSourceName = s.Name
			} else if parts := strings.Split(s.Name, ":"); len(parts) > 0 {
				if _, ok := allIndexes[parts[0]]; ok {
					properSourceName = parts[0]
				}
			}

			if properSourceName == "" {
				clean := filepath.ToSlash(filepath.Clean(s.Name))
				if name, ok := pathToSource[clean]; ok {
					properSourceName = name
				}
			}

			if properSourceName != "" {
				if locationsToScan[properSourceName] == nil {
					locationsToScan[properSourceName] = make(map[string]struct{})
				}
				scopePath := s.Scope
				if scopePath == "" {
					scopePath = "/"
				}
				locationsToScan[properSourceName][scopePath] = struct{}{}
			}
		}
	}

	for sourceName, paths := range locationsToScan {
		for path := range paths {
			// Initialize Issue Collection for this Scope
			var problemFolders []string
			var mu sync.Mutex

			// Callback to add a problem folder
			recordIssue := func(folderPath string) {
				mu.Lock()
				defer mu.Unlock()
				problemFolders = append(problemFolders, folderPath)
			}

			// Initialize ProbFolders.json (Empty)
			idx := indexing.GetIndex(sourceName)
			if idx != nil {
				realRoot, _, err := idx.GetRealPath(path)
				if err == nil {
					probFile := filepath.Join(realRoot, "ProbFolders.json")
					empty := []string{}
					bytes, _ := json.MarshalIndent(empty, "", "  ")
					_ = os.WriteFile(probFile, bytes, 0644)
				}
			}

			// Scan
			ScanRecursive(sourceName, path, recordIssue)

			// Write Final ProbFolders.json
			if idx != nil && len(problemFolders) > 0 {
				realRoot, _, err := idx.GetRealPath(path)
				if err == nil {
					probFile := filepath.Join(realRoot, "ProbFolders.json")
					bytes, _ := json.MarshalIndent(problemFolders, "", "  ")
					_ = os.WriteFile(probFile, bytes, 0644)
					logger.Info(fmt.Sprintf("Integrity Scanner: Wrote %d identified problematic folders to %s", len(problemFolders), probFile))
				}
			}
		}
	}

	logger.Info("Integrity Scan Complete")
}

func ScanRecursive(sourceName, rootPath string, recordIssue func(string)) {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		return
	}

	// 1. Scan this folder
	ScanFolder(idx, rootPath, recordIssue)

	// 2. Recurse
	dirInfo, exists := idx.GetReducedMetadata(rootPath, true)
	if exists {
		var wg sync.WaitGroup
		// Limit concurrency for recursion to avoid system overload
		sem := make(chan struct{}, 5)

		for _, sub := range dirInfo.Folders {
			wg.Add(1)
			sem <- struct{}{} // Acquire
			go func(subName string) {
				defer wg.Done()
				defer func() { <-sem }() // Release
				nextPath := filepath.ToSlash(filepath.Join(rootPath, subName))
				ScanRecursive(sourceName, nextPath, recordIssue)
			}(sub.Name)
		}
		wg.Wait()
	}
}

// classifySeverity determines if an issue is minor or critical
func classifySeverity(res ExifResult) string {
	errorText := strings.ToLower(res.Error + " " + res.Warning)

	// Critical errors
	criticalPatterns := []string{
		"jpeg format error",
		"eoi marker",
		"truncated",
		"corrupted",
		"invalid jpeg",
	}

	for _, pattern := range criticalPatterns {
		if strings.Contains(errorText, pattern) {
			return "critical"
		}
	}

	// Check for file size issue (< 20KB is critical)
	if res.FileSize > 0 && res.FileSize < 20000 {
		return "critical"
	}

	// Minor errors (metadata issues)
	minorPatterns := []string{
		"non-standard format",
		"missing required",
		"unrecognized makernotes",
	}

	for _, pattern := range minorPatterns {
		if strings.Contains(errorText, pattern) {
			return "minor"
		}
	}

	// Default to critical for unknown errors (safer to flag)
	return "critical"
}

func ScanFolder(idx *indexing.Index, virtualPath string, recordIssue func(string)) {
	realPath, _, err := idx.GetRealPath(virtualPath)
	if err != nil {
		return
	}

	// Requirement: Output server logs that indicate: Scanning folder ( folder name )
	// User request: Lets remove the line from the log that says 'Scanning folder'
	// logger.Info(fmt.Sprintf("Scanning folder ( %s )", virtualPath))

	// Construct ExifTool command
	cmd := exec.Command("exiftool",
		"-ext", "jpg",
		"-ext", "jpeg",
		"-validate", // Perform deep validation of file structure (detects missing EOI, etc.)
		"-warning", "-error",
		"-ExifVersion",
		"-FileSize",
		"-n", // numeric output
		"-json",
		".", // Current directory
	)
	cmd.Dir = realPath

	// CombinedOutput captures both stdout and stderr.
	// ExifTool with -json writes JSON to stdout and summary/errors to stderr.
	// If we combine them, the JSON is corrupted by the summary text.
	// We must separate them.

	// cmd.Output() returns Standard Output.
	// If the command exits with non-zero (which ExifTool does if it finds warnings), it returns an ExitError.
	// The ExitError contains the Stderr.
	output, err := cmd.Output()

	// Handle execution errors (non-zero exit code)
	if err != nil {
		// If it's an ExitError, we can still parse the stdout if available?
		// ExifTool usually writes valid JSON to stdout even if it exits with 1 due to minor warnings.
		// So we should try to proceed with 'output' even if err is not nil.

		if exitErr, ok := err.(*exec.ExitError); ok {
			// Log stderr for debugging, but don't fail yet if we have output
			stderr := string(exitErr.Stderr)
			if len(stderr) > 0 {
				logger.Debug(fmt.Sprintf("Integrity Scanner: ExifTool Stderr in %s: %s", virtualPath, stderr))
			}
		} else {
			// Serious execution error (not found, etc)
			logger.Error(fmt.Sprintf("Integrity Scanner: ExifTool Execution Error in %s: %v", virtualPath, err))
			return
		}
	}

	fileCount := 0
	issueCount := 0
	var badFiles []string

	// Parse Output if any
	var results []ExifResult
	if len(output) > 0 {
		if jsonErr := json.Unmarshal(output, &results); jsonErr == nil {
			fileCount = len(results)
		} else {
			// Debug logging for JSON error
			if len(output) > 200 {
				logger.Error(fmt.Sprintf("Integrity Scanner: JSON Parse Error in %s: %v. Output start: %.100s", virtualPath, jsonErr, string(output)))
			} else {
				logger.Error(fmt.Sprintf("Integrity Scanner: JSON Parse Error in %s: %v. Output: %s", virtualPath, jsonErr, string(output)))
			}
		}
	}

	// Filter Problems
	var problems []ExifResult
	for _, res := range results {
		isProblem := false

		// Check 1: Corruption
		if res.Error != "" || res.Warning != "" {
			// Ignore known minor warnings
			if strings.Contains(res.Warning, "Unrecognized MakerNotes") ||
				strings.Contains(res.Error, "Unrecognized MakerNotes") ||
				strings.Contains(res.Warning, "[minor]") ||
				strings.Contains(res.Error, "[minor]") {
				// Skip if it's just this
				if res.Error == "" && strings.Contains(res.Warning, "Unrecognized MakerNotes") {
					// Only warning is this, so ignore
				} else {
					// It might have other errors?
					// For now, if it contains this string, we consider it safe per user request for these specific errors.
					// But if there is ALSO a "Corrupted" error, we might miss it if we just disable isProblem.
					// Let's count it only if it's NOT just the ignored one.

					// Re-evaluate without ignored strings
					cleanWarn := res.Warning
					cleanWarn = strings.ReplaceAll(cleanWarn, "Unrecognized MakerNotes", "")
					cleanWarn = strings.ReplaceAll(cleanWarn, "[minor]", "")
					// If removing them leaves something significant?
					// But strings.Contains is safer.

					// Simplest: If the ONLY warning is "Unrecognized MakerNotes", ignore it.
					// ExifTool often concatenates: "Warning: Unrecognized MakerNotes. Error: truncated."
					// So if error is empty, and warning only contains that?

					// User said: "These types of errors are not worrysome and can be ignored."
					// So let's suppress them.

					// If Error is empty AND Warning IS effectively just "Unrecognized MakerNotes" (with minor variations)
					// Or just simpler: don't flag if the warning is strictly about MakerNotes.
					// But if there is a separate Error field, we should probably still flag it?

					// Let's implement: If Error is present, ALWAYS flag. If Warning matches ignored list, ignore warning.
					if res.Error != "" {
						isProblem = true
					} else {
						// Only check warning
						if !strings.Contains(res.Warning, "Unrecognized MakerNotes") && !strings.Contains(res.Warning, "[minor]") {
							isProblem = true
						}
					}
				}
			} else {
				isProblem = true
			}
		}

		// Check 2: Size < 20KB
		if res.FileSize < 20000 {
			if res.Warning == "" {
				res.Warning = "File size too small (< 20KB)"
			} else {
				res.Warning += "; File size too small (< 20KB)"
			}
			isProblem = true
		}

		if isProblem {
			// Classify severity
			severity := classifySeverity(res)
			res.Severity = severity

			problems = append(problems, res)
			issueCount++
			badFiles = append(badFiles, fmt.Sprintf("%s (%s)", filepath.Base(res.SourceFile), res.Error+" "+res.Warning))
		}
	}

	// Prepare Report
	now := time.Now().Format(time.RFC3339)
	report := IssueReport{
		RunDate: now,
	}

	if len(problems) > 0 {
		report.Problems = problems
		report.Message = fmt.Sprintf("Found %d issues.", len(problems))
		if recordIssue != nil {
			recordIssue(virtualPath)
		}
	} else {
		report.Message = "No problematic files found."
	}

	// Write JSON
	reportBytes, _ := json.MarshalIndent(report, "", "  ")
	issuePath := filepath.Join(realPath, IssueFilename)
	_ = os.WriteFile(issuePath, reportBytes, 0644)

	// Requirement: Folder ( folder name and path ) scan completed, number files, number of issues.
	// Requirement: If there are bad files list them.
	logMsg := fmt.Sprintf("Folder ( %s ) scan completed, %d files, %d issues.", virtualPath, fileCount, issueCount)
	if issueCount > 0 {
		logMsg += fmt.Sprintf(" Bad files: %v", badFiles)
		logger.Error(logMsg) // Log errors if issues found
	} else {
		logger.Debug(logMsg) // Debug only for clean folders
	}
}
