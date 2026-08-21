import 'dart:convert';

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
          future: ref.read(companionServiceProvider).mindState(characterId: characterId),
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
    final psyche = _asMap(state['psyche']);
    final relationships = _asMapList(state['relationships']);
    final needs = _asMapList(state['needs']);

    final emotion = _humanizeStructuredValue(psyche['emotion']);
    final mood = _humanizeStructuredValue(psyche['mood']);
    final stress = _toPercent(psyche['stress']);
    final energy = _toPercent(psyche['energy']);
    final updatedAt = (psyche['updatedAt'] ?? '').toString();

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
                emotion.isNotEmpty ? emotion : (mood.isNotEmpty ? mood : '暂无情绪标签'),
                style: AppTypography.pageLargeTitle(context).copyWith(color: context.accentPrimary),
                textAlign: TextAlign.center,
              ),
              if (mood.isNotEmpty && mood != emotion) ...[
                SizedBox(height: AppSpacing.xs),
                Text('心境：$mood', style: AppTypography.caption(context), textAlign: TextAlign.center),
              ],
              if (updatedAt.isNotEmpty) ...[
                SizedBox(height: AppSpacing.xs),
                Text('更新时间：$updatedAt', style: AppTypography.label(context)),
              ],
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '状态指标'),
        SizedBox(height: AppSpacing.sm),
        _metricCard(context, '压力', stress, Icons.speed_outlined, context.warning),
        SizedBox(height: AppSpacing.sm),
        _metricCard(context, '精力', energy, Icons.bolt_outlined, context.accentPrimary),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '需求状态'),
        SizedBox(height: AppSpacing.sm),
        if (needs.isEmpty)
          Text('暂无需求状态', style: AppTypography.caption(context))
        else
          ...needs.map((need) => _needCard(context, need)),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '关系状态'),
        SizedBox(height: AppSpacing.sm),
        if (relationships.isEmpty)
          Text('暂无关系状态', style: AppTypography.caption(context))
        else
          ...relationships.map((item) => _relationshipCard(context, item)),
        SizedBox(height: AppSpacing.xxl),
      ],
    );
  }

  Widget _metricCard(BuildContext context, String label, int value, IconData icon, Color color) {
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

  Widget _needCard(BuildContext context, Map<String, dynamic> need) {
    final key = (need['needKey'] ?? need['key'] ?? '未命名需求').toString();
    final current = _toPercent(need['currentValue']);
    final baseline = _toPercent(need['baseline']);
    final trend = _toDouble(need['trend']);
    final saturated = need['saturated'] == true;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(key, style: AppTypography.cardTitle(context))),
                AmitiaStatusBadge(
                  label: saturated ? '已满足' : trend > 0 ? '上升' : trend < 0 ? '下降' : '稳定',
                  type: saturated ? BadgeType.success : BadgeType.neutral,
                ),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaProgressBar(progress: current / 100, color: context.accentPrimary, height: 7),
            SizedBox(height: AppSpacing.xs),
            Text('当前 $current · 基线 $baseline', style: AppTypography.label(context)),
          ],
        ),
      ),
    );
  }

  Widget _relationshipCard(BuildContext context, Map<String, dynamic> relationship) {
    final data = _asMap(relationship['data']);
    final title = (relationship['relation_type'] ?? relationship['relationType'] ?? relationship['target_id'] ?? relationship['targetId'] ?? '关系').toString();
    final detail = data.isNotEmpty ? _humanizeStructuredValue(data) : _humanizeStructuredValue(relationship);
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
              child: Icon(Icons.favorite_outline, color: context.accentPrimary, size: 20),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  if (detail.isNotEmpty) ...[
                    SizedBox(height: AppSpacing.xs),
                    Text(detail, style: AppTypography.caption(context), maxLines: 4, overflow: TextOverflow.ellipsis),
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

  static String _humanizeStructuredValue(dynamic value) {
    if (value == null) return '';
    if (value is String) {
      final trimmed = value.trim();
      if (trimmed.isEmpty || trimmed == '{}') return '';
      try {
        return _humanizeStructuredValue(jsonDecode(trimmed));
      } catch (_) {
        return trimmed;
      }
    }
    if (value is Map) {
      final entries = value.entries
          .where((e) => e.value != null && e.value.toString().isNotEmpty)
          .take(4)
          .map((e) => '${e.key}: ${_humanizeStructuredValue(e.value)}')
          .where((e) => !e.endsWith(': '));
      return entries.join(' · ');
    }
    if (value is List) {
      return value.take(4).map(_humanizeStructuredValue).where((e) => e.isNotEmpty).join('、');
    }
    return value.toString();
  }
}
