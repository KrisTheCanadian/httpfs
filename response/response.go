package response

import (
	"fmt"
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
	fmt.Println(responseString)
	return responseString
}

func NewResponseHeaders() *map[string]string {
	headers := make(map[string]string, 10)
	headers["Server"] = "httpfs"
	headers["Connection"] = "keep-alive"

	return &headers
}
