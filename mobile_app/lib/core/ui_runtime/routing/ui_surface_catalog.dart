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
  if (route == '/chat' || _hasPrefix(route, '/chat')) {
    return 'surface.conversation';
  }
  if (route == '/tasks' || _hasPrefix(route, '/tasks')) {
    return 'surface.tasks';
  }
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
  if (_hasPrefix(route, '/workspaces')) {
    return 'surface.workspace';
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
  if (_hasPrefix(route, '/extensions') ||
      _hasPrefix(route, '/extension/page')) {
    return 'surface.extension';
  }
  return 'surface.page';
}

class MobileUISurfaceSpec {
  const MobileUISurfaceSpec({
    required this.id,
    required this.providerCapabilities,
    required this.slotIds,
  });

  final String id;
  final List<String> providerCapabilities;
  final List<String> slotIds;

  bool supportsSlot(String slotId) => slotIds.contains(slotId.trim());

  bool supportsProviderCapability(String capability) =>
      providerCapabilities.contains(capability.trim());
}

const mobileUISurfaceSpecs = <MobileUISurfaceSpec>[
  MobileUISurfaceSpec(
    id: 'surface.app',
    providerCapabilities: <String>[
      'app.shell',
      'app.navigation',
      'app.workspace',
      'app.workspace.sidebar',
      'route.registry',
      'ui.theme',
      'ui.tokens',
      'ui.icons',
      'ui.components',
    ],
    slotIds: <String>['root', 'app.shell.quota_notice', 'app.shell.bottom'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.conversation',
    providerCapabilities: <String>[
      'conversation.shell',
      'conversation.header',
      'conversation.messages',
      'conversation.message_renderer',
      'conversation.sidebar',
      'conversation.composer',
      'conversation.overlay',
    ],
    slotIds: <String>[
      'chat.header.action',
      'chat.header.leading',
      'chat.header.lineage',
      'chat.header.corner',
      'chat.sidebar.panel',
      'chat.profile_summary.panel',
      'chat.memory_context.panel',
      'chat.message.action',
      'chat.message.renderer',
      'chat.message.images',
      'chat.message.user.images',
      'chat.message.custom_renderer',
      'chat.message.attachment_renderer',
      'chat.message.badge',
      'chat.conversation.node',
      'chat.composer.action',
      'chat.composer.attachment',
      'chat.composer.hint',
      'chat.composer.dock.top',
      'chat.composer.dock.bottom',
      'chat.composer.left',
      'chat.composer.right',
      'chat.composer.overlay',
      'chat.composer.model',
      'chat.composer.model.label',
      'chat.composer.model.options',
      'chat.composer.permission',
      'chat.empty_state.card',
      'chat.status.item',
    ],
  ),
  MobileUISurfaceSpec(
    id: 'surface.character.list',
    providerCapabilities: <String>['character.shell'],
    slotIds: <String>['character.sidebar.card'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.character.detail',
    providerCapabilities: <String>['character.detail'],
    slotIds: <String>[
      'character.detail.tab',
      'character.detail.action',
      'character.sidebar.card',
    ],
  ),
  MobileUISurfaceSpec(
    id: 'surface.memory.hub',
    providerCapabilities: <String>['memory.shell', 'memory.detail'],
    slotIds: <String>['memory.page.section'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.memory.detail',
    providerCapabilities: <String>['memory.shell', 'memory.detail'],
    slotIds: <String>['memory.page.section'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.tasks',
    providerCapabilities: <String>['page.provider'],
    slotIds: <String>['tasks.page.section'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.workshop',
    providerCapabilities: <String>['page.provider'],
    slotIds: <String>[],
  ),
  MobileUISurfaceSpec(
    id: 'surface.kernel',
    providerCapabilities: <String>['page.provider'],
    slotIds: <String>[],
  ),
  MobileUISurfaceSpec(
    id: 'surface.settings.section',
    providerCapabilities: <String>['settings.shell', 'settings.section'],
    slotIds: <String>[
      'system.status.item',
      'system.settings.section',
      'system.diagnostics.tab',
      'extension.settings.page',
      'extension.settings.section',
    ],
  ),
  MobileUISurfaceSpec(
    id: 'surface.settings.devices',
    providerCapabilities: <String>['settings.section'],
    slotIds: <String>[
      'system.status.item',
      'system.settings.section',
      'system.diagnostics.tab',
    ],
  ),
  MobileUISurfaceSpec(
    id: 'surface.extension',
    providerCapabilities: <String>['extension.center', 'extension.page'],
    slotIds: <String>[
      'extension.center.header.action',
      'extension.center.card.badge',
      'extension.detail.tab',
      'extension.detail.action',
      'extension.settings.page',
      'extension.settings.section',
    ],
  ),
  MobileUISurfaceSpec(
    id: 'surface.workspace',
    providerCapabilities: <String>[
      'app.workspace',
      'app.workspace.sidebar',
      'workspace.tab',
    ],
    slotIds: <String>['provider.app.workspace.sidebar'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.page',
    providerCapabilities: <String>['page.provider', 'route.registry'],
    slotIds: <String>['root'],
  ),
  MobileUISurfaceSpec(
    id: 'surface.overlay',
    providerCapabilities: <String>['conversation.overlay'],
    slotIds: <String>[
      'ui.portal.overlay',
      'ui.portal.modal',
      'ui.portal.drawer',
      'ui.portal.toast',
      'ui.sidebar.right.pane.tab',
      'ui.sidebar.right.pane.tab.title',
    ],
  ),
];

MobileUISurfaceSpec? mobileUISurfaceSpec(String surfaceId) {
  final id = surfaceId.trim();
  for (final spec in mobileUISurfaceSpecs) {
    if (spec.id == id) return spec;
  }
  return null;
}

MobileUISurfaceSpec? mobileUISurfaceForRoute(String rawRoute) =>
    mobileUISurfaceSpec(canonicalUISurfaceId(rawRoute));

List<String> mobileUISelectionSlotsForRoute(String rawRoute) =>
    mobileUISurfaceForRoute(rawRoute)?.slotIds ?? const <String>[];
