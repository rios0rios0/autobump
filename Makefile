SCRIPTS_DIR ?= $(HOME)/Development/github.com/rios0rios0/pipelines
-include $(SCRIPTS_DIR)/makefiles/common.mk
-include $(SCRIPTS_DIR)/makefiles/golang.mk

.PHONY: debug build build-musl install run

build:
	rm -rf bin
	go build -o bin/autobump ./cmd/autobump
	strip -s bin/autobump

debug:
	rm -rf bin
	go build -gcflags "-N -l" -o bin/autobump ./cmd/autobump

build-musl:
	CGO_ENABLED=1 CC=musl-gcc go build \
		--ldflags 'linkmode external -extldflags="-static"' -o bin/autobump ./cmd/autobump
	strip -s bin/autobump

run:
	go run ./cmd/autobump

install:
	make build
	mkdir -p ~/.local/bin
	cp -v bin/autobump ~/.local/bin/autobump
