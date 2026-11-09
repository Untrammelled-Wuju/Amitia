import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:amitia_app/core/ui_runtime/provider_tab_host.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_runtime_controller.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

UIProviderDefinition _provider({
  required String id,
  required String capability,
  required Map<String, dynamic> metadata,
  required Map<String, UIProviderEntry> entries,
}) {
  return UIProviderDefinition(
    providerId: id,
    extensionId: 'com.example/shell-page',
    capability: capability,
    mode: UIProviderMode.replace,
    priority: 100,
    platforms: const <String>[],
    entries: entries,
    permissions: const <String>[],
    placement: UIProviderPlacement.any,
    generation: 1,
    enabled: true,
    builtin: false,
    metadata: metadata,
  );
}

void main() {
  testWidgets('shell tab content comes from the route-selected provider', (
    tester,
  ) async {
    registerMobileHostRuntime(
      'host.test.shell-tab',
      (_) => const Text('provider shell content'),
    );
    final snapshot = UIProviderSnapshot(
      providers: [
        _provider(
          id: 'shell-page-provider',
          capability: UICapability.pageProvider,
          metadata: const {
            'routes': ['/tasks'],
          },
          entries: const {
            'mobile': UIProviderEntry(
              type: UIProviderEntryType.schemaRenderer,
              contributionId: 'shell-tab-page',
            ),
          },
        ),
      ],
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
              contributionId: 'shell-tab-page',
              extensionId: 'com.example/shell-page',
              moduleId: 'main',
              kind: 'schema_page',
              slotId: 'provider.page.provider',
              contractVersion: 1,
              entryType: 'host_native',
              runtimeId: 'host.test.shell-tab',
              entryPath: 'builtin://com.example/shell-page/tasks',
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
      version: 1,
    );
    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          uiRuntimeSnapshotOverrideProvider.overrideWithValue(snapshot),
        ],
        child: const MaterialApp(
          home: ProviderTabHost(
            routePath: '/tasks',
            fallback: Text('fallback shell content'),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('provider shell content'), findsOneWidget);
    expect(find.text('fallback shell content'), findsNothing);
    expect(find.text('页面插件不可用'), findsNothing);
  });
}
