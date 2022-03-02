package main

import (
	"httpfs/cli"
	"httpfs/serve"
	"io"
	"log"
	"strconv"
)

func main() {
	opts := cli.Parse()
	if !opts.Verbose {
		log.SetOutput(io.Discard)
	}

	log.Println(opts)
	log.Println("Verbose Enabled.")

	serve.Serve(strconv.Itoa(opts.Port))
}
