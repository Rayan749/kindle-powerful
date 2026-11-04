package main

import (
	"crypto/sha256"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

var libraryPixels [32]byte
var libraryPixelsValid bool

// Hash only list rows below the quick panel, leaving clock changes alone.
func listPixels() ([32]byte, bool) {
	var zero [32]byte
	raw, err := os.ReadFile("/sys/class/graphics/fb0/stride")
	if err != nil {
		return zero, false
	}
	stride, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || stride < screenW || stride > screenW*8 {
		return zero, false
	}
	f, err := os.Open("/dev/fb0")
	if err != nil {
		return zero, false
	}
	defer f.Close()
	data := make([]byte, stride*(screenH-300))
	n, err := f.ReadAt(data, int64(stride*300))
	if err != nil && err != io.EOF || n != len(data) {
		return zero, false
	}
	return sha256.Sum256(data), true
}

func canReclaimLibrary(app, power string, native bool) bool {
	return !native && power == "active" && (app == "com.lab126.KPPMainApp" || app == "com.lab126.booklet.home" || app == "com.lab126.webview")
}

func watchLibrary(books []Book) {
	for {
		time.Sleep(time.Second)
		if readerMode.Load() || panelOpen.Load() {
			continue
		}
		app := activeApp()
		power := lipcGet("com.lab126.powerd", "state")
		if !canReclaimLibrary(app, power, readerMode.Load()) {
			continue
		}
		drawMu.Lock()
		pixels, ok := listPixels()
		changed := ok && libraryPixelsValid && pixels != libraryPixels && !readerMode.Load() && !panelOpen.Load()
		drawMu.Unlock()
		if changed {
			// Recheck after sampling so a native launch never loses its screen.
			if readerMode.Load() || !canReclaimLibrary(activeApp(), power, false) {
				continue
			}
			logf("native home overwrote library; reclaiming")
			drawPageChecked(books, int(pageNum.Load()), true)
		}
	}
}
