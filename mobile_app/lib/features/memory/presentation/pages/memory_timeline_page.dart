import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class MemoryTimelinePage extends ConsumerStatefulWidget {
  const MemoryTimelinePage({super.key});

  @override
  ConsumerState<MemoryTimelinePage> createState() => _MemoryTimelinePageState();
}

class _MemoryTimelinePageState extends ConsumerState<MemoryTimelinePage> {
  late List<MemoryTimelineEntry> _entries;
  String _monthFilter = '全部';
  String _typeFilter = '全部';
  String _characterFilter = '全部';

  final _months = ['全部', '7月', '6月', '5月'];
  final _types = ['全部', '对话', '主动消息', '记忆形成', '情绪', '关系', '行为'];
  final _characters = ['全部', '阿米娅', '小雨', 'Epsilon', 'Karin'];

  @override
  void initState() {
    super.initState();
    _entries = List.from(MockMemory.timelineEntries);
  }

  List<MemoryTimelineEntry> get _filteredEntries {
    return _entries.where((e) {
      if (_typeFilter != '全部' && e.type != _typeFilter) return false;
      if (_characterFilter != '全部' && e.characterId != null) {
        final charName = _getCharacterName(e.characterId!);
        if (charName != _characterFilter) return false;
      }
      if (_monthFilter != '全部') {
        final monthNum = int.parse(_monthFilter.replaceAll('月', ''));
        if (e.time.month != monthNum) return false;
      }
      return true;
    }).toList();
  }

  Map<String, List<MemoryTimelineEntry>> get _groupedEntries {
    final groups = <String, List<MemoryTimelineEntry>>{};
    for (final e in _filteredEntries) {
      final dateKey = _formatDate(e.time);
      groups.putIfAbsent(dateKey, () => []).add(e);
    }
    return groups;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆时间线',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildFilters(context),
            Expanded(
              child: _filteredEntries.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.timeline,
                      title: '暂无时间线条目',
                      subtitle: '互动后将自动生成时间线',
                    )
                  : ListView(
                      padding: const EdgeInsets.all(AppSpacing.pagePadding),
                      children: [
                        ..._groupedEntries.entries.map((entry) {
                          return _buildDateGroup(context, entry.key, entry.value);
                        }),
                        const SizedBox(height: AppSpacing.xxl),
                      ],
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilters(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _buildFilterChip(context, '月份', _monthFilter, _months, (v) => setState(() => _monthFilter = v)),
            const SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '类型', _typeFilter, _types, (v) => setState(() => _typeFilter = v)),
            const SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '角色', _characterFilter, _characters, (v) => setState(() => _characterFilter = v)),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterChip(BuildContext context, String label, String current, List<String> options, ValueChanged<String> onSelected) {
    return GestureDetector(
      onTap: () => showModalBottomSheet(
        context: context,
        builder: (ctx) => Container(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('$label筛选', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: options.map((o) => GestureDetector(
                  onTap: () { onSelected(o); Navigator.pop(ctx); },
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                    decoration: BoxDecoration(
                      color: o == current ? context.accentPrimary : context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Text(o, style: TextStyle(fontSize: 14, color: o == current ? Colors.white : context.accentPrimary)),
                  ),
                )).toList(),
              ),
              const SizedBox(height: AppSpacing.xl),
            ],
          ),
        ),
      ),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('$label: $current', style: TextStyle(fontSize: 12, color: context.textSecondary)),
            const SizedBox(width: 4),
            Icon(Icons.arrow_drop_down, size: 16, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Widget _buildDateGroup(BuildContext context, String date, List<MemoryTimelineEntry> entries) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: AppSpacing.sm, top: AppSpacing.sm),
          child: Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                child: Text(date, style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w600)),
              ),
            ],
          ),
        ),
        Stack(
          children: [
            Positioned(left: 20, top: 0, bottom: 0, child: Container(width: 1.5, color: context.borderPrimary)),
            Column(children: entries.map((e) => _buildTimelineItem(context, e)).toList()),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
      ],
    );
  }

  Widget _buildTimelineItem(BuildContext context, MemoryTimelineEntry entry) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 42,
            child: Padding(
              padding: const EdgeInsets.only(top: 14),
              child: Text(
                '${entry.time.hour.toString().padLeft(2, '0')}:${entry.time.minute.toString().padLeft(2, '0')}',
                style: AppTypography.label(context),
                textAlign: TextAlign.center,
              ),
            ),
          ),
          Container(
            width: 18,
            margin: const EdgeInsets.only(top: 14),
            child: Container(
              width: 10,
              height: 10,
              decoration: BoxDecoration(
                color: _getTypeColor(context, entry.type),
                shape: BoxShape.circle,
                border: Border.all(color: context.surfacePrimary, width: 2),
              ),
            ),
          ),
          const SizedBox(width: AppSpacing.xs),
          Expanded(
            child: AmitiaCard(
              border: Border.all(
                color: entry.isImportant ? context.warning.withValues(alpha: 0.3) : context.borderPrimary,
                width: entry.isImportant ? 1 : 0.5,
              ),
              onTap: () => context.push(AppRoutes.memoryManager),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(_getTypeIcon(entry.type), size: 16, color: _getTypeColor(context, entry.type)),
                      const SizedBox(width: AppSpacing.xs),
                      Expanded(child: Text(entry.title, style: AppTypography.cardTitle(context))),
                      if (entry.isImportant)
                        Padding(
                          padding: const EdgeInsets.only(left: AppSpacing.xs),
                          child: Icon(Icons.star, size: 14, color: context.warning),
                        ),
                      AmitiaStatusBadge(label: entry.type, type: _getTypeBadge(entry.type)),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text(entry.description, style: AppTypography.caption(context)),
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      if (entry.characterId != null)
                        Text('角色：${_getCharacterName(entry.characterId!)}', style: AppTypography.label(context)),
                      if (entry.isImportant) ...[
                        const SizedBox(width: AppSpacing.sm),
                        AmitiaStatusBadge(label: '重要', type: BadgeType.warning),
                      ],
                      const Spacer(),
                      GestureDetector(
                        onTap: () => context.push(AppRoutes.memoryManager),
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                          decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(Icons.memory, size: 12, color: context.accentPrimary),
                              const SizedBox(width: 4),
                              Text('记忆详情', style: TextStyle(fontSize: 11, color: context.accentPrimary)),
                            ],
                          ),
                        ),
                      ),
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

  Color _getTypeColor(BuildContext context, String type) {
    switch (type) {
      case '对话': return context.accentPrimary;
      case '主动消息': return context.info;
      case '记忆形成': return context.success;
      case '情绪': return context.warning;
      case '关系': return context.error;
      case '行为': return context.accentSecondary;
      default: return context.textSecondary;
    }
  }

  IconData _getTypeIcon(String type) {
    switch (type) {
      case '对话': return Icons.chat_bubble_outline;
      case '主动消息': return Icons.notifications_active_outlined;
      case '记忆形成': return Icons.memory;
      case '情绪': return Icons.mood;
      case '关系': return Icons.favorite_outline;
      case '行为': return Icons.analytics_outlined;
      default: return Icons.circle_outlined;
    }
  }

  BadgeType _getTypeBadge(String type) {
    switch (type) {
      case '对话': return BadgeType.accent;
      case '主动消息': return BadgeType.info;
      case '记忆形成': return BadgeType.success;
      case '情绪': return BadgeType.warning;
      case '关系': return BadgeType.error;
      default: return BadgeType.neutral;
    }
  }

  String _getCharacterName(String id) {
    switch (id) {
      case 'c1': return '阿米娅';
      case 'c2': return '小雨';
      case 'c3': return 'Epsilon';
      case 'c4': return 'Karin';
      default: return '未知';
    }
  }

  String _formatDate(DateTime time) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final date = DateTime(time.year, time.month, time.day);
    final diff = today.difference(date).inDays;
    if (diff == 0) return '今天';
    if (diff == 1) return '昨天';
    if (diff == 2) return '前天';
    return '${time.month}月${time.day}日';
  }
}
