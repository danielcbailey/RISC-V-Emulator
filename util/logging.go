package util

import (
	"fmt"
	"net/http"
	"strings"
)

var LoggingEnabled = false

func LogF(format string, args ...interface{}) {
	if !LoggingEnabled {
		return
	}
	message := fmt.Sprintf(format, args...)
	go http.Post("http://localhost:8006/log", "text/plain", strings.NewReader(message))
}
