enum MobilePluginSurfaceKind {
  hostRuntime,
  schema,
  web,
  native,
  none,
}

MobilePluginSurfaceKind resolveMobilePluginSurface({
  required String kind,
  String? entryType,
  String? runtimeId,
}) {
  if (runtimeId?.startsWith('host.') == true) {
    return MobilePluginSurfaceKind.hostRuntime;
  }
  if (entryType == 'schema_renderer') {
    return MobilePluginSurfaceKind.schema;
  }
  if (entryType == 'web_restricted' || entryType == 'web_isolated') {
    return MobilePluginSurfaceKind.web;
  }
  if (entryType == 'host_native' || entryType == 'builtin_native') {
    return MobilePluginSurfaceKind.native;
  }
  if (const {
    'schema_page',
    'settings_section',
    'panel',
    'card',
    'detail_section',
    'chat_sidebar',
  }.contains(kind)) {
    return MobilePluginSurfaceKind.schema;
  }
  if (kind == 'web_page' || kind == 'message_renderer') {
    return MobilePluginSurfaceKind.web;
  }
  if (const {
    'action',
    'menu_item',
    'toolbar_item',
    'status_item',
    'message_action',
    'composer_action',
    'desktop_command',
    'badge',
  }.contains(kind)) {
    return MobilePluginSurfaceKind.native;
  }
  return MobilePluginSurfaceKind.none;
}
