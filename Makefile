build:
        go build -o bin/autobump ./cmd/autobump/main.go
        strip -s bin/autobump

run:
        go run ./cmd/autobump/main.go

install:
        cp -v bin/autobump /usr/local/bin/autobump
