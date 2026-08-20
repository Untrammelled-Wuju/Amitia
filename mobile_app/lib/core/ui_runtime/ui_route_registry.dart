import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'ui_provider.dart';
import 'ui_provider_host.dart';

class ProviderRouteUnavailable extends StatelessWidget {
  const ProviderRouteUnavailable({super.key, required this.providerId});

  final String providerId;

  @override
  Widget build(BuildContext context) => Scaffold(
        body: Center(child: Text('UI provider unavailable: $providerId')),
      );
}

const _protectedRoutePrefixes = <String>{
  '/onboarding', '/login', '/setup', '/privacy', '/usage-boundary',
  '/settings/ui-providers', '/chat', '/characters', '/character', '/memory',
  '/settings', '/extensions', '/extension/page',
};

bool _shadowsCoreRoute(String path) {
  for (final prefix in _protectedRoutePrefixes) {
    if (path == prefix || path.startsWith('$prefix/')) return true;
  }
  return false;
}

/// Build routes only from the profile-resolved route.registry provider.
/// Installing or enabling an extension never grants it route ownership by
/// itself; the user must explicitly select the provider in the UI profile.
List<RouteBase> buildProviderRoutes(UIProviderSnapshot? snapshot) {
  if (snapshot == null) return const <RouteBase>[];
  final platform = currentUIPlatform();
  final registryProvider = snapshot.resolve(UICapability.routeRegistry);
  if (registryProvider == null ||
      !registryProvider.enabled ||
      registryProvider.builtin ||
      registryProvider.entryFor(platform) == null) {
    return const <RouteBase>[];
  }

  final rawRoutes = registryProvider.metadata['routes'];
  if (rawRoutes is! List) return const <RouteBase>[];

  final routes = <RouteBase>[];
  final seenPaths = <String>{};
  final seenNames = <String>{};
  for (final raw in rawRoutes) {
    if (raw is! Map) continue;
    final row = raw.cast<String, dynamic>();
    final id = (row['id'] ?? '').toString().trim();
    final path = (row['path'] ?? '').toString().trim();
    final providerId = (row['providerId'] ?? '').toString().trim();
    final capability =
        (row['capability'] ?? UICapability.pageProvider).toString();
    final routeName =
        'ui-provider-${registryProvider.extensionId}-$id'.replaceAll(':', '-');
    if (id.isEmpty ||
        providerId.isEmpty ||
        !path.startsWith('/') ||
        !UICapability.all.contains(capability) ||
        _shadowsCoreRoute(path) ||
        !seenPaths.add(path) ||
        !seenNames.add(routeName)) {
      continue;
    }
    routes.add(
      GoRoute(
        name: routeName,
        path: path,
        builder: (context, state) => UIProviderHost(
          capability: capability,
          providerId: providerId,
          context: <String, dynamic>{
            'route': state.uri.toString(),
            'pathParameters': state.pathParameters,
            'queryParameters': state.uri.queryParameters,
            'routeRegistryProviderId': registryProvider.providerId,
          },
          fallback: ProviderRouteUnavailable(providerId: providerId),
        ),
      ),
    );
  }
  return routes;
}
