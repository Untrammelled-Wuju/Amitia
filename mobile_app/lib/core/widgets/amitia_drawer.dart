import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../app/theme/design_tokens.dart';
import '../../app/theme/app_colors.dart';
import '../../app/app_routes.dart';
import '../services/extension_service.dart';
import '../services/providers.dart';
import '../ui_runtime/ui_navigation_registry.dart';
import '../ui_runtime/ui_runtime_controller.dart';

final themeModeProvider = StateProvider<ThemeMode>((ref) => ThemeMode.light);
final currentCharacterIdProvider = StateProvider<String>((ref) => 'c1');
final isAgentModeProvider = StateProvider<bool>((ref) => false);
final isDeveloperModeProvider = StateProvider<bool>((ref) => false);

class _CharInfo {
  final String name, status, description, avatarInitial, avatarColor;
  _CharInfo(
    this.name,
    this.status,
    this.description,
    this.avatarInitial,
    this.avatarColor,
  );
}

final _installedExtensionViewProvider = FutureProvider.autoDispose<ExtensionCenterView>((ref) async {
  final svc = ref.read(extensionServiceProvider);
  return await svc.getExtensionCenterView();
});

class AmitiaDrawer extends ConsumerWidget {
  final String currentRoute;

  const AmitiaDrawer({super.key, required this.currentRoute});

  _CharInfo _getCharacter(String id) {
    const chars = {
      'c1': ('Amitia', '在线 · 空闲中', '温柔、细心，喜欢帮助你解决问题', '阿', '#7668EE'),
      'c2': ('小雨', '在线 · 专注中', '理性、高效，擅长分析和规划', '雨', '#52B788'),
      'c3': ('Epsilon', '离线', '冷静、专业，精通技术问题', 'E', '#6C8FEA'),
      'c4': ('Karin', '在线 · 活力满满', '活泼、充满创意，喜欢头脑风暴', 'K', '#E9A23B'),
    };
    final c = chars[id] ?? chars['c1']!;
    return _CharInfo(c.$1, c.$2, c.$3, c.$4, c.$5);
  }

  void _navigateTo(BuildContext context, String route) {
    final router = GoRouter.of(context);
    final currentRoute = router.routerDelegate.currentConfiguration.fullPath;
    Navigator.of(context).pop();
    if (currentRoute == route) return;
    router.push(route);
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final themeMode = ref.watch(themeModeProvider);
    final isDark = themeMode == ThemeMode.dark;
    final characterId = ref.watch(currentCharacterIdProvider);
    final character = _getCharacter(characterId);
    final navigationItems = UINavigationRegistry.resolve(
      ref.watch(uiRuntimeProvider).valueOrNull,
    );
    final installedExtAsync = ref.watch(_installedExtensionViewProvider);

    return Material(
      color: Colors.transparent,
      child: SafeArea(
        right: false,
        child: SizedBox(
          width: MediaQuery.sizeOf(context).width * 0.82 > context.uiComponents.drawerMaxWidth
              ? context.uiComponents.drawerMaxWidth
              : MediaQuery.sizeOf(context).width * 0.82,
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
                Expanded(
                  child: ListView(
                    padding: EdgeInsets.zero,
                    children: [
                      _DrawerHeader(
                        name: character.name,
                        isDark: isDark,
                        onToggleTheme: () {
                          ref.read(themeModeProvider.notifier).state = isDark
                              ? ThemeMode.light
                              : ThemeMode.dark;
                        },
                        onSearchTap: () => _navigateTo(context, AppRoutes.conversations),
                      ),
                      const SizedBox(height: 15),
                      ...navigationItems.map(
                        (item) => _MainMenuItem(
                          icon: item.icon,
                          label: item.label,
                          isSelected: item.matches(currentRoute),
                          onTap: () => _navigateTo(context, item.route),
                        ),
                      ),
                      installedExtAsync.when(
                        data: (view) => _buildInstalledExtensions(context, view),
                        loading: () => const Padding(
                          padding: EdgeInsets.all(16),
                          child: Center(
                            child: SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(strokeWidth: 2),
                            ),
                          ),
                        ),
                        error: (_, __) => const SizedBox.shrink(),
                      ),
                    ],
                  ),
                ),
                _DrawerBottomArea(
                  onSettingsTap: () => _navigateTo(context, AppRoutes.settings),
                  settingsSelected: currentRoute.startsWith('/settings'),
                  userName: character.name,
                  userInitial: character.avatarInitial,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildInstalledExtensions(BuildContext context, ExtensionCenterView view) {
    if (view.installed.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 16),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 23),
          child: Text(
            '已安装扩展',
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: context.textTertiary,
              letterSpacing: 0.5,
            ),
          ),
        ),
        const SizedBox(height: 4),
        ...view.installed.map((ext) => _ExtensionMenuItem(
              name: ext.displayName,
              description: ext.status,
              isEnabled: ext.enabled,
              onTap: () {
                _navigateTo(context, AppRoutes.pluginDetail(ext.extensionId));
              },
            )),
      ],
    );
  }
}

class _DrawerHeader extends StatelessWidget {
  final String name;
  final bool isDark;
  final VoidCallback onToggleTheme;
  final VoidCallback onSearchTap;

  const _DrawerHeader({
    required this.name,
    required this.isDark,
    required this.onToggleTheme,
    required this.onSearchTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(20, 12, 12, 8),
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
                colors: [
                  context.accentPrimary,
                  context.accentSecondary,
                ],
              ),
            ),
            child: Center(
              child: Text(
                name.isNotEmpty ? name[0] : '?',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 22,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
          const Spacer(),
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
    return Tooltip(
      message: tooltip,
      child: Material(
        color: context.surfaceSecondary,
        shape: RoundedRectangleBorder(
          side: BorderSide(color: context.borderPrimary),
          borderRadius: BorderRadius.circular(12),
        ),
        child: InkWell(
          borderRadius: BorderRadius.circular(12),
          onTap: onTap,
          child: SizedBox(
            width: 36,
            height: 36,
            child: Icon(icon, size: 18, color: context.textSecondary),
          ),
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
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 1),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(13),
        child: InkWell(
          borderRadius: BorderRadius.circular(13),
          onTap: onTap,
          child: Container(
            constraints: const BoxConstraints(minHeight: 48),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
            decoration: BoxDecoration(
              color: isSelected ? context.accentSoft : Colors.transparent,
              borderRadius: BorderRadius.circular(13),
            ),
            child: Row(
              children: [
                Icon(
                  icon,
                  size: 22,
                  color: isSelected ? context.accentPrimary : context.textSecondary,
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Text(
                    label,
                    style: TextStyle(
                      fontSize: 15,
                      color: isSelected ? context.accentPrimary : context.textPrimary,
                      fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ExtensionMenuItem extends StatelessWidget {
  final String name;
  final String description;
  final bool isEnabled;
  final VoidCallback onTap;

  const _ExtensionMenuItem({
    required this.name,
    required this.description,
    required this.isEnabled,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 11, vertical: 1),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(13),
        child: InkWell(
          borderRadius: BorderRadius.circular(13),
          onTap: onTap,
          child: Container(
            constraints: const BoxConstraints(minHeight: 44),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: isEnabled
                        ? context.accentSoft
                        : context.surfaceSecondary,
                    borderRadius: BorderRadius.circular(9),
                  ),
                  child: Icon(
                    Icons.extension_outlined,
                    size: 17,
                    color: isEnabled
                        ? context.accentPrimary
                        : context.textTertiary,
                  ),
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(
                        name,
                        style: TextStyle(
                          fontSize: 14,
                          color: context.textPrimary,
                          fontWeight: FontWeight.w500,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      Text(
                        description,
                        style: TextStyle(
                          fontSize: 11,
                          color: context.textTertiary,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ],
                  ),
                ),
                Container(
                  width: 8,
                  height: 8,
                  decoration: BoxDecoration(
                    shape: BoxShape.circle,
                    color: isEnabled ? context.success : context.borderPrimary,
                  ),
                ),
              ],
            ),
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
    return Container(
      padding: const EdgeInsets.fromLTRB(11, 8, 11, 12),
      decoration: BoxDecoration(
        border: Border(
          top: BorderSide(color: context.borderPrimary, width: 0.5),
        ),
      ),
      child: Material(
        color: Colors.transparent,
        borderRadius: BorderRadius.circular(13),
        child: InkWell(
          borderRadius: BorderRadius.circular(13),
          onTap: onSettingsTap,
          child: Container(
            constraints: const BoxConstraints(minHeight: 48),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
            decoration: BoxDecoration(
              color: settingsSelected ? context.accentSoft : Colors.transparent,
              borderRadius: BorderRadius.circular(13),
            ),
            child: Row(
              children: [
                Icon(
                  Icons.settings_outlined,
                  size: 22,
                  color: settingsSelected ? context.accentPrimary : context.textSecondary,
                ),
                const SizedBox(width: 11),
                Expanded(
                  child: Text(
                    '设置',
                    style: TextStyle(
                      fontSize: 15,
                      color: settingsSelected ? context.accentPrimary : context.textPrimary,
                      fontWeight: settingsSelected ? FontWeight.w600 : FontWeight.w400,
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
