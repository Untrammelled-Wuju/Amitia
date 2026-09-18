import socket
import time
from dataclasses import dataclass
from typing import Dict, Optional
from urllib import error, request


@dataclass
class HttpProbeResult:
    ok: bool
    status: int = 0
    body: str = ""
    headers: Dict[str, str] = None
    error: Optional[str] = None
    durationMs: int = 0

    def __post_init__(self):
        if self.headers is None:
            self.headers = {}

    def to_safe_dict(self) -> dict:
        return {
            "ok": self.ok,
            "status": self.status,
            "error": self.error,
            "durationMs": self.durationMs,
            "bodyLength": len(self.body) if self.body else 0,
        }


def http_get_json(url: str, timeout: float = 5.0, headers: Optional[Dict[str, str]] = None) -> HttpProbeResult:
    start = int(time.time() * 1000)
    req = request.Request(url, method="GET", headers=headers or {})
    try:
        with request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read().decode("utf-8", errors="replace")
            response_headers = {k: v for k, v in resp.getheaders()}
            return HttpProbeResult(
                ok=True,
                status=resp.status,
                body=raw,
                headers=response_headers,
                durationMs=int(time.time() * 1000) - start,
            )
    except error.HTTPError as e:
        raw = ""
        try:
            raw = e.read().decode("utf-8", errors="replace")
        except Exception:
            pass
        return HttpProbeResult(
            ok=False,
            status=e.code,
            body=raw,
            error=f"HTTP {e.code}",
            durationMs=int(time.time() * 1000) - start,
        )
    except (error.URLError, socket.timeout, OSError) as e:
        return HttpProbeResult(
            ok=False,
            error=str(e),
            durationMs=int(time.time() * 1000) - start,
        )


def http_get_text(url: str, timeout: float = 5.0) -> HttpProbeResult:
    return http_get_json(url, timeout=timeout)


def wait_for_http_ready(
    url: str,
    expected_status: int = 200,
    timeout: float = 30.0,
    interval: float = 0.5,
) -> HttpProbeResult:
    deadline = time.time() + timeout
    last_result = HttpProbeResult(ok=False, error="timeout")
    while time.time() < deadline:
        last_result = http_get_json(url)
        if last_result.ok and last_result.status == expected_status:
            return last_result
        time.sleep(interval)
    return last_result


def check_port_in_use(host: str, port: int, timeout: float = 1.0) -> bool:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(timeout)
    try:
        sock.connect((host, port))
        return True
    except OSError:
        return False
    finally:
        sock.close()
