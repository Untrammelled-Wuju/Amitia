enum DrawerPanel {
  main,
  more,
}

enum MainDrawerItem {
  chat,
  characters,
  memory,
  devices,
  extensions,
  workshop,
  none,
}

enum MoreDrawerItem {
  extensions,
  workshop,
  gameCenter,
  channels,
  desktopPet,
  reminders,
  dashboard,
  chatLogs,
  chatImport,
  emotes,
  developer,
  none,
}

class DrawerRouteState {
  const DrawerRouteState({
    required this.initialPanel,
    required this.mainItem,
    required this.moreItem,
    required this.settingsSelected,
  });

  final DrawerPanel initialPanel;
  final MainDrawerItem mainItem;
  final MoreDrawerItem moreItem;
  final bool settingsSelected;
}

bool isRouteFamily(String location, String root) {
  return location == root || location.startsWith('$root/');
}

DrawerRouteState resolveDrawerRouteState(String location) {
  var mainItem = MainDrawerItem.none;
  var settingsSelected = false;

  if (isRouteFamily(location, '/chat') ||
      isRouteFamily(location, '/conversations')) {
    mainItem = MainDrawerItem.chat;
  } else if (isRouteFamily(location, '/characters')) {
    mainItem = MainDrawerItem.characters;
  } else if (isRouteFamily(location, '/memory')) {
    mainItem = MainDrawerItem.memory;
  } else if (isRouteFamily(location, '/settings/devices')) {
    mainItem = MainDrawerItem.devices;
  } else if (isRouteFamily(location, '/extensions') ||
      isRouteFamily(location, '/extension')) {
    mainItem = MainDrawerItem.extensions;
  } else if (isRouteFamily(location, '/workshop')) {
    mainItem = MainDrawerItem.workshop;
  } else if (isRouteFamily(location, '/settings') ||
      isRouteFamily(location, '/channels') ||
      isRouteFamily(location, '/reminders') ||
      isRouteFamily(location, '/dashboard') ||
      isRouteFamily(location, '/chat-logs') ||
      isRouteFamily(location, '/chat-import') ||
      isRouteFamily(location, '/emotes')) {
    // These pages are reached from Settings in the mobile information
    // architecture, so the account/settings entry remains highlighted.
    settingsSelected = true;
  }

  return DrawerRouteState(
    initialPanel: DrawerPanel.main,
    mainItem: mainItem,
    moreItem: MoreDrawerItem.none,
    settingsSelected: settingsSelected,
  );
}
