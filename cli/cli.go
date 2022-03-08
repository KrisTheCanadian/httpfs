package cli

import (
	"flag"
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
	directoryFlag := flag.String("d", "./", "Specifies the directory that the server will use to read/write\nrequested files. Default is the current directory when\nlaunching the application.")

	flag.Parse()
	opts.Verbose = *verboseFlag
	opts.Port = *portFlag
	opts.Path, _ = filepath.Abs(*directoryFlag)
	opts.Path = filepath.Clean(opts.Path)

	return &opts
}
