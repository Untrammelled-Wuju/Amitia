import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class MemoryTimelinePage extends ConsumerStatefulWidget {
  const MemoryTimelinePage({super.key});

  @override
  ConsumerState<MemoryTimelinePage> createState() => _MemoryTimelinePageState();
}

class _MemoryTimelinePageState extends ConsumerState<MemoryTimelinePage> {
  String _monthFilter = '全部';
  String _typeFilter = '全部';
  String _characterFilter = '全部';

  late final List<String> _months = [
    '全部',
    ...List.generate(12, (index) {
      var month = DateTime.now().month - index;
      while (month <= 0) month += 12;
      return '$month月';
    }),
  ];
  final _types = [
    '全部',
    '记忆新增',
    '记忆编辑',
    '记忆删除',
    '候选已采纳',
    '候选已拒绝',
    '候选待确认',
    '记忆操作',
    '情景记忆',
  ];

  @override
  Widget build(BuildContext context) {
    final timelineAsync = ref.watch(_timelineProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆时间线',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
      ),
      body: SafeArea(
        top: false,
        child: timelineAsync.when(
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
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(_timelineProvider)),
                ],
              ),
            ),
          ),
          data: (entries) {
            final filtered = _filterEntries(entries);
            final grouped = _groupEntries(filtered);
            return Column(
              children: [
                _buildFilters(context),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.timeline,
                          title: '暂无时间线条目',
                          subtitle: '互动后将自动生成时间线',
                        )
                      : ListView(
                          padding: EdgeInsets.all(AppSpacing.pagePadding),
                          children: [
                            ...grouped.entries.map((entry) {
                              return _buildDateGroup(context, entry.key, entry.value);
                            }),
                            SizedBox(height: AppSpacing.xxl),
                          ],
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  List<_TimelineEntry> _filterEntries(List<Map<String, dynamic>> raw) {
    return raw.map((e) => _TimelineEntry.fromMap(e)).where((e) {
      if (_typeFilter != '全部' && e.type != _typeFilter) return false;
      if (_characterFilter != '全部' && e.characterId != null) {
        final charName = _getCharacterName(e.characterId!);
        if (charName != _characterFilter) return false;
      }
      if (_monthFilter != '全部') {
        final monthNum = int.tryParse(_monthFilter.replaceAll('月', '')) ?? 0;
        if (e.time.month != monthNum) return false;
      }
      return true;
    }).toList();
  }

  Map<String, List<_TimelineEntry>> _groupEntries(List<_TimelineEntry> entries) {
    final groups = <String, List<_TimelineEntry>>{};
    for (final e in entries) {
      final dateKey = _formatDate(e.time);
      groups.putIfAbsent(dateKey, () => []).add(e);
    }
    return groups;
  }

  Widget _buildFilters(BuildContext context) {
    final characters = ref.watch(characterListProvider).valueOrNull ?? const [];
    final characterOptions = ['全部', ...characters.map((e) => e.name).where((e) => e.isNotEmpty)];
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _buildFilterChip(context, '月份', _monthFilter, _months, (v) => setState(() => _monthFilter = v)),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '类型', _typeFilter, _types, (v) => setState(() => _typeFilter = v)),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '角色', _characterFilter, characterOptions, (v) => setState(() => _characterFilter = v)),
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
          padding: EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('$label筛选', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
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
              SizedBox(height: AppSpacing.xl),
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

  Widget _buildDateGroup(BuildContext context, String date, List<_TimelineEntry> entries) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.sm, top: AppSpacing.sm),
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
        SizedBox(height: AppSpacing.md),
      ],
    );
  }

  Widget _buildTimelineItem(BuildContext context, _TimelineEntry entry) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
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
          SizedBox(width: AppSpacing.xs),
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
                      SizedBox(width: AppSpacing.xs),
                      Expanded(child: Text(entry.title, style: AppTypography.cardTitle(context))),
                      if (entry.isImportant)
                        Padding(
                          padding: EdgeInsets.only(left: AppSpacing.xs),
                          child: Icon(Icons.star, size: 14, color: context.warning),
                        ),
                      AmitiaStatusBadge(label: entry.type, type: _getTypeBadge(entry.type)),
                    ],
                  ),
                  SizedBox(height: AppSpacing.xs),
                  if (entry.description.isNotEmpty)
                    Text(entry.description, style: AppTypography.caption(context)),
                  SizedBox(height: AppSpacing.sm),
                  Wrap(
                    spacing: AppSpacing.sm,
                    runSpacing: AppSpacing.xs,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      if (entry.characterId != null && entry.characterId!.isNotEmpty)
                        Text('角色：${_getCharacterName(entry.characterId!)}', style: AppTypography.label(context)),
                      if (entry.source.isNotEmpty)
                        AmitiaStatusBadge(label: '来源 ${entry.source}', type: BadgeType.info),
                      if (entry.memoryType.isNotEmpty)
                        AmitiaStatusBadge(label: entry.memoryType, type: BadgeType.neutral),
                      if (entry.isImportant)
                        AmitiaStatusBadge(label: '重要', type: BadgeType.warning),
                    ],
                  ),
                  SizedBox(height: AppSpacing.xs),
                  Row(
                    children: [
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
    if (type.contains('删除') || type.contains('拒绝')) return context.error;
    if (type.contains('编辑') || type.contains('待确认')) return context.warning;
    if (type.contains('新增') || type.contains('采纳')) return context.success;
    if (type.contains('情景')) return context.info;
    return context.accentPrimary;
  }

  IconData _getTypeIcon(String type) {
    if (type.contains('删除')) return Icons.delete_outline;
    if (type.contains('编辑')) return Icons.edit_outlined;
    if (type.contains('待确认')) return Icons.hourglass_empty;
    if (type.contains('拒绝')) return Icons.close;
    if (type.contains('采纳')) return Icons.check_circle_outline;
    if (type.contains('情景')) return Icons.auto_stories_outlined;
    return Icons.memory;
  }

  BadgeType _getTypeBadge(String type) {
    if (type.contains('删除') || type.contains('拒绝')) return BadgeType.error;
    if (type.contains('编辑') || type.contains('待确认')) return BadgeType.warning;
    if (type.contains('新增') || type.contains('采纳')) return BadgeType.success;
    if (type.contains('情景')) return BadgeType.info;
    return BadgeType.accent;
  }

  String _getCharacterName(String id) {
    final characters = ref.read(characterListProvider).valueOrNull ?? const [];
    for (final character in characters) {
      if (character.id == id) return character.name.isEmpty ? id : character.name;
    }
    return id.isEmpty ? '未知' : id;
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

final _timelineProvider = FutureProvider.autoDispose<List<Map<String, dynamic>>>((ref) async {
  final svc = ref.read(memoryServiceProvider);
  return svc.timeline();
});

class _TimelineEntry {
  final String id;
  final DateTime time;
  final String type;
  final String title;
  final String description;
  final bool isImportant;
  final String? characterId;
  final String source;
  final String memoryType;

  _TimelineEntry({
    required this.id,
    required this.time,
    required this.type,
    required this.title,
    required this.description,
    this.isImportant = false,
    this.characterId,
    this.source = '',
    this.memoryType = '',
  });

  static String _string(Map<String, dynamic> map, List<String> keys) {
    for (final key in keys) {
      final value = map[key];
      if (value != null && value.toString().trim().isNotEmpty) {
        return value.toString();
      }
    }
    return '';
  }

  static DateTime _date(Map<String, dynamic> map) {
    final raw = _string(map, const [
      'created_at',
      'createdAt',
      'message_time_start',
      'messageTimeStart',
      'timestamp',
      'time',
    ]);
    if (raw.isEmpty) return DateTime.fromMillisecondsSinceEpoch(0);
    return DateTime.tryParse(raw)?.toLocal() ?? DateTime.fromMillisecondsSinceEpoch(0);
  }

  static String _eventLabel(String eventType, String timelineType) {
    const labels = <String, String>{
      'memory_created': '记忆新增',
      'memory_edited': '记忆编辑',
      'memory_deleted': '记忆删除',
      'candidate_accepted': '候选已采纳',
      'candidate_rejected': '候选已拒绝',
      'candidate_pending': '候选待确认',
      'memory_operation': '记忆操作',
      'memory_reinforced': '记忆增强',
      'memory_merged': '记忆合并',
      'consolidation': '记忆整合',
    };
    if (eventType.isNotEmpty) return labels[eventType] ?? eventType;
    if (timelineType == 'episodic') return '情景记忆';
    return '记忆事件';
  }

  factory _TimelineEntry.fromMap(Map<String, dynamic> map) {
    final eventType = _string(map, const ['event_type', 'eventType']);
    final timelineType = _string(map, const ['timelineType', 'timeline_type']);
    final key = _string(map, const ['key', 'title', 'summary', 'scene_type', 'sceneType']);
    final value = _string(map, const [
      'value',
      'content',
      'summary',
      'context_after',
      'contextAfter',
      'key_quote',
      'keyQuote',
    ]);
    final importanceRaw = map['importance'];
    final important = map['isImportant'] == true ||
        map['is_important'] == true ||
        (importanceRaw is num && importanceRaw.toDouble() >= 0.7);

    return _TimelineEntry(
      id: _string(map, const ['id', 'memory_id', 'memoryId']),
      time: _date(map),
      type: _eventLabel(eventType, timelineType),
      title: key.isEmpty ? _eventLabel(eventType, timelineType) : key,
      description: value,
      isImportant: important,
      characterId: _string(map, const ['character_id', 'characterId']).isEmpty
          ? null
          : _string(map, const ['character_id', 'characterId']),
      source: _string(map, const ['source']),
      memoryType: _string(map, const ['memory_type', 'memoryType', 'scene_type', 'sceneType']),
    );
  }
}
