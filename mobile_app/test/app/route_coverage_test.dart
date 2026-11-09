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

Map<String, String> _staticRoutes() {
  final source = File('lib/app/app_routes.dart').readAsStringSync();
  return {
    for (final match in RegExp(
      r"static const ([A-Za-z][A-Za-z0-9_]*) = '([^']+)';",
    ).allMatches(source))
      match.group(1)!: match.group(2)!,
  };
}

Set<String> _usedStaticRoutes() {
  final names = <String>{};
  for (final entry in Directory('lib').listSync(recursive: true)) {
    if (entry is! File || !entry.path.endsWith('.dart')) continue;
    final source = entry.readAsStringSync();
    names.addAll(
      RegExp(r'AppRoutes\.([A-Za-z][A-Za-z0-9_]*)')
          .allMatches(source)
          .map((match) => match.group(1)!),
    );
  }
  return names;
}

bool _matchesRoute(String path, String pattern) {
  final pathSegments = Uri.parse(path).pathSegments;
  final patternSegments = Uri.parse(pattern).pathSegments;
  if (pathSegments.length != patternSegments.length) return false;
  for (var index = 0; index < patternSegments.length; index++) {
    final expected = patternSegments[index];
    if (expected.startsWith(':') || expected.startsWith('*')) continue;
    if (expected != pathSegments[index]) return false;
  }
  return true;
}

void main() {
  final routes = <String>{
    ..._routePaths(buildBuiltinBusinessRoutes()),
    AppRoutes.onboarding,
    AppRoutes.privacy,
    AppRoutes.settingsUIProviders,
  };
  final staticRoutes = _staticRoutes();
  final usedStaticRoutes = _usedStaticRoutes()
      .where(staticRoutes.containsKey)
      .toList()
    ..sort();

  test('every referenced static AppRoutes path resolves to a mobile route', () {
    for (final name in usedStaticRoutes) {
      final path = staticRoutes[name]!;
      expect(
        routes.any((pattern) => _matchesRoute(path, pattern)),
        isTrue,
        reason: 'AppRoutes.$name ($path) has no matching route',
      );
    }
  });

  test('every dynamic AppRoutes helper resolves to a mobile route', () {
    final paths = <String>[
      AppRoutes.character('c1'),
      AppRoutes.characterLifeRules('c1'),
      AppRoutes.characterVoice('c1'),
      AppRoutes.characterPsyche('c1'),
      AppRoutes.agentTask('t1'),
      AppRoutes.continuityDetail('c1'),
      AppRoutes.mcpDetail('m1'),
      AppRoutes.mcpEdit('m1'),
      AppRoutes.extensionPage('page1'),
      AppRoutes.workflowEditor('w1'),
      AppRoutes.modelConfig('llm'),
      AppRoutes.kernelPage('wasm'),
    ];
    for (final path in paths) {
      expect(
        routes.any((pattern) => _matchesRoute(path, pattern)),
        isTrue,
        reason: '$path has no matching route',
      );
    }
  });
}
