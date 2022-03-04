package request

import (
	"bufio"
	"net"
	"net/http"
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

func Parse(conn net.Conn) *Request {
	req := Request{}
	scnr := bufio.NewScanner(conn)

	if !scnr.Scan() {
		// TODO Handle No Status Line
		panic("No status line!")
	}

	line := scnr.Text()
	split := strings.Split(line, " ")

	// TODO Validate Data
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

	// Read Headers
	for scnr.Scan() {
		line := scnr.Text()

		if line == "" {
			break
		}

		index := strings.Index(line, ":")
		key := line[:index]
		value := line[index+1:]
		req.Headers[key] = value
	}

	// Read Body
	for scnr.Scan() {
		line := scnr.Text()
		req.Body = req.Body + line + "\n"
	}

	return &req
}

func Handle(req *Request) {
	if req == nil {
		panic("nullptr!")
	}

	validateRequest(req)

	switch req.Method {
	case http.MethodGet:
		handleGet(req)
	case http.MethodPost:
		handlePost(req)
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

func handleGet(req *Request) {

}

func handlePost(req *Request) {

}
