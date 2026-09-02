//go:build linux && !android

package network

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strings"
)

const maxMultipartParts = 64

type MultipartPart struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	DataBase64  string `json:"dataBase64,omitempty"`
}

type MultipartRequest struct {
	Method           string            `json:"method"`
	URL              string            `json:"url"`
	Headers          map[string]string `json:"headers,omitempty"`
	Parts            []MultipartPart   `json:"parts"`
	TimeoutMS        int               `json:"timeoutMs,omitempty"`
	FollowRedirects  *bool             `json:"followRedirects,omitempty"`
	MaxResponseBytes int64             `json:"maxResponseBytes,omitempty"`
}

func performMultipartRequest(ctx context.Context, req MultipartRequest, policy Policy) (HTTPResponse, error) {
	if len(req.Parts) == 0 {
		return HTTPResponse{}, ErrHTTPDenied("multipart parts are required")
	}
	if len(req.Parts) > maxMultipartParts {
		return HTTPResponse{}, ErrHTTPDenied(fmt.Sprintf("multipart parts exceed %d", maxMultipartParts))
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for _, part := range req.Parts {
		name := strings.TrimSpace(part.Name)
		if name == "" || len(name) > 256 {
			return HTTPResponse{}, ErrHTTPDenied("multipart part name is invalid")
		}
		var data []byte
		if part.DataBase64 != "" {
			decoded, err := base64.StdEncoding.DecodeString(part.DataBase64)
			if err != nil {
				return HTTPResponse{}, ErrHTTPDenied("invalid multipart base64 data: " + err.Error())
			}
			data = decoded
		} else {
			data = []byte(part.Value)
		}
		if int64(body.Len()+len(data)) > policy.MaxHTTPBodyBytes {
			return HTTPResponse{}, ErrHTTPDenied("multipart body exceeds configured limit")
		}

		if part.Filename == "" {
			field, err := writer.CreateFormField(name)
			if err != nil {
				return HTTPResponse{}, ErrHTTPDenied(err.Error())
			}
			if _, err := field.Write(data); err != nil {
				return HTTPResponse{}, ErrHTTPDenied(err.Error())
			}
			continue
		}

		filename := strings.TrimSpace(part.Filename)
		if filename == "" || len(filename) > 512 || strings.ContainsAny(filename, "\r\n") {
			return HTTPResponse{}, ErrHTTPDenied("multipart filename is invalid")
		}
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, name, filename))
		contentType := strings.TrimSpace(part.ContentType)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		if strings.ContainsAny(contentType, "\r\n") || len(contentType) > 256 {
			return HTTPResponse{}, ErrHTTPDenied("multipart content type is invalid")
		}
		header.Set("Content-Type", contentType)
		field, err := writer.CreatePart(header)
		if err != nil {
			return HTTPResponse{}, ErrHTTPDenied(err.Error())
		}
		if _, err := field.Write(data); err != nil {
			return HTTPResponse{}, ErrHTTPDenied(err.Error())
		}
	}
	if err := writer.Close(); err != nil {
		return HTTPResponse{}, ErrHTTPDenied(err.Error())
	}
	if int64(body.Len()) > policy.MaxHTTPBodyBytes {
		return HTTPResponse{}, ErrHTTPDenied("multipart body exceeds configured limit")
	}

	headers := make(map[string]string, len(req.Headers)+1)
	for key, value := range req.Headers {
		headers[key] = value
	}
	headers["Content-Type"] = writer.FormDataContentType()
	method := strings.TrimSpace(req.Method)
	if method == "" {
		method = "POST"
	}
	return performHTTPRequest(ctx, HTTPRequest{
		Method:           method,
		URL:              req.URL,
		Headers:          headers,
		BodyBase64:       base64.StdEncoding.EncodeToString(body.Bytes()),
		TimeoutMS:        req.TimeoutMS,
		FollowRedirects:  req.FollowRedirects,
		MaxResponseBytes: req.MaxResponseBytes,
	}, policy)
}
