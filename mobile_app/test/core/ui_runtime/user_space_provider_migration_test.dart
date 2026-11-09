import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/mobile_host_runtime_registry.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test(
    'settings children use provider routes while repair stays protected',
    () {
      expect(isProtectedProviderRoutePath(AppRoutes.settings), isFalse);
      expect(isProtectedProviderRoutePath(AppRoutes.settingsSystem), isFalse);
      expect(
        isProtectedProviderRoutePath(AppRoutes.settingsUIProviders),
        isTrue,
      );
    },
  );

  test('settings children are no longer fixed in the mobile route catalog', () {
    final source = File(
      'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
    ).readAsStringSync();
    expect(source, isNot(contains("path: '/settings'")));
    expect(source, isNot(contains("path: '/settings/system'")));
    expect(source, isNot(contains("path: '/settings/models/:modelType'")));
    expect(source, isNot(contains("path: '/settings/toolbox/workspace'")));
    expect(source, isNot(contains("path: '/settings/category/:categoryId'")));
  });

  test('mobile host registry resolves the settings surface', () {
    expect(resolveMobileHostRuntime('host.user-space'), isNotNull);
  });

  test('user space manifest publishes mobile route and page entries', () {
    final manifest =
        jsonDecode(
              File(
                '../plugins-builtin/user-space-hub/amitia-extension.json',
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
      (item) => item['id'] == 'user-space-routes',
    );
    final provider = contributions.firstWhere(
      (item) => item['id'] == 'user-space-page-provider',
    );
    final registryEntries =
        ((registry['spec'] as Map)['entries'] as Map<String, dynamic>);
    final providerEntries =
        ((provider['spec'] as Map)['entries'] as Map<String, dynamic>);
    expect(registryEntries, contains('mobile'));
    expect(providerEntries, contains('*'));
  });
}
