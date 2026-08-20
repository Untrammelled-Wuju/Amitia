import 'ui_provider.dart';

abstract final class UIPageProviderRegistry {
  static UIProviderDefinition? resolve(
    UIProviderSnapshot? snapshot, {
    required String capability,
    required String route,
  }) {
    if (snapshot == null) return null;
    final platform = currentUIPlatform();
    final selected = snapshot.resolve(capability);
    if (selected != null &&
        selected.enabled &&
        selected.entryFor(platform) != null &&
        _matchesRoute(selected, route)) {
      return selected;
    }

    final candidates = snapshot.providers
        .where((provider) =>
            provider.enabled &&
            !provider.builtin &&
            provider.capability == capability &&
            provider.entryFor(platform) != null &&
            _matchesRoute(provider, route))
        .toList()
      ..sort((a, b) {
        final priority = b.priority.compareTo(a.priority);
        if (priority != 0) return priority;
        return a.providerId.compareTo(b.providerId);
      });
    if (candidates.isNotEmpty) return candidates.first;

    for (final provider in snapshot.providers) {
      if (provider.builtin &&
          provider.enabled &&
          provider.capability == capability &&
          provider.entryFor(platform) != null) {
        return provider;
      }
    }
    return null;
  }

  static bool _matchesRoute(UIProviderDefinition provider, String route) {
    final metadata = provider.metadata;
    final routes = ((metadata['routes'] as List?) ?? const <dynamic>[])
        .map((e) => e.toString())
        .where((e) => e.isNotEmpty)
        .toList();
    final patterns = ((metadata['routePatterns'] as List?) ?? const <dynamic>[])
        .map((e) => e.toString())
        .where((e) => e.isNotEmpty)
        .toList();
    if (routes.isEmpty && patterns.isEmpty) return true;
    if (routes.any((candidate) => _routeFamily(route, candidate))) return true;
    return patterns.any((pattern) => _globMatch(route, pattern));
  }

  static bool _routeFamily(String route, String root) {
    if (root == '*') return true;
    if (!root.startsWith('/')) return false;
    return route == root || route.startsWith('$root/');
  }

  static bool _globMatch(String route, String pattern) {
    if (pattern == '*' || pattern == '/*') return true;
    final escaped = RegExp.escape(pattern).replaceAll(r'\*', '.*');
    return RegExp('^$escaped\$').hasMatch(route);
  }
}
