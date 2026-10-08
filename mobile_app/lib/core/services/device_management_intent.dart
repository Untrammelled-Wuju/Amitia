import 'dart:convert';

class DeviceManagementIntent {
  final bool Function() isCurrent;
  final Map<String, dynamic> policy;
  final String coreId;
  final bool canAdminister;

  DeviceManagementIntent(Map<String, dynamic> state, {required this.isCurrent})
    : policy = Map.unmodifiable(
        Map<String, dynamic>.from(
          state['policy'] is Map ? state['policy'] as Map : const {},
        ),
      ),
      coreId = (state['coreId'] ?? '').toString(),
      canAdminister =
          state['canAdminister'] == true &&
          state['policy'] is Map &&
          state['policy']['coordinated'] == true {
    if (state['coordinationAvailable'] != true ||
        coreId.isEmpty ||
        policy['deviceId'] is! String ||
        (policy['deviceId'] as String).isEmpty ||
        state['canAdminister'] is! bool ||
        policy['coordinated'] is! bool)
      throw StateError('设备管理身份与权限无法确认');
    for (final key in const [
      'providerEpoch',
      'modeRevision',
      'permissionRevision',
    ]) {
      if (policy[key] is! int ||
          policy[key] < 1 ||
          policy[key] > 9007199254740991)
        throw StateError('设备管理缺少有效权限版本');
    }
    assertConnection();
  }

  Map<String, String> get headers => {
    'X-Amitia-Expected-Core-ID': coreId,
    'X-Amitia-Expected-Configuration-Policy':
        '${policy['providerEpoch']}:${policy['modeRevision']}:${policy['permissionRevision']}',
  };

  void assertConnection() {
    if (!isCurrent()) throw StateError('Core连接或设备模式已变化，原设备管理操作已取消');
  }

  void requireTarget(String target) {
    assertConnection();
    if (!canAdminister && policy['deviceId'] != target)
      throw StateError('只能管理本设备授权或使用当前统筹Core管理员权限');
  }

  void requireAdministrator() {
    assertConnection();
    if (!canAdminister) throw StateError('只有开启统筹模式的当前Core管理员可以管理设备');
  }

  void validate(Map<String, dynamic> state, {int permissionIncrement = 0}) {
    assertConnection();
    final current = state['policy'];
    final expected = {
      ...policy,
      'permissionRevision': policy['permissionRevision'] + permissionIncrement,
    };
    if (state['coordinationAvailable'] != true ||
        state['coreId'] != coreId ||
        state['canAdminister'] != canAdminister ||
        current is! Map ||
        jsonEncode([
              for (final key in const [
                'deviceId',
                'coordinated',
                'providerEpoch',
                'modeRevision',
                'permissionRevision',
              ])
                current[key],
            ]) !=
            jsonEncode([
              for (final key in const [
                'deviceId',
                'coordinated',
                'providerEpoch',
                'modeRevision',
                'permissionRevision',
              ])
                expected[key],
            ]))
      throw StateError('Core或原设备管理权限已变化，请重新加载原页面');
  }
}
