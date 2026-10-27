# Go ZebraGRF

A Go library for encoding and decoding black-and-white images to/from Zebra GRF (Graphics Format) ASCII, used by Zebra printers.

This is a port of the original [go-zebraimg](https://github.com/fabiomarini/go-zebraimg) Kotlin library.

## Features

- **Encoding**: Convert PNG images to GRF ASCII format
- **Decoding**: Convert GRF ASCII back to images
- **Floyd-Steinberg Dithering**: Optional error diffusion for better image quality
- **Run-Length Compression**: Efficient ASCII encoding using G-Y/g-z notation

## Installation

```bash
go get github.com/fabiomarini/go-zebraimg
```

## Usage

### Basic Encoding

```go
package main

import (
    "fmt"
    "github.com/fabiomarini/go-zebraimg/pkg/go-zebraimg"
)

func main() {
    // Load an image
    img, err := go-zebraimg.LoadImage("input.png")
    if err != nil {
        panic(err)
    }

    // Encode to GRF without dithering (dither=true for Floyd-Steinberg dithering)
    result, err := go-zebraimg.EncodeImageToGRF(img, false)
    if err != nil {
        panic(err)
    }

    fmt.Printf("GRF data: %s\n", result.AsciiData)
    fmt.Printf("Dimensions: %dx%d, Bytes per row: %d\n",
        result.Width, result.Height, result.BytesPerRow)
}
```

### Decoding

```go
// Decode GRF back to image
decoded, err := go-zebraimg.DecodeGRFToImage(result)
if err != nil {
    panic(err)
}

// Save the decoded image
err = go-zebraimg.SaveImage(decoded, "output.png")
if err != nil {
    panic(err)
}
```

### Using the ZebraGRF struct directly

```go
// Create encoder with specific bytes per row
encoder := go-zebraimg.New(10)

// Encode image
grfString, err := encoder.EncodeImage(img, true)

// Decode back
decoded, err := encoder.DecodeImage(grfString)
```

## GRF Format

The GRF format uses run-length encoding with the following special characters:

| Character | Meaning |
|-----------|---------|
| `G`-`Y` | Repeat count 1-19 |
| `g`-`z` | Repeat count 20-400 (multiples of 20) |
| `:` | Repeat previous line |
| `,` | Fill rest with 0s (black) |
| `!` | Fill rest with Fs (white) |

## Testing

Run the built-in tests:

```bash
go run main.go
```

This runs three test suites:
1. Specification example encoding/decoding
2. Edge cases (repeat counts, fills, line limits)
3. Real image round-trip test

## Notes

- **Dithering**: The `dither` parameter in `EncodeImageToGRF` controls whether to use Floyd-Steinberg dithering. Setting `dither=false` produces a more accurate pixel-for-pixel representation, while `dither=true` applies error diffusion which may improve visual quality for grayscale images but can cause some color inversion in borderline cases.

- **GRF Output**: The ASCII output uses the Zebra GRF run-length compression format. For best results, use black-and-white images.

## License

MIT License