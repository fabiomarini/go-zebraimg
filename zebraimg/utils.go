package zebraimg

import (
	"bytes"
	"image"
	"image/png"
	"os"
)

// ZebraGRFResult holds the encoded GRF data with metadata.
type ZebraGRFResult struct {
	AsciiData   string
	Width       int
	Height      int
	BytesPerRow int
}

// LoadImage loads a PNG image from a file path.
func LoadImage(filePath string) (image.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return png.Decode(file)
}

// LoadImageFromBytes loads a PNG image from a byte array.
func LoadImageFromBytes(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
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
