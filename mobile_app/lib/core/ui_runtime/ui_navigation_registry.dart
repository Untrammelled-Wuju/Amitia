import 'package:flutter/material.dart';

import '../../app/app_routes.dart';
import 'ui_provider.dart';
import 'ui_icon_registry.dart';
import 'ui_route_registry.dart';

enum UINavigationPanel { main, more }

@immutable
class UINavigationItem {
  const UINavigationItem({
    required this.id,
    required this.label,
    required this.route,
    required this.icon,
    required this.panel,
    this.group = '',
    this.order = 0,
    this.routePrefixes = const <String>[],
    this.builtin = false,
    this.extensionId,
  });

  final String id;
  final String label;
  final String route;
  final IconData icon;
  final UINavigationPanel panel;
  final String group;
  final int order;
  final List<String> routePrefixes;
  final bool builtin;
  final String? extensionId;

  bool matches(String location) {
    final prefixes = routePrefixes.isEmpty ? <String>[route] : routePrefixes;
    return prefixes.any((prefix) => location == prefix || location.startsWith('$prefix/'));
  }
}

abstract final class UINavigationRegistry {
  static const List<UINavigationItem> builtinItems = <UINavigationItem>[
    UINavigationItem(
      id: 'builtin.chat',
      label: '对话',
      route: AppRoutes.chat,
      icon: Icons.chat_bubble_outline,
      panel: UINavigationPanel.main,
      order: 10,
      routePrefixes: <String>['/chat', '/conversations'],
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.characters',
      label: '角色',
      route: AppRoutes.characters,
      icon: Icons.people_outline,
      panel: UINavigationPanel.main,
      order: 20,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.memory',
      label: '记忆',
      route: AppRoutes.memory,
      icon: Icons.memory_outlined,
      panel: UINavigationPanel.main,
      order: 30,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.devices',
      label: '设备',
      route: AppRoutes.settingsDevices,
      icon: Icons.devices_outlined,
      panel: UINavigationPanel.main,
      order: 40,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.extensions',
      label: '扩展中心',
      route: AppRoutes.extensions,
      icon: Icons.extension_outlined,
      panel: UINavigationPanel.main,
      order: 50,
      routePrefixes: <String>['/extensions', '/extension'],
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.workshop',
      label: '创意工坊',
      route: AppRoutes.workshop,
      icon: Icons.brush_outlined,
      panel: UINavigationPanel.main,
      order: 60,
      builtin: true,
    ),
  ];

  static List<UINavigationItem> resolve(UIProviderSnapshot? snapshot) {
    final items = <UINavigationItem>[...builtinItems];
    if (snapshot == null) return _sorted(items);
    final platform = currentUIPlatform();

    // Navigation items are registry contributions: every enabled compatible
    // provider may add items. app.navigation selection still controls the whole
    // navigation surface through UIProviderHost; it does not suppress other
    // providers' item contributions.
    final sources = snapshot.providers
        .where((provider) =>
            provider.enabled &&
            !provider.builtin &&
            (provider.capability == UICapability.appNavigation ||
                provider.capability == UICapability.routeRegistry) &&
            provider.compatibleWith(snapshot.context, platform) &&
            provider.metadata['navigationItems'] is List)
        .toList()
      ..sort((a, b) {
        final priority = b.priority.compareTo(a.priority);
        return priority != 0 ? priority : a.providerId.compareTo(b.providerId);
      });

    final effectiveRoutes = effectiveProviderRouteKeys(snapshot);
    final effectiveExtensionRoutes = effectiveExtensionRouteKeys(snapshot);
    final seenIds = <String>{};
    for (final provider in sources) {
      final raw = provider.metadata['navigationItems'];
      if (raw is! List) continue;
      for (final value in raw) {
        if (value is! Map) continue;
        final row = value.cast<String, dynamic>();
        final id = (row['id'] ?? '').toString().trim();
        final label = (row['label'] ?? '').toString().trim();
        final route = (row['route'] ?? '').toString().trim();
        final compositeId = '${provider.extensionId}:$id';
        if (id.isEmpty ||
            label.isEmpty ||
            !route.startsWith('/') ||
            route == '/') {
          continue;
        }
        if (provider.capability == UICapability.routeRegistry) {
          if (!effectiveRoutes.contains('${provider.providerId}\u0000$route')) continue;
        } else if (!isProtectedProviderRoutePath(route) &&
            !effectiveExtensionRoutes.contains('${provider.extensionId}\u0000$route')) {
          continue;
        }
        if (!seenIds.add(compositeId)) continue;
        final rawOrder = row['order'];
        final rawPrefixes = row['routePrefixes'] ?? row['match'];
        final baseIcon = UIIconRegistry.iconFromName((row['icon'] ?? 'extension').toString());
        items.add(
          UINavigationItem(
            id: compositeId,
            label: label,
            route: route,
            icon: baseIcon,
            panel: (row['panel'] ?? '').toString() == 'main' ||
                    (row['panel'] == null && row['mobile'] == true)
                ? UINavigationPanel.main
                : UINavigationPanel.more,
            group: (row['group'] ?? '扩展').toString().trim().isEmpty
                ? '扩展'
                : (row['group'] ?? '扩展').toString().trim(),
            order: rawOrder is num ? rawOrder.toInt() : 1000,
            routePrefixes: (rawPrefixes is List ? rawPrefixes : const <dynamic>[])
                .map((e) => e.toString().trim())
                .where((e) => e.startsWith('/') && e != '/')
                .toList(),
            extensionId: provider.extensionId,
          ),
        );
      }
    }

    final withIconOverrides = items
        .map((item) => UINavigationItem(
              id: item.id,
              label: item.label,
              route: item.route,
              icon: UIIconRegistry.resolve(snapshot, item.id, item.icon),
              panel: item.panel,
              group: item.group,
              order: item.order,
              routePrefixes: item.routePrefixes,
              builtin: item.builtin,
              extensionId: item.extensionId,
            ))
        .toList();
    return _sorted(withIconOverrides);
  }

  static List<UINavigationItem> _sorted(List<UINavigationItem> items) {
    items.sort((a, b) {
      final panel = a.panel.index.compareTo(b.panel.index);
      if (panel != 0) return panel;
      final group = a.group.compareTo(b.group);
      if (group != 0) return group;
      final order = a.order.compareTo(b.order);
      if (order != 0) return order;
      return a.id.compareTo(b.id);
    });
    return items;
  }

  static IconData iconFromName(String raw) => UIIconRegistry.iconFromName(raw);
}
