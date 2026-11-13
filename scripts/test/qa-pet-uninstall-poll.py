import argparse
import pathlib
import sqlite3
import subprocess


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("data_dir", type=pathlib.Path)
    parser.add_argument("electron_log", type=pathlib.Path)
    args = parser.parse_args()
    root = args.data_dir.resolve()
    log = args.electron_log.resolve()
    assert (root / ".qa-pet-isolated").is_file(), root
    assert f"数据目录: {root}" in log.read_text(), log
    project = pathlib.Path(__file__).resolve().parents[2]
    node = project / "AmitiaData/node/bin/node"
    driver = project / "scripts/test/qa-electron-dom.mjs"

    with sqlite3.connect(root / "app.db", timeout=10) as db:
        row = db.execute("""
            SELECT id, runtime_sync_state FROM desktop_pet_installations
            WHERE pet_id = 'qa-pet-uninstall-regression' AND status = 'uninstalled'
            AND lifecycle_state = 'uninstalled' AND is_active = 0
            ORDER BY created_at DESC LIMIT 1
        """).fetchone()
        assert row, "需要一次已完成的隔离桌宠卸载"
        key, sync_state = row
        with db:
            changed = db.execute("""
                UPDATE desktop_pet_installations
                SET status = 'uninstalling', lifecycle_state = 'uninstalling', runtime_sync_state = 'pending'
                WHERE id = ? AND status = 'uninstalled' AND lifecycle_state = 'uninstalled'
            """, (key,)).rowcount
            assert changed == 1
        try:
            subprocess.run([node, driver, log, "uninstallpoll"], cwd=project, check=True, timeout=110)
        finally:
            with db:
                changed = db.execute("""
                    UPDATE desktop_pet_installations
                    SET status = 'uninstalled', lifecycle_state = 'uninstalled', runtime_sync_state = ?
                    WHERE id = ? AND status = 'uninstalling' AND lifecycle_state = 'uninstalling'
                    AND runtime_sync_state = 'pending'
                """, (sync_state, key)).rowcount
                assert changed == 1, (key, changed)
                assert db.execute("PRAGMA quick_check").fetchone()[0] == "ok"


if __name__ == "__main__":
    main()
