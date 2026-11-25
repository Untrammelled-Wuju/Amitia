package plugincontract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/devicemesh/business"
)

var memorySearchInputProperties = []string{
	"contextUsable",
	"includeDormant",
	"types",
	"layers",
	"minimumImportance",
	"time",
	"exactTitle",
	"retentionLevel",
	"pinned",
	"decayState",
	"query",
	"mode",
	"memoryType",
	"source",
	"sort",
	"cursor",
	"limit",
}

var memorySearchOutputProperties = []string{
	"total",
	"resources",
	"executionScope",
	"saved",
	"acknowledgement",
	"nextCursor",
	"scores",
	"scoreDetails",
}

var memoryTimeFilterProperties = []string{
	"basis",
	"from",
	"to",
	"at",
	"localDateFrom",
	"localDateTo",
	"includeUnknown",
}

func decodeSchemaObject(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	return decoded
}

func schemaPropertyNames(t *testing.T, schema map[string]any) []string {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties object: %v", schema)
	}
	names := make([]string, 0, len(properties))
	for name := range properties {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func schemaProperty(t *testing.T, schema map[string]any, name string) map[string]any {
	t.Helper()
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema has no properties object")
	}
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("schema property %s missing", name)
	}
	return property
}

func sortedNames(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func schemaRequired(t *testing.T, schema map[string]any) []string {
	t.Helper()
	raw, ok := schema["required"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("required must be an array, got %T", raw)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("required item must be a string, got %T", item)
		}
		out = append(out, text)
	}
	sort.Strings(out)
	return out
}

func mustCatalog(t *testing.T) CoreDescriptorCatalog {
	t.Helper()
	catalog, err := DefaultCoreDescriptorCatalog()
	if err != nil {
		t.Fatalf("build default core catalog: %v", err)
	}
	return catalog
}

func TestDefaultCoreCatalogHasMemorySearch(t *testing.T) {
	catalog := mustCatalog(t)
	if catalog.ProviderModule.PluginID != CoreProviderPluginID || catalog.ProviderModule.ModuleID != CoreProviderModuleID {
		t.Fatalf("provider module = %+v", catalog.ProviderModule)
	}
	if len(catalog.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(catalog.Entries))
	}
	entry, ok := catalog.Find(CoreMemorySearchServiceID)
	if !ok {
		t.Fatalf("memory.search entry missing")
	}
	if entry.ProviderID != "builtin.core/core/memory.search" {
		t.Fatalf("provider id = %q", entry.ProviderID)
	}
	if entry.CapabilityID != "builtin/core/memory.search" {
		t.Fatalf("capability id = %q", entry.CapabilityID)
	}
	if entry.Descriptor.ID != entry.CapabilityID {
		t.Fatalf("descriptor id = %q", entry.Descriptor.ID)
	}
	if entry.Version != CoreDescriptorVersion {
		t.Fatalf("version = %q", entry.Version)
	}
	if err := ValidateContractHash(entry.ContractHash); err != nil {
		t.Fatalf("contract hash %q invalid: %v", entry.ContractHash, err)
	}
	if entry.Descriptor.Placement != ServicePlacementDevice {
		t.Fatalf("placement = %q", entry.Descriptor.Placement)
	}
	if len(entry.Descriptor.Scopes) != 2 || entry.Descriptor.Scopes[0] != ServiceScopeConversation || entry.Descriptor.Scopes[1] != ServiceScopeDevice {
		t.Fatalf("scopes = %v", entry.Descriptor.Scopes)
	}
	if len(entry.Descriptor.Permissions) != 1 {
		t.Fatalf("permissions = %+v", entry.Descriptor.Permissions)
	}
	if entry.Descriptor.Permissions[0].Capability != "memory.read" || entry.Descriptor.Permissions[0].Risk != "low" {
		t.Fatalf("permission = %+v", entry.Descriptor.Permissions[0])
	}
	if entry.Descriptor.InputRef == nil || entry.Descriptor.OutputRef == nil {
		t.Fatalf("schema refs must be present")
	}
	if entry.Descriptor.InputRef.Hash == entry.Descriptor.OutputRef.Hash {
		t.Fatalf("input and output schema hashes must differ")
	}
	permissionFields := make([]string, 0, 4)
	permissionType := reflect.TypeOf(DescriptorPermission{})
	for index := 0; index < permissionType.NumField(); index++ {
		permissionFields = append(permissionFields, permissionType.Field(index).Name)
	}
	sort.Strings(permissionFields)
	if !reflect.DeepEqual(permissionFields, []string{"Capability", "Risk"}) {
		t.Fatalf("descriptor permission fields = %v", permissionFields)
	}
}

func TestMemorySearchInputSchemaTracksBusinessStruct(t *testing.T) {
	catalog := mustCatalog(t)
	entry, _ := catalog.Find(CoreMemorySearchServiceID)
	schema := decodeSchemaObject(t, entry.InputSchema)
	if schema["type"] != "object" {
		t.Fatalf("schema type = %v", schema["type"])
	}
	if names := schemaPropertyNames(t, schema); !reflect.DeepEqual(names, sortedNames(memorySearchInputProperties)) {
		t.Fatalf("input properties = %v", names)
	}
	if required := schemaRequired(t, schema); !reflect.DeepEqual(required, []string{"mode", "query"}) {
		t.Fatalf("input required = %v", required)
	}
	if got := schemaProperty(t, schema, "query")["type"]; got != "string" {
		t.Fatalf("query type = %v", got)
	}
	if got := schemaProperty(t, schema, "limit")["type"]; got != "integer" {
		t.Fatalf("limit type = %v", got)
	}
	if got := schemaProperty(t, schema, "pinned")["type"]; got != "boolean" {
		t.Fatalf("pinned type = %v", got)
	}
	types := schemaProperty(t, schema, "types")
	if types["type"] != "array" {
		t.Fatalf("types type = %v", types["type"])
	}
	items, ok := types["items"].(map[string]any)
	if !ok || items["type"] != "string" {
		t.Fatalf("types items = %v", types["items"])
	}
	timeSchema := schemaProperty(t, schema, "time")
	if names := schemaPropertyNames(t, timeSchema); !reflect.DeepEqual(names, sortedNames(memoryTimeFilterProperties)) {
		t.Fatalf("time properties = %v", names)
	}
	if required := schemaRequired(t, timeSchema); !reflect.DeepEqual(required, []string{"basis"}) {
		t.Fatalf("time required = %v", required)
	}
}

func TestMemorySearchOutputSchemaTracksBusinessStruct(t *testing.T) {
	catalog := mustCatalog(t)
	entry, _ := catalog.Find(CoreMemorySearchServiceID)
	schema := decodeSchemaObject(t, entry.OutputSchema)
	if names := schemaPropertyNames(t, schema); !reflect.DeepEqual(names, sortedNames(memorySearchOutputProperties)) {
		t.Fatalf("output properties = %v", names)
	}
	if required := schemaRequired(t, schema); !reflect.DeepEqual(required, []string{"acknowledgement", "executionScope", "resources", "saved", "total"}) {
		t.Fatalf("output required = %v", required)
	}
	resources := schemaProperty(t, schema, "resources")
	if resources["type"] != "array" {
		t.Fatalf("resources type = %v", resources["type"])
	}
	resourceItems, ok := resources["items"].(map[string]any)
	if !ok {
		t.Fatalf("resources items missing")
	}
	body := schemaProperty(t, resourceItems, "body")
	if len(body) != 0 {
		t.Fatalf("body schema must stay open, got %v", body)
	}
	versions := schemaProperty(t, schemaProperty(t, schema, "acknowledgement"), "versions")
	additional, ok := versions["additionalProperties"].(map[string]any)
	if !ok || additional["type"] != "integer" {
		t.Fatalf("versions additionalProperties = %v", versions["additionalProperties"])
	}
	scores := schemaProperty(t, schema, "scores")
	scoreValue, ok := scores["additionalProperties"].(map[string]any)
	if !ok || scoreValue["type"] != "number" {
		t.Fatalf("scores additionalProperties = %v", scores["additionalProperties"])
	}
	scoreDetails := schemaProperty(t, schema, "scoreDetails")
	scoreDetailValue, ok := scoreDetails["additionalProperties"].(map[string]any)
	if !ok {
		t.Fatalf("scoreDetails additionalProperties missing")
	}
	if names := schemaPropertyNames(t, scoreDetailValue); len(names) != 13 {
		t.Fatalf("score detail properties = %v", names)
	}
	if required := schemaRequired(t, scoreDetailValue); len(required) != 13 {
		t.Fatalf("score detail required = %v", required)
	}
	if got := schemaProperty(t, scoreDetailValue, "finalScore")["type"]; got != "number" {
		t.Fatalf("finalScore type = %v", got)
	}
	if got := schemaProperty(t, scoreDetailValue, "keywordMatch")["type"]; got != "boolean" {
		t.Fatalf("keywordMatch type = %v", got)
	}
}

func TestCoreSchemasAreContentAddressed(t *testing.T) {
	catalog := mustCatalog(t)
	entry, _ := catalog.Find(CoreMemorySearchServiceID)
	for _, item := range []struct {
		name string
		ref  SchemaRef
		raw  json.RawMessage
	}{
		{name: "input", ref: *entry.Descriptor.InputRef, raw: entry.InputSchema},
		{name: "output", ref: *entry.Descriptor.OutputRef, raw: entry.OutputSchema},
	} {
		resolved, err := catalog.Resolve(item.ref)
		if err != nil {
			t.Fatalf("%s resolve: %v", item.name, err)
		}
		canonical, err := CanonicalizeJSONValue(item.raw)
		if err != nil {
			t.Fatalf("%s canonicalize: %v", item.name, err)
		}
		if string(resolved) != string(canonical) {
			t.Fatalf("%s resolved bytes differ from canonical schema", item.name)
		}
		sum := sha256.Sum256(canonical)
		if hex.EncodeToString(sum[:]) != item.ref.Hash {
			t.Fatalf("%s hash is not sha256 of canonical schema", item.name)
		}
		if item.ref.MediaType != DefaultSchemaMediaType {
			t.Fatalf("%s media type = %q", item.name, item.ref.MediaType)
		}
	}
	if len(catalog.Schemas) != 2 {
		t.Fatalf("schema table entries = %d, want 2", len(catalog.Schemas))
	}
	if _, err := catalog.Resolve(SchemaRef{MediaType: DefaultSchemaMediaType, Hash: strings.Repeat("a", 64)}); !errors.Is(err, ErrUnknownSchemaRef) {
		t.Fatalf("unknown ref error = %v", err)
	}
}

func TestCoreCatalogSnapshotIsDeterministicAndSchemaFree(t *testing.T) {
	catalog := mustCatalog(t)
	first, err := catalog.Snapshot()
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	second, err := catalog.Snapshot()
	if err != nil {
		t.Fatalf("snapshot twice: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("snapshot is not deterministic")
	}
	var decoded CoreDescriptorCatalog
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(decoded.Entries) != 1 || len(decoded.Schemas) != 2 {
		t.Fatalf("snapshot entries=%d schemas=%d", len(decoded.Entries), len(decoded.Schemas))
	}
	listing, err := json.Marshal(catalog.Listing())
	if err != nil {
		t.Fatalf("encode listing: %v", err)
	}
	if strings.Contains(string(listing), `"properties"`) {
		t.Fatalf("listing must not inline json schema bodies")
	}
	if !strings.Contains(string(listing), `"inputSchemaRef"`) {
		t.Fatalf("listing must carry schema refs, got %s", string(listing))
	}
	if !strings.Contains(string(first), `"properties"`) {
		t.Fatalf("snapshot carries schema bodies by design")
	}
}

func TestCoreContractHashCoversSchemaVersionAndScopes(t *testing.T) {
	base := CoreMemorySearchSpec()
	baseline, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{base})
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}
	baselineEntry, _ := baseline.Find(CoreMemorySearchServiceID)

	alternateInput := base
	alternateInput.Input = business.MemoryManagementInput{}
	alternate, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{alternateInput})
	if err != nil {
		t.Fatalf("alternate input: %v", err)
	}
	alternateEntry, _ := alternate.Find(CoreMemorySearchServiceID)
	if alternateEntry.ContractHash == baselineEntry.ContractHash {
		t.Fatalf("contract hash must change with input schema")
	}
	if alternateEntry.CapabilityID != baselineEntry.CapabilityID || alternateEntry.ProviderID != baselineEntry.ProviderID {
		t.Fatalf("identity must stay stable when only the schema changes")
	}

	bumped := base
	bumped.Version = "1.1.0"
	versioned, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{bumped})
	if err != nil {
		t.Fatalf("version bump: %v", err)
	}
	versionedEntry, _ := versioned.Find(CoreMemorySearchServiceID)
	if versionedEntry.ContractHash == baselineEntry.ContractHash {
		t.Fatalf("contract hash must change with version")
	}

	scoped := base
	scoped.Scopes = []ServiceScope{ServiceScopeDevice}
	scopedCatalog, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{scoped})
	if err != nil {
		t.Fatalf("scope change: %v", err)
	}
	scopedEntry, _ := scopedCatalog.Find(CoreMemorySearchServiceID)
	if scopedEntry.ContractHash == baselineEntry.ContractHash {
		t.Fatalf("contract hash must change with scopes")
	}

	expected, err := CanonicalContractHash(ContractDigest{
		ProtocolVersion: ContractProtocolVersion,
		ServiceID:       CoreMemorySearchServiceID,
		Version:         CoreDescriptorVersion,
		Placement:       ServicePlacementDevice,
		Scopes:          base.Scopes,
		InputSchema:     baselineEntry.InputSchema,
		OutputSchema:    baselineEntry.OutputSchema,
	})
	if err != nil {
		t.Fatalf("independent digest: %v", err)
	}
	if expected != baselineEntry.ContractHash {
		t.Fatalf("contract hash is not reproducible: %q vs %q", expected, baselineEntry.ContractHash)
	}
}

func TestCoreCatalogRejectsInvalidSpecs(t *testing.T) {
	base := CoreMemorySearchSpec()

	duplicated := base
	if _, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{base, duplicated}); !errors.Is(err, ErrInvalidCoreDescriptor) {
		t.Fatalf("duplicated service error = %v", err)
	}

	noInput := base
	noInput.Input = nil
	if _, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{noInput}); !errors.Is(err, ErrInvalidCoreDescriptor) {
		t.Fatalf("missing input error = %v", err)
	}

	badPlacement := base
	badPlacement.Placement = ServicePlacement("orbit")
	if _, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{badPlacement}); !errors.Is(err, ErrInvalidCoreDescriptor) {
		t.Fatalf("bad placement error = %v", err)
	}

	badSummary := base
	badSummary.Summary = "search\nmemories"
	if _, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{badSummary}); !errors.Is(err, ErrInvalidCoreDescriptor) {
		t.Fatalf("multi-line summary error = %v", err)
	}

	badService := base
	badService.ServiceID = "Memory Search"
	if _, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{badService}); !errors.Is(err, ErrInvalidCoreDescriptor) {
		t.Fatalf("bad service id error = %v", err)
	}

	badPermission := base
	badPermission.Permissions = []DescriptorPermission{{Capability: "memory.read", Risk: "extreme"}}
	if _, err := BuildCoreDescriptorCatalog([]CoreCapabilitySpec{badPermission}); !errors.Is(err, ErrInvalidCoreDescriptor) {
		t.Fatalf("bad permission risk error = %v", err)
	}
}

type schemaSampleNested struct {
	Label   string            `json:"label"`
	Count   int               `json:"count,omitempty"`
	Flag    bool              `json:"flag"`
	Ratio   float64           `json:"ratio"`
	Moment  time.Time         `json:"moment,omitempty"`
	Payload []byte            `json:"payload,omitempty"`
	Raw     json.RawMessage   `json:"raw,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Tags    []string          `json:"tags"`
	Child   *schemaSampleLeaf `json:"child,omitempty"`
	Skipped string            `json:"-"`
}

type schemaSampleLeaf struct {
	Value uint64 `json:"value"`
}

type schemaSampleEmbedded struct {
	EmbeddedField string `json:"embeddedField"`
}

type schemaSampleOuter struct {
	schemaSampleEmbedded
	Own string `json:"own"`
}

func TestSchemaForGoTypeCoversStructShapes(t *testing.T) {
	raw, err := SchemaForGoType(reflect.TypeOf(schemaSampleNested{}))
	if err != nil {
		t.Fatalf("nested schema: %v", err)
	}
	schema := decodeSchemaObject(t, raw)
	want := []string{"label", "count", "flag", "ratio", "moment", "payload", "raw", "labels", "tags", "child"}
	if names := schemaPropertyNames(t, schema); !reflect.DeepEqual(names, sortedNames(want)) {
		t.Fatalf("properties = %v", names)
	}
	if required := schemaRequired(t, schema); !reflect.DeepEqual(required, []string{"flag", "label", "ratio", "tags"}) {
		t.Fatalf("required = %v", required)
	}
	if got := schemaProperty(t, schema, "moment")["format"]; got != "date-time" {
		t.Fatalf("moment format = %v", got)
	}
	if got := schemaProperty(t, schema, "payload")["contentEncoding"]; got != "base64" {
		t.Fatalf("payload encoding = %v", got)
	}
	if got := len(schemaProperty(t, schema, "raw")); got != 0 {
		t.Fatalf("raw message schema must stay open, got %d keys", got)
	}
	labels := schemaProperty(t, schema, "labels")
	if labels["type"] != "object" {
		t.Fatalf("labels type = %v", labels["type"])
	}
	child := schemaProperty(t, schema, "child")
	if names := schemaPropertyNames(t, child); !reflect.DeepEqual(names, []string{"value"}) {
		t.Fatalf("child properties = %v", names)
	}
	if got := schemaProperty(t, child, "value"); got["type"] != "integer" || got["minimum"] != float64(0) && got["minimum"] != 0 {
		t.Fatalf("uint schema = %v", got)
	}

	embeddedRaw, err := SchemaForGoType(reflect.TypeOf(schemaSampleOuter{}))
	if err != nil {
		t.Fatalf("embedded schema: %v", err)
	}
	embedded := decodeSchemaObject(t, embeddedRaw)
	if names := schemaPropertyNames(t, embedded); !reflect.DeepEqual(names, []string{"embeddedField", "own"}) {
		t.Fatalf("embedded properties = %v", names)
	}
}

type schemaSampleRecursive struct {
	Next *schemaSampleRecursive `json:"next,omitempty"`
}

func TestSchemaForGoTypeRejectsUnsupportedShapes(t *testing.T) {
	if _, err := SchemaForGoType(reflect.TypeOf(schemaSampleRecursive{})); !errors.Is(err, ErrUnsupportedSchemaType) {
		t.Fatalf("recursive error = %v", err)
	}
	if _, err := SchemaForGoType(reflect.TypeOf(make(chan int))); !errors.Is(err, ErrUnsupportedSchemaType) {
		t.Fatalf("channel error = %v", err)
	}
	if _, err := SchemaForGoType(reflect.TypeOf(map[int]string{})); !errors.Is(err, ErrUnsupportedSchemaType) {
		t.Fatalf("non-string map key error = %v", err)
	}
	if _, err := SchemaForGoValue(nil); !errors.Is(err, ErrUnsupportedSchemaType) {
		t.Fatalf("nil sample error = %v", err)
	}
}

func TestSchemaForGoTypeIsDeterministic(t *testing.T) {
	first, err := SchemaForGoValue(business.MemoryManagementResponse{})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := SchemaForGoValue(business.MemoryManagementResponse{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("schema derivation is not deterministic")
	}
	if _, err := CanonicalizeJSONValue(first); err != nil {
		t.Fatalf("schema is not valid json: %v", err)
	}
}
