package main

import (
	"fmt"
	"keess/application"
	"log"
	"net/http"
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

	// Create an HTTP server and add the health check handler as a handler
	http.HandleFunc("/health", healthHandler)
	http.ListenAndServe(":8080", nil)

	if !isHelp {
		select {}
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	// Check the health of the server and return a status code accordingly
	if serverIsHealthy() {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Server is healthy")
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Server is not healthy")
	}
}

func serverIsHealthy() bool {
	// Check the health of the server and return true or false accordingly
	// For example, check if the server can connect to the database
	return true
}
