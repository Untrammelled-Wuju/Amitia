package network

import "fmt"

type Error struct {
	code    string
	message string
}

func (e *Error) Error() string {
	return e.code + ": " + e.message
}

func (e *Error) Code() string {
	return e.code
}

const (
	ErrCodeNetworkUnavailable    = "network.unavailable"
	ErrCodeNetworkDenied         = "network.denied"
	ErrCodeEndpointDenied        = "network.endpoint_denied"
	ErrCodeDNSTimeout            = "network.dns_timeout"
	ErrCodeICMPUnavailable       = "network.icmp_unavailable"
	ErrCodeTCPDenied             = "network.tcp_denied"
	ErrCodeHTTPDenied            = "network.http_denied"
	ErrCodeDownloadDenied        = "network.download_denied"
	ErrCodeResponseTooLarge      = "network.response_too_large"
	ErrCodeTooManyRedirects      = "network.too_many_redirects"
	ErrCodeInvalidURL            = "network.invalid_url"
	ErrCodeCRLFInjection         = "network.crlf_injection"
	ErrCodeDownloadPartial       = "network.download_partial"
	ErrCodeDownloadFileSizeLimit = "network.download_file_size_limit"
)

func ErrUnavailable(reason string) *Error {
	return &Error{code: ErrCodeNetworkUnavailable, message: "network unavailable: " + reason}
}

func ErrDenied(target string) *Error {
	return &Error{code: ErrCodeNetworkDenied, message: "network access denied: " + target}
}

func ErrEndpointDenied(reason string) *Error {
	return &Error{code: ErrCodeEndpointDenied, message: "endpoint denied: " + reason}
}

func ErrDNSTimeout(host string) *Error {
	return &Error{code: ErrCodeDNSTimeout, message: "DNS lookup timed out: " + host}
}

func ErrICMPUnavailable() *Error {
	return &Error{code: ErrCodeICMPUnavailable, message: "ICMP not available in this runtime"}
}

func ErrTCPDenied(target string) *Error {
	return &Error{code: ErrCodeTCPDenied, message: "TCP probe denied: " + target}
}

func ErrHTTPDenied(reason string) *Error {
	return &Error{code: ErrCodeHTTPDenied, message: "HTTP request denied: " + reason}
}

func ErrDownloadDenied(reason string) *Error {
	return &Error{code: ErrCodeDownloadDenied, message: "download denied: " + reason}
}

func ErrResponseTooLarge(maxBytes int64) *Error {
	return &Error{code: ErrCodeResponseTooLarge, message: fmt.Sprintf("response exceeds %d bytes", maxBytes)}
}

func ErrTooManyRedirects(maxRedirects int) *Error {
	return &Error{code: ErrCodeTooManyRedirects, message: fmt.Sprintf("exceeded %d redirects", maxRedirects)}
}

func ErrInvalidURL(url string, reason string) *Error {
	return &Error{code: ErrCodeInvalidURL, message: fmt.Sprintf("invalid URL %s: %s", url, reason)}
}

func ErrCRLFInjection(detail string) *Error {
	return &Error{code: ErrCodeCRLFInjection, message: "CRLF injection detected: " + detail}
}

func ErrDownloadPartial(reason string) *Error {
	return &Error{code: ErrCodeDownloadPartial, message: "download partial failure: " + reason}
}

func ErrDownloadFileSizeLimit(maxBytes int64) *Error {
	return &Error{code: ErrCodeDownloadFileSizeLimit, message: fmt.Sprintf("download exceeds %d bytes", maxBytes)}
}
