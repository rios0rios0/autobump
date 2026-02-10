package main

import (
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/rios0rios0/autobump/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Errorf("Uncaught error: %v", err)
		os.Exit(1)
	}
}
