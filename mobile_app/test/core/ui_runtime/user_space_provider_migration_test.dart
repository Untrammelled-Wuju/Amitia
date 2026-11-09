import 'dart:io';

import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/routing/builtin_route_catalog.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

Set<String> _routePaths(List<RouteBase> routes) {
  final paths = <String>{};
  for (final route in routes) {
    if (route is GoRoute) paths.add(route.path);
    paths.addAll(_routePaths(route.routes));
  }
  return paths;
}

void main() {
  test('settings and profile roots stay in the core namespace', () {
    expect(isProtectedProviderRoutePath(AppRoutes.settings), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.settingsSystem), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.me), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.settingsUIProviders), isTrue);
  });

  test('the core catalog contains settings and archive routes', () {
    final paths = _routePaths(buildBuiltinBusinessRoutes());
    expect(paths, contains(AppRoutes.settings));
    expect(paths, contains(AppRoutes.chatLogs));
    expect(paths, contains(AppRoutes.meCharacters));
  });

  test('the user space plugin source is removed', () {
    expect(
      File(
        '../plugins-builtin/user-space-hub/amitia-extension.json',
      ).existsSync(),
      isFalse,
    );
  });

  test('core mobile surfaces retain extension injection points', () {
    const surfaces = <String, List<String>>{
      'lib/features/characters/presentation/pages/character_detail_page.dart': [
        'MobileExtensionSlot',
      ],
      'lib/features/settings/presentation/pages/system_settings_page.dart': [
        'MobileExtensionSlot',
      ],
      'lib/features/settings/presentation/pages/maintenance_page.dart': [
        'MobileExtensionSlot',
      ],
      'lib/features/game_center/presentation/pages/plugin_detail_page.dart': [
        'MobileExtensionSlot',
      ],
      'lib/features/chat/presentation/pages/chat_page.dart': [
        'MobileExtensionSlot',
        'UIProviderHost',
      ],
    };
    for (final entry in surfaces.entries) {
      final source = File(entry.key).readAsStringSync();
      for (final marker in entry.value) {
        expect(source, contains(marker), reason: '${entry.key}: $marker');
      }
    }
  });
}
