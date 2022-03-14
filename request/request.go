package request

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"httpfs/cli"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Data struct {
	Name    string
	Content string
}

type Request struct {
	Method   string
	Url      string
	Protocol string
	Version  string
	Headers  map[string]string
	Body     string
}

type JsonPostData struct {
	Name        string `json:"name"`
	Content     string `json:"content"`
	IsDirectory bool   `json:"isDirectory,omitempty"`
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

	// Clean Body
	req.Body = strings.Replace(req.Body, "\x00", "", -1)
	return &req
}

func Handle(req *Request, opts *cli.Options) (*Data, error) {
	var err error
	if req == nil {
		panic("nullptr!")
	}
	validateRequest(req)
	var data *Data
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

// {"name": "value", "content": "content value","isDirectory": false}
// isDirectory will be false by default (allows the user to create a directory)
// name corresponds to the file name or directory name (depending on the boolean value of isDirectory)
func write(req *Request, opts *cli.Options) (*Data, error) {
	// checking if json is valid
	var err error
	var jsonData JsonPostData
	var data Data
	dec := json.NewDecoder(bytes.NewReader([]byte(req.Body)))
	dec.DisallowUnknownFields()
	parsedJson, _ := json.MarshalIndent(jsonData, "", "  ")
	if err := dec.Decode(&jsonData); err != nil {
		err = errors.New(strconv.Itoa(http.StatusBadRequest))
		return nil, err
	}
	fmt.Println(parsedJson)

	path, err := validatePath(req, opts)
	if err != nil {
		return nil, err
	}
	requestBodyBytes := []byte(jsonData.Content)
	if jsonData.IsDirectory {
		newPath := path + jsonData.Name
		err := validateDirCreation(newPath, req, opts)
		if err != nil {
			err = errors.New(strconv.Itoa(http.StatusUnauthorized))
			return nil, err
		}
		err = os.MkdirAll(newPath, os.ModePerm)
		if err != nil {
			err = errors.New(strconv.Itoa(http.StatusInternalServerError))
			return nil, err
		}
		data.Name = newPath
		data.Content = ""
	} else {
		err = os.WriteFile(path, requestBodyBytes, 0644)

		data.Name = path
		data.Content = string(requestBodyBytes)

		if err != nil {
			err = errors.New(strconv.Itoa(http.StatusInternalServerError))
			return nil, err
		}
	}

	return &data, nil
}

func read(req *Request, opts *cli.Options) (*Data, error) {
	path, err := validatePath(req, opts)
	if err != nil {
		return nil, err
	}
	fmt.Println("Opening a file " + path)
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	fileInfo, err := file.Stat()

	if fileInfo.IsDir() {
		fileSB := strings.Builder{}
		files, err := ioutil.ReadDir(path)
		if err != nil {
			log.Fatal(err)
		}
		for _, file := range files {
			if file.IsDir() {
				fileSB.WriteString(file.Name() + "/" + "\n")
			} else {
				fileSB.WriteString(file.Name() + "\n")
			}
		}
		d := Data{Name: path, Content: fileSB.String()}
		return &d, err
	}

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
		// TODO Handle Error
		panic("cannot read file.")
	}
	split := strings.Split(path, "/")
	fileName := split[len(split)-1]
	FileData := Data{Name: fileName, Content: buffer.String()}
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

func validateDirCreation(dir string, req *Request, opts *cli.Options) error {
	path := filepath.Clean(opts.Path + req.Url + dir)
	dirRootTree := strings.Split(opts.Path, "/")
	reqRootTree := strings.Split(path, "/")
	var err error
	if len(reqRootTree) < len(dirRootTree) {
		err = errors.New(strconv.Itoa(http.StatusForbidden))
		return err
	}
	for i, node := range dirRootTree {
		if node != reqRootTree[i] {
			err = errors.New(strconv.Itoa(http.StatusForbidden))
			return err
		}
	}
	return err
}
