// Command noisefree removes background noise from your microphone in real time,
// on any GPU or CPU, and exposes the clean audio as a virtual microphone.
package main

import (
	_ "embed"
	"image"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/noisefree/internal/core"
	"github.com/xmhrnx/noisefree/ui"
)

//go:embed assets/icon.png
var icon []byte

func main() {
	if err := ui.SetupTheme(); err != nil {
		log.Fatal(err)
	}
	svc, warn := core.Open()
	if svc == nil {
		log.Fatal(warn)
	}
	if warn != nil {
		log.Print(warn)
	}

	var root *ui.App
	app := giode.New(giode.Config{
		Title:              "NoiseFree",
		Icon:               icon,
		Size:               image.Pt(1240, 820),
		MinSize:            image.Pt(760, 600),
		Resizable:          true,
		Center:             true,
		Frameless:          ui.Frameless,
		Background:         "theme.background",
		CloseToTray:        svc.Settings().CloseToTray && giode.TraySupported(),
		StartInTray:        giode.StartInTrayAtLogin,
		SingleInstanceLock: &giode.SingleInstanceLock{UniqueID: "io.github.xmhrnx.noisefree"},
		OnReady:            func() { root.Start() },
		OnShutdown:         svc.Close,
	})
	root = ui.New(app, svc)
	root.SetupTray(icon)
	svc.Pipe.OnStop = root.AudioStopped
	// Ending the app from a terminal or the session manager still removes
	// the virtual microphone.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		svc.Close()
		os.Exit(0)
	}()
	if err := app.Run(root.View); err != nil {
		log.Fatal(err)
	}
}
