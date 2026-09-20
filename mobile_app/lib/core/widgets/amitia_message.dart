import 'dart:async';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_motion.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../shared/models/models.dart';
import '../../features/conversation/rendering/amitia_message_view.dart';
import '../../features/conversation/rendering/amrp.dart';
import 'amitia_button.dart';
import 'amitia_misc.dart';
import 'amitia_popup_menu.dart';

enum _UserMessageAction { showTime, reply, copy, edit }

class AmitiaAgentActivity {
  final String id;
  final String title;
  final String status;
  final String? errorCode;
  final DateTime time;

  const AmitiaAgentActivity({
    required this.id,
    required this.title,
    required this.status,
    required this.time,
    this.errorCode,
  });

  bool get isError {
    final normalized = status.trim().toUpperCase();
    return normalized == 'FAILED' ||
        normalized == 'CANCELLED' ||
        normalized == 'UNKNOWN' ||
        (errorCode ?? '').isNotEmpty;
  }

  bool get isMemoryActivity {
    final normalized = title.trim().toLowerCase();
    return normalized == 'save_memory' ||
        normalized == 'save_profile' ||
        normalized == 'save_episodic_memory' ||
        normalized == 'request_memory_consolidation';
  }

  String get displayTitle {
    switch (title.trim().toLowerCase()) {
      case 'save_memory':
        return '记忆更新';
      case 'save_profile':
        return '用户画像更新';
      case 'save_episodic_memory':
        return '情景记忆更新';
      case 'request_memory_consolidation':
        return '记忆整理';
      default:
        return title;
    }
  }
}

class AmitiaMessageBubble extends StatelessWidget {
  final ChatMessage message;
  final bool showAvatar;
  final bool showHeader;
  final bool compactBottom;
  final String? avatarInitial;
  final String? avatarColor;
  final String? characterName;
  final String? userInitial;
  final String? userAvatarColor;
  final String? userName;
  final List<AmitiaAgentActivity> agentActivities;
  final bool showThinking;
  final VoidCallback? onRetry;
  final VoidCallback? onReply;
  final VoidCallback? onCopy;
  final VoidCallback? onEdit;
  final VoidCallback? onAgentTaskTap;
  final VoidCallback? onPauseAgentTask;
  final VoidCallback? onResumeAgentTask;
  final String? agentTaskStatusLabel;

  const AmitiaMessageBubble({
    super.key,
    required this.message,
    this.showAvatar = true,
    this.showHeader = true,
    this.compactBottom = false,
    this.avatarInitial,
    this.avatarColor,
    this.characterName,
    this.userInitial,
    this.userAvatarColor,
    this.userName,
    this.agentActivities = const <AmitiaAgentActivity>[],
    this.showThinking = false,
    this.onRetry,
    this.onReply,
    this.onCopy,
    this.onEdit,
    this.onAgentTaskTap,
    this.onPauseAgentTask,
    this.onResumeAgentTask,
    this.agentTaskStatusLabel,
  });

  @override
  Widget build(BuildContext context) {
    if (message.role != MessageRole.user) {
      return Padding(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.lg,
          0,
          AppSpacing.lg,
          compactBottom ? 10 : 38,
        ),
        child: AmitiaMessageView(
          key: ValueKey<String>('amrp:${message.renderId}'),
          message: message,
          characterId: message.characterId,
          characterName: (characterName ?? '').trim().isEmpty
              ? 'Amitia'
              : characterName!.trim(),
          avatarInitial: (avatarInitial ?? '').trim().isEmpty
              ? 'A'
              : avatarInitial!.trim(),
          avatarColor: (avatarColor ?? '').trim().isEmpty
              ? '#7060E8'
              : avatarColor!.trim(),
          showAvatar: showAvatar,
          showHeader: showHeader,
          showThinking: showThinking,
          toolBlocks: [
            for (final activity in agentActivities)
              AmrpToolBlock(
                id: activity.id,
                name: activity.displayTitle,
                error: activity.errorCode ?? '',
                status: _toolStatus(activity.status),
              ),
          ],
          onRetry: onRetry,
          onReply: onReply,
          onCopy: onCopy,
        ),
      );
    }

    final messageColumn = Flexible(
      child: Builder(
        builder: (bubbleContext) => GestureDetector(
          behavior: HitTestBehavior.opaque,
          onLongPressStart: (_) => _showUserActions(bubbleContext, message),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              if ((message.replyToMessageId ?? '').isNotEmpty) ...[
                Container(
                  constraints: const BoxConstraints(maxWidth: 280),
                  margin: const EdgeInsets.only(bottom: 5),
                  padding: const EdgeInsets.symmetric(
                    horizontal: 9,
                    vertical: 6,
                  ),
                  decoration: BoxDecoration(
                    color: context.surfaceSecondary,
                    borderRadius: AppRadius.brSmall,
                    border: Border(
                      left: BorderSide(color: context.accentPrimary, width: 2),
                    ),
                  ),
                  child: Text(
                    '引用：${(message.replyToExcerpt ?? '原消息').trim()}',
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: AppTypography.label(
                      context,
                    ).copyWith(color: context.textSecondary),
                  ),
                ),
              ],
              if (message.reasoningContent.trim().isNotEmpty)
                Theme(
                  data: Theme.of(
                    context,
                  ).copyWith(dividerColor: Colors.transparent),
                  child: ExpansionTile(
                    tilePadding: EdgeInsets.zero,
                    childrenPadding: const EdgeInsets.only(bottom: 6),
                    dense: true,
                    iconColor: context.textTertiary,
                    collapsedIconColor: context.textTertiary,
                    title: Text(
                      '思考内容',
                      style: AppTypography.label(
                        context,
                      ).copyWith(color: context.textTertiary),
                    ),
                    children: [
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.symmetric(
                          horizontal: 10,
                          vertical: 8,
                        ),
                        decoration: BoxDecoration(
                          color: context.surfaceSecondary,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: SelectableText(
                          message.reasoningContent,
                          style: AppTypography.bodySmall(
                            context,
                          ).copyWith(color: context.textSecondary),
                        ),
                      ),
                    ],
                  ),
                ),
              _buildContent(context, true),
              if (message.status == MessageStatus.error)
                Padding(
                  padding: const EdgeInsets.only(top: 4),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.error_outline, size: 14, color: context.error),
                      const SizedBox(width: 4),
                      GestureDetector(
                        onTap: onRetry,
                        behavior: HitTestBehavior.opaque,
                        child: Text(
                          '重试',
                          style: TextStyle(fontSize: 12, color: context.error),
                        ),
                      ),
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );

    return Padding(
      padding: EdgeInsets.only(
        left: AppSpacing.lg,
        right: AppSpacing.lg,
        bottom: 14,
      ),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.end,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [messageColumn],
      ),
    );
  }

  Future<void> _showUserActions(
    BuildContext context,
    ChatMessage message,
  ) async {
    final renderObject = context.findRenderObject();
    if (renderObject is! RenderBox || !renderObject.hasSize) return;
    final topLeft = renderObject.localToGlobal(Offset.zero);
    final bottomRight = renderObject.localToGlobal(
      renderObject.size.bottomRight(Offset.zero),
    );
    final action = await showAmitiaPopupMenu<_UserMessageAction>(
      context: context,
      anchorRect: Rect.fromPoints(topLeft, bottomRight),
      menuWidth: 180,
      itemHorizontalMargin: 0,
      itemHorizontalPadding: 14,
      itemVerticalPadding: 0,
      itemFontSize: 14.5,
      itemIconSize: 18,
      itemMinHeight: 46,
      items: [
        const PopupMenuItem(
          value: _UserMessageAction.showTime,
          child: _UserMessageMenuLabel(
            icon: Icons.schedule_rounded,
            label: '显示时间',
          ),
        ),
        PopupMenuItem(
          value: _UserMessageAction.reply,
          enabled: onReply != null,
          child: const _UserMessageMenuLabel(
            icon: Icons.format_quote_rounded,
            label: '引用',
          ),
        ),
        PopupMenuItem(
          value: _UserMessageAction.copy,
          enabled: onCopy != null,
          child: const _UserMessageMenuLabel(
            icon: Icons.copy_outlined,
            label: '复制',
          ),
        ),
        PopupMenuItem(
          value: _UserMessageAction.edit,
          enabled: onEdit != null,
          child: const _UserMessageMenuLabel(
            icon: Icons.edit_outlined,
            label: '修改',
          ),
        ),
      ],
    );
    if (!context.mounted || action == null) return;
    switch (action) {
      case _UserMessageAction.showTime:
        amitiaSnackBar(context, '发送时间：${_formatDateTime(message.time)}');
        return;
      case _UserMessageAction.reply:
        onReply?.call();
        return;
      case _UserMessageAction.copy:
        onCopy?.call();
        return;
      case _UserMessageAction.edit:
        onEdit?.call();
        return;
    }
  }

  Widget _buildContent(BuildContext context, bool isUser) {
    if (!isUser &&
        (showThinking ||
            agentActivities.isNotEmpty ||
            message.type == MessageType.toolCall)) {
      final activities = <AmitiaAgentActivity>[
        ...agentActivities,
        if (message.type == MessageType.toolCall)
          AmitiaAgentActivity(
            id: message.id,
            title: (message.toolName ?? '').trim().isEmpty
                ? '工具调用'
                : message.toolName!.trim(),
            status: message.status == MessageStatus.error
                ? 'failed'
                : 'completed',
            errorCode: message.status == MessageStatus.error
                ? message.toolResult
                : null,
            time: message.time,
          ),
      ];
      return _UnifiedAgentMessage(
        finalText: message.type == MessageType.toolCall ? '' : message.content,
        activities: activities,
        showThinking: showThinking,
      );
    }

    switch (message.type) {
      case MessageType.image:
        return _ImageMessage(
          name: message.fileName ?? '图片',
          url: message.mediaUrl,
          isUser: isUser,
        );
      case MessageType.video:
        return _VideoMessage(
          title: message.fileName ?? '视频',
          durationMs: message.durationMs ?? 0,
          isUser: isUser,
        );
      case MessageType.audio:
        return _AudioMessage(
          title: message.fileName ?? '语音消息',
          durationMs: message.durationMs ?? 0,
          isUser: isUser,
        );
      case MessageType.emote:
        return _EmoteMessage(emoji: message.content, name: '', isUser: isUser);
      case MessageType.code:
        final parsed = _parseCodeFence(message.content);
        return _CodeMessage(lang: parsed.$1, body: parsed.$2, isUser: isUser);
      case MessageType.file:
        return _FileMessage(
          fileName: message.fileName ?? message.content,
          fileSizeKB: message.fileSizeKB ?? 0,
          isUser: isUser,
        );
      case MessageType.text:
      case MessageType.agentTask:
      case MessageType.toolCall:
      case MessageType.systemNotice:
        break;
    }
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * (isUser ? 0.76 : 0.80),
      ),
      padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 10),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: BorderRadius.circular(17),
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Text(
        message.content,
        style: AppTypography.bodySmall(context).copyWith(
          color: isUser ? context.accentPressed : context.textPrimary,
          height: 1.52,
        ),
      ),
    );
  }

  String _formatTime(DateTime value) {
    final hour = value.hour.toString().padLeft(2, '0');
    final minute = value.minute.toString().padLeft(2, '0');
    return '$hour:$minute';
  }

  String _formatDateTime(DateTime value) {
    final month = value.month.toString().padLeft(2, '0');
    final day = value.day.toString().padLeft(2, '0');
    return '${value.year}-$month-$day ${_formatTime(value)}';
  }

  (String, String) _parseCodeFence(String content) {
    final trimmed = content.trim();
    if (!trimmed.startsWith('```') || !trimmed.endsWith('```')) {
      return ('text', content);
    }
    final withoutPrefix = trimmed.substring(3);
    final newline = withoutPrefix.indexOf('\n');
    if (newline < 0)
      return ('text', withoutPrefix.replaceFirst(RegExp(r'```$'), ''));
    final language = withoutPrefix.substring(0, newline).trim();
    final body = withoutPrefix.substring(newline + 1, withoutPrefix.length - 3);
    return (language.isEmpty ? 'text' : language, body);
  }

  AmrpToolStatus _toolStatus(String value) {
    switch (value.trim().toUpperCase()) {
      case 'QUEUED':
      case 'PENDING':
        return AmrpToolStatus.queued;
      case 'RUNNING':
      case 'SENDING':
        return AmrpToolStatus.running;
      case 'CANCELLED':
      case 'CANCELED':
        return AmrpToolStatus.cancelled;
      case 'FAILED':
      case 'ERROR':
      case 'UNKNOWN':
        return AmrpToolStatus.failed;
      default:
        return AmrpToolStatus.success;
    }
  }
}

class _UserMessageMenuLabel extends StatelessWidget {
  final IconData icon;
  final String label;

  const _UserMessageMenuLabel({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Icon(icon, color: context.textSecondary),
        const SizedBox(width: 10),
        Text(label),
      ],
    );
  }
}

class _UnifiedAgentMessage extends StatefulWidget {
  final String finalText;
  final List<AmitiaAgentActivity> activities;
  final bool showThinking;

  const _UnifiedAgentMessage({
    required this.finalText,
    required this.activities,
    required this.showThinking,
  });

  @override
  State<_UnifiedAgentMessage> createState() => _UnifiedAgentMessageState();
}

class _UnifiedAgentMessageState extends State<_UnifiedAgentMessage> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final count = widget.activities.length;
    final hasProcess = widget.showThinking || count > 0;
    final hasFinal = widget.finalText.trim().isNotEmpty;
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.80,
      ),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(17),
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          if (hasProcess)
            GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTap: count > 0
                  ? () => setState(() => _expanded = !_expanded)
                  : null,
              child: Padding(
                padding: const EdgeInsets.symmetric(vertical: 1),
                child: Row(
                  children: [
                    Container(
                      width: 28,
                      height: 28,
                      decoration: BoxDecoration(
                        color: context.surfaceSecondary,
                        borderRadius: BorderRadius.circular(9),
                      ),
                      alignment: Alignment.center,
                      child: widget.showThinking && count == 0
                          ? SizedBox(
                              width: 14,
                              height: 14,
                              child: CircularProgressIndicator(
                                strokeWidth: 1.6,
                                color: context.accentPrimary,
                              ),
                            )
                          : Icon(
                              Icons.auto_awesome_outlined,
                              size: 15,
                              color: context.accentPrimary,
                            ),
                    ),
                    const SizedBox(width: 9),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            widget.showThinking ? 'AI 正在生成回复' : '执行过程',
                            style: AppTypography.bodySmall(context).copyWith(
                              fontWeight: FontWeight.w600,
                              fontSize: 12,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            count == 0
                                ? '正在思考并组织回复'
                                : widget.showThinking
                                ? '正在处理 · 已完成 $count 个工具调用'
                                : '已完成 $count 个工具调用',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.label(
                              context,
                            ).copyWith(fontSize: 9.5),
                          ),
                        ],
                      ),
                    ),
                    if (count > 0) ...[
                      Text(
                        '$count 项',
                        style: AppTypography.label(
                          context,
                        ).copyWith(fontSize: 9.5),
                      ),
                      const SizedBox(width: 3),
                      AnimatedRotation(
                        turns: _expanded ? 0.25 : 0,
                        duration: AppMotion.standard,
                        child: Icon(
                          Icons.chevron_right,
                          size: 16,
                          color: context.textTertiary,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          if (_expanded && count > 0) ...[
            const SizedBox(height: 8),
            Container(height: 1, color: context.borderPrimary),
            const SizedBox(height: 7),
            for (final activity in widget.activities)
              _AgentActivityRow(activity: activity),
          ],
          if (hasProcess && hasFinal) ...[
            const SizedBox(height: 9),
            Container(height: 1, color: context.borderPrimary),
            const SizedBox(height: 9),
          ],
          if (hasFinal)
            Text(
              widget.finalText,
              style: AppTypography.bodySmall(
                context,
              ).copyWith(color: context.textPrimary, height: 1.52),
            ),
        ],
      ),
    );
  }
}

class _AgentActivityRow extends StatelessWidget {
  final AmitiaAgentActivity activity;
  const _AgentActivityRow({required this.activity});

  @override
  Widget build(BuildContext context) {
    final failed = activity.isError;
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 26,
            height: 26,
            decoration: BoxDecoration(
              color: failed
                  ? context.error.withValues(alpha: 0.10)
                  : context.surfaceSecondary,
              borderRadius: BorderRadius.circular(8),
            ),
            alignment: Alignment.center,
            child: Icon(
              failed
                  ? Icons.error_outline
                  : activity.isMemoryActivity
                  ? Icons.memory_outlined
                  : Icons.build_outlined,
              size: 14,
              color: failed ? context.error : context.textSecondary,
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  activity.displayTitle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: AppTypography.bodySmall(
                    context,
                  ).copyWith(fontWeight: FontWeight.w600, fontSize: 11),
                ),
                const SizedBox(height: 2),
                Text(
                  failed
                      ? ((activity.errorCode ?? '').isEmpty
                            ? '执行失败'
                            : '执行失败 · ${activity.errorCode}')
                      : activity.isMemoryActivity
                      ? '已更新记忆'
                      : '执行完成',
                  style: AppTypography.label(context).copyWith(
                    fontSize: 9.5,
                    color: failed ? context.error : context.textTertiary,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _FileMessage extends StatelessWidget {
  final String fileName;
  final int fileSizeKB;
  final bool isUser;

  const _FileMessage({
    required this.fileName,
    required this.fileSizeKB,
    required this.isUser,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.7,
      ),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              Icons.description_outlined,
              color: context.accentPrimary,
              size: 22,
            ),
          ),
          const SizedBox(width: 12),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  fileName,
                  style: AppTypography.bodySmall(context),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  '${(fileSizeKB / 1024).toStringAsFixed(1)} MB',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _ImageMessage extends StatelessWidget {
  final String name;
  final String? url;
  final bool isUser;
  const _ImageMessage({required this.name, this.url, required this.isUser});

  @override
  Widget build(BuildContext context) {
    final hasUrl = url != null && url!.isNotEmpty;
    return GestureDetector(
      onTap: hasUrl ? () => _preview(context) : null,
      child: Container(
        constraints: BoxConstraints(
          maxWidth: MediaQuery.sizeOf(context).width * 0.6,
        ),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brMedium,
          border: isUser
              ? null
              : Border.all(color: context.borderPrimary, width: 0.5),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            SizedBox(
              height: 160,
              width: double.infinity,
              child: hasUrl
                  ? Image.network(
                      url!,
                      fit: BoxFit.cover,
                      cacheWidth: 360,
                      frameBuilder:
                          (context, child, frame, wasSynchronouslyLoaded) {
                            if (wasSynchronouslyLoaded) return child;
                            if (frame != null) return child;
                            return _placeholder(context);
                          },
                      errorBuilder: (_, __, ___) => _placeholder(context),
                    )
                  : _placeholder(context),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
              child: Text(
                name,
                style: AppTypography.label(context),
                overflow: TextOverflow.ellipsis,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _placeholder(BuildContext context) => Container(
    color: context.accentSoft,
    alignment: Alignment.center,
    child: Icon(Icons.image_outlined, size: 40, color: context.accentPrimary),
  );

  void _preview(BuildContext context) {
    final value = url;
    if (value == null || value.isEmpty) return;
    showDialog(
      context: context,
      builder: (ctx) => GestureDetector(
        onTap: () => Navigator.pop(ctx),
        child: Material(
          color: Colors.black87,
          child: SafeArea(
            child: Center(
              child: InteractiveViewer(
                child: Image.network(
                  value,
                  fit: BoxFit.contain,
                  cacheWidth: 1080,
                  errorBuilder: (_, __, ___) => const Icon(
                    Icons.broken_image_outlined,
                    color: Colors.white,
                    size: 64,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _VideoMessage extends StatelessWidget {
  final String title;
  final int durationMs;
  final bool isUser;
  const _VideoMessage({
    required this.title,
    required this.durationMs,
    required this.isUser,
  });

  String get _duration {
    if (durationMs <= 0) return '视频';
    final total = durationMs ~/ 1000;
    final min = total ~/ 60;
    final sec = (total % 60).toString().padLeft(2, '0');
    return '$min:$sec';
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.6,
      ),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 44,
            height: 44,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(Icons.videocam_outlined, color: context.accentPrimary),
          ),
          const SizedBox(width: 10),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: AppTypography.bodySmall(context),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(_duration, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _AudioMessage extends StatelessWidget {
  final String title;
  final int durationMs;
  final bool isUser;
  const _AudioMessage({
    required this.title,
    required this.durationMs,
    required this.isUser,
  });

  String get _duration {
    if (durationMs <= 0) return '语音消息';
    final total = durationMs ~/ 1000;
    final min = total ~/ 60;
    final sec = (total % 60).toString().padLeft(2, '0');
    return '$min:$sec';
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.62,
      ),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: isUser
            ? null
            : Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentSoft,
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.graphic_eq_rounded,
              color: context.accentPrimary,
              size: 20,
            ),
          ),
          const SizedBox(width: 10),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: AppTypography.bodySmall(context),
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(_duration, style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _EmoteMessage extends StatelessWidget {
  final String emoji;
  final String name;
  final bool isUser;
  const _EmoteMessage({
    required this.emoji,
    required this.name,
    required this.isUser,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: isUser
          ? CrossAxisAlignment.end
          : CrossAxisAlignment.start,
      children: [
        Container(
          padding: const EdgeInsets.all(8),
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: AppRadius.brMedium,
          ),
          child: Text(emoji, style: const TextStyle(fontSize: 48)),
        ),
        if (name.isNotEmpty)
          Padding(
            padding: EdgeInsets.only(top: 4),
            child: Text(name, style: AppTypography.label(context)),
          ),
      ],
    );
  }
}

class _CodeMessage extends StatelessWidget {
  final String lang;
  final String body;
  final bool isUser;
  const _CodeMessage({
    required this.lang,
    required this.body,
    required this.isUser,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(
        maxWidth: MediaQuery.sizeOf(context).width * 0.82,
      ),
      decoration: BoxDecoration(
        color: const Color(0xFF282C34),
        borderRadius: AppRadius.brMedium,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            child: Row(
              children: [
                Icon(
                  Icons.code,
                  size: 14,
                  color: Colors.white.withValues(alpha: 0.6),
                ),
                const SizedBox(width: 6),
                Text(
                  lang,
                  style: TextStyle(
                    fontSize: 12,
                    color: Colors.white.withValues(alpha: 0.7),
                  ),
                ),
              ],
            ),
          ),
          Container(
            width: double.infinity,
            padding: EdgeInsets.symmetric(horizontal: 12, vertical: 10),
            child: Text(
              body,
              style: const TextStyle(
                fontFamily: 'monospace',
                fontSize: 13,
                color: Color(0xFFABB2BF),
                height: 1.5,
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _AgentTaskMessage extends StatelessWidget {
  final ChatMessage message;
  final VoidCallback? onAgentTaskTap;
  final VoidCallback? onPauseAgentTask;
  final VoidCallback? onResumeAgentTask;
  final String statusLabel;

  const _AgentTaskMessage({
    required this.message,
    this.onAgentTaskTap,
    this.onPauseAgentTask,
    this.onResumeAgentTask,
    required this.statusLabel,
  });

  BadgeType get _badgeType {
    switch (statusLabel) {
      case '运行中':
        return BadgeType.accent;
      case '已暂停':
        return BadgeType.neutral;
      case '已完成':
        return BadgeType.success;
      case '已失败':
        return BadgeType.error;
      default:
        return BadgeType.accent;
    }
  }

  @override
  Widget build(BuildContext context) {
    final isRunning = statusLabel == '运行中';
    final isPaused = statusLabel == '已暂停';
    return Padding(
      padding: EdgeInsets.only(
        left: AppSpacing.lg,
        right: 60,
        bottom: AppSpacing.sm,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(width: 40),
          Flexible(
            child: Container(
              padding: const EdgeInsets.all(14),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.5),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.auto_awesome,
                        size: 16,
                        color: context.accentPrimary,
                      ),
                      const SizedBox(width: 6),
                      Expanded(
                        child: Text(
                          '正在执行：${message.agentTaskTitle ?? ''}',
                          style: AppTypography.cardTitle(
                            context,
                          ).copyWith(fontSize: 14),
                        ),
                      ),
                      AmitiaStatusBadge(label: statusLabel, type: _badgeType),
                    ],
                  ),
                  const SizedBox(height: 12),
                  ...(message.agentTaskSteps ?? []).map(
                    (step) => Padding(
                      padding: EdgeInsets.only(bottom: 6),
                      child: Row(
                        children: [
                          Container(
                            width: 6,
                            height: 6,
                            decoration: BoxDecoration(
                              color: context.accentPrimary,
                              shape: BoxShape.circle,
                            ),
                          ),
                          const SizedBox(width: 8),
                          Text(step, style: AppTypography.caption(context)),
                        ],
                      ),
                    ),
                  ),
                  const SizedBox(height: 10),
                  AmitiaProgressBar(
                    progress: (message.agentTaskProgress ?? 0) / 100,
                  ),
                  const SizedBox(height: 8),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text(
                        '进度 ${message.agentTaskProgress ?? 0}%',
                        style: AppTypography.label(context),
                      ),
                      Text(
                        '已用时 ${message.agentTaskElapsed ?? '00:00'}',
                        style: AppTypography.label(context),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      GestureDetector(
                        onTap: onAgentTaskTap,
                        child: Text(
                          '查看详情',
                          style: TextStyle(
                            fontSize: 13,
                            color: context.accentPrimary,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                      const Spacer(),
                      if (isRunning)
                        GestureDetector(
                          onTap: onPauseAgentTask,
                          child: Container(
                            padding: EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 6,
                            ),
                            decoration: BoxDecoration(
                              color: context.accentSoft,
                              borderRadius: AppRadius.brTag,
                            ),
                            child: Text(
                              '暂停',
                              style: TextStyle(
                                fontSize: 12,
                                color: context.accentPrimary,
                              ),
                            ),
                          ),
                        )
                      else if (isPaused)
                        GestureDetector(
                          onTap: onResumeAgentTask,
                          child: Container(
                            padding: EdgeInsets.symmetric(
                              horizontal: 12,
                              vertical: 6,
                            ),
                            decoration: BoxDecoration(
                              color: context.accentSoft,
                              borderRadius: AppRadius.brTag,
                            ),
                            child: Text(
                              '继续',
                              style: TextStyle(
                                fontSize: 12,
                                color: context.accentPrimary,
                              ),
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
}

class _ToolCallMessage extends StatelessWidget {
  final ChatMessage message;
  const _ToolCallMessage({required this.message});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(
        left: AppSpacing.lg,
        right: 60,
        bottom: AppSpacing.sm,
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(width: 40),
          Flexible(
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.build_outlined,
                    size: 16,
                    color: context.textTertiary,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: RichText(
                      text: TextSpan(
                        style: AppTypography.caption(context),
                        children: [
                          TextSpan(
                            text: '${message.toolName ?? '工具'}: ',
                            style: TextStyle(
                              color: context.accentPrimary,
                              fontWeight: FontWeight.w500,
                            ),
                          ),
                          TextSpan(text: message.toolResult ?? ''),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}

const List<(String, List<String>)> _emojiGroups = [
  ('常用', ['😀', '😂', '🥰', '😎', '🤔', '😴', '👍', '❤️', '🔥', '🎉']),
  ('Amitia', ['😊', '🤗', '✨', '🌟', '💫', '🌸', '🌈', '☕']),
  ('动物', ['🐶', '🐱', '🐰', '🦊', '🐼', '🐨', '🐯', '🐸']),
];

const List<String> _codeLanguages = [
  'Dart',
  'Python',
  'JavaScript',
  'Go',
  'SQL',
  'JSON',
  'Shell',
];

class AmitiaChatInput extends StatefulWidget {
  final ValueChanged<String> onSend;
  final TextEditingController? controller;
  final String? recipientName;
  final Widget? workspaceSelector;
  final FutureOr<void> Function()? onPickFile;
  final FutureOr<void> Function(bool camera)? onPickImage;
  final FutureOr<void> Function(bool camera)? onPickVideo;
  final void Function(String lang, String code)? onSendCode;
  final Future<List<Map<String, dynamic>>> Function()? onLoadEmotes;
  final void Function(String emoteId, String displayText)? onSendEmote;
  final Future<List<Map<String, dynamic>>> Function()? onLoadAgentSkills;
  final FutureOr<void> Function()? onStartVoiceRecording;
  final FutureOr<void> Function({required bool transcribe})?
  onFinishVoiceRecording;
  final FutureOr<void> Function()? onCancelVoiceRecording;
  final String? replyPreview;
  final VoidCallback? onCancelReply;
  final List<Map<String, dynamic>> models;
  final int selectedModelId;
  final String reasoningEffort;
  final bool supportsReasoning;
  final bool reasoningEnabled;
  final String permissionMode;
  final ValueChanged<String>? onPermissionChanged;
  final void Function(
    int modelId,
    String reasoningEffort,
    bool reasoningEnabled,
  )?
  onModelPreviewChanged;
  final void Function(
    int modelId,
    String reasoningEffort,
    bool reasoningEnabled,
  )?
  onModelChanged;

  const AmitiaChatInput({
    super.key,
    required this.onSend,
    this.controller,
    this.recipientName,
    this.workspaceSelector,
    this.onPickFile,
    this.onPickImage,
    this.onPickVideo,
    this.onSendCode,
    this.onLoadEmotes,
    this.onSendEmote,
    this.onLoadAgentSkills,
    this.onStartVoiceRecording,
    this.onFinishVoiceRecording,
    this.onCancelVoiceRecording,
    this.replyPreview,
    this.onCancelReply,
    this.models = const <Map<String, dynamic>>[],
    this.selectedModelId = 0,
    this.reasoningEffort = 'high',
    this.supportsReasoning = false,
    this.reasoningEnabled = true,
    this.permissionMode = 'request_approval',
    this.onPermissionChanged,
    this.onModelPreviewChanged,
    this.onModelChanged,
  });

  @override
  State<AmitiaChatInput> createState() => _AmitiaChatInputState();
}

class _AmitiaChatInputState extends State<AmitiaChatInput>
    with SingleTickerProviderStateMixin {
  static const double _composerInputHeight = 58;
  static const double _composerInputVerticalInset = 7;
  static const double _composerTextSize = 15;
  late TextEditingController _controller;
  late bool _ownsController;
  final _inputFocusNode = FocusNode();
  bool _hasText = false;
  bool _voiceMode = false;
  bool _voiceRecording = false;
  Offset? _voiceStart;
  _VoiceGestureIntent _voiceIntent = _VoiceGestureIntent.send;
  final List<String> _selectedSkillNames = <String>[];
  bool _modelMenuOpen = false;
  bool _modelMenuTriggerHovered = false;
  _ComposerModelMenuPage _modelMenuPage = _ComposerModelMenuPage.effort;
  StateSetter? _modelMenuStateSetter;
  double _draftReasoningValue = 1;
  bool _reasoningDragging = false;

  @override
  void initState() {
    super.initState();
    _ownsController = widget.controller == null;
    _controller = widget.controller ?? TextEditingController();
    _hasText = _controller.text.trim().isNotEmpty;
    _draftReasoningValue = _reasoningIndexFor(
      widget.reasoningEffort,
    ).toDouble();
    _controller.addListener(_syncControllerText);
  }

  @override
  void didUpdateWidget(covariant AmitiaChatInput oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!_reasoningDragging &&
        oldWidget.reasoningEffort != widget.reasoningEffort) {
      _draftReasoningValue = _reasoningIndexFor(
        widget.reasoningEffort,
      ).toDouble();
    }
    if (oldWidget.controller == widget.controller) return;
    _controller.removeListener(_syncControllerText);
    if (_ownsController) _controller.dispose();
    _ownsController = widget.controller == null;
    _controller = widget.controller ?? TextEditingController();
    _hasText = _controller.text.trim().isNotEmpty;
    _controller.addListener(_syncControllerText);
  }

  void _syncControllerText() {
    final hasText = _controller.text.trim().isNotEmpty;
    if (hasText == _hasText || !mounted) return;
    setState(() => _hasText = hasText);
  }

  void _toggleVoiceMode() {
    _inputFocusNode.unfocus();
    setState(() {
      _voiceMode = !_voiceMode;
      _voiceIntent = _VoiceGestureIntent.send;
    });
  }

  Future<void> _startVoiceGesture(LongPressStartDetails details) async {
    final start = widget.onStartVoiceRecording;
    if (start == null) return;
    setState(() {
      _voiceStart = details.localPosition;
      _voiceIntent = _VoiceGestureIntent.send;
      _voiceRecording = true;
    });
    try {
      await start();
    } catch (error) {
      if (mounted) {
        setState(() => _voiceRecording = false);
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('录音失败：$error')));
      }
    }
  }

  void _updateVoiceGesture(LongPressMoveUpdateDetails details) {
    final start = _voiceStart;
    if (!_voiceRecording || start == null) return;
    final dx = details.localPosition.dx - start.dx;
    final dy = details.localPosition.dy - start.dy;
    final next = dx < -52 && dy < -52
        ? _VoiceGestureIntent.cancel
        : dx > 52 && dy < -52
        ? _VoiceGestureIntent.transcribe
        : _VoiceGestureIntent.send;
    if (next == _voiceIntent) return;
    setState(() => _voiceIntent = next);
  }

  Future<void> _endVoiceGesture(LongPressEndDetails details) async {
    if (!_voiceRecording) return;
    final intent = _voiceIntent;
    setState(() {
      _voiceRecording = false;
      _voiceStart = null;
      _voiceIntent = _VoiceGestureIntent.send;
      _voiceMode = false;
    });
    if (intent == _VoiceGestureIntent.cancel) {
      await widget.onCancelVoiceRecording?.call();
      return;
    }
    await widget.onFinishVoiceRecording?.call(
      transcribe: intent == _VoiceGestureIntent.transcribe,
    );
  }

  Future<void> _cancelVoiceGesture() async {
    if (!_voiceRecording) return;
    setState(() {
      _voiceRecording = false;
      _voiceStart = null;
      _voiceIntent = _VoiceGestureIntent.send;
      _voiceMode = false;
    });
    await widget.onCancelVoiceRecording?.call();
  }

  @override
  void dispose() {
    _controller.removeListener(_syncControllerText);
    _inputFocusNode.dispose();
    if (_ownsController) _controller.dispose();
    super.dispose();
  }

  void _send() {
    final text = _controller.text.trim();
    if (text.isEmpty && _selectedSkillNames.isEmpty) return;
    final prefix = _selectedSkillNames.map((name) => '\$$name').join(' ');
    final outgoing = [
      prefix,
      text,
    ].where((part) => part.trim().isNotEmpty).join(' ');
    widget.onSend(outgoing);
    _controller.clear();
    setState(() {
      _hasText = false;
      _selectedSkillNames.clear();
    });
  }

  void _showFileSheet() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('添加文件', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.folder_open_outlined,
                  context.accentPrimary,
                ),
                title: const Text('从本机选择'),
                subtitle: const Text('上传后作为真实 Artifact 发送'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickFile?.call();
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showImagePicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('添加图片', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.photo_library_outlined,
                  context.accentPrimary,
                ),
                title: const Text('从相册选择'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickImage?.call(false);
                },
              ),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.photo_camera_outlined,
                  context.accentPrimary,
                ),
                title: const Text('拍照'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickImage?.call(true);
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showVideoPicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 8, 20, 24),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('添加视频', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 12),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.video_library_outlined,
                  context.accentPrimary,
                ),
                title: const Text('从相册选择'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickVideo?.call(false);
                },
              ),
              ListTile(
                leading: _sheetIcon(
                  context,
                  Icons.videocam_outlined,
                  context.accentPrimary,
                ),
                title: const Text('拍摄视频'),
                onTap: () {
                  Navigator.pop(sheetCtx);
                  widget.onPickVideo?.call(true);
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showCodeDialog() {
    String lang = _codeLanguages.first;
    final codeCtrl = TextEditingController();
    showDialog(
      context: context,
      builder: (ctx) {
        return StatefulBuilder(
          builder: (ctx, setSheetState) {
            return AlertDialog(
              backgroundColor: context.surfacePrimary,
              shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
              title: Text('插入代码', style: AppTypography.cardTitle(ctx)),
              content: SizedBox(
                width: MediaQuery.sizeOf(context).width * 0.9,
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '语言',
                      style: AppTypography.label(
                        ctx,
                      ).copyWith(fontWeight: FontWeight.w600),
                    ),
                    const SizedBox(height: 6),
                    DropdownButtonFormField<String>(
                      value: lang,
                      decoration: InputDecoration(
                        isDense: true,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brSmall,
                        ),
                      ),
                      items: _codeLanguages
                          .map(
                            (l) => DropdownMenuItem(
                              value: l,
                              child: Text(
                                l,
                                style: AppTypography.bodySmall(ctx),
                              ),
                            ),
                          )
                          .toList(),
                      onChanged: (v) =>
                          setSheetState(() => lang = v ?? _codeLanguages.first),
                    ),
                    const SizedBox(height: 14),
                    Text(
                      '代码',
                      style: AppTypography.label(
                        ctx,
                      ).copyWith(fontWeight: FontWeight.w600),
                    ),
                    const SizedBox(height: 6),
                    TextField(
                      controller: codeCtrl,
                      maxLines: 6,
                      style: const TextStyle(
                        fontFamily: 'monospace',
                        fontSize: 13,
                      ),
                      decoration: InputDecoration(
                        hintText: '输入代码……',
                        hintStyle: TextStyle(color: context.textTertiary),
                        isDense: true,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brSmall,
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(ctx),
                  child: Text(
                    '取消',
                    style: TextStyle(color: context.textSecondary),
                  ),
                ),
                AmitiaButton(
                  label: '插入',
                  height: 40,
                  onPressed: () {
                    final code = codeCtrl.text.trim();
                    if (code.isEmpty) return;
                    Navigator.pop(ctx);
                    widget.onSendCode?.call(lang, code);
                  },
                ),
              ],
            );
          },
        );
      },
    );
  }

  Future<void> _showAgentSkillPicker() async {
    final loader = widget.onLoadAgentSkills;
    if (loader == null) return;
    List<Map<String, dynamic>> skills;
    try {
      skills = await loader();
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('加载 Agent Skill 失败：$error')));
      return;
    }
    if (!mounted) return;
    final usable = skills.where((skill) {
      final enabled =
          skill['enabled'] == true ||
          skill['isEnabled'] == true ||
          skill['enabled'] == 1;
      final status =
          (skill['compatibilityStatus'] ?? skill['compatibility'] ?? '')
              .toString()
              .toLowerCase();
      return enabled && status != 'blocked' && status != 'incompatible';
    }).toList();
    await showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.surfacePrimary,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: SizedBox(
            height: MediaQuery.sizeOf(sheetContext).height * 0.62,
            child: Column(
              children: [
                Padding(
                  padding: const EdgeInsets.fromLTRB(20, 18, 12, 10),
                  child: Row(
                    children: [
                      Expanded(
                        child: Text(
                          '本次消息使用 Agent Skill',
                          style: AppTypography.pageTitle(sheetContext),
                        ),
                      ),
                      IconButton(
                        onPressed: () => Navigator.pop(sheetContext),
                        icon: const Icon(Icons.close),
                      ),
                    ],
                  ),
                ),
                Divider(height: 1, color: sheetContext.borderSecondary),
                Expanded(
                  child: usable.isEmpty
                      ? Center(
                          child: Text(
                            '暂无已启用且兼容的 Agent Skill',
                            style: AppTypography.caption(sheetContext),
                          ),
                        )
                      : ListView.builder(
                          itemCount: usable.length,
                          itemBuilder: (_, index) {
                            final skill = usable[index];
                            final name = (skill['name'] ?? '').toString();
                            final displayName = (skill['displayName'] ?? name)
                                .toString();
                            final selected = _selectedSkillNames.contains(name);
                            return CheckboxListTile(
                              value: selected,
                              title: Text(displayName),
                              subtitle: Text(
                                (skill['shortDescription'] ??
                                        skill['description'] ??
                                        '')
                                    .toString(),
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                              ),
                              onChanged: name.isEmpty
                                  ? null
                                  : (value) {
                                      setState(() {
                                        if (value == true) {
                                          if (!_selectedSkillNames.contains(
                                            name,
                                          ))
                                            _selectedSkillNames.add(name);
                                        } else {
                                          _selectedSkillNames.remove(name);
                                        }
                                      });
                                      setSheetState(() {});
                                    },
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  void _showEmotePicker() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) {
        return _EmotePicker(
          loadEmotes: widget.onLoadEmotes,
          onSend: (emoteId, displayText) {
            Navigator.pop(sheetCtx);
            widget.onSendEmote?.call(emoteId, displayText);
          },
        );
      },
    );
  }

  Widget _sheetIcon(BuildContext context, IconData icon, Color color) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: color),
    );
  }

  void _showComposerTools() {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(28)),
      ),
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(16, 8, 16, 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: 12),
              _ComposerTool(
                icon: Icons.photo_library_outlined,
                label: '添加图片',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showImagePicker();
                },
              ),
              _ComposerTool(
                icon: Icons.video_library_outlined,
                label: '添加视频',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showVideoPicker();
                },
              ),
              _ComposerTool(
                icon: Icons.attach_file_outlined,
                label: '添加文件',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showFileSheet();
                },
              ),
              _ComposerTool(
                icon: Icons.code,
                label: '插入代码',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showCodeDialog();
                },
              ),
              if (widget.onLoadAgentSkills != null)
                _ComposerTool(
                  icon: Icons.auto_awesome_outlined,
                  label: '使用 Agent Skill',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showAgentSkillPicker();
                  },
                ),
              _ComposerTool(
                icon: Icons.emoji_emotions_outlined,
                label: '选择表情',
                onTap: () {
                  Navigator.pop(sheetContext);
                  _showEmotePicker();
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showPermissionPicker() {
    final fullAccess = widget.permissionMode == 'full_access';
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(24)),
      ),
      builder: (sheetContext) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 20),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              const SizedBox(height: 14),
              Align(
                alignment: Alignment.centerLeft,
                child: Text('工具权限', style: AppTypography.sectionTitle(context)),
              ),
              const SizedBox(height: 4),
              Align(
                alignment: Alignment.centerLeft,
                child: Text(
                  '权限模式仅影响从下一条消息开始的工具执行',
                  style: AppTypography.caption(context),
                ),
              ),
              const SizedBox(height: 10),
              ListTile(
                shape: RoundedRectangleBorder(borderRadius: AppRadius.brSmall),
                leading: Icon(
                  Icons.lock_outline_rounded,
                  color: context.textPrimary,
                ),
                title: const Text('请求批准'),
                subtitle: const Text('敏感工具执行前需要你批准'),
                trailing: fullAccess
                    ? null
                    : Icon(Icons.check_rounded, color: context.accentPrimary),
                onTap: () {
                  widget.onPermissionChanged?.call('request_approval');
                  Navigator.pop(sheetContext);
                },
              ),
              ListTile(
                shape: RoundedRectangleBorder(borderRadius: AppRadius.brSmall),
                leading: Icon(
                  Icons.lock_open_rounded,
                  color: fullAccess ? context.warning : context.textPrimary,
                ),
                title: const Text('完全访问'),
                subtitle: const Text('自动放行当前会话中可批准的工具操作'),
                trailing: fullAccess
                    ? Icon(Icons.check_rounded, color: context.accentPrimary)
                    : null,
                onTap: () {
                  widget.onPermissionChanged?.call('full_access');
                  Navigator.pop(sheetContext);
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final recipient = (widget.recipientName ?? '').trim();
    return SafeArea(
      top: false,
      child: Stack(
        clipBehavior: Clip.none,
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(10, 8, 10, 4),
            child: Container(
              key: const ValueKey('chat-composer-surface'),
              constraints: const BoxConstraints(minHeight: 98, maxHeight: 170),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: BorderRadius.circular(23),
                border: Border.all(color: context.borderPrimary, width: 0.8),
                boxShadow: [
                  BoxShadow(
                    color: Colors.black.withValues(
                      alpha: Theme.of(context).brightness == Brightness.dark
                          ? 0.16
                          : 0.045,
                    ),
                    blurRadius: 18,
                    offset: const Offset(0, 6),
                  ),
                ],
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  if ((widget.replyPreview ?? '').trim().isNotEmpty)
                    Container(
                      margin: const EdgeInsets.fromLTRB(12, 10, 12, 2),
                      padding: const EdgeInsets.symmetric(
                        horizontal: 10,
                        vertical: 7,
                      ),
                      decoration: BoxDecoration(
                        color: context.surfaceSecondary,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Row(
                        children: [
                          Icon(
                            Icons.format_quote_rounded,
                            size: 16,
                            color: context.accentPrimary,
                          ),
                          const SizedBox(width: 7),
                          Expanded(
                            child: Text(
                              widget.replyPreview!,
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.caption(context),
                            ),
                          ),
                          GestureDetector(
                            onTap: widget.onCancelReply,
                            child: Icon(
                              Icons.close,
                              size: 17,
                              color: context.textTertiary,
                            ),
                          ),
                        ],
                      ),
                    ),
                  if (_selectedSkillNames.isNotEmpty)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                      child: Wrap(
                        spacing: 6,
                        runSpacing: 6,
                        children: _selectedSkillNames
                            .map(
                              (name) => InputChip(
                                visualDensity: VisualDensity.compact,
                                avatar: const Icon(
                                  Icons.auto_awesome_outlined,
                                  size: 14,
                                ),
                                label: Text('\$$name'),
                                onDeleted: () => setState(
                                  () => _selectedSkillNames.remove(name),
                                ),
                              ),
                            )
                            .toList(),
                      ),
                    ),
                  if (_voiceMode)
                    SizedBox(
                      height: _composerInputHeight,
                      child: _buildHoldToTalkButton(context),
                    )
                  else
                    SizedBox(
                      height: _composerInputHeight,
                      child: Align(
                        alignment: Alignment.center,
                        child: SizedBox(
                          width: double.infinity,
                          child: TapRegion(
                            onTapOutside: (_) => _inputFocusNode.unfocus(),
                            child: TextField(
                              controller: _controller,
                              focusNode: _inputFocusNode,
                              minLines: 1,
                              maxLines: 2,
                              textAlignVertical: TextAlignVertical.center,
                              textCapitalization: TextCapitalization.sentences,
                              onSubmitted: (_) => _send(),
                              style: AppTypography.bodySmall(
                                context,
                              ).copyWith(fontSize: _composerTextSize),
                              decoration: InputDecoration(
                                hintText: recipient.isEmpty
                                    ? '发消息…'
                                    : '给 $recipient 发消息…',
                                hintStyle: AppTypography.bodySmall(context)
                                    .copyWith(
                                      color: context.textTertiary,
                                      fontSize: _composerTextSize,
                                    ),
                                isDense: true,
                                contentPadding: const EdgeInsets.fromLTRB(
                                  16,
                                  10,
                                  16,
                                  10,
                                ),
                                border: InputBorder.none,
                                enabledBorder: InputBorder.none,
                                focusedBorder: InputBorder.none,
                                disabledBorder: InputBorder.none,
                                errorBorder: InputBorder.none,
                                focusedErrorBorder: InputBorder.none,
                                fillColor: Colors.transparent,
                                focusColor: Colors.transparent,
                                hoverColor: Colors.transparent,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  Padding(
                    padding: const EdgeInsets.fromLTRB(10, 0, 10, 7),
                    child: SizedBox(
                      height: 38,
                      child: Row(
                        children: [
                          _ComposerRoundButton(
                            key: const ValueKey('composer-add-button'),
                            icon: Icons.add_rounded,
                            tooltip: '添加内容',
                            onTap: _showComposerTools,
                          ),
                          const SizedBox(width: 4),
                          Semantics(
                            button: true,
                            label: widget.permissionMode == 'full_access'
                                ? '完全访问'
                                : '请求批准',
                            child: _ComposerRoundButton(
                              key: const ValueKey('composer-permission-button'),
                              icon: widget.permissionMode == 'full_access'
                                  ? Icons.lock_open_rounded
                                  : Icons.lock_outline_rounded,
                              iconSize: 16,
                              tooltip: widget.permissionMode == 'full_access'
                                  ? '完全访问'
                                  : '请求批准',
                              onTap: _showPermissionPicker,
                            ),
                          ),
                          if (widget.workspaceSelector != null) ...[
                            const SizedBox(width: 4),
                            Expanded(
                              child: Align(
                                key: const ValueKey(
                                  'composer-workspace-selector',
                                ),
                                alignment: Alignment.centerLeft,
                                child: widget.workspaceSelector!,
                              ),
                            ),
                          ] else
                            const Spacer(),
                          Builder(
                            builder: (anchorContext) => MouseRegion(
                              onEnter: (_) => setState(
                                () => _modelMenuTriggerHovered = true,
                              ),
                              onExit: (_) => setState(
                                () => _modelMenuTriggerHovered = false,
                              ),
                              child: GestureDetector(
                                onTap: () => _showModelMenu(anchorContext),
                                child: AnimatedContainer(
                                  key: const ValueKey('composer-model-trigger'),
                                  duration: const Duration(milliseconds: 160),
                                  curve: Curves.easeOut,
                                  constraints: const BoxConstraints(
                                    minWidth: 44,
                                    maxWidth: 64,
                                  ),
                                  height: 31,
                                  padding: const EdgeInsets.symmetric(
                                    horizontal: 9,
                                  ),
                                  decoration: BoxDecoration(
                                    border: Border.all(
                                      color: _modelMenuTriggerHovered
                                          ? context.borderPrimary
                                          : Colors.transparent,
                                    ),
                                    borderRadius: BorderRadius.circular(8),
                                  ),
                                  alignment: Alignment.center,
                                  child: Text(
                                    _reasoningLabel(),
                                    key: const ValueKey(
                                      'composer-reasoning-label',
                                    ),
                                    maxLines: 1,
                                    overflow: TextOverflow.ellipsis,
                                    style: AppTypography.label(context)
                                        .copyWith(
                                          fontSize: 14,
                                          color: context.textSecondary,
                                        ),
                                  ),
                                ),
                              ),
                            ),
                          ),
                          const SizedBox(width: 4),
                          if (!_voiceMode &&
                              (_hasText || _selectedSkillNames.isNotEmpty))
                            GestureDetector(
                              behavior: HitTestBehavior.opaque,
                              onTap: _send,
                              child: Tooltip(
                                message: '发送消息',
                                child: Container(
                                  key: const ValueKey('composer-send-button'),
                                  width: 31,
                                  height: 31,
                                  decoration: BoxDecoration(
                                    color: context.accentPrimary,
                                    shape: BoxShape.circle,
                                  ),
                                  alignment: Alignment.center,
                                  child: Icon(
                                    Icons.arrow_upward_rounded,
                                    size: 18,
                                    color: context.surfacePrimary,
                                  ),
                                ),
                              ),
                            )
                          else
                            _ComposerRoundButton(
                              key: const ValueKey('composer-trailing-button'),
                              icon: _voiceMode
                                  ? Icons.keyboard_outlined
                                  : Icons.mic_none_outlined,
                              tooltip: _voiceMode ? '切换到键盘输入' : '按住说话',
                              onTap: _toggleVoiceMode,
                            ),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Future<void> _showModelMenu(BuildContext anchorContext) async {
    if (_modelMenuOpen) return;
    final anchorBox = anchorContext.findRenderObject();
    if (anchorBox is! RenderBox || !anchorBox.hasSize) return;
    final topLeft = anchorBox.localToGlobal(Offset.zero);
    final bottomRight = anchorBox.localToGlobal(
      anchorBox.size.bottomRight(Offset.zero),
    );
    setState(() {
      _modelMenuOpen = true;
      _modelMenuPage = _ComposerModelMenuPage.effort;
      _draftReasoningValue = _reasoningIndexFor(
        widget.reasoningEffort,
      ).toDouble();
    });
    await showAmitiaPopupSurface<void>(
      context: context,
      anchorRect: Rect.fromPoints(topLeft, bottomRight),
      menuWidth: 260,
      estimatedHeight: 380,
      builder: (popupContext) => StatefulBuilder(
        builder: (context, setPopupState) {
          _modelMenuStateSetter = setPopupState;
          return _buildModelMenuSurface(context);
        },
      ),
    );
    _modelMenuStateSetter = null;
    if (!mounted) return;
    setState(() {
      _modelMenuOpen = false;
      _modelMenuPage = _ComposerModelMenuPage.effort;
    });
  }

  List<Map<String, dynamic>> get _llmModels => widget.models
      .where((model) {
        final type = (model['apiType'] ?? model['provider'] ?? '')
            .toString()
            .toLowerCase();
        return !const <String>{
          'voice',
          'asr',
          'embedding',
          'vector',
          'vision',
          'imagegen',
        }.contains(type);
      })
      .toList(growable: false);

  String _selectedModelLabel() {
    final selected = _llmModels
        .where((model) => _intValue(model['id']) == widget.selectedModelId)
        .firstOrNull;
    if (selected == null) return '选择模型';
    return _modelDisplayName(selected);
  }

  String _modelDisplayName(Map<String, dynamic> model) {
    return (model['modelName'] ?? model['model'] ?? model['name'] ?? '')
        .toString();
  }

  String _reasoningLabel() {
    return switch (widget.reasoningEffort) {
      'low' => '低',
      'medium' => '中',
      'high' => '高',
      'xhigh' => '极高',
      _ => '高',
    };
  }

  int _reasoningIndexFor(String effort) {
    return switch (effort) {
      'low' => 0,
      'medium' => 1,
      'high' => 2,
      'xhigh' => 3,
      _ => 2,
    };
  }

  String _reasoningLabelForValue(double value) {
    const labels = <String>['低', '中', '高', '极高'];
    return labels[value.round().clamp(0, labels.length - 1)];
  }

  int _intValue(dynamic value) {
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? 0;
  }

  Widget _buildModelMenuSurface(BuildContext context) {
    return AmitiaPopupSurface(
      width: 260,
      maxHeight: MediaQuery.sizeOf(context).height * 0.72,
      padding: const EdgeInsets.all(12),
      child: AnimatedSwitcher(
        duration: const Duration(milliseconds: 220),
        reverseDuration: const Duration(milliseconds: 150),
        switchInCurve: Curves.easeOutCubic,
        switchOutCurve: Curves.easeInCubic,
        transitionBuilder: (child, animation) =>
            FadeTransition(opacity: animation, child: child),
        child: KeyedSubtree(
          key: ValueKey(_modelMenuPage),
          child: switch (_modelMenuPage) {
            _ComposerModelMenuPage.models => _buildModelList(context),
            _ComposerModelMenuPage.reasoning => _buildReasoningModeList(
              context,
            ),
            _ComposerModelMenuPage.effort => _buildModelEffortPanel(context),
          },
        ),
      ),
    );
  }

  Widget _buildModelMenuRow({
    required BuildContext context,
    required String label,
    required Widget trailing,
    VoidCallback? onTap,
  }) {
    final row = Container(
      constraints: const BoxConstraints(minHeight: 34),
      padding: const EdgeInsets.symmetric(horizontal: 8),
      child: Row(
        children: [
          Text(
            label,
            style: TextStyle(fontSize: 12, color: context.textTertiary),
          ),
          const Spacer(),
          Flexible(child: trailing),
        ],
      ),
    );
    if (onTap == null) return row;
    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(7),
        child: row,
      ),
    );
  }

  Widget _buildModelMenuValue({
    required BuildContext context,
    required String label,
    bool showArrow = true,
  }) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        Flexible(
          child: Text(
            label,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
            textAlign: TextAlign.right,
            style: TextStyle(fontSize: 12, color: context.textPrimary),
          ),
        ),
        if (showArrow) ...[
          const SizedBox(width: 5),
          Icon(
            Icons.chevron_right_rounded,
            size: 18,
            color: context.textTertiary,
          ),
        ],
      ],
    );
  }

  Widget _buildModelEffortPanel(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildModelMenuRow(
          context: context,
          label: '强度',
          trailing: Text(
            _reasoningLabelForValue(_draftReasoningValue),
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w600,
              color: context.textPrimary,
            ),
          ),
        ),
        _buildModelMenuRow(
          context: context,
          label: '思考',
          onTap: () => _openModelMenuPage(_ComposerModelMenuPage.reasoning),
          trailing: _buildModelMenuValue(
            context: context,
            label: widget.reasoningEnabled ? '支持' : '不支持',
          ),
        ),
        _buildModelMenuRow(
          context: context,
          label: '模型',
          onTap: () => _openModelMenuPage(_ComposerModelMenuPage.models),
          trailing: _buildModelMenuValue(
            context: context,
            label: _selectedModelLabel(),
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(8, 7, 8, 4),
          child: Column(
            children: [
              _buildReasoningSlider(context),
              Transform.translate(
                offset: const Offset(0, -3),
                child: Row(
                  mainAxisAlignment: MainAxisAlignment.spaceBetween,
                  children: const [
                    Text('低', style: TextStyle(fontSize: 10)),
                    Text('中', style: TextStyle(fontSize: 10)),
                    Text('高', style: TextStyle(fontSize: 10)),
                    Text('极高', style: TextStyle(fontSize: 10)),
                  ],
                ),
              ),
              if (!widget.reasoningEnabled)
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Align(
                    alignment: Alignment.centerLeft,
                    child: Text(
                      '该模型不支持思考强度',
                      style: TextStyle(fontSize: 11, color: context.error),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildReasoningSlider(BuildContext context) {
    return _ComposerReasoningSlider(
      value: _draftReasoningValue,
      enabled: widget.reasoningEnabled,
      onChanged: _previewReasoningIndex,
      onChangeEnd: _applyReasoningIndex,
    );
  }

  Widget _buildModelList(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildModelMenuHeader(
          context: context,
          title: '选择模型',
          onBack: _backToModelEffort,
        ),
        for (final model in _llmModels)
          _buildModelSelectionItem(
            context: context,
            selected: _intValue(model['id']) == widget.selectedModelId,
            title: Text(
              (model['name'] ?? model['modelName'] ?? '').toString(),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 14, color: context.textPrimary),
            ),
            subtitle: Text(
              '${_modelDisplayName(model)} · ${_modelProviderLabel(model)}',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(fontSize: 10, color: context.textTertiary),
            ),
            onTap: () {
              final effort = (model['defaultReasoningEffort'] ?? 'high')
                  .toString();
              widget.onModelChanged?.call(
                _intValue(model['id']),
                effort,
                model['supportsReasoning'] == true,
              );
              setState(() {
                _draftReasoningValue = _reasoningIndexFor(effort).toDouble();
              });
              _backToModelEffort();
            },
          ),
      ],
    );
  }

  Widget _buildReasoningModeList(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildModelMenuHeader(
          context: context,
          title: '选择思考模式',
          onBack: _backToModelEffort,
        ),
        _buildModelSelectionItem(
          context: context,
          selected: widget.reasoningEnabled,
          title: Text(
            '支持',
            style: TextStyle(fontSize: 14, color: context.textPrimary),
          ),
          subtitle: Text(
            '允许模型按所选强度进行思考',
            style: TextStyle(fontSize: 10, color: context.textTertiary),
          ),
          onTap: () => _selectReasoningEnabled(true),
        ),
        _buildModelSelectionItem(
          context: context,
          selected: !widget.reasoningEnabled,
          title: Text(
            '不支持',
            style: TextStyle(fontSize: 14, color: context.textPrimary),
          ),
          subtitle: Text(
            '关闭模型的思考过程',
            style: TextStyle(fontSize: 10, color: context.textTertiary),
          ),
          onTap: () => _selectReasoningEnabled(false),
        ),
      ],
    );
  }

  Widget _buildModelMenuHeader({
    required BuildContext context,
    required String title,
    required VoidCallback onBack,
  }) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 2, 4, 8),
      child: Row(
        children: [
          SizedBox(
            width: 26,
            height: 26,
            child: IconButton(
              padding: EdgeInsets.zero,
              iconSize: 18,
              onPressed: onBack,
              icon: Icon(Icons.arrow_back_rounded, color: context.textTertiary),
            ),
          ),
          const SizedBox(width: 8),
          Text(
            title,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: context.textPrimary,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildModelSelectionItem({
    required BuildContext context,
    required bool selected,
    required Widget title,
    required Widget subtitle,
    required VoidCallback onTap,
  }) {
    return Material(
      color: selected ? context.accentSoft : Colors.transparent,
      borderRadius: BorderRadius.circular(8),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Container(
          constraints: const BoxConstraints(minHeight: 48),
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 6),
          child: Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [title, const SizedBox(height: 2), subtitle],
                ),
              ),
              if (selected) ...[
                const SizedBox(width: 10),
                Icon(
                  Icons.check_rounded,
                  size: 18,
                  color: context.accentPrimary,
                ),
              ],
            ],
          ),
        ),
      ),
    );
  }

  String _modelProviderLabel(Map<String, dynamic> model) {
    return (model['apiType'] ?? model['provider'] ?? '').toString();
  }

  void _selectReasoningEnabled(bool value) {
    widget.onModelChanged?.call(
      widget.selectedModelId,
      widget.reasoningEffort,
      value,
    );
    _backToModelEffort();
  }

  void _openModelMenuPage(_ComposerModelMenuPage page) {
    if (page == _modelMenuPage) return;
    setState(() => _modelMenuPage = page);
    _modelMenuStateSetter?.call(() {});
  }

  void _backToModelEffort() {
    _openModelMenuPage(_ComposerModelMenuPage.effort);
  }

  void _applyReasoningIndex(double value) {
    _reasoningDragging = false;
    const efforts = <String>['low', 'medium', 'high', 'xhigh'];
    final clamped = value.round().clamp(0, efforts.length - 1);
    setState(() => _draftReasoningValue = clamped.toDouble());
    _modelMenuStateSetter?.call(() {});
    widget.onModelChanged?.call(
      widget.selectedModelId,
      efforts[clamped],
      widget.reasoningEnabled,
    );
  }

  void _previewReasoningIndex(double value) {
    const efforts = <String>['low', 'medium', 'high', 'xhigh'];
    final clamped = value.clamp(0, 3).toDouble();
    setState(() {
      _draftReasoningValue = clamped;
      _reasoningDragging = true;
    });
    _modelMenuStateSetter?.call(() {});
    widget.onModelPreviewChanged?.call(
      widget.selectedModelId,
      efforts[clamped.round()],
      widget.reasoningEnabled,
    );
  }

  Widget _buildHoldToTalkButton(BuildContext context) {
    final cancelling = _voiceIntent == _VoiceGestureIntent.cancel;
    final transcribing = _voiceIntent == _VoiceGestureIntent.transcribe;
    final label = cancelling
        ? '松开取消'
        : transcribing
        ? '松开转文字'
        : '按住说话';
    final color = cancelling
        ? context.error
        : transcribing
        ? context.accentPrimary
        : context.textPrimary;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: 12,
        vertical: _composerInputVerticalInset,
      ),
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onLongPressStart: (details) => unawaited(_startVoiceGesture(details)),
        onLongPressMoveUpdate: _updateVoiceGesture,
        onLongPressEnd: (details) => unawaited(_endVoiceGesture(details)),
        onLongPressCancel: () => unawaited(_cancelVoiceGesture()),
        child: Container(
          key: const ValueKey('hold-to-talk-button'),
          height: double.infinity,
          alignment: Alignment.center,
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: BorderRadius.circular(14),
          ),
          child: Row(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                cancelling
                    ? Icons.close_rounded
                    : transcribing
                    ? Icons.transcribe_outlined
                    : Icons.mic_none_outlined,
                size: 18,
                color: color,
              ),
              const SizedBox(width: 8),
              Text(
                label,
                style: TextStyle(
                  color: color,
                  fontSize: _composerTextSize,
                  fontWeight: FontWeight.w600,
                  height: 1.5,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ComposerReasoningSlider extends StatefulWidget {
  const _ComposerReasoningSlider({
    required this.value,
    required this.enabled,
    required this.onChanged,
    required this.onChangeEnd,
  });

  final double value;
  final bool enabled;
  final ValueChanged<double> onChanged;
  final ValueChanged<double> onChangeEnd;

  @override
  State<_ComposerReasoningSlider> createState() =>
      _ComposerReasoningSliderState();
}

class _ComposerReasoningSliderState extends State<_ComposerReasoningSlider> {
  int? _dragIndex;

  int _indexAt(double x, double width) {
    final span = math.max(1.0, width - 64);
    final ratio = ((x - 32) / span).clamp(0.0, 1.0);
    return (ratio * 3).round();
  }

  void _previewAt(double x, double width) {
    if (!widget.enabled) return;
    final index = _indexAt(x, width);
    _dragIndex = index;
    widget.onChanged(index.toDouble());
  }

  @override
  Widget build(BuildContext context) {
    final index = widget.value.round().clamp(0, 3);
    final labels = const <String>['低', '中', '高', '极高'];
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        final span = math.max(0.0, width - 64);
        final trackWidth = math.max(0.0, width - 32);
        final opacity = widget.enabled ? 1.0 : 0.45;
        return Semantics(
          slider: true,
          enabled: widget.enabled,
          value: labels[index],
          increasedValue: labels[math.min(index + 1, 3)],
          decreasedValue: labels[math.max(index - 1, 0)],
          onIncrease: widget.enabled && index < 3
              ? () => widget.onChangeEnd((index + 1).toDouble())
              : null,
          onDecrease: widget.enabled && index > 0
              ? () => widget.onChangeEnd((index - 1).toDouble())
              : null,
          child: GestureDetector(
            key: const ValueKey('composer-reasoning-slider'),
            behavior: HitTestBehavior.opaque,
            onTapDown: widget.enabled
                ? (details) {
                    final next = _indexAt(details.localPosition.dx, width);
                    widget.onChangeEnd(next.toDouble());
                  }
                : null,
            onHorizontalDragStart: widget.enabled
                ? (details) => _previewAt(details.localPosition.dx, width)
                : null,
            onHorizontalDragUpdate: widget.enabled
                ? (details) => _previewAt(details.localPosition.dx, width)
                : null,
            onHorizontalDragEnd: widget.enabled
                ? (_) {
                    final next = _dragIndex ?? index;
                    _dragIndex = null;
                    widget.onChangeEnd(next.toDouble());
                  }
                : null,
            child: Opacity(
              opacity: opacity,
              child: TweenAnimationBuilder<double>(
                tween: Tween<double>(
                  begin: index.toDouble(),
                  end: index.toDouble(),
                ),
                duration: const Duration(milliseconds: 180),
                curve: Curves.easeInOutCubic,
                builder: (context, animatedIndex, _) {
                  final thumbCenter = 32 + span * animatedIndex / 3;
                  final activeWidth = math.max(0.0, thumbCenter - 16);
                  final activeOpacity = animatedIndex.clamp(0.0, 1.0);
                  return SizedBox(
                    height: 52,
                    child: Stack(
                      clipBehavior: Clip.none,
                      children: [
                        Positioned(
                          left: 16,
                          top: 11,
                          width: trackWidth,
                          height: 30,
                          child: DecoratedBox(
                            key: const ValueKey(
                              'composer-reasoning-track-base',
                            ),
                            decoration: BoxDecoration(
                              borderRadius: BorderRadius.circular(999),
                              border: Border.all(color: context.borderPrimary),
                              color: context.surfaceSecondary,
                            ),
                          ),
                        ),
                        Positioned(
                          left: 16,
                          top: 11,
                          width: activeWidth,
                          height: 30,
                          child: DecoratedBox(
                            key: const ValueKey(
                              'composer-reasoning-active-track',
                            ),
                            decoration: BoxDecoration(
                              borderRadius: BorderRadius.circular(999),
                              color: context.accentPrimary.withValues(
                                alpha: activeOpacity,
                              ),
                            ),
                          ),
                        ),
                        for (var marker = 0; marker < 4; marker += 1)
                          Positioned(
                            key: ValueKey('composer-reasoning-marker-$marker'),
                            left: 32 + span * marker / 3 - 3,
                            top: 23,
                            width: 6,
                            height: 6,
                            child: DecoratedBox(
                              decoration: BoxDecoration(
                                shape: BoxShape.circle,
                                color: marker <= animatedIndex
                                    ? context.surfacePrimary.withValues(
                                        alpha: 0.78,
                                      )
                                    : context.textTertiary.withValues(
                                        alpha: 0.55,
                                      ),
                              ),
                            ),
                          ),
                        Positioned(
                          key: const ValueKey('composer-reasoning-thumb'),
                          left: thumbCenter - 18,
                          top: 8,
                          width: 36,
                          height: 36,
                          child: DecoratedBox(
                            decoration: BoxDecoration(
                              shape: BoxShape.circle,
                              color: context.surfacePrimary,
                              boxShadow: const [
                                BoxShadow(
                                  color: Color(0x3D000000),
                                  blurRadius: 7,
                                  offset: Offset(0, 2),
                                ),
                                BoxShadow(
                                  color: Color(0x0D000000),
                                  spreadRadius: 1,
                                ),
                              ],
                            ),
                          ),
                        ),
                      ],
                    ),
                  );
                },
              ),
            ),
          ),
        );
      },
    );
  }
}

enum _ComposerModelMenuPage { effort, models, reasoning }

enum _VoiceGestureIntent { send, cancel, transcribe }

class _ComposerRoundButton extends StatelessWidget {
  final IconData icon;
  final double iconSize;
  final String tooltip;
  final VoidCallback onTap;

  const _ComposerRoundButton({
    super.key,
    required this.icon,
    this.iconSize = 17,
    required this.tooltip,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Tooltip(
      message: tooltip,
      child: GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: onTap,
        child: Container(
          width: 31,
          height: 31,
          alignment: Alignment.center,
          child: Icon(icon, size: iconSize, color: context.textPrimary),
        ),
      ),
    );
  }
}

class _ComposerTool extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _ComposerTool({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: SizedBox(
        height: 52,
        child: Row(
          children: [
            Icon(icon, size: 22, color: context.textPrimary),
            const SizedBox(width: 16),
            Text(label, style: AppTypography.body(context)),
          ],
        ),
      ),
    );
  }
}

class _EmotePicker extends StatefulWidget {
  final Future<List<Map<String, dynamic>>> Function()? loadEmotes;
  final void Function(String emoteId, String displayText) onSend;
  const _EmotePicker({this.loadEmotes, required this.onSend});

  @override
  State<_EmotePicker> createState() => _EmotePickerState();
}

class _EmotePickerState extends State<_EmotePicker> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _items = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final loader = widget.loadEmotes;
    if (loader == null) {
      setState(() => _loading = false);
      return;
    }
    try {
      final items = await loader();
      if (!mounted) return;
      setState(() {
        _items = items
            .where((item) {
              final enabled = item['enabled'];
              return enabled == null || enabled == true || enabled == 1;
            })
            .toList(growable: false);
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 20),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Center(
              child: Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: 14),
            Row(
              children: [
                Expanded(
                  child: Text('角色表情', style: AppTypography.cardTitle(context)),
                ),
                IconButton(
                  onPressed: _load,
                  icon: const Icon(Icons.refresh, size: 20),
                ),
              ],
            ),
            const SizedBox(height: 8),
            SizedBox(
              height: 240,
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                  ? Center(
                      child: Text(
                        '加载表情失败：$_error',
                        textAlign: TextAlign.center,
                      ),
                    )
                  : _items.isEmpty
                  ? const Center(child: Text('暂无已启用的服务端表情，请先在“表情管理”中导入'))
                  : GridView.builder(
                      gridDelegate:
                          const SliverGridDelegateWithFixedCrossAxisCount(
                            crossAxisCount: 4,
                            mainAxisSpacing: 8,
                            crossAxisSpacing: 8,
                            childAspectRatio: .9,
                          ),
                      itemCount: _items.length,
                      itemBuilder: (context, index) {
                        final item = _items[index];
                        final id = (item['id'] ?? '').toString();
                        final name = (item['name'] ?? item['altText'] ?? '表情')
                            .toString();
                        final emoji = (item['emoji'] ?? '').toString();
                        final imageUrl =
                            (item['imageUrl'] ??
                                    item['url'] ??
                                    item['path'] ??
                                    '')
                                .toString();
                        return InkWell(
                          borderRadius: AppRadius.brSmall,
                          onTap: id.isEmpty
                              ? null
                              : () => widget.onSend(
                                  id,
                                  emoji.isNotEmpty ? emoji : name,
                                ),
                          child: Container(
                            padding: const EdgeInsets.all(6),
                            decoration: BoxDecoration(
                              color: context.surfaceSecondary,
                              borderRadius: AppRadius.brSmall,
                            ),
                            child: Column(
                              mainAxisAlignment: MainAxisAlignment.center,
                              children: [
                                Expanded(
                                  child:
                                      imageUrl.startsWith('http') ||
                                          imageUrl.startsWith('/')
                                      ? Image.network(
                                          imageUrl,
                                          fit: BoxFit.contain,
                                          cacheWidth: 64,
                                          errorBuilder: (_, __, ___) => Text(
                                            emoji.isNotEmpty ? emoji : '🙂',
                                            style: const TextStyle(
                                              fontSize: 28,
                                            ),
                                          ),
                                        )
                                      : Center(
                                          child: Text(
                                            emoji.isNotEmpty ? emoji : '🙂',
                                            style: const TextStyle(
                                              fontSize: 28,
                                            ),
                                          ),
                                        ),
                                ),
                                const SizedBox(height: 4),
                                Text(
                                  name,
                                  maxLines: 1,
                                  overflow: TextOverflow.ellipsis,
                                  style: const TextStyle(fontSize: 11),
                                ),
                              ],
                            ),
                          ),
                        );
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }
}

class AmitiaPermissionSheet extends StatelessWidget {
  final String taskTitle;
  final List<String> permissions;
  final VoidCallback onAllowOnce;
  final VoidCallback onAllowAlways;
  final VoidCallback onDeny;

  const AmitiaPermissionSheet({
    super.key,
    required this.taskTitle,
    required this.permissions,
    required this.onAllowOnce,
    required this.onAllowAlways,
    required this.onDeny,
  });

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 0, 20, 34),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Amitia 想执行以下操作', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 4),
          Text(taskTitle, style: AppTypography.caption(context)),
          const SizedBox(height: 20),
          ...permissions.map(
            (p) => Padding(
              padding: EdgeInsets.only(bottom: 12),
              child: Row(
                children: [
                  Icon(
                    Icons.check_circle_outline,
                    size: 20,
                    color: context.accentPrimary,
                  ),
                  const SizedBox(width: 12),
                  Expanded(child: Text(p, style: AppTypography.body(context))),
                ],
              ),
            ),
          ),
          const SizedBox(height: 8),
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: context.warning.withValues(alpha: 0.08),
              borderRadius: AppRadius.brSmall,
            ),
            child: Row(
              children: [
                Icon(Icons.shield_outlined, size: 18, color: context.warning),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    '风险等级：中等 · 操作范围：本地文件',
                    style: AppTypography.label(
                      context,
                    ).copyWith(color: context.warning),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),
          AmitiaButton(
            label: '此次允许',
            isFullWidth: true,
            onPressed: onAllowOnce,
          ),
          const SizedBox(height: 8),
          AmitiaButton(
            label: '始终允许此工具',
            isFullWidth: true,
            isSecondary: true,
            onPressed: onAllowAlways,
          ),
          const SizedBox(height: 8),
          AmitiaButton(
            label: '拒绝',
            isFullWidth: true,
            isDestructive: true,
            onPressed: onDeny,
          ),
        ],
      ),
    );
  }
}
