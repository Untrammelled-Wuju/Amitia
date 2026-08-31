import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_motion.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_typography.dart';
import '../../app/theme/design_tokens.dart';
import '../../app/app_routes.dart';
import '../../app/drawer_route_state.dart';
import '../services/extension_service.dart';
import '../services/providers.dart';
import '../ui_runtime/ui_navigation_registry.dart';
import '../ui_runtime/ui_runtime_controller.dart';
import 'amitia_misc.dart';

final themeModeProvider = StateProvider<ThemeMode>((ref) => ThemeMode.light);
final currentCharacterIdProvider = StateProvider<String>((ref) => 'c1');
final isAgentModeProvider = StateProvider<bool>((ref) => false);
final isDeveloperModeProvider = StateProvider<bool>((ref) => false);
final _installedExtensionViewProvider =
    FutureProvider.autoDispose<ExtensionCenterView>((ref) async {
      final service = ref.read(extensionServiceProvider);
      return service.getExtensionCenterView();
    });

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

class AmitiaDrawer extends ConsumerStatefulWidget {
  final String currentRoute;

  const AmitiaDrawer({super.key, required this.currentRoute});

  @override
  ConsumerState<AmitiaDrawer> createState() => _AmitiaDrawerState();
}

class _AmitiaDrawerState extends ConsumerState<AmitiaDrawer> {
  void _navigateTo(String route) {
    final router = GoRouter.of(context);
    final currentRoute = router.routerDelegate.currentConfiguration.fullPath;
    Navigator.of(context).pop();
    if (currentRoute == route) return;
    router.push(route);
  }

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

  @override
  Widget build(BuildContext context) {
    final themeMode = ref.watch(themeModeProvider);
    final isDark = themeMode == ThemeMode.dark;
    final characterId = ref.watch(currentCharacterIdProvider);
    final character = _getCharacter(characterId);
    final navigationItems = UINavigationRegistry.resolve(
      ref.watch(uiRuntimeProvider).valueOrNull,
    );
    final routeState = resolveDrawerRouteState(widget.currentRoute);
    final installedExtensions = ref.watch(_installedExtensionViewProvider);

    return Material(
      color: context.surfacePrimary,
      child: SafeArea(
        child: SizedBox(
          width:
              MediaQuery.sizeOf(context).width * 0.82 >
                  context.uiComponents.drawerMaxWidth
              ? context.uiComponents.drawerMaxWidth
              : MediaQuery.sizeOf(context).width * 0.82,
          child: _DrawerMainPanel(
            character: character,
            isDark: isDark,
            onToggleTheme: () {
              ref.read(themeModeProvider.notifier).state = isDark
                  ? ThemeMode.light
                  : ThemeMode.dark;
            },
            onSearchTap: () => _navigateTo(AppRoutes.conversations),
            onNavigate: _navigateTo,
            onSettingsTap: () => _navigateTo(AppRoutes.settings),
            navigationItems: navigationItems,
            installedExtensions: installedExtensions,
            currentRoute: widget.currentRoute,
            settingsSelected: routeState.settingsSelected,
          ),
        ),
      ),
    );
  }
}

class _DrawerMainPanel extends StatelessWidget {
  final _CharInfo character;
  final bool isDark;
  final VoidCallback onToggleTheme;
  final VoidCallback onSearchTap;
  final ValueChanged<String> onNavigate;
  final VoidCallback onSettingsTap;
  final List<UINavigationItem> navigationItems;
  final AsyncValue<ExtensionCenterView> installedExtensions;
  final String currentRoute;
  final bool settingsSelected;

  const _DrawerMainPanel({
    required this.character,
    required this.isDark,
    required this.onToggleTheme,
    required this.onSearchTap,
    required this.onNavigate,
    required this.onSettingsTap,
    required this.navigationItems,
    required this.installedExtensions,
    required this.currentRoute,
    required this.settingsSelected,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(
          child: ListView(
            padding: EdgeInsets.zero,
            children: [
              _DrawerHeader(
                name: character.name,
                isDark: isDark,
                onToggleTheme: onToggleTheme,
                onSearchTap: onSearchTap,
              ),
              SizedBox(height: AppSpacing.sm),
              ...navigationItems
                  .where((item) => item.panel == UINavigationPanel.main)
                  .map(
                    (item) => _MainMenuItem(
                      icon: item.icon,
                      label: item.label,
                      isSelected: item.matches(currentRoute),
                      onTap: () => onNavigate(item.route),
                    ),
                  ),
              installedExtensions.when(
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
                error: (_, _) => const SizedBox.shrink(),
              ),
            ],
          ),
        ),
        _DrawerBottomArea(
          onSettingsTap: onSettingsTap,
          settingsSelected: settingsSelected,
          userName: character.name,
          userInitial: character.avatarInitial,
        ),
      ],
    );
  }

  Widget _buildInstalledExtensions(
    BuildContext context,
    ExtensionCenterView view,
  ) {
    if (view.installed.isEmpty) return const SizedBox.shrink();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: 16),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 20),
          child: Text('已安装扩展', style: AppTypography.label(context)),
        ),
        const SizedBox(height: 4),
        ...view.installed.map(
          (extension) => _InstalledExtensionMenuItem(
            name: extension.displayName,
            status: extension.status,
            isEnabled: extension.enabled,
            isSelected: isRouteFamily(
              currentRoute,
              AppRoutes.pluginDetail(extension.extensionId),
            ),
            onTap: () =>
                onNavigate(AppRoutes.pluginDetail(extension.extensionId)),
          ),
        ),
      ],
    );
  }
}

class _InstalledExtensionMenuItem extends StatelessWidget {
  final String name;
  final String status;
  final bool isEnabled;
  final bool isSelected;
  final VoidCallback onTap;

  const _InstalledExtensionMenuItem({
    required this.name,
    required this.status,
    required this.isEnabled,
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
            constraints: const BoxConstraints(minHeight: 44),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 9),
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
                        status,
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
    this.userName = '无拘',
    this.userInitial = '无',
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Divider(height: 1, color: context.borderPrimary),
        Padding(
          padding: const EdgeInsets.fromLTRB(11, 8, 11, 12),
          child: GestureDetector(
            onTap: onSettingsTap,
            child: Container(
              padding: const EdgeInsets.all(11),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: BorderRadius.circular(19),
                border: Border.all(color: context.borderPrimary, width: 1),
                boxShadow: [
                  BoxShadow(
                    color: context.scrim.withValues(alpha: 0.05),
                    blurRadius: 16,
                    offset: const Offset(0, 4),
                  ),
                ],
              ),
              child: Row(
                children: [
                  Container(
                    width: 42,
                    height: 42,
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(15),
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
                        userInitial,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 14,
                          fontWeight: FontWeight.w600,
                        ),
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
                          userName,
                          style: TextStyle(
                            fontSize: 12,
                            fontWeight: FontWeight.w700,
                            color: context.textPrimary,
                          ),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const SizedBox(height: 2),
                        Row(
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Container(
                              width: 6,
                              height: 6,
                              decoration: BoxDecoration(
                                color: const Color(0xFF4F9B6B),
                                shape: BoxShape.circle,
                                boxShadow: [
                                  BoxShadow(
                                    color: const Color(
                                      0xFF4F9B6B,
                                    ).withValues(alpha: 0.2),
                                    blurRadius: 4,
                                    spreadRadius: 1,
                                  ),
                                ],
                              ),
                            ),
                            const SizedBox(width: 4),
                            Text(
                              '本地运行',
                              style: TextStyle(
                                fontSize: 9,
                                color: context.textTertiary,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: 6),
                  Icon(
                    Icons.settings_outlined,
                    size: 17,
                    color: context.textTertiary,
                  ),
                ],
              ),
            ),
          ),
        ),
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
          Expanded(
            child: Text(
              name,
              style: AppTypography.pageTitle(context).copyWith(fontSize: 17),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
          IconButton(
            icon: const Icon(Icons.search, size: 22),
            onPressed: onSearchTap,
            visualDensity: VisualDensity.compact,
          ),
          IconButton(
            icon: Icon(
              isDark ? Icons.light_mode_outlined : Icons.dark_mode_outlined,
              size: 22,
            ),
            onPressed: onToggleTheme,
            visualDensity: VisualDensity.compact,
          ),
        ],
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
    final variant = context.uiComponentVariant('navigationItem');
    double number(String key, double fallback) =>
        variant[key] is num ? (variant[key] as num).toDouble() : fallback;
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: number('outerPaddingX', 12),
          vertical: number('outerPaddingY', 2),
        ),
        child: AnimatedContainer(
          duration: AppMotion.standard,
          curve: AppMotion.standardCurve,
          constraints: BoxConstraints(minHeight: number('minHeight', 44)),
          padding: EdgeInsets.symmetric(
            horizontal: number('paddingX', 12),
            vertical: number('paddingY', 11),
          ),
          decoration: BoxDecoration(
            color: isSelected ? context.accentSoft : Colors.transparent,
            borderRadius: BorderRadius.circular(
              number('radius', AppRadius.small),
            ),
          ),
          child: Row(
            children: [
              Icon(
                icon,
                size: number('iconSize', 20),
                color: isSelected
                    ? context.accentPrimary
                    : context.textSecondary,
              ),
              const SizedBox(width: 14),
              Text(
                label,
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
                  color: isSelected
                      ? context.accentPrimary
                      : context.textPrimary,
                ),
              ),
              const Spacer(),
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
