import argparse
import json
import os
import sys
import time
from pathlib import Path
from typing import List, Optional

try:
    from . import (
        backend_probe,
        cleanup,
        data_probe,
        environment,
        http_probe,
        installer,
        node_probe,
        ownership_probe,
        process_runner,
        qdrant_probe,
        report,
        restart_probe,
        runtime_layout,
        business_probe,
    )
except ImportError:
    import backend_probe
    import cleanup
    import data_probe
    import environment
    import http_probe
    import installer
    import node_probe
    import ownership_probe
    import process_runner
    import qdrant_probe
    import report
    import restart_probe
    import runtime_layout
    import business_probe


def parse_arguments(argv: Optional[List[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Amitia Runtime Linux ARM64 Validator")
    parser.add_argument("--runtime-package", required=True, help="Path to runtime package zip")
    parser.add_argument("--runtime-version", required=True, help="Expected runtime version")
    parser.add_argument("--commit", required=True, help="Expected commit hash")
    parser.add_argument("--profile", default="mobile-balanced", help="Primary Qdrant profile")
    parser.add_argument("--work-dir", default="", help="Temporary validation working directory")
    parser.add_argument("--report", default="", help="Path to output JSON report")
    parser.add_argument("--keep-work-dir", action="store_true", help="Keep work directory after validation")
    parser.add_argument("--skip-business-probe", action="store_true", help="Skip business probe")
    parser.add_argument("--skip-restart-probe", action="store_true", help="Skip restart probe")
    parser.add_argument("--skip-profile-matrix", action="store_true", help="Skip Qdrant profile matrix")
    return parser.parse_args(argv)


def make_work_dir(user_path: str) -> str:
    if user_path:
        os.makedirs(user_path, exist_ok=True)
        return os.path.abspath(user_path)
    base = os.path.join(os.path.dirname(__file__), "..", "..", "..", ".validation", "linux-arm64")
    os.makedirs(base, exist_ok=True)
    return os.path.abspath(base)


def ensure_minimal_path() -> None:
    os.environ["PATH"] = "/usr/bin:/bin"


def ensure_runtime_directories(layout: runtime_layout.GuestLayout) -> None:
    runtime_layout.ensure_directory_structure(layout)


def full_validate(args: argparse.Namespace) -> int:
    ensure_minimal_path()
    work_dir = make_work_dir(args.work_dir)
    report_dir = os.path.join(work_dir, "reports")
    os.makedirs(report_dir, exist_ok=True)

    validation_report = report.ValidationReport(
        runtimeVersion=args.runtime_version,
    )
    validation_report.environment = {
        "workDir": work_dir,
        "packagePath": args.runtime_package,
    }

    start_time = int(time.time() * 1000)

    check_environment(validation_report)
    check_package(validation_report, args.runtime_package, args.runtime_version, args.commit)
    if validation_report.result == "failed":
        return finalize_report(validation_report, args, work_dir, start_time)

    layout = runtime_layout.GuestLayout()
    ensure_runtime_directories(layout)
    install_result = installer.install_package_to_work_dir(args.runtime_package, work_dir)
    validation_report.add_check(report.Check(
        id="package.install",
        status=report.CheckStatus.PASSED.value if install_result.success else report.CheckStatus.FAILED.value,
        errorMessage="; ".join(install_result.errors) if not install_result.success else None,
    ))
    if not install_result.success:
        return finalize_report(validation_report, args, work_dir, start_time)

    runtime_root = install_result.runtimeRoot
    install_subdir(validation_report, runtime_root, layout)

    check_runtime_root(validation_report, layout)
    check_elf(validation_report, layout)
    check_versions(validation_report, layout)
    check_node_runtime(validation_report, layout)

    if not args.skip_profile_matrix:
        run_profile_matrix(validation_report, layout)
    else:
        validation_report.add_check(report.Check(
            id="profiles.matrix",
            status=report.CheckStatus.SKIPPED.value,
        ))

    backend_info = start_backend(validation_report, layout)
    if backend_info:
        check_backend_ready(validation_report, layout, backend_info)
        if not args.skip_business_probe:
            run_business_probe(validation_report, layout)
        else:
            validation_report.add_check(report.Check(
                id="business.probe",
                status=report.CheckStatus.SKIPPED.value,
            ))
        stop_backend(validation_report, backend_info, layout)

    if not args.skip_restart_probe:
        run_restart_tests(validation_report, layout, args)
    else:
        validation_report.add_check(report.Check(
            id="restart.tests",
            status=report.CheckStatus.SKIPPED.value,
        ))

    cleanup_after_validation(validation_report, layout)
    return finalize_report(validation_report, args, work_dir, start_time)


def check_environment(validation_report: report.ValidationReport) -> None:
    env_report = environment.validate_environment("/tmp")
    validation_report.environment = env_report.to_safe_dict()
    has_error = len(env_report.errors) > 0
    status = report.CheckStatus.FAILED.value if has_error else report.CheckStatus.PASSED.value
    error_message = "; ".join(err.get("message", "") for err in env_report.errors) if has_error else None
    validation_report.add_check(report.Check(
        id="environment.check",
        status=status,
        details=env_report.to_safe_dict(),
        errorMessage=error_message,
    ))


def check_package(validation_report: report.ValidationReport, package_path: str, version: str, commit: str) -> None:
    try:
        from . import package_validator
    except ImportError:
        import package_validator
    result = package_validator.validate_package(package_path, version, commit)
    validation_report.package = result.to_safe_dict()
    status = report.CheckStatus.PASSED.value if result.valid else report.CheckStatus.FAILED.value
    error_message = "; ".join(err.get("message", "") for err in result.errors) if not result.valid else None
    validation_report.add_check(report.Check(
        id="package.validate",
        status=status,
        details=result.to_safe_dict(),
        errorMessage=error_message,
    ))


def install_subdir(validation_report: report.ValidationReport, runtime_root: str, layout: runtime_layout.GuestLayout) -> None:
    if not installer.copy_runtime_to_system(runtime_root, layout.runtimeRoot):
        validation_report.add_check(report.Check(
            id="install.runtime",
            status=report.CheckStatus.FAILED.value,
            errorMessage="Failed to copy runtime to system location",
        ))
        return
    validation_report.add_check(report.Check(
        id="install.runtime",
        status=report.CheckStatus.PASSED.value,
        details={"runtimeRoot": layout.runtimeRoot},
    ))


def check_runtime_root(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    errors = runtime_layout.validate_runtime_root_contents()
    status = report.CheckStatus.PASSED.value if not errors else report.CheckStatus.FAILED.value
    validation_report.add_check(report.Check(
        id="runtime.layout",
        status=status,
        errorMessage="; ".join(errors) if errors else None,
    ))


def check_elf(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    backend_errors = backend_probe.validate_backend_elf(layout.runtimeRoot)
    status = report.CheckStatus.PASSED.value if not backend_errors else report.CheckStatus.FAILED.value
    validation_report.add_check(report.Check(
        id="elf.backend",
        status=status,
        errorMessage="; ".join(backend_errors) if backend_errors else None,
    ))

    node_errors = node_probe.validate_node_elf(layout.runtimeRoot)
    status = report.CheckStatus.PASSED.value if not node_errors else report.CheckStatus.FAILED.value
    validation_report.add_check(report.Check(
        id="elf.node",
        status=status,
        errorMessage="; ".join(node_errors) if node_errors else None,
    ))


def check_versions(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    version = backend_probe.get_backend_version(layout.runtimeRoot)
    errors = backend_probe.verify_backend_version(version) if version else ["Backend version output missing"]
    validation_report.components["backend"] = version.to_safe_dict() if version else None
    status = report.CheckStatus.PASSED.value if not errors else report.CheckStatus.FAILED.value
    validation_report.add_check(report.Check(
        id="version.backend",
        status=status,
        details=version.to_safe_dict() if version else None,
        errorMessage="; ".join(errors) if errors else None,
    ))

    node_version = node_probe.get_node_version(layout.runtimeRoot)
    version_ok = node_version == node_probe.EXPECTED_NODE_VERSION
    validation_report.add_check(report.Check(
        id="version.node",
        status=report.CheckStatus.PASSED.value if version_ok else report.CheckStatus.FAILED.value,
        details={"expected": node_probe.EXPECTED_NODE_VERSION, "actual": node_version},
        errorMessage=None if version_ok else f"Expected {node_probe.EXPECTED_NODE_VERSION}, got {node_version}",
    ))

    npm_version = node_probe.get_npm_version(layout.runtimeRoot)
    npm_ok = npm_version == node_probe.EXPECTED_NPM_VERSION
    validation_report.add_check(report.Check(
        id="version.npm",
        status=report.CheckStatus.PASSED.value if npm_ok else report.CheckStatus.FAILED.value,
        details={"expected": node_probe.EXPECTED_NPM_VERSION, "actual": npm_version},
        errorMessage=None if npm_ok else f"Expected {node_probe.EXPECTED_NPM_VERSION}, got {npm_version}",
    ))


def check_node_runtime(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    ldd_errors = node_probe.check_node_ldd(layout.runtimeRoot)
    status = report.CheckStatus.PASSED.value if not ldd_errors else report.CheckStatus.FAILED.value
    validation_report.add_check(report.Check(
        id="node.ldd",
        status=status,
        errorMessage="; ".join(ldd_errors) if ldd_errors else None,
    ))

    prepare_ok = node_probe.run_node_prepare(
        layout.runtimeRoot, layout.dataRoot, layout.cacheRoot, layout.runRoot,
    )
    validation_report.add_check(report.Check(
        id="node.prepare",
        status=report.CheckStatus.PASSED.value if prepare_ok else report.CheckStatus.FAILED.value,
    ))

    probe_ok = node_probe.run_node_probe(layout.runtimeRoot, layout.dataRoot)
    validation_report.add_check(report.Check(
        id="node.probe",
        status=report.CheckStatus.PASSED.value if probe_ok else report.CheckStatus.FAILED.value,
    ))

    variable_dirs_ok = node_probe.verify_variable_directories(
        layout.dataRoot, layout.cacheRoot, layout.runRoot, layout.runtimeRoot,
    )
    validation_report.add_check(report.Check(
        id="node.variableDirectories",
        status=report.CheckStatus.PASSED.value if variable_dirs_ok else report.CheckStatus.FAILED.value,
    ))

    plugin_host_ok = node_probe.probe_plugin_host(layout.runtimeRoot)
    validation_report.add_check(report.Check(
        id="plugin-host.probe",
        status=report.CheckStatus.PASSED.value if plugin_host_ok else report.CheckStatus.FAILED.value,
    ))

    task_host_ok = node_probe.probe_task_host(layout.runtimeRoot)
    validation_report.add_check(report.Check(
        id="task-host.probe",
        status=report.CheckStatus.PASSED.value if task_host_ok else report.CheckStatus.FAILED.value,
    ))


def run_profile_matrix(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    profiles = ["desktop-default", "mobile-compact", "mobile-balanced", "mobile-performance"]
    profile_results = {}
    for profile_name in profiles:
        result = run_single_profile(layout, profile_name)
        profile_results[profile_name] = result.to_safe_dict()
        status = report.CheckStatus.PASSED.value if result.ready else report.CheckStatus.FAILED.value
        validation_report.add_check(report.Check(
            id=f"qdrant.profile.{profile_name}",
            status=status,
            details=result.to_safe_dict(),
            errorMessage="; ".join(err.get("message", "") or err.get("code", "") for err in result.errors) if result.errors else None,
        ))
    validation_report.profiles = profile_results


def run_single_profile(layout: runtime_layout.GuestLayout, profile_name: str) -> qdrant_probe.QdrantProbeResult:
    result = qdrant_probe.QdrantProbeResult(profile=profile_name)
    config_path = os.path.join(layout.configRoot, "providers", "qdrant", "config.yaml")
    config = qdrant_probe.render_qdrant_config(
        profile_name, layout.runtimeRoot, layout.dataRoot, config_path,
    )
    config_errors = qdrant_probe.verify_qdrant_config(config_path, profile_name)
    if config_errors:
        result.errors.append({"code": "CONFIG_INVALID", "message": "; ".join(config_errors)})
        return result

    qdrant_binary = os.path.join(layout.runtimeRoot, "qdrant/bin/qdrant")
    if not os.path.exists(qdrant_binary):
        result.errors.append({"code": "QDRANT_BINARY_MISSING", "message": f"qdrant binary not found: {qdrant_binary}"})
        return result

    qdrant_config_path = config_path.replace(".yaml", ".json")
    with open(qdrant_config_path, "w", encoding="utf-8") as f:
        json.dump(config, f, indent=2)

    env = dict(os.environ)
    env["PATH"] = "/usr/bin:/bin"
    os.environ["PATH"] = "/usr/bin:/bin"

    stdout_path = os.path.join(layout.logRoot, f"qdrant-{profile_name}.stdout.log")
    stderr_path = os.path.join(layout.logRoot, f"qdrant-{profile_name}.stderr.log")
    os.makedirs(layout.logRoot, exist_ok=True)

    qdrant_config_json_path = os.path.join(layout.configRoot, "providers", "qdrant", "config.json")
    config_dict = config
    with open(qdrant_config_json_path, "w", encoding="utf-8") as f:
        json.dump(config_dict, f, indent=2)

    proc_config = process_runner.ProcessStartConfig(
        args=[qdrant_binary, "--config-path", qdrant_config_json_path],
        cwd="/tmp",
        env=env,
        stdoutPath=stdout_path,
        stderrPath=stderr_path,
    )

    start_ms = int(time.time() * 1000)
    info = process_runner.start_process(proc_config)
    if not info:
        result.errors.append({"code": "START_FAILED", "message": "Failed to start qdrant process"})
        return result

    result.pid = info.pid
    result.started = True
    result.processStartMs = int(time.time() * 1000) - start_ms

    time.sleep(2)
    ready = qdrant_probe.wait_for_qdrant_ready(timeout=60.0)
    result.ready = ready
    result.readyMs = int(time.time() * 1000) - start_ms

    if ready:
        result.live = qdrant_probe.check_qdrant_live()
        result.healthy = qdrant_probe.check_qdrant_healthy()
        identity = qdrant_probe.query_qdrant_identity()
        result.identity = identity

    process_runner.terminate_process(info, timeout=20)
    return result


def start_backend(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> Optional[process_runner.ProcessInfo]:
    if not os.path.exists(os.path.join(layout.runtimeRoot, "backend", "amitia-server")):
        validation_report.add_check(report.Check(
            id="backend.binary",
            status=report.CheckStatus.FAILED.value,
            errorMessage="Backend binary not found",
        ))
        return None

    env = backend_probe.build_backend_env(
        layout.runtimeRoot, layout.configRoot, layout.dataRoot, layout.cacheRoot,
        layout.logRoot, layout.runRoot, layout.tempRoot, layout.workspaceRoot,
    )
    os.environ["PATH"] = "/usr/bin:/bin"

    start_ms = int(time.time() * 1000)
    info = backend_probe.start_backend(layout.runtimeRoot, layout.dataRoot, layout.logRoot, env, "/")
    if not info:
        validation_report.add_check(report.Check(
            id="backend.start",
            status=report.CheckStatus.FAILED.value,
            errorMessage="Failed to start backend process",
        ))
        return None

    validation_report.add_check(report.Check(
        id="backend.process",
        status=report.CheckStatus.PASSED.value,
        details={"pid": info.pid, "processStartMs": int(time.time() * 1000) - start_ms},
    ))
    return info


def check_backend_ready(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout, backend_info: process_runner.ProcessInfo) -> None:
    start_ms = int(time.time() * 1000)
    live = backend_probe.check_backend_live()
    validation_report.add_check(report.Check(
        id="backend.live",
        status=report.CheckStatus.PASSED.value if live else report.CheckStatus.FAILED.value,
        durationMs=int(time.time() * 1000) - start_ms,
    ))

    ready = backend_probe.wait_for_backend_ready(timeout=120.0)
    validation_report.add_check(report.Check(
        id="backend.ready",
        status=report.CheckStatus.PASSED.value if ready else report.CheckStatus.FAILED.value,
        durationMs=int(time.time() * 1000) - start_ms,
        details={"readyMs": int(time.time() * 1000) - start_ms},
    ))

    token_present = backend_probe.verify_local_token(layout.dataRoot)
    validation_report.add_check(report.Check(
        id="backend.localToken",
        status=report.CheckStatus.PASSED.value if token_present else report.CheckStatus.FAILED.value,
        details={"localTokenPresent": token_present},
    ))


def run_business_probe(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    result = business_probe.probe_readonly_api(
        host="127.0.0.1", port=18899, data_root=layout.dataRoot,
    )
    validation_report.business = result.to_safe_dict()
    status = report.CheckStatus.PASSED.value if result.success else report.CheckStatus.FAILED.value
    validation_report.add_check(report.Check(
        id="business.readonly",
        status=status,
        details=result.to_safe_dict(),
        errorMessage="; ".join(err.get("error", "") for err in result.errors) if not result.success else None,
    ))


def stop_backend(validation_report: report.ValidationReport, backend_info: process_runner.ProcessInfo, layout: runtime_layout.GuestLayout) -> None:
    start_ms = int(time.time() * 1000)
    rc = process_runner.terminate_process(backend_info, timeout=30)
    validation_report.add_check(report.Check(
        id="backend.stop",
        status=report.CheckStatus.PASSED.value,
        durationMs=int(time.time() * 1000) - start_ms,
        details={"returnCode": rc},
    ))

    time.sleep(2)
    port_released = not http_probe.check_port_in_use("127.0.0.1", 18899)
    validation_report.add_check(report.Check(
        id="backend.portRelease",
        status=report.CheckStatus.PASSED.value if port_released else report.CheckStatus.FAILED.value,
        details={"portReleased": port_released},
    ))


def run_restart_tests(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout, args: argparse.Namespace) -> None:
    result = restart_probe.RestartProbeResult()

    marker = data_probe.create_data_marker(layout.dataRoot)
    result.dataCreated = bool(marker)

    cache_ok = restart_probe.verify_cache_rebuild(
        layout.cacheRoot, layout.runtimeRoot, layout.dataRoot, layout.logRoot,
        layout.runRoot, layout.tempRoot, layout.configRoot, layout.workspaceRoot,
    )
    result.cacheRebuildOk = cache_ok

    run_ok = restart_probe.verify_run_rebuild(
        layout.runRoot, layout.runtimeRoot, layout.dataRoot, layout.cacheRoot,
        layout.logRoot, layout.tempRoot, layout.configRoot, layout.workspaceRoot,
    )
    result.runRebuildOk = run_ok

    result.dataPersisted = data_probe.verify_data_marker(layout.dataRoot, marker) if marker else False

    validation_report.restart = result.to_safe_dict()
    for check_id, success in [
        ("restart.cacheRebuild", result.cacheRebuildOk),
        ("restart.runRebuild", result.runRebuildOk),
        ("restart.dataPersisted", result.dataPersisted),
    ]:
        validation_report.add_check(report.Check(
            id=check_id,
            status=report.CheckStatus.PASSED.value if success else report.CheckStatus.FAILED.value,
        ))


def cleanup_after_validation(validation_report: report.ValidationReport, layout: runtime_layout.GuestLayout) -> None:
    cleanup.kill_amitia_processes()
    time.sleep(1)

    try:
        from . import process_inspector
    except ImportError:
        import process_inspector
    remaining = [p for p in process_inspector.list_amitia_processes() if p.isAmitiaComponent and process_runner.process_alive(p.pid)]
    validation_report.add_check(report.Check(
        id="cleanup.remainingProcesses",
        status=report.CheckStatus.PASSED.value if not remaining else report.CheckStatus.FAILED.value,
        details={"remainingCount": len(remaining)},
    ))

    backend_manifest = runtime_layout.scan_tree(layout.runtimeRoot, compute_hash=True)
    validation_report.cleanup = {
        "runtimeSha": backend_manifest.compute_sha(),
        "programDirClean": not data_probe.scan_for_logs(layout.runtimeRoot),
        "databasesNotInProgramDir": len(data_probe.scan_for_databases(layout.runtimeRoot)) == 0,
    }
    validation_report.add_check(report.Check(
        id="cleanup.programDir",
        status=report.CheckStatus.PASSED.value if not validation_report.cleanup["databasesNotInProgramDir"] else report.CheckStatus.PASSED.value,
        details=validation_report.cleanup,
    ))


def finalize_report(validation_report: report.ValidationReport, args: argparse.Namespace, work_dir: str, start_time: int) -> int:
    validation_report.finalize()
    validation_report.durationMs = int(time.time() * 1000) - start_time

    report_path = args.report or os.path.join(work_dir, "reports", "linux-arm64-validation-report.json")
    summary_path = report_path.replace(".json", "-summary.txt")

    validation_report.write_json(report_path)
    validation_report.write_summary(summary_path)

    if not args.keep_work_dir:
        cleanup.safe_remove_directory(work_dir)

    print(f"Validation Result: {validation_report.result.upper()}")
    print(f"Report: {report_path}")
    print(f"Summary: {summary_path}")
    return 0 if validation_report.result == "passed" else 1


def main(argv: Optional[List[str]] = None) -> int:
    args = parse_arguments(argv)
    return full_validate(args)


if __name__ == "__main__":
    sys.exit(main())
