build:
	rm -rf bin
	go build -o bin/autobump ./cmd/autobump
	strip -s bin/autobump

build-musl:
	CGO_ENABLED=1 CC=musl-gcc go build \
		--ldflags 'linkmode external -extldflags="-static"' -o bin/autobump ./cmd/autobump
	strip -s bin/autobump

run:
	go run ./cmd/autobump

install:
	make build
	sudo cp -v bin/autobump /usr/local/bin/autobump

exportkey:
	gpg --export-secret-key --armor $(git config user.signingkey) > ~/.gnupg/autobump.asc
