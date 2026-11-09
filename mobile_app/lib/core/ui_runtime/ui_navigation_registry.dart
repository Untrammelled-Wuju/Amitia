import 'package:flutter/material.dart';

import '../../app/app_routes.dart';
import 'ui_provider.dart';
import 'ui_icon_registry.dart';
import 'ui_route_registry.dart';
import 'channel_presentation.dart';

enum UINavigationPanel { main, more }

enum UINavigationPlacement { primary, settings, profile, workspace, contextual }

@immutable
class UINavigationItem {
  const UINavigationItem({
    required this.id,
    required this.label,
    required this.route,
    required this.icon,
    required this.panel,
    this.placement = UINavigationPlacement.contextual,
    this.group = '',
    this.order = 0,
    this.routePrefixes = const <String>[],
    this.builtin = false,
    this.profile = false,
    this.tabLabel = '',
    this.extensionId,
  });

  final String id;
  final String label;
  final String route;
  final IconData icon;
  final UINavigationPanel panel;
  final UINavigationPlacement placement;
  final String group;
  final int order;
  final List<String> routePrefixes;
  final bool builtin;
  final bool profile;
  final String tabLabel;
  final String? extensionId;

  bool matches(String location) {
    final prefixes = routePrefixes.isEmpty ? <String>[route] : routePrefixes;
    return prefixes.any(
      (prefix) => location == prefix || location.startsWith('$prefix/'),
    );
  }
}

abstract final class UINavigationRegistry {
  static const List<UINavigationItem> builtinItems = <UINavigationItem>[
    UINavigationItem(
      id: 'core.tasks',
      label: '任务',
      route: AppRoutes.tasks,
      icon: Icons.task_alt_outlined,
      panel: UINavigationPanel.main,
      placement: UINavigationPlacement.primary,
      group: 'tasks',
      order: 69,
      builtin: true,
      profile: true,
      tabLabel: '任务',
    ),
    UINavigationItem(
      id: 'core.memory',
      label: '记忆',
      route: AppRoutes.memoryHub,
      icon: Icons.psychology_outlined,
      panel: UINavigationPanel.main,
      placement: UINavigationPlacement.primary,
      group: 'memory',
      order: 50,
      builtin: true,
      profile: true,
      tabLabel: '记忆',
    ),
    UINavigationItem(
      id: 'core.characters',
      label: '角色',
      route: AppRoutes.characters,
      icon: Icons.people_outline,
      panel: UINavigationPanel.more,
      placement: UINavigationPlacement.profile,
      group: 'character',
      order: 20,
      builtin: true,
      profile: true,
    ),
  ];

  static const UINavigationItem channelMessagesItem = UINavigationItem(
    id: 'builtin.channelMessages',
    label: '渠道消息',
    route: AppRoutes.channelMessages,
    icon: Icons.forum_outlined,
    panel: UINavigationPanel.main,
    placement: UINavigationPlacement.primary,
    order: 55,
    builtin: true,
    profile: true,
  );

  static List<UINavigationItem> resolve(UIProviderSnapshot? snapshot) {
    final items = <UINavigationItem>[
      ...builtinItems,
      if (ChannelPresentationRegistry.resolve(snapshot).isNotEmpty)
        channelMessagesItem,
    ];
    if (snapshot == null) return _sorted(items);
    final platform = currentUIPlatform();

    final sources =
        snapshot.providers
            .where(
              (provider) =>
                  provider.enabled &&
                  !provider.builtin &&
                  (provider.capability == UICapability.appNavigation ||
                      provider.capability == UICapability.routeRegistry) &&
                  provider.compatibleWith(snapshot.context, platform) &&
                  provider.metadata['navigationItems'] is List,
            )
            .toList()
          ..sort((a, b) {
            final priority = b.priority.compareTo(a.priority);
            return priority != 0
                ? priority
                : a.providerId.compareTo(b.providerId);
          });

    final effectiveProviderRoutes = effectiveProviderRouteKeys(snapshot);
    final effectiveExtensionRoutes = effectiveExtensionRouteKeys(snapshot);
    final seenIds = <String>{};
    final occupiedRoutes = items.map((item) => item.route).toSet();
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
          if (!_hasEffectiveRoute(
            effectiveProviderRoutes,
            provider.providerId,
            route,
          )) {
            continue;
          }
        } else if (!isProtectedProviderRoutePath(route) &&
            !_hasEffectiveRoute(
              effectiveExtensionRoutes,
              provider.extensionId,
              route,
            )) {
          continue;
        }
        if (!seenIds.add(compositeId)) continue;
        if (occupiedRoutes.contains(route)) continue;
        final rawOrder = row['order'];
        final rawPrefixes = row['routePrefixes'] ?? row['match'];
        final baseIcon = UIIconRegistry.iconFromName(
          (row['icon'] ?? 'extension').toString(),
        );
        items.add(
          UINavigationItem(
            id: compositeId,
            label: label,
            route: route,
            icon: baseIcon,
            panel:
                (row['panel'] ?? '').toString() == 'main' ||
                    (row['panel'] == null &&
                        (row['mobile'] == true ||
                            (row['tabLabel'] ?? '')
                                .toString()
                                .trim()
                                .isNotEmpty))
                ? UINavigationPanel.main
                : UINavigationPanel.more,
            placement: _resolvePlacement(
              row,
              route,
              (row['group'] ?? '扩展').toString().trim(),
            ),
            group: (row['group'] ?? '扩展').toString().trim().isEmpty
                ? '扩展'
                : (row['group'] ?? '扩展').toString().trim(),
            order: rawOrder is num ? rawOrder.toInt() : 1000,
            routePrefixes:
                (rawPrefixes is List ? rawPrefixes : const <dynamic>[])
                    .map((e) => e.toString().trim())
                    .where((e) => e.startsWith('/') && e != '/')
                    .toList(),
            profile: row['profile'] == true || row['profileHub'] == true,
            tabLabel: (row['tabLabel'] ?? '').toString().trim(),
            extensionId: provider.extensionId,
          ),
        );
      }
    }

    final withIconOverrides = items
        .map(
          (item) => UINavigationItem(
            id: item.id,
            label: item.label,
            route: item.route,
            icon: UIIconRegistry.resolve(snapshot, item.id, item.icon),
            panel: item.panel,
            placement: item.placement,
            group: item.group,
            order: item.order,
            routePrefixes: item.routePrefixes,
            builtin: item.builtin,
            profile: item.profile,
            tabLabel: item.tabLabel,
            extensionId: item.extensionId,
          ),
        )
        .toList();
    return _sorted(withIconOverrides);
  }

  static bool _hasEffectiveRoute(
    Set<String> effectiveRoutes,
    String ownerId,
    String route,
  ) {
    final exact = '$ownerId\u0000$route';
    if (effectiveRoutes.contains(exact)) return true;
    final childPrefix = '$exact/';
    return effectiveRoutes.any((key) => key.startsWith(childPrefix));
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

  static List<UINavigationItem> resolvePrimary(UIProviderSnapshot? snapshot) {
    return resolve(snapshot)
        .where((item) => item.placement == UINavigationPlacement.primary)
        .toList(growable: false);
  }

  static List<UINavigationItem> resolveMobile(UIProviderSnapshot? snapshot) {
    return resolve(snapshot)
        .where(
          (item) =>
              item.panel == UINavigationPanel.main &&
              (item.placement == UINavigationPlacement.primary ||
                  item.placement == UINavigationPlacement.profile),
        )
        .toList(growable: false);
  }

  static UINavigationPlacement _resolvePlacement(
    Map<String, dynamic> row,
    String route,
    String group,
  ) {
    final explicit = (row['placement'] ?? row['navigationPlacement'] ?? '')
        .toString()
        .trim()
        .toLowerCase();
    for (final placement in UINavigationPlacement.values) {
      if (placement.name == explicit) return placement;
    }
    if (group == 'settings') return UINavigationPlacement.settings;
    if (route == '/settings' ||
        route.startsWith('/settings/') ||
        route == '/logs' ||
        route.startsWith('/logs/')) {
      return UINavigationPlacement.profile;
    }
    if (group == 'workspace' ||
        route == '/workspaces' ||
        route.startsWith('/workspaces/')) {
      return UINavigationPlacement.workspace;
    }
    if (row['profile'] == true ||
        row['profileHub'] == true ||
        row['mobile'] == true ||
        (row['panel'] ?? '').toString().trim() == 'main' ||
        (row['tabLabel'] ?? '').toString().trim().isNotEmpty) {
      return UINavigationPlacement.primary;
    }
    return UINavigationPlacement.contextual;
  }

  static IconData iconFromName(String raw) => UIIconRegistry.iconFromName(raw);
}
