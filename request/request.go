package request

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"httpfs/cli"
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
	Version  string
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
	req.Version = strings.Replace(split2[1], "\x00", "", -1)

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

func Handle(req *Request, opts *cli.Options) (*FileData, error) {
	var err error
	if req == nil {
		panic("nullptr!")
	}
	validateRequest(req)
	var data *FileData
	switch req.Method {
	case http.MethodGet:
		data, err = read(req, opts)
	case http.MethodPost:
		data, err = write(req, opts)
	default:
		err = errors.New(strconv.Itoa(http.StatusMethodNotAllowed))
	}
	return data, err
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
	if req.Version != "1.0" && req.Version != "1.1" {
		// TODO Handle response
		panic("HTTP Version is not supported.")
	}
}

// TODO
func write(req *Request, opts *cli.Options) (*FileData, error) {
	_, err := validatePath(req, opts)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func read(req *Request, opts *cli.Options) (*FileData, error) {
	path, err := validatePath(req, opts)
	if err != nil {
		return nil, err
	}
	fmt.Println("Opening a file " + path)
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		return nil, errors.New(strconv.Itoa(http.StatusNotFound))
	}
	defer func(file *os.File) {
		err = file.Close()
		err = errors.New(strconv.Itoa(http.StatusNotFound))
	}(file)

	var buffer bytes.Buffer
	scnr := bufio.NewScanner(file)

	for scnr.Scan() {
		buffer.WriteString(scnr.Text() + "\n")
	}

	if err := scnr.Err(); err != nil {
		// TODO List Directory Files
	}
	split := strings.Split(path, "/")
	fileName := split[len(split)-1]
	FileData := FileData{FileName: fileName, Content: buffer.String()}
	return &FileData, err
}

func validatePath(req *Request, opts *cli.Options) (string, error) {
	path := filepath.Clean(opts.Path + req.Url)
	dirRootTree := strings.Split(opts.Path, "/")
	reqRootTree := strings.Split(path, "/")
	var err error
	if len(reqRootTree) < len(dirRootTree) {
		err = errors.New(strconv.Itoa(http.StatusForbidden))
		return "", err
	}
	for i, node := range dirRootTree {
		if node != reqRootTree[i] {
			err = errors.New(strconv.Itoa(http.StatusForbidden))
			return "", err
		}
	}
	return path, err
}
