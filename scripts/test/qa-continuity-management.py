import argparse
import concurrent.futures
import json
import pathlib
import urllib.error
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    args = parser.parse_args()
    data = args.data_dir.resolve()
    assert "artifacts" in data.parts and any(part.startswith("qa-") for part in data.parts)
    endpoint = json.loads((data / "run/endpoint.json").read_text())
    base = endpoint["httpBaseURL"]
    assert urllib.parse.urlsplit(base).hostname == "127.0.0.1", endpoint
    token = (data / "security/local-token").read_text().strip()
    prefix = "/api/device-mesh/v1/business"

    def call(path, body=None, **query):
        suffix = "?" + urllib.parse.urlencode(query) if query else ""
        request = urllib.request.Request(
            base + prefix + path + suffix,
            data=None if body is None else json.dumps(body).encode(),
            headers={"X-Amitia-Local-Token": token, "Content-Type": "application/json"},
        )
        try:
            response = urllib.request.urlopen(request, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            return response.status, json.load(response)

    def success(result):
        status, response = result
        assert status == 200, result
        return response.get("data", response)

    roles = success(call("/roles"))["roles"]
    assert roles, "隔离数据需要一个可用角色"
    role_id = roles[0]["id"]
    query = success(call("/data", kind="continuity", characterId=role_id, memoryView="all"))
    scope = dict(query["executionScope"])
    scope.pop("memoryViewAll", None)
    assert scope["coordinated"] and scope["resourceOwnerId"] == scope["coreId"] != scope["targetDeviceId"], scope
    request_id = str(uuid.uuid4())
    title = "QA-CONTINUITY-" + request_id
    create = {
        "requestId": request_id, "action": "create", "title": title,
        "characterId": role_id, "expectedRevision": 0,
        "expectedCoreId": scope["coreId"], "expectedOwnerId": scope["resourceOwnerId"],
        "expectedModeRevision": scope["modeRevision"], "expectedExecutionScope": scope,
    }
    foreign = dict(create, expectedExecutionScope=dict(scope, authorizationRealm="foreign-qa"))
    assert call("/continuity", foreign)[0] == 409
    wrong_role = dict(create, expectedExecutionScope=dict(scope, roleId="foreign-role"))
    assert call("/continuity", wrong_role)[0] == 409
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        results = list(pool.map(lambda _: call("/continuity", create), range(4)))
    print(json.dumps({"createStatuses": [status for status, _ in results]}, ensure_ascii=False))
    created = [success(result) for result in results]
    first = created[0]["document"]
    thread = first["thread"]
    assert thread["title"] == title and thread["revision"] == 1 and first["ownerId"] == scope["coreId"], first
    assert all(result["document"]["thread"]["id"] == thread["id"] for result in created), created
    current = success(call("/continuity", id=thread["id"], characterId=role_id, memoryView="role:" + role_id))
    assert current["thread"]["id"] == thread["id"] and current["thread"]["revision"] == 1, current
    complete = dict(create, id=thread["id"], requestId=str(uuid.uuid4()), action="complete",
                    expectedRevision=1, expectedExecutionScope=current["managementExecutionScope"])
    finished = success(call("/continuity", complete))["document"]["thread"]
    assert finished["status"] == "completed" and finished["revision"] == 2, finished
    stale = dict(complete, requestId=str(uuid.uuid4()), action="pause")
    assert call("/continuity", stale)[0] == 409
    final = success(call("/continuity", id=thread["id"], characterId=role_id, memoryView="role:" + role_id))
    assert final["thread"]["status"] == "completed" and final["thread"]["revision"] == 2, final
    print("四次相同请求只生成一条 Core 归属事项；错误归属和过期修订均被拒绝")


if __name__ == "__main__":
    main()
