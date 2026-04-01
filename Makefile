SCRIPTS_DIR ?= $(HOME)/Development/github.com/rios0rios0/pipelines
-include $(SCRIPTS_DIR)/makefiles/common.mk
-include $(SCRIPTS_DIR)/makefiles/golang.mk

VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "dev")
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
