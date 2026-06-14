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
		models := scanModels()
		names := make([]string, 0, len(models))
		for _, m := range models {
			names = append(names, m)
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

	runTranscription := func() {
		if audioPath == "" {
			dialog.ShowInformation("No file", "Select an audio file first.", w)
			return
		}
		if modelPath == "" {
			dialog.ShowInformation("No model", "Select a whisper model (.bin).", w)
			return
		}
		transcribeBtn.Disable()
		progress.Show()
		lang := langSelect.Selected
		start := time.Now()

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
						status.SetText(fmt.Sprintf("Transcribing… %s", fmtDuration(elapsed)))
					})
				}
			}
		}()
		status.SetText("Transcribing… 0s")

		go func() {
			text, err := Transcribe(context.Background(), "", modelPath, audioPath, lang)
			close(done)
			elapsed := time.Since(start)
			fyne.Do(func() {
				progress.Hide()
				transcribeBtn.Enable()
				if err != nil {
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
		w.Clipboard().SetContent(editor.Text)
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
		container.NewBorder(nil, nil, widget.NewButton("Select audio", chooseFile), nil, fileLabel),
		container.NewBorder(nil, nil, widget.NewLabel("Model:"),
			container.NewHBox(
				widget.NewButton("Browse…", chooseModel),
				widget.NewButton("Download Whisper", downloadWhisper),
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
