import 'package:amitia_app/core/device_mesh/device_presence.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('device presence accepts runtime ready states', () {
    expect(isDevicePresenceOnline('ready'), isTrue);
    expect(isDevicePresenceOnline('DEGRADED'), isTrue);
    expect(isDevicePresenceOnline('online'), isTrue);
    expect(isDevicePresenceOnline('connected'), isTrue);
    expect(isDevicePresenceOnline('disconnected'), isFalse);
    expect(isDevicePresenceOnline(''), isFalse);
  });

  test('only trusted devices permit sync and management probes', () {
    expect(isDeviceMeshTrusted('trusted'), isTrue);
    expect(isDeviceMeshTrusted('revoked'), isFalse);
    expect(isDeviceMeshTrusted('pending'), isFalse);
    expect(deviceSyncBlockedLabel('trusted'), isEmpty);
    expect(deviceSyncBlockedLabel('revoked'), '已撤销，不再同步');
    expect(deviceSyncBlockedLabel('pending'), '未受信任，暂不可同步');
  });
}
