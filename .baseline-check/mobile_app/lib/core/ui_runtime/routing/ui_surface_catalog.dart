class _RouteAliasRule {
  const _RouteAliasRule(this.canonicalPrefix, this.aliasPrefix, {this.exact = false});

  final String canonicalPrefix;
  final String aliasPrefix;
  final bool exact;

  bool matches(String route, String prefix) {
    if (exact) return route == prefix;
    return route == prefix || route.startsWith('$prefix/');
  }

  String swap(String route, String from, String to) {
    if (exact) return to;
    return '$to${route.substring(from.length)}';
  }
}

const _routeAliasRules = <_RouteAliasRule>[
  _RouteAliasRule('/characters', '/character'),
  _RouteAliasRule('/workshop', '/creative-workshop'),
  _RouteAliasRule('/channels/wechat', '/wechat', exact: true),
  _RouteAliasRule('/channels/qq', '/qq', exact: true),
  _RouteAliasRule('/settings/devices', '/devices'),
  _RouteAliasRule('/developer/kernel', '/kernel'),
  _RouteAliasRule('/memory/manager', '/memory-manager', exact: true),
  _RouteAliasRule('/memory/timeline', '/memory-timeline', exact: true),
  _RouteAliasRule('/memory/episodic', '/episodic', exact: true),
  _RouteAliasRule('/memory/graph', '/graph', exact: true),
  _RouteAliasRule('/memory/profiles', '/profiles', exact: true),
  _RouteAliasRule('/memory/world-book', '/world-book', exact: true),
  _RouteAliasRule('/chat-logs', '/logs', exact: true),
  _RouteAliasRule('/chat-import', '/import', exact: true),
  _RouteAliasRule('/settings/runtime-mode', '/runtime-mode', exact: true),
  _RouteAliasRule('/settings/long-running', '/long-running', exact: true),
  _RouteAliasRule('/settings/decision-viz', '/decision-viz', exact: true),
  _RouteAliasRule('/settings/privacy-scan', '/privacy-scan', exact: true),
  _RouteAliasRule('/settings/storage', '/storage', exact: true),
  _RouteAliasRule('/settings/user', '/user-settings', exact: true),
];

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

Set<String> uiRouteAliases(String rawRoute) {
  final route = normalizeUIRoute(rawRoute);
  final aliases = <String>{route};
  for (final rule in _routeAliasRules) {
    if (rule.matches(route, rule.canonicalPrefix)) {
      aliases.add(rule.swap(route, rule.canonicalPrefix, rule.aliasPrefix));
    }
    if (rule.matches(route, rule.aliasPrefix)) {
      aliases.add(rule.swap(route, rule.aliasPrefix, rule.canonicalPrefix));
    }
  }
  return aliases;
}

String canonicalUISurfaceId(String rawRoute) {
  final aliases = uiRouteAliases(rawRoute);
  bool hasPrefix(String prefix) => aliases.any(
        (route) => route == prefix || route.startsWith('$prefix/'),
      );

  if (aliases.contains('/characters') || aliases.contains('/character')) {
    return 'surface.character.list';
  }
  if (hasPrefix('/characters') || hasPrefix('/character')) {
    return 'surface.character.detail';
  }
  if (aliases.contains('/memory/manager') || aliases.contains('/memory-manager')) {
    return 'surface.memory.manager';
  }
  if (hasPrefix('/memory') ||
      aliases.any((route) => const {
            '/memory-timeline',
            '/episodic',
            '/graph',
            '/profiles',
            '/world-book',
          }.contains(route))) {
    return 'surface.memory.detail';
  }
  if (hasPrefix('/workshop') || hasPrefix('/creative-workshop')) {
    return 'surface.workshop';
  }
  if (aliases.contains('/channels/wechat') || aliases.contains('/wechat')) {
    return 'surface.channel.wechat';
  }
  if (aliases.contains('/channels/qq') || aliases.contains('/qq')) {
    return 'surface.channel.qq';
  }
  if (hasPrefix('/settings/devices') || hasPrefix('/devices')) {
    return 'surface.settings.devices';
  }
  if (hasPrefix('/developer/kernel') || hasPrefix('/kernel')) {
    return 'surface.kernel';
  }
  if (hasPrefix('/settings')) return 'surface.settings.section';
  if (hasPrefix('/extensions') || hasPrefix('/extension/page')) {
    return 'surface.extension';
  }
  return 'surface.page';
}
