BINARY := audiototext
PKG    := .

# Extra flags for the binary, e.g. `make run ARGS="-debug"`.
ARGS   :=

.PHONY: build run tidy vet clean model

# Build. Fyne pulls in CGO + system GL/X11 dependencies; they are the same as
# in OS-MANUL, so no extra setup is needed on the same machine.
build:
	go build -o $(BINARY) $(PKG)

# Always rebuilds first (run depends on build), so a stale version won't run.
# Pass arguments via ARGS.
run: build
	./$(BINARY) $(ARGS)

vet:
	go vet ./...

tidy:
	go mod tidy

# Convenience: download the `small` whisper model into the app cache so it
# works out of the box. Override the model like this:
#   make model MODEL=base
MODEL ?= small
model:
	@mkdir -p $(HOME)/.cache/audiototext/models
	@echo "Downloading ggml-$(MODEL).bin…"
	@curl -L -o $(HOME)/.cache/audiototext/models/ggml-$(MODEL).bin \
		https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-$(MODEL).bin

clean:
	rm -f $(BINARY)
