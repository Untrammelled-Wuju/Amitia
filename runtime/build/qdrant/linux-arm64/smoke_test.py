import argparse
import hashlib
import json
import os
import pathlib
import platform
import shutil
import signal
import socket
import subprocess
import sys
import tempfile
import time

API_READYZ = "/readyz"
API_COLLECTIONS = "/collections"
API_POINTS = "/collections/{name}/points"
API_QUERY = "/collections/{name}/points/query"
DEFAULT_MAX_STARTUP_SECONDS = 60
DEFAULT_MAX_READY_SECONDS = 60
DEFAULT_MAX_STOP_SECONDS = 10
DEFAULT_MAX_IDLE_RSS_MB = 0
TEST_VECTOR_DIM = 4
TEST_POINTS = 3
TEST_COLLECTION_PREFIX = "amitia_smoke_"


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        while True:
            chunk = f.read(1048576)
            if not chunk:
                break
            h.update(chunk)
    return h.hexdigest()


def free_port():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


def check_env_errors():
    errors = []
    if platform.system().lower() != "linux":
        errors.append(f"非 Linux: {platform.system()}")
    if platform.machine().lower() not in ("aarch64", "arm64"):
        errors.append(f"非 arm64: {platform.machine()}")
    return errors


def build_config(port, storage_dir, snapshots_dir):
    config = {
        "service": {
            "host": "127.0.0.1",
            "http_port": port,
            "grpc_port": port + 1,
            "enable_cors": False,
        },
        "storage": {
            "storage_path": storage_dir,
            "snapshots_path": snapshots_dir,
            "on_disk_payload": True,
        },
        "log_level": "WARN",
        "telemetry_disabled": True,
    }
    return config


def wait_for_ready(proc, base_url, timeout_seconds):
    deadline = time.monotonic() + timeout_seconds
    started = time.monotonic()
    ready = False
    while time.monotonic() < deadline:
        if proc.poll() is not None:
            raise RuntimeError(f"进程已退出，code={proc.returncode}")
        try:
            import urllib.request
            req = urllib.request.Request(base_url + API_READYZ, method="GET")
            with urllib.request.urlopen(req, timeout=2) as resp:
                if resp.status == 200:
                    ready = True
                    break
        except Exception:
            pass
        time.sleep(0.5)
    elapsed = time.monotonic() - started
    if not ready:
        raise RuntimeError(f"Ready 超时 ({timeout_seconds}s)")
    return elapsed


def http_json(method, url, body=None):
    import urllib.request
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    if method in ("POST", "PUT", "PATCH"):
        req.add_header("Accept", "application/json")
    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            raw = resp.read()
            if not raw:
                return resp.status, {}
            return resp.status, json.loads(raw)
    except urllib.error.HTTPError as e:
        raw = e.read() if e.fp else b""
        if not raw:
            return e.code, {}
        return e.code, json.loads(raw)


def stop_process(proc, timeout_seconds):
    if proc.poll() is not None:
        return proc.returncode, 0.0, False
    started = time.monotonic()
    proc.terminate()
    try:
        proc.wait(timeout=timeout_seconds)
        elapsed = time.monotonic() - started
        return proc.returncode, elapsed, False
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait(timeout=5)
        elapsed = time.monotonic() - started
        return proc.returncode, elapsed, True


def collect_metrics(proc):
    metrics = {}
    try:
        rss_path = f"/proc/{proc.pid}/statm"
        with open(rss_path, "r") as f:
            parts = f.read().split()
            if parts:
                rss_pages = int(parts[1])
                page_size = os.sysconf("SC_PAGE_SIZE")
                metrics["rss_bytes"] = rss_pages * page_size
    except Exception:
        pass
    return metrics


def run_smoke(args):
    env_errors = check_env_errors()
    if env_errors:
        for e in env_errors:
            print(f"[错误] {e}", file=sys.stderr)
        return 1

    dist = pathlib.Path(args.distribution)
    bin_path = dist / "bin" / "qdrant"
    if not bin_path.exists():
        print(f"[错误] 二进制不存在: {bin_path}", file=sys.stderr)
        return 1

    max_startup = args.max_startup_seconds or DEFAULT_MAX_STARTUP_SECONDS
    max_ready = args.max_ready_seconds or DEFAULT_MAX_READY_SECONDS
    max_stop = args.max_stop_seconds or DEFAULT_MAX_STOP_SECONDS
    max_idle_rss_mb = args.max_idle_rss_mb or DEFAULT_MAX_IDLE_RSS_MB

    work = pathlib.Path(tempfile.mkdtemp(prefix=".qdrant-smoke-"))
    config_dir = work / "config"
    storage_dir = work / "storage"
    snapshots_dir = work / "snapshots"
    logs_dir = work / "logs"
    for d in (config_dir, storage_dir, snapshots_dir, logs_dir):
        d.mkdir(parents=True, exist_ok=True)

    port = free_port()
    config = build_config(port, str(storage_dir), str(snapshots_dir))
    config_path = work / "config/config.json"
    with open(str(config_path), "w", encoding="utf-8", newline="") as f:
        json.dump(config, f, indent=2)

    base_url = f"http://127.0.0.1:{port}"
    collection_name = TEST_COLLECTION_PREFIX + os.urandom(4).hex()
    report = {
        "collection": collection_name,
        "storage_dir": str(storage_dir),
        "port": port,
        "startup_seconds": None,
        "ready_seconds": None,
        "stop_seconds": None,
        "restart_ready_seconds": None,
        "errors": [],
        "metrics": {},
    }

    page_size = os.sysconf("SC_PAGE_SIZE")
    report["page_size"] = page_size
    print(f"[环境] arch={platform.machine()} page_size={page_size}")

    proc = None
    try:
        cmd = [str(bin_path), "--config-path", str(config_path)]
        env = os.environ.copy()
        env["Q__LOG__LEVEL"] = "WARN"
        print(f"[启动] qdrant :{port}")
        proc = subprocess.Popen(cmd, env=env, cwd=str(work))

        start_time = time.monotonic()
        ready_time = wait_for_ready(proc, base_url, max_ready)
        report["ready_seconds"] = round(ready_time, 3)
        print(f"[Ready] {ready_time:.2f}s")

        create_url = base_url + API_COLLECTIONS + "/" + collection_name
        create_body = {
            "vectors": {
                "size": TEST_VECTOR_DIM,
                "distance": "Cosine",
            },
            "on_disk": False,
        }
        status, resp = http_json("PUT", create_url, create_body)
        if status not in (200,):
            report["errors"].append(f"创建集合失败: {status} {resp}")
            print(f"[错误] 创建集合失败: {status} {resp}", file=sys.stderr)
            return 1
        print(f"[集合] {collection_name} 创建成功")

        points_url = base_url + API_POINTS.format(name=collection_name)
        points = []
        for i in range(TEST_POINTS):
            points.append({
                "id": i + 1,
                "vector": [float(i + 1) / 10] * TEST_VECTOR_DIM,
                "payload": {"index": i, "label": f"point_{i}"},
            })
        upsert_body = {"points": points}
        status, resp = http_json("PUT", points_url, upsert_body)
        if status != 200:
            report["errors"].append(f"Upsert 失败: {status} {resp}")
            print(f"[错误] Upsert 失败: {status}")
            return 1
        print(f"[写入] Upsert {len(points)} 点成功")

        query_url = base_url + API_QUERY.format(name=collection_name)
        query_body = {
            "query": points[0]["vector"],
            "limit": 2,
            "with_payload": True,
            "with_vector": False,
        }
        status, resp = http_json("POST", query_url, query_body)
        if status != 200:
            report["errors"].append(f"Query 失败: {status} {resp}")
            return 1
        result_points = resp.get("result", {}).get("points", [])
        if result_points and result_points[0].get("id") != 1:
            report["errors"].append(f"Query 首条非期望最近邻: {result_points}")
            return 1
        print(f"[查询] Query 返回 {len(result_points)} 点")

        metrics = collect_metrics(proc)
        report["metrics"] = metrics
        if metrics.get("rss_bytes"):
            print(f"[RSS] {metrics['rss_bytes'] / 1048576:.1f} MB")

        returncode, stop_elapsed, killed = stop_process(proc, max_stop)
        report["stop_seconds"] = round(stop_elapsed, 3)
        report["stop_killed"] = killed
        print(f"[停止] returncode={returncode} time={stop_elapsed:.2f}s killed={killed}")

        if args.test_crash_recovery:
            print("[跳过] 正常流程已通过，跳过 crash 恢复测试分支")
        else:
            print("[重启] 使用相同 storage 重启")
            proc = subprocess.Popen(cmd, env=env, cwd=str(work))
            restart_start = time.monotonic()
            restart_ready = wait_for_ready(proc, base_url, max_ready)
            report["restart_ready_seconds"] = round(restart_ready, 3)
            print(f"[重启Ready] {restart_ready:.2f}s")

            collections_url = base_url + API_COLLECTIONS
            status, resp = http_json("GET", collections_url)
            if status != 200:
                report["errors"].append(f"重启后获取集合失败: {status}")
                return 1
            names = [c["name"] for c in resp.get("result", {}).get("collections", [])]
            if collection_name not in names:
                report["errors"].append(f"重启后集合不存在: {names}")
                return 1
            print(f"[持久化] 集合存在: {collection_name}")

            q_status, q_resp = http_json("POST", query_url, query_body)
            if q_status != 200:
                report["errors"].append(f"重启后 Query 失败: {q_status}")
                return 1
            q_points = q_resp.get("result", {}).get("points", [])
            if q_points and q_points[0].get("id") != 1:
                report["errors"].append(f"重启后 Query 结果异常: {q_points}")
                return 1
            print(f"[持久化] Query 返回 {len(q_points)} 点")

            returncode, stop_elapsed, killed = stop_process(proc, max_stop)
            print(f"[二次停止] returncode={returncode} time={stop_elapsed:.2f}s")

        if report["errors"]:
            return 1
        print("[Smoke Test] 通过")
        return 0

    except Exception as e:
        report["errors"].append(str(e))
        print(f"[错误] {e}", file=sys.stderr)
        return 1

    finally:
        if proc and proc.poll() is None:
            try:
                proc.kill()
                proc.wait(timeout=5)
            except Exception:
                pass
        if work.exists():
            shutil.rmtree(str(work), ignore_errors=True)
            print("[清理] 临时目录已删除")

        if args.report:
            rpath = pathlib.Path(args.report)
            rpath.parent.mkdir(parents=True, exist_ok=True)
            safe_report = {
                k: v for k, v in report.items()
                if k not in ("storage_dir",)
            }
            with open(str(rpath), "w", encoding="utf-8", newline="") as f:
                json.dump(safe_report, f, indent=2, ensure_ascii=False)
                f.write("\n")


def parse_args():
    parser = argparse.ArgumentParser(description="Qdrant Linux ARM64 Runtime Smoke Test")
    parser.add_argument("--distribution", required=True, help="分发目录路径")
    parser.add_argument("--report", help="报告输出路径")
    parser.add_argument("--test-crash-recovery", action="store_true",
                        help="测试崩溃恢复")
    parser.add_argument("--max-startup-seconds", type=int, default=DEFAULT_MAX_STARTUP_SECONDS)
    parser.add_argument("--max-ready-seconds", type=int, default=DEFAULT_MAX_READY_SECONDS)
    parser.add_argument("--max-stop-seconds", type=int, default=DEFAULT_MAX_STOP_SECONDS)
    parser.add_argument("--max-idle-rss-mb", type=int, default=DEFAULT_MAX_IDLE_RSS_MB)
    return parser.parse_args()


def main():
    args = parse_args()
    try:
        sys.exit(run_smoke(args))
    except Exception as e:
        print(f"[错误] {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
