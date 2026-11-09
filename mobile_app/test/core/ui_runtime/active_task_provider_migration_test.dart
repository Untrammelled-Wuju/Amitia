import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('active task routes stay in the protected core namespace', () {
    expect(isProtectedProviderRoutePath('/tasks'), isTrue);
    expect(isProtectedProviderRoutePath('/reminders'), isTrue);
    expect(isProtectedProviderRoutePath('/continuity'), isTrue);
  });

  test('mobile host runtime registry keeps active task extension points', () {
    expect(resolveMobileHostRuntime('host.tasks.continuity'), isNotNull);
    expect(resolveMobileHostRuntime('host.character.detail'), isNotNull);
    expect(resolveMobileHostRuntime('host.memory.overview'), isNotNull);
    expect(resolveMobileHostRuntime('host.unknown'), isNull);
  });
}
