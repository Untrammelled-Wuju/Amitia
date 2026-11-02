import 'package:amitia_app/core/backend_transport/routed_backend_service_api.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('extension management uses the device local authority', () {
    expect(isDeviceLocalApiPath('/api/extension-center/view'), isTrue);
    expect(
      isDeviceLocalApiPath('/api/extensions/packages/artifacts'),
      isTrue,
    );
    expect(isDeviceLocalApiPath('/api/extensions/updates/check'), isTrue);
    expect(
      isDeviceLocalApiPath('/api/extensions/kernel/extensions/enable'),
      isTrue,
    );
  });

  test('independently placed extension capabilities keep their authority', () {
    expect(isDeviceLocalApiPath('/api/extensions/workflows'), isFalse);
    expect(isDeviceLocalApiPath('/api/extensions/mcp/servers'), isFalse);
    expect(
      isDeviceLocalApiPath('/api/extensions/kernel/agent-skills'),
      isFalse,
    );
  });
}
