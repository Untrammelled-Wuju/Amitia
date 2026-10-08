import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/core/notifications/native_data_capabilities.dart';

void main() {
  test('requires the intersection of device and current Cloud Core', () {
    expect(
      effectiveNativeDataProviders(
        ['mipush', 'OPPO', 'vivo'],
        {'mipush': true, 'oppo': false, 'vivo': false, 'hms': true},
      ),
      ['mipush'],
    );
  });

  test('switching Cloud Core immediately changes effective capability', () {
    const device = ['mipush', 'hms'];
    expect(effectiveNativeDataProviders(device, {'mipush': true}), ['mipush']);
    expect(effectiveNativeDataProviders(device, {'hms': true}), ['hms']);
    expect(effectiveNativeDataProviders(device, const {}), isEmpty);
  });

  test('never treats token presence or malformed capability as support', () {
    expect(effectiveNativeDataProviders(['oppo'], null), isEmpty);
    expect(effectiveNativeDataProviders(['oppo'], {'oppo': 'true'}), isEmpty);
    expect(effectiveNativeDataProviders([], {'oppo': true}), isEmpty);
  });
}
