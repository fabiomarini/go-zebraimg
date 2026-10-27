package gozebraimg

import (
	"image"
	"image/color"
	"strings"
)

// ZebraGRF is the core encoder/decoder for Zebra GRF ASCII format
type ZebraGRF struct {
	BytesPerRow int
}

// New creates a new ZebraGRF instance with the specified bytes per row
func New(bytesPerRow int) *ZebraGRF {
	return &ZebraGRF{BytesPerRow: bytesPerRow}
}

// EncodeImage converts an image.Image to GRF ASCII encoded string
// If dither is true, applies Floyd-Steinberg dithering
func (z *ZebraGRF) EncodeImage(img image.Image, dither bool) (string, error) {
	var hexData string
	var err error

	if dither {
		hexData, err = z.imageToHexString(img)
	} else {
		hexData, err = z.imageToHexBWString(img)
	}
	if err != nil {
		return "", err
	}

	return z.encodeHexToAscii(hexData), nil
}

// DecodeImage converts GRF ASCII encoded string back to image.Image
func (z *ZebraGRF) DecodeImage(asciiData string) (image.Image, error) {
	hexString, err := z.decodeAsciiToHex(asciiData)
	if err != nil {
		return nil, err
	}

	width := z.BytesPerRow * 8
	height := len(hexString) / (2 * z.BytesPerRow)

	return z.hexStringToImage(hexString, width, height)
}

// imageToHexString converts image.Image to hexadecimal string using Floyd-Steinberg dithering
func (z *ZebraGRF) imageToHexString(img image.Image) (string, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	actualBytesPerRow := (width + 7) / 8

	// Create a grayscale buffer with error diffusion (flattened to 1D slice)
	grayscale := make([]float32, height*width)

	// Convert to grayscale
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			idx := y*width + x
			// Transparent pixels (alpha=0) should be white (255)
			if a == 0 {
				grayscale[idx] = 255.0
			} else {
				// Convert from 0-65535 to 0-255: each channel / 257, then average = (r+g+b) / 771
				grayscale[idx] = float32(r+g+b) / 771.0
			}
		}
	}

	// Apply Floyd-Steinberg error diffusion dithering (flattened to 1D slice)
	binary := make([]bool, height*width)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			oldPixel := grayscale[idx]
			newPixel := float32(0)
			if oldPixel >= 128 {
				newPixel = 255
			}
			// true = white (bit will be set to 1, which maps to Y=255)
			binary[idx] = newPixel == 255

			errorVal := oldPixel - newPixel

			// Sierra Lite error distribution (smoother than Atkinson)
			//    x   2
			// 1   1   1
			// Denominator = 8
			if x+1 < width {
				grayscale[idx+1] += errorVal * 2.0 / 8.0
			}
			if y+1 < height {
				nextRowIdx := (y + 1) * width
				if x > 0 {
					grayscale[nextRowIdx+x-1] += errorVal * 1.0 / 8.0
				}
				grayscale[nextRowIdx+x] += errorVal * 1.0 / 8.0
				if x+1 < width {
					grayscale[nextRowIdx+x+1] += errorVal * 1.0 / 8.0
				}
			}
		}
	}

	// Convert binary array to hex string
	hexChars := "0123456789ABCDEF"
	expectedLen := height * actualBytesPerRow * 2
	var result strings.Builder
	result.Grow(expectedLen)

	for y := 0; y < height; y++ {
		for byteIndex := 0; byteIndex < actualBytesPerRow; byteIndex++ {
			byteValue := 0
			for bitIndex := 0; bitIndex <= 7; bitIndex++ {
				x := byteIndex*8 + bitIndex
				if x < width && binary[y*width+x] {
					byteValue |= 1 << (7 - bitIndex)
				}
			}
			result.WriteByte(hexChars[byteValue>>4])
			result.WriteByte(hexChars[byteValue&0xF])
		}
	}

	return result.String(), nil
}

// imageToHexBWString converts image.Image to hexadecimal string without dithering
// Uses simple grayscale threshold
func (z *ZebraGRF) imageToHexBWString(img image.Image) (string, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	actualBytesPerRow := (width + 7) / 8

	hexChars := "0123456789ABCDEF"
	expectedLen := height * actualBytesPerRow * 2
	var result strings.Builder
	result.Grow(expectedLen)

	for y := 0; y < height; y++ {
		for byteIndex := 0; byteIndex < actualBytesPerRow; byteIndex++ {
			byteValue := 0
			for bitIndex := 0; bitIndex <= 7; bitIndex++ {
				x := byteIndex*8 + bitIndex
				if x < width {
					r, g, b, a := img.At(x, y).RGBA()
					// Transparent pixels (alpha=0) should be white (255)
					var gray uint8
					if a == 0 {
						gray = 255
					} else {
						gray = uint8((r + g + b) / 771.0)
					}
					if gray >= 128 { // White pixel (bit=1 maps to Y=255)
						byteValue |= 1 << (7 - bitIndex)
					}
				}
			}
			result.WriteByte(hexChars[byteValue>>4])
			result.WriteByte(hexChars[byteValue&0xF])
		}
	}

	return result.String(), nil
}

// encodeHexToAscii compresses hex string to ASCII using run-length encoding
func (z *ZebraGRF) encodeHexToAscii(hexString string) string {
	// Pre-allocate: worst case is no compression (same length)
	var result strings.Builder
	result.Grow(len(hexString))
	hexCharsPerRow := z.BytesPerRow * 2

	var previousLine string
	pos := 0

	for pos < len(hexString) {
		lineEnd := pos + hexCharsPerRow
		if lineEnd > len(hexString) {
			lineEnd = len(hexString)
		}

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

// encodeLine encodes a single line of hex data with run-length compression
func (z *ZebraGRF) encodeLine(line string) string {
	var result strings.Builder
	i := 0

	for i < len(line) {
		currentChar := line[i]
		count := 1

		for i+count < len(line) && line[i+count] == currentChar {
			count++
		}

		// Check if line ends with all 0s or Fs
		if i+count == len(line) {
			if currentChar == '0' {
				if i == 0 {
					return ","
				}
				if count > 1 {
					result.WriteString(z.encodeRepeatCount(count))
				}
				return result.String() + string(currentChar)
			} else if currentChar == 'F' {
				if i == 0 {
					return "!"
				}
				if count > 1 {
					result.WriteString(z.encodeRepeatCount(count))
				}
				return result.String() + string(currentChar)
			}
		}

		if count > 1 {
			result.WriteString(z.encodeRepeatCount(count))
		}
		result.WriteString(string(currentChar))

		i += count
	}

	return result.String()
}

// encodeRepeatCount encodes repeat count using G-Y and g-z notation
// G-Y: 1-19, g-z: 20-400 (multiples of 20)
func (z *ZebraGRF) encodeRepeatCount(count int) string {
	if count <= 0 {
		return ""
	}

	result := ""
	remaining := count

	for remaining >= 20 {
		twenties := remaining / 20
		if twenties > 20 {
			twenties = 20
		}
		result += string(rune('g' + twenties - 1))
		remaining -= twenties * 20
	}

	if remaining > 0 {
		result += string(rune('G' + remaining - 1))
	}

	return result
}

// decodeAsciiToHex expands ASCII compressed format to hex string
func (z *ZebraGRF) decodeAsciiToHex(asciiData string) (string, error) {
	result := ""
	hexCharsPerRow := z.BytesPerRow * 2

	var previousLine string
	i := 0

	for i < len(asciiData) {
		char := asciiData[i]

		if char == ':' {
			result += previousLine
			i++
		} else {
			lineData, newIndex, err := z.decodeLine(asciiData, i, hexCharsPerRow)
			if err != nil {
				return "", err
			}
			previousLine = lineData
			result += lineData
			i = newIndex
		}
	}

	return result, nil
}

// decodeLine decodes a single line from ASCII data
// Returns the decoded line and the new index position
func (z *ZebraGRF) decodeLine(asciiData string, startIndex, hexCharsPerRow int) (string, int, error) {
	lineData := ""
	i := startIndex

	for i < len(asciiData) && len(lineData) < hexCharsPerRow {
		c := asciiData[i]

		switch {
		case c == ':':
			// Start of next line, stop here
			return lineData, i, nil

		case (c >= 'G' && c <= 'Y') || (c >= 'g' && c <= 'z'):
			// Repeat count
			totalCount := 0

			// Accumulate multiple repeat counts (e.g., vMB)
			for i < len(asciiData) && ((asciiData[i] >= 'G' && asciiData[i] <= 'Y') || (asciiData[i] >= 'g' && asciiData[i] <= 'z')) {
				totalCount += z.decodeRepeatCount(asciiData[i])
				i++
			}

			if i < len(asciiData) {
				nextChar := asciiData[i]

				switch {
				case nextChar == ',':
					// Fill rest with 0s
					remainingChars := hexCharsPerRow - len(lineData)
					lineData += repeatChar('0', remainingChars)
					i++
					return lineData, i, nil

				case nextChar == '!':
					// Fill rest with Fs
					remainingChars := hexCharsPerRow - len(lineData)
					lineData += repeatChar('F', remainingChars)
					i++
					return lineData, i, nil

				case isHexDigit(nextChar):
					// Repeat the hex character, but respect line limit
					charsToAdd := totalCount
					if charsToAdd > hexCharsPerRow-len(lineData) {
						charsToAdd = hexCharsPerRow - len(lineData)
					}
					lineData += repeatChar(nextChar, charsToAdd)
					i++

					// If we've filled the line, stop
					if len(lineData) >= hexCharsPerRow {
						return lineData, i, nil
					}

				default:
					// Invalid character after repeat count, skip
					i++
				}
			}

		case c == ',':
			// Fill rest with 0s
			remainingChars := hexCharsPerRow - len(lineData)
			lineData += repeatChar('0', remainingChars)
			i++
			return lineData, i, nil

		case c == '!':
			// Fill rest with Fs
			remainingChars := hexCharsPerRow - len(lineData)
			lineData += repeatChar('F', remainingChars)
			i++
			return lineData, i, nil

		case isHexDigit(c):
			// Single hex character
			lineData += string(c)
			i++

			// If we've reached the line limit, stop
			if len(lineData) >= hexCharsPerRow {
				return lineData, i, nil
			}

		default:
			// Unknown character, skip
			i++
		}
	}

	return lineData, i, nil
}

// decodeRepeatCount decodes a single repeat count character
func (z *ZebraGRF) decodeRepeatCount(c byte) int {
	if c >= 'G' && c <= 'Y' {
		return int(c) - 'G' + 1 // 1-19
	}
	if c >= 'g' && c <= 'z' {
		return (int(c) - 'g' + 1) * 20 // 20-400
	}
	return 1
}

// hexStringToImage converts hex string to image.Image
func (z *ZebraGRF) hexStringToImage(hexString string, width, height int) (image.Image, error) {
	// Create a grayscale image (we'll set pixels to black or white)
	img := image.NewGray(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for byteIndex := 0; byteIndex < z.BytesPerRow; byteIndex++ {
			hexIndex := (y*z.BytesPerRow + byteIndex) * 2
			if hexIndex+1 >= len(hexString) {
				break
			}

			hexByte := hexString[hexIndex : hexIndex+2]
			byteValue, err := hexToByte(hexByte)
			if err != nil {
				return nil, err
			}

			for bitIndex := 0; bitIndex <= 7; bitIndex++ {
				x := byteIndex*8 + bitIndex
				if x < width {
					// Bit=1 means white (Y=255), Bit=0 means black (Y=0)
					// This matches Kotlin's inverted color mapping
					isBlack := (byteValue & (1 << (7 - bitIndex))) != 0
					if isBlack {
						img.SetGray(x, y, color.Gray{Y: 255})
					} else {
						img.SetGray(x, y, color.Gray{Y: 0})
					}
				}
			}
		}
	}

	return img, nil
}

// isHexDigit checks if a byte is a valid hexadecimal digit
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}

// repeatChar creates a string with char repeated n times
func repeatChar(c byte, n int) string {
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = c
	}
	return string(result)
}

// hexToByte converts a 2-character hex string to a byte
func hexToByte(hex string) (byte, error) {
	if len(hex) != 2 {
		return 0, ErrInvalidHex
	}

	var high, low byte

	high = hexCharToByte(hex[0])
	if high == 255 {
		return 0, ErrInvalidHex
	}

	low = hexCharToByte(hex[1])
	if low == 255 {
		return 0, ErrInvalidHex
	}

	return high<<4 | low, nil
}

// hexCharToByte converts a single hex character to its byte value
// Returns 255 if the character is invalid
func hexCharToByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return 255
	}
}

// ErrInvalidHex is returned when hex string contains invalid characters
var ErrInvalidHex = &InvalidHexError{}

// InvalidHexError represents an error with invalid hex characters
type InvalidHexError struct{}

func (e *InvalidHexError) Error() string {
	return "invalid hex character"
}

// DecodeAsciiToHex is the exported wrapper for decodeAsciiToHex
func (z *ZebraGRF) DecodeAsciiToHex(asciiData string) (string, error) {
	return z.decodeAsciiToHex(asciiData)
}

// DecodeLine is the exported wrapper for decodeLine
func (z *ZebraGRF) DecodeLine(asciiData string, startIndex, hexCharsPerRow int) (string, int, error) {
	return z.decodeLine(asciiData, startIndex, hexCharsPerRow)
}

// EncodeLine is the exported wrapper for encodeLine
func (z *ZebraGRF) EncodeLine(line string) string {
	return z.encodeLine(line)
}
