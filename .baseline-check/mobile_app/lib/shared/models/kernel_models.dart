enum HookStatus { active, inactive, circuitOpen }

class WasmModule {
  final String id;
  final String name;
  final String status;
  final int quota;
  final int used;

  WasmModule({
    required this.id,
    required this.name,
    this.status = '已加载',
    this.quota = 100,
    this.used = 0,
  });
}

class HookEntry {
  final String id;
  final String point;
  final String contributor;
  final int priority;
  final HookStatus status;

  HookEntry({
    required this.id,
    required this.point,
    required this.contributor,
    this.priority = 10,
    this.status = HookStatus.active,
  });
}

class TrustedService {
  final String id;
  final String name;
  final String runStatus;
  final bool isolated;

  TrustedService({
    required this.id,
    required this.name,
    this.runStatus = '已启动',
    this.isolated = false,
  });
}

class KernelTask {
  final String id;
  final String name;
  final String status;
  final String? output;
  final String? error;
  final bool hasCheckpoint;

  KernelTask({
    required this.id,
    required this.name,
    required this.status,
    this.output,
    this.error,
    this.hasCheckpoint = false,
  });
}

class KernelEvent {
  final String id;
  final String type;
  final String status;
  final DateTime time;
  final String? detail;

  KernelEvent({
    required this.id,
    required this.type,
    required this.status,
    required this.time,
    this.detail,
  });
}

class ScheduleEntry {
  final String id;
  final String name;
  final DateTime? nextRun;
  final DateTime? lastRun;
  final bool isEnabled;

  ScheduleEntry({
    required this.id,
    required this.name,
    this.nextRun,
    this.lastRun,
    this.isEnabled = true,
  });
}

class DesktopContribution {
  final String id;
  final String type;
  final String label;
  final String value;

  DesktopContribution({
    required this.id,
    required this.type,
    required this.label,
    required this.value,
  });
}

class UpdateInfo {
  final String version;
  final String status;
  final DateTime? date;
  final bool isAvailable;

  UpdateInfo({
    required this.version,
    this.status = '已安装',
    this.date,
    this.isAvailable = false,
  });
}

class DevConsoleLog {
  final String level;
  final String module;
  final String message;
  final DateTime time;

  DevConsoleLog({
    required this.level,
    required this.module,
    required this.message,
    required this.time,
  });
}

class MigrationPlan {
  final String id;
  final String name;
  final String status;
  final int progress;
  final String? rollbackReason;

  MigrationPlan({
    required this.id,
    required this.name,
    required this.status,
    this.progress = 0,
    this.rollbackReason,
  });
}

class DevWorkspace {
  final String id;
  final String name;
  final String version;
  final String status;

  DevWorkspace({
    required this.id,
    required this.name,
    this.version = '1.0.0',
    this.status = '已注册',
  });
}
