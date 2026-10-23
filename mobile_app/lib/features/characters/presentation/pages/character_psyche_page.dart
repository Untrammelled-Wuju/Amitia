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

class CharacterPsychePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterPsychePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterPsychePage> createState() =>
      _CharacterPsychePageState();
}

class _CharacterPsychePageState extends ConsumerState<CharacterPsychePage> {
  late Future<Map<String, dynamic>?> _psycheFuture;
  late Future<List<Map<String, dynamic>>> _historyFuture;
  String? _deletingEmotionId;

  @override
  void initState() {
    super.initState();
    _reload();
  }

  @override
  void didUpdateWidget(covariant CharacterPsychePage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.characterId != widget.characterId) _reload();
  }

  void _reload() {
    final service = ref.read(characterDetailServiceProvider);
    _psycheFuture = service.psycheSnapshot(widget.characterId);
    _historyFuture = service.emotionHistory(widget.characterId);
  }

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
        child: FutureBuilder<Map<String, dynamic>?>(
          future: _psycheFuture,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              return AmitiaErrorState(message: '加载失败: ${snapshot.error}');
            }
            final state = snapshot.data;
            if (state == null || state.isEmpty) {
              return const AmitiaEmptyState(
                icon: Icons.psychology_outlined,
                title: '暂无心理状态数据',
              );
            }
            return _buildContent(context, state, _historyFuture);
          },
        ),
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    Map<String, dynamic> state,
    Future<List<Map<String, dynamic>>> historyFuture,
  ) {
    final emotion = _asMap(state['emotion']);
    final mood = _asMap(state['mood']);
    final needs = _asMap(state['needs']);
    final relationship = _asMap(state['relationship']);
    final beliefs = _asMapList(state['beliefs']);
    final affectLabel = (state['affectLabel'] ?? '平静').toString();
    final collectedAt =
        (state['collectedAt'] ??
                emotion['updatedAt'] ??
                mood['updatedAt'] ??
                '')
            .toString();

    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
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
                child: Icon(
                  Icons.psychology_outlined,
                  size: 42,
                  color: context.accentPrimary,
                ),
              ),
              SizedBox(height: AppSpacing.md),
              Text('当前心理状态', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Text(
                affectLabel,
                style: AppTypography.pageLargeTitle(
                  context,
                ).copyWith(color: context.accentPrimary),
                textAlign: TextAlign.center,
              ),
              if (collectedAt.isNotEmpty) ...[
                SizedBox(height: AppSpacing.xs),
                Text(
                  '采集时间：$collectedAt',
                  style: AppTypography.label(context),
                  textAlign: TextAlign.center,
                ),
              ],
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '状态指标'),
        SizedBox(height: AppSpacing.sm),
        _metricCard(
          context,
          '压力',
          state['stress'],
          Icons.speed_outlined,
          context.warning,
        ),
        SizedBox(height: AppSpacing.sm),
        _metricCard(
          context,
          '精力',
          state['energy'],
          Icons.bolt_outlined,
          context.accentPrimary,
        ),
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
        AmitiaSectionHeader(title: '情绪历史'),
        SizedBox(height: AppSpacing.sm),
        FutureBuilder<List<Map<String, dynamic>>>(
          future: historyFuture,
          builder: (context, history) {
            if (history.connectionState == ConnectionState.waiting) {
              return AmitiaCard(
                child: Row(
                  children: [
                    const SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Text('正在读取设备情绪历史', style: AppTypography.caption(context)),
                  ],
                ),
              );
            }
            if (history.hasError) {
              return AmitiaCard(
                child: Text(
                  '情绪历史不可用：${history.error}',
                  style: AppTypography.caption(context),
                ),
              );
            }
            final items = history.data ?? const <Map<String, dynamic>>[];
            if (items.isEmpty) {
              return AmitiaCard(
                child: Row(
                  children: [
                    Icon(
                      Icons.history_toggle_off_outlined,
                      color: context.textTertiary,
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Text(
                        '暂无情绪历史。开启“情绪检测”且变化达到阈值后会记录。',
                        style: AppTypography.caption(context),
                      ),
                    ),
                  ],
                ),
              );
            }
            return AmitiaCard(
              child: Column(
                children: [
                  Align(
                    alignment: Alignment.centerRight,
                    child: TextButton(
                      onPressed: _deletingEmotionId == null
                          ? () => _clearEmotionHistory(context, items)
                          : null,
                      child: const Text('清空'),
                    ),
                  ),
                  for (var index = 0; index < items.length; index++) ...[
                    _emotionHistoryRow(context, items[index]),
                    if (index != items.length - 1) const Divider(height: 1),
                  ],
                ],
              ),
            );
          },
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaSectionHeader(title: '需求状态'),
        SizedBox(height: AppSpacing.sm),
        if (needs.isEmpty)
          Text('暂无需求状态', style: AppTypography.caption(context))
        else
          ...needs.entries.map(
            (entry) => _needCard(context, entry.key, entry.value),
          ),
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

  Widget _emotionHistoryRow(BuildContext context, Map<String, dynamic> item) {
    final createdAt = DateTime.tryParse(
      (item['createdAt'] ?? '').toString(),
    )?.toLocal();
    final confidence = _toPercent(item['confidence']);
    final emotion = _emotionLabel(
      (item['emotionLabel'] ?? item['moodLabel'] ?? '').toString(),
    );
    final recovery = item['recoveryApplied'] == true
        ? ' · 恢复 ${_emotionLabel((item['recoveredMood'] ?? '').toString())}'
        : '';
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 9),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(
            Icons.auto_graph_outlined,
            size: 18,
            color: context.accentPrimary,
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '${item['affectLabel'] ?? '未标注'} · $emotion$recovery',
                  style: AppTypography.body(context),
                ),
                SizedBox(height: AppSpacing.xs),
                Text(
                  createdAt == null
                      ? ''
                      : '${createdAt.year}-${createdAt.month.toString().padLeft(2, '0')}-${createdAt.day.toString().padLeft(2, '0')} '
                            '${createdAt.hour.toString().padLeft(2, '0')}:${createdAt.minute.toString().padLeft(2, '0')}',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          Text('$confidence%', style: AppTypography.label(context)),
          SizedBox(width: AppSpacing.xs),
          IconButton(
            tooltip: '删除',
            onPressed: _deletingEmotionId == null
                ? () => _deleteEmotionHistory(context, item)
                : null,
            icon: _deletingEmotionId == item['id'].toString()
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.delete_outline, size: 18),
          ),
        ],
      ),
    );
  }

  Future<void> _deleteEmotionHistory(
    BuildContext context,
    Map<String, dynamic> item,
  ) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除情绪历史'),
        content: const Text('确定删除这条情绪历史？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _deletingEmotionId = id);
    try {
      await ref
          .read(characterDetailServiceProvider)
          .deleteEmotionHistory(widget.characterId, id);
      if (!mounted) return;
      setState(() {
        _deletingEmotionId = null;
        _historyFuture = ref
            .read(characterDetailServiceProvider)
            .emotionHistory(widget.characterId);
      });
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('情绪历史已删除')));
    } catch (error) {
      if (!mounted) return;
      setState(() => _deletingEmotionId = null);
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('删除失败：$error')));
    }
  }

  Future<void> _clearEmotionHistory(
    BuildContext context,
    List<Map<String, dynamic>> items,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('清空情绪历史'),
        content: const Text('确定清空当前角色可管理的全部情绪历史？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const Text('清空'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    setState(() => _deletingEmotionId = 'all');
    try {
      final service = ref.read(characterDetailServiceProvider);
      for (final item in items) {
        final id = (item['id'] ?? '').toString();
        if (id.isNotEmpty) {
          await service.deleteEmotionHistory(widget.characterId, id);
        }
      }
      if (!mounted) return;
      setState(() {
        _deletingEmotionId = null;
        _historyFuture = service.emotionHistory(widget.characterId);
      });
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('情绪历史已清空')));
    } catch (error) {
      if (!mounted) return;
      setState(() => _deletingEmotionId = null);
      if (!context.mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('清空失败：$error')));
    }
  }

  String _emotionLabel(String value) {
    return switch (value.trim().toLowerCase()) {
      'happy' => '愉快',
      'calm' => '平静',
      'positive' => '积极',
      'excited' => '兴奋',
      'tense' => '紧张',
      'angry' => '愤怒',
      'low' => '低落',
      'sad' => '悲伤',
      'neutral' || '' => '中性',
      _ => value,
    };
  }

  Widget _metricCard(
    BuildContext context,
    String label,
    dynamic rawValue,
    IconData icon,
    Color color,
  ) {
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
              Text(
                '$value',
                style: AppTypography.cardTitle(context).copyWith(color: color),
              ),
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
        children: values.entries
            .map((entry) {
              final value = _toPercent(entry.value);
              return SizedBox(
                width: 132,
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(entry.key, style: AppTypography.label(context)),
                    const SizedBox(height: 4),
                    Text(
                      '$value/100',
                      style: AppTypography.cardTitle(
                        context,
                      ).copyWith(color: context.accentPrimary),
                    ),
                    const SizedBox(height: 6),
                    AmitiaProgressBar(
                      progress: value / 100,
                      color: context.accentPrimary,
                      height: 5,
                    ),
                  ],
                ),
              );
            })
            .toList(growable: false),
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
                Expanded(
                  child: Text(
                    _needLabel(key),
                    style: AppTypography.cardTitle(context),
                  ),
                ),
                Text('$value/100', style: AppTypography.label(context)),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaProgressBar(
              progress: value / 100,
              color: context.accentPrimary,
              height: 7,
            ),
          ],
        ),
      ),
    );
  }

  Widget _relationshipCard(
    BuildContext context,
    Map<String, dynamic> relationship,
  ) {
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
        children: labels.entries
            .map((entry) {
              final value = _toPercent(relationship[entry.key]);
              return Padding(
                padding: EdgeInsets.only(
                  bottom: entry.key == labels.keys.last ? 0 : AppSpacing.sm,
                ),
                child: Row(
                  children: [
                    SizedBox(
                      width: 76,
                      child: Text(
                        entry.value,
                        style: AppTypography.bodySmall(context),
                      ),
                    ),
                    Expanded(
                      child: AmitiaProgressBar(
                        progress: value / 100,
                        color: context.accentPrimary,
                        height: 6,
                      ),
                    ),
                    const SizedBox(width: 10),
                    SizedBox(
                      width: 48,
                      child: Text(
                        '$value',
                        textAlign: TextAlign.end,
                        style: AppTypography.label(context),
                      ),
                    ),
                  ],
                ),
              );
            })
            .toList(growable: false),
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
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(
                conflicted
                    ? Icons.compare_arrows_outlined
                    : Icons.lightbulb_outline,
                color: context.accentPrimary,
                size: 20,
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          key,
                          style: AppTypography.cardTitle(context),
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: conflicted ? '冲突' : '置信 $confidence%',
                        type: conflicted
                            ? BadgeType.warning
                            : BadgeType.neutral,
                      ),
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
    return value
        .whereType<Map>()
        .map((e) => Map<String, dynamic>.from(e))
        .toList(growable: false);
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
