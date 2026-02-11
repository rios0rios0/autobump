SCRIPTS_DIR ?= $(HOME)/Development/github.com/rios0rios0/pipelines
-include $(SCRIPTS_DIR)/makefiles/common.mk
-include $(SCRIPTS_DIR)/makefiles/golang.mk

build:
	rm -rf bin
	go build -o bin/autobump .
	strip -s bin/autobump

debug:
	rm -rf bin
	go build -gcflags "-N -l" -o bin/autobump .

build-musl:
	CGO_ENABLED=1 CC=musl-gcc go build \
		--ldflags 'linkmode external -extldflags="-static"' -o bin/autobump .
	strip -s bin/autobump

run:
	go run .

install:
	sudo cp -v bin/autobump /usr/local/bin/autobump
