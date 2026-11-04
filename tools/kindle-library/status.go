package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	wifiIconW      = 36
	wifiIconH      = 36
	batteryIconW   = 52
	batteryIconH   = 30
	statusGap      = 26
	batteryTextGap = 8

	statusTextTop   = 11
	fallbackIconPx  = 18
	fallbackTextTop = 9
)

type statusState struct {
	Clock     string
	Batt      int
	Charging  bool
	WifiOn    bool
	WifiLevel int
}

type textPlace struct {
	X, Y int
	Text string
	Bold bool
}

type imagePlace struct {
	X, Y int
}

type headerPlan struct {
	BarLeft, BarTop, BarWidth, BarHeight int

	Title       textPlace
	Clock       textPlace
	BatteryText textPlace
	Page        textPlace

	Wifi             imagePlace
	WifiIconWidth    int
	WifiLevel        int
	WifiOn           bool
	Battery          imagePlace
	BatteryIconWidth int
	BatteryLevel     int
	BatteryCharging  bool

	RefreshLeft, RefreshTop, RefreshWidth, RefreshHeight int
}

// textWidthPx estimates the rendered width of s at the given pixel size.
// It is used only to center and right-align the header; fbink does the real
// shaping. ASCII glyphs average a little over half an em in the STHeiti faces.
func textWidthPx(s string, px int) int {
	w := 0
	for _, r := range s {
		if runeWidth(r) == 2 {
			w += px
		} else {
			w += px * 55 / 100
		}
	}
	return w
}

func parseBattLevel(raw string) int {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n < 0 || n > 100 {
		return -1
	}
	return n
}

func parseCharging(isCharging, battStatus string) bool {
	switch strings.ToLower(strings.TrimSpace(isCharging)) {
	case "1", "true", "yes", "charging":
		return true
	case "0", "false", "no", "discharging", "not charging":
		return false
	}
	s := strings.ToLower(strings.TrimSpace(battStatus))
	return s == "charging" || strings.HasPrefix(s, "charging")
}

func parseWifiState(cmState, signalStrength string) (bool, int) {
	on := strings.EqualFold(strings.TrimSpace(cmState), "CONNECTED")
	if !on {
		return false, 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.SplitN(signalStrength, "/", 2)[0]))
	if err != nil {
		return true, 3
	}
	if n < 0 {
		n = 0
	}
	if n > 5 {
		n = 5
	}
	return true, n
}

func wifiArcs(level int) int {
	switch {
	case level <= 1:
		return 1
	case level <= 3:
		return 2
	default:
		return 3
	}
}

// canDrawHeader keeps the periodic clock refresh away from native apps
// while permitting the library overlay on the live KPP home. The library is a raw framebuffer overlay; while it owns
// the visible page, activeApp is usually empty, webview or the legacy home.
func canDrawHeader(foregroundApp string) bool {
	return !isRestrictedNativeApp(foregroundApp) &&
		!strings.Contains(foregroundApp, "booklet.reader")
}

// isRestrictedNativeApp identifies Settings/Store, which are restricted on an
// unregistered device and must not be painted over by the library.
func isRestrictedNativeApp(app string) bool {
	return strings.Contains(app, "settings") || strings.Contains(app, "store")
}

// libraryOwnsScreen is false while the native reader is in front. In that
// state the library must not touch the framebuffer or react to input events.
func libraryOwnsScreen(readerActive bool) bool {
	return !readerActive
}

func parseKppReady(probeOutput string) bool {
	return strings.Contains(probeOutput, "com.lab126.KPPAnnotationController")
}

func kppServicesReady() bool {
	out, err := runOutput(3*time.Second, "/usr/bin/lipc-probe", "-l")
	return err == nil && parseKppReady(string(out))
}

// isReaderReturnGesture is the escape hatch while the native reader owns the
// screen: hold the top-left corner for two seconds. It must not fire while
// reading text in the page body.
func isReaderReturnGesture(startX, startY, endX, endY int, held time.Duration) bool {
	if startX < 0 || startY < 0 || endX < 0 || endY < 0 {
		return false
	}
	return startX < 150 && startY < 150 &&
		abs(endX-startX) < 40 && abs(endY-startY) < 40 &&
		held >= 2*time.Second
}

func makeHeaderPlan(st statusState, page, total int) headerPlan {
	p := headerPlan{
		BarLeft:   0,
		BarTop:    0,
		BarWidth:  screenW,
		BarHeight: headerH,

		Title: textPlace{X: marginX, Y: statusTextTop, Text: "图书馆", Bold: true},
		Clock: textPlace{X: 0, Y: statusTextTop, Text: st.Clock, Bold: false},
		Page:  textPlace{Y: statusTextTop, Bold: true},

		Wifi:             imagePlace{Y: (headerH-wifiIconH)/2 - 1},
		WifiIconWidth:    wifiIconW,
		WifiLevel:        st.WifiLevel,
		WifiOn:           st.WifiOn,
		Battery:          imagePlace{Y: (headerH - batteryIconH) / 2},
		BatteryIconWidth: batteryIconW,
		BatteryLevel:     st.Batt,
		BatteryCharging:  st.Charging,
	}

	if st.Batt < 0 {
		p.BatteryText.Text = "--"
	} else {
		p.BatteryText.Text = fmt.Sprintf("%d%%", st.Batt)
	}
	p.BatteryText.Y = statusTextTop

	p.Page.Text = fmt.Sprintf("%d/%d", page+1, total)
	pageW := textWidthPx(p.Page.Text, headerPx)
	p.Page.X = screenW - marginX - pageW

	clockW := textWidthPx(p.Clock.Text, headerPx)
	battSlotW := textWidthPx("100%", headerPx)
	groupW := clockW + statusGap + wifiIconW + statusGap + batteryIconW + batteryTextGap + battSlotW
	startX := (screenW - groupW) / 2

	p.Clock.X = startX
	p.Wifi.X = p.Clock.X + clockW + statusGap
	p.Battery.X = p.Wifi.X + wifiIconW + statusGap
	p.BatteryText.X = p.Battery.X + batteryIconW + batteryTextGap

	p.RefreshLeft = p.BarLeft
	p.RefreshTop = p.BarTop
	p.RefreshWidth = p.BarWidth
	p.RefreshHeight = p.BarHeight

	return p
}

type fbinkOpKind int

const (
	opCls fbinkOpKind = iota
	opText
	opImage
	opRefresh
)

type fbinkOp struct {
	Kind fbinkOpKind

	Left, Top, Width, Height int
	X, Y                     int
	Text                     string
	Font                     string
	Px                       int
	Invert                   bool
	Color                    string
	Image                    []byte
}

func iconPNG(img *image.Gray) []byte {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		logf("png encode error: %v", err)
		return nil
	}
	return buf.Bytes()
}

func textOp(p textPlace, font string) fbinkOp {
	return fbinkOp{
		Kind:   opText,
		X:      p.X,
		Y:      p.Y,
		Text:   p.Text,
		Font:   font,
		Px:     headerPx,
		Invert: true,
	}
}

func headerOps(p headerPlan) []fbinkOp {
	wifiOp := fbinkOp{
		Kind: opImage,
		X:    p.Wifi.X, Y: p.Wifi.Y,
		Image: iconPNG(wifiIcon(p.WifiLevel, p.WifiOn)),
	}
	batteryOp := fbinkOp{
		Kind: opImage,
		X:    p.Battery.X, Y: p.Battery.Y,
		Image: iconPNG(batteryIcon(p.BatteryLevel, p.BatteryCharging)),
	}
	if !imageIcons {
		wifiOp = fbinkOp{
			Kind: opText, X: p.Wifi.X, Y: fallbackTextTop,
			Text: wifiFallbackText(p), Font: fontTitle, Px: fallbackIconPx, Invert: true,
		}
		batteryOp = fbinkOp{
			Kind: opText, X: p.Battery.X, Y: fallbackTextTop,
			Text: batteryFallbackText(p), Font: fontTitle, Px: fallbackIconPx, Invert: true,
		}
	}

	return []fbinkOp{
		{
			Kind: opCls,
			Left: p.BarLeft, Top: p.BarTop,
			Width: p.BarWidth, Height: p.BarHeight,
		},
		textOp(p.Title, fontBold),
		textOp(p.Clock, fontTitle),
		wifiOp,
		batteryOp,
		textOp(p.BatteryText, fontTitle),
		textOp(p.Page, fontBold),
		{
			Kind: opRefresh,
			Left: p.RefreshLeft, Top: p.RefreshTop,
			Width: p.RefreshWidth, Height: p.RefreshHeight,
		},
	}
}

func wifiFallbackText(p headerPlan) string {
	if !p.WifiOn {
		return "×"
	}
	switch wifiArcs(p.WifiLevel) {
	case 1:
		return "▂"
	case 2:
		return "▂▄"
	default:
		return "▂▄▆"
	}
}

func batteryFallbackText(p headerPlan) string {
	if p.BatteryCharging {
		return "[+]"
	}
	level := p.BatteryLevel
	if level < 0 {
		level = 0
	}
	filled := level / 34
	if filled > 3 {
		filled = 3
	}
	return "[" + strings.Repeat("█", filled) + strings.Repeat(" ", 3-filled) + "]"
}

// imageIcons is enabled when the device build of fbink reports image support.
// It is replaced at startup by detectImageIcons.
var imageIcons = true

func parseImageSupport(versionOutput string) bool {
	return strings.Contains(versionOutput, "Image=1") ||
		strings.Contains(versionOutput, "Image=Yes")
}

// appGone reports whether the foreground app has left. An empty activeApp can
// also mean a timed-out lipc query, so it must be seen three times in a row
// before we treat the app as closed.
func appGone(app, appID string, emptyStreak int) (bool, int) {
	if strings.Contains(app, appID) {
		return false, 0
	}
	if app != "" {
		return true, 0
	}
	emptyStreak++
	return emptyStreak >= 3, emptyStreak
}

func detectImageIcons() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// --help prints the build feature banner (e.g. "Image=1") and exits 0.
	out, err := exec.CommandContext(ctx, fbink, "--help").CombinedOutput()
	if err != nil {
		return false
	}
	return parseImageSupport(string(out))
}

func fbinkArgs(op fbinkOp) []string {
	switch op.Kind {
	case opCls:
		color := op.Color
		if color == "" {
			color = "BLACK"
		}
		return []string{"-q", "-b", "-B", color, "-k",
			fmt.Sprintf("top=%d,left=%d,width=%d,height=%d",
				op.Top, op.Left, op.Width, op.Height)}
	case opText:
		spec := fmt.Sprintf("regular=%s,px=%d,top=%d,left=%d", op.Font, op.Px, op.Y, op.X)
		args := []string{"-q", "-b", "-t", spec}
		if op.Invert {
			args = append(args, "-h")
		}
		// "--" keeps a leading dash in the text from being parsed as an option.
		return append(args, "--", op.Text)
	case opImage:
		return []string{"-q", "-b", "-g",
			fmt.Sprintf("file=-,x=%d,y=%d", op.X, op.Y)}
	case opRefresh:
		return []string{"-q", "-s",
			fmt.Sprintf("top=%d,left=%d,width=%d,height=%d",
				op.Top, op.Left, op.Width, op.Height)}
	}
	return nil
}

func executeOps(ops []fbinkOp) {
	for _, op := range ops {
		if nativeDisplay != nil {
			nativeDisplay.op(op)
			continue
		}
		args := fbinkArgs(op)
		if args == nil {
			continue
		}
		cmd := exec.Command(fbink, args...)
		if op.Kind == opImage {
			cmd.Stdin = bytes.NewReader(op.Image)
		}
		if out, err := cmd.CombinedOutput(); err != nil {
			logf("fbink op %d error: %v out=%s", op.Kind, err, string(out))
		}
	}
}

// runOutput runs a short read-only command with a deadline so a wedged LIPC or
// status query cannot block a page turn while drawMu is held.
func runOutput(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

func lipcGet(service, prop string) string {
	out, err := runOutput(2*time.Second, "/usr/bin/lipc-get-prop", service, prop)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func readBattery() (int, bool) {
	level := parseBattLevel(lipcGet("com.lab126.powerd", "battLevel"))
	charging := readCharging(func(prop string) string { return lipcGet("com.lab126.powerd", prop) })
	return level, charging
}

func readWifi() (bool, int) {
	return parseWifiState(
		lipcGet("com.lab126.wifid", "cmState"),
		lipcGet("com.lab126.wifid", "signalStrength"))
}

func readClock() string {
	out, err := runOutput(2*time.Second, "/bin/date", "+%H:%M")
	if err == nil {
		s := strings.TrimSpace(string(out))
		if len(s) == 5 {
			return s
		}
	}
	return time.Now().Format("15:04")
}

func setWhite(img *image.Gray, x, y int) {
	if image.Pt(x, y).In(img.Bounds()) {
		img.SetGray(x, y, color.Gray{Y: 255})
	}
}

func wifiIcon(level int, on bool) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, wifiIconW, wifiIconH))
	const cx, cy = 18, 29

	drawArc := func(radius float64, shade uint8) {
		for y := 0; y < wifiIconH; y++ {
			for x := 0; x < wifiIconW; x++ {
				dx := float64(x - cx)
				up := float64(cy - y)
				if up <= 0 || math.Abs(dx) > up*1.6 {
					continue
				}
				if math.Abs(math.Hypot(dx, up)-radius) <= 1.6 {
					img.SetGray(x, y, color.Gray{Y: shade})
				}
			}
		}
	}

	if !on {
		for _, r := range []float64{7.5, 14, 20.5} {
			drawArc(r, 128)
		}
		drawLine(img, 7, 7, 29, 29, 2.0, 255)
		return img
	}

	for i, r := range []float64{7.5, 14, 20.5} {
		if i >= wifiArcs(level) {
			break
		}
		drawArc(r, 255)
	}
	for y := cy - 2; y <= cy+2; y++ {
		for x := cx - 2; x <= cx+2; x++ {
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= 6 {
				setWhite(img, x, y)
			}
		}
	}
	return img
}

func batteryIcon(level int, charging bool) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, batteryIconW, batteryIconH))

	// Body outline, 2px thick, plus the positive terminal.
	for y := 3; y <= 27; y++ {
		for x := 1; x <= 45; x++ {
			if x <= 2 || x >= 44 || y <= 4 || y >= 26 {
				setWhite(img, x, y)
			}
		}
	}
	for y := 10; y <= 20; y++ {
		for x := 46; x <= 50; x++ {
			setWhite(img, x, y)
		}
	}

	if charging {
		// Simple lightning bolt; the level text still carries the exact charge.
		drawLine(img, 29, 6, 22, 15, 2.5, 255)
		drawLine(img, 22, 15, 27, 15, 2.5, 255)
		drawLine(img, 27, 15, 21, 24, 2.5, 255)
		return img
	}

	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	fillW := 37 * level / 100
	for y := 7; y <= 23; y++ {
		for x := 4; x < 4+fillW; x++ {
			setWhite(img, x, y)
		}
	}
	return img
}

// drawLine paints a line with the given width using simple distance testing.
func drawLine(img *image.Gray, x0, y0, x1, y1 int, width float64, shade uint8) {
	ax, ay := float64(x0), float64(y0)
	bx, by := float64(x1), float64(y1)
	dx, dy := bx-ax, by-ay
	length2 := dx*dx + dy*dy
	if length2 == 0 {
		return
	}
	half := width / 2
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			t := ((float64(x)-ax)*dx + (float64(y)-ay)*dy) / length2
			if t < 0 || t > 1 {
				continue
			}
			px, py := ax+t*dx, ay+t*dy
			ddx, ddy := float64(x)-px, float64(y)-py
			if ddx*ddx+ddy*ddy <= half*half {
				img.SetGray(x, y, color.Gray{Y: shade})
			}
		}
	}
}

// Only query the legacy fallback when isCharging is unavailable.
func readCharging(get func(string) string) bool {
	value := strings.TrimSpace(get("isCharging"))
	if value != "" {
		return parseCharging(value, "")
	}
	return parseCharging("", get("battStatus"))
}

// Missing properties keep the last successful value; zero is a real reading.
func sampleStatus(previous statusState, get func(string, string) string) statusState {
	st := previous
	if level := parseBattLevel(get("com.lab126.powerd", "battLevel")); level >= 0 {
		st.Batt = level
	}
	if raw := get("com.lab126.powerd", "isCharging"); raw != "" {
		st.Charging = parseCharging(raw, "")
	}
	if raw := get("com.lab126.wifid", "cmState"); raw != "" {
		signal := get("com.lab126.wifid", "signalStrength")
		st.WifiOn, st.WifiLevel = parseWifiState(raw, signal)
		if signal == "" && previous.WifiOn && st.WifiOn {
			st.WifiLevel = previous.WifiLevel
		}
	}
	return st
}

func refreshStatus() {
	drawMu.Lock()
	previous := curStatus
	drawMu.Unlock()
	st := sampleStatus(previous, lipcGet)
	st.Clock = readClock()
	drawMu.Lock()
	curStatus = st
	drawMu.Unlock()
}
