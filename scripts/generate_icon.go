//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

// ICONDIR represents the ICO file header
type ICONDIR struct {
	Reserved uint16 // Must be 0
	Type     uint16 // 1 for ICO
	Count    uint16 // Number of images
}

// ICONDIRENTRY represents each image entry in the ICO file
type ICONDIRENTRY struct {
	Width      byte   // Width (0 = 256)
	Height     byte   // Height (0 = 256)
	ColorCount byte   // 0 for truecolor
	Reserved   byte   // 0
	Planes     uint16 // 1
	BitCount   uint16 // 32
	BytesInRes uint32 // Length of image data
	ImageOffset uint32 // Offset in file
}

// resizeBilinear resizes an image to specified dimensions with high quality
func resizeBilinear(src image.Image, targetWidth, targetHeight int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	xRatio := float64(srcW) / float64(targetWidth)
	yRatio := float64(srcH) / float64(targetHeight)

	for y := 0; y < targetHeight; y++ {
		srcY := (float64(y) + 0.5)*yRatio - 0.5
		y0 := int(math.Floor(srcY))
		y1 := y0 + 1
		yWeight := srcY - float64(y0)

		if y0 < 0 {
			y0 = 0
		}
		if y1 >= srcH {
			y1 = srcH - 1
		}

		for x := 0; x < targetWidth; x++ {
			srcX := (float64(x) + 0.5)*xRatio - 0.5
			x0 := int(math.Floor(srcX))
			x1 := x0 + 1
			xWeight := srcX - float64(x0)

			if x0 < 0 {
				x0 = 0
			}
			if x1 >= srcW {
				x1 = srcW - 1
			}

			c00 := colorRGBA(src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y0))
			c10 := colorRGBA(src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y0))
			c01 := colorRGBA(src.At(srcBounds.Min.X+x0, srcBounds.Min.Y+y1))
			c11 := colorRGBA(src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y1))

			// Interpolate horizontally
			topR := lerp(float64(c00.R), float64(c10.R), xWeight)
			topG := lerp(float64(c00.G), float64(c10.G), xWeight)
			topB := lerp(float64(c00.B), float64(c10.B), xWeight)
			topA := lerp(float64(c00.A), float64(c10.A), xWeight)

			botR := lerp(float64(c01.R), float64(c11.R), xWeight)
			botG := lerp(float64(c01.G), float64(c11.G), xWeight)
			botB := lerp(float64(c01.B), float64(c11.B), xWeight)
			botA := lerp(float64(c01.A), float64(c11.A), xWeight)

			// Interpolate vertically
			finalR := uint8(math.Round(lerp(topR, botR, yWeight)))
			finalG := uint8(math.Round(lerp(topG, botG, yWeight)))
			finalB := uint8(math.Round(lerp(topB, botB, yWeight)))
			finalA := uint8(math.Round(lerp(topA, botA, yWeight)))

			dst.SetRGBA(x, y, color.RGBA{R: finalR, G: finalG, B: finalB, A: finalA})
		}
	}

	return dst
}

func colorRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

func lerp(a, b float64, t float64) float64 {
	return a*(1.0-t) + b*t
}

func main() {
	masterPath := filepath.Join("build", "icon_master.jpg")
	f, err := os.Open(masterPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening master icon %s: %v\n", masterPath, err)
		os.Exit(1)
	}
	defer f.Close()

	srcImg, err := jpeg.Decode(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decoding master image: %v\n", err)
		os.Exit(1)
	}

	// 1. Save master as PNG
	pngPath := filepath.Join("build", "icon.png")
	pngFile, err := os.Create(pngPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", pngPath, err)
		os.Exit(1)
	}
	if err := png.Encode(pngFile, srcImg); err != nil {
		pngFile.Close()
		fmt.Fprintf(os.Stderr, "Error encoding %s: %v\n", pngPath, err)
		os.Exit(1)
	}
	pngFile.Close()
	fmt.Printf("Saved master PNG: %s\n", pngPath)

	// 2. Generate standard icon sizes
	sizes := []int{256, 128, 64, 48, 32, 16}
	type iconBuffer struct {
		size int
		data []byte
	}
	var encodedIcons []iconBuffer

	for _, s := range sizes {
		resized := resizeBilinear(srcImg, s, s)
		var buf bytes.Buffer
		if err := png.Encode(&buf, resized); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding %dx%d PNG: %v\n", s, s, err)
			os.Exit(1)
		}
		encodedIcons = append(encodedIcons, iconBuffer{size: s, data: buf.Bytes()})
	}

	// 3. Assemble ICO file
	icoPath := filepath.Join("build", "icon.ico")
	icoFile, err := os.Create(icoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", icoPath, err)
		os.Exit(1)
	}
	defer icoFile.Close()

	hdr := ICONDIR{
		Reserved: 0,
		Type:     1, // ICO
		Count:    uint16(len(encodedIcons)),
	}
	if err := binary.Write(icoFile, binary.LittleEndian, &hdr); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing ICO header: %v\n", err)
		os.Exit(1)
	}

	// Calculate initial offset: 6 bytes header + (16 bytes * count)
	headerSize := uint32(6 + 16*len(encodedIcons))
	currentOffset := headerSize

	for _, icon := range encodedIcons {
		w := byte(icon.size)
		h := byte(icon.size)
		if icon.size >= 256 {
			w = 0
			h = 0
		}
		entry := ICONDIRENTRY{
			Width:       w,
			Height:      h,
			ColorCount:  0,
			Reserved:    0,
			Planes:      1,
			BitCount:    32,
			BytesInRes:  uint32(len(icon.data)),
			ImageOffset: currentOffset,
		}
		if err := binary.Write(icoFile, binary.LittleEndian, &entry); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing ICO entry: %v\n", err)
			os.Exit(1)
		}
		currentOffset += uint32(len(icon.data))
	}

	for _, icon := range encodedIcons {
		if _, err := icoFile.Write(icon.data); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing ICO image payload: %v\n", err)
			os.Exit(1)
		}
	}
	fmt.Printf("Generated multi-resolution ICO: %s (%d sizes)\n", icoPath, len(sizes))

	// 4. Generate .syso files for Windows (amd64 and arm64) using pinned go tool rsrc
	for _, arch := range []string{"amd64", "arm64"} {
		sysoName := fmt.Sprintf("rsrc_windows_%s.syso", arch)
		cmd := exec.Command("go", "tool", "rsrc", "-ico", icoPath, "-arch", arch, "-o", sysoName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating %s with go tool rsrc: %v\nOutput: %s\n", sysoName, err, string(out))
			os.Exit(1)
		}
		fmt.Printf("Generated Windows resource COFF: %s (arch=%s)\n", sysoName, arch)
	}

	fmt.Println("Successfully generated icons and Windows syso resources!")
}
