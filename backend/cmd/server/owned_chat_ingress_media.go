package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/interaction"
)

func ownedIngressAttachments(ctx context.Context, req *interaction.UnifiedEntryRequest) ([]business.Attachment, error) {
	items := []business.Attachment{}
	for _, input := range []struct{ kind, source string }{{"image", req.ImageUrl}, {"audio", req.AudioUrl}, {"video", req.VideoUrl}} {
		if strings.TrimSpace(input.source) == "" {
			continue
		}
		data, contentType, name, err := ownedIngressMedia(ctx, input.source)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(data)
		items = append(items, business.Attachment{Kind: input.kind, Name: name, MIME: contentType, Data: base64.StdEncoding.EncodeToString(data), Hash: hex.EncodeToString(digest[:])})
	}
	if err := business.ValidateAttachments(items); err != nil {
		return nil, err
	}
	return items, nil
}

func trustedExtensionMediaSource(source string, resolver extensionResourceResolver) (string, error) {
	const prefix = "/api/extension/resources/"
	if !strings.HasPrefix(source, prefix) {
		return source, nil
	}
	if resolver == nil {
		return "", errors.New("扩展资源解析器不可用")
	}
	_, _, fullPath, err := resolver.ResolveResourceLink(strings.TrimPrefix(source, prefix))
	if err != nil {
		return "", errors.New("扩展资源链接无效")
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", errors.New("扩展资源不可读取")
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return "", errors.New("媒体内容为空或超过附件大小上限")
	}
	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func ownedIngressMedia(ctx context.Context, source string) ([]byte, string, string, error) {
	var reader io.ReadCloser
	contentType, name := "", "附件"
	if strings.HasPrefix(source, "data:") {
		header, encoded, ok := strings.Cut(source, ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			return nil, "", "", errors.New("媒体编码无效")
		}
		contentType = strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		if len(encoded) > base64.StdEncoding.EncodedLen(1<<20) {
			return nil, "", "", errors.New("媒体超过附件大小上限")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(encoded)
		return data, contentType, name, err
	}
	u, err := url.Parse(source)
	if err != nil || u.User != nil || u.Fragment != "" {
		return nil, "", "", errors.New("媒体地址无效")
	}
	if u.Scheme == "" && u.Host == "" {
		base := ""
		for prefix, directory := range map[string]string{"/voice/": "data/voice_msg", "/images/": "data/images", "/videos/": "data/videos"} {
			if strings.HasPrefix(u.Path, prefix) {
				base, name = directory, strings.TrimPrefix(u.Path, prefix)
				break
			}
		}
		if base == "" || name != filepath.Base(name) || strings.ContainsAny(name, "\\/\x00") || name == "." || name == ".." {
			return nil, "", "", errors.New("媒体路径不属于受信上传目录")
		}
		reader, err = os.Open(filepath.Join(base, name))
		contentType = mime.TypeByExtension(filepath.Ext(name))
	} else {
		if u.Scheme != "https" && u.Scheme != "http" || u.Hostname() == "" {
			return nil, "", "", errors.New("媒体地址必须为 HTTP 或 HTTPS")
		}
		dialer := &net.Dialer{Timeout: 15 * time.Second}
		transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, address := range addresses {
				ip := address.IP
				if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
					return nil, errors.New("媒体远程地址不允许访问内网")
				}
			}
			for _, address := range addresses {
				connection, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				err = dialErr
			}
			if err == nil {
				err = errors.New("媒体地址没有可用网络地址")
			}
			return nil, err
		}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 4 || req.URL.User != nil || req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return errors.New("媒体重定向无效")
			}
			return nil
		}}
		request, requestErr := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
		if requestErr != nil {
			return nil, "", "", requestErr
		}
		response, responseErr := client.Do(request)
		if responseErr != nil {
			return nil, "", "", responseErr
		}
		reader = response.Body
		if response.StatusCode != http.StatusOK {
			reader.Close()
			return nil, "", "", errors.New("下载媒体失败")
		}
		contentType, name = response.Header.Get("Content-Type"), path.Base(u.Path)
	}
	if err != nil {
		return nil, "", "", err
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
	if err != nil {
		return nil, "", "", err
	}
	if len(data) == 0 || len(data) > 1<<20 {
		return nil, "", "", errors.New("媒体内容为空或超过附件大小上限")
	}
	contentType, _, _ = mime.ParseMediaType(contentType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = http.DetectContentType(data)
	}
	if name == "" || name == "." || name == "/" {
		name = "附件"
	}
	return data, contentType, name, nil
}

func ownedIngressResponseOutputs(response *interaction.ProcessResponse, raw json.RawMessage) error {
	var outputs []chat.MessageOutput
	if len(raw) > 0 && string(raw) != "null" {
		if err := json.Unmarshal(raw, &outputs); err != nil {
			return err
		}
	}
	plan := &interaction.MessagePlan{DeliveryGroupID: response.RequestID, Managed: true, Items: []interaction.MessagePlanItem{}}
	appendOutput := func(output chat.MessageOutput) {
		part := output.Part
		id := output.OutputID
		if id == "" {
			id = fmt.Sprintf("%s/output/%d", response.RequestID, len(plan.Items)+1)
		}
		plan.Items = append(plan.Items, interaction.MessagePlanItem{MessageID: id, Sequence: len(plan.Items) + 1, Type: part.Type, ExtensionType: part.ExtensionType, Content: part.Content, AltText: part.AltText, MIMEType: part.MIMEType, IsAnimated: part.IsAnimated, Width: part.Width, Height: part.Height, OriginalAssetReference: part.URL, FallbackAssetReference: part.FallbackURL})
		if part.Type == "audio" && part.URL != "" {
			response.AudioUrls = append(response.AudioUrls, part.URL)
		}
	}
	for _, output := range outputs {
		if output.Placement == "before_text" {
			appendOutput(output)
		}
	}
	if strings.TrimSpace(response.Reply) != "" {
		plan.Items = append(plan.Items, interaction.MessagePlanItem{MessageID: response.RequestID + "/assistant", Sequence: len(plan.Items) + 1, Type: "text", Content: response.Reply})
	}
	for _, output := range outputs {
		if output.Placement != "before_text" {
			appendOutput(output)
		}
	}
	response.MessagePlan = plan
	return nil
}
