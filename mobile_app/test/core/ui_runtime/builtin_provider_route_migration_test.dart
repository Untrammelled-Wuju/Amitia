import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:flutter_test/flutter_test.dart';

const migratedChildPaths = <String>[
  '/settings',
  '/characters',
  '/characters/:id',
  '/characters/teams',
  '/memory',
  '/memory/episodic',
  '/memory/graph',
  '/workshop/character-cards',
  '/workshop/pet/installations',
  '/game-center',
  '/extensions/packages',
  '/extensions/mcp/:id',
  '/chat-logs',
  '/chat-import',
  '/developer',
  '/developer/kernel/tasks',
  '/channel-center',
  '/desktop-pet',
];

void main() {
  test('migrated child pages are absent from the fixed mobile catalog', () {
    final source = File(
      'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
    ).readAsStringSync();
    for (final path in migratedChildPaths) {
      expect(source, isNot(contains("path: '$path'")));
    }
  });

  test('all migrated mobile plugins publish route registries', () {
    const manifests = <String>[
      'character-hub',
      'memory-hub',
      'creative-workshop-hub',
      'game-center-hub',
      'plugin-market',
      'chat-logs',
      'chat-import',
      'runtime-insights',
      'channels-hub',
    ];
    for (final name in manifests) {
      final manifest =
          jsonDecode(
                File(
                  '../plugins-builtin/$name/amitia-extension.json',
                ).readAsStringSync(),
              )
              as Map<String, dynamic>;
      final modules = (manifest['modules'] as List)
          .cast<Map<String, dynamic>>();
      final contributions = modules
          .expand(
            (module) =>
                (module['contributions'] as List).cast<Map<String, dynamic>>(),
          )
          .toList();
      final registries = contributions.where(
        (item) => (item['spec'] as Map?)?['capability'] == 'route.registry',
      );
      expect(
        registries.any(
          (item) => (((item['spec'] as Map)['entries'] as Map?) ?? const {})
              .containsKey('mobile'),
        ),
        isTrue,
        reason: name,
      );
    }
  });

  test('generic mobile native page runtime is registered', () {
    expect(resolveMobileHostRuntime('host.mobile.native-page'), isNotNull);
  });
}
