.PHONY: build dev install install-deb install-rpm clean deb rpm packages

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

deb: build
	VERSION=$(VERSION) nfpm package --packager deb --target dist/

rpm: build
	VERSION=$(VERSION) nfpm package --packager rpm --target dist/

packages: deb rpm

install-deb: deb
	sudo dpkg -i dist/rrun_$(VERSION)_amd64.deb

install-rpm: rpm
	sudo rpm -i dist/rrun-$(VERSION)-1.x86_64.rpm

install:
	@if command -v dpkg > /dev/null 2>&1; then \
		$(MAKE) install-deb; \
	elif command -v rpm > /dev/null 2>&1; then \
		$(MAKE) install-rpm; \
	else \
		$(MAKE) build && sudo install -Dm755 $(BINARY) /usr/local/bin/rrun; \
	fi

clean:
	rm -f $(BINARY)
	rm -rf dist/
