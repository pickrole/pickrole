// PickRole: sign in with AWS IAM Identity Center, pick an account and a
// role, and get working credentials in every terminal.
package main

import (
	"context"
	"embed"
	"errors"
	"log"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/pickrole/pickrole/internal/app"
	"github.com/pickrole/pickrole/internal/clipboard"
	"github.com/pickrole/pickrole/internal/i18n"
	"github.com/pickrole/pickrole/internal/update"
)

// Set at build time with -ldflags "-X main.version=… -X main.commit=… -X main.buildDate=…"
// (see scripts/build-*.{sh,ps1}). An empty commit falls back to the VCS stamp.
var (
	version   = "dev"
	commit    = ""
	buildDate = ""
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

// platform adapts the Wails runtime to app.Platform.
type platform struct{ ctx context.Context }

// jsonFilter is built on each call so it follows the current language.
func jsonFilter() []runtime.FileFilter {
	return []runtime.FileFilter{{DisplayName: i18n.T("app.json_filter"), Pattern: "*.json"}}
}

func (p platform) OpenURL(url string) { runtime.BrowserOpenURL(p.ctx, url) }

func (p platform) SetClipboard(text string) error { return runtime.ClipboardSetText(p.ctx, text) }

// SetSecretClipboard keeps credentials out of the Windows clipboard history
// and cloud sync; elsewhere it is a plain copy.
func (p platform) SetSecretClipboard(text string) error {
	if err := clipboard.SetSecret(text); !errors.Is(err, clipboard.ErrUnsupported) {
		return err
	}
	return runtime.ClipboardSetText(p.ctx, text)
}

func (p platform) Emit(event string, data ...any) { runtime.EventsEmit(p.ctx, event, data...) }

func (p platform) Quit() { runtime.Quit(p.ctx) }

func (p platform) OpenFile(title string) (string, error) {
	return runtime.OpenFileDialog(p.ctx, runtime.OpenDialogOptions{Title: title, Filters: jsonFilter()})
}

func (p platform) SaveFile(title, defaultName string) (string, error) {
	return runtime.SaveFileDialog(p.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: defaultName,
		Filters:         jsonFilter(),
	})
}

func main() {
	// After an update, the new version waits for the old one to exit (single
	// instance), and removes what the old one left behind.
	update.WaitForPrevious(os.Args)
	update.Cleanup()

	svc, start := app.New(app.Build{Version: version, Commit: commit, Date: buildDate})
	var appCtx context.Context

	err := wails.Run(&options.App{
		Title:            "PickRole",
		Width:            760,
		Height:           600,
		MinWidth:         680,
		MinHeight:        520,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 19, G: 20, B: 24, A: 255},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			start(ctx, platform{ctx})
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "io.github.pickrole.pickrole",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				if appCtx != nil {
					runtime.WindowUnminimise(appCtx)
					runtime.WindowShow(appCtx)
				}
			},
		},
		Bind: []interface{}{svc},
		Linux: &linux.Options{
			Icon:             icon,
			ProgramName:      "pickrole",
			WebviewGpuPolicy: linux.WebviewGpuPolicyOnDemand,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
