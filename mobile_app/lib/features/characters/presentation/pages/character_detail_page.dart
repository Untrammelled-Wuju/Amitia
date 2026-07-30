import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class CharacterDetailPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterDetailPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterDetailPage> createState() => _CharacterDetailPageState();
}

class _CharacterDetailPageState extends ConsumerState<CharacterDetailPage> {
  int _selectedTab = 0;
  late final Character _character;
  late final TextEditingController _promptController;

  final _tabs = const ['概览', '设定', '记忆', '关系', '能力', '生活'];

  @override
  void initState() {
    super.initState();
    _character = MockData.characters.firstWhere(
      (c) => c.id == widget.characterId,
      orElse: () => MockData.characters.first,
    );
    _promptController = TextEditingController(text: _character.prompt);
  }

  @override
  void dispose() {
    _promptController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _character.name,
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.more_horiz,
            onPressed: () {},
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildInfoSection(context),
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
              child: _buildTabContent(context),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoSection(BuildContext context) {
    final color = Color(int.parse('FF${_character.avatarColor.replaceAll('#', '')}', radix: 16));
    final isOnline = _character.status == '在线';

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
                  color: color,
                  shape: BoxShape.circle,
                ),
                child: Center(
                  child: Text(
                    _character.avatarInitial,
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
                Text(_character.name, style: AppTypography.sectionTitle(context)),
                const SizedBox(height: 4),
                Row(
                  children: [
                    AmitiaStatusBadge(
                      label: _character.status,
                      type: isOnline ? BadgeType.success : BadgeType.neutral,
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Flexible(
                      child: Text(
                        '· ${_character.mood}',
                        style: AppTypography.caption(context),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  _character.description,
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

  Widget _buildTabContent(BuildContext context) {
    switch (_selectedTab) {
      case 0:
        return _buildOverviewTab(context);
      case 1:
        return _buildSettingsTab(context);
      case 2:
        return _buildMemoryTab(context);
      case 3:
        return _buildRelationTab(context);
      case 4:
        return _buildAbilityTab(context);
      case 5:
        return _buildLifeTab(context);
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildOverviewTab(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('基本资料', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '名字', _character.name),
              _buildInfoRow(context, '身份', _character.identity),
              _buildInfoRow(context, '性格', _character.personality),
              _buildInfoRow(context, '当前心情', _character.mood),
              _buildInfoRow(context, '所在位置', _character.location),
              _buildInfoRow(context, '当前活动', _character.currentActivity),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: IntrinsicHeight(
            child: Row(
              children: [
                _buildStatItem(context, '${_character.relationshipDays}', '关系天数'),
                VerticalDivider(width: 1, color: context.borderPrimary),
                _buildStatItem(context, '${_character.messageCount}', '对话数量'),
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
              ...MockData.memories.take(3).map((m) => _buildMemoryItem(context, m)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildSettingsTab(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('角色设定', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '名字', _character.name),
              _buildInfoRow(context, '身份', _character.identity),
              _buildInfoRow(context, '性格', _character.personality),
              _buildInfoRow(context, '说话方式', _character.speakingStyle),
              _buildInfoRow(context, '用户关系', _character.userRelation),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('提示词', style: AppTypography.cardTitle(context)),
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

  Widget _buildMemoryTab(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('最近记忆', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.xs),
              ...MockData.memories.take(5).map((m) => _buildMemoryItem(context, m)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRelationTab(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: IntrinsicHeight(
            child: Row(
              children: [
                _buildStatItem(context, '${_character.relationshipDays}', '关系天数'),
                VerticalDivider(width: 1, color: context.borderPrimary),
                _buildStatItem(context, '${_character.messageCount}', '对话数量'),
                VerticalDivider(width: 1, color: context.borderPrimary),
                _buildStatItem(
                  context,
                  '${(_character.messageCount / _character.relationshipDays).round()}',
                  '日均消息',
                ),
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
                  AmitiaStatusBadge(label: _character.userRelation, type: BadgeType.accent),
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

  Widget _buildAbilityTab(BuildContext context) {
    final mcpExtensions = MockData.installedExtensions
        .where((e) => e.type == ExtensionType.mcp && e.isEnabled)
        .toList();
    final skillExtensions = MockData.installedExtensions
        .where((e) => e.type == ExtensionType.skill && e.isEnabled)
        .toList();
    final pluginExtensions = MockData.installedExtensions
        .where((e) => e.type == ExtensionType.plugin)
        .toList();

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
          'MCP',
          mcpExtensions.isEmpty ? '暂无' : mcpExtensions.map((e) => e.name).join('、'),
          Icons.extension_outlined,
        ),
        const SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(
          context,
          'Skill',
          skillExtensions.isEmpty ? '暂无' : skillExtensions.map((e) => e.name).join('、'),
          Icons.auto_awesome_outlined,
        ),
        const SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(
          context,
          '插件',
          pluginExtensions.isEmpty ? '暂无' : pluginExtensions.map((e) => e.name).join('、'),
          Icons.dashboard_outlined,
        ),
      ],
    );
  }

  Widget _buildLifeTab(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('当前状态', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '当前活动', _character.currentActivity),
              _buildInfoRow(context, '所在位置', _character.location),
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
            child: Text(value, style: AppTypography.bodySmall(context)),
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
            value,
            style: AppTypography.sectionTitle(context).copyWith(color: context.accentPrimary),
          ),
          const SizedBox(height: 2),
          Text(label, style: AppTypography.label(context)),
        ],
      ),
    );
  }

  Widget _buildMemoryItem(BuildContext context, Memory memory) {
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
                  '${memory.category} · ${_formatMemoryTime(memory.time)}',
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

  String _formatMemoryTime(DateTime time) {
    final now = DateTime.now();
    final diff = now.difference(time);
    if (diff.inDays == 0) {
      return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
    } else if (diff.inDays < 7) {
      return '${diff.inDays}天前';
    }
    return '${time.month}/${time.day}';
  }
}
