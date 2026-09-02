//go:build linux && !android

package network

import (
	"context"
	"fmt"
	"strconv"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Handle(ctx context.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case OpStatus:
		return h.handleStatus(ctx)
	case OpInterfaces:
		return h.handleInterfaces(ctx)
	case OpRoutes:
		return h.handleRoutes(ctx)
	case OpDNSLookup:
		return h.handleDNSLookup(ctx, payload)
	case OpPing:
		return h.handlePing(ctx, payload)
	case OpTCPProbe:
		return h.handleTCPProbe(ctx, payload)
	case OpHTTPRequest:
		return h.handleHTTPRequest(ctx, payload)
	case OpMultipartRequest:
		return h.handleMultipartRequest(ctx, payload)
	case OpDownload:
		return h.handleDownload(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown network operation: %s", operation)
	}
}

func (h *Handler) handleStatus(ctx context.Context) (map[string]any, error) {
	status := h.service.Status(ctx)
	return map[string]any{
		"available":         status.Available,
		"dnsAvailable":      status.DNSAvailable,
		"outboundAvailable": status.OutboundAvailable,
		"loopbackAvailable": status.LoopbackAvailable,
		"httpAvailable":     status.HTTPAvailable,
		"tcpAvailable":      status.TCPAvailable,
		"interfaceCount":    status.InterfaceCount,
	}, nil
}

func (h *Handler) handleInterfaces(ctx context.Context) (map[string]any, error) {
	interfaces := h.service.Interfaces(ctx)
	list := make([]map[string]any, 0, len(interfaces))
	for _, iface := range interfaces {
		list = append(list, interfaceToMap(iface))
	}
	return map[string]any{
		"interfaces": list,
		"count":      len(list),
	}, nil
}

func (h *Handler) handleRoutes(ctx context.Context) (map[string]any, error) {
	routes := h.service.Routes(ctx)
	list := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		list = append(list, routeToMap(route))
	}
	return map[string]any{
		"routes": list,
		"count":  len(list),
	}, nil
}

func (h *Handler) handleDNSLookup(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := DNSLookupRequest{
		Host: getStringKey(payload, "host"),
		Type: getStringKey(payload, "type"),
	}
	result, err := h.service.DNSLookup(ctx, req)
	if err != nil {
		return nil, err
	}
	return dnsResultToMap(result), nil
}

func (h *Handler) handlePing(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := PingRequest{
		Host:      getStringKey(payload, "host"),
		Count:     getIntKey(payload, "count", 0),
		TimeoutMS: getIntKey(payload, "timeoutMs", 0),
	}
	result, err := h.service.Ping(ctx, req)
	if err != nil {
		return nil, err
	}
	return pingResultToMap(result), nil
}

func (h *Handler) handleTCPProbe(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := TCPProbeRequest{
		Host:      getStringKey(payload, "host"),
		Port:      getIntKey(payload, "port", 0),
		TimeoutMS: getIntKey(payload, "timeoutMs", 0),
	}
	result, err := h.service.TCPProbe(ctx, req)
	if err != nil {
		return nil, err
	}
	return tcpResultToMap(result), nil
}

func (h *Handler) handleHTTPRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := HTTPRequest{
		Method:     getStringKey(payload, "method"),
		URL:        getStringKey(payload, "url"),
		Headers:    getStringMapKey(payload, "headers"),
		Body:       getStringKey(payload, "body"),
		BodyBase64: getStringKey(payload, "bodyBase64"),
		TimeoutMS:  getIntKey(payload, "timeoutMs", 0),
	}
	if mr, ok := payload["maxResponseBytes"].(float64); ok {
		req.MaxResponseBytes = int64(mr)
	}
	if fr, ok := payload["followRedirects"].(bool); ok {
		req.FollowRedirects = &fr
	}
	result, err := h.service.HTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return httpResponseToMap(result), nil
}

func (h *Handler) handleMultipartRequest(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := MultipartRequest{
		Method:    getStringKey(payload, "method", "POST"),
		URL:       getStringKey(payload, "url"),
		Headers:   getStringMapKey(payload, "headers"),
		TimeoutMS: getIntKey(payload, "timeoutMs", 0),
	}
	if mr, ok := payload["maxResponseBytes"].(float64); ok {
		req.MaxResponseBytes = int64(mr)
	}
	if fr, ok := payload["followRedirects"].(bool); ok {
		req.FollowRedirects = &fr
	}
	if rawParts, ok := payload["parts"].([]any); ok {
		for _, rawPart := range rawParts {
			partMap, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			req.Parts = append(req.Parts, MultipartPart{
				Name: getStringKey(partMap, "name"), Value: getStringKey(partMap, "value"),
				Filename: getStringKey(partMap, "filename"), ContentType: getStringKey(partMap, "contentType"),
				DataBase64: getStringKey(partMap, "dataBase64"),
			})
		}
	}
	result, err := h.service.MultipartRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	return httpResponseToMap(result), nil
}

func (h *Handler) handleDownload(ctx context.Context, payload map[string]any) (map[string]any, error) {
	req := DownloadRequest{
		URL:       getStringKey(payload, "url"),
		Target:    getStringKey(payload, "target"),
		Overwrite: getBoolKey(payload, "overwrite", false),
		TimeoutMS: getIntKey(payload, "timeoutMs", 0),
	}
	if mb, ok := payload["maxBytes"].(float64); ok {
		req.MaxBytes = int64(mb)
	}
	result, err := h.service.Download(ctx, req)
	if err != nil {
		return nil, err
	}
	return downloadResultToMap(result), nil
}

func interfaceToMap(iface Interface) map[string]any {
	return map[string]any{
		"name":         iface.Name,
		"index":        iface.Index,
		"mtu":          iface.MTU,
		"hardwareAddr": iface.HardwareAddr,
		"flags":        iface.Flags,
		"addresses":    iface.Addresses,
	}
}

func routeToMap(route Route) map[string]any {
	return map[string]any{
		"interface":   route.Interface,
		"destination": route.Destination,
		"gateway":     route.Gateway,
		"prefix":      route.Prefix,
		"metric":      route.Metric,
		"family":      route.Family,
	}
}

func dnsResultToMap(result DNSLookupResult) map[string]any {
	m := map[string]any{
		"host": result.Host,
		"type": result.Type,
	}
	if len(result.Addresses) > 0 {
		m["addresses"] = result.Addresses
	}
	if result.CNAME != "" {
		m["cname"] = result.CNAME
	}
	if len(result.TXT) > 0 {
		m["txt"] = result.TXT
	}
	if len(result.MX) > 0 {
		mxList := make([]map[string]any, 0, len(result.MX))
		for _, mx := range result.MX {
			mxList = append(mxList, map[string]any{"host": mx.Host, "pref": mx.Pref})
		}
		m["mx"] = mxList
	}
	return m
}

func pingResultToMap(result PingResult) map[string]any {
	return map[string]any{
		"host":        result.Host,
		"resolvedIp":  result.ResolvedIP,
		"sent":        result.Sent,
		"received":    result.Received,
		"lossPercent": result.LossPercent,
		"minMs":       result.MinMs,
		"avgMs":       result.AvgMs,
		"maxMs":       result.MaxMs,
		"mode":        result.Mode,
	}
}

func tcpResultToMap(result TCPProbeResult) map[string]any {
	return map[string]any{
		"host":          result.Host,
		"resolvedIp":    result.ResolvedIP,
		"port":          result.Port,
		"reachable":     result.Reachable,
		"connectTimeMs": result.ConnectTimeMS,
	}
}

func httpResponseToMap(result HTTPResponse) map[string]any {
	m := map[string]any{
		"statusCode": result.StatusCode,
		"status":     result.Status,
		"headers":    result.Headers,
		"bytesRead":  result.BytesRead,
		"truncated":  result.Truncated,
		"finalUrl":   result.FinalURL,
		"durationMs": result.DurationMS,
	}
	if result.Body != "" {
		m["body"] = result.Body
	}
	if result.BodyBase64 != "" {
		m["bodyBase64"] = result.BodyBase64
	}
	if result.MIMEType != "" {
		m["mimeType"] = result.MIMEType
	}
	if result.Charset != "" {
		m["charset"] = result.Charset
	}
	return m
}

func downloadResultToMap(result DownloadResult) map[string]any {
	return map[string]any{
		"path":         result.Path,
		"bytesWritten": result.BytesWritten,
		"mimeType":     result.MIMEType,
		"sha256":       result.SHA256,
		"statusCode":   result.StatusCode,
		"finalUrl":     result.FinalURL,
	}
}

func getStringKey(m map[string]any, key string, defaultVal ...string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return ""
}

func getIntKey(m map[string]any, key string, defaultVal ...int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return 0
}

func getBoolKey(m map[string]any, key string, defaultVal bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return defaultVal
}

func getStringMapKey(m map[string]any, key string) map[string]string {
	result := map[string]string{}
	if v, ok := m[key].(map[string]any); ok {
		for k, val := range v {
			if s, ok := val.(string); ok {
				result[k] = s
			}
		}
	}
	return result
}
