import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:flutter_test/flutter_test.dart';

const migratedChildPaths = <String>[
  '/extensions/packages',
  '/extensions/mcp/:id',
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
    const manifests = <String>['plugin-market'];
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

  test('personal settings and archive routes remain in the core catalog', () {
    final source = File(
      'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
    ).readAsStringSync();
    expect(source, contains('path: AppRoutes.settings'));
    expect(source, contains('path: AppRoutes.chatLogs'));
    expect(source, contains('path: AppRoutes.me'));
  });

  test(
    'core secondary pages are registered instead of falling through to 404',
    () {
      final source = File(
        'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
      ).readAsStringSync();
      expect(source, contains("path: '/settings/:pathMatch(.*)*'"));
      expect(source, contains("path: '/characters/:pathMatch(.*)*'"));
      expect(source, contains("path: '/memory/:pathMatch(.*)*'"));
      expect(source, contains("path: '/reminders'"));
      expect(source, contains("path: '/continuity'"));
      expect(source, contains("path: '/workshop'"));
      expect(source, contains("path: '/game-center'"));
      expect(source, contains("path: '/chat-import'"));
      expect(source, contains("path: '/developer'"));
      expect(source, contains("path: '/workspaces'"));
      expect(source, contains("path: '/storage'"));
    },
  );
}
