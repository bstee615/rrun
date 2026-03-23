.PHONY: build dev install install-deb install-rpm clean deb rpm packages \
        test test-packages check-version \
        release release-aur release-ppa release-copr

export PATH := $(PATH):$(HOME)/go/bin

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/bstee615/rrun/cmd.version=$(VERSION)
BINARY   := $(PWD)/rrun

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

# Dev install: build once, then symlink /usr/local/bin/rrun → repo binary.
# After this, 'make build' or 'go build .' updates the binary without sudo.
dev: build
	sudo ln -sf $(BINARY) /usr/local/bin/rrun
	@echo "Symlinked /usr/local/bin/rrun → $(BINARY)"
	@echo "From now on: 'go build .' (or 'make build') to update, no sudo needed."

test:
	go test ./...

deb: build
	mkdir -p dist
	VERSION=$(VERSION) nfpm package --packager deb --target dist/

rpm: build
	mkdir -p dist
	VERSION=$(VERSION) nfpm package --packager rpm --target dist/

packages: deb rpm

install-deb: deb
	sudo dpkg -i dist/rrun_$(VERSION)_amd64.deb

install-rpm: rpm
	sudo dnf install -y dist/rrun-$(VERSION)-1.x86_64.rpm

install: build
	sudo install -Dm755 $(BINARY) /usr/local/bin/rrun

test-packages: packages
	./scripts/test-packages.sh

# Guard: must be a clean semver tag (e.g. v1.2.3), no -dirty or commit hash.
check-version:
	@echo "$(VERSION)" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$$' \
	    || (echo "error: '$(VERSION)' is not a clean tag. Commit all changes and: git tag vX.Y.Z" >&2; exit 1)

release-aur:
	./scripts/release-aur.sh

release-ppa:
	./scripts/release-ppa.sh

release-copr:
	./scripts/release-copr.sh

release: check-version clean test packages test-packages
	gh release create $(VERSION) dist/* \
		--title "rrun $(VERSION)" \
		--generate-notes
	$(MAKE) release-aur
# 	$(MAKE) release-ppa
# 	$(MAKE) release-copr

clean:
	rm -f $(BINARY)
	rm -rf dist/
