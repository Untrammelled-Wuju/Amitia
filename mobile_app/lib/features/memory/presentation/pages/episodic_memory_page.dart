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

class EpisodicMemoryPage extends ConsumerWidget {
  const EpisodicMemoryPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
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
            return RefreshIndicator(
              onRefresh: () async => ref.invalidate(episodicListProvider),
              child: ListView.separated(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: memories.length,
                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) => _buildCard(context, ref, memories[index]),
              ),
            );
          },
        ),
      ),
    );
  }

  Widget _buildCard(BuildContext context, WidgetRef ref, EpisodicDto memory) {
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
                    Text(_formatTime(memory.messageTimeStart.isNotEmpty ? memory.messageTimeStart : memory.createdAt), style: AppTypography.caption(context)),
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
              if (memory.sceneType.isNotEmpty) AmitiaStatusBadge(label: memory.sceneType, type: BadgeType.info),
              if (memory.triggerKeywords.isNotEmpty) AmitiaStatusBadge(label: memory.triggerKeywords, type: BadgeType.neutral),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(memory.content, style: AppTypography.bodySmall(context), maxLines: 3, overflow: TextOverflow.ellipsis),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              TextButton.icon(onPressed: () => _showDetail(context, memory), icon: const Icon(Icons.visibility_outlined, size: 16), label: const Text('详情')),
              const Spacer(),
              TextButton.icon(
                onPressed: () => _delete(context, ref, memory),
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

  Future<void> _delete(BuildContext context, WidgetRef ref, EpisodicDto memory) async {
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
