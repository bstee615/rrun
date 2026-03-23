.PHONY: build dev install clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X rrun/cmd.version=$(VERSION)
BINARY   := $(PWD)/rrun

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Dev install: build once, then symlink /usr/local/bin/rrun → repo binary.
# After this, 'make build' or 'go build .' updates the binary without sudo.
dev: build
	sudo ln -sf $(BINARY) /usr/local/bin/rrun
	@echo "Symlinked /usr/local/bin/rrun → $(BINARY)"
	@echo "From now on: 'go build .' (or 'make build') to update, no sudo needed."

# Production install: copy the binary (no symlink).
install: build
	sudo install -Dm755 $(BINARY) /usr/local/bin/rrun

clean:
	rm -f $(BINARY)
