package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"sync"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// The native backend keeps pixels in process memory. X owns presentation.
type xDisplay struct {
	c      *xgb.Conn
	window xproto.Window
	gc     xproto.Gcontext
	depth  byte
	pixels *image.Gray
	faces  map[string]font.Face
	mu     sync.Mutex
}

var nativeDisplay *xDisplay

func newXDisplay() (*xDisplay, error) {
	c, err := xgb.NewConnDisplay(":0")
	if err != nil {
		return nil, err
	}
	s := xproto.Setup(c).DefaultScreen(c)
	if s.RootDepth != 8 || int(s.WidthInPixels) != screenW || int(s.HeightInPixels) != screenH {
		c.Close()
		return nil, fmt.Errorf("unsupported X screen %dx%d depth=%d", s.WidthInPixels, s.HeightInPixels, s.RootDepth)
	}
	d := &xDisplay{c: c, depth: s.RootDepth, pixels: image.NewGray(image.Rect(0, 0, screenW, screenH)), faces: map[string]font.Face{}}
	d.window, err = xproto.NewWindowId(c)
	if err != nil {
		c.Close()
		return nil, err
	}
	err = xproto.CreateWindowChecked(c, s.RootDepth, d.window, s.Root, 0, 0, screenW, screenH, 0, xproto.WindowClassInputOutput, s.RootVisual, xproto.CwBackPixel|xproto.CwEventMask, []uint32{s.WhitePixel, xproto.EventMaskExposure | xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease | xproto.EventMaskPointerMotion}).Check()
	if err != nil {
		c.Close()
		return nil, err
	}
	title := "L:A_N:application_ID:org.kindlepowerful.library_PC:N_O:U"
	err = xproto.ChangePropertyChecked(c, xproto.PropModeReplace, d.window, xproto.AtomWmName, xproto.AtomString, 8, uint32(len(title)), []byte(title)).Check()
	if err != nil {
		c.Close()
		return nil, err
	}
	d.gc, err = xproto.NewGcontextId(c)
	if err != nil {
		c.Close()
		return nil, err
	}
	err = xproto.CreateGCChecked(c, d.gc, xproto.Drawable(d.window), 0, nil).Check()
	if err != nil {
		c.Close()
		return nil, err
	}
	d.fill(image.Rect(0, 0, screenW, screenH), 255)
	return d, nil
}
func (d *xDisplay) fill(r image.Rectangle, shade uint8) {
	draw.Draw(d.pixels, r, &image.Uniform{C: color.Gray{Y: shade}}, image.Point{}, draw.Src)
}
func shade(name string) uint8 {
	switch name {
	case "WHITE":
		return 255
	case "GRAY5":
		return 85
	case "GRAY7":
		return 119
	default:
		return 0
	}
}
func (d *xDisplay) text(top, left, px int, path, ink, text string, invert bool) {
	key := fmt.Sprintf("%s:%d", path, px)
	face := d.faces[key]
	if face == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			logf("X font: %v", err)
			return
		}
		f, err := opentype.Parse(data)
		if err != nil {
			logf("X font parse: %v", err)
			return
		}
		face, err = opentype.NewFace(f, &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			return
		}
		d.faces[key] = face
	}
	value := shade(ink)
	if invert {
		value = 255
	}
	drawer := font.Drawer{Dst: d.pixels, Src: &image.Uniform{C: color.Gray{Y: value}}, Face: face, Dot: fixed.P(left, top+face.Metrics().Ascent.Ceil())}
	drawer.DrawString(text)
}
func (d *xDisplay) present(r image.Rectangle) {
	r = r.Intersect(d.pixels.Bounds())
	if r.Empty() {
		return
	}
	// Full-width strips obey X request-size limits and scanline alignment.
	for y := r.Min.Y; y < r.Max.Y; y += 48 {
		h := 48
		if y+h > r.Max.Y {
			h = r.Max.Y - y
		}
		start := y * d.pixels.Stride
		if err := xproto.PutImageChecked(d.c, xproto.ImageFormatZPixmap, xproto.Drawable(d.window), d.gc, screenW, uint16(h), 0, int16(y), 0, d.depth, d.pixels.Pix[start:start+h*d.pixels.Stride]).Check(); err != nil {
			logf("X paint: %v", err)
			return
		}
	}
	d.c.Sync()
}
func (d *xDisplay) op(op fbinkOp) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch op.Kind {
	case opCls:
		d.fill(image.Rect(op.Left, op.Top, op.Left+op.Width, op.Top+op.Height), shade(op.Color))
	case opText:
		d.text(op.Y, op.X, op.Px, op.Font, op.Color, op.Text, op.Invert)
	case opImage:
		img, err := png.Decode(bytes.NewReader(op.Image))
		if err == nil {
			draw.Draw(d.pixels, image.Rectangle{Min: image.Pt(op.X, op.Y), Max: image.Pt(op.X+img.Bounds().Dx(), op.Y+img.Bounds().Dy())}, img, img.Bounds().Min, draw.Src)
		}
	case opRefresh:
		r := image.Rect(op.Left, op.Top, op.Left+op.Width, op.Top+op.Height)
		if op.Width == 0 || op.Height == 0 {
			r = d.pixels.Bounds()
		}
		d.present(r)
	}
}
func (d *xDisplay) events(books []Book) {
	for {
		ev, err := d.c.WaitForEvent()
		if err != nil {
			logf("X event: %v", err)
			return
		}
		if ev == nil {
			return
		}
		switch e := ev.(type) {
		case xproto.ExposeEvent:
			if e.Count == 0 && !screenAsleep.Load() {
				d.mu.Lock()
				d.present(d.pixels.Bounds())
				d.mu.Unlock()
			}

		}
	}
}

func (d *xDisplay) show() {
	if err := xproto.MapWindowChecked(d.c, d.window).Check(); err != nil {
		logf("X map: %v", err)
	}
	d.c.Sync()
}
func (d *xDisplay) hide() {
	if err := xproto.UnmapWindowChecked(d.c, d.window).Check(); err != nil {
		logf("X unmap: %v", err)
	}
	d.c.Sync()
}

// System dialogs take focus while the app ID may remain unchanged.
func (d *xDisplay) acceptsInput() bool {
	reply, err := xproto.GetInputFocus(d.c).Reply()
	return err == nil && reply.Focus == d.window
}
