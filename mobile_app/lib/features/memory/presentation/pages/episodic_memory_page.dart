import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/episodic.dart';

class EpisodicMemoryPage extends ConsumerWidget {
  const EpisodicMemoryPage({super.key});

  String _formatTimeString(String timeStr) {
    if (timeStr.isEmpty) return '';
    try {
      final time = DateTime.parse(timeStr);
      return '${time.month}月${time.day}日 ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
    } catch (_) {
      return timeStr;
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final episodicAsync = ref.watch(episodicListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '情景记忆',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
      ),
      body: SafeArea(
        top: false,
        child: episodicAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                  const SizedBox(height: 16),
                  Text('加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                    style: AppTypography.body(context).copyWith(color: context.error),
                    textAlign: TextAlign.center),
                  const SizedBox(height: 16),
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(episodicListProvider)),
                ],
              ),
            ),
          ),
          data: (memories) {
            if (memories.isEmpty) {
              return AmitiaEmptyState(
                icon: Icons.psychology_outlined,
                title: '暂无情景记忆',
                subtitle: '互动后将自动记录情景记忆',
              );
            }
            return ListView.separated(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              itemCount: memories.length,
              separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
              itemBuilder: (context, index) => _buildMemoryCard(context, ref, memories[index]),
            );
          },
        ),
      ),
    );
  }

  Widget _buildMemoryCard(BuildContext context, WidgetRef ref, EpisodicDto memory) {
    return AmitiaCard(
      onTap: () => _showDetailSheet(context, memory),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: _getEmotionColor(context, memory.emotion).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.psychology_outlined, size: 22, color: _getEmotionColor(context, memory.emotion)),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(memory.summary.isNotEmpty ? memory.summary : memory.title, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(_formatTimeString(memory.timestamp.isNotEmpty ? memory.timestamp : memory.createdAt), style: AppTypography.caption(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: memory.emotion, type: _getEmotionBadge(memory.emotion)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(memory.content, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              GestureDetector(
                onTap: () => _showDetailSheet(context, memory),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.visibility_outlined, size: 14, color: context.accentPrimary),
                      const SizedBox(width: 4),
                      Text('详情', style: TextStyle(fontSize: 12, color: context.accentPrimary)),
                    ],
                  ),
                ),
              ),
              const Spacer(),
              GestureDetector(
                onTap: () => _showDeleteConfirm(context, ref, memory),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: context.error.withValues(alpha: 0.1), borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.delete_outline, size: 14, color: context.error),
                      const SizedBox(width: 4),
                      Text('删除', style: TextStyle(fontSize: 12, color: context.error)),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  void _showDetailSheet(BuildContext context, EpisodicDto memory) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => DraggableScrollableSheet(
        initialChildSize: 0.8,
        maxChildSize: 0.95,
        minChildSize: 0.5,
        expand: false,
        builder: (ctx, controller) => Container(
          padding: EdgeInsets.all(AppSpacing.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              SizedBox(height: AppSpacing.lg),
              Row(
                children: [
                  Container(
                    width: 56,
                    height: 56,
                    decoration: BoxDecoration(
                      color: _getEmotionColor(context, memory.emotion).withValues(alpha: 0.12),
                      borderRadius: AppRadius.brMedium,
                    ),
                    child: Icon(Icons.psychology_outlined, size: 28, color: _getEmotionColor(context, memory.emotion)),
                  ),
                  SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(memory.summary.isNotEmpty ? memory.summary : memory.title, style: AppTypography.sectionTitle(context)),
                        const SizedBox(height: 2),
                        Text(_formatTimeString(memory.timestamp.isNotEmpty ? memory.timestamp : memory.createdAt), style: AppTypography.caption(context)),
                      ],
                    ),
                  ),
                  AmitiaStatusBadge(label: memory.emotion, type: _getEmotionBadge(memory.emotion)),
                ],
              ),
              SizedBox(height: AppSpacing.lg),
              Expanded(
                child: SingleChildScrollView(
                  controller: controller,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _buildDetailSection(context, '标题', memory.title),
                      _buildDetailSection(context, '情绪', memory.emotion),
                      SizedBox(height: AppSpacing.md),
                      Text('详细内容', style: AppTypography.cardTitle(context)),
                      SizedBox(height: AppSpacing.sm),
                      Container(
                        width: double.infinity,
                        padding: EdgeInsets.all(AppSpacing.lg),
                        decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
                        child: Text(memory.content.isNotEmpty ? memory.content : memory.summary, style: AppTypography.bodySmall(context).copyWith(height: 1.6)),
                      ),
                    ],
                  ),
                ),
              ),
              SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '关闭',
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => Navigator.pop(ctx),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDetailSection(BuildContext context, String label, String value) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 80,
            child: Text(label, style: AppTypography.label(context)),
          ),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, WidgetRef ref, EpisodicDto memory) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除情景记忆', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${memory.summary.isNotEmpty ? memory.summary : memory.title}」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(episodicServiceProvider);
                await svc.delete(memory.id);
                ref.invalidate(episodicListProvider);
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('情景记忆已删除'), duration: Duration(seconds: 1)));
                }
              } catch (e) {
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                }
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  Color _getEmotionColor(BuildContext context, String emotion) {
    switch (emotion) {
      case '愉快': case '开心': case '兴奋': return context.success;
      case '关心': return context.info;
      case '满足': return context.accentPrimary;
      case '悲伤': case '难过': return context.error;
      default: return context.warning;
    }
  }

  BadgeType _getEmotionBadge(String emotion) {
    switch (emotion) {
      case '愉快': case '开心': case '兴奋': return BadgeType.success;
      case '关心': return BadgeType.info;
      case '满足': return BadgeType.accent;
      default: return BadgeType.warning;
    }
  }
}
