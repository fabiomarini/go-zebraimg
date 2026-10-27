package zebraimg

import (
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestDecodeAsciiToHexSpecExample(t *testing.T) {
	// Example from the Zebra GRF specification.
	const (
		ascii       = "!8I0JFJ0JFI01::JFJ0JFJ0!::!"
		expectedHex = "FFFFFFFFFFFFFFFFFFFF8000FFFF0000FFFF00018000FFFF0000FFFF00018000FFFF0000FFFF0001FFFF0000FFFF0000FFFFFFFF0000FFFF0000FFFFFFFF0000FFFF0000FFFFFFFFFFFFFFFFFFFFFFFF"
	)

	z, err := New(10)
	if err != nil {
		t.Fatal(err)
	}
	got, err := z.decodeAsciiToHex(ascii)
	if err != nil {
		t.Fatal(err)
	}
	if got != expectedHex {
		t.Errorf("decodeAsciiToHex() =\n%q\nwant\n%q", got, expectedHex)
	}
}

func TestEncodeRepeatCount(t *testing.T) {
	z := &ZebraGRF{BytesPerRow: 1}
	tests := []struct {
		count int
		want  string
	}{
		{-5, ""},
		{0, ""},
		{1, "G"},
		{19, "Y"},
		{20, "g"},
		{21, "gG"},
		{327, "vM"},
		{400, "z"},
		{401, "zG"},
		{444, "zhJ"},
	}
	for _, tt := range tests {
		if got := z.encodeRepeatCount(tt.count); got != tt.want {
			t.Errorf("encodeRepeatCount(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestDecodeRepeatCount(t *testing.T) {
	tests := []struct {
		c    byte
		want int
	}{
		{'G', 1},
		{'Y', 19},
		{'g', 20},
		{'z', 400},
	}
	for _, tt := range tests {
		if got := decodeRepeatCount(tt.c); got != tt.want {
			t.Errorf("decodeRepeatCount(%q) = %d, want %d", tt.c, got, tt.want)
		}
	}
}

func TestEncodeLine(t *testing.T) {
	z := &ZebraGRF{BytesPerRow: 200}
	tests := []struct {
		name string
		line string
		want string
	}{
		{"all zeros", strings.Repeat("0", 20), ","},
		{"all Fs", strings.Repeat("F", 20), "!"},
		{"no runs", "AB", "AB"},
		{"trailing zero run", "A000", "AI0"},
		{"trailing zeros after hex", "8000", "8I0"},
		{"long run", strings.Repeat("B", 327), "vMB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := z.encodeLine(tt.line); got != tt.want {
				t.Errorf("encodeLine(%d chars) = %q, want %q", len(tt.line), got, tt.want)
			}
		})
	}
}

func TestDecodeLine(t *testing.T) {
	tests := []struct {
		name        string
		bytesPerRow int
		ascii       string
		want        string
		wantIdx     int
	}{
		{"chained repeat counts", 200, "vMB", strings.Repeat("B", 327), 3},
		{"repeat clamped to row", 20, "z0", strings.Repeat("0", 40), 2},
		{"zeros then comma fill", 10, "I0,", strings.Repeat("0", 20), 3},
		{"Fs then bang fill", 10, "JF!", strings.Repeat("F", 20), 3},
		{"bare comma", 10, ",", strings.Repeat("0", 20), 1},
		{"bare bang", 10, "!", strings.Repeat("F", 20), 1},
		{"stops at line repeat", 10, "FF:", "FF", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z, err := New(tt.bytesPerRow)
			if err != nil {
				t.Fatal(err)
			}
			got, idx, err := z.decodeLine(tt.ascii, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("decodeLine(%q) = %q (%d chars), want %d chars", tt.ascii, got, len(got), len(tt.want))
			}
			if idx != tt.wantIdx {
				t.Errorf("decodeLine(%q) index = %d, want %d", tt.ascii, idx, tt.wantIdx)
			}
		})
	}
}

func TestDecodeLineErrors(t *testing.T) {
	tests := []struct {
		name  string
		ascii string
	}{
		{"invalid character", "Z"},
		{"invalid after repeat count", "G:"},
		{"repeat count at end of data", "v"},
		{"invalid character after hex", "0@"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			z, err := New(10)
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = z.decodeLine(tt.ascii, 0)
			if err == nil {
				t.Fatalf("decodeLine(%q) succeeded, want error", tt.ascii)
			}
		})
	}
}

func TestDecodeAsciiToHexErrors(t *testing.T) {
	t.Run("leading line repeat", func(t *testing.T) {
		z, _ := New(1)
		if _, err := z.decodeAsciiToHex(":FF"); err == nil {
			t.Error("decodeAsciiToHex(\":FF\") succeeded, want error for ':' with no previous line")
		}
	})
	t.Run("zero value struct does not hang", func(t *testing.T) {
		z := &ZebraGRF{}
		if _, err := z.decodeAsciiToHex("00"); !errors.Is(err, ErrInvalidBytesPerRow) {
			t.Errorf("decodeAsciiToHex() error = %v, want ErrInvalidBytesPerRow", err)
		}
	})
	t.Run("line repeat copies previous line", func(t *testing.T) {
		z, _ := New(1)
		got, err := z.decodeAsciiToHex("FF:")
		if err != nil {
			t.Fatal(err)
		}
		if want := "FFFF"; got != want {
			t.Errorf("decodeAsciiToHex(\"FF:\") = %q, want %q", got, want)
		}
	})
}

func TestNewValidation(t *testing.T) {
	for _, bpr := range []int{0, -1} {
		if _, err := New(bpr); !errors.Is(err, ErrInvalidBytesPerRow) {
			t.Errorf("New(%d) error = %v, want ErrInvalidBytesPerRow", bpr, err)
		}
	}
}

func TestDecodeImageZeroValueErrors(t *testing.T) {
	// Regression: a zero-value ZebraGRF used to spin forever in
	// decodeAsciiToHex (hexCharsPerRow == 0 meant no progress).
	z := &ZebraGRF{}
	if _, err := z.DecodeImage("00"); !errors.Is(err, ErrInvalidBytesPerRow) {
		t.Errorf("DecodeImage() error = %v, want ErrInvalidBytesPerRow", err)
	}
}

func TestDecodeImageErrors(t *testing.T) {
	z, _ := New(10)
	if _, err := z.DecodeImage("0"); err == nil {
		t.Error("DecodeImage(\"0\") succeeded, want truncated-data error")
	}
	if _, err := z.DecodeImage("@"); !errors.Is(err, ErrInvalidCharacter) {
		t.Errorf("DecodeImage(\"@\") error = %v, want ErrInvalidCharacter", err)
	}
}

// blackImage returns a w x h fully black image; whiteImage fully white.
func blackImage(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0
	}
	return img
}

func whiteImage(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	return img
}

func TestBitConventionMatchesZPLSpec(t *testing.T) {
	// ZPL ^GF: bit 1 = black print dot, bit 0 = white.
	// A fully black 8x1 row is byte 0xFF -> GRF "!"; fully white is 0x00 -> ",".
	z, _ := New(1)

	got, err := z.EncodeImage(blackImage(8, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "!" {
		t.Errorf("EncodeImage(black) = %q, want %q", got, "!")
	}

	got, err = z.EncodeImage(whiteImage(8, 1), false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "," {
		t.Errorf("EncodeImage(white) = %q, want %q", got, ",")
	}

	// Decoding must map back: '!' (all Fs) = black, ',' (all 0s) = white.
	img, err := z.DecodeImage("!")
	if err != nil {
		t.Fatal(err)
	}
	if y := img.At(0, 0).(color.Gray).Y; y != 0 {
		t.Errorf("DecodeImage(\"!\") pixel Y = %d, want 0 (black)", y)
	}
	img, err = z.DecodeImage(",")
	if err != nil {
		t.Fatal(err)
	}
	if y := img.At(0, 0).(color.Gray).Y; y != 255 {
		t.Errorf("DecodeImage(\",\") pixel Y = %d, want 255 (white)", y)
	}
}

func TestLumaWeights(t *testing.T) {
	// Rec. 601: R=76.2 (black), G=149.7 (white), B=29.1 (black).
	// A plain (r+g+b)/3 average would call green black; this test pins the
	// luma convention.
	tests := []struct {
		name  string
		c     color.NRGBA
		black bool
	}{
		{"red", color.NRGBA{255, 0, 0, 255}, true},
		{"green", color.NRGBA{0, 255, 0, 255}, false},
		{"blue", color.NRGBA{0, 0, 255, 255}, true},
		{"transparent", color.NRGBA{0, 0, 0, 0}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := image.NewRGBA(image.Rect(0, 0, 8, 1))
			for x := 0; x < 8; x++ {
				img.Set(x, 0, tt.c)
			}
			z, _ := New(1)
			got, err := z.EncodeImage(img, false)
			if err != nil {
				t.Fatal(err)
			}
			want := ","
			if tt.black {
				want = "!"
			}
			if got != want {
				t.Errorf("EncodeImage(%s) = %q, want %q", tt.name, got, want)
			}
		})
	}
}

func TestEncodeImageBytesPerRowMismatch(t *testing.T) {
	z, _ := New(10) // 10 bytes/row = 80px, but image is 100px wide
	if _, err := z.EncodeImage(whiteImage(100, 2), false); err == nil {
		t.Error("EncodeImage succeeded with mismatched BytesPerRow, want error")
	}
}

func TestEncodeImageSubImageBounds(t *testing.T) {
	// SubImage has non-zero bounds.Min; encoding must read from the sub-image
	// region, not from the origin of the parent.
	parent := image.NewGray(image.Rect(0, 0, 16, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 16; x++ {
			if (x+y)%2 == 0 {
				parent.SetGray(x, y, color.Gray{Y: 0})
			} else {
				parent.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	sub := parent.SubImage(image.Rect(3, 1, 11, 3))

	standalone := image.NewGray(image.Rect(0, 0, 8, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 8; x++ {
			standalone.SetGray(x, y, parent.GrayAt(3+x, 1+y))
		}
	}

	z, _ := New(1)
	gotSub, err := z.EncodeImage(sub, false)
	if err != nil {
		t.Fatal(err)
	}
	gotStandalone, err := z.EncodeImage(standalone, false)
	if err != nil {
		t.Fatal(err)
	}
	if gotSub != gotStandalone {
		t.Errorf("SubImage encoding %q != standalone encoding %q", gotSub, gotStandalone)
	}
}

func TestRoundTripPreservesPixels(t *testing.T) {
	// 9px wide forces padding bits; decoded width rounds up to 16.
	src := image.NewGray(image.Rect(0, 0, 9, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 9; x++ {
			gray := color.Gray{Y: 255}
			if x%3 == 0 {
				gray.Y = 0
			}
			src.SetGray(x, y, gray)
		}
	}

	result, err := EncodeImageToGRF(src, false)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeGRFToImage(result)
	if err != nil {
		t.Fatal(err)
	}

	if w, h := decoded.Bounds().Dx(), decoded.Bounds().Dy(); w != 16 || h != 2 {
		t.Fatalf("decoded size = %dx%d, want 16x2", w, h)
	}
	for y := 0; y < 2; y++ {
		for x := 0; x < 9; x++ {
			want := src.GrayAt(x, y).Y
			got := decoded.At(x, y).(color.Gray).Y
			if got != want {
				t.Errorf("pixel (%d,%d) = %d, want %d", x, y, got, want)
			}
		}
		// Regression: padding bits are 0 = white. The old inverted mapping
		// painted solid black bars on the right edge.
		for x := 9; x < 16; x++ {
			if got := decoded.At(x, y).(color.Gray).Y; got != 255 {
				t.Errorf("padding pixel (%d,%d) = %d, want 255 (white)", x, y, got)
			}
		}
	}
}

func TestRoundTripDitherSolidExact(t *testing.T) {
	// Solid black/white must survive Floyd-Steinberg unchanged.
	for _, tc := range []struct {
		name string
		src  *image.Gray
	}{
		{"black", blackImage(17, 3)},
		{"white", whiteImage(17, 3)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := EncodeImageToGRF(tc.src, true)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := DecodeGRFToImage(result)
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < 3; y++ {
				for x := 0; x < 17; x++ {
					if got, want := decoded.At(x, y).(color.Gray).Y, tc.src.GrayAt(x, y).Y; got != want {
						t.Fatalf("pixel (%d,%d) = %d, want %d", x, y, got, want)
					}
				}
			}
		})
	}
}

func TestEncodeHexToAsciiLineRepeat(t *testing.T) {
	z, _ := New(2) // 4 hex chars per row
	got := z.encodeHexToAscii("ABABABAB")
	if want := "ABAB:"; got != want {
		t.Errorf("encodeHexToAscii(\"ABABABAB\") = %q, want %q", got, want)
	}
	img, err := z.DecodeImage("ABAB:")
	if err != nil {
		t.Fatal(err)
	}
	if h := img.Bounds().Dy(); h != 2 {
		t.Errorf("DecodeImage(\"ABAB:\") height = %d, want 2", h)
	}
}

func TestRealImageRoundTrip(t *testing.T) {
	img, err := LoadImage("../examples/input_1.png")
	if err != nil {
		t.Fatal(err)
	}

	result, err := EncodeImageToGRF(img, true)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeGRFToImage(result)
	if err != nil {
		t.Fatal(err)
	}

	wantW := result.BytesPerRow * 8
	if w, h := decoded.Bounds().Dx(), decoded.Bounds().Dy(); w != wantW || h != result.Height {
		t.Fatalf("decoded size = %dx%d, want %dx%d", w, h, wantW, result.Height)
	}

	// Regression: padding columns (when the width is not byte-aligned)
	// must be white, not black.
	if result.Width%8 != 0 {
		lastCol := decoded.Bounds().Dx() - 1
		for y := 0; y < result.Height; y++ {
			if got := decoded.At(lastCol, y).(color.Gray).Y; got != 255 {
				t.Fatalf("padding column pixel (x=%d, y=%d) = %d, want 255 (white)", lastCol, y, got)
			}
		}
	}

	if err := SaveImage(decoded, t.TempDir()+"/decoded.png"); err != nil {
		t.Fatal(err)
	}
}
