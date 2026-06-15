---
name: add-audio-format
description: Add support for a new audio file extension (e.g. .aiff, .wma) to AudioToText, wiring the file-picker filter and deciding native-vs-ffmpeg-conversion. Use when the user wants the app to accept another audio format.
allowed-tools: Bash, Read, Edit
---

# add-audio-format — support a new audio extension

Add an audio extension end-to-end so users can pick it and transcribe it.

## Decide: native or converted?

`whisper-cli` decodes only `wav`, `mp3`, `ogg`, `flac` directly. Everything else
must be converted to 16 kHz mono WAV via `ffmpeg` first.

- If the new format is one whisper-cli reads natively → add it to `nativeFormats`
  in [transcribe.go](transcribe.go).
- Otherwise → leave it out of `nativeFormats`; the existing `convertToWav` path
  in [transcribe.go](transcribe.go) handles it automatically (requires `ffmpeg`).

When unsure whether a format is native, treat it as **needing conversion** — the
fallback path is safe.

## Steps

1. **File picker filter** — add the extension (with leading dot, lowercase) to
   the `storage.NewExtensionFileFilter` list in `chooseFile` in
   [main.go](main.go).

2. **Native decode (only if native)** — add the extension to the `nativeFormats`
   map in [transcribe.go](transcribe.go).

3. **Docs** — update the supported-formats list in [README.md](README.md),
   placing the extension under "directly" or "via auto-conversion" accordingly.

4. **Verify** — run the project gate:

   ```sh
   go build ./... && go vet ./... && gofmt -l .
   ```

   (Or invoke the `check` skill.)

## Notes

- Extensions are matched case-insensitively (`strings.ToLower` /
  `strings.EqualFold`), but list them lowercase with a leading dot.
- Do not duplicate an extension that is already present.
- Conversion always produces a temp WAV that is removed via `defer os.Remove`;
  do not change that cleanup behavior.
