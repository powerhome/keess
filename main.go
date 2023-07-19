package main

import (
	"keess/application"
	"log"
	"os"
	"strings"
)

func main() {
	app := application.New()

	if error := app.Run(os.Args); error != nil {
		log.Fatal(error)
	}

	isHelp := false
	for _, arg := range os.Args {
		if strings.Contains(arg, "--help") || strings.HasPrefix(arg, "-h") {
			isHelp = true
		}
	}

	if !isHelp {
		select {}
	}
}
