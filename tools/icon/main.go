// Render the repository's geometric bank icon without external imaging tools.
package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, 256, 256))
	bg := color.NRGBA{38, 48, 56, 255}
	gold := color.NRGBA{198, 164, 106, 255}
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			dx, dy := max(12-x, 0, x-243), max(12-y, 0, y-243)
			if dx*dx+dy*dy < 144 {
				img.SetNRGBA(x, y, bg)
			}
		}
	}
	line := func(x1, y1, x2, y2 int) {
		for y := 0; y < 256; y++ {
			for x := 0; x < 256; x++ {
				vx, vy := float64(x2-x1), float64(y2-y1)
				t := (float64(x-x1)*vx + float64(y-y1)*vy) / (vx*vx + vy*vy)
				t = max(0, min(1, t))
				dx, dy := float64(x-x1)-t*vx, float64(y-y1)-t*vy
				if dx*dx+dy*dy <= 64 {
					img.SetNRGBA(x, y, gold)
				}
			}
		}
	}
	line(56, 104, 128, 60)
	line(128, 60, 200, 104)
	line(56, 104, 200, 104)
	for _, x := range []int{76, 128, 180} {
		line(x, 124, x, 188)
	}
	line(56, 204, 200, 204)
	var data bytes.Buffer
	png.Encode(&data, img)
	os.WriteFile("web/app.png", data.Bytes(), 0644)
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, []uint16{0, 1, 1})
	ico.Write([]byte{0, 0, 0, 0})
	binary.Write(&ico, binary.LittleEndian, []uint16{1, 32})
	binary.Write(&ico, binary.LittleEndian, uint32(data.Len()))
	binary.Write(&ico, binary.LittleEndian, uint32(22))
	ico.Write(data.Bytes())
	os.WriteFile("web/app.ico", ico.Bytes(), 0644)
}
