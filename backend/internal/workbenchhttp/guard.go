package workbenchhttp

import (
	"net/http"
)

const BrowserCommandLimit int64 = 96 * 1024
const TerminalCommandLimit int64 = 16 * 1024

func ProtectResponse(headers http.Header) {
	headers.Set("Cache-Control", "no-store, private")
	headers.Set("Pragma", "no-cache")
	headers.Set("X-Content-Type-Options", "nosniff")
}

func LimitRequest(w http.ResponseWriter, r *http.Request, maxBytes int64) bool {
	if maxBytes <= 0 {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		if r.ContentLength > maxBytes {
			return false
		}
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		}
	}
	return true
}
