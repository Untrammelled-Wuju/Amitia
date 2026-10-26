import 'package:amitia_app/core/ui_runtime/route_surface_provider_host.dart';
import 'package:amitia_app/core/ui_runtime/routing/ui_surface_catalog.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('plugin character subroutes resolve through page providers', () {
    expect(
      capabilityForBuiltinRoute('/characters/c1/active-task'),
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

  test('character route identity is available to plugin providers', () {
    expect(
      providerRouteIdentity('/characters/c1/active-task'),
      <String, String>{'characterId': 'c1'},
    );
  });

  test('memory routes follow the root hub and detail hierarchy', () {
    expect(
      capabilityForBuiltinRoute('/memory-hub'),
      UICapability.memoryShell,
    );
    expect(capabilityForBuiltinRoute('/memory'), UICapability.memoryDetail);
    expect(
      capabilityForBuiltinRoute('/memory/timeline'),
      UICapability.memoryDetail,
    );
    expect(canonicalUISurfaceId('/memory-hub'), 'surface.memory.hub');
    expect(canonicalUISurfaceId('/memory'), 'surface.memory.detail');
  });
}
