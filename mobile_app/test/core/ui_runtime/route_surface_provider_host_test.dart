import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:amitia_app/core/ui_runtime/route_surface_provider_host.dart';
import 'package:amitia_app/core/ui_runtime/routing/ui_surface_catalog.dart';
import 'package:amitia_app/core/ui_runtime/ui_page_provider_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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
    expect(capabilityForBuiltinRoute('/memory-hub'), UICapability.memoryShell);
    expect(capabilityForBuiltinRoute('/memory'), UICapability.memoryDetail);
    expect(
      capabilityForBuiltinRoute('/memory/timeline'),
      UICapability.memoryDetail,
    );
    expect(canonicalUISurfaceId('/memory-hub'), 'surface.memory.hub');
    expect(canonicalUISurfaceId('/memory'), 'surface.memory.detail');
  });

  testWidgets('channel routes use the generic provider surface', (
    tester,
  ) async {
    registerMobileHostRuntime(
      'host.test.channel',
      (_) => const Text('provider channel content'),
    );
    final provider = UIProviderDefinition(
      providerId: 'channel-page-provider',
      extensionId: 'com.example/channel',
      capability: UICapability.pageProvider,
      mode: UIProviderMode.replace,
      priority: 100,
      platforms: const <String>[],
      entries: const {
        '*': UIProviderEntry(
          type: UIProviderEntryType.schemaRenderer,
          contributionId: 'channel-page',
        ),
      },
      permissions: const <String>[],
      placement: UIProviderPlacement.any,
      generation: 1,
      enabled: true,
      builtin: false,
      metadata: const {
        'routes': ['/channels/test'],
      },
    );
    final snapshot = UIProviderSnapshot(
      providers: [provider],
      slots: const [
        UISlotSnapshotEntry(
          slotId: 'provider.page.provider',
          contractVersion: 1,
          kind: 'single',
          layout: 'stack',
          fallbackPolicy: 'default',
          declarationEpoch: 1,
          scope: 'root',
          dynamicSlot: false,
          contributions: [
            UIContributionSnapshotEntry(
              contributionId: 'channel-page',
              extensionId: 'com.example/channel',
              moduleId: 'main',
              kind: 'schema_page',
              slotId: 'provider.page.provider',
              contractVersion: 1,
              entryType: 'host_native',
              runtimeId: 'host.test.channel',
              entryPath: 'builtin://com.example/channel/test',
              permissions: <String>[],
              dataContract: <String, dynamic>{},
              visibility: <String, dynamic>{},
              ordering: 100,
              priority: 100,
            ),
          ],
        ),
      ],
      profile: const UIProfile(
        profileId: 'default',
        name: 'Default',
        selections: <String, String>{},
      ),
      profileLayers: const <UIProfile>[],
      context: const UIProviderResolveContext(localRuntime: true),
      resolved: const <String, UIProviderDefinition>{},
      providerResolutions: const <String, UIProviderResolution>{
        'channel-page-provider': UIProviderResolution(
          providerId: 'channel-page-provider',
          capability: UICapability.pageProvider,
          compatible: true,
        ),
      },
      version: 1,
    );
    expect(
      UIPageProviderRegistry.resolve(
        snapshot,
        capability: UICapability.pageProvider,
        route: '/channels/test',
      )?.providerId,
      'channel-page-provider',
    );
    expect(
      snapshot
          .fallbackChain(
            UICapability.pageProvider,
            providerId: 'channel-page-provider',
          )
          .map((item) => item.providerId),
      contains('channel-page-provider'),
    );
    expect(
      snapshot.contributionById('channel-page')?.runtimeId,
      'host.test.channel',
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          uiRuntimeSnapshotOverrideProvider.overrideWithValue(snapshot),
        ],
        child: const MaterialApp(
          home: RouteSurfaceProviderHost(
            route: '/channels/test',
            child: Text('fallback channel content'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('provider channel content'), findsOneWidget);
    expect(find.text('fallback channel content'), findsNothing);
  });
}
