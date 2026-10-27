package imageprocessing

import (
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// gradient builds a synthetic RGB test image whose colors vary across it.
func gradient(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / width), G: uint8(y * 255 / height), B: 128, A: 255})
		}
	}
	return img
}

func TestSupportedExtension(t *testing.T) {
	cases := map[string]bool{
		"a.jpg": true, "a.JPEG": true, "dir/a.png": true,
		"a.gif": false, "a": false, "a.jpg.txt": false,
	}
	for path, want := range cases {
		if got := SupportedExtension(path); got != want {
			t.Errorf("SupportedExtension(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestGrayscaleKnownColors(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	img.Set(0, 0, color.RGBA{255, 255, 255, 255}) // white
	img.Set(1, 0, color.RGBA{0, 0, 0, 255})       // black
	img.Set(2, 0, color.RGBA{255, 0, 0, 255})     // pure red

	gray := Grayscale(img)
	want := []uint8{255, 0, 76} // red luminance = 0.299 * 255 ≈ 76
	for x, expected := range want {
		if got := gray.GrayAt(x, 0).Y; got != expected {
			t.Errorf("pixel %d: got %d, want %d", x, got, expected)
		}
	}
	if gray.Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), gray.Bounds())
	}
}

func TestResizeGivesRequestedSize(t *testing.T) {
	resized := Resize(gradient(120, 80), 60, 40)
	if size := resized.Bounds().Size(); size != image.Pt(60, 40) {
		t.Errorf("got %v, want 60x40", size)
	}
}

func TestScaledSize(t *testing.T) {
	cases := []struct {
		name                  string
		width, height, side   int
		keepAspect            bool
		wantWidth, wantHeight int
	}{
		{"landscape", 1200, 600, 500, true, 500, 250},
		{"portrait (tall)", 300, 900, 500, true, 167, 500},
		{"square", 1411, 1411, 500, true, 500, 500},
		{"rounding", 640, 427, 500, true, 500, 334},
		{"very thin keeps 1 pixel", 5000, 2, 500, true, 500, 1},
		{"upscale small image", 100, 50, 500, true, 500, 250},
		{"original behavior", 1200, 600, 500, false, 500, 500},
	}
	for _, c := range cases {
		width, height := ScaledSize(c.width, c.height, c.side, c.keepAspect)
		if width != c.wantWidth || height != c.wantHeight {
			t.Errorf("%s: ScaledSize(%d, %d, %d) = %dx%d, want %dx%d",
				c.name, c.width, c.height, c.side, width, height, c.wantWidth, c.wantHeight)
		}
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	original := gradient(40, 30)
	for _, name := range []string{"out.png", "out.jpg", "OUT.JPEG"} {
		path := filepath.Join(t.TempDir(), name)
		if err := Write(path, original); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		decoded, err := Read(path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if decoded.Bounds() != original.Bounds() {
			t.Errorf("%s: bounds %v, want %v", name, decoded.Bounds(), original.Bounds())
		}
	}
}

func TestReadDetectsFormatFromContents(t *testing.T) {
	// A PNG saved with a .jpg name must still decode.
	pngPath := filepath.Join(t.TempDir(), "really.png")
	if err := Write(pngPath, gradient(8, 8)); err != nil {
		t.Fatal(err)
	}
	misnamed := filepath.Join(t.TempDir(), "misnamed.jpg")
	data, _ := os.ReadFile(pngPath)
	os.WriteFile(misnamed, data, 0o644)
	if _, err := Read(misnamed); err != nil {
		t.Errorf("misnamed PNG should decode: %v", err)
	}
}

func TestReadErrors(t *testing.T) {
	dir := t.TempDir()
	corrupt := filepath.Join(dir, "corrupt.jpg")
	os.WriteFile(corrupt, []byte("definitely not an image"), 0o644)

	if _, err := Read(filepath.Join(dir, "missing.jpg")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file: want os.ErrNotExist, got %v", err)
	}
	if _, err := Read(corrupt); err == nil {
		t.Error("corrupt file: expected a decode error")
	}
}

func TestWriteErrors(t *testing.T) {
	dir := t.TempDir()
	if err := Write(filepath.Join(dir, "out.gif"), gradient(4, 4)); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("unsupported extension: want ErrUnsupportedFormat, got %v", err)
	}
	if err := Write(filepath.Join(dir, "no-such-folder", "out.png"), gradient(4, 4)); err == nil {
		t.Error("missing folder: expected an error")
	}
}

func TestWriteRemovesPartialFileOnEncodeFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "too-wide.jpg")
	// JPEG cannot store images 65,536 or more pixels wide, so encoding fails
	// after the file has been created.
	if err := Write(path, image.NewGray(image.Rect(0, 0, 70_000, 1))); err == nil {
		t.Fatal("expected an encode error for a too-wide image")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("partial file should have been removed, stat error: %v", err)
	}
}

func BenchmarkGrayscale(b *testing.B) {
	img := gradient(1000, 750)
	for b.Loop() {
		Grayscale(img)
	}
}

func BenchmarkResize(b *testing.B) {
	img := gradient(1000, 750)
	for b.Loop() {
		Resize(img, 500, 375)
	}
}
