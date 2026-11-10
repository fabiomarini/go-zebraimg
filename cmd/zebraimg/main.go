// Command zebraimg converts PNG or JPEG images to Zebra GRF ASCII,
// emitting a full ^GF command by default.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fabiomarini/go-zebraimg/zebraimg"
)

const usage = `usage: zebraimg [flags] image...

Converts PNG/JPEG images to Zebra GRF ASCII (bit 1 = black print dot),
wrapped in a full ^GFA,b,c,d,data^FS command.

  -width n    resize to n pixels wide (aspect preserved unless -height set)
  -height n   resize to n pixels tall (aspect preserved unless -width set)
  -dither     apply Floyd-Steinberg dithering (default true; use
              -dither=false for crisp black-and-white artwork)
  -raw        output bare GRF ASCII instead of the ^GF command
  -o path     output file, "-" for stdout; default: <input>.zpl
              (.grf with -raw). Only valid with a single input image.

Status 0 on success, 1 on conversion error, 2 on bad usage.`

type options struct {
	dither bool
	raw    bool
	width  int
	height int
	out    string
}

func main() {
	var o options
	flag.BoolVar(&o.dither, "dither", true, "apply Floyd-Steinberg dithering")
	flag.BoolVar(&o.raw, "raw", false, "output bare GRF ASCII instead of a ^GF command")
	flag.IntVar(&o.width, "width", 0, "resize to width in pixels (0 = no resize)")
	flag.IntVar(&o.height, "height", 0, "resize to height in pixels (0 = no resize)")
	flag.StringVar(&o.out, "o", "", `output file ("-" for stdout); default <input>.zpl`)
	flag.Usage = func() { fmt.Fprintln(flag.CommandLine.Output(), usage) }
	flag.Parse()

	inputs := flag.Args()
	if len(inputs) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if o.out != "" && len(inputs) > 1 {
		fmt.Fprintln(os.Stderr, "zebraimg: -o requires exactly one input image")
		os.Exit(2)
	}
	if o.width < 0 || o.height < 0 {
		fmt.Fprintln(os.Stderr, "zebraimg: -width and -height must be positive")
		os.Exit(2)
	}

	for _, in := range inputs {
		if err := convert(in, o); err != nil {
			fmt.Fprintf(os.Stderr, "zebraimg: %v\n", err)
			os.Exit(1)
		}
	}
}

func convert(in string, o options) error {
	img, err := zebraimg.LoadImage(in)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}

	if o.width > 0 || o.height > 0 {
		img, err = zebraimg.ResizeImage(img, o.width, o.height)
		if err != nil {
			return fmt.Errorf("%s: %w", in, err)
		}
	}

	result, err := zebraimg.EncodeImageToGRF(img, o.dither)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}

	data, ext := result.AsciiData, ".grf"
	if !o.raw {
		data, ext = result.ZPL(), ".zpl"
	}

	if o.out == "-" {
		_, err := os.Stdout.WriteString(data)
		return err
	}
	out := o.out
	if out == "" {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ext
	}

	if err := os.WriteFile(out, []byte(data), 0o644); err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	fmt.Fprintf(os.Stderr, "%s -> %s (%dx%d, %d B/row, %d bytes)\n",
		in, out, result.Width, result.Height, result.BytesPerRow, len(data))
	return nil
}
