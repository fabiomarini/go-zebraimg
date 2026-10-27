package zebraimg

import (
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
)

// Bit convention follows the ZPL ^GF spec: bit 1 = black (print dot),
// bit 0 = white.
type ZebraGRF struct {
	BytesPerRow int
}

// ErrInvalidBytesPerRow is returned when a non-positive bytesPerRow is used.
var ErrInvalidBytesPerRow = errors.New("zebraimg: BytesPerRow must be positive")

// ErrInvalidCharacter is returned when GRF data contains a character that is
// not a hex digit, repeat count, line repeat (:), or fill (, or !).
var ErrInvalidCharacter = errors.New("zebraimg: invalid GRF character")

// New creates a new ZebraGRF instance with the specified bytes per row.
// bytesPerRow must be positive; use (width+7)/8 for an image of the given
// pixel width.
func New(bytesPerRow int) (*ZebraGRF, error) {
	if bytesPerRow <= 0 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidBytesPerRow, bytesPerRow)
	}
	return &ZebraGRF{BytesPerRow: bytesPerRow}, nil
}

// EncodeImage converts an image.Image to a GRF ASCII encoded string.
// If dither is true, applies Floyd-Steinberg error diffusion; otherwise a
// simple Rec. 601 luma threshold is used.
// z.BytesPerRow must equal (width+7)/8 for the image, or an error is returned.
func (z *ZebraGRF) EncodeImage(img image.Image, dither bool) (string, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	if want := (width + 7) / 8; z.BytesPerRow != want {
		return "", fmt.Errorf("zebraimg: BytesPerRow %d does not match image width %d (expected %d)", z.BytesPerRow, width, want)
	}

	var hexData string
	if dither {
		hexData = z.imageToHexString(img)
	} else {
		hexData = z.imageToHexBWString(img)
	}
	return z.encodeHexToAscii(hexData), nil
}

// DecodeImage converts a GRF ASCII encoded string back to an image.
// The returned image width is BytesPerRow*8; pixels beyond the original
// encoded width are padding bits (0), which decode as white.
func (z *ZebraGRF) DecodeImage(asciiData string) (image.Image, error) {
	hexString, err := z.decodeAsciiToHex(asciiData)
	if err != nil {
		return nil, err
	}

	hexCharsPerRow := z.BytesPerRow * 2
	if len(hexString)%hexCharsPerRow != 0 {
		return nil, fmt.Errorf("zebraimg: truncated data: %d hex chars is not a multiple of row length %d", len(hexString), hexCharsPerRow)
	}

	width := z.BytesPerRow * 8
	height := len(hexString) / hexCharsPerRow

	return z.hexStringToImage(hexString, width, height)
}

// luminance converts a color to a 0-255 grayscale value using Rec. 601 luma
// weights. Fully transparent pixels are treated as white.
func luminance(c color.Color) float32 {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return 255
	}
	return (0.299*float32(r) + 0.587*float32(g) + 0.114*float32(b)) / 257
}

// imageToHexString converts image.Image to a hexadecimal string using
// Floyd-Steinberg error diffusion dithering. Bit 1 = black.
func (z *ZebraGRF) imageToHexString(img image.Image) string {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	actualBytesPerRow := (width + 7) / 8

	grayscale := make([]float32, height*width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			grayscale[y*width+x] = luminance(img.At(bounds.Min.X+x, bounds.Min.Y+y))
		}
	}

	// Floyd-Steinberg error diffusion, distributing the full quantization
	// error (16/16):
	//           X   7/16
	//   3/16  5/16  1/16
	black := make([]bool, height*width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			oldPixel := grayscale[idx]
			newPixel := float32(0)
			if oldPixel >= 128 {
				newPixel = 255
			}
			black[idx] = newPixel == 0
			errorVal := oldPixel - newPixel

			if x+1 < width {
				grayscale[idx+1] += errorVal * 7 / 16
			}
			if y+1 < height {
				nextRowIdx := (y + 1) * width
				if x > 0 {
					grayscale[nextRowIdx+x-1] += errorVal * 3 / 16
				}
				grayscale[nextRowIdx+x] += errorVal * 5 / 16
				if x+1 < width {
					grayscale[nextRowIdx+x+1] += errorVal * 1 / 16
				}
			}
		}
	}

	hexChars := "0123456789ABCDEF"
	var result strings.Builder
	result.Grow(height * actualBytesPerRow * 2)

	for y := 0; y < height; y++ {
		for byteIndex := 0; byteIndex < actualBytesPerRow; byteIndex++ {
			byteValue := 0
			for bitIndex := 0; bitIndex <= 7; bitIndex++ {
				x := byteIndex*8 + bitIndex
				// Padding bits (x >= width) stay 0, i.e. white.
				if x < width && black[y*width+x] {
					byteValue |= 1 << (7 - bitIndex)
				}
			}
			result.WriteByte(hexChars[byteValue>>4])
			result.WriteByte(hexChars[byteValue&0xF])
		}
	}

	return result.String()
}

// imageToHexBWString converts image.Image to a hexadecimal string without
// dithering, using a simple Rec. 601 luma threshold. Bit 1 = black.
func (z *ZebraGRF) imageToHexBWString(img image.Image) string {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	actualBytesPerRow := (width + 7) / 8

	hexChars := "0123456789ABCDEF"
	var result strings.Builder
	result.Grow(height * actualBytesPerRow * 2)

	for y := 0; y < height; y++ {
		for byteIndex := 0; byteIndex < actualBytesPerRow; byteIndex++ {
			byteValue := 0
			for bitIndex := 0; bitIndex <= 7; bitIndex++ {
				x := byteIndex*8 + bitIndex
				if x < width && luminance(img.At(bounds.Min.X+x, bounds.Min.Y+y)) < 128 {
					byteValue |= 1 << (7 - bitIndex)
				}
			}
			result.WriteByte(hexChars[byteValue>>4])
			result.WriteByte(hexChars[byteValue&0xF])
		}
	}

	return result.String()
}

// encodeHexToAscii compresses a hex string to GRF ASCII using run-length
// encoding and line repeats.
func (z *ZebraGRF) encodeHexToAscii(hexString string) string {
	var result strings.Builder
	result.Grow(len(hexString))
	hexCharsPerRow := z.BytesPerRow * 2

	var previousLine string
	pos := 0

	for pos < len(hexString) {
		lineEnd := min(pos+hexCharsPerRow, len(hexString))
		currentLine := hexString[pos:lineEnd]

		if currentLine == previousLine {
			result.WriteString(":")
		} else {
			result.WriteString(z.encodeLine(currentLine))
		}

		previousLine = currentLine
		pos = lineEnd
	}

	return result.String()
}

// encodeLine encodes a single line of hex data with run-length compression.
// A line consisting entirely of 0s or Fs collapses to "," or "!"; a trailing
// run of 0s or Fs still emits its repeat count followed by the fill character.
func (z *ZebraGRF) encodeLine(line string) string {
	var result strings.Builder
	i := 0

	for i < len(line) {
		currentChar := line[i]
		count := 1

		for i+count < len(line) && line[i+count] == currentChar {
			count++
		}

		// Line ends with a run of fill characters: the fill character alone
		// covers the rest of the line on decode.
		if i+count == len(line) && (currentChar == '0' || currentChar == 'F') {
			if i == 0 {
				if currentChar == '0' {
					return ","
				}
				return "!"
			}
			if count > 1 {
				result.WriteString(z.encodeRepeatCount(count))
			}
			result.WriteByte(currentChar)
			return result.String()
		}

		if count > 1 {
			result.WriteString(z.encodeRepeatCount(count))
		}
		result.WriteByte(currentChar)

		i += count
	}

	return result.String()
}

// encodeRepeatCount encodes a repeat count using G-Y and g-z notation.
// G-Y: 1-19, g-z: 20-400 (multiples of 20); counts above 400 chain
// multiple g-z characters.
func (z *ZebraGRF) encodeRepeatCount(count int) string {
	if count <= 0 {
		return ""
	}

	var buf []byte
	remaining := count

	for remaining >= 20 {
		twenties := min(remaining/20, 20)
		buf = append(buf, byte('g'+twenties-1))
		remaining -= twenties * 20
	}

	if remaining > 0 {
		buf = append(buf, byte('G'+remaining-1))
	}

	return string(buf)
}

// decodeAsciiToHex expands GRF ASCII compressed data to a hex string.
func (z *ZebraGRF) decodeAsciiToHex(asciiData string) (string, error) {
	if z.BytesPerRow <= 0 {
		return "", fmt.Errorf("%w: %d", ErrInvalidBytesPerRow, z.BytesPerRow)
	}

	var result strings.Builder
	result.Grow(len(asciiData) * 2)

	var previousLine string
	i := 0

	for i < len(asciiData) {
		if asciiData[i] == ':' {
			if previousLine == "" {
				return "", fmt.Errorf("zebraimg: ':' at index %d has no previous line to repeat", i)
			}
			result.WriteString(previousLine)
			i++
			continue
		}

		lineData, newIndex, err := z.decodeLine(asciiData, i)
		if err != nil {
			return "", err
		}
		if newIndex <= i {
			return "", fmt.Errorf("zebraimg: decoder made no progress at index %d", i)
		}
		previousLine = lineData
		result.WriteString(lineData)
		i = newIndex
	}

	return result.String(), nil
}

// decodeLine decodes a single line of GRF ASCII data starting at startIndex.
// Returns the decoded line (at most BytesPerRow*2 hex chars), the new index
// position, and any error.
func (z *ZebraGRF) decodeLine(asciiData string, startIndex int) (string, int, error) {
	hexCharsPerRow := z.BytesPerRow * 2
	var lineData strings.Builder
	i := startIndex

	for i < len(asciiData) && lineData.Len() < hexCharsPerRow {
		c := asciiData[i]

		switch {
		case c == ':':
			// Start of next line; stop here.
			return lineData.String(), i, nil

		case isRepeatCount(c):
			// Accumulate chained repeat counts (e.g. vMB = 320 + 7).
			totalCount := 0
			for i < len(asciiData) && isRepeatCount(asciiData[i]) {
				totalCount += decodeRepeatCount(asciiData[i])
				i++
			}
			if i >= len(asciiData) {
				return "", i, fmt.Errorf("zebraimg: repeat count at index %d has no value character", i)
			}

			nextChar := asciiData[i]
			switch {
			case nextChar == ',':
				lineData.WriteString(strings.Repeat("0", hexCharsPerRow-lineData.Len()))
				return lineData.String(), i + 1, nil
			case nextChar == '!':
				lineData.WriteString(strings.Repeat("F", hexCharsPerRow-lineData.Len()))
				return lineData.String(), i + 1, nil
			case isHexDigit(nextChar):
				charsToAdd := min(totalCount, hexCharsPerRow-lineData.Len())
				lineData.WriteString(strings.Repeat(string(nextChar), charsToAdd))
				i++
			default:
				return "", i, fmt.Errorf("zebraimg: %w: %q after repeat count at index %d", ErrInvalidCharacter, nextChar, i)
			}

		case c == ',':
			lineData.WriteString(strings.Repeat("0", hexCharsPerRow-lineData.Len()))
			return lineData.String(), i + 1, nil

		case c == '!':
			lineData.WriteString(strings.Repeat("F", hexCharsPerRow-lineData.Len()))
			return lineData.String(), i + 1, nil

		case isHexDigit(c):
			lineData.WriteByte(c)
			i++

		default:
			return "", i, fmt.Errorf("zebraimg: %w: %q at index %d", ErrInvalidCharacter, c, i)
		}
	}

	return lineData.String(), i, nil
}

// isRepeatCount reports whether c is a G-Y or g-z repeat count character.
func isRepeatCount(c byte) bool {
	return (c >= 'G' && c <= 'Y') || (c >= 'g' && c <= 'z')
}

// decodeRepeatCount decodes a single repeat-count character. Only valid for
// characters accepted by isRepeatCount.
func decodeRepeatCount(c byte) int {
	switch {
	case c >= 'G' && c <= 'Y':
		return int(c) - 'G' + 1 // 1-19
	case c >= 'g' && c <= 'z':
		return (int(c) - 'g' + 1) * 20 // 20-400
	}
	return 0
}

// hexStringToImage converts a hex string to a grayscale image.
// Per the ZPL ^GF spec, bit 1 = black (Y=0), bit 0 = white (Y=255).
func (z *ZebraGRF) hexStringToImage(hexString string, width, height int) (image.Image, error) {
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("zebraimg: %w", err)
	}

	img := image.NewGray(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for byteIndex := 0; byteIndex < z.BytesPerRow; byteIndex++ {
			byteValue := data[y*z.BytesPerRow+byteIndex]
			for bitIndex := 0; bitIndex <= 7; bitIndex++ {
				x := byteIndex*8 + bitIndex
				if x < width {
					if byteValue&(1<<(7-bitIndex)) != 0 {
						img.SetGray(x, y, color.Gray{Y: 0})
					} else {
						img.SetGray(x, y, color.Gray{Y: 255})
					}
				}
			}
		}
	}

	return img, nil
}

// isHexDigit checks if a byte is a valid hexadecimal digit.
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}
