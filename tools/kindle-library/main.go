package main

import (
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	screenW = 1072
	screenH = 1448
	fbink   = "/mnt/us/libkh/bin/fbink"
	docRoot = "/mnt/us/documents"
	logPath = "/mnt/us/kindle-library.log"

	fontTitle  = "/usr/java/lib/fonts/STHeitiMedium.ttf"
	fontBold   = "/usr/java/lib/fonts/STHeitiBold.ttf"
	fontAuthor = "/usr/java/lib/fonts/STSongMedium.ttf"

	titlePx  = 34
	authorPx = 24
	headerPx = 28

	headerH   = 48
	listTop   = 62
	rowH      = 116
	titleOff  = 0
	authorOff = 42

	marginX       = 40
	maxTitleCols  = 60
	maxAuthorCols = 84
)

type Book struct {
	Path   string
	Title  string
	Author string
}

var ebookExt = map[string]bool{
	".mobi": true, ".azw3": true, ".azw": true,
	".pdf": true, ".epub": true, ".txt": true,
}

func logf(format string, args ...any) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, args...))
}

func runeWidth(r rune) int {
	switch {
	case r >= 0x1100 && r <= 0x115F,
		r >= 0x2E80 && r <= 0xA4CF,
		r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF,
		r >= 0xFE30 && r <= 0xFE4F,
		r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6:
		return 2
	}
	return 1
}

func truncate(s string, max int) string {
	var b strings.Builder
	width := 0
	for _, r := range s {
		w := runeWidth(r)
		if width+w > max {
			b.WriteString("...")
			break
		}
		b.WriteRune(r)
		width += w
	}
	return b.String()
}

func clean(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

// exthMeta reads the MOBI/AZW3 EXTH block from the first MiB of the file.
func exthMeta(path string) (title, author string) {
	f, err := os.Open(path)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	buf := make([]byte, 256<<10)
	n, _ := f.Read(buf)
	d := buf[:n]
	if len(d) < 100 {
		return "", ""
	}
	rec0 := int(binary.BigEndian.Uint32(d[78:82]))
	if rec0 <= 0 || rec0+24 > len(d) {
		return "", ""
	}
	if string(d[rec0+16:rec0+20]) != "MOBI" {
		return "", ""
	}
	mobiLen := int(binary.BigEndian.Uint32(d[rec0+20 : rec0+24]))
	exth := rec0 + 16 + mobiLen
	if exth+12 > len(d) || string(d[exth:exth+4]) != "EXTH" {
		return "", ""
	}
	count := int(binary.BigEndian.Uint32(d[exth+8 : exth+12]))
	off := exth + 12
	for i := 0; i < count && off+8 <= len(d); i++ {
		typ := int(binary.BigEndian.Uint32(d[off : off+4]))
		ln := int(binary.BigEndian.Uint32(d[off+4 : off+8]))
		if ln < 8 || off+ln > len(d) {
			break
		}
		data := d[off+8 : off+ln]
		switch typ {
		case 100:
			if author == "" {
				author = clean(string(data))
			}
		case 503:
			if title == "" {
				title = clean(string(data))
			}
		}
		off += ln
	}
	return title, author
}

func filenameMeta(path string) (string, string) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	title, author := base, filepath.Base(filepath.Dir(path))
	if idx := strings.LastIndex(base, " - "); idx > 0 {
		title = base[:idx]
		author = base[idx+3:]
	}
	return clean(title), clean(author)
}

func scanBooks() []Book {
	var books []Book
	filepath.Walk(docRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasSuffix(strings.ToLower(name), ".sdr") ||
				strings.EqualFold(name, "dictionaries") ||
				strings.EqualFold(name, "downloads") ||
				strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		name := info.Name()
		if strings.HasPrefix(name, "._") {
			return nil
		}
		if !ebookExt[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		lower := strings.ToLower(name)
		if strings.Contains(lower, "kppmainappv2_") ||
			strings.Contains(lower, "jobman") ||
			strings.Contains(lower, "my clippings") ||
			strings.Contains(lower, "crash") {
			return nil
		}
		title, author := exthMeta(path)
		ft, fa := filenameMeta(path)
		if title == "" {
			title = ft
		}
		if author == "" {
			author = fa
		}
		books = append(books, Book{
			Path:   path,
			Title:  title,
			Author: author,
		})
		return nil
	})
	sort.Slice(books, func(i, j int) bool {
		return strings.ToLower(books[i].Title) < strings.ToLower(books[j].Title)
	})
	return books
}

func booksPerPage() int {
	return 12
}

func pageCount(total int) int {
	per := booksPerPage()
	if total == 0 {
		return 1
	}
	return (total + per - 1) / per
}

func fbinkText(top, left, px int, font, color, text string, invert bool) {
	if nativeDisplay != nil {
		nativeDisplay.op(fbinkOp{Kind: opText, X: left, Y: top, Px: px, Font: font, Color: color, Text: text, Invert: invert})
		return
	}
	spec := fmt.Sprintf("regular=%s,px=%d,top=%d,left=%d", font, px, top, left)
	args := []string{"-q", "-b", "-t", spec}
	if color != "" {
		args = append(args, "-C", color)
	}
	if invert {
		args = append(args, "-h")
	}
	// "--" keeps a leading dash in a title from being parsed as an option.
	args = append(args, "--", text)
	out, err := exec.Command(fbink, args...).CombinedOutput()
	if err != nil {
		logf("fbink error: %v out=%s", err, string(out))
	}
}

func fbinkRefresh() {
	if nativeDisplay != nil {
		nativeDisplay.op(fbinkOp{Kind: opRefresh})
		return
	}
	exec.Command(fbink, "-q", "-s").Run()
}

func fbinkClear() {
	if nativeDisplay != nil {
		nativeDisplay.op(fbinkOp{Kind: opCls, Width: screenW, Height: screenH, Color: "WHITE"})
		return
	}
	if out, err := exec.Command(fbink, "-q", "-b", "-c").CombinedOutput(); err != nil {
		logf("fbink clear error: %v out=%s", err, string(out))
	}
}

var (
	drawMu     sync.Mutex
	pageNum    atomic.Int32
	curStatus  statusState
	panelOpen  atomic.Bool
	readerMode atomic.Bool
)

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func startInBrightnessTrack(x, y int) bool {
	return x >= brightnessTrackX && x < brightnessTrackX+brightnessTrackW &&
		y >= brightnessTrackY-12 && y < brightnessTrackY+brightnessTrackH+12
}

func drawBookRow(b Book, y int) {
	fbinkText(y+titleOff, marginX, titlePx, fontTitle, "BLACK", truncate(b.Title, maxTitleCols), false)
	fbinkText(y+authorOff, marginX, authorPx, fontAuthor, "GRAY5", truncate(b.Author, maxAuthorCols), false)
	executeOps([]fbinkOp{{Kind: opImage, X: marginX, Y: y + separatorOffset, Image: bookSeparator()}})
}

func readerForeground() bool {
	return strings.Contains(activeApp(), "booklet.reader")
}

// drawHeaderLocked redraws only the header band. The caller must hold drawMu.
// When partial is true it refreshes just the bar; a full page redraw refreshes
// the whole screen afterwards instead.
func drawHeaderLocked(books []Book, page int, partial bool) {
	st := curStatus
	st.Clock = readClock()
	p := makeHeaderPlan(st, page, pageCount(len(books)))
	ops := headerOps(p)
	if !partial {
		ops = ops[:len(ops)-1]
	}
	executeOps(ops)
}

func drawPage(books []Book, page int) { drawPageChecked(books, page, false) }

func drawPageChecked(books []Book, page int, onlyLibrary bool) {
	drawMu.Lock()
	defer drawMu.Unlock()
	if onlyLibrary && (readerMode.Load() || panelOpen.Load()) {
		return
	}
	panelOpen.Store(false)
	per := booksPerPage()
	total := pageCount(len(books))
	start := page * per
	end := start + per
	if end > len(books) {
		end = len(books)
	}
	logf("draw page %d/%d books %d-%d per=%d", page+1, total, start, end, per)

	fbinkClear()
	drawHeaderLocked(books, page, false)

	y := listTop
	if len(books) == 0 {
		fbinkText(y, marginX, titlePx, fontTitle, "BLACK", "没有找到可打开的书籍", false)
	} else {
		for _, b := range books[start:end] {
			drawBookRow(b, y)
			y += rowH
		}
	}
	fbinkRefresh()
	if nativeDisplay == nil {
		libraryPixels, libraryPixelsValid = listPixels()
	}
}

// drawCoveredRowsLocked repaints the book rows hidden by the panel. The caller
// must hold drawMu and must have cleared the panel rectangle to white first.
func drawCoveredRowsLocked(books []Book, page int) {
	per := booksPerPage()
	first := 0
	last := (panelBottom - listTop) / rowH
	if last >= per {
		last = per - 1
	}
	for row := first; row <= last; row++ {
		idx := page*per + row
		if idx >= len(books) {
			break
		}
		drawBookRow(books[idx], listTop+row*rowH)
	}
}

func openPanel() {
	drawMu.Lock()
	defer drawMu.Unlock()
	if panelOpen.Load() {
		return
	}
	panelOpen.Store(true)
	level := readBrightness()
	executeOps(panelOps(level))
}

func closePanel(books []Book, page int) {
	drawMu.Lock()
	defer drawMu.Unlock()
	if !panelOpen.Load() {
		return
	}
	panelOpen.Store(false)
	if nativeDisplay != nil {
		executeOps([]fbinkOp{{Kind: opCls, Top: panelTop, Left: panelLeft, Width: panelWidth, Height: panelHeight, Color: "WHITE"}})
		drawCoveredRowsLocked(books, page)
		executeOps([]fbinkOp{{Kind: opRefresh, Top: panelTop, Width: screenW, Height: panelHeight}})
		return
	}
	spec := fmt.Sprintf("top=%d,left=%d,width=%d,height=%d",
		panelTop, panelLeft, panelWidth, panelHeight)
	if out, err := exec.Command(fbink, "-q", "-b", "-B", "WHITE", "-k", spec).CombinedOutput(); err != nil {
		logf("panel clear error: %v out=%s", err, string(out))
	}
	if len(books) == 0 {
		fbinkText(listTop, marginX, titlePx, fontTitle, "BLACK", "没有找到可打开的书籍", false)
	} else {
		drawCoveredRowsLocked(books, page)
	}
	exec.Command(fbink, "-q", "-s", spec).Run()
}

func updatePanelBrightness(level int) {
	setBrightness(level)
	drawMu.Lock()
	defer drawMu.Unlock()
	if !panelOpen.Load() {
		return
	}
	executeOps(brightnessOps(level))
}

func launchSettings(books []Book) {
	readerMode.Store(true)
	grabTouch(false)
	logf("launching settings")
	// Clear the raw framebuffer first so library text cannot remain in areas
	// the Settings window does not repaint.
	if nativeDisplay != nil {
		nativeDisplay.hide()
	}
	if nativeDisplay == nil {
		if out, err := exec.Command(fbink, "-q", "-c").CombinedOutput(); err != nil {
			logf("settings pre-clear error: %v out=%s", err, string(out))
		}
	}
	if out, err := runOutput(5*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "start",
		"app://com.lab126.booklet.settings"); err != nil {
		logf("settings launch error: %v out=%s", err, string(out))
		if nativeDisplay != nil {
			nativeDisplay.show()
		}
		grabTouch(true)
		readerMode.Store(false)
		return
	}
	// Settings is usable on this unregistered device, but the library must not
	// paint over it. Status/header drawing already skips restricted apps; this
	// watcher restores the library once Settings is left.
	go func() {
		seen := false
		for i := 0; i < 10; i++ {
			if strings.Contains(activeApp(), "settings") {
				seen = true
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		if !seen {
			logf("settings did not become active")
			if nativeDisplay != nil {
				nativeDisplay.show()
			}
			grabTouch(true)
			readerMode.Store(false)
			return
		}
		repaintNative()
		empty := 0
		for {
			gone, next := appGone(activeApp(), "settings", empty)
			empty = next
			if gone {
				break
			}
			time.Sleep(time.Second)
		}
		if nativeDisplay != nil {
			activateNativeHome()
			return
		}
		time.Sleep(500 * time.Millisecond)
		// Keep KPP alive: home/chrome and annotation services depend on it.
		time.Sleep(500 * time.Millisecond)
		drawPage(books, int(pageNum.Load()))
		if nativeDisplay != nil {
			nativeDisplay.show()
		}
		grabTouch(true)
		readerMode.Store(false)
	}()
}

func activeApp() string {
	out, err := runOutput(2*time.Second, "/usr/bin/lipc-get-prop", "com.lab126.appmgrd", "activeApp")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// The native reader asks KPP for its annotation controller and profile state.
// With KPP stopped, sideloaded books throw ReaderExceptionDialog. Start KPP
// first (the reader still takes the foreground) and keep it alive for the
// duration of the reading session.
func ensureKppForReader() bool {
	if kppServicesReady() {
		return true
	}
	for attempt := 0; attempt < 2; attempt++ {
		logf("starting KPP for reader annotation services (attempt %d)", attempt+1)
		if !kppRunning() {
			exec.Command("/sbin/start", "kppmainapp").Run()
		}
		// KPP registers KPPAnnotationController from its JS bundle; wait for the
		// LIPC source instead of guessing a fixed startup delay.
		for i := 0; i < 60; i++ {
			if kppServicesReady() {
				return true
			}
			time.Sleep(500 * time.Millisecond)
		}
		logf("KPP annotation service did not appear; waiting again")
		// Keep KPP alive: home/chrome and annotation services depend on it.
		time.Sleep(2 * time.Second)
	}
	return false
}

func openBook(b Book, books []Book) {
	// From launch until the reader closes, the reader owns the screen. Ignore
	// touch input and suppress every library redraw; otherwise a top-edge tap
	// meant for the reader toolbar can paint the library over it. The session
	// runs in its own goroutine so touchLoop can still catch the return gesture.
	readerMode.Store(true)
	showSelectedBook(b, books)
	grabTouch(false)
	showBanner("正在打开：" + truncate(b.Title, 28) + "…")
	logf("reader session start")
	go runReaderSession(b, books)
}

// showBanner gives immediate feedback while KPP starts and the reader loads.
func showBanner(text string) {
	if nativeDisplay != nil {
		drawMu.Lock()
		defer drawMu.Unlock()
		executeOps([]fbinkOp{{Kind: opCls, Top: screenH - 84, Width: screenW, Height: 84, Color: "BLACK"}})
		fbinkText(screenH-62, marginX, 28, fontBold, "", text, true)
		executeOps([]fbinkOp{{Kind: opRefresh, Top: screenH - 84, Width: screenW, Height: 84}})
		return
	}
	drawMu.Lock()
	defer drawMu.Unlock()
	top := screenH - 84
	spec := fmt.Sprintf("top=%d,left=0,width=%d,height=84", top, screenW)
	if out, err := exec.Command(fbink, "-q", "-b", "-B", "BLACK", "-k", spec).CombinedOutput(); err != nil {
		logf("banner bar error: %v out=%s", err, string(out))
	}
	fbinkText(top+22, marginX, 28, fontBold, "", text, true)
	exec.Command(fbink, "-q", "-s", spec).Run()
}

func returnFromReader() {
	logf("reader return gesture; closing reader")
	// The reader toolbar is not reachable on this deregistered firmware.
	// Close the reader through appmgr without launching legacy home.
	// KPP stays alive for native chrome and annotation services.
	if out, err := runOutput(5*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "stop",
		"com.lab126.booklet.reader"); err != nil {
		logf("return error: %v out=%s", err, string(out))
	}
}

func runReaderSession(b Book, books []Book) {
	defer func() {
		logf("reader session end")
		if nativeDisplay != nil {
			activateNativeHome()
			return
		}
		// The system may bring KPP back when the reader closes; keep it out of
		// the way and give the reader a moment to finish its own teardown.
		// Keep KPP alive: home/chrome and annotation services depend on it.
		time.Sleep(1 * time.Second)
		drawPage(books, int(pageNum.Load()))
		// Keep the reader-mode guard until the library is fully repainted so a
		// tap during cleanup cannot start a second session underneath this draw.
		if nativeDisplay != nil {
			nativeDisplay.show()
		}
		grabTouch(true)
		readerMode.Store(false)
	}()

	if !ensureKppForReader() {
		logf("KPP annotation service unavailable; not opening reader")
		return
	}

	// The native KPP opens a book as "file://" + encodeFilePath(path); the
	// app://...reader/<path> form is different and rejects some sideloaded
	// MOBI/AZW3 files on a deregistered device.
	if nativeDisplay != nil {
		nativeDisplay.hide()
	}
	uri := "file://" + (&url.URL{Path: b.Path}).EscapedPath()
	logf("open %q uri=%s", b.Title, uri)
	out, err := runOutput(5*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "start", uri)
	if err != nil {
		logf("open error: %v out=%s", err, string(out))
		return
	}
	seen := false
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(activeApp(), "booklet.reader") {
			seen = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !seen && strings.Contains(activeApp(), "booklet.reader") {
		seen = true
		logf("reader became active late")
	}
	if !seen {
		logf("reader did not become active")
		return
	}
	repaintNative()
	empty := 0
	for {
		gone, next := appGone(activeApp(), "booklet.reader", empty)
		empty = next
		if gone {
			return
		}
		time.Sleep(time.Second)
	}
}

func kppRunning() bool {
	out, err := runOutput(2*time.Second, "/sbin/status", "kppmainapp")
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "start/running")
}

// statusLoop refreshes only the header when the clock, battery or Wi-Fi state
// changes. It stays out of the way while the native reader or KPP owns the screen.
func statusLoop(books []Book) {
	for {
		if screenAsleep.Load() {
			time.Sleep(time.Second)
			continue
		}
		drawMu.Lock()
		previous := curStatus
		drawMu.Unlock()
		refreshStatus()
		app := activeApp()
		drawMu.Lock()
		if curStatus != previous && !screenAsleep.Load() && libraryOwnsScreen(readerMode.Load()) && canDrawHeader(app) {
			drawHeaderLocked(books, int(pageNum.Load()), true)
		}
		drawMu.Unlock()
		time.Sleep(10 * time.Second)
	}

}

func handleTap(x, y, startX, startY int, books []Book) {
	if !libraryOwnsScreen(readerMode.Load()) || screenAsleep.Load() {
		return
	}
	if nativeDisplay != nil && !nativeDisplay.acceptsInput() {
		return
	}
	// A cancelled contact or a multi-touch slot can leave -1 coordinates.
	if x < 0 || y < 0 || startX < 0 || startY < 0 {
		return
	}
	per := booksPerPage()
	total := pageCount(len(books))
	cur := int(pageNum.Load())
	logf("tap x=%d y=%d start=%d,%d page=%d", x, y, startX, startY, cur+1)

	dx, dy := 0, 0
	if startX >= 0 && startY >= 0 && x >= 0 && y >= 0 {
		dx, dy = x-startX, y-startY
	}
	isTap := abs(dx) < 30 && abs(dy) < 30

	if panelOpen.Load() {
		if startInBrightnessTrack(startX, startY) {
			updatePanelBrightness(brightnessFromX(x))
			return
		}
		if dy < -100 {
			closePanel(books, cur)
			return
		}
		if !isTap {
			return
		}
		action, level := panelHit(x, y)
		switch action {
		case panelClose:
			closePanel(books, cur)
		case panelBrightness:
			updatePanelBrightness(level)
		case panelJobman:
			closePanel(books, cur)
			launchJobman(books)
		case panelSettings:
			closePanel(books, cur)
			launchSettings(books)
		}
		return
	}

	// A downward swipe that starts on the top edge opens the quick panel.
	if startY >= 0 && startY < 80 && dy > 100 {
		openPanel()
		return
	}

	setPage := func(p int) {
		pageNum.Store(int32(p))
		drawPage(books, p)
	}

	if abs(dx) > 100 && abs(dx) > abs(dy) {
		if dx < 0 && cur < total-1 {
			setPage(cur + 1)
		} else if dx > 0 && cur > 0 {
			setPage(cur - 1)
		}
		return
	}
	if startY > 0 && y > 0 {
		if dy < -100 {
			if cur < total-1 {
				setPage(cur + 1)
			}
			return
		}
		if dy > 100 {
			if cur > 0 {
				setPage(cur - 1)
			}
			return
		}
	}
	if y < headerH {
		if cur > 0 {
			setPage(cur - 1)
		}
		return
	}
	if y > screenH-54 {
		if cur < total-1 {
			setPage(cur + 1)
		}
		return
	}
	row := bookRowAt(y)
	if row < 0 || row >= per {
		return
	}
	idx := cur*per + row
	if idx < len(books) {
		openBook(books[idx], books)
	}
}

var touchFile atomic.Pointer[os.File]

func grabTouch(grab bool) {
	if nativeDisplay != nil {
		return
	}
	f := touchFile.Load()
	if f == nil {
		return
	}
	value := uintptr(0)
	if grab {
		value = 1
	}
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(0x40044590), value)
	if err != 0 {
		logf("touch grab=%v error=%v", grab, err)
	}
}

func touchLoop(books []Book) {
	f, err := os.Open("/dev/input/event2")
	if err != nil {
		logf("input open error: %v", err)
		return
	}
	defer f.Close()
	touchFile.Store(f)
	var ev struct {
		Sec   int32
		Usec  int32
		Type  uint16
		Code  uint16
		Value int32
	}
	x, y := -1, -1
	startX, startY := -1, -1
	ended := false
	var lastBrightness time.Time
	var gestureStart time.Time
	for {
		if err := binary.Read(f, binary.LittleEndian, &ev); err != nil {
			logf("input read error: %v", err)
			return
		}
		switch {
		case ev.Type == 3 && ev.Code == 53:
			x = int(ev.Value)
			if startX < 0 {
				startX = x
			}
			if gestureStart.IsZero() {
				gestureStart = time.Now()
			}
			if panelOpen.Load() && libraryOwnsScreen(readerMode.Load()) &&
				startX >= 0 && startY >= 0 &&
				startInBrightnessTrack(startX, startY) &&
				time.Since(lastBrightness) >= 250*time.Millisecond {
				updatePanelBrightness(brightnessFromX(x))
				lastBrightness = time.Now()
			}
		case ev.Type == 3 && ev.Code == 54:
			y = int(ev.Value)
			if startY < 0 {
				startY = y
			}
			if gestureStart.IsZero() {
				gestureStart = time.Now()
			}
		case ev.Type == 3 && ev.Code == 57:
			if ev.Value == -1 {
				ended = true
			}
		case ev.Type == 0 && ev.Code == 0 && ended:
			if startX < 0 {
				startX = x
			}
			if startY < 0 {
				startY = y
			}
			held := time.Since(gestureStart)
			if readerMode.Load() && readerForeground() && isReaderReturnGesture(startX, startY, x, y, held) {
				returnFromReader()
			} else {
				handleTap(x, y, startX, startY, books)
			}
			startX, startY = -1, -1
			ended = false
			gestureStart = time.Time{}
		}
	}
}

func main() {
	if os.Getenv("KINDLE_DISPLAY") == "x11" {
		var err error
		nativeDisplay, err = newXDisplay()
		if err != nil {
			logf("X startup error: %v", err)
			return
		}
	}
	imageIcons = detectImageIcons()
	books := scanBooks()
	logf("scanned %d books imageIcons=%v", len(books), imageIcons)
	if nativeDisplay != nil {
		runNativeHome(books)
		return
	}
	// Start the touch reader immediately so the top-left long-press can bring
	// KPP forward even while we are waiting for a foreground reader to close.
	readerMode.Store(true)
	go touchLoop(books)
	if readerForeground() {
		logf("reader foreground at startup; waiting")
		empty := 0
		for {
			gone, next := appGone(activeApp(), "booklet.reader", empty)
			empty = next
			if gone {
				break
			}
			time.Sleep(time.Second)
		}
		logf("reader exited; taking over")
	}
	// Settings and Store are restricted on this unregistered device and may
	// still be open from an older build. Never paint the library over them;
	// wait until the user leaves those apps.
	for {
		if !isRestrictedNativeApp(activeApp()) {
			break
		}
		logf("restricted app foreground; waiting before taking over")
		time.Sleep(2 * time.Second)
	}
	// Keep KPP alive: home/chrome and annotation services depend on it.
	time.Sleep(5 * time.Second)
	pageNum.Store(0)
	curStatus.Clock = readClock()
	curStatus.Batt, curStatus.Charging = readBattery()
	curStatus.WifiOn, curStatus.WifiLevel = readWifi()
	drawPage(books, 0)
	if nativeDisplay != nil {
		nativeDisplay.show()
	}
	grabTouch(true)
	readerMode.Store(false)
	// KPP must remain alive; do not run the old stop watchdog.
	go statusLoop(books)
	if nativeDisplay == nil {
		go watchLibrary(books)
	} else {
		nativeDisplay.show()
		go nativeDisplay.events(books)
	}
	select {}
}

// Ask X clients to repaint their backing content after leaving the raw FB UI.
func repaintNative() {
	out, err := runOutput(3*time.Second, "/usr/bin/xrefresh", "-display", ":0")
	if err != nil {
		logf("native repaint error: %v out=%s", err, out)
	}
}

func showSelectedBook(b Book, books []Book) {
	drawMu.Lock()
	defer drawMu.Unlock()
	first := int(pageNum.Load()) * booksPerPage()
	for row := 0; row < booksPerPage() && first+row < len(books); row++ {
		if books[first+row].Path != b.Path {
			continue
		}
		y := listTop + row*rowH
		bounds := bookRowRect(row)
		if nativeDisplay != nil {
			executeOps([]fbinkOp{{Kind: opCls, Top: bounds.Min.Y, Width: screenW, Height: bounds.Dy(), Color: "BLACK"}})
			fbinkText(y+titleOff, marginX, titlePx, fontTitle, "", truncate(b.Title, maxTitleCols), true)
			fbinkText(y+authorOff, marginX, authorPx, fontAuthor, "", truncate(b.Author, maxAuthorCols), true)
			executeOps([]fbinkOp{{Kind: opRefresh, Top: bounds.Min.Y, Width: screenW, Height: bounds.Dy()}})
			return
		}
		spec := fmt.Sprintf("top=%d,left=0,width=%d,height=%d", bounds.Min.Y, screenW, bounds.Dy())
		if out, err := exec.Command(fbink, "-q", "-b", "-B", "BLACK", "-k", spec).CombinedOutput(); err != nil {
			logf("selection error: %v %s", err, out)
			return
		}
		fbinkText(y+titleOff, marginX, titlePx, fontTitle, "", truncate(b.Title, maxTitleCols), true)
		fbinkText(y+authorOff, marginX, authorPx, fontAuthor, "", truncate(b.Author, maxAuthorCols), true)
		exec.Command(fbink, "-q", "-s", spec).Run()
		logf("selected row %d", row)
		return
	}
}
