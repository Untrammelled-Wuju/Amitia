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

class CharacterPsychePage extends ConsumerWidget {
  final String characterId;

  const CharacterPsychePage({super.key, required this.characterId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '心理状态',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
      ),
      body: SafeArea(
        top: false,
        child: FutureBuilder<Map<String, dynamic>?>(
          future: ref.read(companionServiceProvider).mindState(),
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              return Center(child: Text('加载失败: ${snapshot.error}'));
            }
            final state = snapshot.data;
            if (state == null) {
              return const Center(child: Text('暂无心理状态数据'));
            }
            return _buildContent(context, state);
          },
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, Map<String, dynamic> state) {
    final emotion = state['emotion']?.toString() ?? state['mood']?.toString() ?? '未知';
    final intensity = (state['intensity'] as num?)?.toInt() ?? 50;
    final stability = (state['stability'] as num?)?.toInt() ?? 50;
    final influence = state['influence']?.toString() ?? '';
    final relationshipStatus = state['relationshipStatus']?.toString() ?? '';
    final description = state['description']?.toString() ?? '';
    final summary = state['summary']?.toString() ?? '';

    return ListView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            children: [
              Container(
                width: 80,
                height: 80,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  shape: BoxShape.circle,
                ),
                child: Icon(Icons.mood, size: 44, color: context.accentPrimary),
              ),
              const SizedBox(height: AppSpacing.md),
              Text('当前情绪', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Text(emotion, style: AppTypography.pageLargeTitle(context).copyWith(color: context.accentPrimary)),
              const SizedBox(height: AppSpacing.xs),
              if (description.isNotEmpty)
                Text(description, style: AppTypography.caption(context)),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.sectionGap),
        Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AmitiaSectionHeader(title: '状态指标'),
            const SizedBox(height: AppSpacing.sm),
            AmitiaCard(
              child: Column(
                children: [
                  _buildMetricRow(context, '情绪强度', intensity, context.accentPrimary, Icons.bolt),
                  const Divider(height: AppSpacing.lg),
                  _buildMetricRow(context, '稳定性', stability, context.success, Icons.balance),
                ],
              ),
            ),
          ],
        ),
        if (influence.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.sectionGap),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AmitiaSectionHeader(title: '影响因素'),
              const SizedBox(height: AppSpacing.sm),
              AmitiaCard(
                child: Row(
                  children: [
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: context.warning.withValues(alpha: 0.12),
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Icon(Icons.lightbulb_outline, size: 20, color: context.warning),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Text(influence, style: AppTypography.bodySmall(context)),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ],
        if (relationshipStatus.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.sectionGap),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AmitiaSectionHeader(title: '关系状态'),
              const SizedBox(height: AppSpacing.sm),
              AmitiaCard(
                child: Row(
                  children: [
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: context.accentSoft,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Icon(Icons.favorite, size: 20, color: context.accentPrimary),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('与用户关系', style: AppTypography.label(context)),
                          const SizedBox(height: 2),
                          Text(relationshipStatus, style: AppTypography.cardTitle(context)),
                        ],
                      ),
                    ),
                    AmitiaStatusBadge(label: '稳定', type: BadgeType.success),
                  ],
                ),
              ),
            ],
          ),
        ],
        if (summary.isNotEmpty) ...[
          const SizedBox(height: AppSpacing.sectionGap),
          AmitiaCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('状态总结', style: AppTypography.cardTitle(context)),
                const SizedBox(height: AppSpacing.sm),
                Text(summary, style: AppTypography.bodySmall(context)),
              ],
            ),
          ),
        ],
        const SizedBox(height: AppSpacing.xxl),
      ],
    );
  }

  Widget _buildMetricRow(BuildContext context, String label, int value, Color color, IconData icon) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Icon(icon, size: 18, color: color),
            const SizedBox(width: AppSpacing.sm),
            Text(label, style: AppTypography.body(context)),
            const Spacer(),
            Text('$value', style: AppTypography.cardTitle(context).copyWith(color: color)),
            Text('/100', style: AppTypography.label(context)),
          ],
        ),
        const SizedBox(height: AppSpacing.xs),
        AmitiaProgressBar(progress: value / 100, color: color, height: 8),
      ],
    );
  }
}
