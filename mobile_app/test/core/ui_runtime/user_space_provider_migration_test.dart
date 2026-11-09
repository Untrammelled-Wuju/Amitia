import 'dart:io';

import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('settings and profile roots stay in the core namespace', () {
    expect(isProtectedProviderRoutePath(AppRoutes.settings), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.settingsSystem), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.me), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.settingsUIProviders), isTrue);
  });

  test('the core catalog contains settings and archive fallbacks', () {
    final source = File(
      'lib/core/ui_runtime/routing/builtin_route_catalog.dart',
    ).readAsStringSync();
    expect(source, contains('path: AppRoutes.settings'));
    expect(source, contains('path: AppRoutes.chatLogs'));
    expect(source, contains('path: AppRoutes.meCharacters'));
  });

  test('the user space plugin source is removed', () {
    expect(
      File(
        '../plugins-builtin/user-space-hub/amitia-extension.json',
      ).existsSync(),
      isFalse,
    );
  });
}
