package files

import (
	"fmt"
	"os/exec"
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
