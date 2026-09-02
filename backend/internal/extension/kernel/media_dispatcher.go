package kernel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/media"
	"github.com/u-ai/backend/internal/media/conversion"
	"github.com/u-ai/backend/internal/media/metadata"
)

type MediaToolDispatcher struct {
	service *media.Service
}

func NewMediaToolDispatcher(svc *media.Service) *MediaToolDispatcher {
	return &MediaToolDispatcher{service: svc}
}

func (d *MediaToolDispatcher) Dispatch(ctx context.Context, handlerName string, input json.RawMessage) (json.RawMessage, error) {
	switch handlerName {
	case "media.metadata":
		return d.handleMetadata(ctx, input)
	case "media.convert":
		return d.handleConvert(ctx, input)
	case "media.ffmpeg.execute":
		return d.handleFFmpegExecute(ctx, input)
	default:
		return nil, fmt.Errorf("unknown media tool: %s", handlerName)
	}
}

func (d *MediaToolDispatcher) handleMetadata(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Resource string `json:"resource"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	meta, err := d.service.GetMetadata(ctx, req.Resource, metadata.MetadataRequest{
		SourceURI:        req.Resource,
		IncludeStreams:   true,
		IncludeChapters:  true,
		IncludeTags:      true,
		IncludeTechnical: true,
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"metadata": meta})
}

func (d *MediaToolDispatcher) handleConvert(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req struct {
		Resource        string `json:"resource"`
		Target          string `json:"target"`
		TargetContainer string `json:"targetContainer"`
		VideoCodec      string `json:"videoCodec"`
		AudioCodec      string `json:"audioCodec"`
	}
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	convertReq := conversion.ConvertRequest{
		SourceURI: req.Resource,
		TargetURI: req.Target,
		Output: conversion.MediaOutputSpec{
			Container: req.TargetContainer,
		},
		Video: &conversion.VideoConversionSpec{
			Codec: req.VideoCodec,
		},
		Audio: &conversion.AudioConversionSpec{
			Codec: req.AudioCodec,
		},
	}
	result, err := d.service.Convert(ctx, convertReq, conversion.ConvertOptions{})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"resource": result.ResourceURI})
}

func (d *MediaToolDispatcher) handleFFmpegExecute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var req media.FFmpegExecuteRequest
	if err := json.Unmarshal(input, &req); err != nil {
		return nil, err
	}
	result, err := d.service.ExecuteFFmpeg(ctx, req)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
