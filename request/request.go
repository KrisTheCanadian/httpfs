package request

import (
	"bufio"
	"fmt"
	"httpfs/cli"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Request struct {
	Method   string
	Url      string
	Protocol string
	Version  float64
	Headers  map[string]string
	Body     string
}

func Parse(raw string) *Request {
	req := Request{}
	req.Headers = make(map[string]string, 10)

	if raw == "" {
		// TODO Handle No Status Line
		panic("No status line!")
	}

	lines := strings.Split(raw, "\r\n")
	split := strings.Split(lines[0], " ")

	req.Method = split[0]
	req.Url = split[1]
	split2 := strings.Split(split[2], "/")
	req.Protocol = split2[0]
	version, err := strconv.ParseFloat(split2[1], 64)

	if err != nil {
		// TODO Handle Error using HTTP
		panic("Error converting http version to float64.")
	}
	req.Version = version

	lineCount := 1
	// Reading headers
	for i := lineCount; i < len(lines); i++ {
		lineCount++
		if lines[i] == "" {
			break
		}

		index := strings.Index(lines[i], ":")
		key := lines[i][:index]
		value := lines[i][index+1:]
		req.Headers[key] = value
	}

	// Read Body
	for i := lineCount; i < len(lines); i++ {
		line := lines[i]
		req.Body = req.Body + line + "\n"
	}

	return &req
}

func Handle(req *Request, opts *cli.Options) {
	if req == nil {
		panic("nullptr!")
	}

	validateRequest(req)

	switch req.Method {
	case http.MethodGet:
		handleGet(req, opts)
	case http.MethodPost:
		handlePost(req, opts)
	default:
		// TODO HTTP Error Message
		panic("Http method cannot be handled")
	}

}

func validateRequest(req *Request) {
	if req.Method == "" {
		// TODO Handle missing method HTTP ERROR
		panic("Method is missing")
	}

	if req.Protocol != "HTTP" {
		// TODO HTTP ERROR
		panic("Protocol is unsupported")
	}

	if req.Version != 1.0 && req.Version != 1.1 {
		// TODO Handle response
		panic("HTTP Version is not supported.")
	}
}

func handleGet(req *Request, opts *cli.Options) {
	read(req, opts)
	// Allow to read to file
	// send response with content + status
}

func handlePost(req *Request, opts *cli.Options) {
	// Validate the URL (to the directory of the project)
	// Allow to write to file
	// send response with content + status
}

func write(req *Request, opts *cli.Options) {
	validatePath(req, opts)
}

func read(req *Request, opts *cli.Options) {
	path := validatePath(req, opts)
	fmt.Println("Opening a file ")
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		panic("file reading error")
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic("Error closing file")
		}
	}(file)

	scnr := bufio.NewScanner(file)
	for scnr.Scan() {
		// do something with a line
		fmt.Printf("line: %s\n", scnr.Text())
	}

	if err := scnr.Err(); err != nil {
		log.Fatal(err)
	}
}

func validatePath(req *Request, opts *cli.Options) string {
	path := filepath.Clean(opts.Path + req.Url)
	dirRootTree := strings.Split(opts.Path, "/")
	reqRootTree := strings.Split(path, "/")
	if len(reqRootTree) < len(dirRootTree) {
		panic("access violation")
	}
	for i, node := range dirRootTree {
		if node != reqRootTree[i] {
			panic("access violation")
		}
	}
	return path
}
