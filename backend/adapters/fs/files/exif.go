package files

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// UpdateExif updates or adds GPS coordinates to an image file using exiftool.
func UpdateExif(path string, lat, lon float64) error {
	// Construct exiftool command arguments
	// -overwrite_original: Overwrite the original file instead of creating _original backup

	latRef := "N"
	if lat < 0 {
		latRef = "S"
		lat = -lat // Make positive
	}
	lonRef := "E"
	if lon < 0 {
		lonRef = "W"
		lon = -lon // Make positive
	}

	// Format: -GPSLatitude=val -GPSLatitudeRef=val -GPSLongitude=val -GPSLongitudeRef=val
	// ExifTool handles decimal degrees automatically if we just pass them.
	// But refs might be needed if not signed.
	// Actually ExifTool is smart enough: -GPSLatitude=lat -GPSLongitude=lon
	// If we pass signed values, it might handle it?
	// The documentation says: "GPSLatitude and GPSLongitude will also accept signed values."

	// Let's use the explicit assignment to be safe and standard.
	// We need to pass the values.

	cmd := exec.Command("exiftool",
		"-m", // Ignore minor errors (e.g. missing EOI)
		"-overwrite_original",
		fmt.Sprintf("-GPSLatitude=%f", lat),
		fmt.Sprintf("-GPSLatitudeRef=%s", latRef),
		fmt.Sprintf("-GPSLongitude=%f", lon),
		fmt.Sprintf("-GPSLongitudeRef=%s", lonRef),
		path,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %s: %w", string(output), err)
	}

	return nil
}

// GetGPS reads the GPS coordinates from a file using exiftool.
func GetGPS(path string) (float64, float64, error) {
	cmd := exec.Command("exiftool", "-n", "-p", "$GPSLatitude,$GPSLongitude", path)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("exiftool failed: %w", err)
	}

	output := strings.TrimSpace(string(outputBytes))
	parts := strings.Split(output, ",")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid GPS data found")
	}

	lat, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid latitude: %w", err)
	}

	lon, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid longitude: %w", err)
	}

	return lat, lon, nil
}

// RemoveExif removes GPS coordinates from an image file using exiftool.
func RemoveExif(path string) error {
	cmd := exec.Command("exiftool",
		"-m", // Ignore minor errors
		"-overwrite_original",
		"-GPSLatitude=",
		"-GPSLatitudeRef=",
		"-GPSLongitude=",
		"-GPSLongitudeRef=",
		path,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("exiftool failed: %s: %w", string(output), err)
	}

	return nil
}
