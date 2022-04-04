package response

import (
	"fmt"
	"httpfs/request"
	"log"
	"net/http"
	"strings"
)

func SendNewResponse(status int, protocol string, version string, headers *map[string]string, body string) string {
	responseSB := strings.Builder{}
	// Writing Response Status Line
	responseSB.WriteString(protocol + "/" + version)
	responseSB.WriteString(" " + fmt.Sprintf("%v", status))
	responseSB.WriteString(" " + http.StatusText(status) + "\r\n")
	for key, value := range *headers {
		responseSB.WriteString(key + ": " + value + "\r\n")
	}
	responseSB.WriteString("\r\n")
	responseSB.WriteString(body)
	responseString := responseSB.String()
	log.Println(responseString)
	return responseString
}

func NewResponseHeaders(req *request.Request) (*map[string]string, bool) {
	stayConnected := true
	headers := make(map[string]string, 10)
	headers["Server"] = "httpfs"
	headers["Connection"] = "keep-alive"
	if (req.Protocol == "HTTP" && req.Version == "1.0") || req.Headers["Connection"] != "keep-alive" {
		headers["Connection"] = "close"
		stayConnected = false
	}

	return &headers, stayConnected
}

func SendHTTPError(status int, protocol string, version string) string {
	headers := make(map[string]string, 10)
	headers["Server"] = "httpfs"
	headers["Connection"] = "close"
	responseSB := strings.Builder{}
	// Writing Response Status Line
	responseSB.WriteString(protocol + "/" + version)
	responseSB.WriteString(" " + fmt.Sprintf("%v", status))
	responseSB.WriteString(" " + http.StatusText(status) + "\r\n")
	for key, value := range headers {
		responseSB.WriteString(key + ": " + value + "\r\n")
	}
	responseSB.WriteString("\r\n")
	responseString := responseSB.String()
	log.Println(responseString)
	return responseString
}
