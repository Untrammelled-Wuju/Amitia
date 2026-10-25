import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/character_avatar.dart';

String formatConversationTime(String updatedAt) {
  final time = DateTime.tryParse(updatedAt);
  if (time == null) return '';
  final now = DateTime.now();
  final today = DateTime(now.year, now.month, now.day);
  final conversationDate = DateTime(time.year, time.month, time.day);
  if (conversationDate == today) {
    return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
  if (conversationDate == today.subtract(const Duration(days: 1))) {
    return '昨天 ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
  final dayOffset = today.difference(conversationDate).inDays;
  if (dayOffset > 0 && dayOffset < 7) {
    const weekdays = ['星期一', '星期二', '星期三', '星期四', '星期五', '星期六', '星期日'];
    return weekdays[conversationDate.weekday - 1];
  }
  if (time.year == now.year) return '${time.month}月${time.day}日';
  return '${time.year}/${time.month}/${time.day}';
}

class ConversationListTile extends StatelessWidget {
  const ConversationListTile({
    super.key,
    required this.conversation,
    required this.timeText,
    required this.showDivider,
    required this.onTap,
    this.onLongPress,
    this.character,
  });

  final CharacterDto? character;
  final ConversationDto conversation;
  final String timeText;
  final bool showDivider;
  final VoidCallback onTap;
  final VoidCallback? onLongPress;

  @override
  Widget build(BuildContext context) {
    final title = conversation.title.trim().isEmpty
        ? '未命名对话'
        : conversation.title.trim();
    final preview = conversation.lastMessage.trim().replaceAll(
      RegExp(r'\s+'),
      ' ',
    );
    final summary = preview.isNotEmpty
        ? preview
        : conversation.isGenerating
        ? '正在生成回复…'
        : conversation.hasUnread
        ? '有新消息'
        : conversation.pinnedAt.trim().isNotEmpty
        ? '已置顶'
        : conversation.messageCount > 0
        ? '${conversation.messageCount} 条消息'
        : '打开查看对话';
    final pinned = conversation.pinnedAt.trim().isNotEmpty;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        onLongPress: onLongPress,
        child: Column(
          children: [
            SizedBox(
              height: 76,
              child: Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 10,
                ),
                child: Row(
                  children: [
                    Stack(
                      clipBehavior: Clip.none,
                      children: [
                        CharacterAvatar(
                          characterId: conversation.characterId,
                          avatar: character?.avatar ?? '',
                          initial: title,
                          size: 46,
                          color: context.accentPrimary,
                        ),
                        if (conversation.hasUnread)
                          Positioned(
                            right: -1,
                            top: -1,
                            child: Container(
                              width: 10,
                              height: 10,
                              decoration: BoxDecoration(
                                color: context.error,
                                shape: BoxShape.circle,
                                border: Border.all(
                                  color: context.backgroundPrimary,
                                  width: 2,
                                ),
                              ),
                            ),
                          ),
                      ],
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        mainAxisAlignment: MainAxisAlignment.center,
                        children: [
                          Row(
                            children: [
                              if (pinned) ...[
                                AmitiaIcon(
                                  AmitiaIconGlyph.pin,
                                  size: 13,
                                  color: context.textTertiary,
                                ),
                                const SizedBox(width: 4),
                              ],
                              Expanded(
                                child: Text(
                                  title,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTypography.body(context).copyWith(
                                    fontSize: 15.5,
                                    fontWeight: FontWeight.w500,
                                    height: 1.25,
                                  ),
                                ),
                              ),
                              const SizedBox(width: 10),
                              Text(
                                timeText,
                                style: AppTypography.label(
                                  context,
                                ).copyWith(fontSize: 11.5, height: 1.2),
                              ),
                            ],
                          ),
                          const SizedBox(height: 5),
                          Row(
                            children: [
                              Expanded(
                                child: Text(
                                  summary,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: AppTypography.bodySmall(context)
                                      .copyWith(
                                        fontSize: 13,
                                        color: context.textSecondary,
                                        height: 1.3,
                                      ),
                                ),
                              ),
                              if (conversation.isGenerating) ...[
                                const SizedBox(width: 8),
                                SizedBox(
                                  width: 12,
                                  height: 12,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 1.5,
                                    color: context.accentPrimary,
                                  ),
                                ),
                              ],
                            ],
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
            if (showDivider)
              Padding(
                padding: const EdgeInsets.only(left: 74, right: 16),
                child: Divider(
                  height: 0.5,
                  thickness: 0.5,
                  color: context.borderSecondary,
                ),
              ),
          ],
        ),
      ),
    );
  }
}
