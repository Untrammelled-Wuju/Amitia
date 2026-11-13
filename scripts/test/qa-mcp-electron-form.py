import argparse
import json
import pathlib
import subprocess
import urllib.parse
import urllib.request
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("electron_log", type=pathlib.Path)
    args = parser.parse_args()
    root = args.data_dir.resolve()
    log = args.electron_log.resolve()
    assert (root / ".qa-pet-isolated").is_file(), root
    assert f"数据目录: {root}" in log.read_text(), log
    base = json.loads((root / "run/endpoint.json").read_text())["httpBaseURL"]
    assert urllib.parse.urlsplit(base).hostname == "127.0.0.1", base
    token = (root / "security/local-token").read_text().strip()
    project = pathlib.Path(__file__).resolve().parents[2]
    node = project / "AmitiaData/node/bin/node"
    driver = project / "scripts/test/qa-electron-dom.mjs"
    name = "QA-MCP-页面-" + uuid.uuid4().hex[:8]

    def call(method, path):
        request = urllib.request.Request(base + path, method=method, headers={"X-Amitia-Local-Token": token})
        with urllib.request.urlopen(request, timeout=10) as response:
            return json.load(response)["data"]

    def ui(*steps):
        subprocess.run([node, driver, log, *steps], cwd=project, check=True, timeout=45)

    def field(selector, value):
        return "fill=" + json.dumps([selector, str(value)], ensure_ascii=False)

    def matching_servers():
        return [server for server in call("GET", "/api/mcp/servers") if server["name"] == name]

    def on_row(button):
        return "clickwithin=" + json.dumps([".el-table__row", name, button], ensure_ascii=False)

    submit = 'clickwithin=[".el-dialog","stdio 会启动本地进程","添加服务"]'
    try:
        ui("route=/extensions", "click=MCP 服务", "click=添加服务", "click=本地 stdio",
           field('input[placeholder="例如 github"]', name), "click=取消", "absent=添加 MCP 服务",
           "click=添加服务", "click=本地 stdio", submit, "expect=请输入名称",
           field('input[placeholder="例如 github"]', name), submit, "expect=请输入命令")
        assert not matching_servers(), "未通过表单验证的配置被保存"
        ui(field('input[placeholder="已安装的可执行程序"]', node),
           field('textarea[placeholder="每行一个参数"]', project / "scripts/test/qa-mcp-stdio-server.mjs"),
           submit, "expect=服务已添加", "expect=" + name, "absent=添加 MCP 服务")
        server, = matching_servers()
        path = "/api/mcp/servers/" + server["id"]
        assert server["status"] == "disconnected" and server["enabled"] == 0, server
        ui(on_row("连接"), "expect=连接成功", on_row("详情"), "click=工具 0", "expect=Echo a QA message",
           "route=/chat", "route=/extensions/mcp", "expect=" + name, on_row("断开"), "expect=已断开")
        assert call("GET", path)["status"] == "disconnected"
        assert [tool["remoteName"] for tool in call("GET", path + "/tools")] == ["echo"]
    finally:
        for server in matching_servers():
            assert call("DELETE", "/api/mcp/servers/" + server["id"])["deleted"]


if __name__ == "__main__":
    main()
