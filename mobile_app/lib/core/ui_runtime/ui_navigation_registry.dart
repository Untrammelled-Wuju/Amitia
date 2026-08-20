import 'package:flutter/material.dart';

import '../../app/app_routes.dart';
import 'ui_provider.dart';

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
      id: 'builtin.tasks',
      label: '任务',
      route: AppRoutes.agent,
      icon: Icons.auto_awesome,
      panel: UINavigationPanel.main,
      order: 20,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.characters',
      label: '角色',
      route: AppRoutes.characters,
      icon: Icons.people_outline,
      panel: UINavigationPanel.main,
      order: 30,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.memory',
      label: '记忆',
      route: AppRoutes.memory,
      icon: Icons.memory,
      panel: UINavigationPanel.main,
      order: 40,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.devices',
      label: '设备',
      route: AppRoutes.settingsDevices,
      icon: Icons.devices_outlined,
      panel: UINavigationPanel.main,
      order: 50,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.extensions',
      label: '扩展中心',
      route: AppRoutes.extensions,
      icon: Icons.extension_outlined,
      panel: UINavigationPanel.more,
      group: '能力与扩展',
      order: 10,
      routePrefixes: <String>['/extensions', '/extension'],
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.workshop',
      label: '创意工坊',
      route: AppRoutes.workshop,
      icon: Icons.brush_outlined,
      panel: UINavigationPanel.more,
      group: '能力与扩展',
      order: 20,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.game-center',
      label: '游戏中心',
      route: AppRoutes.gameCenter,
      icon: Icons.sports_esports_outlined,
      panel: UINavigationPanel.more,
      group: '连接与体验',
      order: 10,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.channels',
      label: '渠道中心',
      route: AppRoutes.channels,
      icon: Icons.sync_alt,
      panel: UINavigationPanel.more,
      group: '连接与体验',
      order: 20,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.desktop-pet',
      label: '桌宠中心',
      route: AppRoutes.desktopPet,
      icon: Icons.pets_outlined,
      panel: UINavigationPanel.more,
      group: '连接与体验',
      order: 30,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.reminders',
      label: '日程与提醒',
      route: AppRoutes.reminders,
      icon: Icons.notifications_active_outlined,
      panel: UINavigationPanel.more,
      group: '连接与体验',
      order: 40,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.dashboard',
      label: '数据概览',
      route: AppRoutes.dashboard,
      icon: Icons.dashboard_outlined,
      panel: UINavigationPanel.more,
      group: '数据与内容',
      order: 10,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.chat-logs',
      label: '聊天记录',
      route: AppRoutes.chatLogs,
      icon: Icons.history_edu_outlined,
      panel: UINavigationPanel.more,
      group: '数据与内容',
      order: 20,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.chat-import',
      label: '聊天记录导入',
      route: AppRoutes.chatImport,
      icon: Icons.file_download_outlined,
      panel: UINavigationPanel.more,
      group: '数据与内容',
      order: 30,
      builtin: true,
    ),
    UINavigationItem(
      id: 'builtin.emotes',
      label: '表情包',
      route: AppRoutes.emotes,
      icon: Icons.emoji_emotions_outlined,
      panel: UINavigationPanel.more,
      group: '数据与内容',
      order: 40,
      builtin: true,
    ),
  ];

  static List<UINavigationItem> resolve(UIProviderSnapshot? snapshot) {
    final items = <UINavigationItem>[...builtinItems];
    if (snapshot == null) return _sorted(items);

    // Navigation metadata is consumed only from profile-resolved providers.
    // Merely installing/enabling an extension must never mutate global nav.
    final sources = <UIProviderDefinition>[];
    for (final capability in <String>[
      UICapability.appNavigation,
      UICapability.routeRegistry,
    ]) {
      final provider = snapshot.resolve(capability);
      if (provider == null || !provider.enabled || provider.builtin) continue;
      if (sources.any((item) => item.providerId == provider.providerId)) continue;
      sources.add(provider);
    }

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
            !seenIds.add(compositeId)) {
          continue;
        }
        items.add(
          UINavigationItem(
            id: compositeId,
            label: label,
            route: route,
            icon: iconFromName((row['icon'] ?? 'extension').toString()),
            panel: (row['panel'] ?? 'more').toString() == 'main'
                ? UINavigationPanel.main
                : UINavigationPanel.more,
            group: (row['group'] ?? '扩展').toString(),
            order: (row['order'] as num?)?.toInt() ?? 1000,
            routePrefixes: ((row['routePrefixes'] as List?) ?? const <dynamic>[])
                .map((e) => e.toString())
                .where((e) => e.startsWith('/'))
                .toList(),
            extensionId: provider.extensionId,
          ),
        );
      }
    }

    final iconProvider = snapshot.resolve(UICapability.icons);
    final aliases = iconProvider?.metadata['iconAliases'];
    final iconAliases = aliases is Map ? aliases.cast<String, dynamic>() : const <String, dynamic>{};
    final withIconOverrides = items
        .map((item) {
          final alias = iconAliases[item.id] ?? iconAliases[item.group] ?? iconAliases['default'];
          if (alias == null) return item;
          return UINavigationItem(
            id: item.id,
            label: item.label,
            route: item.route,
            icon: iconFromName(alias.toString()),
            panel: item.panel,
            group: item.group,
            order: item.order,
            routePrefixes: item.routePrefixes,
            builtin: item.builtin,
            extensionId: item.extensionId,
          );
        })
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

  static IconData iconFromName(String raw) {
    return switch (raw.toLowerCase()) {
      'chat' || 'message' => Icons.chat_bubble_outline,
      'task' || 'sparkles' => Icons.auto_awesome,
      'people' || 'character' => Icons.people_outline,
      'memory' => Icons.memory,
      'devices' => Icons.devices_outlined,
      'settings' => Icons.settings_outlined,
      'game' => Icons.sports_esports_outlined,
      'dashboard' => Icons.dashboard_outlined,
      'history' => Icons.history_edu_outlined,
      'download' => Icons.file_download_outlined,
      'brush' || 'workshop' => Icons.brush_outlined,
      'pet' => Icons.pets_outlined,
      'notification' || 'reminder' => Icons.notifications_active_outlined,
      'link' || 'channel' => Icons.sync_alt,
      _ => Icons.extension_outlined,
    };
  }
}
