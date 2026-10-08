// Generates an AMD64 COFF resource object using only the Go standard library.
// Run from the repository root: go run ./tools/resources
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

var le = binary.LittleEndian

func put16(b []byte, p int, v uint16) { le.PutUint16(b[p:], v) }
func put32(b []byte, p int, v uint32) { le.PutUint32(b[p:], v) }
func iconBMP() []byte {
	b := make([]byte, 40+32*32*4+128)
	put32(b, 0, 40)
	put32(b, 4, 32)
	put32(b, 8, 64)
	put16(b, 12, 1)
	put16(b, 14, 32)
	put32(b, 20, 32*32*4)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			i := 40 + ((31-y)*32+x)*4
			blue, green, red := byte(145), byte(175), byte(9)
			if x >= 5 && x <= 26 && y >= 9 && y <= 23 {
				blue, green, red = 230, 255, 240
				if x > 6 && x < 25 && y > 10 && y < 22 {
					blue, green, red = 100, 140, 0
					if ((y == 13 || y == 16) && x%4 == 0) || (y == 19 && x > 10 && x < 21) {
						blue, green, red = 230, 255, 240
					}
				}
			}
			b[i], b[i+1], b[i+2], b[i+3] = blue, green, red, 255
		}
	}
	return b
}
func main() {
	manifest, e := os.ReadFile("assets/app.manifest")
	if e != nil {
		panic(e)
	}
	bmp := iconBMP()
	group := make([]byte, 20)
	put16(group, 2, 1)
	put16(group, 4, 1)
	group[6], group[7] = 32, 32
	put16(group, 10, 1)
	put16(group, 12, 32)
	put32(group, 14, uint32(len(bmp)))
	put16(group, 18, 1)
	ico := make([]byte, 22)
	put16(ico, 2, 1)
	put16(ico, 4, 1)
	ico[6], ico[7] = 32, 32
	put16(ico, 10, 1)
	put16(ico, 12, 32)
	put32(ico, 14, uint32(len(bmp)))
	put32(ico, 18, 22)
	ico = append(ico, bmp...)
	os.MkdirAll("assets/icons", 0755)
	must(os.WriteFile("assets/icons/cleanpause.ico", ico, 0644))
	// Store tiles use the same generated keyboard icon as the executable.
	for name, size := range map[string]int{"Square44x44Logo": 44, "Square150x150Logo": 150, "StoreLogo": 50} {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				i := 40 + ((31-y*32/size)*32+x*32/size)*4
				img.SetNRGBA(x, y, color.NRGBA{bmp[i+2], bmp[i+1], bmp[i], 255})
			}
		}
		var encoded bytes.Buffer
		must(png.Encode(&encoded, img))
		must(os.WriteFile("assets/icons/"+name+".png", encoded.Bytes(), 0644))
	}
	blobs := [][]byte{bmp, group, manifest}
	types := []uint32{3, 14, 24}
	// Resource hierarchy: type -> integer ID 1 -> language 1033 -> data entry.
	const rootSize = 40
	const branchSize = 48
	const dataEntries = rootSize + 3*branchSize
	resource := make([]byte, dataEntries+3*16)
	put16(resource, 14, 3)
	var relocOffsets []uint32
	for i, blob := range blobs {
		branch := rootSize + i*branchSize
		put32(resource, 16+i*8, types[i])
		put32(resource, 20+i*8, uint32(branch)|0x80000000)
		put16(resource, branch+14, 1)
		put32(resource, branch+16, 1)
		put32(resource, branch+20, uint32(branch+24)|0x80000000)
		put16(resource, branch+38, 1)
		put32(resource, branch+40, 1033)
		entry := dataEntries + i*16
		put32(resource, branch+44, uint32(entry))
		for len(resource)%4 != 0 {
			resource = append(resource, 0)
		}
		offset := len(resource)
		put32(resource, entry, uint32(offset))
		put32(resource, entry+4, uint32(len(blob)))
		resource = append(resource, blob...)
		relocOffsets = append(relocOffsets, uint32(entry))
	}
	out := make([]byte, 60)
	put16(out, 0, 0x8664)
	put16(out, 2, 1)
	put32(out, 8, uint32(60+len(resource)+30))
	put32(out, 12, 1)
	copy(out[20:28], ".rsrc")
	put32(out, 36, uint32(len(resource)))
	put32(out, 40, 60)
	put32(out, 44, uint32(60+len(resource)))
	put16(out, 52, 3)
	put32(out, 56, 0x40000040)
	out = append(out, resource...)
	for _, offset := range relocOffsets {
		r := make([]byte, 10)
		put32(r, 0, offset)
		put16(r, 8, 3)
		out = append(out, r...)
	}
	symbol := make([]byte, 18)
	copy(symbol, ".rsrc")
	put16(symbol, 12, 1)
	symbol[16] = 3
	out = append(out, symbol...)
	out = append(out, 4, 0, 0, 0)
	if !bytes.Equal(out[20:25], []byte(".rsrc")) {
		panic("invalid COFF")
	}
	must(os.WriteFile("cmd/cleanpause/resources_windows_amd64.syso", out, 0644))
	fmt.Println("Generated embedded manifest and icon")
}
func must(e error) {
	if e != nil {
		panic(e)
	}
}
