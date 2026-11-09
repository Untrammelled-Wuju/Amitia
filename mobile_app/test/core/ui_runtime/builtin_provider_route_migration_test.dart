import 'dart:convert';
import 'dart:io';

import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/routing/builtin_route_catalog.dart';
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
  final paths = _routePaths(buildBuiltinBusinessRoutes());

  test('critical mobile pages use explicit core routes', () {
    expect(
      paths,
      containsAll(<String>[
        AppRoutes.knowledge,
        AppRoutes.memoryEpisodic,
        AppRoutes.memoryGraph,
        AppRoutes.memoryTimeline,
        AppRoutes.memoryProfiles,
        AppRoutes.settingsSystem,
        AppRoutes.settingsStorage,
        AppRoutes.settingsMaintenance,
        AppRoutes.toolboxWorkspace,
        AppRoutes.developerKernel,
        AppRoutes.kernelPage('trusted-services'),
        AppRoutes.extensionsPackages,
        AppRoutes.extensionsMcp,
        AppRoutes.workshopWorkflows,
        '/workshop/workflows/:id',
        AppRoutes.gameCenter,
        AppRoutes.chatImport,
      ]),
    );
  });

  test('plugin market remains the only bundled official package', () {
    final manifest =
        jsonDecode(
              File(
                '../plugins-builtin/plugin-market/amitia-extension.json',
              ).readAsStringSync(),
            )
            as Map<String, dynamic>;
    expect((manifest['plugin'] as Map)['id'], 'com.amitia/plugin-market');
  });
}
