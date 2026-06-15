---
name: check
description: Build, vet, format-check and static-analyze the AudioToText Go project. Use after any code change to confirm it compiles and is clean before finishing or committing.
allowed-tools: Bash, Read, Edit
---

# check — verify the AudioToText project

Run the full quality gate for this Go project and fix anything it flags.

## Steps

1. From the repo root, run the gate:

   ```sh
   go build ./...
   go vet ./...
   gofmt -l .
   ```

   - `go build` must succeed.
   - `go vet` must print nothing.
   - `gofmt -l .` must print nothing. If it lists a file, run
     `gofmt -d <file>` to see the diff, then `gofmt -w <file>` to fix it.

2. Run static analysis (it has caught real bugs here — deprecated Fyne APIs,
   redundant loops):

   ```sh
   go run honnef.co/go/tools/cmd/staticcheck@latest ./...
   ```

   Address every finding. Common ones in this repo:
   - `SA1019` deprecated API → switch to the suggested replacement (e.g. use
     `App.Clipboard()` instead of `Window.Clipboard()`).
   - `S1011` redundant append loop → replace with `append(dst, src...)`.

3. If tests exist, run `go test ./...`.

4. Report the result as a short table (build / vet / gofmt / staticcheck /
   tests, each pass or fail) and list any fixes you made.

## Notes

- Keep `transcribe.go` free of Fyne imports — the engine must stay UI-agnostic.
- Any widget update from a goroutine must be inside `fyne.Do(...)`.
- Do not commit or push unless explicitly asked.
