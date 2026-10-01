package imageintelligence

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/imageprovider"
)

type testArtifactCreator struct {
	created artifact.CreateRequest
}

func (c *testArtifactCreator) Create(_ context.Context, req artifact.CreateRequest) (artifact.Artifact, error) {
	c.created = req
	return artifact.Artifact{
		ID:        "art_generated",
		Kind:      req.Kind,
		MIMEType:  req.MIMEType,
		Filename:  req.Filename,
		SizeBytes: int64(len(req.Filename)),
	}, nil
}

func TestResourceifyImagePersistsArtifact(t *testing.T) {
	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	creator := &testArtifactCreator{}
	provider := NewGenerateProvider(nil, nil, creator)
	image, genErr := provider.resourceifyImage(context.Background(), "space_test", imageprovider.GeneratedImage{
		Bytes:    pngData,
		MimeType: "image/png",
		Width:    1,
		Height:   1,
	})
	if genErr != nil {
		t.Fatal(genErr)
	}
	if image.ResourceURI != "amitia://artifacts/art_generated" {
		t.Fatalf("unexpected generated resource URI: %s", image.ResourceURI)
	}
	if creator.created.OwnerSpaceID != "space_test" || creator.created.Source != artifact.SourceGenerated {
		t.Fatalf("unexpected artifact create request: %#v", creator.created)
	}
}
