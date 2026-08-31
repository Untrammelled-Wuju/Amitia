import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../app/app_routes.dart';
import '../../app/drawer_route_state.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';
import '../services/providers.dart';
import '../ui_runtime/ui_navigation_registry.dart';
import '../ui_runtime/ui_runtime_controller.dart';
import 'amitia_misc.dart';

final themeModeProvider = StateProvider<ThemeMode>((ref) => ThemeMode.light);
final currentCharacterIdProvider = StateProvider<String>((ref) => 'c1');
final isAgentModeProvider = StateProvider<bool>((ref) => false);
final isDeveloperModeProvider = StateProvider<bool>((ref) => false);

const _placeholderCharacterName = 'Amitia';
const _placeholderCharacterStatus = '在线 · 当前角色';
const _placeholderCharacterInitial = 'A';

class AmitiaDrawer extends ConsumerWidget {
  final String currentRoute;

  const AmitiaDrawer({super.key, required this.currentRoute});

  void _navigateTo(BuildContext context, String route) {
    final router = GoRouter.of(context);
    final current = router.routerDelegate.currentConfiguration.fullPath;
    Navigator.of(context).pop();
    if (current == route) return;
    router.push(route);
  }

  void _openGlobalSearch(BuildContext context) {
    final router = GoRouter.of(context);
    Navigator.of(context).pop();
    Navigator.of(context, rootNavigator: true).push(
      MaterialPageRoute<void>(
        fullscreenDialog: true,
        builder: (_) => _DrawerGlobalSearchPage(router: router),
      ),
    );
  }

  List<UINavigationItem> _extensionEntries(List<UINavigationItem> items) {
    final deduped = <String, UINavigationItem>{};
    for (final item in items) {
      final extensionId = item.extensionId?.trim() ?? '';
      if (item.builtin || extensionId.isEmpty) continue;
      deduped.putIfAbsent(extensionId, () => item);
    }
    return deduped.values.toList(growable: false);
  }

  bool _isBuiltinSelected(DrawerRouteState state, UINavigationItem item) {
    return switch (item.id) {
      'builtin.chat' => state.mainItem == MainDrawerItem.chat,
      'builtin.characters' => state.mainItem == MainDrawerItem.characters,
      'builtin.memory' => state.mainItem == MainDrawerItem.memory,
      'builtin.devices' => state.mainItem == MainDrawerItem.devices,
      'builtin.extensions' => state.mainItem == MainDrawerItem.extensions,
      'builtin.workshop' => state.mainItem == MainDrawerItem.workshop,
      _ => item.matches(currentRoute),
    };
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final routeState = resolveDrawerRouteState(currentRoute);
    final themeMode = ref.watch(themeModeProvider);
    final isDark = themeMode == ThemeMode.dark;
    final navigationItems = UINavigationRegistry.resolve(
      ref.watch(uiRuntimeProvider).valueOrNull,
    );
    final builtins = navigationItems
        .where((item) => item.builtin && item.panel == UINavigationPanel.main)
        .toList(growable: false);
    final extensionItems = _extensionEntries(navigationItems);
    final user = ref.watch(currentUserProvider).valueOrNull;
    final userName = (user?.username.trim().isNotEmpty ?? false)
        ? user!.username.trim()
        : '用户';
    final userInitial = userName.isNotEmpty ? userName.substring(0, 1) : '用';

    final width = MediaQuery.sizeOf(context).width * 0.84;
    final drawerWidth = width > context.uiComponents.drawerMaxWidth
        ? context.uiComponents.drawerMaxWidth
        : width;

    return Material(
      color: Colors.transparent,
      child: SafeArea(
        right: false,
        child: SizedBox(
          width: drawerWidth,
          child: Material(
            color: context.surfacePrimary,
            clipBehavior: Clip.antiAlias,
            shape: const RoundedRectangleBorder(
              borderRadius: BorderRadius.only(
                topRight: Radius.circular(29),
                bottomRight: Radius.circular(29),
              ),
            ),
            child: Column(
              children: [
                _DrawerHeader(
                  isDark: isDark,
                  onToggleTheme: () {
                    ref.read(themeModeProvider.notifier).state =
                        isDark ? ThemeMode.light : ThemeMode.dark;
                  },
                  onSearchTap: () => _openGlobalSearch(context),
                ),
                Expanded(
                  child: ListView(
                    padding: const EdgeInsets.fromLTRB(11, 0, 11, 12),
                    children: [
                      const SizedBox(height: 4),
                      ...builtins.map(
                        (item) => _MainMenuItem(
                          icon: item.icon,
                          label: item.label,
                          isSelected: _isBuiltinSelected(routeState, item),
                          onTap: () => _navigateTo(context, item.route),
                        ),
                      ),
                      if (extensionItems.isNotEmpty) ...[
                        const SizedBox(height: 12),
                        const _DrawerSectionLabel('已启用扩展'),
                        const SizedBox(height: 5),
                        ...extensionItems.map(
                          (item) => _ExtensionMenuItem(
                            item: item,
                            isSelected: item.matches(currentRoute),
                            onTap: () => _navigateTo(context, item.route),
                          ),
                        ),
                      ],
                    ],
                  ),
                ),
                _DrawerBottomArea(
                  onSettingsTap: () => _navigateTo(context, AppRoutes.settings),
                  settingsSelected: routeState.settingsSelected,
                  userName: userName,
                  userInitial: userInitial,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _DrawerHeader extends StatelessWidget {
  final bool isDark;
  final VoidCallback onToggleTheme;
  final VoidCallback onSearchTap;

  const _DrawerHeader({
    required this.isDark,
    required this.onToggleTheme,
    required this.onSearchTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 14, 12, 15),
      child: Row(
        children: [
          Container(
            width: 46,
            height: 46,
            decoration: BoxDecoration(
              borderRadius: BorderRadius.circular(16),
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [context.accentPrimary, context.accentSecondary],
              ),
            ),
            alignment: Alignment.center,
            child: const Text(
              _placeholderCharacterInitial,
              style: TextStyle(
                color: Colors.white,
                fontSize: 14,
                fontWeight: FontWeight.w700,
              ),
            ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(
                  _placeholderCharacterName,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                    color: context.textPrimary,
                  ),
                ),
                const SizedBox(height: 3),
                Text(
                  _placeholderCharacterStatus,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 10,
                    color: context.textTertiary,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: 6),
          _DrawerIconButton(
            icon: Icons.search,
            tooltip: '全局搜索',
            onTap: onSearchTap,
          ),
          const SizedBox(width: 6),
          _DrawerIconButton(
            icon: isDark ? Icons.light_mode_outlined : Icons.dark_mode_outlined,
            tooltip: '切换深浅色模式',
            onTap: onToggleTheme,
          ),
        ],
      ),
    );
  }
}

class _DrawerIconButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;

  const _DrawerIconButton({
    required this.icon,
    required this.tooltip,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: tooltip,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          width: 31,
          height: 31,
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: BorderRadius.circular(11),
            border: Border.all(color: context.borderPrimary),
          ),
          alignment: Alignment.center,
          child: Icon(icon, size: 16, color: context.textSecondary),
        ),
      ),
    );
  }
}

class _DrawerSectionLabel extends StatelessWidget {
  final String text;

  const _DrawerSectionLabel(this.text);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 12),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 9.5,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.7,
          color: context.textTertiary,
        ),
      ),
    );
  }
}

class _MainMenuItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  const _MainMenuItem({
    required this.icon,
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          height: 43,
          margin: const EdgeInsets.symmetric(vertical: 2),
          padding: const EdgeInsets.symmetric(horizontal: 12),
          decoration: BoxDecoration(
            color: isSelected ? context.accentSoft : Colors.transparent,
            borderRadius: BorderRadius.circular(13),
            border: Border.all(
              color: isSelected
                  ? context.accentPrimary.withValues(alpha: 0.18)
                  : Colors.transparent,
            ),
          ),
          child: Row(
            children: [
              Icon(
                icon,
                size: 18,
                color: isSelected ? context.accentPrimary : context.textSecondary,
              ),
              const SizedBox(width: 11),
              Expanded(
                child: Text(
                  label,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 12.5,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? context.accentPrimary : context.textSecondary,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ExtensionMenuItem extends StatelessWidget {
  final UINavigationItem item;
  final bool isSelected;
  final VoidCallback onTap;

  const _ExtensionMenuItem({
    required this.item,
    required this.isSelected,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          constraints: const BoxConstraints(minHeight: 48),
          margin: const EdgeInsets.symmetric(vertical: 2),
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
          decoration: BoxDecoration(
            color: isSelected ? context.accentSoft : Colors.transparent,
            borderRadius: BorderRadius.circular(13),
          ),
          child: Row(
            children: [
              Container(
                width: 32,
                height: 32,
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: BorderRadius.circular(10),
                ),
                alignment: Alignment.center,
                child: Icon(item.icon, size: 16, color: context.accentPrimary),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      item.label,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 11.5,
                        fontWeight: FontWeight.w600,
                        color: context.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      item.extensionId ?? '扩展',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(fontSize: 8.8, color: context.textTertiary),
                    ),
                  ],
                ),
              ),
              Icon(Icons.chevron_right, size: 15, color: context.textTertiary),
            ],
          ),
        ),
      ),
    );
  }
}

class _DrawerBottomArea extends StatelessWidget {
  final VoidCallback onSettingsTap;
  final bool settingsSelected;
  final String userName;
  final String userInitial;

  const _DrawerBottomArea({
    required this.onSettingsTap,
    required this.settingsSelected,
    required this.userName,
    required this.userInitial,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Divider(height: 1, color: context.borderSecondary),
        Padding(
          padding: const EdgeInsets.fromLTRB(14, 10, 14, 12),
          child: Semantics(
            button: true,
            selected: settingsSelected,
            child: GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: onSettingsTap,
              child: Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: settingsSelected ? context.accentSoft : Colors.transparent,
                  borderRadius: BorderRadius.circular(15),
                ),
                child: Row(
                  children: [
                    Container(
                      width: 36,
                      height: 36,
                      decoration: BoxDecoration(
                        borderRadius: BorderRadius.circular(12),
                        gradient: LinearGradient(
                          begin: Alignment.topLeft,
                          end: Alignment.bottomRight,
                          colors: [context.accentPrimary, context.accentSecondary],
                        ),
                      ),
                      alignment: Alignment.center,
                      child: Text(
                        userInitial,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 12,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                    ),
                    const SizedBox(width: 9),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Text(
                            userName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 11.5,
                              fontWeight: FontWeight.w600,
                              color: context.textPrimary,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            '设置与账号',
                            style: TextStyle(fontSize: 9.5, color: context.textTertiary),
                          ),
                        ],
                      ),
                    ),
                    Icon(Icons.settings_outlined, size: 17, color: context.textTertiary),
                  ],
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _SearchPageTarget {
  final String label;
  final String description;
  final String route;
  final IconData icon;

  const _SearchPageTarget({
    required this.label,
    required this.description,
    required this.route,
    required this.icon,
  });
}

const _builtinSearchTargets = <_SearchPageTarget>[
  _SearchPageTarget(label: '对话', description: 'AI 对话', route: AppRoutes.chat, icon: Icons.chat_bubble_outline),
  _SearchPageTarget(label: '角色管理', description: 'AI 角色编辑', route: AppRoutes.characters, icon: Icons.people_outline),
  _SearchPageTarget(label: '记忆总览', description: '长期记忆与关系数据', route: AppRoutes.memory, icon: Icons.memory_outlined),
  _SearchPageTarget(label: '设备', description: '设备能力与连接状态', route: AppRoutes.settingsDevices, icon: Icons.devices_outlined),
  _SearchPageTarget(label: '扩展中心', description: '扩展包与能力管理', route: AppRoutes.extensions, icon: Icons.extension_outlined),
  _SearchPageTarget(label: '创意工坊', description: 'Skill 与桌宠制作', route: AppRoutes.workshop, icon: Icons.brush_outlined),
  _SearchPageTarget(label: '渠道中心', description: '微信与 QQ 接入', route: AppRoutes.channels, icon: Icons.sync_alt),
  _SearchPageTarget(label: '日程提醒', description: '主动陪伴与提醒', route: AppRoutes.reminders, icon: Icons.notifications_active_outlined),
  _SearchPageTarget(label: '数据概览', description: '使用数据统计', route: AppRoutes.dashboard, icon: Icons.dashboard_outlined),
  _SearchPageTarget(label: '聊天记录', description: '历史对话', route: AppRoutes.chatLogs, icon: Icons.history_edu_outlined),
  _SearchPageTarget(label: '表情包', description: '聊天表情管理', route: AppRoutes.emotes, icon: Icons.emoji_emotions_outlined),
  _SearchPageTarget(label: '设置', description: '账号、AI、数据与系统', route: AppRoutes.settings, icon: Icons.settings_outlined),
];

class _DrawerGlobalSearchPage extends ConsumerStatefulWidget {
  final GoRouter router;

  const _DrawerGlobalSearchPage({required this.router});

  @override
  ConsumerState<_DrawerGlobalSearchPage> createState() => _DrawerGlobalSearchPageState();
}

class _DrawerGlobalSearchPageState extends ConsumerState<_DrawerGlobalSearchPage> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();
  String _query = '';

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _focusNode.requestFocus());
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  bool _matches(String label, String description) {
    final q = _query.trim().toLowerCase();
    if (q.isEmpty) return false;
    return label.toLowerCase().contains(q) || description.toLowerCase().contains(q);
  }

  void _openRoute(String route) {
    Navigator.of(context).pop();
    widget.router.push(route);
  }

  @override
  Widget build(BuildContext context) {
    final characters = ref.watch(characterListProvider).valueOrNull ?? const [];
    final runtimeItems = UINavigationRegistry.resolve(ref.watch(uiRuntimeProvider).valueOrNull);
    final extensionTargets = runtimeItems
        .where((item) => !item.builtin && (item.extensionId?.isNotEmpty ?? false))
        .map(
          (item) => _SearchPageTarget(
            label: item.label,
            description: '扩展 · ${item.extensionId}',
            route: item.route,
            icon: item.icon,
          ),
        )
        .toList(growable: false);
    final pages = <_SearchPageTarget>[..._builtinSearchTargets, ...extensionTargets]
        .where((item) => _matches(item.label, item.description))
        .toList(growable: false);
    final matchedCharacters = characters
        .where((character) => _matches(character.name, '${character.identity} ${character.description}'))
        .toList(growable: false);
    final hasQuery = _query.trim().isNotEmpty;

    return Material(
      color: context.backgroundPrimary,
      child: SafeArea(
        child: Column(
          children: [
            Container(
              height: 58,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              decoration: BoxDecoration(
                color: context.backgroundPrimary,
                border: Border(bottom: BorderSide(color: context.borderSecondary)),
              ),
              child: Row(
                children: [
                  GestureDetector(
                    behavior: HitTestBehavior.opaque,
                    onTap: () => Navigator.of(context).pop(),
                    child: SizedBox(
                      width: 38,
                      height: 38,
                      child: Icon(Icons.arrow_back_ios_new, size: 18, color: context.textSecondary),
                    ),
                  ),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Container(
                      height: 40,
                      padding: const EdgeInsets.symmetric(horizontal: 11),
                      decoration: BoxDecoration(
                        color: context.surfacePrimary,
                        borderRadius: BorderRadius.circular(14),
                        border: Border.all(color: context.borderPrimary),
                      ),
                      child: Row(
                        children: [
                          Icon(Icons.search, size: 17, color: context.textTertiary),
                          const SizedBox(width: 8),
                          Expanded(
                            child: TextField(
                              controller: _controller,
                              focusNode: _focusNode,
                              onChanged: (value) => setState(() => _query = value),
                              decoration: const InputDecoration(
                                border: InputBorder.none,
                                isDense: true,
                                hintText: '搜索功能、角色...',
                              ),
                              style: TextStyle(fontSize: 13, color: context.textPrimary),
                            ),
                          ),
                          if (_controller.text.isNotEmpty)
                            GestureDetector(
                              onTap: () {
                                _controller.clear();
                                setState(() => _query = '');
                                _focusNode.requestFocus();
                              },
                              child: SizedBox(
                                width: 28,
                                height: 28,
                                child: Icon(Icons.close, size: 16, color: context.textTertiary),
                              ),
                            ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Expanded(
              child: !hasQuery
                  ? _SearchEmptyHint(
                      icon: Icons.search,
                      text: '输入关键词搜索页面或角色',
                    )
                  : pages.isEmpty && matchedCharacters.isEmpty
                      ? _SearchEmptyHint(
                          icon: Icons.search_off_outlined,
                          text: '未找到匹配结果',
                        )
                      : ListView(
                          padding: const EdgeInsets.fromLTRB(12, 8, 12, 24),
                          children: [
                            if (pages.isNotEmpty) ...[
                              const _SearchSectionTitle('页面'),
                              ...pages.map(
                                (item) => _GlobalSearchRow(
                                  icon: item.icon,
                                  title: item.label,
                                  subtitle: item.description,
                                  onTap: () => _openRoute(item.route),
                                ),
                              ),
                            ],
                            if (matchedCharacters.isNotEmpty) ...[
                              const _SearchSectionTitle('角色'),
                              ...matchedCharacters.map(
                                (character) => _GlobalSearchRow(
                                  icon: Icons.person_outline,
                                  title: character.name,
                                  subtitle: character.identity.isNotEmpty
                                      ? character.identity
                                      : '角色',
                                  onTap: () => _openRoute(AppRoutes.character(character.id)),
                                  accent: true,
                                ),
                              ),
                            ],
                          ],
                        ),
            ),
          ],
        ),
      ),
    );
  }
}

class _SearchEmptyHint extends StatelessWidget {
  final IconData icon;
  final String text;

  const _SearchEmptyHint({required this.icon, required this.text});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 28, color: context.textTertiary),
          const SizedBox(height: 10),
          Text(text, style: TextStyle(fontSize: 11.5, color: context.textTertiary)),
        ],
      ),
    );
  }
}

class _SearchSectionTitle extends StatelessWidget {
  final String text;

  const _SearchSectionTitle(this.text);

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(8, 10, 8, 5),
      child: Text(
        text,
        style: TextStyle(
          fontSize: 9.5,
          fontWeight: FontWeight.w700,
          letterSpacing: 0.8,
          color: context.textTertiary,
        ),
      ),
    );
  }
}

class _GlobalSearchRow extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;
  final bool accent;

  const _GlobalSearchRow({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
    this.accent = false,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          constraints: const BoxConstraints(minHeight: 54),
          padding: const EdgeInsets.all(8),
          child: Row(
            children: [
              Container(
                width: 36,
                height: 36,
                decoration: BoxDecoration(
                  color: accent ? context.accentSoft : context.surfaceSecondary,
                  borderRadius: BorderRadius.circular(12),
                ),
                alignment: Alignment.center,
                child: Icon(
                  icon,
                  size: 17,
                  color: accent ? context.accentPrimary : context.textSecondary,
                ),
              ),
              const SizedBox(width: 10),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text(
                      title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(
                        fontSize: 12.5,
                        fontWeight: FontWeight.w600,
                        color: context.textPrimary,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      subtitle,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(fontSize: 9.5, color: context.textTertiary),
                    ),
                  ],
                ),
              ),
              Icon(Icons.chevron_right, size: 16, color: context.textTertiary),
            ],
          ),
        ),
      ),
    );
  }
}

class AmitiaCharacterCard extends StatelessWidget {
  final String name;
  final String status;
  final String identity;
  final String avatarInitial;
  final String avatarColor;
  final String mood;
  final String lastActive;
  final VoidCallback? onTap;

  const AmitiaCharacterCard({
    super.key,
    required this.name,
    required this.status,
    required this.identity,
    required this.avatarInitial,
    required this.avatarColor,
    required this.mood,
    required this.lastActive,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final color = Color(
      int.parse('FF${avatarColor.replaceAll('#', '')}', radix: 16),
    );
    final isOnline = status == '在线';
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Stack(
              children: [
                Container(
                  width: 52,
                  height: 52,
                  decoration: BoxDecoration(
                    color: color,
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: Text(
                      avatarInitial,
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 22,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                ),
                if (isOnline)
                  Positioned(
                    right: 0,
                    bottom: 0,
                    child: Container(
                      width: 14,
                      height: 14,
                      decoration: BoxDecoration(
                        color: context.success,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: context.surfacePrimary,
                          width: 2,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
            const SizedBox(width: 14),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(width: 8),
                      AmitiaStatusBadge(
                        label: status,
                        type: isOnline ? BadgeType.success : BadgeType.neutral,
                      ),
                    ],
                  ),
                  const SizedBox(height: 3),
                  Text(identity, style: AppTypography.caption(context)),
                  const SizedBox(height: 3),
                  Row(
                    children: [
                      Text('心情：$mood', style: AppTypography.label(context)),
                      const SizedBox(width: 12),
                      Text(lastActive, style: AppTypography.label(context)),
                    ],
                  ),
                ],
              ),
            ),
            Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
          ],
        ),
      ),
    );
  }
}

class AmitiaExtensionCard extends StatelessWidget {
  final String name;
  final String description;
  final IconData icon;
  final String typeLabel;
  final bool isInstalled;
  final bool isEnabled;
  final bool isRecommended;
  final VoidCallback? onAction;
  final ValueChanged<bool>? onToggle;

  const AmitiaExtensionCard({
    super.key,
    required this.name,
    required this.description,
    required this.icon,
    required this.typeLabel,
    required this.isInstalled,
    required this.isEnabled,
    this.isRecommended = false,
    this.onAction,
    this.onToggle,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(14),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(icon, size: 22, color: context.accentPrimary),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(name, style: AppTypography.cardTitle(context)),
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: context.borderSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(
                        typeLabel,
                        style: TextStyle(
                          fontSize: 10,
                          color: context.textTertiary,
                        ),
                      ),
                    ),
                    if (isRecommended) ...[
                      const SizedBox(width: 4),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 6,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text(
                          '推荐',
                          style: TextStyle(
                            fontSize: 10,
                            color: context.accentPrimary,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  description,
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          if (isInstalled)
            SizedBox(
              width: 44,
              child: Switch(
                value: isEnabled,
                onChanged: onToggle,
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
              ),
            )
          else
            GestureDetector(
              onTap: onAction,
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 8,
                ),
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(
                  '安装',
                  style: TextStyle(
                    fontSize: 13,
                    color: Colors.white,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
