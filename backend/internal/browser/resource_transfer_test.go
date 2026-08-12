package browser

import (
	"context"
	"testing"
)

func TestProductionResourceTransferDownloadValidationError(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	ctx := context.Background()

	_, err = provider.Resources().Download(ctx, BrowserDownloadRequest{})
	if err == nil {
		t.Fatal("Download should fail with empty request")
	}
	if IsUnsupportedOperation(err) {
		t.Fatal("Download should not be unsupported")
	}
}

func TestProductionResourceTransferUploadValidationError(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	ctx := context.Background()

	_, err = provider.Resources().Upload(ctx, BrowserUploadRequest{})
	if err == nil {
		t.Fatal("Upload should fail with empty request")
	}
	if IsUnsupportedOperation(err) {
		t.Fatal("Upload should not be unsupported")
	}
}

func TestProductionResourceTransferScreenshotValidationError(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	ctx := context.Background()

	_, err = provider.Resources().Screenshot(ctx, BrowserScreenshotRequest{})
	if err == nil {
		t.Fatal("Screenshot should fail with empty request")
	}
	if IsUnsupportedOperation(err) {
		t.Fatal("Screenshot should not be unsupported")
	}
}

func TestProductionResourceTransferScreenshotInvalidFormat(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	ctx := context.Background()

	_, err = provider.Resources().Screenshot(ctx, BrowserScreenshotRequest{
		SessionID: "s1",
		TabID:     "t1",
		Format:    "bmp",
	})
	if err == nil {
		t.Fatal("Screenshot should fail with invalid format")
	}
	be, ok := err.(*BrowserError)
	if !ok || be.Code != ErrCodeScreenshotInvalidFormat {
		t.Fatalf("Expected screenshot_invalid_format error, got: %v", err)
	}
}

func TestProductionResourceTransferScreenshotInvalidQuality(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	ctx := context.Background()

	_, err = provider.Resources().Screenshot(ctx, BrowserScreenshotRequest{
		SessionID: "s1",
		TabID:     "t1",
		Quality:   101,
	})
	if err == nil {
		t.Fatal("Screenshot should fail with invalid quality")
	}
	be, ok := err.(*BrowserError)
	if !ok || be.Code != ErrCodeScreenshotInvalidQuality {
		t.Fatalf("Expected screenshot_invalid_quality error, got: %v", err)
	}
}

func TestProductionResourceTransferContextCancelled(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = provider.Resources().Download(ctx, BrowserDownloadRequest{
		SessionID: "s1",
		TabID:     "t1",
	})
	if err == nil {
		t.Fatal("Download should fail with cancelled context")
	}
}

func TestCapabilitiesIncludeResourceSupport(t *testing.T) {
	engine := &fakeEngine{state: BrowserRuntimeReady, stopped: false}
	factory := &fakeEngineFactory{engine: engine}
	config := BrowserConfig{
		Enabled:     true,
		MaxSessions: 8,
		Headless:    true,
	}

	provider, err := NewProductionProvider(config, factory)
	if err != nil {
		t.Fatalf("NewProductionProvider failed: %v", err)
	}

	caps := provider.BrowserCapabilities()
	if !caps.SupportsDownload {
		t.Fatal("SupportsDownload should be true")
	}
	if !caps.SupportsUpload {
		t.Fatal("SupportsUpload should be true")
	}
	if !caps.SupportsScreenshot {
		t.Fatal("SupportsScreenshot should be true")
	}
	found := false
	for _, level := range caps.RiskLevels {
		if level == "browser_resource" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("RiskLevels should include browser_resource")
	}
}
