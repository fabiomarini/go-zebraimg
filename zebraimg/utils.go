package zebraimg

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // register JPEG decoder for image.Decode
	"image/png"    // named: SaveImage encodes PNG; also registers the decoder
	"os"

	"golang.org/x/image/draw"
)

// ZebraGRFResult holds the encoded GRF data with metadata.
type ZebraGRFResult struct {
	AsciiData   string
	Width       int
	Height      int
	BytesPerRow int
}

// ZPL returns the encoded data as a complete ZPL ^GF command:
// ^GFA,b,c,d,data^FS, where b and c are the total graphic bytes and d is
// the bytes per row. Recall the graphic inside a label (^XA...^XZ) with ^XG.
func (r *ZebraGRFResult) ZPL() string {
	total := r.BytesPerRow * r.Height
	return fmt.Sprintf("^GFA,%d,%d,%d,%s^FS", total, total, r.BytesPerRow, r.AsciiData)
}

// LoadImage loads a PNG or JPEG image from a file path.
func LoadImage(filePath string) (image.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

// LoadImageFromBytes loads a PNG or JPEG image from a byte array.
func LoadImageFromBytes(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

// ResizeImage scales an image to the given pixel dimensions using
// Catmull-Rom resampling, which downscales print artwork cleanly. A zero
// dimension is derived from the other to preserve the aspect ratio; both
// zero or any negative is an error.
func ResizeImage(img image.Image, width, height int) (image.Image, error) {
	if width < 0 || height < 0 || (width == 0 && height == 0) {
		return nil, fmt.Errorf("zebraimg: invalid resize dimensions %dx%d", width, height)
	}

	b := img.Bounds()
	if width == 0 {
		width = max(b.Dx()*height/b.Dy(), 1)
	}
	if height == 0 {
		height = max(b.Dy()*width/b.Dx(), 1)
	}
	if width == b.Dx() && height == b.Dy() {
		return img, nil
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst, nil
}

// EncodeImageToGRF encodes an image to GRF format with metadata.
// If dither is true, applies Floyd-Steinberg error diffusion.
func EncodeImageToGRF(img image.Image, dither bool) (*ZebraGRFResult, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	encoder, err := New((width + 7) / 8)
	if err != nil {
		return nil, err
	}

	asciiData, err := encoder.EncodeImage(img, dither)
	if err != nil {
		return nil, err
	}

	return &ZebraGRFResult{
		AsciiData:   asciiData,
		Width:       width,
		Height:      height,
		BytesPerRow: encoder.BytesPerRow,
	}, nil
}

// DecodeGRFToImage decodes a GRF result back to an image. The decoded image
// width is BytesPerRow*8, rounded up from the original width; padding pixels
// are white.
func DecodeGRFToImage(result *ZebraGRFResult) (image.Image, error) {
	encoder, err := New(result.BytesPerRow)
	if err != nil {
		return nil, err
	}
	return encoder.DecodeImage(result.AsciiData)
}

// SaveImage saves an image to a PNG file.
func SaveImage(img image.Image, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
