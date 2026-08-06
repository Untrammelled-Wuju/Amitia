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
import '../../../../core/services/providers.dart';

class CharacterTimelinePage extends ConsumerWidget {
  final String characterId;

  const CharacterTimelinePage({super.key, required this.characterId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '时间线',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
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
        child: FutureBuilder<List<Map<String, dynamic>>>(
          future: _loadTimeline(ref),
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              return Center(child: Text('加载失败: ${snapshot.error}'));
            }
            final events = snapshot.data ?? [];
            if (events.isEmpty) {
              return AmitiaEmptyState(
                icon: Icons.timeline,
                title: '暂无时间线事件',
                subtitle: '角色互动后将自动生成时间线',
              );
            }
            final grouped = _groupEventsByDate(events);
            return ListView(
              padding: const EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                ...grouped.entries.map((entry) => _buildDateGroup(context, entry.key, entry.value)),
                const SizedBox(height: AppSpacing.xxl),
              ],
            );
          },
        ),
      ),
    );
  }

  Future<List<Map<String, dynamic>>> _loadTimeline(WidgetRef ref) async {
    final memorySvc = ref.read(memoryServiceProvider);
    final timelineData = await memorySvc.timeline();
    final companionSvc = ref.read(companionServiceProvider);
    final fixedEvents = await companionSvc.todaySchedule();
    final allEvents = <Map<String, dynamic>>[...timelineData, ...fixedEvents];
    allEvents.sort((a, b) {
      final timeA = DateTime.tryParse(a['time']?.toString() ?? '') ?? DateTime(2000);
      final timeB = DateTime.tryParse(b['time']?.toString() ?? '') ?? DateTime(2000);
      return timeB.compareTo(timeA);
    });
    return allEvents;
  }

  Map<String, List<Map<String, dynamic>>> _groupEventsByDate(List<Map<String, dynamic>> events) {
    final groups = <String, List<Map<String, dynamic>>>{};
    for (final e in events) {
      final time = DateTime.tryParse(e['time']?.toString() ?? '') ?? DateTime.now();
      final dateKey = _formatDate(time);
      groups.putIfAbsent(dateKey, () => []).add(e);
    }
    return groups;
  }

  Widget _buildDateGroup(BuildContext context, String date, List<Map<String, dynamic>> events) {
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

  Widget _buildTimelineItem(BuildContext context, Map<String, dynamic> event) {
    final time = DateTime.tryParse(event['time']?.toString() ?? '') ?? DateTime.now();
    final type = event['type']?.toString() ?? '互动';
    final title = event['title']?.toString() ?? event['name']?.toString() ?? '';
    final description = event['description']?.toString() ?? '';
    final emotion = event['emotion']?.toString();

    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 42,
            padding: const EdgeInsets.only(top: 14),
            child: Text(
              '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}',
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
                color: _getTypeColor(context, type),
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
                      Icon(_getTypeIcon(type), size: 16, color: _getTypeColor(context, type)),
                      const SizedBox(width: AppSpacing.xs),
                      Expanded(
                        child: Text(title, style: AppTypography.cardTitle(context)),
                      ),
                      AmitiaStatusBadge(label: type, type: _getTypeBadge(type)),
                    ],
                  ),
                  if (description.isNotEmpty) ...[
                    const SizedBox(height: AppSpacing.xs),
                    Text(description, style: AppTypography.caption(context)),
                  ],
                  if (emotion != null) ...[
                    const SizedBox(height: AppSpacing.xs),
                    Row(
                      children: [
                        Icon(Icons.mood_outlined, size: 14, color: context.warning),
                        const SizedBox(width: 4),
                        Text('情绪：$emotion', style: AppTypography.label(context).copyWith(color: context.warning)),
                      ],
                    ),
                  ],
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
