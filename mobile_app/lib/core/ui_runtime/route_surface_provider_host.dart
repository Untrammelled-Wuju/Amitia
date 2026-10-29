import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'routing/ui_surface_catalog.dart';
import 'ui_page_provider_registry.dart';
import 'ui_provider.dart';
import 'ui_provider_host.dart';
import 'ui_runtime_controller.dart';

String? capabilityForBuiltinRoute(String route) {
  if (route == '/chat') return null; // Conversation runtime has its own host.
  if (route == '/characters') return UICapability.characterShell;
  if (route.startsWith('/characters/') &&
      (route.endsWith('/life-rules') ||
          route.endsWith('/active-task') ||
          route.endsWith('/proactive'))) {
    return UICapability.pageProvider;
  }
  if (route.startsWith('/characters/')) return UICapability.characterDetail;
  if (route == '/memory-hub') {
    return UICapability.memoryShell;
  }
  if (route == '/memory' || route.startsWith('/memory/')) {
    return UICapability.memoryDetail;
  }
  if (route == '/settings/ui-providers') return null;
  if (route == '/settings') return UICapability.settingsShell;
  if (route.startsWith('/settings/')) return UICapability.settingsSection;
  if (route == '/extensions') return UICapability.extensionCenter;
  if (route.startsWith('/extensions/') ||
      route.startsWith('/extension/page/')) {
    return UICapability.extensionPage;
  }
  return UICapability.pageProvider;
}

/// Provider boundary applied to every built-in business route. Route-targeted
/// providers can replace one page, a route family, or an entire surface class
/// without adding a static GoRoute to the core router.
class RouteSurfaceProviderHost extends ConsumerWidget {
  const RouteSurfaceProviderHost({
    super.key,
    required this.route,
    required this.child,
  });

  final String route;
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (route.startsWith('/channels/')) return child;
    final capability = capabilityForBuiltinRoute(route);
    if (capability == null) return child;
    final snapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final selected = UIPageProviderRegistry.resolve(
      snapshot,
      capability: capability,
      route: route,
    );
    return UIProviderHost(
      capability: capability,
      providerId: selected?.providerId,
      context: <String, dynamic>{
        'route': route,
        'pathParameters': providerRoutePathParameters(route),
        'surface': capability,
        'surfaceId': canonicalUISurfaceId(route),
        'routeAliases': uiRouteAliases(route).toList(growable: false),
        ...providerRouteIdentity(route),
      },
      fallback: child,
    );
  }
}

Map<String, String> providerRoutePathParameters(String route) {
  final segments = Uri.tryParse(route)?.pathSegments ?? const <String>[];
  if (segments.length >= 3 && segments.first == 'characters') {
    return <String, String>{'id': segments[1]};
  }
  return const <String, String>{};
}

Map<String, String> providerRouteIdentity(String route) {
  final parameters = providerRoutePathParameters(route);
  final characterId = (parameters['characterId'] ?? parameters['id'] ?? '')
      .trim();
  final conversationId = (parameters['conversationId'] ?? '').trim();
  return <String, String>{
    if (characterId.isNotEmpty) 'characterId': characterId,
    if (conversationId.isNotEmpty) 'conversationId': conversationId,
  };
}
