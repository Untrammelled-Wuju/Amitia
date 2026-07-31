import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
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

class ConversationListPage extends ConsumerStatefulWidget {
  const ConversationListPage({super.key});

  @override
  ConsumerState<ConversationListPage> createState() => _ConversationListPageState();
}

class _ConversationListPageState extends ConsumerState<ConversationListPage> {
  final _searchController = TextEditingController();
  List<Conversation> _filtered = [];

  @override
  void initState() {
    super.initState();
    _filtered = List.from(MockData.conversations);
    _searchController.addListener(_onSearch);
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _onSearch() {
    final query = _searchController.text.toLowerCase();
    setState(() {
      if (query.isEmpty) {
        _filtered = List.from(MockData.conversations);
      } else {
        _filtered = MockData.conversations.where((c) {
          return c.title.toLowerCase().contains(query) ||
              c.lastMessage.toLowerCase().contains(query);
        }).toList();
      }
    });
  }

  Map<String, List<Conversation>> _groupConversations(List<Conversation> conversations) {
    final now = DateTime.now();
    final today = DateTime(now.year, now.month, now.day);
    final yesterday = today.subtract(const Duration(days: 1));

    final pinned = <Conversation>[];
    final todayList = <Conversation>[];
    final yesterdayList = <Conversation>[];
    final earlier = <Conversation>[];

    for (final conv in conversations) {
      if (conv.isPinned) {
        pinned.add(conv);
        continue;
      }
      final convDate = DateTime(conv.lastTime.year, conv.lastTime.month, conv.lastTime.day);
      if (convDate == today) {
        todayList.add(conv);
      } else if (convDate == yesterday) {
        yesterdayList.add(conv);
      } else {
        earlier.add(conv);
      }
    }

    final groups = <String, List<Conversation>>{};
    if (pinned.isNotEmpty) groups['置顶'] = pinned;
    if (todayList.isNotEmpty) groups['今天'] = todayList;
    if (yesterdayList.isNotEmpty) groups['昨天'] = yesterdayList;
    if (earlier.isNotEmpty) groups['更早'] = earlier;
    return groups;
  }

  String _formatTime(DateTime time) {
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
    final groups = _groupConversations(_filtered);

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
            child: _filtered.isEmpty
                ? AmitiaEmptyState(
                    icon: Icons.chat_bubble_outline,
                    title: '没有找到对话',
                    subtitle: '试试其他关键词，或者新建一个对话',
                  )
                : ListView(
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
                                timeText: _formatTime(conv.lastTime),
                                onTap: () => context.go(AppRoutes.chat),
                                onMore: () => _showActionsSheet(conv),
                              )),
                        ],
                      );
                    }).toList(),
                  ),
          ),
        ],
      ),
    );
  }

  void _showActionsSheet(Conversation conv) {
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
                  icon: conv.isPinned ? Icons.push_pin_outlined : Icons.push_pin,
                  label: conv.isPinned ? '取消置顶' : '置顶',
                  onTap: () => Navigator.pop(sheetContext),
                ),
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
  final Conversation conversation;
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
    final character = MockData.characters.firstWhere(
      (c) => c.id == conversation.characterId,
      orElse: () => MockData.characters.first,
    );
    final avatarColor = Color(
      int.parse('FF${character.avatarColor.replaceAll('#', '')}', radix: 16),
    );

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
                color: avatarColor,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  character.avatarInitial,
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
                  Row(
                    children: [
                      if (conversation.isPinned) ...[
                        Icon(Icons.push_pin, size: 12, color: context.accentPrimary),
                        const SizedBox(width: 4),
                      ],
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
                    ],
                  ),
                  const SizedBox(height: 3),
                  Text(
                    conversation.lastMessage,
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
