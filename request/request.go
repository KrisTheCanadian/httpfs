package request

import (
	"bufio"
	"bytes"
	"fmt"
	"httpfs/cli"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type FileData struct {
	FileName string
	Content  string
}

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

func Handle(req *Request, opts *cli.Options) *FileData {
	if req == nil {
		panic("nullptr!")
	}
	validateRequest(req)
	data := FileData{}
	switch req.Method {
	case http.MethodGet:
		data = *read(req, opts)
	case http.MethodPost:
		data = *write(req, opts)
	default:
		// TODO HTTP Error Message
		panic("Http method cannot be handled")
	}
	return &data
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

// TODO
func write(req *Request, opts *cli.Options) *FileData {
	validatePath(req, opts)
	return nil
}

func read(req *Request, opts *cli.Options) *FileData {
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

	var buffer bytes.Buffer
	scnr := bufio.NewScanner(file)

	for scnr.Scan() {
		buffer.WriteString(scnr.Text() + "\n")
	}

	if err := scnr.Err(); err != nil {
		log.Fatal(err)
	}

	fileName := strings.Split(path, "/")[len(req.Url)-1]
	FileData := FileData{FileName: fileName, Content: buffer.String()}
	return &FileData
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
