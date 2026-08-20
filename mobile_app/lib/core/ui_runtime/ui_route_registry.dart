import 'dart:convert';

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

class _ProviderRouteSpec {
  const _ProviderRouteSpec({
    required this.registry,
    required this.id,
    required this.path,
    required this.providerId,
    required this.capability,
    required this.priority,
  });
  final UIProviderDefinition registry;
  final String id;
  final String path;
  final String providerId;
  final String capability;
  final int priority;
}

List<UIProviderDefinition> _activeRegistries(UIProviderSnapshot snapshot) {
  final platform = currentUIPlatform();
  final providers = snapshot.providers
      .where((provider) =>
          provider.enabled &&
          !provider.builtin &&
          provider.capability == UICapability.routeRegistry &&
          provider.compatibleWith(snapshot.context, platform))
      .toList()
    ..sort((a, b) {
      final priority = b.priority.compareTo(a.priority);
      return priority != 0 ? priority : a.providerId.compareTo(b.providerId);
    });
  return providers;
}

List<_ProviderRouteSpec> _routeSpecs(UIProviderSnapshot snapshot) {
  final specs = <_ProviderRouteSpec>[];
  for (final registry in _activeRegistries(snapshot)) {
    final rawRoutes = registry.metadata['routes'];
    if (rawRoutes is! List) continue;
    for (final raw in rawRoutes) {
      if (raw is! Map) continue;
      final row = raw.cast<String, dynamic>();
      final id = (row['id'] ?? '').toString().trim();
      final path = (row['path'] ?? '').toString().trim();
      final providerId = (row['providerId'] ?? '').toString().trim();
      final capability = (row['capability'] ?? UICapability.pageProvider).toString();
      if (id.isEmpty || providerId.isEmpty || !path.startsWith('/') ||
          !UICapability.all.contains(capability) || _shadowsCoreRoute(path)) {
        continue;
      }
      final target = snapshot.providers.where((p) => p.providerId == providerId).firstOrNull;
      if (target == null ||
          target.capability != capability ||
          !target.enabled ||
          !target.compatibleWith(snapshot.context, currentUIPlatform())) {
        continue;
      }
      specs.add(_ProviderRouteSpec(
        registry: registry,
        id: id,
        path: path,
        providerId: providerId,
        capability: capability,
        priority: (row['priority'] as num?)?.toInt() ?? registry.priority,
      ));
    }
  }
  specs.sort((a, b) {
    final priority = b.priority.compareTo(a.priority);
    if (priority != 0) return priority;
    final extension = a.registry.extensionId.compareTo(b.registry.extensionId);
    return extension != 0 ? extension : a.id.compareTo(b.id);
  });
  final seen = <String>{};
  return specs.where((spec) => seen.add(spec.path)).toList(growable: false);
}

/// Stable signature used by the router provider. Snapshot/theme refreshes do not
/// recreate GoRouter unless the effective extension route table actually changes.
String uiRouteRegistrySignature(UIProviderSnapshot? snapshot) {
  if (snapshot == null) return 'builtin';
  final rows = _routeSpecs(snapshot)
      .map((spec) => <String, Object?>{
            'registry': spec.registry.providerId,
            'generation': spec.registry.generation,
            'id': spec.id,
            'path': spec.path,
            'providerId': spec.providerId,
            'capability': spec.capability,
            'priority': spec.priority,
          })
      .toList(growable: false);
  return jsonEncode(rows);
}

/// All enabled compatible route.registry providers contribute routes. Conflicts
/// are deterministic: higher route/provider priority wins and core paths are protected.
List<RouteBase> buildProviderRoutes(UIProviderSnapshot? snapshot) {
  if (snapshot == null) return const <RouteBase>[];
  return _routeSpecs(snapshot).map((spec) {
    final routeName = 'ui-provider-${spec.registry.extensionId}-${spec.id}'.replaceAll(':', '-');
    return GoRoute(
      name: routeName,
      path: spec.path,
      builder: (context, state) => UIProviderHost(
        capability: spec.capability,
        providerId: spec.providerId,
        context: <String, dynamic>{
          'route': state.uri.toString(),
          'pathParameters': state.pathParameters,
          'queryParameters': state.uri.queryParameters,
          'routeRegistryProviderId': spec.registry.providerId,
        },
        fallback: ProviderRouteUnavailable(providerId: spec.providerId),
      ),
    );
  }).toList(growable: false);
}

extension _FirstOrNull<T> on Iterable<T> {
  T? get firstOrNull {
    final iterator = this.iterator;
    return iterator.moveNext() ? iterator.current : null;
  }
}
