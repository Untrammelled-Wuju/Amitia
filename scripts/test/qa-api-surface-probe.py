#!/usr/bin/env python3
"""Probe a running Amitia Core HTTP surface with adversarial requests.

The script never invents results: it issues real requests against an isolated
Core instance, records the raw status codes and bodies, and prints every
response whose status is 5xx or whose transport failed. Exit code is non-zero
when the script finds a server-side failure, so it can gate a QA run.

GET routes receive adversarial path values plus hostile pagination queries.
Mutation routes receive malformed and empty JSON bodies. Only the isolated QA
instance is ever contacted; never point this at a user's live runtime.

Usage:
  qa-api-surface-probe.py <base-url> <token> <routes.tsv> [GET,POST,...]
"""

import concurrent.futures
import pathlib
import sys
import urllib.error
import urllib.parse
import urllib.request

PROBE_VALUES = ("probe", "..")
QUERY_VARIANTS = (
    "",
    "?page=9223372036854775807&pageSize=2",
    "?limit=-1&offset=9223372036854775807&cursor=%00&id=%FF",
)
MUTATION_BODIES = (b"{}", b'{"id":"probe","name":"probe","action":"probe"}', b"")
SKIP_PREFIXES = (
    "/api/workbench/terminal/stream",
    "/api/device-mesh/v1/business/chat-runtime/events",
    "/api/runtime/tasks/status",
    "/internal/",
    "/api/wasm/",
    "/api/extension-center/",
    "/api/extensions/ui/",
    "/api/dev-console/",
    "/api/artifacts/v1/previews",
    "/api/storage/restore",
    "/api/storage/import",
)


def expand_path(template, value):
    parts = template.split("/")
    out = []
    for part in parts:
        if part.startswith(":") or part.startswith("*"):
            out.append(urllib.parse.quote(value, safe=""))
        else:
            out.append(part)
    return "/".join(out)


def main():
    base = sys.argv[1].rstrip("/")
    token = sys.argv[2]
    routes = pathlib.Path(sys.argv[3])
    methods = sys.argv[4].split(",") if len(sys.argv) > 4 else ["GET"]
    templates = []
    for line in routes.read_text().splitlines():
        fields = line.split("\t")
        if len(fields) < 2:
            continue
        if fields[0] not in methods:
            continue
        if any(fields[1].startswith(prefix) for prefix in SKIP_PREFIXES):
            continue
        templates.append((fields[0], fields[1]))

    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def call(job):
        method, path, body = job
        request = urllib.request.Request(
            base + path,
            data=body,
            headers={
                "X-Amitia-Local-Token": token,
                "Accept": "application/json",
                "Content-Type": "application/json",
            },
            method=method,
        )
        try:
            with client.open(request, timeout=15) as response:
                return response.status, response.read(512)
        except urllib.error.HTTPError as error:
            return error.code, error.read(512)
        except Exception as error:  # noqa: BLE001 - record transport failures verbatim
            return 0, repr(error).encode()

    jobs = []
    for method, template in templates:
        for value in PROBE_VALUES:
            path = expand_path(template, value)
            if method == "GET":
                for query in QUERY_VARIANTS:
                    jobs.append(("GET", path + query, None))
            else:
                for body in MUTATION_BODIES:
                    jobs.append((method, path, body))
    deduped = list(dict.fromkeys(jobs))
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as pool:
        results = list(pool.map(call, deduped))
    failures = []
    for (method, path, _), (status, body) in zip(deduped, results):
        if status >= 500 or status == 0:
            failures.append((method, path, status, body[:300]))
    print(f"probed {len(deduped)} requests across {','.join(methods)}, {len(failures)} server failures")
    for method, path, status, body in failures:
        text = body.decode("utf-8", "replace") if isinstance(body, bytes) else str(body)
        print(f"{status} {method} {path} :: {text}")
    return 1 if failures else 0


if __name__ == "__main__":
    sys.exit(main())
