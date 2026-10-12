import argparse
import concurrent.futures
import hashlib
import http.server
import json
import os
import pathlib
import sqlite3
import subprocess
import threading
import time
import urllib.error
import urllib.request
import uuid


ROOT = pathlib.Path(__file__).resolve().parents[2]
OUTPUT = ROOT / "artifacts/cloud-mode-test"
CLOUD = "http://127.0.0.2:18899"
AGENT = "http://127.0.0.3:18899"
OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))
TLS_CASES = [
    "TestActualTLSProjectAPIPreservesSourceHistoryAndRejectsChangedOwnership",
    "TestOwnedAuthorityAcrossRealTLSMeshReconnectAndPermissionChange",
    "TestThreeRealCoreBusinessSwitchPreservesOwnershipAndExpiresAdministrator",
    "TestThreeRealCoreBusinessSwitchKeepsDeviceOwnedConversationWhenCoordinationOff",
    "TestThreeRealCoreSixMemoryLayersSummaryAndContinuityKeepTheirOwners",
    "TestRealPairingPlatformMatrixRejectsDuplicateReverseAndSelfScans",
    "TestActualTLSMemoryManagementPreservesOwnerAndRejectsOldIntent",
    "TestActualTLSMemoryCandidatesUseHistoricalSourceAndCurrentCoreOwner",
    "TestActualTLSAttachmentsStayAtTheirSelectedOwner",
    "TestOwnedQuoteRealTLSUsesSourceHistoryAfterCoordinationWithoutMirroringIt",
    "TestSourceTaskEventUsesActualTLSNativeRPCAndSourceDurableOutbox",
    "TestMeshSummaryHTTPRequiresActorCurrentPageAndOwnerAcknowledgement",
    "TestMeshConversationSearchOwnerRoleAndCursorIsolation",
    "TestDeviceManagementRealTLSRequiresOriginalAuthorityAcrossPolicyABA",
    "TestCloudApplicationRegistrationWithDefaultDesktopPet",
    "TestDeviceDesktopPetRegistrationHonorsConfiguration",
]


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


def api(path, method="GET", body=None, authenticated=True, accepted=(200,), endpoint=CLOUD, token=None, extra_headers=None):
    headers = {"Accept": "application/json"}
    if authenticated:
        headers["X-Amitia-Local-Token"] = TOKEN if token is None else token
    if body is not None:
        headers["Content-Type"] = "application/json"
    headers.update(extra_headers or {})
    request = urllib.request.Request(endpoint + path, data=encode(body) if body is not None else None, headers=headers, method=method)
    try:
        response = OPENER.open(request, timeout=240)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        status = response.status
        raw = response.read()
    result = json.loads(raw) if raw else {}
    if status not in accepted and not (status == 200 and isinstance(result, dict) and result.get("code") in accepted and result.get("data") is None):
        raise RuntimeError(f"{method} {path}: HTTP {status}: {result.get('message', result.get('msg', '')) if isinstance(result, dict) else ''}")
    if accepted == (200,) and isinstance(result, dict) and result.get("code", 200) not in (0, 200):
        raise RuntimeError(f"{method} {path}: business code {result['code']}: {result.get('msg', '')}")
    return result


def agent_api(path, method="GET", body=None, authenticated=True, accepted=(200,), extra_headers=None):
    token = (OUTPUT / "agent/data/security/local-token").read_text().strip()
    return api(path, method, body, authenticated, accepted, AGENT, token, extra_headers)


def provider_api(path, method="GET", body=None, extra_headers=None):
    return agent_api("/internal/device-mesh/provider" + path, method, body, extra_headers=extra_headers)


def change_mode(coordinated, role_id):
    state = provider_api("/api/device-mesh/v1/coordination/me")
    policy = state["policy"]
    headers = {"X-Amitia-Expected-Core-ID": state["coreId"], "X-Amitia-Expected-Configuration-Policy": f"{policy['providerEpoch']}:{policy['modeRevision']}:{policy['permissionRevision']}"}
    return provider_api("/api/device-mesh/v1/coordination/me", "PUT", {"coordinated": coordinated, "selectedRole": role_id, "expectedRevision": policy["modeRevision"]}, headers)["policy"]


def live_owned(role_id, expected_owner, coordinated):
    change_mode(coordinated, role_id)
    marker = "OWNED-" + uuid.uuid4().hex[:12]
    body = {"characterId": role_id, "message": f"请记住本轮标识 {marker}，仅回复它。", "requestId": uuid.uuid4().hex}
    first = provider_api("/api/device-mesh/v1/business/messages", "POST", body)["data"]
    if not first["saved"] or first["memoryStatus"] != "saved" or marker not in first["reply"] or first["executionScope"]["resourceOwnerId"] != expected_owner:
        raise RuntimeError("owned live generation or data owner did not match")
    duplicate = provider_api("/api/device-mesh/v1/business/messages", "POST", body)["data"]
    if duplicate["conversationId"] != first["conversationId"] or duplicate["reply"] != first["reply"]:
        raise RuntimeError("owned duplicate request changed its result")
    second = provider_api("/api/device-mesh/v1/business/messages", "POST", {"characterId": role_id, "conversationId": first["conversationId"], "message": "刚才标识是什么？仅回复它。", "requestId": uuid.uuid4().hex})["data"]
    if not second["saved"] or second["memoryStatus"] != "saved" or marker not in second["reply"] or second["executionScope"]["resourceOwnerId"] != expected_owner:
        raise RuntimeError("owned live continuation lost context or changed its owner")
    return {"coordinated": coordinated, "owner": expected_owner, "conversationId": first["conversationId"], "memoryStatuses": [first["memoryStatus"], second["memoryStatus"]], "replyHash": hashlib.sha256(second["reply"].encode()).hexdigest()}


def round_tls(number):
    env = dict(os.environ, AMITIA_TEST_NODE=str(ROOT / "desktop/resources/core/node/node.exe"), GIN_MODE="release")

    def run_group(group):
        cases = TLS_CASES[group::2]
        log_path = OUTPUT / f"round-{number:02d}-tls-{group + 1}.log"
        with log_path.open("w", encoding="utf-8") as log:
            process = subprocess.run([str(OUTPUT / "cloud-regression.test.exe"), "-test.v", "-test.run=^(" + "|".join(cases) + ")$", "-test.count=1", "-test.timeout=15m"], cwd=ROOT / "backend/cmd/server", env=env, stdout=log, stderr=subprocess.STDOUT)
        if process.returncode:
            raise RuntimeError(f"round {number} TLS suite failed; see {log_path.name}")
        return log_path.name

    with concurrent.futures.ThreadPoolExecutor(max_workers=2) as executor:
        return list(executor.map(run_group, range(2)))


def pair_agent(core_id):
    identity = agent_api("/internal/device-mesh/identity")
    offer = api("/api/device-mesh/v1/pairing/offers", "POST", {"endpoint": CLOUD})
    body = {"deviceId": identity["deviceId"], "runtimeId": identity["runtimeId"], "platform": identity["platform"], "label": "真实云端验收设备", "offerToken": offer["offerToken"]}
    signed = agent_api("/internal/device-mesh/identity/sign-claim", "POST", {"coreId": core_id, "body": body})
    claim = api("/api/public/device-mesh/v1/pairing/claim", "POST", dict(body, proof=signed), authenticated=False, accepted=(200, 202))
    if claim.get("pending"):
        requests = api("/api/device-mesh/v1/pairing/approvals")["requests"]
        approval = next(item for item in requests if item["requestId"] == claim["requestId"])
        api("/api/device-mesh/v1/pairing/approvals/" + claim["requestId"], "PUT", {"allow": True, "expectedRevision": approval["revision"]})
        signed = agent_api("/internal/device-mesh/identity/sign-claim", "POST", {"coreId": core_id, "body": body})
        claim = api("/api/public/device-mesh/v1/pairing/claim", "POST", dict(body, proof=signed), authenticated=False)
    agent_api("/internal/device-mesh/bootstrap", "POST", {"cloudBaseUrl": CLOUD, "bootstrapTicket": claim["ticket"], "coreId": core_id})
    return identity["deviceId"]


class Gateway(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        raw = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        target = MODEL["base_url"].rstrip("/") + self.path.removeprefix("/v1")
        request = urllib.request.Request(target, data=raw, headers={"Authorization": "Bearer " + MODEL["api_key"], "Content-Type": "application/json"}, method="POST")
        started = time.monotonic()
        status = 502
        try:
            with urllib.request.urlopen(request, timeout=180) as response:
                status = response.status
                self.send_response(status)
                self.send_header("Content-Type", response.headers.get("Content-Type", "application/json"))
                self.end_headers()
                while chunk := response.read(4096):
                    self.wfile.write(chunk)
                    self.wfile.flush()
        except Exception as error:
            upstream_status = getattr(error, "code", None)
            print(f"PROVIDER_GATEWAY_FAIL type={type(error).__name__} upstreamStatus={upstream_status}", flush=True)
            if status == 502:
                status = upstream_status if isinstance(upstream_status, int) else 502
                self.send_response(status)
                self.end_headers()
                self.wfile.write(b'{"error":{"message":"live provider gateway failed"}}')
        finally:
            CALLS.append({"status": status, "requestHash": hashlib.sha256(raw).hexdigest(), "elapsedSeconds": round(time.monotonic() - started, 3)})


def round_live(number, role_id):
    marker = "CLOUD-" + uuid.uuid4().hex[:12]
    start_calls = len(CALLS)
    api("/api/public/health", authenticated=False)
    agent_api("/readyz", authenticated=False)
    agent_api("/api/workspaces")
    agent_api("/api/workspaces", authenticated=False, accepted=(401, 403))
    for path in ("/api/model/configs", "/api/chats/conversations", "/api/memories", "/api/world-book", "/api/runtime/health"):
        api(path, authenticated=False, accepted=(401, 403))
    first_body = {"characterId": role_id, "message": f"本轮测试标识是 {marker}。请记住它，仅回复该标识。", "requestId": uuid.uuid4().hex, "channel": "web"}
    first = api("/api/chat", "POST", first_body)["data"]
    conversation_id = first["conversationId"]
    reply = first.get("message", {}).get("content", "")
    if marker not in reply:
        raise RuntimeError("first live generation did not return the unique marker")
    before = api(f"/api/chats/conversations/{conversation_id}/messages")["data"]
    duplicate = api("/api/chat", "POST", first_body, accepted=(200, 400, 409, 429))
    after = api(f"/api/chats/conversations/{conversation_id}/messages")["data"]
    if before != after:
        raise RuntimeError("duplicate request changed saved message history")
    second = api("/api/chat", "POST", {"characterId": role_id, "conversationId": conversation_id, "message": "上一轮测试标识是什么？仅回复那个标识。", "requestId": uuid.uuid4().hex, "channel": "web"})["data"]
    if marker not in second.get("message", {}).get("content", ""):
        raise RuntimeError("live continuation lost the previous turn marker")
    saved = api(f"/api/chats/conversations/{conversation_id}/messages")["data"]
    saved_hash = hashlib.sha256(encode(saved)).hexdigest()
    summary = api(f"/api/chats/conversations/{conversation_id}/summary/generate", "POST", {})["data"]
    if not summary:
        raise RuntimeError("live summary was empty")
    summary_read = api(f"/api/chats/conversations/{conversation_id}/summary")["data"]
    if not summary_read:
        raise RuntimeError("saved summary was unavailable")
    memory = api("/api/memories", "POST", {"characterId": role_id, "key": marker, "value": "云端真实文本验收", "importance": 5})["data"]
    memory_id = memory["id"]
    api(f"/api/memories/{memory_id}", "PUT", {"value": "云端记忆更新验收"})
    memory_list = api("/api/memories?characterId=" + role_id)["data"]
    if "云端记忆更新验收" not in json.dumps(memory_list, ensure_ascii=False):
        raise RuntimeError("memory update was not visible")
    api(f"/api/memories/{memory_id}", "DELETE")
    world = api("/api/world-book", "POST", {"matchType": "keyword", "matchPattern": marker, "injectContent": "云端世界书验收", "enabled": True})["data"]
    api(f"/api/world-book/{world['id']}", "PUT", {"injectContent": "云端世界书更新验收"})
    api(f"/api/world-book/{world['id']}", "DELETE")
    api(f"/api/chats/conversations/{conversation_id}/summary", "DELETE")
    api(f"/api/chats/conversations/{conversation_id}", "DELETE")
    api("/api/runtime/check-db-integrity", "POST", {})
    calls = CALLS[start_calls:]
    if len(calls) < 3 or any(call["status"] != 200 for call in calls):
        raise RuntimeError("live provider calls were incomplete or unsuccessful")
    return {"round": number, "marker": marker, "conversationId": conversation_id, "historyHash": saved_hash, "summaryHash": hashlib.sha256(encode(summary_read)).hexdigest(), "providerCalls": calls, "duplicateBusinessCode": duplicate.get("code", 200)}


def main():
    global TOKEN, MODEL, CALLS
    parser = argparse.ArgumentParser()
    parser.add_argument("--rounds", type=int, default=10)
    parser.add_argument("--skip-tls", action="store_true")
    args = parser.parse_args()
    OUTPUT.mkdir(parents=True, exist_ok=True)
    if not args.skip_tls:
        with (OUTPUT / "tls-compile.log").open("w", encoding="utf-8") as log:
            compiled = subprocess.run([r"C:\Code\Go\bin\go.exe", "test", "-c", "-ldflags=-w", "-o", str(OUTPUT / "cloud-regression.test.exe"), "./cmd/server"], cwd=ROOT / "backend", stdout=log, stderr=subprocess.STDOUT)
        if compiled.returncode:
            raise RuntimeError("TLS test compilation failed; see tls-compile.log")
    TOKEN = (OUTPUT / "cloud/data/security/local-token").read_text().strip()
    CALLS = []
    with sqlite3.connect(f"file:{ROOT / 'AmitiaData/app.db'}?mode=ro", uri=True) as database:
        database.row_factory = sqlite3.Row
        MODEL = dict(database.execute("SELECT * FROM model_configs WHERE is_active=1 LIMIT 1").fetchone())
    info = api("/api/public/core/info", authenticated=False)["data"]
    if info["runtimeProfile"] != "cloud-core":
        raise RuntimeError("test endpoint is not Cloud Core")
    gateway = http.server.ThreadingHTTPServer(("127.0.0.4", 18899), Gateway)
    threading.Thread(target=gateway.serve_forever, daemon=True).start()
    report = {"scope": "Production Cloud Core and Device Agent live text generation in coordinated and device-owned modes; TLS ownership tests use deterministic model fixtures", "excluded": ["ASR", "TTS", "vision", "image generation", "physical client UI", "vector and graph providers"], "core": info, "rounds": [], "consecutivePassed": 0, "complete": False}
    report_path = OUTPUT / "live-text-report.json"
    report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
    role_id = None
    agent_role_id = None
    model_id = None
    try:
        model = api("/api/model/configs", "POST", {"name": "cloud-live-test-gateway", "apiType": MODEL["api_type"], "protocol": MODEL["protocol"], "baseUrl": "http://127.0.0.4:18899/v1", "apiKey": "memory-only-gateway", "modelName": MODEL["model_name"], "maxTokens": MODEL["max_tokens"], "timeoutSeconds": MODEL["timeout_seconds"], "isActive": True})["data"]
        model_id = model["id"]
        role = api("/api/characters", "POST", {"name": "云端文本验收", "basePrompt": "你是精确的文本测试助手。保持上下文，按用户要求简短回复。", "isDefault": True})["data"]
        role_id = role["id"]
        identity = agent_api("/internal/device-mesh/identity")
        status = agent_api("/internal/device-mesh/status")
        if status.get("coreId") != info["spaceId"]:
            pair_agent(info["spaceId"])
        agent_role = agent_api("/api/characters", "POST", {"name": "设备文本验收", "basePrompt": "你是精确的文本测试助手。保持上下文，按用户要求简短回复。", "isDefault": True})["data"]
        agent_role_id = agent_role["id"]
        for number in range(1, args.rounds + 1):
            print(f"ROUND {number} START", flush=True)
            started = time.monotonic()
            start_calls = len(CALLS)
            result = round_live(number, role_id)
            result["ownedLive"] = [live_owned(agent_role_id, identity["deviceId"], False), live_owned(role_id, info["spaceId"], True)]
            result["providerCalls"] = CALLS[start_calls:]
            if len(result["providerCalls"]) < 7 or any(call["status"] != 200 for call in result["providerCalls"]):
                raise RuntimeError("owned live provider calls were incomplete or unsuccessful")
            if not args.skip_tls:
                result["tlsLogs"] = round_tls(number)
                result["tlsCases"] = TLS_CASES
            result["elapsedSeconds"] = round(time.monotonic() - started, 3)
            report["rounds"].append(result)
            report["consecutivePassed"] = number
            report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")
            print(f"ROUND {number} PASS in {result['elapsedSeconds']}s", flush=True)
        report["complete"] = True
    except Exception as error:
        report["failure"] = str(error)
        report["allProviderCalls"] = CALLS
        report["consecutivePassed"] = 0
        print("FAIL: " + str(error), flush=True)
        raise
    finally:
        if agent_role_id:
            change_mode(False, "")
            agent_api(f"/api/characters/{agent_role_id}", "DELETE")
        if role_id:
            api(f"/api/characters/{role_id}", "DELETE")
        if model_id:
            api(f"/api/model/configs/{model_id}", "DELETE")
        gateway.shutdown()
        report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding="utf-8")


if __name__ == "__main__":
    main()
