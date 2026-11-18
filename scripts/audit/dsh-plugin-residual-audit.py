#!/usr/bin/env python3
import argparse
import json
import os
import sqlite3
import sys

TERMINAL_OPERATION_STATUSES = {"completed", "failed", "cancelled"}
TERMINAL_STEP_STATUSES = {"completed", "failed", "skipped"}


def open_readonly(path):
    uri = "file:%s?mode=ro" % path.replace("?", "%3f").replace("#", "%23")
    connection = sqlite3.connect(uri, uri=True)
    connection.row_factory = sqlite3.Row
    return connection


def table_exists(connection, name):
    row = connection.execute(
        "SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", (name,)
    ).fetchone()
    return row[0] == 1


def section(title):
    print("")
    print("== %s ==" % title)


def query(connection, sql, params=()):
    return connection.execute(sql, params).fetchall()


def audit_operations(connection, findings):
    section("package operations")
    rows = query(
        connection,
        "SELECT operation_id, extension_id, from_version, target_version, status, current_step,"
        " error_code, recovery_required, stable_generation, target_generation, started_at, updated_at"
        " FROM extension_package_operations ORDER BY started_at ASC",
    )
    counts = {}
    for row in rows:
        counts[row["status"]] = counts.get(row["status"], 0) + 1
    for status in sorted(counts):
        print("status %-18s %d" % (status, counts[status]))
    incomplete = [row for row in rows if row["status"] not in TERMINAL_OPERATION_STATUSES]
    print("incomplete_operations=%d" % len(incomplete))
    for row in incomplete:
        print(
            "  INCOMPLETE %s ext=%s %s->%s status=%s step=%s recovery_required=%d error=%s updated=%s"
            % (
                row["operation_id"],
                row["extension_id"],
                row["from_version"],
                row["target_version"],
                row["status"],
                row["current_step"],
                row["recovery_required"],
                row["error_code"],
                row["updated_at"],
            )
        )
        findings.append("incomplete_operation:%s" % row["operation_id"])
    return len(incomplete)


def audit_steps(connection, findings):
    section("package operation steps")
    if not table_exists(connection, "extension_package_operation_steps"):
        print("table extension_package_operation_steps missing")
        findings.append("missing_table:extension_package_operation_steps")
        return 0
    rows = query(
        connection,
        "SELECT status, count(*) AS n FROM extension_package_operation_steps GROUP BY status",
    )
    non_terminal = 0
    for row in rows:
        print("status %-18s %d" % (row["status"], row["n"]))
        if row["status"] not in TERMINAL_STEP_STATUSES:
            non_terminal += row["n"]
    print("non_terminal_steps=%d" % non_terminal)
    if non_terminal:
        for row in query(
            connection,
            "SELECT operation_id, step_name, status FROM extension_package_operation_steps"
            " WHERE status NOT IN ('completed','failed','skipped')",
        ):
            print(
                "  STEP %s %s %s"
                % (row["operation_id"], row["step_name"], row["status"])
            )
            findings.append("non_terminal_step:%s" % row["operation_id"])
    return non_terminal


def audit_leases(connection, findings):
    section("package operation leases")
    if not table_exists(connection, "extension_package_operation_leases"):
        print("table extension_package_operation_leases missing")
        return 0
    rows = query(connection, "SELECT * FROM extension_package_operation_leases")
    print("lease_rows=%d" % len(rows))
    for row in rows:
        payload = {key: row[key] for key in row.keys()}
        print("  LEASE %s" % json.dumps(payload, ensure_ascii=False, sort_keys=True))
        findings.append("stale_lease:%s" % payload.get("operation_id", ""))
    return len(rows)


def audit_update_journal(connection):
    section("update package lifecycle journal")
    for table in (
        "extension_lifecycle_journal",
        "extension_rollback_plans",
        "extension_rollback_steps",
        "extension_side_effect_assessments",
    ):
        if not table_exists(connection, table):
            print("%-36s absent" % table)
            continue
        total = query(connection, "SELECT count(*) AS n FROM %s" % table)[0]["n"]
        print("%-36s rows=%d" % (table, total))
    if table_exists(connection, "extension_lifecycle_journal"):
        rows = query(
            connection,
            "SELECT operation_id, count(*) AS n FROM extension_lifecycle_journal"
            " WHERE status = 'started' GROUP BY operation_id",
        )
        print("started_journal_operations=%d" % len(rows))
        for row in rows:
            print("  STARTED %s entries=%d" % (row["operation_id"], row["n"]))


def audit_consistency(connection):
    section("consistency findings")
    if not table_exists(connection, "package_consistency_findings"):
        print("table package_consistency_findings absent")
        return
    rows = query(
        connection,
        "SELECT finding_id, metric, count, resource_ids_json, recommended_action, created_at, resolved_at"
        " FROM package_consistency_findings ORDER BY created_at ASC",
    )
    unresolved = [row for row in rows if not row["resolved_at"]]
    print("findings=%d unresolved=%d" % (len(rows), len(unresolved)))
    for row in unresolved:
        print(
            "  FINDING %s metric=%s count=%d resources=%s action=%s"
            % (
                row["finding_id"],
                row["metric"],
                row["count"],
                row["resource_ids_json"],
                row["recommended_action"],
            )
        )


def installation_rows(connection):
    if not table_exists(connection, "extension_installations"):
        return []
    return query(
        connection,
        "SELECT extension_id, version, installed, enabled, generation, current_generation_id, recovery_state"
        " FROM extension_installations WHERE installed = 1 ORDER BY extension_id",
    )


def audit_installations(connection, root, findings):
    section("installation pointers")
    base = os.path.join(root, "installations")
    for row in installation_rows(connection):
        directory = os.path.join(base, row["extension_id"].replace("/", "__"))
        current_path = os.path.join(directory, "current.json")
        previous_path = os.path.join(directory, "previous.json")
        staging_path = os.path.join(directory, "staging")
        generations_path = os.path.join(directory, "generations")
        current = {}
        previous = {}
        if os.path.exists(current_path):
            with open(current_path, "r", encoding="utf-8") as handle:
                current = json.load(handle)
        if os.path.exists(previous_path):
            with open(previous_path, "r", encoding="utf-8") as handle:
                previous = json.load(handle)
        staging = sorted(os.listdir(staging_path)) if os.path.isdir(staging_path) else []
        generations = sorted(os.listdir(generations_path)) if os.path.isdir(generations_path) else []
        match = "match"
        if row["current_generation_id"] and current.get("generationID"):
            if row["current_generation_id"] != current.get("generationID"):
                match = "MISMATCH"
                findings.append("pointer_mismatch:%s" % row["extension_id"])
        print(
            "%-38s db_version=%-9s db_generation=%-46s pointer=%s %s"
            % (
                row["extension_id"],
                row["version"],
                row["current_generation_id"] or "-",
                current.get("generationID", "-"),
                match,
            )
        )
        print(
            "    pointer_version=%-9s previous_version=%-9s generations=%d staging=%s"
            % (
                current.get("version", "-"),
                previous.get("version", "-"),
                len(generations),
                staging if staging else "[]",
            )
        )
        if staging:
            findings.append("staging_residue:%s" % row["extension_id"])
        for name in ("current", "previous"):
            value = current if name == "current" else previous
            identifier = value.get("generationID")
            if identifier and identifier not in generations:
                print("    MISSING_TREE %s %s" % (name, identifier))
                findings.append("missing_generation_tree:%s" % identifier)


def audit_rollback_points(connection, root, findings):
    section("rollback points")
    if not table_exists(connection, "extension_package_rollback_points"):
        print("table extension_package_rollback_points absent")
        return
    rows = query(
        connection,
        "SELECT extension_id, source_version, source_generation_id, retention_state, retention_until, created_at"
        " FROM extension_package_rollback_points ORDER BY extension_id, created_at",
    )
    per_extension = {}
    for row in rows:
        per_extension.setdefault(row["extension_id"], []).append(row)
    for extension_id, points in sorted(per_extension.items()):
        generations_path = os.path.join(
            root, "installations", extension_id.replace("/", "__"), "generations"
        )
        available = set(os.listdir(generations_path)) if os.path.isdir(generations_path) else set()
        dangling = [point for point in points if point["source_generation_id"] not in available]
        active = [point for point in points if point["retention_state"] == "active"]
        versions = [point["source_version"] for point in points]
        print(
            "%-38s points=%d active=%d versions=%s..%s dangling=%d"
            % (
                extension_id,
                len(points),
                len(active),
                versions[0] if versions else "-",
                versions[-1] if versions else "-",
                len(dangling),
            )
        )
        for point in dangling:
            print(
                "  DANGLING %s version=%s generation=%s"
                % (point["created_at"], point["source_version"], point["source_generation_id"])
            )
            findings.append("dangling_rollback_point:%s" % point["source_generation_id"])


def audit_dsh_plugin_data(root):
    section("dsh container plugin data")
    plugin_data = os.path.join(root, "plugin-data", "com.amitia__dsh-compatible-host")
    if not os.path.isdir(plugin_data):
        print("plugin_data absent at %s" % plugin_data)
        return
    tree = os.path.join(plugin_data, "dsh-tree")
    data = os.path.join(plugin_data, "dsh-data")
    print("dsh-tree present=%s" % os.path.isdir(tree))
    print("dsh-data present=%s" % os.path.isdir(data))
    manifest_path = os.path.join(tree, "package.json")
    if os.path.exists(manifest_path):
        with open(manifest_path, "r", encoding="utf-8") as handle:
            manifest = json.load(handle)
        print("dsh-tree dependencies=%s" % json.dumps(manifest.get("dependencies") or {}, sort_keys=True))
    lock_path = os.path.join(tree, "package-lock.json")
    print("dsh-tree lockfile present=%s" % os.path.exists(lock_path))
    modules = os.path.join(tree, "node_modules")
    if os.path.isdir(modules):
        names = sorted(os.listdir(modules))
        print("dsh-tree node_modules top-level=%d" % len(names))
        for name in names[:10]:
            print("  module %s" % name)


def audit_plugin_data_include(repo_root):
    section("dsh manifest plugin data include")
    manifest_path = os.path.join(
        repo_root, "plugins-code", "dsh-compatible-host", "amitia-extension.json"
    )
    if not os.path.exists(manifest_path):
        print("manifest absent at %s" % manifest_path)
        return
    with open(manifest_path, "r", encoding="utf-8") as handle:
        manifest = json.load(handle)
    include = (
        manifest.get("data", {}).get("pluginData", {}).get("include")
        if isinstance(manifest.get("data"), dict)
        else None
    )
    print("manifest=%s" % manifest_path)
    print("data.pluginData.include=%s" % json.dumps(include, ensure_ascii=False))
    print("platforms=%s" % json.dumps(manifest.get("compatibility", {}).get("platforms")))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", default=os.path.join("AmitiaData", "extensions-v2"))
    parser.add_argument("--repo-root", default=".")
    arguments = parser.parse_args()
    kernel_db = os.path.join(arguments.root, "kernel.db")
    print("extensions_root=%s" % os.path.abspath(arguments.root))
    print("kernel_db=%s" % os.path.abspath(kernel_db))
    if not os.path.exists(kernel_db):
        print("kernel.db missing")
        return 0
    findings = []
    connection = open_readonly(kernel_db)
    try:
        audit_operations(connection, findings)
        audit_steps(connection, findings)
        audit_leases(connection, findings)
        audit_update_journal(connection)
        audit_consistency(connection)
        audit_installations(connection, arguments.root, findings)
        audit_rollback_points(connection, arguments.root, findings)
    finally:
        connection.close()
    audit_dsh_plugin_data(arguments.root)
    audit_plugin_data_include(os.path.abspath(arguments.repo_root))
    section("summary")
    print("findings=%d" % len(findings))
    for finding in findings:
        print("  %s" % finding)
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main())
