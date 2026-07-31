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

class CharacterPsychePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterPsychePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterPsychePage> createState() => _CharacterPsychePageState();
}

class _CharacterPsychePageState extends ConsumerState<CharacterPsychePage> {
  late List<PsycheState> _states;

  @override
  void initState() {
    super.initState();
    _states = MockCharacters.psycheStates;
  }

  PsycheState get _currentState => _states.first;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '心理状态',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildCurrentEmotionSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildMetricsSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildInfluenceSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildRelationshipSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildTimelineSection(context),
            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildCurrentEmotionSection(BuildContext context) {
    return AmitiaCard(
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
          Text(_currentState.emotion, style: AppTypography.pageLargeTitle(context).copyWith(color: context.accentPrimary)),
          const SizedBox(height: AppSpacing.xs),
          Text(_formatTime(_currentState.time), style: AppTypography.caption(context)),
        ],
      ),
    );
  }

  Widget _buildMetricsSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '状态指标'),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            children: [
              _buildMetricRow(context, '情绪强度', _currentState.intensity, context.accentPrimary, Icons.bolt),
              const Divider(height: AppSpacing.lg),
              _buildMetricRow(context, '稳定性', _currentState.stability, context.success, Icons.balance),
            ],
          ),
        ),
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

  Widget _buildInfluenceSection(BuildContext context) {
    return Column(
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
                child: Text(_currentState.influence, style: AppTypography.bodySmall(context)),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRelationshipSection(BuildContext context) {
    return Column(
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
                    Text(_currentState.relationshipStatus, style: AppTypography.cardTitle(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(label: '稳定', type: BadgeType.success),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildTimelineSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '状态时间线'),
        const SizedBox(height: AppSpacing.sm),
        ..._states.map((s) => _buildTimelineItem(context, s)),
      ],
    );
  }

  Widget _buildTimelineItem(BuildContext context, PsycheState state) {
    final isCurrent = state == _currentState;
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Column(
            children: [
              Container(
                width: 12,
                height: 12,
                decoration: BoxDecoration(
                  color: isCurrent ? context.accentPrimary : context.borderPrimary,
                  shape: BoxShape.circle,
                  border: Border.all(color: context.surfacePrimary, width: 2),
                ),
              ),
              Container(
                width: 1.5,
                height: 60,
                color: context.borderPrimary,
              ),
            ],
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: AmitiaCard(
              border: Border.all(
                color: isCurrent ? context.accentPrimary : context.borderPrimary,
                width: isCurrent ? 1 : 0.5,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(state.emotion, style: AppTypography.cardTitle(context)),
                      if (isCurrent) ...[
                        const SizedBox(width: AppSpacing.sm),
                        AmitiaStatusBadge(label: '当前', type: BadgeType.accent),
                      ],
                      const Spacer(),
                      Text(_formatTime(state.time), style: AppTypography.label(context)),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text(state.influence, style: AppTypography.caption(context)),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      _buildMiniMetric(context, '强度', state.intensity, context.accentPrimary),
                      const SizedBox(width: AppSpacing.md),
                      _buildMiniMetric(context, '稳定性', state.stability, context.success),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMiniMetric(BuildContext context, String label, int value, Color color) {
    return Expanded(
      child: Row(
        children: [
          Text(label, style: AppTypography.label(context)),
          const SizedBox(width: AppSpacing.xs),
          Expanded(
            child: AmitiaProgressBar(progress: value / 100, color: color, height: 4),
          ),
          const SizedBox(width: AppSpacing.xs),
          Text('$value', style: AppTypography.label(context).copyWith(color: color, fontWeight: FontWeight.w600)),
        ],
      ),
    );
  }

  String _formatTime(DateTime time) {
    final now = DateTime.now();
    final diff = now.difference(time);
    if (diff.inHours < 1) return '刚刚';
    if (diff.inDays == 0) return '${diff.inHours}小时前';
    if (diff.inDays < 7) return '${diff.inDays}天前';
    return '${time.month}/${time.day}';
  }
}
