import argparse
import json
import pathlib
import sqlite3
import subprocess
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("electron_log", type=pathlib.Path)
    parser.add_argument("package", type=pathlib.Path)
    parser.add_argument("pet_id")
    parser.add_argument("--fail-uninstall-refresh", action="store_true")
    args = parser.parse_args()
    root = args.data_dir.resolve()
    assert (root / ".qa-pet-isolated").is_file(), root
    assert args.electron_log.resolve().is_relative_to(root.parents[1]), args.electron_log
    assert f"数据目录: {root}" in args.electron_log.read_text(), args.electron_log
    project = pathlib.Path(__file__).resolve().parents[2]
    node = project / "AmitiaData/node/bin/node"
    driver = project / "scripts/test/qa-electron-dom.mjs"

    def ui(*steps):
        subprocess.run([node, driver, args.electron_log, *steps], cwd=project, check=True)

    with sqlite3.connect(root / "app.db") as db:
        def installations():
            return db.execute(
                "SELECT id,status,lifecycle_state,is_active FROM desktop_pet_installations WHERE pet_id=? ORDER BY created_at,id",
                (args.pet_id,),
            ).fetchall()

        initial = installations()
        assert all(row[1:] == ("uninstalled", "uninstalled", 0) for row in initial), initial
        ui("route=/creative-workshop/pet/installations", "expect=com.amitia.builtin.desktop-pet",
           "absent=QA Desktop Pet Plugin")
        seen = {row[0] for row in initial}
        for cycle in range(2):
            upload = "file=" + json.dumps([".package-drop-zone input[type=file]", str(args.package.resolve())])
            if cycle == 0:
                ui("click=安装插件", upload, "expect=我已查看签名、权限和风险信息，并确认继续安装。",
                   "click=取消", "absent=确认安装", "click=安装插件", "expect=选择或拖入 .petx 桌宠插件",
                   "absent=我已查看签名、权限和风险信息，并确认继续安装。")
            else:
                ui("click=安装插件")
            ui(upload, "expect=QA Desktop Pet Plugin",
               "click=我已查看签名、权限和风险信息，并确认继续安装。", "clicktwice=确认安装",
               "expect=桌宠插件已安装", "absent=确认安装", 'clickwithin=[".plugin-item","QA Desktop Pet Plugin","启用"]',
               "expect=桌宠插件已启用")
            rows = installations()
            created = [row for row in rows if row[0] not in seen]
            assert len(created) == 1 and created[0][2] != "uninstalled", rows
            installation_id = created[0][0]
            seen.add(installation_id)
            assert len(rows) == len(seen), rows
            print(f"cycle {cycle + 1}: installed {installation_id}", flush=True)

            fault = args.fail_uninstall_refresh and cycle == 0
            fault_setup = ("failsecondget=/api/desktop-pets/installations",) if fault else ()
            fault_checks = ("expect=刷新桌宠卸载状态失败", "expect=ERR_NETWORK") if fault else ()
            ui('clickwithin=[".plugin-item","QA Desktop Pet Plugin","卸载"]', *fault_setup, "clicktwice=确认卸载",
               "expect=桌宠插件已卸载", "absent=确认卸载桌宠插件", "expect=com.amitia.builtin.desktop-pet",
               "absent=QA Desktop Pet Plugin", *fault_checks)
            deadline = time.monotonic() + 45
            while time.monotonic() < deadline:
                rows = installations()
                if {row[0] for row in rows} == seen and all(row[1:] == ("uninstalled", "uninstalled", 0) for row in rows):
                    break
                time.sleep(1)
            else:
                raise AssertionError(rows)
            operation = db.execute(
                "SELECT id,status,stage FROM desktop_pet_installation_operations WHERE installation_id=? AND operation_type='uninstall'",
                (installation_id,),
            ).fetchone()
            assert operation and operation[1:] == ("completed", "completed"), operation
            trash, = db.execute(
                "SELECT count(*) FROM desktop_pet_installation_trash_entries WHERE operation_id=? AND installation_id=? AND status='retained'",
                (operation[0], installation_id),
            ).fetchone()
            assert trash == 1, (operation, trash)
            ui('expectall=[".installation-card.uninstalled .card-title .el-tag","已卸载"]',
               "absent=刷新桌宠卸载状态失败", "reload",
               'expectall=[".installation-card.uninstalled .card-title .el-tag","已卸载"]')
            print(f"cycle {cycle + 1}: uninstalled {installation_id}, retained trash and prior records", flush=True)


if __name__ == "__main__":
    main()
