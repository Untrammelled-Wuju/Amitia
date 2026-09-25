import 'package:amitia_app/core/ui_runtime/route_surface_provider_host.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('plugin character subroutes resolve through page providers', () {
    expect(
      capabilityForBuiltinRoute('/characters/c1/proactive'),
      UICapability.pageProvider,
    );
    expect(
      capabilityForBuiltinRoute('/characters/c1/life-rules'),
      UICapability.pageProvider,
    );
    expect(
      capabilityForBuiltinRoute('/characters/c1'),
      UICapability.characterDetail,
    );
  });
}
