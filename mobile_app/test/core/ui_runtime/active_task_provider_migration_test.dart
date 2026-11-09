import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_provider.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('active task route is no longer a protected host namespace', () {
    expect(isProtectedProviderRoutePath('/reminders'), isFalse);
  });

  test('active task route resolves from the mobile provider snapshot', () {
    debugDefaultTargetPlatformOverride = TargetPlatform.android;
    addTearDown(() => debugDefaultTargetPlatformOverride = null);
    final snapshot = UIProviderSnapshot(
      providers: <UIProviderDefinition>[
        UIProviderDefinition(
          providerId: 'active-task-routes',
          extensionId: 'com.amitia/active-task',
          capability: UICapability.routeRegistry,
          mode: UIProviderMode.replace,
          priority: 248,
          platforms: const <String>[],
          entries: const <String, UIProviderEntry>{
            'mobile': UIProviderEntry(type: UIProviderEntryType.declarative),
          },
          permissions: const <String>[],
          placement: UIProviderPlacement.any,
          generation: 1,
          enabled: true,
          builtin: false,
          metadata: const <String, dynamic>{
            'routes': <Map<String, dynamic>>[
              <String, dynamic>{
                'id': 'active-task',
                'path': '/reminders',
                'providerId': 'active-task-page-provider',
                'capability': UICapability.pageProvider,
              },
            ],
          },
        ),
        UIProviderDefinition(
          providerId: 'active-task-page-provider',
          extensionId: 'com.amitia/active-task',
          capability: UICapability.pageProvider,
          mode: UIProviderMode.replace,
          priority: 248,
          platforms: const <String>[],
          entries: const <String, UIProviderEntry>{
            'mobile': UIProviderEntry(
              type: UIProviderEntryType.schemaRenderer,
              contributionId: 'active-task-page',
            ),
          },
          permissions: const <String>[],
          placement: UIProviderPlacement.any,
          generation: 1,
          enabled: true,
          builtin: false,
          metadata: const <String, dynamic>{},
        ),
      ],
      slots: const <UISlotSnapshotEntry>[],
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
    expect(
      resolveProviderRoutes(snapshot).map((route) => route.path),
      contains('/reminders'),
    );
  });

  test('mobile host runtime registry resolves active task dependencies', () {
    expect(resolveMobileHostRuntime('host.tasks.continuity'), isNotNull);
    expect(resolveMobileHostRuntime('host.character.detail'), isNotNull);
    expect(resolveMobileHostRuntime('host.memory.overview'), isNotNull);
    expect(resolveMobileHostRuntime('host.unknown'), isNull);
  });

  test('active task manifest publishes mobile provider entries', () {
    final manifest =
        jsonDecode(
              File(
                '../plugins-builtin/active-task/amitia-extension.json',
              ).readAsStringSync(),
            )
            as Map<String, dynamic>;
    final modules = (manifest['modules'] as List).cast<Map<String, dynamic>>();
    final contributions = modules
        .expand(
          (module) =>
              (module['contributions'] as List).cast<Map<String, dynamic>>(),
        )
        .toList();
    final registry = contributions.firstWhere(
      (item) => item['id'] == 'active-task-routes',
    );
    final provider = contributions.firstWhere(
      (item) => item['id'] == 'active-task-page-provider',
    );
    final registryEntries =
        ((registry['spec'] as Map)['entries'] as Map<String, dynamic>);
    final providerEntries =
        ((provider['spec'] as Map)['entries'] as Map<String, dynamic>);
    expect(registryEntries, contains('mobile'));
    expect(providerEntries, contains('mobile'));
  });
}
