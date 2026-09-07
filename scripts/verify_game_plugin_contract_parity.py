#!/usr/bin/env python3
"""Verify public Game Plugin contract parity across Go, JSON Schema, and TypeScript.

The GameHost protocol is intentionally language-neutral. This gate prevents a
public SDK from silently dropping or inventing manifest/channel fields while the
canonical Go contract and JSON Schema continue to accept a different shape.

No third-party Python packages are required so the check can run in CI before
npm/go dependency installation.
"""
from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
GO_GAME = ROOT / "backend/pkg/gameplugin/protocol/game_contract.go"
GO_CHANNEL = ROOT / "backend/pkg/gameplugin/protocol/channel.go"
GO_SERVICE = ROOT / "backend/pkg/gameplugin/protocol/service.go"
TS_GAME = ROOT / "backend/pkg/gameplugin/sdk/game-plugin/src/game.ts"
TS_PROTOCOL = ROOT / "backend/pkg/gameplugin/sdk/game-plugin/src/protocol.ts"
TS_INDEX = ROOT / "backend/pkg/gameplugin/sdk/game-plugin/src/index.ts"
HOST_SCHEMA = ROOT / "backend/pkg/gameplugin/schema/plugin-host-spec.schema.json"
CHANNEL_SCHEMA = ROOT / "backend/pkg/gameplugin/schema/channel.schema.json"
SERVICE_SCHEMA = ROOT / "backend/pkg/gameplugin/schema/service.schema.json"
FEATURE_SCHEMA = ROOT / "backend/pkg/gameplugin/schema/capability.schema.json"
VALIDATOR_FIXTURE = ROOT / "backend/pkg/gameplugin/testdata/conformance/host_spec_validation_cases.json"
GO_VALIDATOR_TEST = ROOT / "backend/pkg/gameplugin/protocol/host_spec_conformance_test.go"
TS_VALIDATOR_TEST = ROOT / "backend/pkg/gameplugin/sdk/game-plugin/test/conformance.test.ts"


def fail(message: str) -> None:
    print(f"ERROR: {message}", file=sys.stderr)
    raise SystemExit(1)


def load_json(path: Path) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:  # pragma: no cover - CI diagnostic path
        fail(f"cannot parse {path.relative_to(ROOT)}: {exc}")


def assert_same(label: str, expected: set[str], actual: set[str]) -> None:
    if expected == actual:
        return
    missing = sorted(expected - actual)
    extra = sorted(actual - expected)
    fail(f"{label} drift: missing={missing} extra={extra}")


def ts_interface(text: str, name: str) -> tuple[set[str], set[str]]:
    match = re.search(rf"(?:export\s+)?interface\s+{re.escape(name)}\s*\{{(.*?)\n\}}", text, re.S)
    if not match:
        fail(f"TypeScript interface {name} not found")
    fields: set[str] = set()
    required: set[str] = set()
    for line in match.group(1).splitlines():
        prop = re.match(r"\s*([A-Za-z_$][\w$]*)(\?)?\s*:", line)
        if not prop:
            continue
        fields.add(prop.group(1))
        if prop.group(2) != "?":
            required.add(prop.group(1))
    return fields, required


def ts_property_literals(text: str, interface: str, prop: str) -> set[str]:
    match = re.search(rf"export\s+interface\s+{re.escape(interface)}\s*\{{(.*?)\n\}}", text, re.S)
    if not match:
        fail(f"TypeScript interface {interface} not found")
    prop_match = re.search(rf"^\s*{re.escape(prop)}\??\s*:\s*([^;]+);", match.group(1), re.M)
    if not prop_match:
        fail(f"TypeScript property {interface}.{prop} not found")
    values = set(re.findall(r"'([^']+)'", prop_match.group(1)))
    if not values:
        fail(f"TypeScript property {interface}.{prop} has no literal union")
    return values


def ts_type_literals(text: str, name: str) -> set[str]:
    match = re.search(rf"export\s+type\s+{re.escape(name)}\s*=\s*([^;]+);", text)
    if not match:
        fail(f"TypeScript type {name} not found")
    values = set(re.findall(r"'([^']+)'", match.group(1)))
    if not values:
        fail(f"TypeScript type {name} has no literal values")
    return values


def ts_const_string_values(text: str, name: str) -> set[str]:
    match = re.search(rf"export\s+const\s+{re.escape(name)}\s*=\s*\{{(.*?)\}}\s*as\s+const;", text, re.S)
    if not match:
        fail(f"TypeScript const {name} not found")
    values = set(re.findall(r"\b[A-Z][A-Z0-9_]*\s*:\s*'([^']+)'", match.group(1)))
    if not values:
        fail(f"TypeScript const {name} has no string values")
    return values


def go_struct_json_fields(text: str, name: str) -> set[str]:
    match = re.search(rf"type\s+{re.escape(name)}\s+struct\s*\{{(.*?)\n\}}", text, re.S)
    if not match:
        fail(f"Go struct {name} not found")
    fields = set(re.findall(r'`json:"([^",]+)(?:,[^"]*)?"`', match.group(1)))
    if not fields:
        fail(f"Go struct {name} has no JSON fields")
    return fields


def go_typed_const_values(text: str, type_name: str) -> set[str]:
    values = set(re.findall(rf"\b[A-Za-z0-9_]+\s+{re.escape(type_name)}\s*=\s*\"([^\"]+)\"", text))
    if not values:
        fail(f"Go typed constants for {type_name} not found")
    return values


def schema_props(schema: dict, key: str) -> tuple[set[str], set[str]]:
    node = schema["properties"][key]
    if key in {"services", "channels", "controlEffectSinks", "artifacts"}:
        node = node["items"]
    return set(node.get("properties", {})), set(node.get("required", []))


def main() -> None:
    for path in (GO_GAME, GO_CHANNEL, GO_SERVICE, TS_GAME, TS_PROTOCOL, TS_INDEX, HOST_SCHEMA, CHANNEL_SCHEMA, SERVICE_SCHEMA, FEATURE_SCHEMA, VALIDATOR_FIXTURE, GO_VALIDATOR_TEST, TS_VALIDATOR_TEST):
        if not path.is_file():
            fail(f"required contract file missing: {path.relative_to(ROOT)}")

    go_game = GO_GAME.read_text(encoding="utf-8")
    go_channel = GO_CHANNEL.read_text(encoding="utf-8")
    go_service = GO_SERVICE.read_text(encoding="utf-8")
    ts_game = TS_GAME.read_text(encoding="utf-8")
    ts_protocol = TS_PROTOCOL.read_text(encoding="utf-8")
    host_schema = load_json(HOST_SCHEMA)
    channel_schema = load_json(CHANNEL_SCHEMA)
    service_schema = load_json(SERVICE_SCHEMA)
    feature_schema = load_json(FEATURE_SCHEMA)

    component_map = {
        "services": "PluginServiceSpec",
        "channels": "PluginChannelSpec",
        "controlEffectSinks": "PluginControlEffectSinkSpec",
        "artifacts": "PluginArtifact",
        "network": "PluginNetworkPolicy",
    }
    for schema_key, contract_name in component_map.items():
        expected_fields, expected_required = schema_props(host_schema, schema_key)
        ts_fields, ts_required = ts_interface(ts_game, contract_name)
        go_fields = go_struct_json_fields(go_game, contract_name)
        assert_same(f"{contract_name} schema<->TypeScript fields", expected_fields, ts_fields)
        assert_same(f"{contract_name} schema<->Go fields", expected_fields, go_fields)
        assert_same(f"{contract_name} schema<->TypeScript required fields", expected_required, ts_required)

    host_fields = set(host_schema.get("properties", {}))
    host_required = set(host_schema.get("required", []))
    base_fields, base_required = ts_interface(ts_game, "PluginHostSpecBase")
    ts_host_fields = base_fields | {"runtimeModuleId", "services"}
    assert_same("PluginHostSpec schema<->TypeScript fields", host_fields, ts_host_fields)
    assert_same("PluginHostSpec schema<->Go fields", host_fields, go_struct_json_fields(go_game, "PluginHostSpec"))
    assert_same("PluginHostSpec schema<->TypeScript base required fields", host_required, base_required)

    # The host-spec channels and standalone channel schema describe the same wire
    # channel except serviceId, which is host-spec-only routing metadata.
    host_channel_props, host_channel_required = schema_props(host_schema, "channels")
    standalone_channel_props = set(channel_schema.get("properties", {}))
    standalone_channel_required = set(channel_schema.get("required", []))
    assert_same(
        "channel.schema<->plugin-host channel fields",
        standalone_channel_props,
        host_channel_props - {"serviceId"},
    )
    assert_same("channel.schema<->plugin-host channel required fields", standalone_channel_required, host_channel_required)

    ts_descriptor_fields, ts_descriptor_required = ts_interface(ts_protocol, "ChannelDescriptor")
    assert_same("channel.schema<->TypeScript ChannelDescriptor fields", standalone_channel_props, ts_descriptor_fields)
    assert_same("channel.schema<->TypeScript ChannelDescriptor required fields", standalone_channel_required, ts_descriptor_required)
    assert_same("channel.schema<->Go ChannelDescriptor fields", standalone_channel_props, go_struct_json_fields(go_channel, "ChannelDescriptor"))

    # service.schema.json intentionally describes the runtime/handshake ServiceDescriptor,
    # not plugin-host-spec.services[] (PluginServiceSpec). Keep this distinction
    # explicit so third-party SDKs cannot accidentally merge the two contracts.
    standalone_service_props = set(service_schema.get("properties", {}))
    standalone_service_required = set(service_schema.get("required", []))
    ts_service_fields, ts_service_required = ts_interface(ts_protocol, "ServiceDescriptor")
    assert_same("service.schema<->TypeScript ServiceDescriptor fields", standalone_service_props, ts_service_fields)
    assert_same("service.schema<->TypeScript ServiceDescriptor required fields", standalone_service_required, ts_service_required)
    assert_same("service.schema<->Go ServiceDescriptor fields", standalone_service_props, go_struct_json_fields(go_service, "ServiceDescriptor"))

    service_props = host_schema["properties"]["services"]["items"]["properties"]
    channel_props = host_schema["properties"]["channels"]["items"]["properties"]
    artifact_props = host_schema["properties"]["artifacts"]["items"]["properties"]
    network_props = host_schema["properties"]["network"]["properties"]

    assert_same(
        "service kind schema<->TypeScript enum",
        set(service_props["kind"]["enum"]),
        ts_property_literals(ts_game, "PluginServiceSpec", "kind"),
    )
    standalone_service_kind = set(service_schema["properties"]["kind"]["enum"])
    assert_same("service.schema kind<->TypeScript enum", standalone_service_kind, ts_type_literals(ts_protocol, "ServiceKind"))
    assert_same("service.schema kind<->Go enum", standalone_service_kind, go_typed_const_values(go_service, "ServiceKind"))
    channel_kind = set(channel_props["kind"]["enum"])
    assert_same("channel kind schema<->TypeScript enum", channel_kind, ts_property_literals(ts_game, "PluginChannelSpec", "kind"))
    assert_same("channel kind schema<->Go enum", channel_kind, go_typed_const_values(go_channel, "ChannelKind"))

    channel_direction = set(channel_props["direction"]["enum"])
    assert_same("channel direction schema<->TypeScript enum", channel_direction, ts_type_literals(ts_protocol, "ChannelDirection"))
    assert_same("channel direction schema<->Go enum", channel_direction, go_typed_const_values(go_channel, "ChannelDirection"))

    frequency_hint = set(channel_props["frequencyHint"]["enum"])
    assert_same("frequencyHint schema<->TypeScript enum", frequency_hint, ts_type_literals(ts_protocol, "FrequencyHint"))
    assert_same("frequencyHint schema<->Go enum", frequency_hint, go_typed_const_values(go_channel, "FrequencyHint"))

    assert_same(
        "artifact type schema<->TypeScript enum",
        set(artifact_props["type"]["enum"]),
        ts_property_literals(ts_game, "PluginArtifact", "type"),
    )
    assert_same(
        "network mode schema<->TypeScript enum",
        set(network_props["mode"]["enum"]),
        ts_property_literals(ts_game, "PluginNetworkPolicy", "mode"),
    )

    feature_values = set(feature_schema.get("enum", []))
    assert_same("host feature schema<->TypeScript enum", feature_values, ts_const_string_values(ts_protocol, "HostFeature"))
    assert_same("host feature schema<->Go enum", feature_values, go_typed_const_values(go_game, "HostFeature"))

    # Guard the non-empty runtime/services union that JSON Schema cannot express
    # with only its top-level required array.
    union_marker = "services: [PluginServiceSpec, ...PluginServiceSpec[]]"
    if union_marker not in ts_game:
        fail("PluginHostSpec must keep the non-empty services tuple branch")

    fixture = load_json(VALIDATOR_FIXTURE) if VALIDATOR_FIXTURE.suffix == ".json" else None
    if not isinstance(fixture, list) or not fixture:
        fail("shared PluginHostSpec validator fixture must be a non-empty JSON array")
    names = [case.get("name") for case in fixture if isinstance(case, dict)]
    if len(names) != len(set(names)) or any(not name for name in names):
        fail("shared PluginHostSpec validator fixture case names must be unique and non-empty")
    fixture_name = VALIDATOR_FIXTURE.name
    if fixture_name not in GO_VALIDATOR_TEST.read_text(encoding="utf-8"):
        fail("Go validator conformance test is not wired to the shared PluginHostSpec fixture")
    if fixture_name not in TS_VALIDATOR_TEST.read_text(encoding="utf-8"):
        fail("TypeScript validator conformance test is not wired to the shared PluginHostSpec fixture")
    ts_index = TS_INDEX.read_text(encoding="utf-8")
    if "export * from './validation';" not in ts_index:
        fail("TypeScript SDK root must export the public validation API")

    print("Game Plugin Go/JSON-Schema/TypeScript contract parity verified")


if __name__ == "__main__":
    main()
