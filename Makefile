SCRIPTS_DIR = $(HOME)/Development/github.com/rios0rios0/pipelines

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
	sudo cp -v bin/autobump /usr/local/bin/autobump

.PHONY: lint
lint:
	${SCRIPTS_DIR}/global/scripts/golangci-lint/run.sh --fix .

.PHONY: test
test:
	${SCRIPTS_DIR}/global/scripts/golang/test/run.sh .
