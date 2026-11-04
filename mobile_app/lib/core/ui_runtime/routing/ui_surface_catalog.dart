String normalizeUIRoute(String raw) {
  final queryIndex = raw.indexOf('?');
  final hashIndex = raw.indexOf('#');
  var end = raw.length;
  if (queryIndex >= 0 && queryIndex < end) end = queryIndex;
  if (hashIndex >= 0 && hashIndex < end) end = hashIndex;
  var route = raw.substring(0, end).trim();
  if (route.isEmpty) return '/';
  if (!route.startsWith('/')) route = '/$route';
  while (route.length > 1 && route.endsWith('/')) {
    route = route.substring(0, route.length - 1);
  }
  return route;
}

bool _hasPrefix(String route, String prefix) {
  return route == prefix || route.startsWith('$prefix/');
}

String canonicalUISurfaceId(String rawRoute) {
  final route = normalizeUIRoute(rawRoute);
  if (route == '/characters') {
    return 'surface.character.list';
  }
  if (_hasPrefix(route, '/characters') || _hasPrefix(route, '/character')) {
    return 'surface.character.detail';
  }
  if (route == '/memory-hub') {
    return 'surface.memory.hub';
  }
  if (_hasPrefix(route, '/memory') ||
      const {
        '/memory-timeline',
        '/episodic',
        '/graph',
        '/profiles',
        '/world-book',
      }.contains(route)) {
    return 'surface.memory.detail';
  }
  if (_hasPrefix(route, '/creative-workshop')) {
    return 'surface.workshop';
  }
  if (_hasPrefix(route, '/settings/devices')) {
    return 'surface.settings.devices';
  }
  if (_hasPrefix(route, '/kernel')) {
    return 'surface.kernel';
  }
  if (_hasPrefix(route, '/settings') ||
      const {
        '/long-running',
        '/decision-viz',
        '/privacy-scan',
        '/storage',
      }.contains(route)) {
    return 'surface.settings.section';
  }
  if (_hasPrefix(route, '/extensions') || _hasPrefix(route, '/extension/page')) {
    return 'surface.extension';
  }
  return 'surface.page';
}
