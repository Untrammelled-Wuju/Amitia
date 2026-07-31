import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../app/app_routes.dart';
import '../../app/drawer_route_state.dart';
import '../../shared/mock_data/mock_data.dart';
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
  bool _pageInitialized = false;

  @override
  void initState() {
    super.initState();
    _routeState = resolveDrawerRouteState(widget.currentRoute);
    _pageController = PageController(initialPage: _routeState.initialPanel == DrawerPanel.more ? 1 : 0);
    _pageInitialized = true;
  }

  @override
  void didUpdateWidget(AmitiaDrawer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.currentRoute != widget.currentRoute) {
      _routeState = resolveDrawerRouteState(widget.currentRoute);
      if (_pageInitialized) {
        final targetPage = _routeState.initialPanel == DrawerPanel.more ? 1 : 0;
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
    _pageController.animateToPage(1, duration: const Duration(milliseconds: 260), curve: Curves.easeInOutCubic);
  }

  void _backToMainPanel() {
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
    final isDevMode = ref.watch(isDeveloperModeProvider);
    final characterId = ref.watch(currentCharacterIdProvider);
    final character = _getCharacter(characterId);

    return PopScope(
      canPop: _routeState.initialPanel == DrawerPanel.main,
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop && _routeState.initialPanel == DrawerPanel.more) {
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
                    children: [
                      _DrawerMainPanel(
                        character: character,
                        routeState: _routeState,
                        onMoreTap: _goToMorePanel,
                        onNavigate: _navigateTo,
                        onCharacterTap: () => _showCharacterSwitchSheet(context),
                      ),
                      _DrawerMorePanel(
                        routeState: _routeState,
                        isDevMode: isDevMode,
                        onBack: _backToMainPanel,
                        onNavigate: _navigateTo,
                      ),
                    ],
                  ),
                ),
                _DrawerBottomArea(
                  isDark: isDark,
                  isDevMode: isDevMode,
                  onToggleTheme: () {
                    ref.read(themeModeProvider.notifier).state = isDark ? ThemeMode.light : ThemeMode.dark;
                  },
                  onToggleDevMode: () {
                    ref.read(isDeveloperModeProvider.notifier).state = !isDevMode;
                  },
                  onSettingsTap: () => _navigateTo(AppRoutes.settings),
                  settingsSelected: _routeState.settingsSelected,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showCharacterSwitchSheet(BuildContext context) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (sheetContext) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: const EdgeInsets.all(AppSpacing.lg),
                child: Text('切换角色', style: AppTypography.sectionTitle(context)),
              ),
              ...MockData.characters.map((c) {
                final isSelected = c.id == ref.read(currentCharacterIdProvider);
                return ListTile(
                  leading: Container(
                    width: 40,
                    height: 40,
                    decoration: BoxDecoration(
                      color: Color(int.parse('FF${c.avatarColor.replaceAll('#', '')}', radix: 16)),
                      shape: BoxShape.circle,
                    ),
                    child: Center(child: Text(c.avatarInitial, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.w600))),
                  ),
                  title: Text(c.name),
                  subtitle: Text(c.status, style: AppTypography.caption(context)),
                  trailing: isSelected ? Icon(Icons.check_circle, color: context.accentPrimary, size: 22) : null,
                  onTap: () {
                    ref.read(currentCharacterIdProvider.notifier).state = c.id;
                    Navigator.pop(sheetContext);
                  },
                );
              }),
              const Divider(height: 1),
              ListTile(
                leading: Icon(Icons.people_outline, color: context.accentPrimary),
                title: Text('管理全部角色', style: TextStyle(color: context.accentPrimary)),
                onTap: () {
                  Navigator.pop(sheetContext);
                  _navigateTo(AppRoutes.characters);
                },
              ),
            ],
          ),
        );
      },
    );
  }
}

class _DrawerMainPanel extends StatelessWidget {
  final _CharInfo character;
  final DrawerRouteState routeState;
  final VoidCallback onMoreTap;
  final ValueChanged<String> onNavigate;
  final VoidCallback onCharacterTap;

  const _DrawerMainPanel({
    required this.character,
    required this.routeState,
    required this.onMoreTap,
    required this.onNavigate,
    required this.onCharacterTap,
  });

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: EdgeInsets.zero,
      children: [
        _CharacterArea(
          name: character.name,
          status: character.status,
          identity: character.description,
          avatarInitial: character.avatarInitial,
          avatarColor: character.avatarColor,
          onTap: onCharacterTap,
        ),
        const Divider(height: 1),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
          child: Row(
            children: [
              Expanded(
                child: _QuickAction(
                  icon: Icons.add_comment_outlined,
                  label: '新建对话',
                  onTap: () => onNavigate(AppRoutes.chat),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: _QuickAction(
                  icon: Icons.search,
                  label: '搜索会话',
                  onTap: () => onNavigate(AppRoutes.conversations),
                ),
              ),
            ],
          ),
        ),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
          child: Text('最近会话', style: AppTypography.label(context)),
        ),
        ...MockData.conversations.take(5).map((conv) => _RecentConversationTile(
              title: conv.title,
              subtitle: conv.lastMessage,
              onTap: () => onNavigate(AppRoutes.chat),
            )),
        const SizedBox(height: AppSpacing.sm),
        const Divider(height: 1),
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
          icon: Icons.apps_outlined,
          label: '更多',
          isSelected: routeState.mainItem == MainDrawerItem.more,
          showArrow: true,
          onTap: onMoreTap,
        ),
      ],
    );
  }
}

class _DrawerMorePanel extends StatelessWidget {
  final DrawerRouteState routeState;
  final bool isDevMode;
  final VoidCallback onBack;
  final ValueChanged<String> onNavigate;

  const _DrawerMorePanel({
    required this.routeState,
    required this.isDevMode,
    required this.onBack,
    required this.onNavigate,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Container(
          padding: const EdgeInsets.fromLTRB(8, 8, 16, 8),
          child: Row(
            children: [
              IconButton(
                icon: const Icon(Icons.arrow_back_ios_new, size: 18),
                onPressed: onBack,
              ),
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
              if (isDevMode)
                _MoreGroup(label: '开发与诊断', onNavigate: onNavigate, selectedItem: routeState.moreItem, items: [
                  _MoreItemData(icon: Icons.developer_mode, label: '开发者工具', route: AppRoutes.developer, item: MoreDrawerItem.developer),
                ]),
            ],
          ),
        ),
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
  final bool isDark;
  final bool isDevMode;
  final VoidCallback onToggleTheme;
  final VoidCallback onToggleDevMode;
  final VoidCallback onSettingsTap;
  final bool settingsSelected;

  const _DrawerBottomArea({
    required this.isDark,
    required this.isDevMode,
    required this.onToggleTheme,
    required this.onToggleDevMode,
    required this.onSettingsTap,
    required this.settingsSelected,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        const Divider(height: 1),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
          child: Row(
            children: [
              GestureDetector(
                onTap: () => onToggleTheme(),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(isDark ? Icons.dark_mode_outlined : Icons.light_mode_outlined, size: 18, color: context.accentPrimary),
                      const SizedBox(width: 6),
                      Text(isDark ? '暗色' : '亮色', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: onToggleDevMode,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  decoration: BoxDecoration(
                    color: isDevMode ? context.accentPrimary.withValues(alpha: 0.12) : context.surfaceSecondary,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.developer_mode, size: 18, color: isDevMode ? context.accentPrimary : context.textTertiary),
                      const SizedBox(width: 6),
                      Text('开发者', style: TextStyle(fontSize: 13, color: isDevMode ? context.accentPrimary : context.textTertiary, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              GestureDetector(
                onTap: onSettingsTap,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
                  decoration: BoxDecoration(
                    color: settingsSelected ? context.accentSoft : context.surfaceSecondary,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.settings_outlined, size: 18, color: settingsSelected ? context.accentPrimary : context.textTertiary),
                      const SizedBox(width: 6),
                      Text('设置', style: TextStyle(fontSize: 13, color: settingsSelected ? context.accentPrimary : context.textTertiary, fontWeight: FontWeight.w500)),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

class _CharacterArea extends StatelessWidget {
  final String name;
  final String status;
  final String identity;
  final String avatarInitial;
  final String avatarColor;
  final VoidCallback onTap;

  const _CharacterArea({
    required this.name,
    required this.status,
    required this.identity,
    required this.avatarInitial,
    required this.avatarColor,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final color = Color(int.parse('FF${avatarColor.replaceAll('#', '')}', radix: 16));
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(20, 16, 16, 16),
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              child: Center(
                child: Text(avatarInitial, style: const TextStyle(color: Colors.white, fontSize: 20, fontWeight: FontWeight.w600)),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(name, style: AppTypography.cardTitle(context).copyWith(fontSize: 17)),
                  const SizedBox(height: 2),
                  Text(status, style: AppTypography.label(context)),
                  const SizedBox(height: 2),
                  Text(identity, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                ],
              ),
            ),
            Icon(Icons.swap_horiz, color: context.textTertiary, size: 20),
          ],
        ),
      ),
    );
  }
}

class _QuickAction extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _QuickAction({required this.icon, required this.label, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Container(
        padding: const EdgeInsets.symmetric(vertical: 10),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brSmall,
        ),
        child: Column(
          children: [
            Icon(icon, size: 20, color: context.accentPrimary),
            const SizedBox(height: 4),
            Text(label, style: AppTypography.label(context)),
          ],
        ),
      ),
    );
  }
}

class _RecentConversationTile extends StatelessWidget {
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _RecentConversationTile({required this.title, required this.subtitle, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 8),
        child: Row(
          children: [
            Icon(Icons.chat_bubble_outline, size: 16, color: context.textTertiary),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.body(context).copyWith(fontSize: 14), maxLines: 1, overflow: TextOverflow.ellipsis),
                  Text(subtitle, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                ],
              ),
            ),
          ],
        ),
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
