import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/workbench/runtime/workbench_mobile_events.dart';

void main() {
  test('explicit authorized route is preserved for downstream server checks', () {
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: 'device-a', explicitTarget: 'device-c', cloudMode: true, localDeviceId: 'device-b'), 'device-c');
  });
  test('rejects malformed explicit device target', () {
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: 'device-a', explicitTarget: '../invalid', cloudMode: true, localDeviceId: 'device-b'), isNull);
  });
  test('cloud browser capability must route to its origin', () {
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: 'device-a', explicitTarget: '', cloudMode: true, localDeviceId: 'device-b'), 'device-a');
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: 'device-a', explicitTarget: '', cloudMode: true, localDeviceId: 'device-a'), '');
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: 'device-a', explicitTarget: '', cloudMode: true, localDeviceId: ''), isNull);
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: '../unsafe', explicitTarget: '', cloudMode: true, localDeviceId: 'device-b'), isNull);
  });
  test('legacy browser event without origin uses local service', () {
    expect(resolveMobileWorkbenchBrowserTarget(originDeviceId: '', explicitTarget: '', cloudMode: false, localDeviceId: ''), '');
  });
}
