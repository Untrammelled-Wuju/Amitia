import 'package:amitia_app/app/app_routes.dart';
import 'package:amitia_app/core/ui_runtime/routing/builtin_route_catalog.dart';
import 'package:amitia_app/core/ui_runtime/ui_route_registry.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';

Set<String> _paths(List<RouteBase> routes) {
  final paths = <String>{};
  for (final route in routes) {
    if (route is GoRoute) paths.add(route.path);
    paths.addAll(_paths(route.routes));
  }
  return paths;
}

void main() {
  test('memory routes follow the actual navigation hierarchy', () {
    final paths = _paths(buildBuiltinBusinessRoutes());
    expect(paths, contains(AppRoutes.memoryHub));
    expect(paths, contains(AppRoutes.meCharacters));
    expect(paths, contains(AppRoutes.memory));
    expect(paths, contains(AppRoutes.knowledge));
    expect(paths, contains(AppRoutes.memoryEpisodic));
    expect(paths, contains(AppRoutes.memoryGraph));
    expect(paths, contains(AppRoutes.memoryTimeline));
    expect(paths, contains(AppRoutes.memoryProfiles));
    expect(paths, contains(AppRoutes.memoryWorldBook));
    expect(paths, isNot(contains('/memory/manager')));
    expect(paths, isNot(contains('/memory-manager')));
    expect(paths, isNot(contains('/knowledge')));
    expect(isProtectedProviderRoutePath(AppRoutes.memoryHub), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.meCharacters), isTrue);
    expect(isProtectedProviderRoutePath(AppRoutes.knowledge), isTrue);
  });
}
