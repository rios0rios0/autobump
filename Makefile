SCRIPTS_DIR ?= $(HOME)/Development/github.com/rios0rios0/pipelines
-include $(SCRIPTS_DIR)/makefiles/common.mk
-include $(SCRIPTS_DIR)/makefiles/golang.mk

# The build version comes from the latest versioned heading in CHANGELOG.md,
# which is AutoBump's own source of truth for releases. Git tags can lag behind
# the CHANGELOG when a release pipeline is interrupted, so a tag is only used as
# a fallback (and finally "dev"). An explicit VERSION from the environment/CLI
# always wins thanks to ?=.
VERSION ?= $(shell { grep -oE '^\#\# \[[0-9]+\.[0-9]+\.[0-9]+\]' CHANGELOG.md 2>/dev/null | head -n1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+'; } || { git describe --tags --abbrev=0 2>/dev/null || echo "dev"; } | sed 's/^v//')
LDFLAGS := -X main.version=$(VERSION)

.PHONY: debug build build-musl install run

build:
	rm -rf bin
	go build -ldflags "$(LDFLAGS) -s -w" -o bin/autobump ./cmd/autobump

debug:
	rm -rf bin
	go build -gcflags "-N -l" -ldflags "$(LDFLAGS)" -o bin/autobump ./cmd/autobump

build-musl:
	CGO_ENABLED=1 CC=musl-gcc go build \
		-ldflags "$(LDFLAGS) -linkmode external -extldflags='-static' -s -w" -o bin/autobump ./cmd/autobump

run:
	go run ./cmd/autobump

install:
	make build
	mkdir -p ~/.local/bin
	cp -v bin/autobump ~/.local/bin/autobump
