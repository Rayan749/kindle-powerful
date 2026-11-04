package main

import (
	"bufio"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"
)

var screenAsleep atomic.Bool
var libraryBeforeSleep atomic.Bool

const libraryApp = "org.kindlepowerful.library"

func runNativeHome(books []Book) {
	readerMode.Store(true)
	go touchLoop(books) // observes the reader escape gesture; X owns library taps
	curStatus.Batt = -1
	refreshStatus()
	drawPage(books, 0)
	cmd := exec.Command("/usr/bin/lua", "/mnt/us/library-lifecycle.lua")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logf("lifecycle pipe: %v", err)
		return
	}
	if err = cmd.Start(); err != nil {
		logf("lifecycle start: %v", err)
		return
	}
	defer cmd.Process.Kill()
	go statusLoop(books)
	go nativeDisplay.events(books)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "ready" {
			go func() {
				_, err := runOutput(5*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "start", "app://"+libraryApp)
				if err != nil {
					logf("native home activation: %v", err)
				}
			}()
			continue
		}
		fields := strings.SplitN(line, "\t", 2)
		if len(fields) != 2 {
			continue
		}
		name, value := fields[0], fields[1]
		if name == "power" {
			if value == "sleep" {
				libraryBeforeSleep.Store(!readerMode.Load() && activeApp() == libraryApp)
				screenAsleep.Store(true)
				logf("screen sleeping; library=%v", libraryBeforeSleep.Load())
			} else if value == "wake" {
				screenAsleep.Store(false)
				if libraryBeforeSleep.Swap(false) {
					go restoreLibraryAfterWake()
				}
			}
			continue
		}
		id := strings.SplitN(value, ":", 2)[0]
		logf("lifecycle %s id=%s", name, id)
		switch name {
		case "go":
			readerMode.Store(true)
			refreshStatus()
			drawPage(books, int(pageNum.Load()))
			nativeDisplay.show()
			readerMode.Store(false)
		case "pause", "unload":
			readerMode.Store(true)
			nativeDisplay.hide()
		case "load":
		default:
			continue
		}
		if _, err := runOutput(3*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.appmgrd", name+"result", id+":0"); err != nil {
			logf("lifecycle reply %s: %v", name, err)
		}
	}
	logf("lifecycle exited: %v", scanner.Err())
}
func activateNativeHome() {
	if _, err := runOutput(5*time.Second, "/usr/bin/lipc-set-prop", "com.lab126.appmgrd", "start", "app://"+libraryApp); err != nil {
		logf("native return error: %v", err)
	}
}

func shouldRestoreLibraryAfterWake(app string) bool {
	return app == libraryApp || app == "com.lab126.KPPMainApp" || app == "com.lab126.booklet.home"
}
func restoreLibraryAfterWake() {
	for attempt := 0; attempt < 20; attempt++ {
		if screenAsleep.Load() {
			return
		}
		state := lipcGet("com.lab126.powerd", "state")
		app := activeApp()
		if state == "active" && shouldRestoreLibraryAfterWake(app) {
			logf("wake restoring native library")
			activateNativeHome()
			return
		}
		if app != "" && !shouldRestoreLibraryAfterWake(app) {
			logf("wake preserving native foreground")
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	logf("wake restore deferred: power state not ready")
}
