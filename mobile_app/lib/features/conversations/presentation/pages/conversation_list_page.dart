import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/conversation.dart';

class ConversationListPage extends ConsumerStatefulWidget {
  const ConversationListPage({super.key});

  @override
  ConsumerState<ConversationListPage> createState() => _ConversationListPageState();
}

class _ConversationListPageState extends ConsumerState<ConversationListPage> {
  final _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Map<String, List<ConversationDto>> _groupConversations(List<ConversationDto> conversations) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final yesterday = today.subtract(const Duration(days: 1));

    final todayList = <ConversationDto>[];
    final yesterdayList = <ConversationDto>[];
    final earlier = <ConversationDto>[];

    for (final conv in conversations) {
      final updatedAt = DateTime.tryParse(conv.updatedAt) ?? now;
      final convDate = DateTime(updatedAt.year, updatedAt.month, updatedAt.day);
      if (convDate == today) {
        todayList.add(conv);
      } else if (convDate == yesterday) {
        yesterdayList.add(conv);
      } else {
        earlier.add(conv);
      }
    }

    final groups = <String, List<ConversationDto>>{};
    if (todayList.isNotEmpty) groups['今天'] = todayList;
    if (yesterdayList.isNotEmpty) groups['昨天'] = yesterdayList;
    if (earlier.isNotEmpty) groups['更早'] = earlier;
    return groups;
  }

  String _formatTime(String updatedAt) {
    final time = DateTime.tryParse(updatedAt);
    if (time == null) return '';
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final convDate = DateTime(time.year, time.month, time.day);
    if (convDate == today) {
      return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
    } else if (convDate == today.subtract(const Duration(days: 1))) {
      return '昨天';
    } else {
      return '${time.month}/${time.day}';
    }
  }

  @override
  Widget build(BuildContext context) {
    final conversationsAsync = ref.watch(conversationListProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '对话',
        showBackButton: true,
      ),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppSpacing.pagePadding,
              vertical: AppSpacing.sm,
            ),
            child: Row(
              children: [
                Expanded(
                  child: Container(
                    decoration: BoxDecoration(
                      color: context.surfacePrimary,
                      borderRadius: AppRadius.brMedium,
                      border: Border.all(color: context.borderPrimary, width: 0.5),
                    ),
                    padding: const EdgeInsets.symmetric(horizontal: 12),
                    child: AmitiaSearchField(
                      controller: _searchController,
                      hintText: '搜索对话',
                      onChanged: (v) => setState(() => _searchQuery = v),
                    ),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                AmitiaIconButton(
                  icon: Icons.add_comment_outlined,
                  backgroundColor: context.accentPrimary,
                  color: Colors.white,
                  onPressed: () => context.go(AppRoutes.chat),
                ),
              ],
            ),
          ),
          Expanded(
            child: conversationsAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) => Center(
                child: Padding(
                  padding: const EdgeInsets.all(32),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                      const SizedBox(height: 16),
                      Text(
                        '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                        style: AppTypography.body(context).copyWith(color: context.error),
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 16),
                      AmitiaButton(
                        label: '重试',
                        onPressed: () => ref.invalidate(conversationListProvider),
                      ),
                    ],
                  ),
                ),
              ),
              data: (conversations) {
                final filtered = _searchQuery.isEmpty
                    ? conversations
                    : conversations.where((c) {
                        return c.title.toLowerCase().contains(_searchQuery.toLowerCase());
                      }).toList();
                final groups = _groupConversations(filtered);
                if (filtered.isEmpty) {
                  return AmitiaEmptyState(
                    icon: Icons.chat_bubble_outline,
                    title: '没有找到对话',
                    subtitle: '试试其他关键词，或者新建一个对话',
                  );
                }
                return ListView(
                  padding: const EdgeInsets.only(bottom: AppSpacing.lg),
                  children: groups.entries.map((entry) {
                    return Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Padding(
                          padding: const EdgeInsets.fromLTRB(
                            AppSpacing.pagePadding,
                            AppSpacing.md,
                            AppSpacing.pagePadding,
                            AppSpacing.xs,
                          ),
                          child: Text(entry.key, style: AppTypography.label(context)),
                        ),
                        ...entry.value.map((conv) => _ConversationItem(
                              conversation: conv,
                              timeText: _formatTime(conv.updatedAt),
                              onTap: () => context.go(AppRoutes.chat),
                              onMore: () => _showActionsSheet(conv),
                            )),
                      ],
                    );
                  }).toList(),
                );
              },
            ),
          ),
        ],
      ),
    );
  }

  void _showActionsSheet(ConversationDto conv) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.large)),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(
              AppSpacing.lg,
              AppSpacing.sm,
              AppSpacing.lg,
              AppSpacing.lg,
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 36,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                _SheetActionItem(
                  icon: Icons.edit_outlined,
                  label: '重命名',
                  onTap: () => Navigator.pop(sheetContext),
                ),
                _SheetActionItem(
                  icon: Icons.delete_outline,
                  label: '删除',
                  iconColor: context.error,
                  textColor: context.error,
                  onTap: () => Navigator.pop(sheetContext),
                ),
              ],
            ),
          ),
        );
      },
    );
  }
}

class _ConversationItem extends StatelessWidget {
  final ConversationDto conversation;
  final String timeText;
  final VoidCallback onTap;
  final VoidCallback onMore;

  const _ConversationItem({
    required this.conversation,
    required this.timeText,
    required this.onTap,
    required this.onMore,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      onLongPress: onMore,
      child: Container(
        margin: const EdgeInsets.symmetric(
          horizontal: AppSpacing.pagePadding,
          vertical: 3,
        ),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 12),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: context.accentPrimary,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  conversation.title.isNotEmpty ? conversation.title[0] : '?',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 18,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Flexible(
                    child: Text(
                      conversation.title,
                      style: AppTypography.body(context).copyWith(
                        fontWeight: FontWeight.w500,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  const SizedBox(height: 3),
                  Text(
                    '${conversation.messageCount} 条消息',
                    style: AppTypography.caption(context),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            const SizedBox(width: 8),
            Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                Text(timeText, style: AppTypography.label(context)),
                const SizedBox(height: 6),
                GestureDetector(
                  onTap: onMore,
                  child: Icon(Icons.more_horiz, size: 18, color: context.textTertiary),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _SheetActionItem extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color? iconColor;
  final Color? textColor;
  final VoidCallback onTap;

  const _SheetActionItem({
    required this.icon,
    required this.label,
    required this.onTap,
    this.iconColor,
    this.textColor,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 14),
        child: Row(
          children: [
            Icon(icon, size: 22, color: iconColor ?? context.textSecondary),
            const SizedBox(width: 16),
            Text(
              label,
              style: AppTypography.body(context).copyWith(color: textColor),
            ),
          ],
        ),
      ),
    );
  }
}
