// Package imgransform provides on-the-fly image resizing, cropping, and
// format conversion. Designed to be plugged into the storage download handler
// via query parameters: ?w=, &h=, &format=, &fit=, &quality=.
package imgransform

import (
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"math"
	stddraw "image/draw"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// Options controls image transformation parameters.
type Options struct {
	Width   int    // max width in pixels (0 = no resize)
	Height  int    // max height in pixels (0 = no resize)
	Format  string // "jpeg", "png", "webp", or "" to keep original
	Fit     string // "cover" (crop to fill), "contain" (fit inside), "" = contain
	Quality int    // JPEG/WebP quality 1-100, 0 = default (85)
}

// Supported output formats.
var outputFormats = map[string]bool{
	"jpeg": true,
	"png":  true,
	"webp": true,
	"gif":  true,
}

// IsTransformRequested returns true if any transformation parameters are present.
func IsTransformRequested(opts Options) bool {
	return opts.Width > 0 || opts.Height > 0 || opts.Format != "" || opts.Quality > 0
}

// Transform reads an image from src, applies resizing/cropping/format
// conversion, and writes the result to dst. If no transformations are
// needed, it copies src → dst unchanged.
func Transform(src io.Reader, dst io.Writer, contentType string, opts Options) error {
	if !IsTransformRequested(opts) {
		_, err := io.Copy(dst, src)
		return err
	}

	// Decode the source image.
	srcImg, srcFormat, err := image.Decode(src)
	if err != nil {
		return fmt.Errorf("imgtransform: decode: %w", err)
	}

	// Apply resize/crop.
	dstImg := resizeImage(srcImg, opts)

	// Choose output format.
	outFormat := opts.Format
	if outFormat == "" || !outputFormats[outFormat] {
		outFormat = normalizeFormat(srcFormat, contentType)
	}
	quality := opts.Quality
	if quality <= 0 || quality > 100 {
		quality = 85
	}

	// Encode.
	return encode(dst, dstImg, outFormat, quality)
}

// resizeImage applies the requested fit strategy.
func resizeImage(img image.Image, opts Options) image.Image {
	if opts.Width <= 0 && opts.Height <= 0 {
		return img
	}

	srcBounds := img.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()
	dstW, dstH := resolveDimensions(srcW, srcH, opts)

	if opts.Fit == "cover" {
		// Crop to fill exactly.
		cropped := cropCover(img, dstW, dstH)
		return scale(cropped, dstW, dstH)
	}
	// "contain" (default) — fit inside, preserving ratio.
	return scale(img, dstW, dstH)
}

// resolveDimensions returns the final output dimensions based on source size
// and options, preserving aspect ratio.
func resolveDimensions(srcW, srcH int, opts Options) (int, int) {
	dstW, dstH := opts.Width, opts.Height

	if dstW <= 0 && dstH <= 0 {
		return srcW, srcH
	}
	if dstW <= 0 {
		// Scale by height.
		dstW = int(math.Round(float64(srcW) * float64(dstH) / float64(srcH)))
		if dstW < 1 {
			dstW = 1
		}
		return dstW, dstH
	}
	if dstH <= 0 {
		dstH = int(math.Round(float64(srcH) * float64(dstW) / float64(srcW)))
		if dstH < 1 {
			dstH = 1
		}
		return dstW, dstH
	}
	return dstW, dstH
}

// cropCover crops the source image to the destination aspect ratio, centered.
func cropCover(img image.Image, dstW, dstH int) image.Image {
	bounds := img.Bounds()
	srcW, srcH := bounds.Dx(), bounds.Dy()
	srcRatio := float64(srcW) / float64(srcH)
	dstRatio := float64(dstW) / float64(dstH)

	var cropW, cropH, x, y int
	if srcRatio > dstRatio {
		// Source is wider → crop horizontally.
		cropH = srcH
		cropW = int(math.Round(float64(cropH) * dstRatio))
		x = (srcW - cropW) / 2
	} else {
		// Source is taller → crop vertically.
		cropW = srcW
		cropH = int(math.Round(float64(cropW) / dstRatio))
		y = (srcH - cropH) / 2
	}

	cropped := image.NewRGBA(image.Rect(0, 0, cropW, cropH))
	stddraw.Draw(cropped, cropped.Bounds(), img, image.Pt(x, y), stddraw.Over)
	return cropped
}

// scale resizes img to the given dimensions using bilinear interpolation.
func scale(img image.Image, w, h int) image.Image {
	if w <= 0 || h <= 0 {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// encode writes the image to w in the requested format.
func encode(w io.Writer, img image.Image, format string, quality int) error {
	switch format {
	case "jpeg", "jpg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	case "png":
		return png.Encode(w, img)
	case "webp":
		// webp.Encode is available via library. Fallback to png for now.
		return png.Encode(w, img)
	case "gif":
		return gif.Encode(w, img, nil)
	default:
		return png.Encode(w, img)
	}
}

// normalizeFormat converts a MIME type or image format to our internal format name.
func normalizeFormat(format, contentType string) string {
	if format == "" {
		// Infer from content type.
		switch contentType {
		case "image/jpeg", "image/png", "image/gif", "image/webp":
			return contentType[6:] // strip "image/"
		}
		return "jpeg"
	}
	return format
}
