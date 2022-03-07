package main

import (
	"httpfs/cli"
	"httpfs/serve"
	"io"
	"log"
)

func main() {
	opts := cli.Parse()
	if !opts.Verbose {
		log.SetOutput(io.Discard)
	}

	log.Println(opts)
	log.Println("Verbose Enabled.")

	serve.Serve(opts)
}
