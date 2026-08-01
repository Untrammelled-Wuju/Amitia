import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../app/app_routes.dart';
import '../../app/drawer_route_state.dart';
import 'amitia_misc.dart';

final themeModeProvider = StateProvider<ThemeMode>((ref) => ThemeMode.light);
final currentCharacterIdProvider = StateProvider<String>((ref) => 'c1');
final isAgentModeProvider = StateProvider<bool>((ref) => false);
final isDeveloperModeProvider = StateProvider<bool>((ref) => false);
final mockStartupStageProvider = StateProvider<MockStartupStage>((ref) => MockStartupStage.ready);

enum MockStartupStage {
  firstLaunch,
  needsLogin,
  privacyRequired,
  ready,
}

class _CharInfo {
  final String name, status, description, avatarInitial, avatarColor;
  _CharInfo(this.name, this.status, this.description, this.avatarInitial, this.avatarColor);
}

class AmitiaDrawer extends ConsumerStatefulWidget {
  final String currentRoute;

  const AmitiaDrawer({super.key, required this.currentRoute});

  @override
  ConsumerState<AmitiaDrawer> createState() => _AmitiaDrawerState();
}

class _AmitiaDrawerState extends ConsumerState<AmitiaDrawer> {
  late PageController _pageController;
  late DrawerRouteState _routeState;
  DrawerPanel _currentPanel = DrawerPanel.main;
  bool _pageInitialized = false;

  @override
  void initState() {
    super.initState();
    _routeState = resolveDrawerRouteState(widget.currentRoute);
    _currentPanel = _routeState.initialPanel;
    _pageController = PageController(initialPage: _routeState.initialPanel == DrawerPanel.more ? 1 : 0);
    _pageInitialized = true;
  }

  @override
  void didUpdateWidget(AmitiaDrawer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.currentRoute != widget.currentRoute) {
      _routeState = resolveDrawerRouteState(widget.currentRoute);
      final targetPanel = _routeState.initialPanel;
      if (_currentPanel != targetPanel) {
        _currentPanel = targetPanel;
      }
      if (_pageInitialized) {
        final targetPage = targetPanel == DrawerPanel.more ? 1 : 0;
        if (_pageController.hasClients && (_pageController.page?.round() ?? 0) != targetPage) {
          _pageController.jumpToPage(targetPage);
        }
      }
    }
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  void _goToMorePanel() {
    if (_currentPanel == DrawerPanel.more) return;
    _currentPanel = DrawerPanel.more;
    _pageController.animateToPage(1, duration: const Duration(milliseconds: 260), curve: Curves.easeInOutCubic);
  }

  void _backToMainPanel() {
    if (_currentPanel == DrawerPanel.main) return;
    _currentPanel = DrawerPanel.main;
    _pageController.animateToPage(0, duration: const Duration(milliseconds: 260), curve: Curves.easeInOutCubic);
  }

  void _navigateTo(String route) {
    Navigator.pop(context);
    Future.delayed(const Duration(milliseconds: 200), () {
      if (!mounted) return;
      GoRouter.of(context).go(route);
    });
  }

  _CharInfo _getCharacter(String id) {
    const chars = {
      'c1': ('阿米娅', '在线 · 空闲中', '温柔、细心，喜欢帮助你解决问题', '阿', '#7668EE'),
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

    return PopScope(
      canPop: _currentPanel == DrawerPanel.main,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop && _currentPanel == DrawerPanel.more) {
          _backToMainPanel();
        }
      },
      child: Material(
        color: context.surfacePrimary,
        child: SafeArea(
          child: SizedBox(
            width: MediaQuery.sizeOf(context).width * 0.82 > 340 ? 340 : MediaQuery.sizeOf(context).width * 0.82,
            child: Column(
              children: [
                Expanded(
                  child: PageView(
                    controller: _pageController,
                    physics: const NeverScrollableScrollPhysics(),
                    onPageChanged: (index) {
                      final nextPanel = index == 0 ? DrawerPanel.main : DrawerPanel.more;
                      if (_currentPanel != nextPanel) {
                        setState(() {
                          _currentPanel = nextPanel;
                        });
                      }
                    },
                    children: [
                      _DrawerMainPanel(
                        character: character,
                        routeState: _routeState,
                        isDark: isDark,
                        onToggleTheme: () {
                          ref.read(themeModeProvider.notifier).state = isDark ? ThemeMode.light : ThemeMode.dark;
                        },
                        onSearchTap: () => _navigateTo(AppRoutes.conversations),
                        onMoreTap: _goToMorePanel,
                        onNavigate: _navigateTo,
                        onSettingsTap: () => _navigateTo(AppRoutes.settings),
                      ),
                      _DrawerMorePanel(
                        routeState: _routeState,
                        onNavigate: _navigateTo,
                        onBack: _backToMainPanel,
                      ),
                    ],
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

class _DrawerMainPanel extends StatelessWidget {
  final _CharInfo character;
  final DrawerRouteState routeState;
  final bool isDark;
  final VoidCallback onToggleTheme;
  final VoidCallback onSearchTap;
  final VoidCallback onMoreTap;
  final ValueChanged<String> onNavigate;
  final VoidCallback onSettingsTap;

  const _DrawerMainPanel({
    required this.character,
    required this.routeState,
    required this.isDark,
    required this.onToggleTheme,
    required this.onSearchTap,
    required this.onMoreTap,
    required this.onNavigate,
    required this.onSettingsTap,
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
              const SizedBox(height: AppSpacing.sm),
              _MainMenuItem(
                icon: Icons.chat_bubble_outline,
                label: '对话',
                isSelected: routeState.mainItem == MainDrawerItem.chat,
                onTap: () => onNavigate(AppRoutes.chat),
              ),
              _MainMenuItem(
                icon: Icons.auto_awesome,
                label: '任务',
                isSelected: routeState.mainItem == MainDrawerItem.tasks,
                onTap: () => onNavigate(AppRoutes.agent),
              ),
              _MainMenuItem(
                icon: Icons.people_outline,
                label: '角色',
                isSelected: routeState.mainItem == MainDrawerItem.characters,
                onTap: () => onNavigate(AppRoutes.characters),
              ),
              _MainMenuItem(
                icon: Icons.memory,
                label: '记忆',
                isSelected: routeState.mainItem == MainDrawerItem.memory,
                onTap: () => onNavigate(AppRoutes.memory),
              ),
              _MainMenuItem(
                icon: Icons.devices_outlined,
                label: '设备',
                isSelected: routeState.mainItem == MainDrawerItem.devices,
                onTap: () => onNavigate(AppRoutes.settingsDevices),
              ),
              _MainMenuItem(
                icon: Icons.apps_outlined,
                label: '更多',
                isSelected: routeState.mainItem == MainDrawerItem.more,
                showArrow: true,
                onTap: onMoreTap,
              ),
            ],
          ),
        ),
        _DrawerBottomArea(
          onSettingsTap: onSettingsTap,
          settingsSelected: routeState.settingsSelected,
          userName: character.name,
          userInitial: character.avatarInitial,
        ),
      ],
    );
  }
}

class _DrawerMorePanel extends StatelessWidget {
  final DrawerRouteState routeState;
  final ValueChanged<String> onNavigate;
  final VoidCallback onBack;

  const _DrawerMorePanel({
    required this.routeState,
    required this.onNavigate,
    required this.onBack,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Container(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
          child: Row(
            children: [
              Text('更多', style: AppTypography.pageTitle(context)),
            ],
          ),
        ),
        const Divider(height: 1),
        Expanded(
          child: ListView(
            padding: const EdgeInsets.symmetric(vertical: AppSpacing.sm),
            children: [
              _MoreGroup(label: '能力与扩展', onNavigate: onNavigate, selectedItem: routeState.moreItem, items: [
                _MoreItemData(icon: Icons.extension_outlined, label: '扩展中心', route: AppRoutes.extensions, item: MoreDrawerItem.extensions),
                _MoreItemData(icon: Icons.brush_outlined, label: '创意工坊', route: AppRoutes.workshop, item: MoreDrawerItem.workshop),
              ]),
              _MoreGroup(label: '连接与体验', onNavigate: onNavigate, selectedItem: routeState.moreItem, items: [
                _MoreItemData(icon: Icons.sports_esports_outlined, label: '游戏中心', route: AppRoutes.gameCenter, item: MoreDrawerItem.gameCenter),
                _MoreItemData(icon: Icons.sync_alt, label: '渠道中心', route: AppRoutes.channels, item: MoreDrawerItem.channels),
                _MoreItemData(icon: Icons.pets_outlined, label: '桌宠中心', route: AppRoutes.desktopPet, item: MoreDrawerItem.desktopPet),
                _MoreItemData(icon: Icons.notifications_active_outlined, label: '日程与提醒', route: AppRoutes.reminders, item: MoreDrawerItem.reminders),
              ]),
              _MoreGroup(label: '数据与内容', onNavigate: onNavigate, selectedItem: routeState.moreItem, items: [
                _MoreItemData(icon: Icons.dashboard_outlined, label: '数据概览', route: AppRoutes.dashboard, item: MoreDrawerItem.dashboard),
                _MoreItemData(icon: Icons.history_edu_outlined, label: '聊天记录', route: AppRoutes.chatLogs, item: MoreDrawerItem.chatLogs),
                _MoreItemData(icon: Icons.file_download_outlined, label: '聊天记录导入', route: AppRoutes.chatImport, item: MoreDrawerItem.chatImport),
                _MoreItemData(icon: Icons.emoji_emotions_outlined, label: '表情包', route: AppRoutes.emotes, item: MoreDrawerItem.emotes),
              ]),
            ],
          ),
        ),
        _DrawerBackBottomArea(onBack: onBack),
      ],
    );
  }
}

class _MoreGroup extends StatelessWidget {
  final String label;
  final List<_MoreItemData> items;
  final ValueChanged<String> onNavigate;
  final MoreDrawerItem selectedItem;

  const _MoreGroup({required this.label, required this.items, required this.onNavigate, required this.selectedItem});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(left: 20, top: 12, bottom: 4),
          child: Text(label, style: AppTypography.label(context)),
        ),
        ...items.map((item) => _MainMenuItem(
              icon: item.icon,
              label: item.label,
              isSelected: item.item == selectedItem,
              onTap: () => onNavigate(item.route),
            )),
      ],
    );
  }
}

class _MoreItemData {
  final IconData icon;
  final String label;
  final String route;
  final MoreDrawerItem item;

  const _MoreItemData({required this.icon, required this.label, required this.route, required this.item});
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
                                    color: const Color(0xFF4F9B6B).withValues(alpha: 0.2),
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

class _DrawerBackBottomArea extends StatelessWidget {
  final VoidCallback onBack;

  const _DrawerBackBottomArea({required this.onBack});

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Divider(height: 1, color: context.borderPrimary),
        Padding(
          padding: const EdgeInsets.fromLTRB(11, 8, 11, 12),
          child: GestureDetector(
            onTap: onBack,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: BorderRadius.circular(19),
                border: Border.all(color: context.borderPrimary, width: 1),
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Icon(Icons.arrow_back_ios_new, size: 17, color: context.textSecondary),
                  const SizedBox(width: 6),
                  Text(
                    '返回主菜单',
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w500,
                      color: context.textSecondary,
                    ),
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
            icon: Icon(isDark ? Icons.light_mode_outlined : Icons.dark_mode_outlined, size: 22),
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
  final bool showArrow;
  final VoidCallback onTap;

  const _MainMenuItem({
    required this.icon,
    required this.label,
    required this.isSelected,
    this.showArrow = false,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 2),
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 180),
          curve: Curves.easeOutCubic,
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 11),
          decoration: BoxDecoration(
            color: isSelected ? context.accentSoft : Colors.transparent,
            borderRadius: AppRadius.brSmall,
          ),
          child: Row(
            children: [
              Icon(icon, size: 20, color: isSelected ? context.accentPrimary : context.textSecondary),
              const SizedBox(width: 14),
              Text(
                label,
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
                  color: isSelected ? context.accentPrimary : context.textPrimary,
                ),
              ),
              const Spacer(),
              if (showArrow)
                Icon(Icons.chevron_right, color: context.textTertiary, size: 22),
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
    final color = Color(int.parse('FF${avatarColor.replaceAll('#', '')}', radix: 16));
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
                  decoration: BoxDecoration(color: color, shape: BoxShape.circle),
                  child: Center(
                    child: Text(avatarInitial, style: const TextStyle(color: Colors.white, fontSize: 22, fontWeight: FontWeight.w600)),
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
                        border: Border.all(color: context.surfacePrimary, width: 2),
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
                      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                      decoration: BoxDecoration(
                        color: context.borderSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(typeLabel, style: TextStyle(fontSize: 10, color: context.textTertiary)),
                    ),
                    if (isRecommended) ...[
                      const SizedBox(width: 4),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text('推荐', style: TextStyle(fontSize: 10, color: context.accentPrimary)),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 4),
                Text(description, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
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
                padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text('安装', style: TextStyle(fontSize: 13, color: Colors.white, fontWeight: FontWeight.w500)),
              ),
            ),
        ],
      ),
    );
  }
}
