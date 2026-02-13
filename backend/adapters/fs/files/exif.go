package files

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// UpdateExif updates or adds GPS coordinates to an image file using exiftool.
func UpdateExif(path string, lat, lon float64) error {
	// Invalidate cache for the folder
	cacheMu.Lock()
	delete(folderMetadataCache, filepath.Dir(path))
	cacheMu.Unlock()

	b, err := GetExifToolBridge()
	if err != nil {
		return err
	}

	// Calculate Refs and absolute values
	latRef := "N"
	if lat < 0 {
		latRef = "S"
		lat = -lat
	}

	lonRef := "E"
	if lon < 0 {
		lonRef = "W"
		lon = -lon
	}

	args := []string{
		"-m",
		"-overwrite_original",
		fmt.Sprintf("-GPSLatitude=%f", lat),
		fmt.Sprintf("-GPSLatitudeRef=%s", latRef),
		fmt.Sprintf("-GPSLongitude=%f", lon),
		fmt.Sprintf("-GPSLongitudeRef=%s", lonRef),
		path,
	}

	_, err = b.Execute(args)
	return err
}

// GetGPS reads the GPS coordinates from a file using the persistent exiftool bridge.
func GetGPS(path string) (float64, float64, error) {
	// Check cache first (it's in files.go, so we check filePath)
	folderPath := filepath.Dir(path)
	fileName := filepath.Base(path)
	if meta, ok := getFromCache(folderPath, fileName); ok {
		// Try to extract GPS from cached metadata
		// exiftool -n -j gives numeric values
		if lat, ok := meta["EXIF:GPSLatitude"].(float64); ok {
			if lon, ok := meta["EXIF:GPSLongitude"].(float64); ok {
				return lat, lon, nil
			}
		}
		// Fallback to checking other groups if EXIF is missing
		if lat, ok := m_fetchFloat(meta, "GPSLatitude"); ok {
			if lon, ok := m_fetchFloat(meta, "GPSLongitude"); ok {
				return lat, lon, nil
			}
		}
	}

	b, err := GetExifToolBridge()
	if err != nil {
		return 0, 0, err
	}

	// Fetch just GPS using bridge if not in cache
	args := []string{"-n", "-p", "$GPSLatitude,$GPSLongitude", path}
	outputBytes, err := b.Execute(args)
	if err != nil {
		return 0, 0, err
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

func m_fetchFloat(meta map[string]interface{}, key string) (float64, bool) {
	for k, v := range meta {
		if strings.HasSuffix(k, ":"+key) || k == key {
			if f, ok := v.(float64); ok {
				return f, true
			}
		}
	}
	return 0, false
}

// RemoveExif removes GPS coordinates from an image file using the persistent exiftool bridge.
func RemoveExif(path string) error {
	// Invalidate cache for the folder
	cacheMu.Lock()
	delete(folderMetadataCache, filepath.Dir(path))
	cacheMu.Unlock()

	b, err := GetExifToolBridge()
	if err != nil {
		return err
	}

	args := []string{
		"-m",
		"-overwrite_original",
		"-GPSLatitude=",
		"-GPSLatitudeRef=",
		"-GPSLongitude=",
		"-GPSLongitudeRef=",
		path,
	}

	_, err = b.Execute(args)
	return err
}
