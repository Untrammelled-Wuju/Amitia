import 'package:amitia_app/core/backend_transport/routed_backend_service_api.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('extension management uses the device local authority', () {
    expect(isDeviceLocalApiPath('/api/extension-center/view'), isTrue);
    expect(isDeviceLocalApiPath('/api/extensions/packages/artifacts'), isTrue);
    expect(isDeviceLocalApiPath('/api/extensions/updates/check'), isTrue);
    expect(
      isDeviceLocalApiPath('/api/extensions/kernel/extensions/enable'),
      isTrue,
    );
    expect(isDeviceLocalApiPath('/api/extensions/kernel/agent-skills'), isTrue);
    expect(isDeviceLocalApiPath('/api/extensions/ui/snapshot'), isTrue);
    expect(isDeviceLocalApiPath('/api/extensions/pet/plugins'), isTrue);
    expect(
      isDeviceLocalApiPath('/api/extensions/pet/plugins/qa-pet/enable'),
      isTrue,
    );
    expect(
      isDeviceLocalApiPath('/api/extensions/desktop-pet/plugins/install'),
      isTrue,
    );
    expect(isDeviceLocalApiPath('/api/mcp/interactions'), isTrue);
    expect(isDeviceLocalApiPath('/api/proactive-sse'), isTrue);
    expect(isDeviceLocalApiPath('/api/reminders'), isTrue);
  });

  test('independently placed extension capabilities keep their authority', () {
    expect(isDeviceLocalApiPath('/api/extensions/workflows'), isFalse);
    expect(isDeviceLocalApiPath('/api/extensions/mcp/servers'), isFalse);
  });
}
