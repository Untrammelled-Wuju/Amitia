import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/conversation.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_icons.dart';
import '../../../../core/widgets/character_avatar.dart';
import '../../../chat/runtime/conversation_runtime_controller.dart';

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

class ConversationListTile extends ConsumerWidget {
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
  Widget build(BuildContext context, WidgetRef ref) {
    final liveActivity = ref
        .watch(conversationRuntimeRegistryProvider)
        .activityFor(conversation.id);
    final role = character?.id == conversation.characterId
        ? character
        : ref
              .watch(conversationCharacterProvider(conversation.characterId))
              .valueOrNull;
    final title = conversation.title.trim().isEmpty
        ? '未命名对话'
        : conversation.title.trim();
    final isGenerating =
        liveActivity?.isGenerating == true || conversation.isGenerating;
    final unreadCount = [
      conversation.unreadCompletedCount,
      liveActivity?.unreadCompletedCount ?? 0,
    ].reduce((left, right) => left > right ? left : right);
    final livePreview = liveActivity?.preview.trim() ?? '';
    final backendPreview = conversation.lastMessage.trim().replaceAll(
      RegExp(r'\s+'),
      ' ',
    );
    final summary = livePreview.isNotEmpty
        ? livePreview
        : isGenerating && conversation.activityLabel.trim().isNotEmpty
        ? conversation.activityLabel.trim()
        : backendPreview.isNotEmpty
        ? backendPreview
        : _messageTypePreview(conversation.lastMessageType) ??
              (isGenerating ? '正在生成回复…' : '打开查看对话');
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
                          avatar: role?.avatar ?? '',
                          initial: role?.name ?? '',
                          placeholder: const Center(
                            child: Icon(
                              Icons.person_rounded,
                              color: Colors.white,
                              size: 26,
                            ),
                          ),
                          size: 46,
                          color: context.accentPrimary,
                        ),
                        if (unreadCount > 0)
                          Positioned(
                            right: -1,
                            top: -1,
                            child: Container(
                              constraints: const BoxConstraints(
                                minWidth: 18,
                                minHeight: 18,
                              ),
                              padding: const EdgeInsets.symmetric(
                                horizontal: 4,
                              ),
                              decoration: BoxDecoration(
                                color: context.error,
                                borderRadius: BorderRadius.circular(9),
                                border: Border.all(
                                  color: context.backgroundPrimary,
                                  width: 2,
                                ),
                              ),
                              alignment: Alignment.center,
                              child: Text(
                                unreadCount > 99 ? '99+' : '$unreadCount',
                                style: AppTypography.label(context).copyWith(
                                  color: Colors.white,
                                  fontSize: unreadCount > 99 ? 8 : 9,
                                  height: 1,
                                  fontWeight: FontWeight.w600,
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
                              Column(
                                crossAxisAlignment: CrossAxisAlignment.end,
                                children: [
                                  Text(
                                    timeText,
                                    style: AppTypography.label(
                                      context,
                                    ).copyWith(fontSize: 11.5, height: 1.2),
                                  ),
                                  if (isGenerating) ...[
                                    const SizedBox(height: 3),
                                    SizedBox(
                                      width: 11,
                                      height: 11,
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

  String? _messageTypePreview(String type) {
    return switch (type.trim().toLowerCase()) {
      'image' => '[图片]',
      'video' => '[视频]',
      'audio' || 'voice' => '[语音]',
      'file' => '[文件]',
      'code' => '[代码]',
      _ => null,
    };
  }
}
