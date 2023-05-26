build:
		go build -o bin/autobump ./cmd/autobump
		strip -s bin/autobump

run:
		go run ./cmd/autobump

install:
		cp -v bin/autobump /usr/local/bin/autobump
