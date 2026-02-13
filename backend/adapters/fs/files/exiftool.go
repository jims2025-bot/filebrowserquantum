package files

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/gtsteffaniak/go-logger/logger"
)

// ExifToolBridge manages a persistent exiftool process using the -stay_open mode.
type ExifToolBridge struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	reader *bufio.Reader
	mu     sync.Mutex
}

var (
	foregroundBridge *ExifToolBridge
	backgroundBridge *ExifToolBridge
	fgOnce           sync.Once
	bgOnce           sync.Once
)

// GetExifToolBridge returns the singleton instance for foreground (immediate) UI requests.
func GetExifToolBridge() (*ExifToolBridge, error) {
	var err error
	fgOnce.Do(func() {
		foregroundBridge, err = newExifToolBridge("Foreground")
	})
	return foregroundBridge, err
}

// GetExifToolBridgeBackground returns the singleton instance for long-running background tasks.
func GetExifToolBridgeBackground() (*ExifToolBridge, error) {
	var err error
	bgOnce.Do(func() {
		backgroundBridge, err = newExifToolBridge("Background")
	})
	return backgroundBridge, err
}

func newExifToolBridge(name string) (*ExifToolBridge, error) {
	logger.Infof("Starting persistent ExifTool bridge [%s]...", name)

	// -stay_open True: Keep the process open
	// -@ -: Read arguments from stdin
	cmd := exec.Command("exiftool", "-stay_open", "True", "-@", "-")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start exiftool: %w", err)
	}

	b := &ExifToolBridge{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		reader: bufio.NewReader(stdout),
	}

	// Start a goroutine to wait for the command to finish and log errors
	go func() {
		err := cmd.Wait()
		if err != nil {
			logger.Errorf("Persistent ExifTool process exited with error: %v", err)
		}
	}()

	return b, nil
}

// Execute runs an exiftool command by sending arguments to the persistent process.
// It returns the JSON output of the command.
func (b *ExifToolBridge) Execute(args []string) ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Write arguments followed by -execute
	for _, arg := range args {
		fmt.Fprintln(b.stdin, arg)
	}
	fmt.Fprintln(b.stdin, "-execute")

	// Read until {ready} marker
	// We use -j so we expect a JSON array
	var output strings.Builder
	for {
		line, err := b.reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read from exiftool: %w", err)
		}
		if strings.TrimSpace(line) == "{ready}" {
			break
		}
		output.WriteString(line)
	}

	return []byte(output.String()), nil
}

// GetMetadataBulk fetches metadata for multiple files in a single pass.
func (b *ExifToolBridge) GetMetadataBulk(paths []string) ([]map[string]interface{}, error) {
	args := []string{"-m", "-j", "-G"}
	args = append(args, "-EXIF:All", "-IPTC:All", "-XMP:All")
	args = append(args, paths...)

	output, err := b.Execute(args)
	if err != nil {
		return nil, err
	}

	var metadataArray []map[string]interface{}
	if err := json.Unmarshal(output, &metadataArray); err != nil {
		return nil, fmt.Errorf("failed to parse bulk metadata: %w", err)
	}

	return metadataArray, nil
}

// GetGPSBulk fetches GPS data specifically for images in a folder.
func (b *ExifToolBridge) GetGPSBulk(folderPath string) ([]map[string]interface{}, error) {
	// -n: Print machine-readable values
	// -j: JSON output
	// -ext: Process only image files to be safe
	args := []string{"-m", "-n", "-j", "-GPSLatitude", "-GPSLongitude", "-Filename", "-r", folderPath}

	output, err := b.Execute(args)
	if err != nil {
		return nil, err
	}

	var gpsArray []map[string]interface{}
	if err := json.Unmarshal(output, &gpsArray); err != nil {
		return nil, fmt.Errorf("failed to parse bulk GPS data: %w", err)
	}

	return gpsArray, nil
}

// Close gracefully shuts down the persistent exiftool process.
func (b *ExifToolBridge) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	fmt.Fprintln(b.stdin, "-stay_open")
	fmt.Fprintln(b.stdin, "False")
	return b.stdin.Close()
}
