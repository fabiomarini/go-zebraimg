package gozebraimg

import (
	"bytes"
	"image"
	"image/png"
	"os"
)

// ZebraGRFResult holds the encoded GRF data with metadata
type ZebraGRFResult struct {
	AsciiData   string
	Width       int
	Height      int
	BytesPerRow int
}

// LoadImage loads an image from a file path
// Supports PNG format
func LoadImage(filePath string) (image.Image, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return png.Decode(file)
}

// LoadImageFromBytes loads an image from a byte array
// Supports PNG format
func LoadImageFromBytes(data []byte) (image.Image, error) {
	reader := bytes.NewReader(data)
	return png.Decode(reader)
}

// EncodeImageToGRF encodes an image to GRF format with metadata
func EncodeImageToGRF(img image.Image, dither bool) (*ZebraGRFResult, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	bytesPerRow := (width + 7) / 8

	encoder := New(bytesPerRow)
	asciiData, err := encoder.EncodeImage(img, dither)
	if err != nil {
		return nil, err
	}

	return &ZebraGRFResult{
		AsciiData:   asciiData,
		Width:       width,
		Height:      height,
		BytesPerRow: bytesPerRow,
	}, nil
}

// DecodeGRFToImage decodes GRF result back to an image
func DecodeGRFToImage(result *ZebraGRFResult) (image.Image, error) {
	encoder := New(result.BytesPerRow)
	return encoder.DecodeImage(result.AsciiData)
}

// SaveImage saves an image to a file
func SaveImage(img image.Image, filePath string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}
