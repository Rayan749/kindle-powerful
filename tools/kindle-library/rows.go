package main

import (
	"image"
	"image/color"
)

const separatorOffset = rowH - 30

// Book text keeps its existing position; the row fills the space between rules.
func bookRowRect(row int) image.Rectangle {
	y := listTop + row*rowH
	top := y + separatorOffset - rowH + 1
	if top < headerH {
		top = headerH
	}
	return image.Rect(0, top, screenW, y+separatorOffset)
}
func bookRowAt(y int) int {
	for row := 0; row < booksPerPage(); row++ {
		r := bookRowRect(row)
		if y >= r.Min.Y && y <= r.Max.Y {
			return row
		}
	}
	return -1
}
func bookSeparator() []byte {
	img := image.NewGray(image.Rect(0, 0, screenW-2*marginX, 1))
	for x := 0; x < img.Bounds().Dx(); x++ {
		value := uint8(255)
		if x%7 < 4 {
			value = 119
		}
		img.SetGray(x, 0, color.Gray{Y: value})
	}
	return iconPNG(img)
}
