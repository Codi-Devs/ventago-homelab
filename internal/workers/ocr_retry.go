package workers

import (
	"regexp"
	"strconv"
	"strings"
)

var httpStatusRe = regexp.MustCompile(`(?i)HTTP (\d{3})`)

func httpStatusFromError(err error) int {
	if err == nil {
		return 0
	}
	m := httpStatusRe.FindStringSubmatch(err.Error())
	if len(m) != 2 {
		return 0
	}
	n, convErr := strconv.Atoi(m[1])
	if convErr != nil {
		return 0
	}
	return n
}

func isTransientAPIError(err error) bool {
	if err == nil {
		return false
	}
	code := httpStatusFromError(err)
	if code == 408 || code == 425 || code == 429 || code >= 500 {
		return true
	}
	if code == 401 || code == 403 {
		return true
	}
	msg := strings.ToLower(err.Error())
	needles := []string{
		"connection refused",
		"connection reset",
		"i/o timeout",
		"timeout",
		"temporary",
		"eof",
		"broken pipe",
		"no such host",
		"network is unreachable",
		"no route to host",
		"tls handshake",
		"server closed",
		"dial tcp",
	}
	for _, n := range needles {
		if strings.Contains(msg, n) {
			return true
		}
	}
	return false
}
