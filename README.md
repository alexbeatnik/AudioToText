# AudioToText

A simple desktop utility for transcribing audio to text. The engine is
[whisper.cpp](https://github.com/ggml-org/whisper.cpp) (`whisper-cli`) run as a
subprocess, with a UI built on [Fyne](https://fyne.io). Modeled after the
OS-MANUL project.

## Features

- Audio file selection:
  - `wav`, `mp3`, `ogg`, `flac` — directly;
  - `m4a`, `opus`, `webm`, `aac`… — via auto-conversion (requires `ffmpeg`).
- Choice of whisper model (`.bin`) and language (auto / uk / en / ru / …)
- One-click transcription
- A text editor where the result can be corrected, **copied**, or **saved**
  as `.txt`
- A live **execution timer** and the final elapsed time in the status bar
- **Remembers** the last selected model and language between runs

## Dependencies

1. **whisper-cli** — from whisper.cpp, must be in `PATH` (or in
   `~/.local/bin`, `/usr/local/bin`, `/usr/bin`, `/opt/whisper.cpp/bin`).
2. **ffmpeg** *(optional)* — needed only for formats that whisper-cli cannot
   read on its own (`m4a`, `opus`, `webm`, `aac`…). Not required for
   `wav/mp3/ogg/flac`. Install with: `sudo apt install ffmpeg`.
3. **A `.bin` model** — the app automatically picks up models from:
   - `~/.cache/os-manul/models/whisper/`
   - `~/.cache/audiototext/models/`
   - `~/.local/share/whisper.cpp/models/`

   Any other model can be selected with the **Browse…** button.

For example, the `small` model can be downloaded like this:

```sh
mkdir -p ~/.cache/audiototext/models
curl -L -o ~/.cache/audiototext/models/ggml-small.bin \
  https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.bin
```

## Build and run

```sh
make build      # build the binary
make run        # build and run
make model      # download the small model into the cache (make model MODEL=base for another)
make vet        # go vet
make tidy       # go mod tidy
make clean      # remove the binary
```

Or without make:

```sh
go build -o audiototext .
./audiototext
```

## Usage

1. **Select audio** → choose a file.
2. Change the **Model** and **Language** if needed.
3. **Transcribe** → wait (the indicator shows progress).
4. The text appears in the editor — **Copy** or **Save…**.
