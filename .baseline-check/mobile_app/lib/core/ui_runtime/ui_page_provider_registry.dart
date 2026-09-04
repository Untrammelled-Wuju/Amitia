import 'routing/ui_surface_catalog.dart';
import 'ui_provider.dart';

abstract final class UIPageProviderRegistry {
  static UIProviderDefinition? resolve(
    UIProviderSnapshot? snapshot, {
    required String capability,
    required String route,
  }) {
    if (snapshot == null) return null;
    final platform = currentUIPlatform();

    final candidates = snapshot.providers
        .where((provider) =>
            provider.enabled &&
            !provider.builtin &&
            provider.capability == capability &&
            provider.compatibleWith(snapshot.context, platform) &&
            _hasSelectors(provider) &&
            _matchesRoute(provider, route))
        .toList()
      ..sort((a, b) {
        final priority = b.priority.compareTo(a.priority);
        if (priority != 0) return priority;
        return a.providerId.compareTo(b.providerId);
      });
    if (candidates.isNotEmpty) return candidates.first;

    final selected = snapshot.resolve(capability);
    if (selected != null &&
        selected.enabled &&
        selected.compatibleWith(snapshot.context, platform) &&
        (selected.builtin || !_hasSelectors(selected))) {
      return selected;
    }

    for (final provider in snapshot.providers) {
      if (provider.builtin &&
          provider.enabled &&
          provider.capability == capability &&
          provider.compatibleWith(snapshot.context, platform)) {
        return provider;
      }
    }
    return null;
  }

  static bool _hasSelectors(UIProviderDefinition provider) {
    final metadata = provider.metadata;
    return ((metadata['routes'] as List?)?.isNotEmpty ?? false) ||
        ((metadata['routePatterns'] as List?)?.isNotEmpty ?? false) ||
        ((metadata['surfaces'] as List?)?.isNotEmpty ?? false) ||
        ((metadata['surfacePatterns'] as List?)?.isNotEmpty ?? false);
  }

  static bool _matchesRoute(UIProviderDefinition provider, String route) {
    final metadata = provider.metadata;
    final routeSelectors = <String>[
      ...((metadata['routes'] as List?) ?? const <dynamic>[]).whereType<String>(),
      ...((metadata['routePatterns'] as List?) ?? const <dynamic>[]).whereType<String>(),
    ].map((e) => e.trim()).where((e) => e.isNotEmpty);
    final aliases = uiRouteAliases(route);
    if (routeSelectors.any(
      (pattern) => aliases.any((candidate) => _matchesPattern(candidate, pattern)),
    )) {
      return true;
    }

    final surfaceSelectors = <String>[
      ...((metadata['surfaces'] as List?) ?? const <dynamic>[]).whereType<String>(),
      ...((metadata['surfacePatterns'] as List?) ?? const <dynamic>[]).whereType<String>(),
    ].map((e) => e.trim()).where((e) => e.isNotEmpty);
    final surfaceId = canonicalUISurfaceId(route);
    return surfaceSelectors.any((pattern) => _matchesSurfacePattern(surfaceId, pattern));
  }

  static bool _matchesSurfacePattern(String surfaceId, String pattern) {
    final clean = pattern.trim();
    if (clean.isEmpty) return false;
    if (clean == '*' || clean == surfaceId) return true;
    if (clean.endsWith('.*')) {
      final prefix = clean.substring(0, clean.length - 2);
      return surfaceId == prefix || surfaceId.startsWith('$prefix.');
    }
    return false;
  }

  static bool _matchesPattern(String route, String pattern) {
    final clean = pattern.trim();
    if (clean.isEmpty) return false;
    if (clean == '*' || clean == '/*') return true;
    if (clean == route) return true;
    if (clean.endsWith('/*')) {
      final prefix = clean.substring(0, clean.length - 2);
      return route == prefix || route.startsWith('$prefix/');
    }
    if (!clean.contains(':')) return false;
    final routeParts = route.split('/').where((part) => part.isNotEmpty).toList();
    final patternParts = clean.split('/').where((part) => part.isNotEmpty).toList();
    if (routeParts.length != patternParts.length) return false;
    for (var index = 0; index < patternParts.length; index++) {
      final part = patternParts[index];
      if (part == '*' || part.startsWith(':')) continue;
      if (part != routeParts[index]) return false;
    }
    return true;
  }

}
