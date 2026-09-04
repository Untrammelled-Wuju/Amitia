import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

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
          future: ref.read(companionServiceProvider).psycheSnapshot(characterId: characterId),
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              return AmitiaErrorState(message: '加载失败: ${snapshot.error}');
            }
            final state = snapshot.data;
            if (state == null || state.isEmpty) {
              return const AmitiaEmptyState(icon: Icons.psychology_outlined, title: '暂无心理状态数据');
            }
            return _buildContent(context, state);
          },
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, Map<String, dynamic> state) {
    final emotion = _asMap(state['emotion']);
    final mood = _asMap(state['mood']);
    final needs = _asMap(state['needs']);
    final relationship = _asMap(state['relationship']);
    final beliefs = _asMapList(state['beliefs']);
    final affectLabel = (state['affectLabel'] ?? '平静').toString();
    final collectedAt = (state['collectedAt'] ?? emotion['updatedAt'] ?? mood['updatedAt'] ?? '').toString();

    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            children: [
              Container(
                width: 80,
                height: 80,
                decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
                child: Icon(Icons.psychology_outlined, size: 42, color: context.accentPrimary),
              ),
              SizedBox(height: AppSpacing.md),
              Text('当前心理状态', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Text(
                affectLabel,
                style: AppTypography.pageLargeTitle(context).copyWith(color: context.accentPrimary),
                textAlign: TextAlign.center,
              ),
              if (collectedAt.isNotEmpty) ...[
                SizedBox(height: AppSpacing.xs),
                Text('采集时间：$collectedAt', style: AppTypography.label(context), textAlign: TextAlign.center),
              ],
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '状态指标'),
        SizedBox(height: AppSpacing.sm),
        _metricCard(context, '压力', state['stress'], Icons.speed_outlined, context.warning),
        SizedBox(height: AppSpacing.sm),
        _metricCard(context, '精力', state['energy'], Icons.bolt_outlined, context.accentPrimary),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '情绪维度'),
        SizedBox(height: AppSpacing.sm),
        _dimensionGrid(context, {
          '积极': emotion['positive'],
          '消极': emotion['negative'],
          '唤醒': emotion['arousal'],
          '支配': emotion['dominance'],
          '心境效价': mood['valence'],
          '心境张力': mood['tension'],
        }),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '需求状态'),
        SizedBox(height: AppSpacing.sm),
        if (needs.isEmpty)
          Text('暂无需求状态', style: AppTypography.caption(context))
        else
          ...needs.entries.map((entry) => _needCard(context, entry.key, entry.value)),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '关系状态'),
        SizedBox(height: AppSpacing.sm),
        if (relationship.isEmpty)
          Text('暂无关系状态', style: AppTypography.caption(context))
        else
          _relationshipCard(context, relationship),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '信念快照'),
        SizedBox(height: AppSpacing.sm),
        if (beliefs.isEmpty)
          Text('暂无高置信度信念', style: AppTypography.caption(context))
        else
          ...beliefs.map((belief) => _beliefCard(context, belief)),
        SizedBox(height: AppSpacing.xxl),
      ],
    );
  }

  Widget _metricCard(BuildContext context, String label, dynamic rawValue, IconData icon, Color color) {
    final value = _toPercent(rawValue);
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 18, color: color),
              SizedBox(width: AppSpacing.sm),
              Text(label, style: AppTypography.body(context)),
              const Spacer(),
              Text('$value', style: AppTypography.cardTitle(context).copyWith(color: color)),
              Text('/100', style: AppTypography.label(context)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          AmitiaProgressBar(progress: value / 100, color: color, height: 8),
        ],
      ),
    );
  }

  Widget _dimensionGrid(BuildContext context, Map<String, dynamic> values) {
    return AmitiaCard(
      child: Wrap(
        spacing: 12,
        runSpacing: 12,
        children: values.entries.map((entry) {
          final value = _toPercent(entry.value);
          return SizedBox(
            width: 132,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(entry.key, style: AppTypography.label(context)),
                const SizedBox(height: 4),
                Text('$value/100', style: AppTypography.cardTitle(context).copyWith(color: context.accentPrimary)),
                const SizedBox(height: 6),
                AmitiaProgressBar(progress: value / 100, color: context.accentPrimary, height: 5),
              ],
            ),
          );
        }).toList(growable: false),
      ),
    );
  }

  Widget _needCard(BuildContext context, String key, dynamic rawValue) {
    final value = _toPercent(rawValue);
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(_needLabel(key), style: AppTypography.cardTitle(context))),
                Text('$value/100', style: AppTypography.label(context)),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaProgressBar(progress: value / 100, color: context.accentPrimary, height: 7),
          ],
        ),
      ),
    );
  }

  Widget _relationshipCard(BuildContext context, Map<String, dynamic> relationship) {
    const labels = <String, String>{
      'trust': '信任',
      'familiarity': '熟悉度',
      'security': '安全感',
      'tension': '紧张度',
      'repairConfidence': '修复信心',
      'boundary': '边界强度',
    };
    return AmitiaCard(
      child: Column(
        children: labels.entries.map((entry) {
          final value = _toPercent(relationship[entry.key]);
          return Padding(
            padding: EdgeInsets.only(bottom: entry.key == labels.keys.last ? 0 : AppSpacing.sm),
            child: Row(
              children: [
                SizedBox(width: 76, child: Text(entry.value, style: AppTypography.bodySmall(context))),
                Expanded(child: AmitiaProgressBar(progress: value / 100, color: context.accentPrimary, height: 6)),
                const SizedBox(width: 10),
                SizedBox(width: 48, child: Text('$value', textAlign: TextAlign.end, style: AppTypography.label(context))),
              ],
            ),
          );
        }).toList(growable: false),
      ),
    );
  }

  Widget _beliefCard(BuildContext context, Map<String, dynamic> belief) {
    final key = (belief['key'] ?? '未命名信念').toString();
    final value = (belief['value'] ?? '').toString();
    final confidence = _toPercent(belief['confidence']);
    final conflicted = belief['conflicted'] == true;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
              child: Icon(conflicted ? Icons.compare_arrows_outlined : Icons.lightbulb_outline, color: context.accentPrimary, size: 20),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(child: Text(key, style: AppTypography.cardTitle(context))),
                      AmitiaStatusBadge(label: conflicted ? '冲突' : '置信 $confidence%', type: conflicted ? BadgeType.warning : BadgeType.neutral),
                    ],
                  ),
                  if (value.isNotEmpty) ...[
                    SizedBox(height: AppSpacing.xs),
                    Text(value, style: AppTypography.caption(context)),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  static Map<String, dynamic> _asMap(dynamic value) {
    if (value is Map<String, dynamic>) return value;
    if (value is Map) return Map<String, dynamic>.from(value);
    return const <String, dynamic>{};
  }

  static List<Map<String, dynamic>> _asMapList(dynamic value) {
    if (value is! List) return const [];
    return value.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  static double _toDouble(dynamic value) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? 0;
  }

  static int _toPercent(dynamic value) {
    final numeric = _toDouble(value);
    final scaled = numeric.abs() <= 1 ? numeric * 100 : numeric;
    return scaled.clamp(0, 100).round();
  }

  static String _needLabel(String key) {
    const labels = <String, String>{
      'reassurance': '被确认',
      'connection': '连接感',
      'autonomy': '自主性',
      'clarity': '清晰度',
      'rest': '休息',
      'expression': '表达',
      'novelty': '新鲜感',
    };
    return labels[key] ?? key;
  }
}
