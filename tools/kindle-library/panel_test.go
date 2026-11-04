package main

import (
	"bytes"
	"testing"
)

func TestParseBrightnessLevel(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0", 0},
		{"12", 12},
		{" 24\n", 24},
		{"", -1},
		{"25", -1},
		{"-1", -1},
		{"abc", -1},
	}
	for _, c := range cases {
		if got := parseBrightnessLevel(c.in); got != c.want {
			t.Errorf("parseBrightnessLevel(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestBrightnessFromX(t *testing.T) {
	cases := []struct {
		x    int
		want int
	}{
		{brightnessTrackX - 50, 0},
		{brightnessTrackX, 0},
		{brightnessTrackX + brightnessTrackW/2, brightnessLevels / 2},
		{brightnessTrackX + brightnessTrackW, brightnessLevels},
		{brightnessTrackX + brightnessTrackW + 50, brightnessLevels},
	}
	for _, c := range cases {
		if got := brightnessFromX(c.x); got != c.want {
			t.Errorf("brightnessFromX(%d) = %d, want %d", c.x, got, c.want)
		}
	}
}

func TestBrightnessFillWidth(t *testing.T) {
	if got := brightnessFillWidth(0); got != 0 {
		t.Errorf("level 0 fill = %d, want 0", got)
	}
	if got := brightnessFillWidth(brightnessLevels); got != brightnessTrackW {
		t.Errorf("level 24 fill = %d, want %d", got, brightnessTrackW)
	}
	mid := brightnessFillWidth(brightnessLevels / 2)
	if mid <= 0 || mid >= brightnessTrackW {
		t.Errorf("level 12 fill = %d, want between 0 and %d", mid, brightnessTrackW)
	}
}

func TestPanelHit(t *testing.T) {
	cases := []struct {
		name   string
		x, y   int
		action panelAction
		level  int
	}{
		{"track", brightnessTrackX + brightnessTrackW/2, brightnessTrackY + brightnessTrackH/2, panelBrightness, brightnessLevels / 2},
		{"settings row", screenW / 2, settingsRowY + 10, panelSettings, 0},
		{"below panel", screenW / 2, panelBottom + 20, panelClose, 0},
		{"header", screenW / 2, 20, panelClose, 0},
		{"pill", panelPillX + panelPillW/2, panelPillY + 2, panelClose, 0},
		{"panel blank", panelLeft + 10, panelTop + 10, panelNone, 0},
	}
	for _, c := range cases {
		action, level := panelHit(c.x, c.y)
		if action != c.action || level != c.level {
			t.Errorf("%s: panelHit(%d,%d) = (%d,%d), want (%d,%d)",
				c.name, c.x, c.y, action, level, c.action, c.level)
		}
	}
}

func TestPanelOpsCoversPanel(t *testing.T) {
	ops := panelOps(12)
	if len(ops) < 8 {
		t.Fatalf("got %d ops, want at least 8", len(ops))
	}
	first := ops[0]
	if first.Kind != opCls || first.Left != panelLeft || first.Top != panelTop ||
		first.Width != panelWidth || first.Height != panelHeight {
		t.Fatalf("first op is not the panel background: %+v", first)
	}
	last := ops[len(ops)-1]
	if last.Kind != opRefresh || last.Left != panelLeft || last.Top != panelTop ||
		last.Width != panelWidth || last.Height != panelHeight {
		t.Fatalf("last op is not the panel refresh: %+v", last)
	}

	wantTexts := map[string]bool{
		"设置":          false,
		"12/24":       false,
		"未注册设备：商店已禁用": false,
	}
	for _, op := range ops {
		if op.Kind == opText {
			if _, ok := wantTexts[op.Text]; ok {
				wantTexts[op.Text] = true
			}
		}
	}
	for text, seen := range wantTexts {
		if !seen {
			t.Errorf("panel ops missing text %q", text)
		}
	}
	for _, op := range ops {
		if op.Kind == opText && op.Text == "商店" {
			t.Errorf("panel still renders the store entry")
		}
	}

	images := 0
	for _, op := range ops {
		if op.Kind == opImage {
			images++
			if !bytes.HasPrefix(op.Image, []byte("\x89PNG")) {
				t.Errorf("image op does not carry PNG data")
			}
		}
	}
	if images != 3 {
		t.Errorf("got %d image ops, want sun, gear and chevron", images)
	}
}

func TestPanelOpsFallsBackWithoutImages(t *testing.T) {
	old := imageIcons
	imageIcons = false
	t.Cleanup(func() { imageIcons = old })

	ops := panelOps(12)
	seen := map[string]bool{
		"设置":          false,
		"12/24":       false,
		"未注册设备：商店已禁用": false,
	}
	for _, op := range ops {
		if op.Kind == opImage {
			t.Fatalf("panel fallback should not use image ops")
		}
		if op.Kind == opText {
			if _, ok := seen[op.Text]; ok {
				seen[op.Text] = true
			}
		}
	}
	for text, ok := range seen {
		if !ok {
			t.Errorf("panel fallback missing text %q", text)
		}
	}
}

func TestBrightnessOpsCoverTrackAndText(t *testing.T) {
	ops := brightnessOps(7)
	if len(ops) < 4 {
		t.Fatalf("got %d brightness ops, want at least 4", len(ops))
	}
	// The whole refresh region must be cleared first so the old knob's 4px
	// overhang above and below the track cannot leave white stripes.
	first := ops[0]
	if first.Kind != opCls || first.Left != brightnessTrackX ||
		first.Top != panelTop+32 || first.Width != screenW-brightnessTrackX-marginX ||
		first.Height != 56 {
		t.Fatalf("first brightness op is not a full-region clear: %+v", first)
	}
	last := ops[len(ops)-1]
	if last.Kind != opRefresh {
		t.Fatalf("last brightness op is not a refresh: %+v", last)
	}
	seenText := false
	seenFill := false
	for _, op := range ops {
		if op.Kind == opText && op.Text == "7/24" {
			seenText = true
		}
		if op.Kind == opCls && op.Color == "WHITE" && op.Width == brightnessFillWidth(7) {
			seenFill = true
		}
	}
	if !seenText || !seenFill {
		t.Fatalf("brightness ops missing level text or fill (text=%v fill=%v)", seenText, seenFill)
	}
}
