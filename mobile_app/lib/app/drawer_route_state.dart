enum DrawerPanel {
  main,
  more,
}

enum MainDrawerItem {
  chat,
  tasks,
  characters,
  memory,
  devices,
  more,
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
  var moreItem = MoreDrawerItem.none;
  var settingsSelected = false;
  var initialPanel = DrawerPanel.main;

  if (isRouteFamily(location, '/chat') ||
      isRouteFamily(location, '/conversations')) {
    mainItem = MainDrawerItem.chat;
  } else if (isRouteFamily(location, '/agent')) {
    mainItem = MainDrawerItem.tasks;
  } else if (isRouteFamily(location, '/characters')) {
    mainItem = MainDrawerItem.characters;
  } else if (isRouteFamily(location, '/memory')) {
    mainItem = MainDrawerItem.memory;
  } else if (isRouteFamily(location, '/settings/devices')) {
    mainItem = MainDrawerItem.devices;
    settingsSelected = true;
  } else if (isRouteFamily(location, '/settings')) {
    settingsSelected = true;
  } else {
    if (isRouteFamily(location, '/extensions') ||
        isRouteFamily(location, '/extension')) {
      moreItem = MoreDrawerItem.extensions;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/workshop')) {
      moreItem = MoreDrawerItem.workshop;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/game-center')) {
      moreItem = MoreDrawerItem.gameCenter;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/channels')) {
      moreItem = MoreDrawerItem.channels;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/desktop-pet')) {
      moreItem = MoreDrawerItem.desktopPet;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/reminders')) {
      moreItem = MoreDrawerItem.reminders;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/dashboard')) {
      moreItem = MoreDrawerItem.dashboard;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/chat-logs')) {
      moreItem = MoreDrawerItem.chatLogs;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/chat-import')) {
      moreItem = MoreDrawerItem.chatImport;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/emotes')) {
      moreItem = MoreDrawerItem.emotes;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    } else if (isRouteFamily(location, '/developer')) {
      moreItem = MoreDrawerItem.developer;
      initialPanel = DrawerPanel.more;
      mainItem = MainDrawerItem.more;
    }
  }

  return DrawerRouteState(
    initialPanel: initialPanel,
    mainItem: mainItem,
    moreItem: moreItem,
    settingsSelected: settingsSelected,
  );
}
