// AudioToText — a simple utility for transcribing audio to text.
//
// Fyne UI: pick an audio file, (optionally) a model and language, press
// "Transcribe" — the text appears in the editor, from where it can be
// copied or saved to a file. The engine is whisper.cpp (whisper-cli) run as
// a subprocess, modeled after the OS-MANUL project.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
)

// languages are offered in the drop-down list. "auto" enables auto-detection.
var languages = []string{"auto", "uk", "en", "ru", "de", "fr", "es", "pl", "it"}

func main() {
	a := app.NewWithID("com.alexbeatnik.audiototext")
	w := a.NewWindow("AudioToText — audio to text")
	w.Resize(fyne.NewSize(760, 560))

	prefs := a.Preferences()

	// --- state ---
	var audioPath string
	// Last model from the previous run; if it's gone — the first one found.
	modelPath := prefs.String("model")
	if modelPath == "" {
		modelPath = defaultModel()
	} else if _, err := os.Stat(modelPath); err != nil {
		modelPath = defaultModel()
	}

	// --- widgets ---
	fileLabel := widget.NewLabel("No file selected")
	fileLabel.Wrapping = fyne.TextWrapWord

	modelSelect := widget.NewSelect(nil, nil)
	refreshModels := func() {
		names := scanModels()
		// Keep a custom (browsed) model selectable across runs: scanModels only
		// looks in the cache dirs, so a model from elsewhere would otherwise
		// vanish from the drop-down even though its path is still valid.
		if modelPath != "" && !contains(names, modelPath) {
			if _, err := os.Stat(modelPath); err == nil {
				names = append(names, modelPath)
			}
		}
		modelSelect.Options = names
		modelSelect.Refresh()
		if modelPath != "" {
			modelSelect.SetSelected(modelPath)
		} else if len(names) > 0 {
			modelSelect.SetSelected(names[0])
		}
	}
	modelSelect.OnChanged = func(s string) {
		modelPath = s
		prefs.SetString("model", s)
	}

	langSelect := widget.NewSelect(languages, func(s string) {
		prefs.SetString("language", s)
	})
	langSelect.SetSelected(prefs.StringWithFallback("language", "uk"))

	editor := widget.NewMultiLineEntry()
	editor.Wrapping = fyne.TextWrapWord
	editor.SetPlaceHolder("The recognized text will appear here…")

	progress := widget.NewProgressBarInfinite()
	progress.Hide()
	status := widget.NewLabel("")

	var transcribeBtn *widget.Button
	// Declared up front so runTranscription can restore the button's default
	// action (it temporarily becomes a "Cancel" button while running).
	var runTranscription func()

	// --- actions ---
	chooseFile := func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			audioPath = rc.URI().Path()
			fileLabel.SetText(audioPath)
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{
			".wav", ".mp3", ".ogg", ".flac", ".m4a", ".opus", ".webm",
		}))
		fd.Show()
	}

	chooseModel := func() {
		fd := dialog.NewFileOpen(func(rc fyne.URIReadCloser, err error) {
			if err != nil || rc == nil {
				return
			}
			defer rc.Close()
			modelPath = rc.URI().Path()
			if !contains(modelSelect.Options, modelPath) {
				modelSelect.Options = append(modelSelect.Options, modelPath)
			}
			modelSelect.SetSelected(modelPath)
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".bin"}))
		fd.Show()
	}

	// downloadWhisper opens the page where whisper.cpp models can be downloaded.
	downloadWhisper := func() {
		u, err := url.Parse("https://huggingface.co/ggerganov/whisper.cpp/tree/main")
		if err != nil {
			return
		}
		if err := a.OpenURL(u); err != nil {
			dialog.ShowError(err, w)
		}
	}

	// Named so they can be disabled while a transcription is running — this,
	// together with snapshotting audioPath/modelPath below, prevents the user
	// from mutating the config the background goroutine is reading (data race).
	selectAudioBtn := widget.NewButton("Select audio", chooseFile)
	browseBtn := widget.NewButton("Browse…", chooseModel)
	downloadBtn := widget.NewButton("Download Whisper", downloadWhisper)

	setInputsEnabled := func(enabled bool) {
		for _, c := range []fyne.Disableable{selectAudioBtn, browseBtn, downloadBtn, modelSelect, langSelect} {
			if enabled {
				c.Enable()
			} else {
				c.Disable()
			}
		}
	}

	runTranscription = func() {
		if audioPath == "" {
			dialog.ShowInformation("No file", "Select an audio file first.", w)
			return
		}
		if modelPath == "" {
			dialog.ShowInformation("No model", "Select a whisper model (.bin).", w)
			return
		}

		// Snapshot the config so the worker reads stable values even though the
		// UI is only partially locked.
		mPath := modelPath
		aPath := audioPath
		lang := langSelect.Selected

		ctx, cancel := context.WithCancel(context.Background())
		start := time.Now()

		// Switch the UI into "running" mode: lock the inputs and turn the
		// Transcribe button into a Cancel button that aborts whisper-cli.
		setInputsEnabled(false)
		progress.Show()
		transcribeBtn.SetText("Cancel")
		transcribeBtn.Importance = widget.DangerImportance
		transcribeBtn.OnTapped = func() {
			cancel()
			transcribeBtn.Disable()
			status.SetText("Canceling…")
		}
		transcribeBtn.Refresh()

		restoreUI := func() {
			progress.Hide()
			setInputsEnabled(true)
			transcribeBtn.SetText("Transcribe")
			transcribeBtn.Importance = widget.HighImportance
			transcribeBtn.OnTapped = runTranscription
			transcribeBtn.Enable()
			transcribeBtn.Refresh()
		}

		// Live timer: refresh the status every second while recognition runs.
		done := make(chan struct{})
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					elapsed := time.Since(start)
					fyne.Do(func() {
						if status.Text != "Canceling…" {
							status.SetText(fmt.Sprintf("Transcribing… %s", fmtDuration(elapsed)))
						}
					})
				}
			}
		}()
		status.SetText("Transcribing… 0s")

		go func() {
			text, err := Transcribe(ctx, "", mPath, aPath, lang)
			close(done)
			cancel() // release the context regardless of outcome
			elapsed := time.Since(start)
			fyne.Do(func() {
				restoreUI()
				if err != nil {
					if ctx.Err() != nil {
						status.SetText("Canceled.")
						return
					}
					status.SetText("Error.")
					dialog.ShowError(err, w)
					return
				}
				editor.SetText(text)
				status.SetText(fmt.Sprintf("Done in %s — %d characters.", fmtDuration(elapsed), len([]rune(text))))
			})
		}()
	}

	transcribeBtn = widget.NewButton("Transcribe", runTranscription)
	transcribeBtn.Importance = widget.HighImportance

	copyBtn := widget.NewButton("Copy", func() {
		a.Clipboard().SetContent(editor.Text)
		status.SetText("Copied to clipboard.")
	})

	clearBtn := widget.NewButton("Clear", func() {
		editor.SetText("")
		status.SetText("")
	})

	saveBtn := widget.NewButton("Save…", func() {
		fd := dialog.NewFileSave(func(wc fyne.URIWriteCloser, err error) {
			if err != nil || wc == nil {
				return
			}
			defer wc.Close()
			if _, err := wc.Write([]byte(editor.Text)); err != nil {
				dialog.ShowError(err, w)
				return
			}
			status.SetText("Saved: " + wc.URI().Path())
		}, w)
		fd.SetFileName("transcript.txt")
		fd.Show()
	})

	refreshModels()

	// --- layout ---
	topControls := container.NewVBox(
		container.NewBorder(nil, nil, selectAudioBtn, nil, fileLabel),
		container.NewBorder(nil, nil, widget.NewLabel("Model:"),
			container.NewHBox(
				browseBtn,
				downloadBtn,
			), modelSelect),
		container.NewBorder(nil, nil, widget.NewLabel("Language:"), nil, langSelect),
		transcribeBtn,
		progress,
	)

	bottomControls := container.NewBorder(nil, nil,
		container.NewHBox(copyBtn, saveBtn, clearBtn), nil, status)

	content := container.NewBorder(
		topControls,    // top
		bottomControls, // bottom
		nil, nil,
		editor, // center — grows to fill the space
	)

	w.SetContent(content)
	w.ShowAndRun()
}

// defaultModel returns the first model found, preferring the OS-MANUL cache
// so the utility works out of the box.
func defaultModel() string {
	models := scanModels()
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

// scanModels looks for whisper .bin models in the standard directories.
func scanModels() []string {
	home, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(home, ".cache", "os-manul", "models", "whisper"),
		filepath.Join(home, ".cache", "audiototext", "models"),
		filepath.Join(home, ".local", "share", "whisper.cpp", "models"),
	}
	var out []string
	seen := map[string]struct{}{}
	for _, d := range dirs {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".bin") {
				continue
			}
			p := filepath.Join(d, e.Name())
			if _, dup := seen[p]; dup {
				continue
			}
			seen[p] = struct{}{}
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// fmtDuration formats a duration like "12s" or "1m 05s".
func fmtDuration(d time.Duration) string {
	secs := int(d.Round(time.Second).Seconds())
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm %02ds", secs/60, secs%60)
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
