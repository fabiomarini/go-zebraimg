package main

import (
	"fmt"
	"strings"

	"github.com/fabiomarini/go-zebraimg/pkg/gozebraimg"
)

func main() {
	testEncodingDecoding()
	fmt.Println("\n" + repeatString("=", 80) + "\n")
	testEdgeCases()
	fmt.Println("\n" + repeatString("=", 80) + "\n")
	testRealImage()
}

func testEncodingDecoding() {
	fmt.Println("Testing encoding/decoding with example from specification...")

	expectedHex := "FFFFFFFFFFFFFFFFFFFF8000FFFF0000FFFF00018000FFFF0000FFFF00018000FFFF0000FFFF0001FFFF0000FFFF0000FFFFFFFF0000FFFF0000FFFFFFFF0000FFFF0000FFFFFFFFFFFFFFFFFFFFFFFF"
	asciiEncoded := "!8I0JFJ0JFI01::JFJ0JFJ0!::!"

	// The hex string has 140 characters = 70 bytes, divided by 7 lines = 10 bytes per line = 20 hex chars per line
	hexCharsPerRow := 20
	bytesPerRow := hexCharsPerRow / 2

	encoder := gozebraimg.New(bytesPerRow)
	decodedHex, err := encoder.DecodeAsciiToHex(asciiEncoded)
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
		return
	}

	fmt.Printf("Expected hex: %s\n", expectedHex)
	fmt.Printf("Decoded hex:  %s\n", decodedHex)
	fmt.Printf("Match: %v\n", expectedHex == decodedHex)
	fmt.Printf("Expected length: %d, Decoded length: %d\n", len(expectedHex), len(decodedHex))
}

func testEdgeCases() {
	fmt.Println("Testing edge cases...")

	// Test 1: Multiple repeat counts (vMB = 340 + 7 = 347 B's)
	encoder := gozebraimg.New(200) // Large enough line

	// Test vMB encoding
	testHex1 := repeatString("B", 327)
	encoded1 := encoder.EncodeLine(testHex1)
	fmt.Println("\nTest 1 - Multiple repeat counts:")
	fmt.Printf("Input: 327 B's\n")
	fmt.Printf("Encoded: %s\n", encoded1)
	fmt.Printf("Expected: vMB (or MvB)\n")

	decodeLine1, _, err := encoder.DecodeLine("vMB", 0, 400)
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
	} else {
		fmt.Printf("Decoded vMB: %d chars (expected 327)\n", len(decodeLine1))
		fmt.Printf("Match: %v\n", decodeLine1 == testHex1)
	}

	// Test 2: Line length limit
	fmt.Println("\nTest 2 - Line length limit:")
	testAscii := "z0"                                        // 400 zeros
	decoded2, _, err := encoder.DecodeLine(testAscii, 0, 40) // Limit to 40 chars
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
	} else {
		fmt.Printf("Encoded: z0 (400 zeros)\n")
		fmt.Printf("With limit of 40: decoded %d chars\n", len(decoded2))
		fmt.Printf("Match: %v\n", decoded2 == repeatString("0", 40))
	}

	// Test 3: Comma fill
	fmt.Println("\nTest 3 - Comma fill:")
	decoded3, _, err := encoder.DecodeLine("I0,", 0, 20)
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
	} else {
		fmt.Printf("Encoded: I0, (3 zeros then fill rest with 0)\n")
		fmt.Printf("Decoded length: %d (expected 20)\n", len(decoded3))
		fmt.Printf("Decoded: %s\n", decoded3)
	}

	// Test 4: Exclamation fill
	fmt.Println("\nTest 4 - Exclamation fill:")
	decoded4, _, err := encoder.DecodeLine("JF!", 0, 20)
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
	} else {
		fmt.Printf("Encoded: JF! (4 F's then fill rest with F)\n")
		fmt.Printf("Decoded length: %d (expected 20)\n", len(decoded4))
		fmt.Printf("Decoded: %s\n", decoded4)
	}
}

func testRealImage() {
	img, err := gozebraimg.LoadImage("examples/input.png")
	if err != nil {
		fmt.Printf("Error loading image: %v\n", err)
		return
	}

	result, err := gozebraimg.EncodeImageToGRF(img, true) // false = no dithering for accurate reproduction
	if err != nil {
		fmt.Printf("Error encoding: %v\n", err)
		return
	}

	fmt.Println("\nTest 5 - Real image encoding:")
	fmt.Printf("Original image size: %dx%d\n", result.Width, result.Height)
	fmt.Printf("Encoded GRF size: %d bytes\n", len(result.AsciiData))
	fmt.Printf("Decoded image size: %dx%d\n", result.Width, result.Height)

	decoded, err := gozebraimg.DecodeGRFToImage(result)
	if err != nil {
		fmt.Printf("Error decoding: %v\n", err)
		return
	}

	fmt.Println("\nTest 5 - decoded image encoding:")
	fmt.Printf("Original image size: %dx%d\n", result.Width, result.Height)
	fmt.Printf("Decoded width: %d\n", decoded.Bounds().Dx())
	fmt.Printf("Decoded height: %d\n", decoded.Bounds().Dy())

	// Write to file
	err = gozebraimg.SaveImage(decoded, "examples/output.png")
	if err != nil {
		fmt.Printf("Error saving image: %v\n", err)
		return
	}
	fmt.Println("Saved to examples/output.png")
}

// repeatString repeats a string n times using pre-allocated strings.Builder
func repeatString(s string, n int) string {
	var builder strings.Builder
	builder.Grow(len(s) * n)
	for i := 0; i < n; i++ {
		builder.WriteString(s)
	}
	return builder.String()
}
