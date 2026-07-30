import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import 'amitia_misc.dart';

final themeModeProvider = StateProvider<ThemeMode>((ref) => ThemeMode.light);
final currentCharacterIdProvider = StateProvider<String>((ref) => 'c1');
final isAgentModeProvider = StateProvider<bool>((ref) => false);

class DrawerMenuItem {
  final String title;
  final IconData icon;
  final String route;

  const DrawerMenuItem({required this.title, required this.icon, required this.route});
}

class DrawerMenuGroup {
  final String? label;
  final List<DrawerMenuItem> items;

  const DrawerMenuGroup({this.label, required this.items});
}

final isDeveloperModeProvider = StateProvider<bool>((ref) => false);

final drawerMenuGroups = <DrawerMenuGroup>[
  const DrawerMenuGroup(items: [
    DrawerMenuItem(title: '对话', icon: Icons.chat_bubble_outline, route: '/chat'),
    DrawerMenuItem(title: '概览', icon: Icons.dashboard_outlined, route: '/dashboard'),
    DrawerMenuItem(title: '角色', icon: Icons.people_outline, route: '/characters'),
    DrawerMenuItem(title: 'Agent', icon: Icons.auto_awesome, route: '/agent'),
    DrawerMenuItem(title: '记忆', icon: Icons.memory, route: '/memory'),
    DrawerMenuItem(title: '扩展', icon: Icons.extension_outlined, route: '/extensions'),
    DrawerMenuItem(title: '创意工坊', icon: Icons.brush_outlined, route: '/workshop'),
    DrawerMenuItem(title: '渠道连接', icon: Icons.sync_alt, route: '/channels/wechat'),
    DrawerMenuItem(title: '游戏中心', icon: Icons.sports_esports_outlined, route: '/game-center'),
    DrawerMenuItem(title: '桌宠中心', icon: Icons.pets_outlined, route: '/desktop-pet'),
  ]),
  const DrawerMenuGroup(label: '辅助功能', items: [
    DrawerMenuItem(title: '数据与备份', icon: Icons.backup_outlined, route: '/settings/backup'),
    DrawerMenuItem(title: '日程提醒', icon: Icons.notifications_active_outlined, route: '/reminders'),
    DrawerMenuItem(title: '聊天记录', icon: Icons.history_edu_outlined, route: '/chat-logs'),
    DrawerMenuItem(title: '表情包', icon: Icons.emoji_emotions_outlined, route: '/emotes'),
    DrawerMenuItem(title: '设置', icon: Icons.settings_outlined, route: '/settings'),
    DrawerMenuItem(title: '工具箱', icon: Icons.build_outlined, route: '/toolbox'),
    DrawerMenuItem(title: '关于 Amitia', icon: Icons.info_outline, route: '/about'),
  ]),
];

const developerMenuGroup = DrawerMenuGroup(label: '开发者模式', items: [
  DrawerMenuItem(title: '开发者主页', icon: Icons.developer_mode, route: '/developer'),
  DrawerMenuItem(title: 'Kernel 内核', icon: Icons.memory, route: '/developer/kernel'),
  DrawerMenuItem(title: 'WASM 模块', icon: Icons.code, route: '/developer/kernel/wasm'),
  DrawerMenuItem(title: 'Hook 系统', icon: Icons.hook, route: '/developer/kernel/hooks'),
  DrawerMenuItem(title: '可信服务', icon: Icons.verified_security, route: '/developer/kernel/trusted-services'),
  DrawerMenuItem(title: '内核任务', icon: Icons.task_alt, route: '/developer/kernel/tasks'),
  DrawerMenuItem(title: '事件总线', icon: Icons.event, route: '/developer/kernel/events'),
  DrawerMenuItem(title: '调度管理', icon: Icons.schedule, route: '/developer/kernel/schedules'),
  DrawerMenuItem(title: '桌面贡献', icon: Icons.desktop_windows, route: '/developer/kernel/desktop'),
  DrawerMenuItem(title: '更新管理', icon: Icons.system_update, route: '/developer/kernel/updates'),
  DrawerMenuItem(title: '诊断控制台', icon: Icons.terminal, route: '/developer/kernel/dev-console'),
  DrawerMenuItem(title: '数据库迁移', icon: Icons.storage, route: '/developer/kernel/migrations'),
  DrawerMenuItem(title: '开发模式设置', icon: Icons.tune, route: '/developer/kernel/dev-mode'),
]);

class AmitiaDrawer extends ConsumerStatefulWidget {
  final String currentRoute;

  const AmitiaDrawer({super.key, required this.currentRoute});

  @override
  ConsumerState<AmitiaDrawer> createState() => _AmitiaDrawerState();
}

class _AmitiaDrawerState extends ConsumerState<AmitiaDrawer> {
  @override
  Widget build(BuildContext context) {
    final themeMode = ref.watch(themeModeProvider);
    final isDark = themeMode == ThemeMode.dark;
    final isDevMode = ref.watch(isDeveloperModeProvider);
    final characterId = ref.watch(currentCharacterIdProvider);
    final character = _getCharacter(characterId);

    final allGroups = isDevMode ? [...drawerMenuGroups, developerMenuGroup] : drawerMenuGroups;

    return Material(
      color: context.surfacePrimary,
      child: SafeArea(
        child: SizedBox(
          width: MediaQuery.sizeOf(context).width * 0.82 > 340 ? 340 : MediaQuery.sizeOf(context).width * 0.82,
          child: Column(
            children: [
              _CharacterArea(
                name: character.name,
                status: character.status,
                identity: character.description,
                avatarInitial: character.avatarInitial,
                avatarColor: character.avatarColor,
                onTap: () => _navigateTo('/characters'),
              ),
              const Divider(height: 1),
              Expanded(
                child: ListView(
                  padding: const EdgeInsets.symmetric(vertical: AppSpacing.sm),
                  children: allGroups.map((group) {
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        if (group.label != null)
                          Padding(
                            padding: const EdgeInsets.only(left: 20, top: 12, bottom: 4),
                            child: Text(group.label!, style: AppTypography.label(context)),
                          ),
                        ...group.items.map((item) => _DrawerItem(
                              item: item,
                              isSelected: widget.currentRoute == item.route ||
                                  (item.route == '/chat' && widget.currentRoute.startsWith('/chat')),
                              onTap: () => _navigateTo(item.route),
                            )),
                      ],
                    );
                  }).toList(),
                ),
              ),
              const Divider(height: 1),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 12),
                child: Row(
                  children: [
                    GestureDetector(
                      onTap: () {
                        ref.read(themeModeProvider.notifier).state = isDark ? ThemeMode.light : ThemeMode.dark;
                      },
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
                      onTap: () {
                        ref.read(isDeveloperModeProvider.notifier).state = !isDevMode;
                      },
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
                    Text('v1.0.0', style: AppTypography.label(context)),
                    const SizedBox(width: 8),
                    GestureDetector(
                      onTap: () => _navigateTo('/settings/runtime'),
                      child: Container(
                        width: 32,
                        height: 32,
                        decoration: BoxDecoration(
                          color: context.success.withValues(alpha: 0.12),
                          shape: BoxShape.circle,
                        ),
                        child: Icon(Icons.circle, size: 10, color: context.success),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ),
    );
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
}

class _CharInfo {
  final String name, status, description, avatarInitial, avatarColor;
  _CharInfo(this.name, this.status, this.description, this.avatarInitial, this.avatarColor);
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
            Icon(Icons.chevron_right, color: context.textTertiary, size: 22),
          ],
        ),
      ),
    );
  }
}

class _DrawerItem extends StatelessWidget {
  final DrawerMenuItem item;
  final bool isSelected;
  final VoidCallback onTap;

  const _DrawerItem({required this.item, required this.isSelected, required this.onTap});

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
              Icon(item.icon, size: 20, color: isSelected ? context.accentPrimary : context.textSecondary),
              const SizedBox(width: 14),
              Text(
                item.title,
                style: TextStyle(
                  fontSize: 15,
                  fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
                  color: isSelected ? context.accentPrimary : context.textPrimary,
                ),
              ),
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
