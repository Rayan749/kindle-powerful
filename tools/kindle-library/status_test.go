package main

import (
	"bytes"
	"image"
	"reflect"
	"testing"
	"time"
)

func TestTextWidthPx(t *testing.T) {
	if got := textWidthPx("图书馆", 28); got != 84 {
		t.Fatalf("CJK width = %d, want 84", got)
	}
	if textWidthPx("1/2", 28) >= textWidthPx("图书馆", 28) {
		t.Fatalf("ASCII width should be narrower than CJK width")
	}
}

func TestParseBattLevel(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"85", 85},
		{"0", 0},
		{"100", 100},
		{" 42\n", 42},
		{"", -1},
		{"abc", -1},
		{"-5", -1},
		{"120", -1},
	}
	for _, c := range cases {
		if got := parseBattLevel(c.in); got != c.want {
			t.Errorf("parseBattLevel(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseCharging(t *testing.T) {
	cases := []struct {
		isCharging string
		battStatus string
		want       bool
	}{
		{"1", "", true},
		{"0", "", false},
		{"true", "", true},
		{"false", "", false},
		{"charging", "", true},
		{"discharging", "", false},
		{"", "Charging", true},
		{"", "Discharging", false},
		{"", "Not charging", false},
		{"", "", false},
	}
	for _, c := range cases {
		if got := parseCharging(c.isCharging, c.battStatus); got != c.want {
			t.Errorf("parseCharging(%q, %q) = %v, want %v", c.isCharging, c.battStatus, got, c.want)
		}
	}
}

func TestParseWifiState(t *testing.T) {
	cases := []struct {
		state string
		sig   string
		on    bool
		level int
	}{
		{"CONNECTED", "5", true, 5},
		{"DISCONNECTED", "5", false, 0},
		{"NOT_CONNECTED", "5", false, 0},
		{"CONNECTED", "5/5", true, 5},
		{"connected", "0", true, 0},
		{"CONNECTED", "", true, 3},
		{"CONNECTED", "7", true, 5},
		{"NA", "5", false, 0},
		{"", "", false, 0},
	}
	for _, c := range cases {
		on, level := parseWifiState(c.state, c.sig)
		if on != c.on || level != c.level {
			t.Errorf("parseWifiState(%q, %q) = (%v, %d), want (%v, %d)",
				c.state, c.sig, on, level, c.on, c.level)
		}
	}
}

func TestWifiArcs(t *testing.T) {
	cases := []struct {
		level int
		want  int
	}{
		{0, 1}, {1, 1}, {2, 2}, {3, 2}, {4, 3}, {5, 3},
	}
	for _, c := range cases {
		if got := wifiArcs(c.level); got != c.want {
			t.Errorf("wifiArcs(%d) = %d, want %d", c.level, got, c.want)
		}
	}
}

func TestHeaderPlanFillsBar(t *testing.T) {
	p := makeHeaderPlan(statusState{Clock: "14:35", Batt: 85, WifiOn: true, WifiLevel: 5}, 0, 2)
	if p.BarLeft != 0 || p.BarTop != 0 || p.BarWidth != screenW || p.BarHeight != headerH {
		t.Fatalf("bar = (%d,%d,%d,%d), want (0,0,%d,%d)",
			p.BarLeft, p.BarTop, p.BarWidth, p.BarHeight, screenW, headerH)
	}
	if p.RefreshLeft != p.BarLeft || p.RefreshTop != p.BarTop ||
		p.RefreshWidth != p.BarWidth || p.RefreshHeight != p.BarHeight {
		t.Fatalf("refresh rect %d,%d,%d,%d does not match bar",
			p.RefreshLeft, p.RefreshTop, p.RefreshWidth, p.RefreshHeight)
	}
}

func TestHeaderPlanCentersStatusGroup(t *testing.T) {
	p := makeHeaderPlan(statusState{Clock: "14:35", Batt: 85, WifiOn: true, WifiLevel: 5}, 0, 2)
	slot := textWidthPx("100%", headerPx)
	mid := (p.Clock.X + p.BatteryText.X + slot) / 2
	if diff := mid - screenW/2; diff < -1 || diff > 1 {
		t.Fatalf("status group midpoint = %d, want %d", mid, screenW/2)
	}
	if !(p.Clock.X < p.Wifi.X && p.Wifi.X+p.WifiIconWidth < p.Battery.X &&
		p.Battery.X+p.BatteryIconWidth <= p.BatteryText.X) {
		t.Fatalf("status elements overlap or are out of order: clock=%d wifi=%d battery=%d text=%d",
			p.Clock.X, p.Wifi.X, p.Battery.X, p.BatteryText.X)
	}
}

func TestHeaderPlanRightAlignsPage(t *testing.T) {
	p := makeHeaderPlan(statusState{Clock: "00:00", Batt: 42}, 9, 100)
	right := p.Page.X + textWidthPx(p.Page.Text, headerPx)
	if right != screenW-marginX {
		t.Fatalf("page right edge = %d, want %d", right, screenW-marginX)
	}
}

func TestHeaderPlanFormatsText(t *testing.T) {
	p := makeHeaderPlan(statusState{Clock: "14:35", Batt: 85, WifiOn: true, WifiLevel: 4}, 2, 12)
	if p.Title.Text != "图书馆" {
		t.Errorf("title = %q", p.Title.Text)
	}
	if p.Clock.Text != "14:35" {
		t.Errorf("clock = %q", p.Clock.Text)
	}
	if p.BatteryText.Text != "85%" {
		t.Errorf("battery text = %q", p.BatteryText.Text)
	}
	if p.Page.Text != "3/12" {
		t.Errorf("page = %q", p.Page.Text)
	}
}

func TestHeaderPlanUnknownBattery(t *testing.T) {
	p := makeHeaderPlan(statusState{Clock: "08:05", Batt: -1}, 0, 1)
	if p.BatteryText.Text != "--" {
		t.Fatalf("unknown battery text = %q, want --", p.BatteryText.Text)
	}
}

func TestWifiIconSize(t *testing.T) {
	img := wifiIcon(5, true)
	if img.Bounds().Dx() != wifiIconW || img.Bounds().Dy() != wifiIconH {
		t.Fatalf("wifi icon = %dx%d, want %dx%d",
			img.Bounds().Dx(), img.Bounds().Dy(), wifiIconW, wifiIconH)
	}
}

func TestWifiIconDrawsWhitePixels(t *testing.T) {
	img := wifiIcon(5, true)
	if img.GrayAt(0, 0).Y != 0 {
		t.Fatalf("wifi icon corner should be black")
	}
	if countWhite(img) == 0 {
		t.Fatalf("wifi icon has no white pixels")
	}
}

func TestBatteryIconFillScales(t *testing.T) {
	full := countWhite(batteryIcon(100, false))
	low := countWhite(batteryIcon(20, false))
	if full <= low {
		t.Fatalf("full battery white pixels = %d, low = %d; fill does not scale", full, low)
	}
}

func TestBatteryIconChargingDiffers(t *testing.T) {
	plain := batteryIcon(50, false)
	charging := batteryIcon(50, true)
	if countWhite(plain) == countWhite(charging) {
		t.Fatalf("charging icon should differ from plain icon")
	}
}

func countWhite(img *image.Gray) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.GrayAt(x, y).Y > 0 {
				n++
			}
		}
	}
	return n
}

func TestHeaderOpsOrderAndGeometry(t *testing.T) {
	p := makeHeaderPlan(statusState{Clock: "14:35", Batt: 85, WifiOn: true, WifiLevel: 5}, 0, 2)
	ops := headerOps(p)

	if len(ops) != 8 {
		t.Fatalf("got %d ops, want 8", len(ops))
	}
	if ops[0].Kind != opCls || ops[0].Left != 0 || ops[0].Top != 0 ||
		ops[0].Width != screenW || ops[0].Height != headerH {
		t.Fatalf("first op is not a full-width black bar: %+v", ops[0])
	}
	refresh := ops[7]
	if refresh.Kind != opRefresh || refresh.Width != screenW || refresh.Height != headerH {
		t.Fatalf("last op is not a header refresh: %+v", refresh)
	}

	wantTexts := []struct {
		index int
		text  string
		x, y  int
	}{
		{1, p.Title.Text, p.Title.X, p.Title.Y},
		{2, p.Clock.Text, p.Clock.X, p.Clock.Y},
		{5, p.BatteryText.Text, p.BatteryText.X, p.BatteryText.Y},
		{6, p.Page.Text, p.Page.X, p.Page.Y},
	}
	for _, want := range wantTexts {
		op := ops[want.index]
		if op.Kind != opText || op.Text != want.text || op.X != want.x || op.Y != want.y {
			t.Errorf("text op %d = %+v, want %q at (%d,%d)", want.index, op, want.text, want.x, want.y)
		}
	}

	if ops[3].Kind != opImage || ops[3].X != p.Wifi.X || ops[3].Y != p.Wifi.Y {
		t.Errorf("wifi op = %+v, want image at (%d,%d)", ops[3], p.Wifi.X, p.Wifi.Y)
	}
	if ops[4].Kind != opImage || ops[4].X != p.Battery.X || ops[4].Y != p.Battery.Y {
		t.Errorf("battery op = %+v, want image at (%d,%d)", ops[4], p.Battery.X, p.Battery.Y)
	}
	for _, op := range []fbinkOp{ops[3], ops[4]} {
		if !bytes.HasPrefix(op.Image, []byte("\x89PNG")) {
			t.Errorf("image op does not carry PNG data")
		}
	}
}

func TestHeaderOpsWifiIconFollowsState(t *testing.T) {
	on := headerOps(makeHeaderPlan(statusState{Clock: "00:00", WifiOn: true, WifiLevel: 5}, 0, 1))
	off := headerOps(makeHeaderPlan(statusState{Clock: "00:00", WifiOn: false}, 0, 1))
	if bytes.Equal(on[3].Image, off[3].Image) {
		t.Fatalf("connected and disconnected wifi icons are identical")
	}
}

func TestHeaderOpsFallsBackToTextIcons(t *testing.T) {
	old := imageIcons
	imageIcons = false
	t.Cleanup(func() { imageIcons = old })

	ops := headerOps(makeHeaderPlan(statusState{Clock: "00:00", Batt: 50, WifiOn: true, WifiLevel: 5}, 0, 1))
	if len(ops) != 8 {
		t.Fatalf("got %d ops, want 8", len(ops))
	}
	if ops[3].Kind != opText || ops[4].Kind != opText {
		t.Fatalf("fallback should draw text icons, got kinds %d and %d", ops[3].Kind, ops[4].Kind)
	}
}

func TestParseImageSupport(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"fbink v1.0 [Draw=1, Bitmap=1, Fonts=1, Unifont=0, OpenType=1, Image=1, Input=1]", true},
		{"fbink v1.0 [Draw=1, Bitmap=1, Fonts=1, Unifont=0, OpenType=1, Image=0, Input=1]", false},
		{"FBInk 1.25.0 for Kindle [Draw=Yes, Bitmap=Yes, Fonts=Yes, Unifont=No, OpenType=Yes, Image=Yes, Input=Yes]", true},
		{"FBInk 1.25.0 for Kindle [Draw=Yes, Bitmap=Yes, Fonts=Yes, Unifont=No, OpenType=Yes, Image=No, Input=Yes]", false},
		{"", false},
	}
	for _, c := range cases {
		if got := parseImageSupport(c.in); got != c.want {
			t.Errorf("parseImageSupport(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsRestrictedNativeApp(t *testing.T) {
	cases := []struct {
		app  string
		want bool
	}{
		{"com.lab126.booklet.settings", true},
		{"com.lab126.store", true},
		{"com.lab126.KPPMainApp", false},
		{"com.lab126.booklet.reader", false},
		{"com.lab126.webview", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isRestrictedNativeApp(c.app); got != c.want {
			t.Errorf("isRestrictedNativeApp(%q) = %v, want %v", c.app, got, c.want)
		}
	}
}

func TestAppGone(t *testing.T) {
	cases := []struct {
		app      string
		streak   int
		want     bool
		wantNext int
	}{
		{"com.lab126.booklet.reader", 0, false, 0},
		{"com.lab126.store", 0, true, 0},
		{"", 0, false, 1},
		{"", 1, false, 2},
		{"", 2, true, 3},
	}
	for _, c := range cases {
		gone, next := appGone(c.app, "com.lab126.booklet.reader", c.streak)
		if gone != c.want || next != c.wantNext {
			t.Errorf("appGone(%q, %d) = (%v, %d), want (%v, %d)",
				c.app, c.streak, gone, next, c.want, c.wantNext)
		}
	}
}

func TestCanDrawHeader(t *testing.T) {
	cases := []struct {
		app  string
		want bool
	}{
		{"", true},
		{"com.lab126.webview", true},
		{"com.lab126.booklet.home", true},
		{"com.lab126.booklet.reader", false},
		{"com.lab126.booklet.settings", false},
		{"com.lab126.store", false},
		{"com.lab126.KPPMainApp", true},
	}
	for _, c := range cases {
		if got := canDrawHeader(c.app); got != c.want {
			t.Errorf("canDrawHeader(%q) = %v, want %v", c.app, got, c.want)
		}
	}
}

func TestLibraryOwnsScreen(t *testing.T) {
	cases := []struct {
		reader bool
		want   bool
	}{
		{false, true},
		{true, false},
	}
	for _, c := range cases {
		if got := libraryOwnsScreen(c.reader); got != c.want {
			t.Errorf("libraryOwnsScreen(%v) = %v, want %v", c.reader, got, c.want)
		}
	}
}

func TestParseKppReady(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"com.lab126.KPPAnnotationController\ncom.lab126.kppsettings\n", true},
		{"com.lab126.webview\ncom.lab126.kppsettings\n", false},
		{"", false},
	}
	for _, c := range cases {
		if got := parseKppReady(c.in); got != c.want {
			t.Errorf("parseKppReady(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsReaderReturnGesture(t *testing.T) {
	cases := []struct {
		name           string
		sx, sy, ex, ey int
		held           time.Duration
		want           bool
	}{
		{"top-left long press", 60, 60, 62, 61, 2500 * time.Millisecond, true},
		{"too short", 60, 60, 60, 60, 500 * time.Millisecond, false},
		{"moved too far", 60, 60, 220, 60, 3 * time.Second, false},
		{"outside corner", 400, 60, 400, 60, 3 * time.Second, false},
		{"negative coords", -1, -1, -1, -1, 3 * time.Second, false},
	}
	for _, c := range cases {
		if got := isReaderReturnGesture(c.sx, c.sy, c.ex, c.ey, c.held); got != c.want {
			t.Errorf("%s: isReaderReturnGesture = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFallbackIconsFitSlots(t *testing.T) {
	states := []statusState{
		{Clock: "00:00", Batt: 0, WifiOn: true, WifiLevel: 0},
		{Clock: "23:59", Batt: 37, WifiOn: true, WifiLevel: 2},
		{Clock: "12:00", Batt: 100, Charging: true, WifiOn: true, WifiLevel: 5},
		{Clock: "12:00", Batt: 84, WifiOn: false},
	}
	for _, st := range states {
		p := makeHeaderPlan(st, 0, 1)
		if w := textWidthPx(wifiFallbackText(p), fallbackIconPx); w > wifiIconW {
			t.Errorf("wifi fallback %q is %dpx, slot is %dpx", wifiFallbackText(p), w, wifiIconW)
		}
		if w := textWidthPx(batteryFallbackText(p), fallbackIconPx); w > batteryIconW {
			t.Errorf("battery fallback %q is %dpx, slot is %dpx", batteryFallbackText(p), w, batteryIconW)
		}
	}
}

func TestFbinkArgs(t *testing.T) {
	cases := []struct {
		name string
		op   fbinkOp
		want []string
	}{
		{
			"black bar",
			fbinkOp{Kind: opCls, Left: 0, Top: 0, Width: screenW, Height: headerH},
			[]string{"-q", "-b", "-B", "BLACK", "-k", "top=0,left=0,width=1072,height=48"},
		},
		{
			"inverted text",
			fbinkOp{Kind: opText, X: 400, Y: 3, Text: "14:35", Font: "/f.ttf", Px: 28, Invert: true},
			[]string{"-q", "-b", "-t", "regular=/f.ttf,px=28,top=3,left=400", "-h", "--", "14:35"},
		},
		{
			"image",
			fbinkOp{Kind: opImage, X: 495, Y: 6},
			[]string{"-q", "-b", "-g", "file=-,x=495,y=6"},
		},
		{
			"partial refresh",
			fbinkOp{Kind: opRefresh, Left: 0, Top: 0, Width: screenW, Height: headerH},
			[]string{"-q", "-s", "top=0,left=0,width=1072,height=48"},
		},
	}
	for _, c := range cases {
		if got := fbinkArgs(c.op); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: fbinkArgs = %#v, want %#v", c.name, got, c.want)
		}
	}
}

func TestReadChargingSkipsUnsupportedFallback(t *testing.T) {
	calls := 0
	got := readCharging(func(prop string) string {
		calls++
		if prop != "isCharging" {
			t.Fatalf("unexpected fallback property %s", prop)
		}
		return "1"
	})
	if !got || calls != 1 {
		t.Fatalf("charging=%v calls=%d", got, calls)
	}
}

func TestLibraryMayDrawOverLiveHomeOnly(t *testing.T) {
	for _, app := range []string{"com.lab126.KPPMainApp", "com.lab126.booklet.home"} {
		if !canDrawHeader(app) {
			t.Errorf("live home must permit library overlay: %s", app)
		}
	}
	if canDrawHeader("com.lab126.booklet.settings") {
		t.Fatal("settings must own screen")
	}
}

func TestReclaimLibraryOnlyAwakeHome(t *testing.T) {
	if !canReclaimLibrary("com.lab126.KPPMainApp", "active", false) {
		t.Fatal("home not reclaimed")
	}
	for _, c := range []struct {
		app, power string
		native     bool
	}{
		{"com.lab126.booklet.reader", "active", false},
		{"com.lab126.booklet.settings", "active", false},
		{"com.lab126.KPPMainApp", "screenSaver", false},
		{"com.lab126.KPPMainApp", "active", true},
	} {
		if canReclaimLibrary(c.app, c.power, c.native) {
			t.Fatalf("unsafe reclaim: %+v", c)
		}
	}
}

func TestSampleStatusRetainsKnownReadingsOnFailure(t *testing.T) {
	old := statusState{Batt: 53, Charging: true, WifiOn: true, WifiLevel: 4}
	got := sampleStatus(old, func(service, prop string) string { return "" })
	if got != old {
		t.Fatalf("lost known status: %+v", got)
	}
	got = sampleStatus(old, func(service, prop string) string {
		switch prop {
		case "battLevel":
			return "52"
		case "isCharging":
			return "0"
		case "cmState":
			return "DISCONNECTED"
		}
		return ""
	})
	if got.Batt != 52 || got.Charging || got.WifiOn {
		t.Fatalf("wrong sampled status: %+v", got)
	}
}

func TestWakeRestorePreservesReaderAndSettings(t *testing.T) {
	for _, app := range []string{libraryApp, "com.lab126.KPPMainApp", "com.lab126.booklet.home"} {
		if !shouldRestoreLibraryAfterWake(app) {
			t.Fatal(app)
		}
	}
	for _, app := range []string{"com.lab126.booklet.reader", "com.lab126.booklet.settings", "com.lab126.webview", ""} {
		if shouldRestoreLibraryAfterWake(app) {
			t.Fatal(app)
		}
	}
}
