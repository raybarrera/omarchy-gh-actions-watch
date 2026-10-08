package ghactions

import (
	"bytes"
	"strconv"
	"strings"
)

// ghResponse is the parsed result of a `gh api -i` call: the HTTP status line,
// the headers we care about, and the body. gh prints all of this to stdout even
// for non-2xx statuses (and exits 1), so status drives interpretation, not the
// exit code.
type ghResponse struct {
	Status     int
	ETag       string
	Body       []byte
	Remaining  int // X-Ratelimit-Remaining, -1 when unknown
	Reset      int64
	RetryAfter int
}

// parseGHResponse splits a `gh api -i` transcript into status, headers, and body.
// The status line ends with LF while header lines end with CRLF, and headers are
// separated from the body by a blank CRLF line.
func parseGHResponse(out []byte) ghResponse {
	resp := ghResponse{Remaining: -1}
	if !bytes.HasPrefix(out, []byte("HTTP/")) {
		return resp
	}

	var head []byte
	if h, b, ok := bytes.Cut(out, []byte("\r\n\r\n")); ok {
		head, resp.Body = h, b
	} else if h, b, ok := bytes.Cut(out, []byte("\n\n")); ok {
		head, resp.Body = h, b
	} else {
		head = out
	}

	for i, line := range bytes.Split(head, []byte("\n")) {
		line = bytes.TrimRight(line, "\r")
		if i == 0 {
			parts := bytes.SplitN(line, []byte(" "), 3)
			if len(parts) >= 2 {
				resp.Status, _ = strconv.Atoi(string(parts[1]))
			}
			continue
		}
		kv := bytes.SplitN(line, []byte(":"), 2)
		if len(kv) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(string(kv[0])))
		val := strings.TrimSpace(string(kv[1]))
		switch key {
		case "etag":
			resp.ETag = val
		case "x-ratelimit-remaining":
			resp.Remaining, _ = strconv.Atoi(val)
		case "x-ratelimit-reset":
			resp.Reset, _ = strconv.ParseInt(val, 10, 64)
		case "retry-after":
			resp.RetryAfter, _ = strconv.Atoi(val)
		}
	}
	return resp
}
