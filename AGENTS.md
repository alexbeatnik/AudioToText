# AGENTS.md

Guidance for AI agents working in the **AudioToText** repository.

## What this project is

A small desktop utility that transcribes audio to text. The transcription
engine is [whisper.cpp](https://github.com/ggml-org/whisper.cpp) (`whisper-cli`)
invoked as a subprocess; the UI is built with [Fyne](https://fyne.io). It is
modeled after the OS-MANUL project and is deliberately kept simple and CGO-free
on the speech-to-text side — the only runtime dependencies are `whisper-cli` and
a `.bin` model on disk (plus `ffmpeg` for non-native audio formats).

## Layout

| File | Responsibility |
|------|----------------|
| [main.go](main.go) | Fyne UI, app state, model/language preferences, layout, all user actions. The `main()` function wires widgets and callbacks. |
| [transcribe.go](transcribe.go) | The transcription engine: locating `whisper-cli`, optional `ffmpeg` conversion, running the subprocess, reading back the `.txt` transcript. No UI code here. |
| [Makefile](Makefile) | `build`, `run`, `vet`, `tidy`, `clean`, and `model` (downloads a whisper model into the cache). |
| [README.md](README.md) | User-facing docs: features, dependencies, build/run, usage. |

Keep the **UI/engine split**: `transcribe.go` must stay free of Fyne imports so
the transcription logic remains testable and reusable.

## Build, run, verify

Always run these from the repo root before considering a change done:

```sh
go build ./...        # must compile
go vet ./...          # must be clean
gofmt -l .            # must print nothing (run `gofmt -w <file>` to fix)
```

If `staticcheck` is available, also run `staticcheck ./...` — it has caught real
issues here (deprecated Fyne APIs, redundant loops). Run it via
`go run honnef.co/go/tools/cmd/staticcheck@latest ./...` when not installed.

- `make build` produces the `audiototext` binary; `make run` rebuilds and runs.
- Running the app needs a graphical session and a real `whisper-cli` + model to
  actually transcribe; the binary builds and starts without them, but a full
  end-to-end transcription cannot be verified headlessly.
- There are no tests yet. If you add logic to `transcribe.go`, prefer adding a
  `transcribe_test.go` with table-driven tests (e.g. for format detection).

## Conventions

- **Go formatting is non-negotiable** — every file must pass `gofmt`. The
  default tool stack (`go build`, `go vet`, `gofmt`) gates every change.
- **Comments explain *why*, not *what*** — match the existing style: short,
  full-sentence comments above non-obvious blocks, in English.
- **UI strings are in English**; keep them concise and user-facing.
- **Error handling**: surface engine errors to the user via `dialog.ShowError`;
  in `transcribe.go` wrap errors with `%w` and include captured `stderr` so the
  user sees what `whisper-cli`/`ffmpeg` actually reported.
- **Goroutines + Fyne**: any widget mutation from a background goroutine MUST be
  wrapped in `fyne.Do(...)`. The transcription runs off the UI thread — see the
  timer and worker goroutines in `runTranscription`.
- **No new heavy dependencies.** The STT path is intentionally CGO-free; do not
  add bindings that pull in C libraries for transcription.

## Key behaviors to preserve

- **Native vs. converted formats**: `nativeFormats` in [transcribe.go](transcribe.go)
  lists what `whisper-cli` decodes directly (`wav/mp3/ogg/flac`). Everything
  else is converted to 16 kHz mono WAV via `ffmpeg` first. If you add a UI file
  filter extension, decide whether it is native or needs conversion.
- **Model discovery**: `scanModels()` in [main.go](main.go) scans three cache
  directories for `.bin` files. The last-used model and language are persisted
  via Fyne `Preferences`.
- **Temp files**: transcription and conversion write to `os.CreateTemp` outputs
  that are always cleaned up with `defer os.Remove(...)`. Keep this — never write
  next to the user's source file.

## When making changes

1. Make the smallest change that satisfies the request.
2. Run the build/vet/gofmt gate above.
3. Do **not** commit or push unless explicitly asked.
4. Update [README.md](README.md) if you change user-facing behavior, dependencies,
   supported formats, or Makefile targets.
