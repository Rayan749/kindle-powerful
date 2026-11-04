package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	panelLeft   = 0
	panelTop    = headerH
	panelWidth  = screenW
	panelHeight = 240
	panelBottom = panelTop + panelHeight

	brightnessLevels = 24
	brightnessTrackX = 100
	brightnessTrackY = panelTop + 44
	brightnessTrackW = 760
	brightnessTrackH = 18

	panelPillW = 80
	panelPillH = 6
	panelPillX = (screenW - panelPillW) / 2
	panelPillY = panelTop + 10

	settingsRowY  = panelTop + 88
	panelIconSize = 32
	panelTextX    = marginX + 56
	panelChevronX = screenW - marginX - 20
	jobmanRowY    = settingsRowY + 56
	panelNoteY    = panelTop + 204
)

type panelAction int

const (
	panelNone panelAction = iota
	panelClose
	panelBrightness
	panelSettings
	panelJobman
)

func parseBrightnessLevel(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > brightnessLevels {
		return -1
	}
	return n
}

func brightnessFromX(x int) int {
	if x <= brightnessTrackX {
		return 0
	}
	if x >= brightnessTrackX+brightnessTrackW {
		return brightnessLevels
	}
	return ((x-brightnessTrackX)*brightnessLevels + brightnessTrackW/2) / brightnessTrackW
}

func brightnessFillWidth(level int) int {
	if level < 0 {
		level = 0
	}
	if level > brightnessLevels {
		level = brightnessLevels
	}
	return brightnessTrackW * level / brightnessLevels
}

func panelHit(x, y int) (panelAction, int) {
	if y < panelTop || y >= panelBottom {
		return panelClose, 0
	}
	if x >= panelPillX && x < panelPillX+panelPillW &&
		y >= panelPillY && y < panelPillY+panelPillH+12 {
		return panelClose, 0
	}
	if x >= brightnessTrackX && x < brightnessTrackX+brightnessTrackW &&
		y >= brightnessTrackY-12 && y < brightnessTrackY+brightnessTrackH+12 {
		return panelBrightness, brightnessFromX(x)
	}
	if y >= settingsRowY && y < settingsRowY+56 {
		return panelSettings, 0
	}
	if y >= jobmanRowY && y < jobmanRowY+56 {
		return panelJobman, 0
	}
	return panelNone, 0
}

func levelText(level int) string {
	if level < 0 {
		return "--"
	}
	return fmt.Sprintf("%d/%d", level, brightnessLevels)
}

func knobPosition(fill int) int {
	x := brightnessTrackX + fill - 6
	if x < brightnessTrackX {
		x = brightnessTrackX
	}
	return x
}

func panelOps(level int) []fbinkOp {
	fill := brightnessFillWidth(level)
	knobX := knobPosition(fill)

	ops := []fbinkOp{
		{Kind: opCls, Left: panelLeft, Top: panelTop, Width: panelWidth, Height: panelHeight},
		{Kind: opCls, Left: panelPillX, Top: panelPillY, Width: panelPillW, Height: panelPillH, Color: "GRAY5"},
	}
	if imageIcons {
		ops = append(ops, fbinkOp{Kind: opImage, X: marginX, Y: panelTop + 36, Image: iconPNG(sunIcon())})
	}
	ops = append(ops,
		fbinkOp{Kind: opCls, Left: brightnessTrackX, Top: brightnessTrackY, Width: brightnessTrackW, Height: brightnessTrackH, Color: "GRAY3"},
	)
	if fill > 0 {
		ops = append(ops, fbinkOp{
			Kind: opCls, Left: brightnessTrackX, Top: brightnessTrackY,
			Width: fill, Height: brightnessTrackH, Color: "WHITE",
		})
	}
	ops = append(ops,
		fbinkOp{
			Kind: opCls, Left: knobX, Top: brightnessTrackY - 4,
			Width: 12, Height: brightnessTrackH + 8, Color: "WHITE",
		},
		textOp(textPlace{X: 880, Y: panelTop + 36, Text: levelText(level)}, fontTitle),
		fbinkOp{Kind: opCls, Left: marginX, Top: settingsRowY, Width: screenW - 2*marginX, Height: 2, Color: "GRAY4"},
	)
	if imageIcons {
		ops = append(ops, fbinkOp{Kind: opImage, X: marginX, Y: settingsRowY + 12, Image: iconPNG(gearIcon())})
	}
	ops = append(ops, textOp(textPlace{X: panelTextX, Y: settingsRowY + 12, Text: "设置"}, fontTitle))
	if imageIcons {
		ops = append(ops, fbinkOp{Kind: opImage, X: panelChevronX, Y: settingsRowY + 18, Image: iconPNG(chevronIcon())})
	} else {
		ops = append(ops, textOp(textPlace{X: panelChevronX, Y: settingsRowY + 12, Text: ">"}, fontBold))
	}
	ops = append(ops, textOp(textPlace{X: panelTextX, Y: jobmanRowY + 12, Text: "越狱管理"}, fontTitle))
	ops = append(ops,
		fbinkOp{
			Kind: opText, X: marginX, Y: panelNoteY,
			Text: "未注册设备：商店已禁用", Font: fontTitle, Px: 20, Invert: true,
		},
		fbinkOp{Kind: opCls, Left: marginX, Top: panelBottom - 4, Width: screenW - 2*marginX, Height: 2, Color: "GRAY4"},
		fbinkOp{Kind: opRefresh, Left: panelLeft, Top: panelTop, Width: panelWidth, Height: panelHeight},
	)
	return ops
}

func brightnessOps(level int) []fbinkOp {
	fill := brightnessFillWidth(level)
	knobX := knobPosition(fill)
	refreshLeft := brightnessTrackX
	refreshTop := panelTop + 32
	refreshWidth := screenW - refreshLeft - marginX
	refreshHeight := 56

	ops := []fbinkOp{
		// Clear the whole refresh region first: the knob is taller than the
		// track, so wiping only the track would leave white overhang stripes
		// behind as the knob moves.
		{Kind: opCls, Left: refreshLeft, Top: refreshTop, Width: refreshWidth, Height: refreshHeight},
		{Kind: opCls, Left: brightnessTrackX, Top: brightnessTrackY, Width: brightnessTrackW, Height: brightnessTrackH, Color: "GRAY3"},
	}
	if fill > 0 {
		ops = append(ops, fbinkOp{
			Kind: opCls, Left: brightnessTrackX, Top: brightnessTrackY,
			Width: fill, Height: brightnessTrackH, Color: "WHITE",
		})
	}
	ops = append(ops,
		fbinkOp{
			Kind: opCls, Left: knobX, Top: brightnessTrackY - 4,
			Width: 12, Height: brightnessTrackH + 8, Color: "WHITE",
		},
		textOp(textPlace{X: 880, Y: panelTop + 36, Text: levelText(level)}, fontTitle),
		fbinkOp{Kind: opRefresh, Left: refreshLeft, Top: refreshTop, Width: refreshWidth, Height: refreshHeight},
	)
	return ops
}

// readBrightness returns -1 when the property is unavailable.
func readBrightness() int {
	return parseBrightnessLevel(lipcGet("com.lab126.powerd", "flIntensity"))
}

func setBrightness(level int) {
	if level < 0 {
		return
	}
	if level > brightnessLevels {
		level = brightnessLevels
	}
	if _, err := runOutput(2*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.powerd", "flIntensity", strconv.Itoa(level)); err != nil {
		logf("set brightness %d error: %v", level, err)
	}
}

func drawCircleOutline(img *image.Gray, cx, cy, radius, thickness float64, shade uint8) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			if math.Abs(math.Hypot(dx, dy)-radius) <= thickness/2 {
				img.SetGray(x, y, color.Gray{Y: shade})
			}
		}
	}
}

func sunIcon() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 36, 36))
	drawCircleOutline(img, 18, 18, 9, 3, 255)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x0 := 18 + int(math.Cos(a)*12)
		y0 := 18 + int(math.Sin(a)*12)
		x1 := 18 + int(math.Cos(a)*16)
		y1 := 18 + int(math.Sin(a)*16)
		drawLine(img, x0, y0, x1, y1, 2.5, 255)
	}
	return img
}

func gearIcon() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 32, 32))
	drawCircleOutline(img, 16, 16, 9, 3.5, 255)
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi / 4
		x0 := 16 + int(math.Cos(a)*10)
		y0 := 16 + int(math.Sin(a)*10)
		x1 := 16 + int(math.Cos(a)*14)
		y1 := 16 + int(math.Sin(a)*14)
		drawLine(img, x0, y0, x1, y1, 4, 255)
	}
	return img
}

func chevronIcon() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, 20, 20))
	drawLine(img, 7, 4, 14, 10, 2.5, 255)
	drawLine(img, 14, 10, 7, 16, 2.5, 255)
	return img
}
