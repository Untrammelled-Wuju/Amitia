package artifact

import "testing"

func TestChatUploadMIMETypeKeepsDeclaredTypeOrFallsBackToExtension(t *testing.T) {
	for _, testCase := range []struct{ declared, filename, expected string }{
		{"application/pdf", "attachment.bin", "application/pdf"},
		{"APPLICATION/PDF; charset=binary", "attachment.txt", "application/pdf"},
		{"application/zip", "deck.pptx", "application/zip"},
		{"application/octet-stream", "notes.txt", "text/plain"},
		{"binary/octet-stream", "notes.txt", "text/plain"},
		{"", "notes.txt", "text/plain"},
		{"application/octet-stream", "notes.unknown-extension", ""},
	} {
		if actual := chatUploadMIMEType(testCase.declared, testCase.filename); actual != testCase.expected {
			t.Fatalf("declared=%q filename=%q expected=%q actual=%q", testCase.declared, testCase.filename, testCase.expected, actual)
		}
	}
}
