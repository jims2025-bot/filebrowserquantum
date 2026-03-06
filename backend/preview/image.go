//go:generate go-enum --sql --marshal --file $GOFILE
package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"log"

	"github.com/disintegration/imaging"
	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
)

// Format is an image file format.
/*
ENUM(
jpeg
png
gif
tiff
bmp
)
*/
type Format int

func (x Format) toImaging() imaging.Format {
	switch x {
	case FormatJpeg:
		return imaging.JPEG
	case FormatPng:
		return imaging.PNG
	case FormatGif:
		return imaging.GIF
	case FormatTiff:
		return imaging.TIFF
	case FormatBmp:
		return imaging.BMP
	default:
		return imaging.JPEG
	}
}

/*
ENUM(
high
medium
low
)
*/
type Quality int

func (x Quality) resampleFilter() imaging.ResampleFilter {
	switch x {
	case QualityHigh:
		return imaging.Lanczos
	case QualityMedium:
		return imaging.Box
	case QualityLow:
		return imaging.NearestNeighbor
	default:
		return imaging.Box
	}
}

/*
ENUM(
fit
fill
)
*/
type ResizeMode int

func (s *Service) FormatFromExtension(ext string) (Format, error) {
	format, err := imaging.FormatFromExtension(ext)
	if err != nil {
		return -1, ErrUnsupportedFormat
	}
	switch format {
	case imaging.JPEG:
		return FormatJpeg, nil
	case imaging.PNG:
		return FormatPng, nil
	case imaging.GIF:
		return FormatGif, nil
	case imaging.TIFF:
		return FormatTiff, nil
	case imaging.BMP:
		return FormatBmp, nil
	default:
		return -1, ErrUnsupportedFormat
	}
}

type resizeConfig struct {
	format     Format
	resizeMode ResizeMode
	quality    Quality
	boxParam   string
}

type Option func(*resizeConfig)

func WithFormat(format Format) Option {
	return func(config *resizeConfig) {
		config.format = format
	}
}

func WithMode(mode ResizeMode) Option {
	return func(config *resizeConfig) {
		config.resizeMode = mode
	}
}

func WithQuality(quality Quality) Option {
	return func(config *resizeConfig) {
		config.quality = quality
	}
}

func WithBox(boxParam string) Option {
	return func(config *resizeConfig) {
		config.boxParam = boxParam
	}
}

func (s *Service) Resize(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...Option) error {
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer s.release()

	config := resizeConfig{
		resizeMode: ResizeModeFit,
		quality:    QualityMedium,
	}
	for _, option := range options {
		option(&config)
	}

	// Read everything into memory once to allow multi-pass (EXIF + Decode)
	data, err := io.ReadAll(in)
	if err != nil {
		return err
	}

	format, _, err := s.detectFormat(bytes.NewReader(data))
	if err != nil {
		return err
	}

	if config.format == 0 {
		config.format = format
	}

	// Disable embedded thumbnail extraction if we need to crop a specific face box,
	// because the thumbnail will likely trim out the face we're actually looking for.
	if config.quality == QualityLow && format == FormatJpeg && config.boxParam == "" {
		thm, _, errThm := getEmbeddedThumbnail(bytes.NewReader(data))
		if errThm == nil {
			_, err = out.Write(thm)
			if err == nil {
				return nil
			}
		}
	}

	// If a box is provided, we must handle orientation manually AFTER cropping
	// to ensure coordinates match the raw pixels.
	var img image.Image
	var orientation int
	if config.boxParam != "" {
		// Bulletproof raw decode: try standard library first
		img, _, err = image.Decode(bytes.NewReader(data))
		if err != nil {
			// Fallback to imaging.Decode if standard fails
			img, err = imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(false))
			if err != nil {
				return err
			}
		}
		orientation = getOrientation(data)
		log.Printf("[PreviewDebug] Face crop requested: %s, Orientation: %d", config.boxParam, orientation)

		// Perform dynamic face cropping
		var y1, x2, y2, x1 int
		_, err = fmt.Sscanf(config.boxParam, "%d,%d,%d,%d", &y1, &x2, &y2, &x1)
		if err == nil && x2 > x1 && y2 > y1 {
			rect := image.Rect(x1, y1, x2, y2)
			bounds := img.Bounds()
			log.Printf("[PreviewDebug] Orientation: %d, ImgSize: %dx%d, Box: [%d,%d,%d,%d], Crop: %v",
				orientation, bounds.Dx(), bounds.Dy(), y1, x2, y2, x1, rect)
			img = imaging.Crop(img, rect)
		}

		// Apply orientation to the crop
		img = applyOrientation(img, orientation)
	} else {
		// Use imaging.Decode with auto-orientation for efficiency
		img, err = imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
		if err != nil {
			return err
		}
	}

	switch config.resizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, width, height, imaging.Center, config.quality.resampleFilter())
	case ResizeModeFit:
		fallthrough
	default:
		img = imaging.Fit(img, width, height, config.quality.resampleFilter())
	}

	return imaging.Encode(out, img, config.format.toImaging())
}

func getOrientation(data []byte) int {
	rawExif, err := exif.SearchAndExtractExif(data)
	if err != nil {
		return 1
	}
	im, _ := exifcommon.NewIfdMappingWithStandard()
	ti := exif.NewTagIndex()
	_, index, err := exif.Collect(im, ti, rawExif)
	if err != nil {
		return 1
	}
	ifd := index.RootIfd
	results, err := ifd.FindTagWithName("Orientation")
	if err != nil {
		return 1
	}
	if len(results) > 0 {
		valAny, err := results[0].Value()
		if err != nil {
			return 1
		}
		if val, ok := valAny.([]uint16); ok && len(val) > 0 {
			return int(val[0])
		}
		if val, ok := valAny.([]int); ok && len(val) > 0 {
			return val[0]
		}
	}
	return 1
}

func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
}

func (s *Service) detectFormat(in io.Reader) (Format, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)

	_, imgFormat, err := image.DecodeConfig(r)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %w", err.Error(), ErrUnsupportedFormat)
	}

	format, err := ParseFormat(imgFormat)
	if err != nil {
		return 0, nil, ErrUnsupportedFormat
	}

	return format, io.MultiReader(buf, in), nil
}

func getEmbeddedThumbnail(in io.Reader) ([]byte, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)
	wrappedReader := io.MultiReader(buf, in)

	offsets := []int{12, 30}
	head := make([]byte, 0xffff)

	_, err := r.Read(head)
	if err != nil {
		return nil, wrappedReader, err
	}

	var offset int
	for _, offset = range offsets {
		if _, err = exif.ParseExifHeader(head[offset:]); err == nil {
			break
		}
	}

	if err != nil {
		return nil, wrappedReader, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, wrappedReader, err
	}

	_, index, err := exif.Collect(im, exif.NewTagIndex(), head[offset:])
	if err != nil {
		return nil, wrappedReader, err
	}

	ifd := index.RootIfd.NextIfd()
	if ifd == nil {
		return nil, wrappedReader, exif.ErrNoThumbnail
	}

	thm, err := ifd.Thumbnail()
	return thm, wrappedReader, err
}

// CreateThumbnail decodes an image and creates a fixed-size thumbnail.
func CreateThumbnail(rawData io.Reader, width, height int) (image.Image, error) {
	img, _, err := image.Decode(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	thumb := imaging.Fit(img, width, height, imaging.Lanczos)
	return thumb, nil
}
