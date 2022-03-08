package response

import (
	"fmt"
	"strings"
)

func newResponse(status int, protocol string, version float64, headers map[string]string, body string) {
	responseSB := strings.Builder{}
	responseSB.WriteString(protocol + "/" + fmt.Sprintf("%v", version))
}
