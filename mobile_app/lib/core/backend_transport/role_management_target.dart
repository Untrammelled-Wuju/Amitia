import 'package:flutter_riverpod/flutter_riverpod.dart';

const String roleManagementTargetHeader = 'X-Amitia-Role-Target';

enum RoleManagementTarget {
  local,
  center;

  String get wire => this == RoleManagementTarget.center ? 'center' : 'local';

  String get label => this == RoleManagementTarget.center ? '统筹中心' : '本机';

  String get description => this == RoleManagementTarget.center
      ? '数据保存在统筹中心，其他设备登录后可继续管理'
      : '数据只保存在这台设备上，不迁移到统筹中心';
}

RoleManagementTarget? roleManagementTargetFromWire(Object? value) {
  switch (value?.toString().trim().toLowerCase()) {
    case 'center':
      return RoleManagementTarget.center;
    case 'local':
      return RoleManagementTarget.local;
    default:
      return null;
  }
}

Map<String, String>? withRoleManagementTarget(
  Map<String, String>? headers,
  RoleManagementTarget? target,
) {
  if (target == null) return headers;
  return <String, String>{...?headers, roleManagementTargetHeader: target.wire};
}

RoleManagementTarget resolveRoleManagementTarget(
  RoleManagementTarget? selected, {
  required bool cloud,
}) =>
    selected ??
    (cloud ? RoleManagementTarget.center : RoleManagementTarget.local);

bool roleManagementCenterAvailable(Map<String, dynamic>? coordination) {
  if (coordination == null) return false;
  final policy = coordination['policy'];
  return coordination['coordinationAvailable'] == true &&
      policy is Map &&
      policy['administrator'] == true;
}

final roleManagementTargetProvider = StateProvider<RoleManagementTarget?>(
  (ref) => null,
);

final mcpManagementTargetProvider = StateProvider<RoleManagementTarget?>(
  (ref) => null,
);

final skillManagementTargetProvider = StateProvider<RoleManagementTarget?>(
  (ref) => null,
);

final taskManagementTargetProvider = StateProvider<RoleManagementTarget?>(
  (ref) => null,
);
