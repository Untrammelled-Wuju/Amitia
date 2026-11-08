import 'package:amitia_app/core/device_mesh/device_presence.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('设备协同将 ready 和 degraded 视为已连接', () {
    expect(isDeviceMeshConnectedState('ready'), isTrue);
    expect(isDeviceMeshConnectedState('READY'), isTrue);
    expect(isDeviceMeshConnectedState('degraded'), isTrue);
    expect(isDeviceMeshConnectedState('connecting'), isFalse);
    expect(isDeviceMeshConnectedState('unprovisioned'), isFalse);
  });
}
