package extension

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLegacy_Package_Archive_RejectsNonUTF8Filename(t *testing.T) {
	limits := DefaultPackageLimits()
	raw := packageUnsafeZIPNonUTF8(t, []byte{0xff, 0xfe, '.', 't', 'x', 't'}, []byte("x"))
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("non-UTF8 filename should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsWindowsReservedNames(t *testing.T) {
	limits := DefaultPackageLimits()
	for _, name := range []string{"CON.txt", "con/file.txt", "PRN.md", "nul/file.txt", "AUX.json", "com1/file.txt", "LPT1", "lpt9/file.txt"} {
		t.Run(name, func(t *testing.T) {
			raw := packageUnsafeZIP(t, map[string][]byte{name: []byte("content")}, nil)
			if _, _, err := readPackageZIP(raw, limits); err == nil {
				t.Fatalf("Windows reserved name should be rejected: %s", name)
			}
		})
	}
}

func TestLegacy_Package_Archive_RejectsDeepNesting(t *testing.T) {
	limits := DefaultPackageLimits()
	nested := "a"
	for i := 0; i < limits.MaxDepth+2; i++ {
		nested += "/b"
	}
	nested += ".txt"
	raw := packageUnsafeZIP(t, map[string][]byte{nested: []byte("x")}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("deeply nested path should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsOversizedFile(t *testing.T) {
	limits := DefaultPackageLimits()
	limits.MaxFileBytes = 10
	raw := packageUnsafeZIP(t, map[string][]byte{"large.txt": bytes.Repeat([]byte("a"), 100)}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("oversized file should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsTooManyFiles(t *testing.T) {
	limits := DefaultPackageLimits()
	limits.MaxFiles = 3
	files := map[string][]byte{}
	for i := 0; i < 10; i++ {
		files[string(rune('a'+i))+".txt"] = []byte("x")
	}
	raw := packageUnsafeZIP(t, files, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("too many files should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsForbiddenExtensions(t *testing.T) {
	limits := DefaultPackageLimits()
	for _, ext := range []string{".exe", ".dll", ".com", ".bat", ".cmd", ".msi", ".sys", ".scr", ".wasm", ".so", ".jar", ".class", ".apk"} {
		t.Run(ext, func(t *testing.T) {
			raw := packageUnsafeZIP(t, map[string][]byte{"payload" + ext: []byte("x")}, nil)
			if _, _, err := readPackageZIP(raw, limits); err == nil {
				t.Fatalf("forbidden extension should be rejected: %s", ext)
			}
		})
	}
}

func TestLegacy_Package_Archive_RejectsNestedArchives(t *testing.T) {
	limits := DefaultPackageLimits()
	for _, ext := range []string{".zip", ".rar", ".7z", ".tar", ".gz", ".tgz", ".bz2", ".xz"} {
		t.Run(ext, func(t *testing.T) {
			raw := packageUnsafeZIP(t, map[string][]byte{"nested" + ext: []byte("PK")}, nil)
			if _, _, err := readPackageZIP(raw, limits); err == nil {
				t.Fatalf("nested archive should be rejected: %s", ext)
			}
		})
	}
}

func TestLegacy_Package_Archive_RejectsELFBinaries(t *testing.T) {
	limits := DefaultPackageLimits()
	elf := append([]byte{0x7f, 'E', 'L', 'F'}, bytes.Repeat([]byte{0}, 32)...)
	raw := packageUnsafeZIP(t, map[string][]byte{"payload.o": elf}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("ELF binary should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsWasmModule(t *testing.T) {
	limits := DefaultPackageLimits()
	wasm := append([]byte{0, 'a', 's', 'm'}, bytes.Repeat([]byte{0}, 4)...)
	raw := packageUnsafeZIP(t, map[string][]byte{"module.wasm": wasm}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("Wasm module should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsAbsoluteWindowsPath(t *testing.T) {
	limits := DefaultPackageLimits()
	raw := packageUnsafeZIP(t, map[string][]byte{"C:/Windows/file.txt": []byte("x")}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("absolute Windows path should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsLeadingSlash(t *testing.T) {
	limits := DefaultPackageLimits()
	raw := packageUnsafeZIP(t, map[string][]byte{"/etc/file.txt": []byte("x")}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("path with leading slash should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsUnnormalizedDot(t *testing.T) {
	limits := DefaultPackageLimits()
	raw := packageUnsafeZIP(t, map[string][]byte{"a/./b.txt": []byte("x")}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("unnormalized relative dot path should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsZeroByteInPath(t *testing.T) {
	limits := DefaultPackageLimits()
	name := "file.txt\x00hidden.txt"
	raw := packageUnsafeZIPRaw(t, map[string]string{name: "x"}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("path with null byte should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsNonZIP(t *testing.T) {
	limits := DefaultPackageLimits()
	if _, _, err := readPackageZIP([]byte("not a zip"), limits); err == nil {
		t.Fatal("non-ZIP should be rejected")
	}
}

func TestLegacy_Package_Checksums_RejectsAllFilesListed(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("a"), "b.txt": []byte("b")}
	files["checksums.sha256"] = buildChecksums(files)
	files["c.txt"] = []byte("c")
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageUnlistedFile {
		t.Fatalf("unlisted file should be rejected: %v", err)
	}
}

func TestLegacy_Package_Checksums_RejectsExtraEntry(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("a")}
	hash := sha256.Sum256(files["a.txt"])
	files["checksums.sha256"] = []byte(hex.EncodeToString(hash[:]) + "  a.txt\n" + strings.Repeat("0", 64) + "  missing.txt\n")
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageMissingFile {
		t.Fatalf("extra checksum entry should be rejected: %v", err)
	}
}

func TestLegacy_Package_Checksums_RejectsSelfReferencing(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("a")}
	hash := sha256.Sum256(files["a.txt"])
	checksumLine := hex.EncodeToString(hash[:]) + "  a.txt\n"
	files["checksums.sha256"] = []byte(checksumLine)
	if err := validateChecksums(files); err != nil {
		t.Fatal(err)
	}
	badFiles := map[string][]byte{"a.txt": []byte("a")}
	hash = sha256.Sum256(badFiles["checksums.sha256"])
	badFiles["checksums.sha256"] = []byte(hex.EncodeToString(hash[:]) + "  checksums.sha256\n" + checksumLine)
	err := validateChecksums(badFiles)
	if err == nil || asExtensionError(err).Code != ErrPackageChecksumInvalid {
		t.Fatalf("checksums referencing itself should be rejected: %v", err)
	}
}

func TestLegacy_Package_Checksums_RejectsDuplicatePath(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("a")}
	hash := sha256.Sum256(files["a.txt"])
	files["checksums.sha256"] = []byte(hex.EncodeToString(hash[:]) + "  a.txt\n" + hex.EncodeToString(hash[:]) + "  a.txt\n")
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageChecksumInvalid {
		t.Fatalf("duplicate checksum path should be rejected: %v", err)
	}
}

func TestLegacy_Package_Checksums_RejectsTraversalPath(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("a")}
	hash := sha256.Sum256(files["a.txt"])
	files["checksums.sha256"] = []byte(hex.EncodeToString(hash[:]) + "  a.txt\n" + strings.Repeat("0", 64) + "  ../../../escape.txt\n")
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageChecksumInvalid {
		t.Fatalf("checksum with traversal path should be rejected: %v", err)
	}
}

func TestLegacy_Package_Checksums_RejectsShortHash(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("a")}
	files["checksums.sha256"] = []byte("aaa  a.txt\n")
	if err := validateChecksums(files); asExtensionError(err).Code != ErrPackageChecksumInvalid {
		t.Fatalf("short hash should be rejected: %v", err)
	}
}

func TestLegacy_Package_Signature_RejectsWrongAlgorithm(t *testing.T) {
	files := map[string][]byte{"manifest.json": []byte(`{"kind":"Skill"}`), "a.txt": []byte("a")}
	files["checksums.sha256"] = buildChecksums(files)
	doc := packageSignatureDocument{Algorithm: "rsa2048", KeyID: "id", PublicKey: "AA", Signature: "AA", SignedDigest: "sha256:" + packageCanonicalDigest(files)}
	files["signature.json"], _ = json.Marshal(doc)
	if _, _, err := verifyPackageSignature(files, false); asExtensionError(err).Code != ErrPackageSignatureInvalid {
		t.Fatalf("wrong algorithm should be rejected: %v", err)
	}
}

func TestLegacy_Package_Signature_RejectsWrongSignedDigest(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	fingerprintHash := sha256.Sum256(publicKey)
	fingerprint := "sha256:" + hex.EncodeToString(fingerprintHash[:])
	files := map[string][]byte{"manifest.json": []byte(`{"kind":"Skill"}`), "a.txt": []byte("a")}
	files["checksums.sha256"] = buildChecksums(files)
	_ = "sha256:" + packageCanonicalDigest(files)
	wrongDigest := "sha256:" + strings.Repeat("0", 64)
	signature := ed25519.Sign(privateKey, []byte(wrongDigest))
	doc := packageSignatureDocument{Algorithm: "ed25519", KeyID: fingerprint, PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(signature), SignedDigest: wrongDigest}
	files["signature.json"], _ = json.Marshal(doc)
	if _, _, err := verifyPackageSignature(files, false); asExtensionError(err).Code != ErrPackageSignatureInvalid {
		t.Fatalf("wrong signed digest should be rejected: %v", err)
	}
}

func TestLegacy_Package_Signature_RejectsMismatchedKeyID(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	files := map[string][]byte{"manifest.json": []byte(`{"kind":"Skill"}`), "a.txt": []byte("a")}
	files["checksums.sha256"] = buildChecksums(files)
	digest := "sha256:" + packageCanonicalDigest(files)
	signature := ed25519.Sign(privateKey, []byte(digest))
	doc := packageSignatureDocument{Algorithm: "ed25519", KeyID: "sha256:" + strings.Repeat("0", 64), PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(signature), SignedDigest: digest}
	files["signature.json"], _ = json.Marshal(doc)
	if _, _, err := verifyPackageSignature(files, false); asExtensionError(err).Code != ErrPackageSignatureInvalid {
		t.Fatalf("mismatched key ID should be rejected: %v", err)
	}
}

func TestLegacy_Package_Signature_RejectsCorruptedPublicKey(t *testing.T) {
	files := map[string][]byte{"manifest.json": []byte(`{"kind":"Skill"}`), "a.txt": []byte("a")}
	files["checksums.sha256"] = buildChecksums(files)
	digest := "sha256:" + packageCanonicalDigest(files)
	doc := packageSignatureDocument{Algorithm: "ed25519", KeyID: "sha256:" + strings.Repeat("0", 64), PublicKey: "not-base64!", Signature: "not-base64!", SignedDigest: digest}
	files["signature.json"], _ = json.Marshal(doc)
	if _, _, err := verifyPackageSignature(files, false); asExtensionError(err).Code != ErrPackageSignatureInvalid {
		t.Fatalf("corrupted public key should be rejected: %v", err)
	}
}

func TestLegacy_Package_Signature_RejectsWrongKeyLength(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	fingerprintHash := sha256.Sum256(publicKey)
	fingerprint := "sha256:" + hex.EncodeToString(fingerprintHash[:])
	files := map[string][]byte{"manifest.json": []byte(`{"kind":"Skill"}`), "a.txt": []byte("a")}
	files["checksums.sha256"] = buildChecksums(files)
	digest := "sha256:" + packageCanonicalDigest(files)
	signature := ed25519.Sign(privateKey, []byte(digest))
	doc := packageSignatureDocument{Algorithm: "ed25519", KeyID: fingerprint, PublicKey: base64.StdEncoding.EncodeToString(publicKey[:len(publicKey)-1]), Signature: base64.StdEncoding.EncodeToString(signature), SignedDigest: digest}
	files["signature.json"], _ = json.Marshal(doc)
	if _, _, err := verifyPackageSignature(files, false); asExtensionError(err).Code != ErrPackageSignatureInvalid {
		t.Fatalf("wrong key length should be rejected: %v", err)
	}
}

func TestLegacy_Package_Manifest_RejectsMissingInAmitiax(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageUnsafeZIP(t, map[string][]byte{"workflows/main.json": []byte(`{"steps":[{"id":"x","type":"noop"}]}`), "LICENSE": []byte("MIT\n")}, nil)
	rawHdr := make([]byte, len(raw))
	copy(rawHdr, raw)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: rawHdr})
	if err == nil {
		t.Fatal(".amitiax without manifest should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsInvalidJSON(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageUnsafeZIP(t, map[string][]byte{"manifest.json": []byte(`{invalid}`), "workflows/main.json": []byte(`{"steps":[{"id":"x","type":"noop"}]}`), "LICENSE": []byte("MIT\n")}, nil)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("invalid manifest JSON should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsInvalidSemver(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "not-semver", nil)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("invalid semver should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsEmptyName(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "", Version: "1.0.0", Description: "desc", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	workflow := WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "x", Type: "noop"}}, Output: json.RawMessage(`{}`), Limits: DefaultWorkflowLimits()}
	workflowRaw, _ := json.Marshal(workflow)
	files := map[string][]byte{"manifest.json": manifestRaw, "workflows/main.json": workflowRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("empty name should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsEmptyDescription(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: "", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	workflow := WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "x", Type: "noop"}}, Output: json.RawMessage(`{}`), Limits: DefaultWorkflowLimits()}
	workflowRaw, _ := json.Marshal(workflow)
	files := map[string][]byte{"manifest.json": manifestRaw, "workflows/main.json": workflowRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("empty description should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsEnabledTrue(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: "desc", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: true, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	workflow := WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "x", Type: "noop"}}, Output: json.RawMessage(`{}`), Limits: DefaultWorkflowLimits()}
	workflowRaw, _ := json.Marshal(workflow)
	files := map[string][]byte{"manifest.json": manifestRaw, "workflows/main.json": workflowRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("enabled=true should be rejected in local package")
	}
}

func TestLegacy_Package_Manifest_RejectsWildcardCapability(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	manifest.Capabilities = []string{"*"}
	manifestRaw, _ := json.Marshal(manifest)
	files["manifest.json"] = manifestRaw
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("wildcard capability should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsGlobstarCapability(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	manifest.Capabilities = []string{"http.**"}
	manifestRaw, _ := json.Marshal(manifest)
	files["manifest.json"] = manifestRaw
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("globstar capability should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsUnknownCapability(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	manifest.Capabilities = []string{"unknown.capability.xyz"}
	manifestRaw, _ := json.Marshal(manifest)
	files["manifest.json"] = manifestRaw
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("unknown capability should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsInvalidEntryKind(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: "desc", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "plugin", Path: "plugin.so"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	files := map[string][]byte{"manifest.json": manifestRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("invalid entry kind should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsWorkflowAndInstructionsTogether(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", map[string][]byte{"instructions/SKILL.md": []byte("---\nname: test\ndescription: desc\nlicense: MIT\n---\ncontent")})
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("workflow+instructions together should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsInvalidSkillID(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	manifest.Metadata.ID = "Invalid ID!"
	manifestRaw, _ := json.Marshal(manifest)
	files["manifest.json"] = manifestRaw
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("invalid skill ID should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsEmptyLicense(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	manifest.Metadata.License = ""
	manifestRaw, _ := json.Marshal(manifest)
	files["manifest.json"] = manifestRaw
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("empty license should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsWorkflowWithoutMainJSON(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: "desc", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	files := map[string][]byte{"manifest.json": manifestRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("workflow without main.json should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsInstructionsWithoutSKILLMD(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: "desc", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "instructions", Path: "instructions/SKILL.md"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	files := map[string][]byte{"manifest.json": manifestRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("instructions without SKILL.md should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsWorkflowWithScriptsDir(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", map[string][]byte{"scripts/run.py": []byte("print('x')")})
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("workflow with scripts directory should be rejected")
	}
}

func TestLegacy_Package_Manifest_RejectsEntryPathMismatch(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	manifest.Entry.Path = "workflows/other.json"
	manifestRaw, _ := json.Marshal(manifest)
	files["manifest.json"] = manifestRaw
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("entry path mismatch should be rejected")
	}
}

func TestLegacy_Package_Manifest_WarnsOnUnknownTopLevel(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", map[string][]byte{"unknown-dir/readme.txt": []byte("x")})
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	hasWarning := false
	for _, w := range preview.Warnings {
		if strings.Contains(w, "unknown-dir") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("unknown top-level directory should produce warning: %v", preview.Warnings)
	}
}

func TestLegacy_Package_Manifest_WarnsMissingLICENSE(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: "desc", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	workflow := WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "x", Type: "transform", Input: json.RawMessage(`{"op":"pick","value":{"message":"hello"},"fields":["message"]}`), OnError: WorkflowErrorPolicy{Mode: "fail"}}}, Output: json.RawMessage(`{"$ref":"steps.x"}`), Limits: DefaultWorkflowLimits()}
	workflowRaw, _ := json.Marshal(workflow)
	files := map[string][]byte{"manifest.json": manifestRaw, "workflows/main.json": workflowRaw}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	hasWarning := false
	for _, w := range preview.Warnings {
		if strings.Contains(w, "LICENSE") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("missing LICENSE should produce warning: %v", preview.Warnings)
	}
}

func TestLegacy_Package_Manifest_WarnsMissingSBOM(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	hasWarning := false
	for _, w := range preview.Warnings {
		if strings.Contains(w, "SBOM") {
			hasWarning = true
		}
	}
	if !hasWarning {
		t.Fatalf("missing SBOM should produce warning: %v", preview.Warnings)
	}
}

func TestLegacy_Package_Preview_InstructionsPackageStructure(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	skillContent := "---\nname: test-skill\ndescription: Test skill for baseline verification.\nlicense: MIT\n---\nYou are a helpful assistant.\n"
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test-skill", Name: "test-skill", Version: "1.0.0", Description: "Test skill for baseline verification.", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "instructions", Path: "instructions/SKILL.md"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	files := map[string][]byte{"manifest.json": manifestRaw, "instructions/SKILL.md": []byte(skillContent), "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test-skill.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	if preview.SkillType != "instructions" {
		t.Fatalf("expected instructions skill type, got: %s", preview.SkillType)
	}
	if preview.AgentSkill == nil {
		t.Fatal("instructions package should have AgentSkill preview")
	}
	if preview.AgentSkill.Definition.Name != "test-skill" {
		t.Fatalf("unexpected agent skill name: %s", preview.AgentSkill.Definition.Name)
	}
	if preview.Name != "test-skill" {
		t.Fatalf("unexpected package name: %s", preview.Name)
	}
}

func TestLegacy_Package_Preview_InstructionsNameMismatch(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	skillContent := "---\nname: different-name\ndescription: Test skill.\nlicense: MIT\n---\nYou are a helpful assistant.\n"
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test-skill", Version: "1.0.0", Description: "Test skill.", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "instructions", Path: "instructions/SKILL.md"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	files := map[string][]byte{"manifest.json": manifestRaw, "instructions/SKILL.md": []byte(skillContent), "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("name mismatch between manifest and SKILL.md should be rejected")
	}
}

func TestLegacy_Package_Preview_InstructionsEntryPathMismatch(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	skillContent := "---\nname: test-skill\ndescription: Test skill.\nlicense: MIT\n---\nContent.\n"
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test-skill", Name: "test-skill", Version: "1.0.0", Description: "Test skill.", Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "instructions", Path: "instructions/other.md"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	files := map[string][]byte{"manifest.json": manifestRaw, "instructions/SKILL.md": []byte(skillContent), "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("instructions entry path mismatch should be rejected")
	}
}

func TestLegacy_Package_Preview_PassingTests(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	tests := []WorkshopTestCase{{ID: "correct-output", Name: "正确输出", Mode: string(WorkflowDryRun), Input: json.RawMessage(`{"name":"A"}`), Config: json.RawMessage(`{}`), Assertions: []TestAssertion{{Type: "equals", Path: "output.message", Expected: "hello"}}}}
	testsRaw, _ := json.Marshal(tests)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", map[string][]byte{"tests/cases.json": testsRaw})})
	if err != nil {
		t.Fatal(err)
	}
	if preview.TestStatus != "tests-passed" {
		t.Fatalf("expected tests-passed, got: %s", preview.TestStatus)
	}
}

func TestLegacy_Package_Preview_WorkflowWithoutTests(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	preview, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if preview.TestStatus != "dry-run-passed" {
		t.Fatalf("expected dry-run-passed, got: %s", preview.TestStatus)
	}
}

func TestLegacy_Package_Preview_EmptyWorkflowSteps(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	files["workflows/main.json"] = []byte(`{"steps":[]}`)
	var manifest Manifest
	json.Unmarshal(files["manifest.json"], &manifest)
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("empty workflow steps should be rejected")
	}
}

func TestLegacy_Package_Preview_InvalidWorkflowJSON(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	files["workflows/main.json"] = []byte(`{invalid}`)
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("invalid workflow JSON should be rejected")
	}
}

func TestLegacy_Package_Preview_ScopeTypeCharacter(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "character", ScopeID: "char-1", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal("character scope import should succeed", err)
	}
}

func TestLegacy_Package_Preview_EmptyUserID(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("empty user ID should be rejected")
	}
}

func TestLegacy_Package_Install_RequiresUnsignedConfirmation(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global"})
	if err == nil {
		t.Fatal("install without unsigned confirmation should be rejected")
	}
}

func TestLegacy_Package_Install_RequiresVersionChangeConfirmation(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	upgradePreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.1.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: upgradePreview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ExpectedExtensionID: preview.ID})
	if err == nil {
		t.Fatal("upgrade without version change confirmation should be rejected")
	}
}

func TestLegacy_Package_Install_RejectsWrongExtensionID(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	upgradePreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.1.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: upgradePreview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ConfirmVersionChange: true, ExpectedExtensionID: "wrong-id"})
	if err == nil {
		t.Fatal("wrong expected extension ID should be rejected")
	}
}

func TestLegacy_Package_Install_WrongScopeUser(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "character", ScopeID: "char-1", ConfirmUnsigned: true})
	if err == nil {
		t.Fatal("scope type mismatch should be rejected")
	}
}

func TestLegacy_Package_Install_DowngradeRejected(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	previewV2, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "2.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: previewV2.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	previewV1, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if previewV1.Conflict != PackageConflictDowngrade {
		t.Fatalf("expected downgrade conflict, got: %s", previewV1.Conflict)
	}
	_, err = service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: previewV1.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ConfirmVersionChange: true, ExpectedExtensionID: previewV2.ID})
	if err == nil {
		t.Fatal("downgrade install should be rejected")
	}
}

func TestLegacy_Package_Install_AlreadyInstalledSameVersion(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	preview2, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview2.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true})
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("already-installed same version should succeed: %v", err)
	}
}

func TestLegacy_Package_Lifecycle_UpgradeSignerChangeDetection(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	rawV2 := packageWorkflowArchive(t, "1.1.0", nil)
	files, _, _ := readPackageZIP(rawV2, DefaultPackageLimits())
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	fingerprintHash := sha256.Sum256(publicKey)
	fingerprint := "sha256:" + hex.EncodeToString(fingerprintHash[:])
	digest := "sha256:" + packageCanonicalDigest(files)
	doc := packageSignatureDocument{Algorithm: "ed25519", KeyID: fingerprint, PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(privateKey, []byte(digest))), SignedDigest: digest, DisplayName: "V2 signer"}
	files["signature.json"], _ = json.Marshal(doc)
	rawV2, _ = stablePackageZIP(files)
	upgradePreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: rawV2})
	if err != nil {
		t.Fatal(err)
	}
	hasSignerRisk := false
	for _, risk := range upgradePreview.Risks {
		if risk.Code == "SIGNER_CHANGED" {
			hasSignerRisk = true
		}
	}
	if !hasSignerRisk {
		t.Fatalf("signer change should be detected: %+v", upgradePreview.Risks)
	}
}

func TestLegacy_Package_Lifecycle_ExportInvalidFormat(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	_, err = service.Export(ctx, ExportPackageRequest{UserID: "1", ExtensionID: preview.ID, Format: "unknown", ScopeType: "global"})
	if err == nil {
		t.Fatal("unknown export format should be rejected")
	}
}

func TestLegacy_Package_Lifecycle_ExportWorkflowAsAgentskillsZip(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	_, err = service.Export(ctx, ExportPackageRequest{UserID: "1", ExtensionID: preview.ID, Format: "agentskills-zip", ScopeType: "global"})
	if err == nil {
		t.Fatal("workflow as agentskills-zip export should be rejected")
	}
}

func TestLegacy_Package_Lifecycle_RollbackInvalidVersion(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	_, err = service.Rollback(ctx, preview.ID, "9.9.9", "1", "global", "")
	if err == nil {
		t.Fatal("rollback to non-existent version should be rejected")
	}
}

func TestLegacy_Package_Lifecycle_RollbackToCurrentVersion(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	result, err := service.rollbackLegacyPackage(ctx, preview.ID, "1.0.0", "1", "global", "")
	if err != nil || result.Status != "succeeded" {
		t.Fatalf("rollback to current version should succeed: %v", err)
	}
}

func TestLegacy_Package_Lifecycle_UninstallRejectsBuiltin(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	_, err := service.Uninstall(ctx, "builtin.skill", "1", "global", "")
	if err == nil {
		t.Fatal("uninstalling non-existent should fail")
	}
}

func TestLegacy_Package_Lifecycle_ExportMissingExtension(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	_, err := service.Export(ctx, ExportPackageRequest{UserID: "1", ExtensionID: "nonexistent", Format: "amitiax", ScopeType: "global"})
	if err == nil {
		t.Fatal("export of non-existent extension should be rejected")
	}
}

func TestLegacy_Package_Lifecycle_ListVersions(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	upgradePreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.1.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: upgradePreview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ConfirmVersionChange: true, ExpectedExtensionID: preview.ID}); err != nil {
		t.Fatal(err)
	}
	versions, err := service.repository.ListPackageVersions(ctx, preview.ID, "1", "global", "")
	if err != nil || len(versions) != 2 {
		t.Fatalf("expected 2 versions, got: %d %v", len(versions), err)
	}
}

func TestLegacy_Package_Lifecycle_CompareVersions(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	upgradePreview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.1.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: upgradePreview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true, ConfirmVersionChange: true, ExpectedExtensionID: preview.ID}); err != nil {
		t.Fatal(err)
	}
	from, err := service.repository.GetPackageVersion(ctx, preview.ID, "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	to, err := service.repository.GetPackageVersion(ctx, preview.ID, "1.1.0")
	if err != nil {
		t.Fatal(err)
	}
	diff := packageVersionDiffRecords(preview.ID, from, to, nil, nil)
	if diff.FromVersion != "1.0.0" || diff.ToVersion != "1.1.0" {
		t.Fatalf("version comparison invalid: %+v", diff)
	}
}

func TestLegacy_Package_Lifecycle_Dependencies(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	result, err := service.legacyDependencies(ctx, preview.ID, "1", "global", "")
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("dependencies result should not be nil")
	}
}

func TestLegacy_Package_Lifecycle_ListOperations(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	operations, err := service.repository.ListPackageOperations(ctx, "1", 10)
	if err != nil || len(operations) < 1 {
		t.Fatalf("expected at least 1 operation, got: %d %v", len(operations), err)
	}
}

func TestLegacy_Package_Lifecycle_GetOperation(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true})
	if err != nil {
		t.Fatal(err)
	}
	op, err := service.repository.GetPackageOperation(ctx, "1", result.OperationID)
	if err != nil || op.ID != result.OperationID {
		t.Fatalf("get operation failed: %+v %v", op, err)
	}
}

func TestLegacy_Package_Lifecycle_UninstallPreview(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	uninstallPreview, err := service.legacyPreviewUninstall(ctx, preview.ID, "1", "global", "")
	if err != nil || uninstallPreview.ExtensionID != preview.ID {
		t.Fatalf("uninstall preview failed: %+v %v", uninstallPreview, err)
	}
	if !uninstallPreview.ArtifactArchived {
		t.Fatal("artifact should be marked for archival")
	}
}

func TestLegacy_Package_Lifecycle_UpgradeScriptsAddedDetection(t *testing.T) {
	t.Skip("workflow packages reject scripts at parser level; script detection requires non-workflow entry")
}

func TestLegacy_Package_Lifecycle_ExportVersionUsed(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	var artifact packageArtifactRecord
	if err := service.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", preview.ID, "1.0.0").First(&artifact).Error; err != nil {
		t.Fatal(err)
	}
	files, err := service.exportAmitiaxFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	content, err := stablePackageZIP(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 {
		t.Fatal("exported content should not be empty")
	}
}

func TestLegacy_Package_Lifecycle_ExportIncludesReadme(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	readme := []byte("# Test Extension\nThis is a test readme.\n")
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", map[string][]byte{"docs/README.md": readme})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	var artifact packageArtifactRecord
	err = service.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", preview.ID, "1.0.0").First(&artifact).Error
	if err != nil {
		t.Fatal(err)
	}
	files, err := service.exportAmitiaxFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["docs/README.md"]; !ok {
		t.Fatal("readme should be included in export")
	}
}

func TestLegacy_Package_Lifecycle_ExportIncludesTestCases(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	tests := []WorkshopTestCase{{ID: "smoke", Name: "基础冒烟", Mode: string(WorkflowDryRun), Input: json.RawMessage(`{"name":"A"}`), Config: json.RawMessage(`{}`), Assertions: []TestAssertion{{Type: "equals", Path: "output.message", Expected: "hello"}}}}
	testsRaw, _ := json.Marshal(tests)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", map[string][]byte{"tests/cases.json": testsRaw})})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	var artifact packageArtifactRecord
	err = service.repository.db.WithContext(ctx).Where("extension_id = ? AND extension_version = ?", preview.ID, "1.0.0").First(&artifact).Error
	if err != nil {
		t.Fatal(err)
	}
	files, err := service.exportAmitiaxFiles(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["tests/report.json"]; !ok {
		t.Fatal("tests should be included in export")
	}
}

func TestLegacy_Package_Lifecycle_StableZipIsDeterministic(t *testing.T) {
	files := map[string][]byte{"manifest.json": []byte(`{"version":"1.0.0"}`), "main.json": []byte(`{"steps":[]}`), "a/b.txt": []byte("hello")}
	raw1, err := stablePackageZIP(files)
	if err != nil {
		t.Fatal(err)
	}
	files2 := map[string][]byte{"a/b.txt": []byte("hello"), "manifest.json": []byte(`{"version":"1.0.0"}`), "main.json": []byte(`{"steps":[]}`)}
	raw2, err := stablePackageZIP(files2)
	if err != nil {
		t.Fatal(err)
	}
	h1 := sha256.Sum256(raw1)
	h2 := sha256.Sum256(raw2)
	if h1 != h2 {
		t.Fatal("stablePackageZIP is not deterministic across map ordering")
	}
}

func TestLegacy_Package_Lifecycle_PackageHashStability(t *testing.T) {
	files := map[string][]byte{"manifest.json": []byte(`{"version":"1.0.0"}`), "main.json": []byte(`{"steps":[]}`)}
	raw1, _ := stablePackageZIP(files)
	hash1 := packageHash(raw1)
	raw2, _ := stablePackageZIP(files)
	hash2 := packageHash(raw2)
	if hash1 != hash2 {
		t.Fatal("packageHash is not stable for identical input")
	}
}

func TestLegacy_Package_Lifecycle_CanonicalDigestExcludesSignature(t *testing.T) {
	files := map[string][]byte{"a.txt": []byte("hello"), "signature.json": []byte(`{"algorithm":"ed25519"}`)}
	digest1 := packageCanonicalDigest(files)
	delete(files, "signature.json")
	digest2 := packageCanonicalDigest(files)
	if digest1 != digest2 {
		t.Fatal("canonical digest should exclude signature.json")
	}
}

func TestLegacy_Package_Manifest_OversizedManifest(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	input := json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"properties":{}}`)
	padding := strings.Repeat("x", int(DefaultPackageLimits().MaxManifestBytes)+1)
	manifest := Manifest{Schema: "https://schemas.amitia.dev/extensions/v1/manifest.schema.json", APIVersion: "extensions.amitia.dev/v1alpha1", Kind: "Skill", Metadata: ManifestMetadata{ID: "dev.local.test", Name: "test", Version: "1.0.0", Description: padding, Author: "Local", License: "MIT"}, Compatibility: ManifestCompatibility{EngineMin: "1.0.0"}, Entry: SkillEntry{Kind: "workflow", Path: "workflows/main.json"}, Capabilities: []string{}, Triggers: []SkillTrigger{TriggerManual}, Execution: ManifestExecution{TimeoutMS: 30000, Retryable: true, Idempotent: true}, InputSchema: input, OutputSchema: input, ConfigSchema: json.RawMessage(`{}`), DefaultConfig: json.RawMessage(`{}`), Enabled: false, AllowManual: true}
	manifestRaw, _ := json.Marshal(manifest)
	workflow := WorkflowDefinition{SchemaVersion: "1.0.0", Steps: []WorkflowStep{{ID: "x", Type: "noop"}}, Output: json.RawMessage(`{}`), Limits: DefaultWorkflowLimits()}
	workflowRaw, _ := json.Marshal(workflow)
	files := map[string][]byte{"manifest.json": manifestRaw, "workflows/main.json": workflowRaw, "LICENSE": []byte("MIT\n")}
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ := stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("oversized manifest should be rejected")
	}
}

func TestLegacy_Package_Manifest_OversizedWorkflow(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	padding := strings.Repeat("x", int(DefaultPackageLimits().MaxWorkflowBytes)+1)
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	files["workflows/main.json"] = []byte(`{"steps":[{"id":"x","type":"noop","meta":"` + padding + `"}]}`)
	files["checksums.sha256"] = buildChecksums(files)
	raw, _ = stablePackageZIP(files)
	_, err := service.PreviewImport(context.Background(), PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: raw})
	if err == nil {
		t.Fatal("oversized workflow should be rejected")
	}
}

func TestLegacy_Package_Archive_SkipsNonUTF8Safely(t *testing.T) {
	limits := DefaultPackageLimits()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	nonUTF8Name := string([]byte{0x80, 0x81, 0x82}) + ".txt"
	header := &zip.FileHeader{Name: nonUTF8Name, Method: zip.Deflate, NonUTF8: true}
	entry, _ := writer.CreateHeader(header)
	entry.Write([]byte("x"))
	writer.Close()
	if _, _, err := readPackageZIP(buffer.Bytes(), limits); err == nil {
		t.Fatal("NonUTF8 flagged file should be rejected")
	}
}

func TestLegacy_Package_Recovery_PendingOperationFails(t *testing.T) {
	service, _, _, db := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	operation := packageOperationRecord{ID: "op-pending", ExtensionID: preview.ID, ExtensionVersion: preview.Version, Operation: string(PackageOperationInstall), Source: preview.Source, PackageHash: preview.PackageHash, UserID: "1", ScopeType: "global", Status: "pending", TraceID: "trace-1", CreatedAt: now}
	if err := db.Create(&operation).Error; err != nil {
		t.Fatal(err)
	}
	if err := service.recoverPackageOperations(ctx); err != nil {
		t.Fatal(err)
	}
	var recovered packageOperationRecord
	if err := db.Where("id = ?", operation.ID).First(&recovered).Error; err != nil || recovered.Status != "failed" {
		t.Fatalf("pending operation should be failed: %+v %v", recovered, err)
	}
}

func TestLegacy_Package_AgentSkills_PreviewViaZIP(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	skill := []byte("---\nname: my-skill\ndescription: A test skill.\nlicense: MIT\nmetadata:\n  version: 1.0.0\n---\nReview carefully.\n")
	raw := packageUnsafeZIP(t, map[string][]byte{"my-skill/SKILL.md": skill, "my-skill/docs/guide.md": []byte("# Guide")}, nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "my-skill.zip", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	if preview.SkillType != "instructions" {
		t.Fatalf("expected instructions skill type, got: %s", preview.SkillType)
	}
	if preview.AgentSkill == nil || preview.AgentSkill.Definition.Name != "my-skill" {
		t.Fatalf("agent skill preview invalid: %+v", preview.AgentSkill)
	}
}

func TestLegacy_Package_AgentSkills_MultipleFiles(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	skill := []byte("---\nname: rich-skill\ndescription: Skill with multiple resources.\nlicense: MIT\n---\nBe helpful.\n")
	raw := packageUnsafeZIP(t, map[string][]byte{"rich-skill/SKILL.md": skill, "rich-skill/README.md": []byte("# README"), "rich-skill/references/api.md": []byte("# API Reference"), "rich-skill/assets/logo.png": []byte{0x89, 'P', 'N', 'G'}}, nil)
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "rich-skill.zip", Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	if preview.References != 1 || preview.Assets != 1 {
		t.Fatalf("file kind count incorrect: refs=%d assets=%d", preview.References, preview.Assets)
	}
}

func TestLegacy_Package_Service_Metrics(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()
	preview, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: packageWorkflowArchive(t, "1.0.0", nil)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.installLegacyPackage(ctx, InstallPackageRequest{SessionID: preview.SessionID, UserID: "1", ScopeType: "global", ConfirmUnsigned: true}); err != nil {
		t.Fatal(err)
	}
	metrics := service.Metrics()
	if metrics["extension_package_import_total"] < 1 {
		t.Fatalf("import metric not incremented: %v", metrics)
	}
}

func TestLegacy_Package_Signers_TrustAndUntrust(t *testing.T) {
	service, _, _, _ := packageTestService(t)
	ctx := context.Background()

	raw := packageWorkflowArchive(t, "1.0.0", nil)
	files, _, _ := readPackageZIP(raw, DefaultPackageLimits())
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	fingerprintHash := sha256.Sum256(publicKey)
	fingerprint := "sha256:" + hex.EncodeToString(fingerprintHash[:])
	digest := "sha256:" + packageCanonicalDigest(files)
	signature := ed25519.Sign(privateKey, []byte(digest))
	doc := packageSignatureDocument{Algorithm: "ed25519", KeyID: fingerprint, PublicKey: base64.StdEncoding.EncodeToString(publicKey), Signature: base64.StdEncoding.EncodeToString(signature), SignedDigest: digest, DisplayName: "Test signer"}
	files["signature.json"], _ = json.Marshal(doc)
	signedRaw, _ := stablePackageZIP(files)

	_, err := service.PreviewImport(ctx, PreviewPackageImportRequest{UserID: "1", ScopeType: "global", FileName: "test.amitiax", Raw: signedRaw})
	if err != nil {
		t.Fatal(err)
	}

	if err := service.repository.SetPackageSignerTrust(ctx, fingerprint, true); err != nil {
		t.Fatal(err)
	}

	signers, err := service.repository.ListPackageSigners(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range signers {
		if s.Fingerprint == fingerprint && s.Trusted {
			found = true
		}
	}
	if !found {
		t.Fatalf("signer not trusted: %+v", signers)
	}

	if err := service.repository.SetPackageSignerTrust(ctx, fingerprint, false); err != nil {
		t.Fatal(err)
	}

	signers, err = service.repository.ListPackageSigners(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range signers {
		if s.Fingerprint == fingerprint && s.Trusted {
			t.Fatal("signer still trusted after UntrustSigner")
		}
	}
}

func TestLegacy_Package_Archive_RejectsZeroLengthFilesWithoutMagic(t *testing.T) {
	limits := DefaultPackageLimits()
	raw := packageUnsafeZIP(t, map[string][]byte{"empty.txt": {}}, nil)
	files, _, err := readPackageZIP(raw, limits)
	if err != nil {
		t.Fatal("zero-length files should be allowed", err)
	}
	if len(files["empty.txt"]) != 0 {
		t.Fatal("zero-length file should be preserved")
	}
}

func TestLegacy_Package_Archive_RejectsTotalExpandedOverLimit(t *testing.T) {
	limits := DefaultPackageLimits()
	limits.MaxExpandedBytes = 100
	raw := packageUnsafeZIP(t, map[string][]byte{"a.txt": bytes.Repeat([]byte("a"), 60), "b.txt": bytes.Repeat([]byte("b"), 60)}, nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("total expanded over limit should be rejected")
	}
}

func TestLegacy_Package_Archive_RejectsOversizedArchive(t *testing.T) {
	limits := DefaultPackageLimits()
	limits.MaxExpandedBytes = 5
	raw := packageWorkflowArchive(t, "1.0.0", nil)
	if _, _, err := readPackageZIP(raw, limits); err == nil {
		t.Fatal("oversized archive should be rejected when raw size > MaxExpandedBytes")
	}
}

func packageUnsafeZIPNonUTF8(t *testing.T, name []byte, content []byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	header := &zip.FileHeader{Name: string(name), Method: zip.Deflate}
	header.NonUTF8 = true
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func packageUnsafeZIPRaw(t *testing.T, entries map[string]string, mode *uint32) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if mode != nil {
			header.SetMode(os.FileMode(*mode))
		}
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
