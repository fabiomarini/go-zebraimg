// Command zebraimg converts PNG or JPEG images to Zebra GRF ASCII.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fabiomarini/go-zebraimg/zebraimg"
)

const usage = `usage: zebraimg [-dither] [-o out.grf] image...

Converts PNG/JPEG images to Zebra GRF ASCII (bit 1 = black print dot).

  -dither   apply Floyd-Steinberg dithering (default true; use
            -dither=false for crisp black-and-white artwork)
  -o path   output file, "-" for stdout; default: <input>.grf.
            Only valid with a single input image.

Status 0 on success, 1 on conversion error, 2 on bad usage.`

func main() {
	dither := flag.Bool("dither", true, "apply Floyd-Steinberg dithering")
	out := flag.String("o", "", "output file (\"-\" for stdout); default <input>.grf")
	flag.Usage = func() { fmt.Fprintln(flag.CommandLine.Output(), usage) }
	flag.Parse()

	inputs := flag.Args()
	if len(inputs) == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if *out != "" && len(inputs) > 1 {
		fmt.Fprintln(os.Stderr, "zebraimg: -o requires exactly one input image")
		os.Exit(2)
	}

	for _, in := range inputs {
		if err := convert(in, *out, *dither); err != nil {
			fmt.Fprintf(os.Stderr, "zebraimg: %v\n", err)
			os.Exit(1)
		}
	}
}

func convert(in, out string, dither bool) error {
	img, err := zebraimg.LoadImage(in)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}

	result, err := zebraimg.EncodeImageToGRF(img, dither)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}

	if out == "-" {
		_, err := os.Stdout.WriteString(result.AsciiData)
		return err
	}
	if out == "" {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ".grf"
	}

	if err := os.WriteFile(out, []byte(result.AsciiData), 0o644); err != nil {
		return fmt.Errorf("%s: %w", out, err)
	}
	fmt.Fprintf(os.Stderr, "%s -> %s (%dx%d, %d B/row, %d bytes GRF)\n",
		in, out, result.Width, result.Height, result.BytesPerRow, len(result.AsciiData))
	return nil
}
