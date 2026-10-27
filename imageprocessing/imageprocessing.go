// Package imageprocessing reads, transforms, and writes images: the
// individual steps that the pipeline package chains together.
//
// Every function that can fail returns an error instead of panicking, so a
// single unreadable file cannot crash a whole batch.
package imageprocessing

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/nfnt/resize"
)

// JPEGQuality is the encoder quality used for .jpg/.jpeg output (1-100).
const JPEGQuality = 90

// ErrUnsupportedFormat is returned for file extensions this package cannot encode.
var ErrUnsupportedFormat = errors.New("unsupported image format (use .jpg, .jpeg, or .png)")

// SupportedExtension reports whether path has an extension this package can
// read and write. The check ignores case, so "PHOTO.JPG" is accepted.
func SupportedExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return true
	}
	return false
}

// Read opens and decodes a JPEG or PNG file.
func Read(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// image.Decode picks the decoder from the file's contents, not its name;
	// the jpeg and png imports above register both decoders.
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}
	if img.Bounds().Empty() {
		return nil, fmt.Errorf("decoding %s: image has no pixels", path)
	}
	return img, nil
}

// Write encodes img to path, choosing JPEG or PNG from the file extension.
// If encoding fails, the partly written file is removed.
func Write(path string, img image.Image) error {
	encode, err := encoderFor(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := encode(file, img); err != nil {
		file.Close()
		os.Remove(path)
		return fmt.Errorf("encoding %s: %w", path, err)
	}
	return file.Close()
}

type encodeFunc func(*os.File, image.Image) error

func encoderFor(path string) (encodeFunc, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return func(file *os.File, img image.Image) error {
			return jpeg.Encode(file, img, &jpeg.Options{Quality: JPEGQuality})
		}, nil
	case ".png":
		return func(file *os.File, img image.Image) error { return png.Encode(file, img) }, nil
	}
	return nil, ErrUnsupportedFormat
}

// Grayscale converts img to 8-bit grayscale, one pixel at a time, using the
// standard library's luminance weights (0.299 R + 0.587 G + 0.114 B).
func Grayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, color.GrayModel.Convert(img.At(x, y)))
		}
	}
	return gray
}

// Resize scales img to exactly width × height pixels with Lanczos-3
// resampling. If the target shape differs from the image's, the picture is
// stretched; use ScaledSize to keep the aspect ratio.
func Resize(img image.Image, width, height int) image.Image {
	return resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
}

// ScaledSize returns the output size for an image of width × height pixels.
//
// With keepAspect, the longer side becomes longSide and the shorter side is
// scaled by the same factor (rounded, and at least 1 pixel), so a 1200 × 600
// photo becomes 500 × 250 rather than being squashed into 500 × 500.
// Without keepAspect, every image becomes longSide × longSide, which is the
// original program's behavior.
func ScaledSize(width, height, longSide int, keepAspect bool) (int, int) {
	if !keepAspect {
		return longSide, longSide
	}
	if width >= height {
		return longSide, scaleSide(height, width, longSide)
	}
	return scaleSide(width, height, longSide), longSide
}

// scaleSide scales short by longSide/long, rounding to the nearest pixel.
func scaleSide(short, long, longSide int) int {
	scaled := (short*longSide + long/2) / long
	return max(scaled, 1)
}
