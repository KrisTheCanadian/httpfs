package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

type Options struct {
	Verbose bool
	Port    int
	Path    string
}

func Parse() *Options {
	opts := Options{}
	verboseFlag := flag.Bool("v", false, "Prints debugging messages.")
	portFlag := flag.Int("p", 8080, "Specifies the port number that the server will listen and serve at.\nDefault is 8080.")
	directoryFlag := flag.String("d", "", "Specifies the directory that the server will use to read/write\nrequested files. Default is the current directory when\nlaunching the application.")

	if len(os.Args) < 2 {
		UsageErrorMessage()
	}
	flag.Parse()
	opts.Verbose = *verboseFlag
	opts.Port = *portFlag
	opts.Path = filepath.Clean(*directoryFlag)

	return &opts
}

func UsageErrorMessage() {
	fmt.Println("httpfs is a simple file server.\nusage: httpfs [-v] [-p PORT] [-d PATH-TO-DIR]" +
		"\n-v Prints debugging messages." +
		"\n-p Specifies the port number that the server will listen and serve at.\nDefault is 8080." +
		"\n-d Specifies the directory that the server will use to read/write\nrequested files. " +
		"Default is the current directory when\nlaunching the application.")
	os.Exit(1)
}
