enum DrawerPanel { main }

enum MainDrawerItem {
  chat,
  characters,
  memory,
  continuity,
  devices,
  extensions,
  workshop,
  none,
}

enum MoreDrawerItem { none }

enum DrawerNavigationAction { none, push, replace }

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

DrawerNavigationAction resolveDrawerNavigationAction({
  required String currentLocation,
  required String targetLocation,
}) {
  if (currentLocation == targetLocation) return DrawerNavigationAction.none;
  final currentPath = Uri.tryParse(currentLocation)?.path ?? currentLocation;
  final targetPath = Uri.tryParse(targetLocation)?.path ?? targetLocation;
  if (currentPath == '/chat' && targetPath == '/chat') {
    return DrawerNavigationAction.replace;
  }
  return DrawerNavigationAction.push;
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
  } else if (isRouteFamily(location, '/continuity')) {
    mainItem = MainDrawerItem.continuity;
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
    settingsSelected = true;
  }

  return DrawerRouteState(
    initialPanel: DrawerPanel.main,
    mainItem: mainItem,
    moreItem: MoreDrawerItem.none,
    settingsSelected: settingsSelected,
  );
}
