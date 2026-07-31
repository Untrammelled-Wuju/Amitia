import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class EpisodicMemoryPage extends ConsumerStatefulWidget {
  const EpisodicMemoryPage({super.key});

  @override
  ConsumerState<EpisodicMemoryPage> createState() => _EpisodicMemoryPageState();
}

class _EpisodicMemoryPageState extends ConsumerState<EpisodicMemoryPage> {
  late List<EpisodicMemory> _memories;

  @override
  void initState() {
    super.initState();
    _memories = List.from(MockMemory.episodicMemories);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '情景记忆',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
      ),
      body: SafeArea(
        top: false,
        child: _memories.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.psychology_outlined,
                title: '暂无情景记忆',
                subtitle: '互动后将自动记录情景记忆',
              )
            : ListView.separated(
                padding: const EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: _memories.length,
                separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) => _buildMemoryCard(context, _memories[index]),
              ),
      ),
    );
  }

  Widget _buildMemoryCard(BuildContext context, EpisodicMemory memory) {
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
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(memory.summary, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(_formatTime(memory.time), style: AppTypography.caption(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: memory.emotion, type: _getEmotionBadge(memory.emotion)),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Container(
            padding: const EdgeInsets.all(AppSpacing.md),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: Column(
              children: [
                _buildInfoRow(context, Icons.location_on_outlined, '地点', memory.location),
                const SizedBox(height: AppSpacing.xs),
                _buildInfoRow(context, Icons.people_outline, '参与', memory.participants.join('、')),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.sm),
          Text(memory.detail, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
          const SizedBox(height: AppSpacing.sm),
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
                onTap: () => _showDeleteConfirm(context, memory),
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

  Widget _buildInfoRow(BuildContext context, IconData icon, String label, String value) {
    return Row(
      children: [
        Icon(icon, size: 14, color: context.textTertiary),
        const SizedBox(width: 4),
        Text('$label：', style: AppTypography.label(context)),
        Expanded(child: Text(value, style: AppTypography.bodySmall(context), overflow: TextOverflow.ellipsis)),
      ],
    );
  }

  void _showDetailSheet(BuildContext context, EpisodicMemory memory) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => DraggableScrollableSheet(
        initialChildSize: 0.8,
        maxChildSize: 0.95,
        minChildSize: 0.5,
        expand: false,
        builder: (ctx, controller) => Container(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              const SizedBox(height: AppSpacing.lg),
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
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(memory.summary, style: AppTypography.sectionTitle(context)),
                        const SizedBox(height: 2),
                        Text(_formatTime(memory.time), style: AppTypography.caption(context)),
                      ],
                    ),
                  ),
                  AmitiaStatusBadge(label: memory.emotion, type: _getEmotionBadge(memory.emotion)),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),
              Expanded(
                child: SingleChildScrollView(
                  controller: controller,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      _buildDetailSection(context, '时间', _formatTime(memory.time)),
                      _buildDetailSection(context, '地点', memory.location),
                      _buildDetailSection(context, '参与角色', memory.participants.join('、')),
                      _buildDetailSection(context, '情绪', memory.emotion),
                      const SizedBox(height: AppSpacing.md),
                      Text('详细内容', style: AppTypography.cardTitle(context)),
                      const SizedBox(height: AppSpacing.sm),
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(AppSpacing.lg),
                        decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brMedium),
                        child: Text(memory.detail, style: AppTypography.bodySmall(context).copyWith(height: 1.6)),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
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
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
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

  void _showDeleteConfirm(BuildContext context, EpisodicMemory memory) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除情景记忆', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${memory.summary}」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _memories.removeWhere((m) => m.id == memory.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('情景记忆已删除'), duration: Duration(seconds: 1)));
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

  String _formatTime(DateTime time) {
    return '${time.month}月${time.day}日 ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
}
