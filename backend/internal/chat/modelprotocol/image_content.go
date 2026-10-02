package modelprotocol

import "strings"

func imagePayload(part ModelContentPart) (string, string) {
	mimeType, data := part.MIMEType, part.ResourceURI
	if strings.HasPrefix(data, "data:") {
		if header, payload, ok := strings.Cut(data, ","); ok {
			mimeType = strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
			data = payload
		}
	}
	if mimeType == "" {
		mimeType = "image/png"
	}
	return mimeType, data
}

func anthropicImageSource(part ModelContentPart) map[string]interface{} {
	if strings.HasPrefix(part.ResourceURI, "https://") || strings.HasPrefix(part.ResourceURI, "http://") {
		return map[string]interface{}{"type": "url", "url": part.ResourceURI}
	}
	mimeType, data := imagePayload(part)
	return map[string]interface{}{"type": "base64", "media_type": mimeType, "data": data}
}

func geminiImagePart(part ModelContentPart) map[string]interface{} {
	if strings.HasPrefix(part.ResourceURI, "https://") || strings.HasPrefix(part.ResourceURI, "http://") {
		return map[string]interface{}{"fileData": map[string]interface{}{"mimeType": part.MIMEType, "fileUri": part.ResourceURI}}
	}
	mimeType, data := imagePayload(part)
	return map[string]interface{}{"inlineData": map[string]interface{}{"mimeType": mimeType, "data": data}}
}
