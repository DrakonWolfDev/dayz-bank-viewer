// Render one geometric wolf-coin design to SVG, PNG and Windows icon assets.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
)

type point struct{ x, y float64 }
type shape struct {
	svg    string
	ink    color.NRGBA
	inside func(float64, float64) bool
}

var shapes []shape

func rgb(hex string) color.NRGBA {
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.NRGBA{r, g, b, 255}
}
func polygon(fill string, vertices ...point) {
	var coordinates []string
	for _, p := range vertices {
		coordinates = append(coordinates, fmt.Sprintf("%g,%g", p.x, p.y))
	}
	shapes = append(shapes, shape{`<polygon fill="` + fill + `" points="` + strings.Join(coordinates, " ") + `"/>`, rgb(fill), func(x, y float64) bool {
		hit := false
		j := len(vertices) - 1
		for i, p := range vertices {
			q := vertices[j]
			if (p.y > y) != (q.y > y) && x < (q.x-p.x)*(y-p.y)/(q.y-p.y)+p.x {
				hit = !hit
			}
			j = i
		}
		return hit
	}})
}
func ring(cx, cy, r, width float64, fill string) {
	shapes = append(shapes, shape{fmt.Sprintf(`<circle cx="%g" cy="%g" r="%g" fill="none" stroke="%s" stroke-width="%g"/>`, cx, cy, r, fill, width), rgb(fill), func(x, y float64) bool { return math.Abs(math.Hypot(x-cx, y-cy)-r) <= width/2 }})
}
func design() {
	const dark = "#263038"
	const gold = "#c6a46a"
	const light = "#e8d5ad"
	shapes = append(shapes, shape{`<rect x="1" y="1" width="62" height="62" rx="15" fill="` + dark + `"/>`, rgb(dark), func(x, y float64) bool {
		if x < 1 || x > 63 || y < 1 || y > 63 {
			return false
		}
		dx := math.Max(16-x, math.Max(x-48, 0))
		dy := math.Max(16-y, math.Max(y-48, 0))
		return dx*dx+dy*dy <= 225
	}})
	// A minted rim, with four edge cuts; no building, columns or currency glyph.
	ring(32, 32, 26, 2, gold)
	for _, v := range [][]point{
		{{31, 5}, {33, 5}, {33, 10}, {31, 10}}, {{31, 54}, {33, 54}, {33, 59}, {31, 59}},
		{{5, 31}, {10, 31}, {10, 33}, {5, 33}}, {{54, 31}, {59, 31}, {59, 33}, {54, 33}},
	} {
		polygon(gold, v...)
	}
	// Broad cheek ruffs, upright ears and a tapered muzzle form the wolf silhouette.
	polygon(gold, point{18, 15}, point{28, 22}, point{36, 22}, point{46, 15}, point{44, 31}, point{48, 37}, point{41, 43}, point{38, 44}, point{32, 51}, point{26, 44}, point{23, 43}, point{16, 37}, point{20, 31})
	// Inner ears and descending eye cuts stay clear at favicon sizes.
	polygon(dark, point{21, 20}, point{26, 24}, point{22, 28})
	polygon(dark, point{43, 20}, point{38, 24}, point{42, 28})
	polygon(dark, point{21, 30}, point{29, 33}, point{27, 36}, point{23, 34})
	polygon(dark, point{43, 30}, point{35, 33}, point{37, 36}, point{41, 34})
	// The central highlight reads as the bridge of the muzzle, not lettering.
	polygon(light, point{32, 28}, point{36, 39}, point{32, 45}, point{28, 39})
	polygon(dark, point{28, 40}, point{36, 40}, point{32, 44})
}
func render(size int) *image.NRGBA {
	const samples = 4
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var red, green, blue, alpha uint32
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					px := (float64(x) + (float64(sx)+.5)/samples) * 64 / float64(size)
					py := (float64(y) + (float64(sy)+.5)/samples) * 64 / float64(size)
					var ink color.NRGBA
					for _, s := range shapes {
						if s.inside(px, py) {
							ink = s.ink
						}
					}
					red += uint32(ink.R) * uint32(ink.A)
					green += uint32(ink.G) * uint32(ink.A)
					blue += uint32(ink.B) * uint32(ink.A)
					alpha += uint32(ink.A)
				}
			}
			if alpha > 0 {
				img.SetNRGBA(x, y, color.NRGBA{uint8(red / alpha), uint8(green / alpha), uint8(blue / alpha), uint8(alpha / (samples * samples))})
			}
		}
	}
	return img
}
func generate() error {
	design()
	var svg strings.Builder
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">`)
	for _, s := range shapes {
		svg.WriteString(s.svg)
	}
	svg.WriteString("</svg>\n")
	if err := os.WriteFile("web/favicon.svg", []byte(svg.String()), 0644); err != nil {
		return err
	}
	sizes := []int{16, 32, 48, 64, 128, 256}
	var images [][]byte
	for _, size := range sizes {
		var data bytes.Buffer
		if err := png.Encode(&data, render(size)); err != nil {
			return err
		}
		images = append(images, data.Bytes())
	}
	if err := os.WriteFile("web/app.png", images[len(images)-1], 0644); err != nil {
		return err
	}
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, []uint16{0, 1, uint16(len(images))})
	offset := 6 + 16*len(images)
	for i, data := range images {
		dimension := byte(sizes[i] % 256)
		ico.Write([]byte{dimension, dimension, 0, 0})
		binary.Write(&ico, binary.LittleEndian, []uint16{1, 32})
		binary.Write(&ico, binary.LittleEndian, uint32(len(data)))
		binary.Write(&ico, binary.LittleEndian, uint32(offset))
		offset += len(data)
	}
	for _, data := range images {
		ico.Write(data)
	}
	return os.WriteFile("web/app.ico", ico.Bytes(), 0644)
}
func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
