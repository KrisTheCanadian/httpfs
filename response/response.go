package response

import (
	"fmt"
	"net/http"
	"strings"
)

func SendNewResponse(status int, protocol string, version float64, headers *map[string]string, body string) {
	responseSB := strings.Builder{}
	// Writing Response Status Line
	responseSB.WriteString(protocol + "/" + fmt.Sprintf("%v", version))
	responseSB.WriteString(" " + fmt.Sprintf("%v", status))
	responseSB.WriteString(" " + http.StatusText(status) + "\r\n")
	for key, value := range *headers {
		responseSB.WriteString(key + ": " + value + "\r\n")
	}
	responseSB.WriteString("\r\n")
	responseSB.WriteString(body)
	// TODO Forward this request to client using sockets
	fmt.Println(responseSB.String())
}

func NewResponseHeaders() *map[string]string {
	headers := make(map[string]string, 10)
	headers["Server"] = "httpfs"
	headers["Connection"] = "keep-alive"

	return &headers
}
