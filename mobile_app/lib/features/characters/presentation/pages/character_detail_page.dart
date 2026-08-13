import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/memory.dart';

class CharacterDetailPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterDetailPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterDetailPage> createState() => _CharacterDetailPageState();
}

class _CharacterDetailPageState extends ConsumerState<CharacterDetailPage> {
  int _selectedTab = 0;
  final _promptController = TextEditingController();

  final _tabs = const ['概览', '设定', '记忆', '关系', '能力', '生活'];

  @override
  void dispose() {
    _promptController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final characterAsync = ref.watch(characterListProvider);
    final memoriesAsync = ref.watch(memoryListProvider);
    final isDevMode = ref.watch(isDeveloperModeProvider);

    return characterAsync.when(
      loading: () => const AmitiaScaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (err, _) => AmitiaScaffold(
        body: Center(child: Text('加载失败: $err')),
      ),
      data: (characters) {
        final character = characters.cast<CharacterDto?>().firstWhere(
          (c) => c?.id == widget.characterId,
          orElse: () => characters.isNotEmpty ? characters.first : null,
        );
        if (character == null) {
          return const AmitiaScaffold(
            body: Center(child: Text('角色不存在')),
          );
        }
        _promptController.text = character.personality;

        return memoriesAsync.when(
          loading: () => const AmitiaScaffold(
            body: Center(child: CircularProgressIndicator()),
          ),
          error: (_, __) => _buildScaffold(context, character, [], isDevMode),
          data: (memories) => _buildScaffold(context, character, memories.take(5).toList(), isDevMode),
        );
      },
    );
  }

  Widget _buildScaffold(BuildContext context, CharacterDto character, List<MemoryDto> memories, bool isDevMode) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: character.name,
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.more_horiz,
            onPressed: () => _showCharacterActionsSheet(context, character),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildInfoSection(context, character),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.md,
                AppSpacing.pagePadding,
                AppSpacing.sm,
              ),
              child: AmitiaSegmentedControl(
                segments: _tabs,
                selectedIndex: _selectedTab,
                onChanged: (index) {
                  setState(() {
                    _selectedTab = index;
                  });
                },
              ),
            ),
            Expanded(
              child: _buildTabContent(context, character, memories, isDevMode),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoSection(BuildContext context, CharacterDto character) {
    final isOnline = character.status == '在线';
    final initial = character.name.isNotEmpty ? character.name[0] : '?';

    return Padding(
      padding: const EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.xs,
      ),
      child: Row(
        children: [
          Stack(
            children: [
              Container(
                width: 72,
                height: 72,
                decoration: BoxDecoration(
                  color: context.accentPrimary,
                  shape: BoxShape.circle,
                ),
                child: Center(
                  child: Text(
                    initial,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 28,
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
                    width: 18,
                    height: 18,
                    decoration: BoxDecoration(
                      color: context.success,
                      shape: BoxShape.circle,
                      border: Border.all(color: context.surfacePrimary, width: 2.5),
                    ),
                  ),
                ),
            ],
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(character.name, style: AppTypography.sectionTitle(context)),
                const SizedBox(height: 4),
                Row(
                  children: [
                    AmitiaStatusBadge(
                      label: character.status,
                      type: isOnline ? BadgeType.success : BadgeType.neutral,
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Flexible(
                      child: Text(
                        '· ${character.speakingStyle}',
                        style: AppTypography.caption(context),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  character.description,
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildTabContent(BuildContext context, CharacterDto character, List<MemoryDto> memories, bool isDevMode) {
    switch (_selectedTab) {
      case 0:
        return _buildOverviewTab(context, character, memories, isDevMode);
      case 1:
        return _buildSettingsTab(context, character);
      case 2:
        return _buildMemoryTab(context, memories);
      case 3:
        return _buildRelationTab(context, character);
      case 4:
        return _buildAbilityTab(context, character);
      case 5:
        return _buildLifeTab(context, character);
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildOverviewTab(BuildContext context, CharacterDto character, List<MemoryDto> memories, bool isDevMode) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('基本资料', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '名字', character.name),
              _buildInfoRow(context, '身份', character.identity),
              _buildInfoRow(context, '性格', character.personality),
              _buildInfoRow(context, '说话方式', character.speakingStyle),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: IntrinsicHeight(
            child: Row(
              children: [
                _buildStatItem(context, character.createdAt, '创建时间'),
                VerticalDivider(width: 1, color: context.borderPrimary),
                _buildStatItem(context, character.voiceType ?? '默认', '语音类型'),
              ],
            ),
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('最近记忆', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.xs),
              ...memories.take(3).map((m) => _buildMemoryItem(context, m)),
              if (memories.isEmpty)
                Text('暂无记忆', style: AppTypography.caption(context)),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        _buildManagementSection(context, character, isDevMode),
      ],
    );
  }

  Widget _buildSettingsTab(BuildContext context, CharacterDto character) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('角色设定', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '名字', character.name),
              _buildInfoRow(context, '身份', character.identity),
              _buildInfoRow(context, '性格', character.personality),
              _buildInfoRow(context, '说话方式', character.speakingStyle),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('性格提示词', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              AmitiaTextField(
                controller: _promptController,
                maxLines: 8,
                hintText: '输入提示词...',
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildMemoryTab(BuildContext context, List<MemoryDto> memories) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('最近记忆', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.xs),
              ...memories.map((m) => _buildMemoryItem(context, m)),
              if (memories.isEmpty)
                Text('暂无记忆', style: AppTypography.caption(context)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRelationTab(BuildContext context, CharacterDto character) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: IntrinsicHeight(
            child: Row(
              children: [
                _buildStatItem(context, character.createdAt, '创建时间'),
                VerticalDivider(width: 1, color: context.borderPrimary),
                _buildStatItem(context, character.isActive == 1 ? '是' : '否', '活跃状态'),
              ],
            ),
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('关系标签', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: [
                  AmitiaStatusBadge(label: 'AI伙伴', type: BadgeType.accent),
                  AmitiaStatusBadge(label: '高频互动', type: BadgeType.info),
                  AmitiaStatusBadge(label: '信任伙伴', type: BadgeType.success),
                  AmitiaStatusBadge(label: '共同成长', type: BadgeType.warning),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAbilityTab(BuildContext context, CharacterDto character) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        _buildAbilitySection(
          context,
          '模型',
          'GPT-4',
          Icons.psychology_outlined,
        ),
        const SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(
          context,
          '语音类型',
          character.voiceType ?? '默认',
          Icons.record_voice_over_outlined,
        ),
      ],
    );
  }

  Widget _buildLifeTab(BuildContext context, CharacterDto character) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('当前状态', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '状态', character.status),
              _buildInfoRow(context, '活跃', character.isActive == 1 ? '是' : '否'),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('作息', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildScheduleItem(context, '06:00 - 08:00', '晨间准备'),
              _buildScheduleItem(context, '08:00 - 12:00', '上午活动'),
              _buildScheduleItem(context, '12:00 - 14:00', '午间休息'),
              _buildScheduleItem(context, '14:00 - 18:00', '下午活动'),
              _buildScheduleItem(context, '18:00 - 22:00', '晚间放松'),
              _buildScheduleItem(context, '22:00 - 06:00', '夜间休眠'),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 72,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(value.isEmpty ? '-' : value, style: AppTypography.bodySmall(context)),
          ),
        ],
      ),
    );
  }

  Widget _buildStatItem(BuildContext context, String value, String label) {
    return Expanded(
      child: Column(
        children: [
          Text(
            value.isEmpty ? '-' : value,
            style: AppTypography.sectionTitle(context).copyWith(color: context.accentPrimary),
          ),
          const SizedBox(height: 2),
          Text(label, style: AppTypography.label(context)),
        ],
      ),
    );
  }

  Widget _buildMemoryItem(BuildContext context, MemoryDto memory) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 6,
            height: 6,
            margin: const EdgeInsets.only(top: 7),
            decoration: BoxDecoration(
              color: context.accentPrimary,
              shape: BoxShape.circle,
            ),
          ),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  memory.content,
                  style: AppTypography.bodySmall(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  '${memory.type} · ${memory.importance}',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAbilitySection(
    BuildContext context,
    String title,
    String content,
    IconData icon,
  ) {
    return AmitiaCard(
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(icon, size: 20, color: context.accentPrimary),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(content, style: AppTypography.caption(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildScheduleItem(BuildContext context, String time, String activity) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        children: [
          SizedBox(
            width: 120,
            child: Text(time, style: AppTypography.caption(context)),
          ),
          Expanded(
            child: Text(activity, style: AppTypography.bodySmall(context)),
          ),
        ],
      ),
    );
  }

  Widget _buildManagementSection(BuildContext context, CharacterDto character, bool isDevMode) {
    final entries = <_ManageEntry>[
      _ManageEntry(
        title: '生活规则',
        icon: Icons.rule_outlined,
        route: AppRoutes.characterLifeRules(widget.characterId),
      ),
      _ManageEntry(
        title: '语音与声音复刻',
        icon: Icons.record_voice_over_outlined,
        route: AppRoutes.characterVoice(widget.characterId),
      ),
      _ManageEntry(
        title: '角色记忆',
        icon: Icons.memory_outlined,
        route: AppRoutes.characterMemory(widget.characterId),
      ),
      _ManageEntry(
        title: '关系时间线',
        icon: Icons.timeline_outlined,
        route: AppRoutes.characterTimeline(widget.characterId),
      ),
      _ManageEntry(
        title: '主动消息',
        icon: Icons.send_outlined,
        route: AppRoutes.characterProactive(widget.characterId),
      ),
      _ManageEntry(
        title: '心理状态',
        icon: Icons.psychology_outlined,
        route: AppRoutes.characterPsyche(widget.characterId),
      ),
      if (isDevMode)
        _ManageEntry(
          title: '调试与诊断',
          icon: Icons.bug_report_outlined,
          route: AppRoutes.characterDebug(widget.characterId),
        ),
    ];

    return AmitiaCard(
      padding: const EdgeInsets.symmetric(vertical: AppSpacing.cardPadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg),
            child: Text('角色管理', style: AppTypography.cardTitle(context)),
          ),
          const SizedBox(height: AppSpacing.xs),
          ...entries.map((e) => AmitiaListTile(
                leading: _buildManagementIcon(context, e.icon),
                title: e.title,
                trailing: Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
                onTap: () => context.push(e.route),
              )),
        ],
      ),
    );
  }

  Widget _buildManagementIcon(BuildContext context, IconData icon) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: context.accentPrimary),
    );
  }

  void _showCharacterActionsSheet(BuildContext context, CharacterDto character) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text('角色操作', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.star_outline, context.accentPrimary),
                  title: '设为当前角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _setAsCurrent(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.copy_outlined, context.accentPrimary),
                  title: '复制角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _copyCharacter(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.file_download_outlined, context.accentPrimary),
                  title: '导出角色包',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _exportCharacter(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.delete_outline, context.error),
                  title: '删除角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showDeleteConfirm(context, character);
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildActionIcon(BuildContext context, IconData icon, Color color) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: color),
    );
  }

  Future<void> _setAsCurrent(CharacterDto character) async {
    ref.read(currentCharacterIdProvider.notifier).state = widget.characterId;
    final svc = ref.read(characterServiceProvider);
    await svc.setActive(widget.characterId);
    ref.invalidate(characterListProvider);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('已将「${character.name}」设为当前角色')),
      );
    }
  }

  void _copyCharacter(CharacterDto character) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('已复制角色「${character.name}」')),
    );
  }

  Future<void> _exportCharacter(CharacterDto character) async {
    final svc = ref.read(characterDetailServiceProvider);
    final result = await svc.exportPack(widget.characterId);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result != null ? '已导出角色包：${character.name}' : '导出失败')),
      );
    }
  }

  void _showDeleteConfirm(BuildContext context, CharacterDto character) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('删除角色', style: AppTypography.cardTitle(dialogContext)),
          content: Text(
            '确定要删除角色「${character.name}」吗？所有相关数据将被清除，此操作不可恢复。',
            style: AppTypography.body(dialogContext),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: dialogContext.textSecondary)),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                final svc = ref.read(characterServiceProvider);
                final ok = await svc.delete(widget.characterId);
                if (ok) {
                  ref.invalidate(characterListProvider);
                }
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(ok ? '已删除角色：${character.name}' : '删除失败'),
                      backgroundColor: ok ? null : context.error,
                    ),
                  );
                  if (ok) Navigator.of(context).pop();
                }
              },
              child: Text('删除', style: TextStyle(color: dialogContext.error)),
            ),
          ],
        );
      },
    );
  }
}

class _ManageEntry {
  final String title;
  final IconData icon;
  final String route;

  _ManageEntry({
    required this.title,
    required this.icon,
    required this.route,
  });
}
