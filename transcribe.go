package main

// Transcription via whisper.cpp's `whisper-cli` as a subprocess.
//
// We hand the audio file straight to whisper-cli (it natively decodes
// flac/mp3/ogg/wav) and read the plaintext transcript it writes next to a
// temp output base. This keeps the binary CGO-free for the STT side — the
// only runtime dependency is whisper-cli on PATH plus a .bin model.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// resolveWhisperCLI looks for the whisper-cli executable in PATH and a few
// well-known local prefixes so users don't have to configure it by hand.
func resolveWhisperCLI() string {
	if p, err := exec.LookPath("whisper-cli"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "whisper-cli"),
		"/usr/local/bin/whisper-cli",
		"/usr/bin/whisper-cli",
		"/opt/whisper.cpp/bin/whisper-cli",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "whisper-cli" // let it fail later with a clear exec error
}

// nativeFormats — formats whisper-cli decodes on its own. The rest
// (m4a, opus, webm, aac…) are first converted to wav via ffmpeg.
var nativeFormats = map[string]bool{
	".wav": true, ".mp3": true, ".ogg": true, ".flac": true,
}

// Transcribe runs whisper-cli against audioPath using the given .bin model
// and ISO-639-1 language code ("auto" lets whisper detect it) and returns
// the recognized text. Formats whisper-cli can't read natively are first
// converted to 16 kHz mono WAV with ffmpeg.
func Transcribe(ctx context.Context, binary, modelPath, audioPath, language string) (string, error) {
	if binary == "" {
		binary = resolveWhisperCLI()
	}
	if modelPath == "" {
		return "", errors.New("no model selected")
	}
	if _, err := os.Stat(modelPath); err != nil {
		return "", fmt.Errorf("model not found: %w", err)
	}
	if _, err := os.Stat(audioPath); err != nil {
		return "", fmt.Errorf("audio file not found: %w", err)
	}
	if language == "" {
		language = "auto"
	}

	// whisper-cli decodes only flac/mp3/ogg/wav. Other formats are converted.
	if !nativeFormats[strings.ToLower(filepath.Ext(audioPath))] {
		converted, err := convertToWav(ctx, audioPath)
		if err != nil {
			return "", err
		}
		defer os.Remove(converted)
		audioPath = converted
	}

	// whisper-cli writes <of>.txt; use a temp base so we never touch the
	// directory of the user's source file. We reserve the .txt path itself with
	// CreateTemp (O_EXCL) and let whisper-cli overwrite that file, rather than
	// removing it first — that would leave a window for a symlink/TOCTOU race
	// on the predictable name in a world-writable temp dir.
	tmp, err := os.CreateTemp("", "a2t-*.txt")
	if err != nil {
		return "", err
	}
	txtPath := tmp.Name()
	tmp.Close()
	base := strings.TrimSuffix(txtPath, ".txt")
	defer os.Remove(txtPath)

	cmd := exec.CommandContext(ctx,
		binary,
		"-m", modelPath,
		"-f", audioPath,
		"-l", language,
		"-nt",   // no timestamps in the output
		"-otxt", // write a .txt transcript
		"-of", base,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("whisper-cli exited with an error: %w\n%s", err, stderr.String())
	}

	data, err := os.ReadFile(txtPath)
	if err != nil {
		return "", fmt.Errorf("failed to read transcript: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// convertToWav uses ffmpeg to bring an arbitrary audio file to
// 16 kHz mono 16-bit WAV — a format whisper-cli is guaranteed to read.
// Returns the path to a temporary wav (the caller must remove it).
func convertToWav(ctx context.Context, srcPath string) (string, error) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("the %s format requires ffmpeg for conversion, but ffmpeg was not found in PATH — install it (e.g. `sudo apt install ffmpeg`)", filepath.Ext(srcPath))
	}

	tmp, err := os.CreateTemp("", "a2t-conv-*.wav")
	if err != nil {
		return "", err
	}
	dst := tmp.Name()
	tmp.Close()

	cmd := exec.CommandContext(ctx, ffmpeg,
		"-y",          // overwrite the temp file
		"-i", srcPath, // input
		"-ac", "1", // mono
		"-ar", "16000", // 16 kHz
		"-c:a", "pcm_s16le",
		dst,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		os.Remove(dst)
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("ffmpeg failed to convert the file: %w\n%s", err, stderr.String())
	}
	return dst, nil
}
