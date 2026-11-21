import 'package:amitia_app/core/device_mesh/device_display_name.dart';
import 'package:amitia_app/core/device_mesh/system_device_name.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('设备列表按中心确认名显示并追加重名序号', () {
    final names = resolveDeviceDisplayNames(const [
      {'deviceId': 'a', 'label': 'MacBook Pro'},
      {'deviceId': 'b', 'label': '红米K50'},
      {'deviceId': 'c', 'label': '红米K50'},
      {'deviceId': 'd', 'label': '红米K50'},
    ]);
    expect(names['a'], 'MacBook Pro');
    expect(names['b'], '红米K50');
    expect(names['c'], '红米K50 (2)');
    expect(names['d'], '红米K50 (3)');
  });

  test('未命名设备回退到自报名，绝不回退到设备标识', () {
    final names = resolveDeviceDisplayNames(const [
      {'deviceId': 'device-6e1f0c', 'label': '', 'reportedName': 'iPad'},
      {'deviceId': 'device-2b7a55', 'label': '', 'reportedName': ''},
    ]);
    expect(names['device-6e1f0c'], 'iPad');
    expect(names['device-2b7a55'], '未命名设备');
    expect(deviceDisplayName('device-2b7a55', const []), '未命名设备');
  });

  test('设备名归一化剔除控制字符并折叠空白', () {
    expect(normalizeDeviceName('  MacBook\u0000 Pro\n'), 'MacBook Pro');
    expect(normalizeDeviceName('红米\u200bK50'), '红米K50');
    expect(normalizeDeviceName('   '), '');
  });

  test('设备名归一化限制在 64 个字符内', () {
    final raw = List<String>.filled(maxDeviceNameRunes + 12, '名').join();
    expect(normalizeDeviceName(raw).runes.length, maxDeviceNameRunes);
  });
}
