import 'package:amitia_app/core/services/device_mesh_service.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('已绑定设备列表排除当前设备', () {
    final devices = <Map<String, dynamic>>[
      <String, dynamic>{'deviceId': 'phone-a', 'label': '手机'},
      <String, dynamic>{'deviceId': 'desktop-b', 'label': '桌面'},
    ];

    final filtered = excludeDeviceMeshIdentity(devices, 'phone-a');

    expect(filtered, hasLength(1));
    expect(filtered.single['deviceId'], 'desktop-b');
    expect(devices, hasLength(2));
  });

  test('本机身份不可用时保留完整列表', () {
    final devices = <Map<String, dynamic>>[
      <String, dynamic>{'deviceId': 'phone-a'},
    ];

    expect(excludeDeviceMeshIdentity(devices, ''), same(devices));
  });
}
