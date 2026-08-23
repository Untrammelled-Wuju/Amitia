import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../app/app_routes.dart';
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

const _protectedRouteNamespaces = <String>{
  AppRoutes.onboarding,
  AppRoutes.login,
  AppRoutes.privacy,
  '/usage-boundary',
  AppRoutes.chat,
  AppRoutes.conversations,
  AppRoutes.dashboard,
  AppRoutes.channels,
  AppRoutes.characters,
  AppRoutes.agent,
  AppRoutes.memory,
  AppRoutes.reminders,
  AppRoutes.emotes,
  AppRoutes.chatLogs,
  AppRoutes.chatImport,
  AppRoutes.extensions,
  '/extension',
  AppRoutes.workshop,
  AppRoutes.settings,
  AppRoutes.developer,
  AppRoutes.gameCenter,
  AppRoutes.desktopPet,
};

String? _rootNamespace(String path) {
  final normalized = path.trim();
  if (!normalized.startsWith('/') || normalized == '/') return null;
  final segment = normalized.substring(1).split('/').first.trim();
  if (segment.isEmpty || segment.startsWith(':') || segment.contains('*')) return null;
  return '/$segment';
}

bool isProtectedProviderRoutePath(String path) {
  final namespace = _rootNamespace(path);
  return namespace != null && _protectedRouteNamespaces.contains(namespace);
}

bool _hasSafeProviderRouteSyntax(String path) {
  final normalized = path.trim();
  if (!normalized.startsWith('/') ||
      normalized == '/' ||
      normalized.contains('\\') ||
      normalized.contains('?') ||
      normalized.contains('#') ||
      normalized.contains('%') ||
      normalized.contains('\u0000') ||
      _rootNamespace(normalized) == null) {
    return false;
  }
  return normalized
      .substring(1)
      .split('/')
      .every((segment) => segment.isNotEmpty && segment != '.' && segment != '..');
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
  final platform = currentUIPlatform();
  for (final registry in _activeRegistries(snapshot)) {
    final rawRoutes = registry.metadata['routes'];
    if (rawRoutes is! List) continue;
    for (final raw in rawRoutes) {
      if (raw is! Map) continue;
      final row = raw.cast<String, dynamic>();
      final id = (row['id'] ?? '').toString().trim();
      final path = (row['path'] ?? '').toString().trim();
      final providerId = (row['providerId'] ?? '').toString().trim();
      final capability = (row['capability'] ?? UICapability.pageProvider).toString().trim();
      if (id.isEmpty ||
          providerId.isEmpty ||
          !_hasSafeProviderRouteSyntax(path) ||
          !UICapability.all.contains(capability) ||
          isProtectedProviderRoutePath(path)) {
        continue;
      }
      final target = snapshot.providers.where((p) => p.providerId == providerId).firstOrNull;
      if (target == null ||
          target.extensionId != registry.extensionId ||
          target.capability != capability ||
          !target.enabled ||
          !target.compatibleWith(snapshot.context, platform)) {
        continue;
      }
      final rawPriority = row['priority'];
      specs.add(_ProviderRouteSpec(
        registry: registry,
        id: id,
        path: path,
        providerId: providerId,
        capability: capability,
        priority: rawPriority is num ? rawPriority.toInt() : registry.priority,
      ));
    }
  }
  specs.sort((a, b) {
    final priority = b.priority.compareTo(a.priority);
    if (priority != 0) return priority;
    final extension = a.registry.extensionId.compareTo(b.registry.extensionId);
    if (extension != 0) return extension;
    final registry = a.registry.providerId.compareTo(b.registry.providerId);
    return registry != 0 ? registry : a.id.compareTo(b.id);
  });
  final seen = <String>{};
  return specs.where((spec) => seen.add(spec.path)).toList(growable: false);
}

/// Keys for routes that survived compatibility, ownership, core-namespace and
/// path-conflict checks. Used by navigation registries to hide stale links.
Set<String> effectiveProviderRouteKeys(UIProviderSnapshot snapshot) => _routeSpecs(snapshot)
    .map((spec) => '${spec.registry.providerId}\u0000${spec.path}')
    .toSet();

Set<String> effectiveExtensionRouteKeys(UIProviderSnapshot snapshot) => _routeSpecs(snapshot)
    .map((spec) => '${spec.registry.extensionId}\u0000${spec.path}')
    .toSet();

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
    final routeName = 'ui-provider-${spec.registry.extensionId}-${spec.registry.providerId}-${spec.id}'.replaceAll(':', '-');
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
