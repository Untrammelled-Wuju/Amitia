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

class CharacterTimelinePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterTimelinePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterTimelinePage> createState() => _CharacterTimelinePageState();
}

class _CharacterTimelinePageState extends ConsumerState<CharacterTimelinePage> {
  late List<TimelineEvent> _events;

  @override
  void initState() {
    super.initState();
    _events = MockCharacters.timelineEvents(widget.characterId);
  }

  Map<String, List<TimelineEvent>> get _groupedEvents {
    final groups = <String, List<TimelineEvent>>{};
    for (final e in _events) {
      final dateKey = _formatDate(e.time);
      groups.putIfAbsent(dateKey, () => []).add(e);
    }
    return groups;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '时间线',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.memory,
            tooltip: '记忆跳转',
            onPressed: () => context.push(AppRoutes.memoryManager),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _events.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.timeline,
                title: '暂无时间线事件',
                subtitle: '角色互动后将自动生成时间线',
              )
            : ListView(
                padding: const EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  ..._groupedEvents.entries.map((entry) {
                    return _buildDateGroup(context, entry.key, entry.value);
                  }),
                  const SizedBox(height: AppSpacing.xxl),
                ],
              ),
      ),
    );
  }

  Widget _buildDateGroup(BuildContext context, String date, List<TimelineEvent> events) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.only(bottom: AppSpacing.sm, top: AppSpacing.sm),
          child: Row(
            children: [
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(date, style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w600)),
              ),
            ],
          ),
        ),
        Stack(
          children: [
            Positioned(
              left: 20,
              top: 0,
              bottom: 0,
              child: Container(
                width: 1.5,
                color: context.borderPrimary,
              ),
            ),
            Column(
              children: events.map((e) => _buildTimelineItem(context, e)).toList(),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.md),
      ],
    );
  }

  Widget _buildTimelineItem(BuildContext context, TimelineEvent event) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 42,
            padding: const EdgeInsets.only(top: 14),
            child: Text(
              '${event.time.hour.toString().padLeft(2, '0')}:${event.time.minute.toString().padLeft(2, '0')}',
              style: AppTypography.label(context),
              textAlign: TextAlign.center,
            ),
          ),
          Container(
            width: 18,
            margin: const EdgeInsets.only(top: 14),
            child: Container(
              width: 10,
              height: 10,
              decoration: BoxDecoration(
                color: _getTypeColor(context, event.type),
                shape: BoxShape.circle,
                border: Border.all(color: context.surfacePrimary, width: 2),
              ),
            ),
          ),
          const SizedBox(width: AppSpacing.xs),
          Expanded(
            child: AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(_getTypeIcon(event.type), size: 16, color: _getTypeColor(context, event.type)),
                      const SizedBox(width: AppSpacing.xs),
                      Expanded(
                        child: Text(event.title, style: AppTypography.cardTitle(context)),
                      ),
                      AmitiaStatusBadge(label: event.type, type: _getTypeBadge(event.type)),
                    ],
                  ),
                  const SizedBox(height: AppSpacing.xs),
                  Text(event.description, style: AppTypography.caption(context)),
                  if (event.emotion != null) ...[
                    const SizedBox(height: AppSpacing.xs),
                    Row(
                      children: [
                        Icon(Icons.mood_outlined, size: 14, color: context.warning),
                        const SizedBox(width: 4),
                        Text('情绪：${event.emotion}', style: AppTypography.label(context).copyWith(color: context.warning)),
                      ],
                    ),
                  ],
                  const SizedBox(height: AppSpacing.sm),
                  Row(
                    children: [
                      if (event.type == '记忆')
                        GestureDetector(
                          onTap: () => context.push(AppRoutes.memoryManager),
                          child: Container(
                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                            decoration: BoxDecoration(
                              color: context.accentSoft,
                              borderRadius: AppRadius.brTag,
                            ),
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(Icons.memory, size: 12, color: context.accentPrimary),
                                const SizedBox(width: 4),
                                Text('查看记忆', style: TextStyle(fontSize: 11, color: context.accentPrimary)),
                              ],
                            ),
                          ),
                        )
                      else
                        GestureDetector(
                          onTap: () {
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('查看「${event.title}」详情'), duration: const Duration(seconds: 1)),
                            );
                          },
                          child: Container(
                            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                            decoration: BoxDecoration(
                              color: context.accentSoft,
                              borderRadius: AppRadius.brTag,
                            ),
                            child: Row(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Icon(Icons.info_outline, size: 12, color: context.accentPrimary),
                                const SizedBox(width: 4),
                                Text('详情', style: TextStyle(fontSize: 11, color: context.accentPrimary)),
                              ],
                            ),
                          ),
                        ),
                      const Spacer(),
                      GestureDetector(
                        onTap: () {
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(content: Text('已收藏'), duration: Duration(seconds: 1)),
                          );
                        },
                        child: Icon(Icons.bookmark_border, size: 16, color: context.textTertiary),
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
      case '互动':
        return context.accentPrimary;
      case '主动消息':
        return context.info;
      case '记忆':
        return context.success;
      case '情绪':
        return context.warning;
      case '关系':
        return context.error;
      default:
        return context.textSecondary;
    }
  }

  IconData _getTypeIcon(String type) {
    switch (type) {
      case '互动':
        return Icons.chat_bubble_outline;
      case '主动消息':
        return Icons.notifications_active_outlined;
      case '记忆':
        return Icons.memory;
      case '情绪':
        return Icons.mood;
      case '关系':
        return Icons.favorite_outline;
      default:
        return Icons.circle_outlined;
    }
  }

  BadgeType _getTypeBadge(String type) {
    switch (type) {
      case '互动':
        return BadgeType.accent;
      case '主动消息':
        return BadgeType.info;
      case '记忆':
        return BadgeType.success;
      case '情绪':
        return BadgeType.warning;
      case '关系':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
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
