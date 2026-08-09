package checkpoint

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"sort"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

func ComputeDescriptorRevision(descriptor domain.PluginDescriptor) string {
	hasher := sha256.New()

	writeString(hasher, string(descriptor.ID))
	writeString(hasher, descriptor.ExtensionID)
	writeString(hasher, descriptor.Version)
	writeString(hasher, descriptor.ProtocolVersion)

	caps := make([]string, len(descriptor.Capabilities))
	copy(caps, descriptor.Capabilities)
	sort.Strings(caps)
	for _, cap := range caps {
		writeString(hasher, string(cap))
	}

	svcIDs := make([]domain.ServiceID, len(descriptor.Services))
	for i, svc := range descriptor.Services {
		svcIDs[i] = svc.ID
	}
	sort.SliceStable(svcIDs, func(i, j int) bool {
		return svcIDs[i] < svcIDs[j]
	})

	for _, svc := range descriptor.Services {
		writeString(hasher, string(svc.ID))
		writeString(hasher, string(svc.Kind))
		writeBool(hasher, svc.Required)
		deps := make([]domain.ServiceID, len(svc.DependsOn))
		copy(deps, svc.DependsOn)
		sort.SliceStable(deps, func(i, j int) bool {
			return deps[i] < deps[j]
		})
		for _, dep := range deps {
			writeString(hasher, string(dep))
		}
	}

	chIDs := make([]domain.ChannelID, len(descriptor.Channels))
	for i, ch := range descriptor.Channels {
		chIDs[i] = ch.ID
	}
	sort.SliceStable(chIDs, func(i, j int) bool {
		return chIDs[i] < chIDs[j]
	})

	for _, ch := range descriptor.Channels {
		writeString(hasher, string(ch.ID))
	}

	_ = svcIDs
	_ = chIDs

	return "rev-" + hexHash(hasher.Sum(nil))
}

func writeString(h *sha256Digest, s string) {
	_, _ = h.Write([]byte(s))
	_, _ = h.Write([]byte{0})
}

func writeBool(h *sha256Digest, b bool) {
	if b {
		_, _ = h.Write([]byte{1})
	} else {
		_, _ = h.Write([]byte{0})
	}
}

func hexHash(sum []byte) string {
	const hexDigits = "0123456789abcdef"
	buf := make([]byte, len(sum)*2)
	for i, b := range sum {
		buf[i*2] = hexDigits[b>>4]
		buf[i*2+1] = hexDigits[b&0x0f]
	}
	return string(buf)
}

type sha256Digest = sha256DigestImpl

type sha256DigestImpl struct {
	buf []byte
}

func (d *sha256DigestImpl) Write(p []byte) (int, error) {
	d.buf = append(d.buf, p...)
	return len(p), nil
}

func (d *sha256DigestImpl) Sum(_ []byte) []byte {
	return d.buf
}

func CanonicalJSON(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", ""); err != nil {
		return data, nil
	}
	return buf.Bytes(), nil
}
