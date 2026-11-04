package main

import "os/exec"

func launchJobman(books []Book) {
	readerMode.Store(true)
	grabTouch(false)
	if nativeDisplay != nil {
		nativeDisplay.hide()
	}
	go func() {
		cmd := exec.Command("/mnt/us/jobman/jobman")
		cmd.Dir = "/mnt/us/jobman"
		logf("opening jailbreak manager")
		if err := cmd.Run(); err != nil {
			logf("jobman exit: %v", err)
		}
		if nativeDisplay != nil {
			activateNativeHome()
			return
		}
		drawPage(books, int(pageNum.Load()))
		grabTouch(true)
		readerMode.Store(false)
	}()
}
