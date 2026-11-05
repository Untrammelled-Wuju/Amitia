import csv
import json
import pathlib
import re

ROOT = pathlib.Path(__file__).resolve().parents[2]
OUT = ROOT / "artifacts/qa-20261105-depth"
DIMENSIONS = ["正常", "错误输入", "空值边界", "重复", "快速连续", "中断恢复", "导航刷新", "进程重启", "前后端一致", "功能关联", "并发", "依赖故障"]


def main():
    OUT.mkdir(parents=True, exist_ok=True)
    entries = []
    sources = [
        ("Web/Electron", ROOT / "front/src/router", "*.ts", r'path:\s*[\"\x27]([^\"\x27]+)'),
        ("Core", ROOT / "backend", "*.go", r'\.((?:GET|POST|PUT|PATCH|DELETE|OPTIONS|HEAD|Group))\(\s*"([^"]*)"'),
        ("Android/iOS", ROOT / "mobile_app/lib", "*.dart", r'(?:class\s+(\w+(?:Page|Screen))\b|path:\s*[\"\x27]([^\"\x27]+))'),
    ]
    for platform, directory, glob, pattern in sources:
        for path in sorted(directory.rglob(glob)):
            if path.name.endswith("_test.go"):
                continue
            for number, line in enumerate(path.read_text(errors="replace").splitlines(), 1):
                for match in re.finditer(pattern, line):
                    entries.append({"platform": platform, "source": str(path.relative_to(ROOT)), "line": number, "entry": " ".join(value for value in match.groups() if value), "classification": "源码候选入口，非动态路由确认"})
    plugins = []
    for path in sorted((ROOT / "plugins-code").rglob("amitia-extension.json")):
        manifest = json.loads(path.read_text())
        plugins.append({"source": str(path.relative_to(ROOT)), "id": manifest.get("id"), "name": manifest.get("name"), "keys": sorted(manifest)})
    models = []
    for path in sorted((ROOT / "backend").rglob("*.go")):
        if path.name.endswith("_test.go"):
            continue
        text = path.read_text(errors="replace")
        tables = re.findall(r'func\s*\([^)]*\)\s*TableName\(\)\s*string\s*\{\s*return\s*"([^"]+)"', text)
        if tables:
            models.append({"source": str(path.relative_to(ROOT)), "tables": tables})
    inventory = {"entries": entries, "plugins": plugins, "gorm_tables": models, "sql_sources": [str(path.relative_to(ROOT)) for path in (ROOT / "backend").rglob("*.sql")], "limits": "静态候选地图：嵌套分组需人工拼接；动态注册与条件启用不代表已验证。管理端不执行修改或业务操作。"}
    (OUT / "source-map.json").write_text(json.dumps(inventory, ensure_ascii=False, indent=2))
    with (OUT / "entry-coverage.csv").open("w", newline="") as file:
        writer = csv.writer(file)
        writer.writerow(["平台", "源码", "行", "候选入口", "证据等级", *DIMENSIONS])
        for entry in entries:
            writer.writerow([entry["platform"], entry["source"], entry["line"], entry["entry"], "仅静态提取", *["未验证"] * len(DIMENSIONS)])
    print(json.dumps({"entries": len(entries), "plugins": len(plugins), "table_sources": len(models)}, ensure_ascii=False))


if __name__ == "__main__":
    main()
