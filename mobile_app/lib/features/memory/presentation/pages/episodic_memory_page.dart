import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/episodic.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class EpisodicMemoryPage extends ConsumerStatefulWidget {
  const EpisodicMemoryPage({super.key});

  @override
  ConsumerState<EpisodicMemoryPage> createState() => _EpisodicMemoryPageState();
}

class _EpisodicMemoryPageState extends ConsumerState<EpisodicMemoryPage> {
  int _retentionFilter = 0;
  String _decayFilter = '';

  @override
  Widget build(BuildContext context) {
    final memoriesAsync = ref.watch(episodicListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '情景记忆',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
      ),
      body: SafeArea(
        top: false,
        child: memoriesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => AmitiaErrorState(
            message: error.toString().replaceFirst('Exception: ', ''),
            onRetry: () => ref.invalidate(episodicListProvider),
          ),
          data: (memories) {
            if (memories.isEmpty) {
              return const AmitiaEmptyState(
                icon: Icons.psychology_outlined,
                title: '暂无情景记忆',
                subtitle: '对话中识别到完整场景后会自动记录',
              );
            }
            final filtered = memories.where((memory) {
              if (_retentionFilter != 0 && memory.retentionLevel != _retentionFilter) return false;
              if (_decayFilter.isNotEmpty && memory.decayState != _decayFilter) return false;
              return true;
            }).toList(growable: false);
            return Column(
              children: [
                _buildFilterBar(context),
                Expanded(
                  child: RefreshIndicator(
                    onRefresh: () async => ref.invalidate(episodicListProvider),
                    child: filtered.isEmpty
                        ? ListView(
                            padding: EdgeInsets.all(AppSpacing.pagePadding),
                            children: const [
                              SizedBox(height: 96),
                              AmitiaEmptyState(
                                icon: Icons.filter_alt_off_outlined,
                                title: '没有符合筛选条件的情景记忆',
                                subtitle: '调整 L1–L5 或状态筛选后重试',
                              ),
                            ],
                          )
                        : ListView.separated(
                            padding: EdgeInsets.fromLTRB(
                              AppSpacing.pagePadding,
                              AppSpacing.sm,
                              AppSpacing.pagePadding,
                              AppSpacing.pagePadding,
                            ),
                            itemCount: filtered.length,
                            separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                            itemBuilder: (context, index) => _buildCard(context, filtered[index]),
                          ),
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildFilterBar(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.xs,
      ),
      child: Wrap(
        spacing: AppSpacing.xs,
        runSpacing: AppSpacing.xs,
        crossAxisAlignment: WrapCrossAlignment.center,
        children: [
          ChoiceChip(
            label: const Text('全部层级'),
            selected: _retentionFilter == 0,
            onSelected: (_) => setState(() => _retentionFilter = 0),
          ),
          for (var level = 1; level <= 5; level++)
            ChoiceChip(
              label: Text('L$level'),
              selected: _retentionFilter == level,
              onSelected: (_) => setState(() => _retentionFilter = level),
            ),
          const SizedBox(width: 4),
          ChoiceChip(
            label: const Text('全部状态'),
            selected: _decayFilter.isEmpty,
            onSelected: (_) => setState(() => _decayFilter = ''),
          ),
          ChoiceChip(
            label: const Text('活跃'),
            selected: _decayFilter == 'active',
            onSelected: (_) => setState(() => _decayFilter = 'active'),
          ),
          ChoiceChip(
            label: const Text('淡化中'),
            selected: _decayFilter == 'fading',
            onSelected: (_) => setState(() => _decayFilter = 'fading'),
          ),
          ChoiceChip(
            label: const Text('已归档'),
            selected: _decayFilter == 'archived',
            onSelected: (_) => setState(() => _decayFilter = 'archived'),
          ),
        ],
      ),
    );
  }

  Widget _buildCard(BuildContext context, EpisodicDto memory) {
    final sentiment = _sentiment(memory.sentimentScore);
    final sentimentType = memory.sentimentScore >= 40
        ? BadgeType.success
        : memory.sentimentScore <= -40
            ? BadgeType.error
            : BadgeType.neutral;
    return AmitiaCard(
      onTap: () => _showDetail(context, memory),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                child: Icon(Icons.psychology_outlined, color: context.accentPrimary, size: 22),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(memory.title.isEmpty ? '未命名情景' : memory.title, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(
                      _formatTime(memory.messageTimeStart.isNotEmpty ? memory.messageTimeStart : memory.createdAt),
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: '$sentiment ${memory.sentimentScore}', type: sentimentType),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            children: [
              AmitiaStatusBadge(
                label: 'L${_retention(memory.retentionLevel)} · ${(memory.memoryStrength.clamp(0.0, 1.0) * 100).round()}%',
                type: _retentionBadge(memory.retentionLevel),
              ),
              if (memory.decayState == 'archived') const AmitiaStatusBadge(label: '已归档', type: BadgeType.neutral),
              if (memory.decayState == 'fading') const AmitiaStatusBadge(label: '淡化中', type: BadgeType.warning),
              if (memory.sceneType.isNotEmpty) AmitiaStatusBadge(label: memory.sceneType, type: BadgeType.info),
              if (memory.triggerKeywords.isNotEmpty) AmitiaStatusBadge(label: memory.triggerKeywords, type: BadgeType.neutral),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(memory.content, style: AppTypography.bodySmall(context), maxLines: 3, overflow: TextOverflow.ellipsis),
          SizedBox(height: AppSpacing.xs),
          Text(
            '强化 ${memory.reinforceCount} 次${memory.lastReinforcedAt.isEmpty ? '' : ' · 上次 ${_formatTime(memory.lastReinforcedAt)}'}',
            style: AppTypography.caption(context),
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              TextButton.icon(
                onPressed: () => _showDetail(context, memory),
                icon: const Icon(Icons.visibility_outlined, size: 16),
                label: const Text('详情'),
              ),
              PopupMenuButton<int>(
                tooltip: '调整 L1–L5',
                onSelected: (level) => _updateRetention(context, memory, level),
                itemBuilder: (_) => [
                  for (var level = 1; level <= 5; level++)
                    PopupMenuItem<int>(value: level, child: Text('L$level${level == memory.retentionLevel ? ' · 当前' : ''}')),
                ],
                child: const Padding(
                  padding: EdgeInsets.symmetric(horizontal: 8, vertical: 8),
                  child: Text('调整层级'),
                ),
              ),
              if (memory.decayState == 'archived')
                TextButton.icon(
                  onPressed: () => _restore(context, memory),
                  icon: const Icon(Icons.restore_outlined, size: 16),
                  label: const Text('恢复'),
                ),
              const Spacer(),
              TextButton.icon(
                onPressed: () => _delete(context, memory),
                icon: Icon(Icons.delete_outline, size: 16, color: context.error),
                label: Text('删除', style: TextStyle(color: context.error)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _showDetail(BuildContext context, EpisodicDto memory) {
    return showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => DraggableScrollableSheet(
        initialChildSize: 0.82,
        minChildSize: 0.5,
        maxChildSize: 0.95,
        expand: false,
        builder: (sheetContext, controller) => Padding(
          padding: EdgeInsets.all(AppSpacing.xl),
          child: ListView(
            controller: controller,
            children: [
              Text(memory.title.isEmpty ? '情景详情' : memory.title, style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              _row(context, '记忆层级', 'L${_retention(memory.retentionLevel)}'),
              _row(context, '当前强度', '${(memory.memoryStrength.clamp(0.0, 1.0) * 100).round()}%'),
              _row(context, '遗忘状态', _decayLabel(memory.decayState)),
              _row(context, '强化次数', '${memory.reinforceCount}'),
              _row(context, '上次强化', memory.lastReinforcedAt),
              _row(context, '场景类型', memory.sceneType),
              _row(context, '情感分值', '${memory.sentimentScore}（${_sentiment(memory.sentimentScore)}）'),
              _row(context, '触发关键词', memory.triggerKeywords),
              _row(context, '对话', memory.sourceConvId),
              _row(context, '开始消息', memory.messageIdStart),
              _row(context, '结束消息', memory.messageIdEnd),
              _row(context, '开始时间', memory.messageTimeStart),
              _row(context, '结束时间', memory.messageTimeEnd),
              SizedBox(height: AppSpacing.md),
              Text('上下文之前', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text(memory.contextBefore.isEmpty ? '—' : memory.contextBefore, style: AppTypography.bodySmall(context)),
              SizedBox(height: AppSpacing.md),
              Text('情景内容', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text(memory.content.isEmpty ? '—' : memory.content, style: AppTypography.bodySmall(context).copyWith(height: 1.6)),
              SizedBox(height: AppSpacing.md),
              Text('上下文之后', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text(memory.contextAfter.isEmpty ? '—' : memory.contextAfter, style: AppTypography.bodySmall(context)),
            ],
          ),
        ),
      ),
    );
  }

  Widget _row(BuildContext context, String label, String value) {
    if (value.trim().isEmpty) return const SizedBox.shrink();
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 88, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  Future<void> _updateRetention(BuildContext context, EpisodicDto memory, int level) async {
    try {
      await ref.read(episodicServiceProvider).updateRetention(memory.id, level);
      ref.invalidate(episodicListProvider);
    } catch (error) {
      if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('调整层级失败：$error')));
    }
  }

  Future<void> _restore(BuildContext context, EpisodicDto memory) async {
    try {
      await ref.read(episodicServiceProvider).restore(memory.id);
      ref.invalidate(episodicListProvider);
    } catch (error) {
      if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('恢复失败：$error')));
    }
  }

  Future<void> _delete(BuildContext context, EpisodicDto memory) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除情景记忆'),
        content: Text('确定删除“${memory.title}”吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('删除', style: TextStyle(color: context.error))),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(episodicServiceProvider).delete(memory.id);
      ref.invalidate(episodicListProvider);
    } catch (error) {
      if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败：$error')));
    }
  }

  static int _retention(int level) => level < 1 || level > 5 ? 4 : level;

  static BadgeType _retentionBadge(int level) {
    final normalized = _retention(level);
    if (normalized <= 2) return BadgeType.success;
    if (normalized == 3) return BadgeType.accent;
    if (normalized == 4) return BadgeType.warning;
    return BadgeType.neutral;
  }

  static String _decayLabel(String state) {
    switch (state) {
      case 'fading':
        return '正在淡化';
      case 'archived':
        return '已归档';
      default:
        return '活跃';
    }
  }

  static String _sentiment(int score) {
    if (score >= 40) return '正向';
    if (score <= -40) return '负向';
    return '中性';
  }

  static String _formatTime(String value) {
    if (value.isEmpty) return '';
    try {
      final date = DateTime.parse(value);
      return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')} ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return value;
    }
  }
}
