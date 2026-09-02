import 'dart:async';

import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_motion.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../shared/models/models.dart';
import 'amitia_button.dart';
import 'amitia_misc.dart';

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
  final VoidCallback? onAgentTaskTap;
  final VoidCallback? onPauseAgentTask;
  final VoidCallback? onResumeAgentTask;
  final String? agentTaskStatusLabel;

  const AmitiaMessageBubble({
    super.key,
    required this.message,
    this.showAvatar = true,
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
    this.onAgentTaskTap,
    this.onPauseAgentTask,
    this.onResumeAgentTask,
    this.agentTaskStatusLabel,
  });

  @override
  Widget build(BuildContext context) {
    if (message.type == MessageType.systemNotice) {
      return Padding(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.sm),
        child: Center(
          child: Text(
            message.content,
            style: TextStyle(fontSize: 11, color: context.textTertiary),
          ),
        ),
      );
    }

    if (message.type == MessageType.agentTask) {
      return _AgentTaskMessage(
        message: message,
        onAgentTaskTap: onAgentTaskTap,
        onPauseAgentTask: onPauseAgentTask,
        onResumeAgentTask: onResumeAgentTask,
        statusLabel: agentTaskStatusLabel ?? '运行中',
      );
    }

    final isUser = message.role == MessageRole.user;
    final displayName = isUser
        ? ((userName ?? '').trim().isEmpty ? '我' : userName!.trim())
        : ((characterName ?? '').trim().isEmpty ? 'AI' : characterName!.trim());
    final initial = isUser
        ? ((userInitial ?? '').trim().isEmpty ? displayName.characters.first : userInitial!.trim())
        : ((avatarInitial ?? '').trim().isEmpty ? displayName.characters.first : avatarInitial!.trim());
    final colorHex = isUser
        ? ((userAvatarColor ?? '').trim().isEmpty ? '#5F6872' : userAvatarColor!.trim())
        : ((avatarColor ?? '').trim().isEmpty ? '#8A5728' : avatarColor!.trim());

    final messageColumn = Flexible(
      child: Column(
        crossAxisAlignment: isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
        children: [
          Padding(
            padding: EdgeInsets.only(
              left: isUser ? 0 : 2,
              right: isUser ? 2 : 0,
              bottom: 4,
            ),
            child: Text(
              displayName,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.label(context).copyWith(
                fontSize: 10,
                color: context.textTertiary,
                height: 1.2,
              ),
            ),
          ),
          if ((message.replyToMessageId ?? '').isNotEmpty) ...[
            Container(
              constraints: const BoxConstraints(maxWidth: 280),
              margin: const EdgeInsets.only(bottom: 5),
              padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 6),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
                border: Border(left: BorderSide(color: context.accentPrimary, width: 2)),
              ),
              child: Text(
                '引用：${(message.replyToExcerpt ?? '原消息').trim()}',
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.label(context).copyWith(color: context.textSecondary),
              ),
            ),
          ],
          _buildContent(context, isUser),
          if (!(showThinking && message.content.trim().isEmpty))
            Padding(
              padding: const EdgeInsets.only(top: 3),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    _formatTime(message.time),
                    style: AppTypography.label(context).copyWith(
                      fontSize: 9,
                      color: context.textTertiary,
                    ),
                  ),
                  if (onReply != null) ...[
                    const SizedBox(width: 7),
                    GestureDetector(
                      behavior: HitTestBehavior.opaque,
                      onTap: onReply,
                      child: Padding(
                        padding: const EdgeInsets.symmetric(horizontal: 2, vertical: 2),
                        child: Icon(Icons.reply_rounded, size: 14, color: context.textTertiary),
                      ),
                    ),
                  ],
                ],
              ),
            ),
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
    );

    return Padding(
      padding: EdgeInsets.only(
        left: AppSpacing.lg,
        right: AppSpacing.lg,
        bottom: 14,
      ),
      child: Row(
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (!isUser) ...[
            if (showAvatar)
              _MiniAvatar(initial: initial, colorHex: colorHex)
            else
              const SizedBox(width: 32, height: 32),
            const SizedBox(width: 8),
          ],
          messageColumn,
          if (isUser) ...[
            const SizedBox(width: 8),
            if (showAvatar)
              _MiniAvatar(initial: initial, colorHex: colorHex)
            else
              const SizedBox(width: 32, height: 32),
          ],
        ],
      ),
    );
  }

  Widget _buildContent(BuildContext context, bool isUser) {
    if (!isUser && (showThinking || agentActivities.isNotEmpty || message.type == MessageType.toolCall)) {
      final activities = <AmitiaAgentActivity>[
        ...agentActivities,
        if (message.type == MessageType.toolCall)
          AmitiaAgentActivity(
            id: message.id,
            title: (message.toolName ?? '').trim().isEmpty ? '工具调用' : message.toolName!.trim(),
            status: message.status == MessageStatus.error ? 'failed' : 'completed',
            errorCode: message.status == MessageStatus.error ? message.toolResult : null,
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
        return _EmoteMessage(
          emoji: message.content,
          name: '',
          isUser: isUser,
        );
      case MessageType.code:
        final parsed = _parseCodeFence(message.content);
        return _CodeMessage(
          lang: parsed.$1,
          body: parsed.$2,
          isUser: isUser,
        );
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
        border: isUser ? null : Border.all(color: context.borderPrimary, width: 0.6),
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

  (String, String) _parseCodeFence(String content) {
    final trimmed = content.trim();
    if (!trimmed.startsWith('```') || !trimmed.endsWith('```')) {
      return ('text', content);
    }
    final withoutPrefix = trimmed.substring(3);
    final newline = withoutPrefix.indexOf('\n');
    if (newline < 0) return ('text', withoutPrefix.replaceFirst(RegExp(r'```$'), ''));
    final language = withoutPrefix.substring(0, newline).trim();
    final body = withoutPrefix.substring(newline + 1, withoutPrefix.length - 3);
    return (language.isEmpty ? 'text' : language, body);
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
      constraints: BoxConstraints(maxWidth: MediaQuery.sizeOf(context).width * 0.80),
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
              onTap: count > 0 ? () => setState(() => _expanded = !_expanded) : null,
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
                          : Icon(Icons.auto_awesome_outlined, size: 15, color: context.accentPrimary),
                    ),
                    const SizedBox(width: 9),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            widget.showThinking ? '思考中' : '执行过程',
                            style: AppTypography.bodySmall(context).copyWith(
                              fontWeight: FontWeight.w600,
                              fontSize: 12,
                            ),
                          ),
                          const SizedBox(height: 2),
                          Text(
                            count == 0
                                ? '正在处理你的请求'
                                : widget.showThinking
                                    ? '正在处理 · 已完成 $count 个工具调用'
                                    : '已完成 $count 个工具调用',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: AppTypography.label(context).copyWith(fontSize: 9.5),
                          ),
                        ],
                      ),
                    ),
                    if (count > 0) ...[
                      Text(
                        '$count 项',
                        style: AppTypography.label(context).copyWith(fontSize: 9.5),
                      ),
                      const SizedBox(width: 3),
                      AnimatedRotation(
                        turns: _expanded ? 0.25 : 0,
                        duration: AppMotion.standard,
                        child: Icon(Icons.chevron_right, size: 16, color: context.textTertiary),
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
            for (final activity in widget.activities) _AgentActivityRow(activity: activity),
          ],
          if (hasProcess && hasFinal) ...[
            const SizedBox(height: 9),
            Container(height: 1, color: context.borderPrimary),
            const SizedBox(height: 9),
          ],
          if (hasFinal)
            Text(
              widget.finalText,
              style: AppTypography.bodySmall(context).copyWith(
                color: context.textPrimary,
                height: 1.52,
              ),
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
                  style: AppTypography.bodySmall(context).copyWith(
                    fontWeight: FontWeight.w600,
                    fontSize: 11,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  failed
                      ? ((activity.errorCode ?? '').isEmpty ? '执行失败' : '执行失败 · ${activity.errorCode}')
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

class _MiniAvatar extends StatelessWidget {
  final String initial;
  final String colorHex;
  const _MiniAvatar({required this.initial, required this.colorHex});

  @override
  Widget build(BuildContext context) {
    final color = Color(
      int.parse('FF${colorHex.replaceAll('#', '')}', radix: 16),
    );
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(11),
      ),
      child: Center(
        child: Text(
          initial,
          style: const TextStyle(
            color: Colors.white,
            fontSize: 14,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}

Color _parseHex(String hex) {
  final cleaned = hex.replaceAll('#', '');
  return Color(int.parse('FF$cleaned', radix: 16));
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
                      frameBuilder: (context, child, frame, wasSynchronouslyLoaded) {
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
        child: Icon(
          Icons.image_outlined,
          size: 40,
          color: context.accentPrimary,
        ),
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
            child: Icon(
              Icons.videocam_outlined,
              color: context.accentPrimary,
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
  final String? recipientName;
  final bool isAgentMode;
  final ValueChanged<bool>? onAgentModeChanged;
  final FutureOr<void> Function()? onPickFile;
  final FutureOr<void> Function(bool camera)? onPickImage;
  final FutureOr<void> Function(bool camera)? onPickVideo;
  final FutureOr<void> Function()? onPickAudio;
  final void Function(String lang, String code)? onSendCode;
  final Future<List<Map<String, dynamic>>> Function()? onLoadEmotes;
  final void Function(String emoteId, String displayText)? onSendEmote;
  final Future<List<Map<String, dynamic>>> Function()? onLoadAgentSkills;
  final String? replyPreview;
  final VoidCallback? onCancelReply;

  const AmitiaChatInput({
    super.key,
    required this.onSend,
    this.recipientName,
    this.isAgentMode = false,
    this.onAgentModeChanged,
    this.onPickFile,
    this.onPickImage,
    this.onPickVideo,
    this.onPickAudio,
    this.onSendCode,
    this.onLoadEmotes,
    this.onSendEmote,
    this.onLoadAgentSkills,
    this.replyPreview,
    this.onCancelReply,
  });

  @override
  State<AmitiaChatInput> createState() => _AmitiaChatInputState();
}

class _AmitiaChatInputState extends State<AmitiaChatInput> {
  final _controller = TextEditingController();
  final _inputFocusNode = FocusNode();
  bool _hasText = false;
  bool _isInputFocused = false;
  final List<String> _selectedSkillNames = <String>[];

  @override
  void initState() {
    super.initState();
    _inputFocusNode.addListener(_syncInputFocus);
  }

  void _syncInputFocus() {
    setState(() => _isInputFocused = _inputFocusNode.hasFocus);
  }

  @override
  void dispose() {
    _inputFocusNode.removeListener(_syncInputFocus);
    _inputFocusNode.dispose();
    _controller.dispose();
    super.dispose();
  }

  void _send() {
    final text = _controller.text.trim();
    if (text.isEmpty && _selectedSkillNames.isEmpty) return;
    final prefix = _selectedSkillNames.map((name) => '\$$name').join(' ');
    final outgoing = [prefix, text].where((part) => part.trim().isNotEmpty).join(' ');
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
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('加载 Agent Skill 失败：$error')));
      return;
    }
    if (!mounted) return;
    final usable = skills.where((skill) {
      final enabled = skill['enabled'] == true || skill['isEnabled'] == true || skill['enabled'] == 1;
      final status = (skill['compatibilityStatus'] ?? skill['compatibility'] ?? '').toString().toLowerCase();
      return enabled && status != 'blocked' && status != 'incompatible';
    }).toList();
    await showModalBottomSheet<void>(
      context: context,
      backgroundColor: context.surfacePrimary,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(24))),
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
                      Expanded(child: Text('本次消息使用 Agent Skill', style: AppTypography.pageTitle(sheetContext))),
                      IconButton(onPressed: () => Navigator.pop(sheetContext), icon: const Icon(Icons.close)),
                    ],
                  ),
                ),
                Divider(height: 1, color: sheetContext.borderSecondary),
                Expanded(
                  child: usable.isEmpty
                      ? Center(child: Text('暂无已启用且兼容的 Agent Skill', style: AppTypography.caption(sheetContext)))
                      : ListView.builder(
                          itemCount: usable.length,
                          itemBuilder: (_, index) {
                            final skill = usable[index];
                            final name = (skill['name'] ?? '').toString();
                            final displayName = (skill['displayName'] ?? name).toString();
                            final selected = _selectedSkillNames.contains(name);
                            return CheckboxListTile(
                              value: selected,
                              title: Text(displayName),
                              subtitle: Text((skill['shortDescription'] ?? skill['description'] ?? '').toString(), maxLines: 2, overflow: TextOverflow.ellipsis),
                              onChanged: name.isEmpty
                                  ? null
                                  : (value) {
                                      setState(() {
                                        if (value == true) {
                                          if (!_selectedSkillNames.contains(name)) _selectedSkillNames.add(name);
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

  void _showVoiceSheet() {
    widget.onPickAudio?.call();
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

  @override
  Widget build(BuildContext context) {
    final recipient = (widget.recipientName ?? '').trim();
    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(10, 8, 10, 12),
        child: Container(
          key: const ValueKey('chat-composer-surface'),
          constraints: const BoxConstraints(minHeight: 98, maxHeight: 170),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: BorderRadius.circular(23),
            border: Border.all(
              color: _isInputFocused ? context.accentPrimary : context.borderPrimary,
              width: _isInputFocused ? 1.0 : 0.8,
            ),
            boxShadow: [
              BoxShadow(
                color: Colors.black.withValues(
                  alpha: Theme.of(context).brightness == Brightness.dark ? 0.16 : 0.045,
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
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
                  decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
                  child: Row(
                    children: [
                      Icon(Icons.reply_rounded, size: 16, color: context.accentPrimary),
                      const SizedBox(width: 7),
                      Expanded(child: Text(widget.replyPreview!, maxLines: 1, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context))),
                      GestureDetector(onTap: widget.onCancelReply, child: Icon(Icons.close, size: 17, color: context.textTertiary)),
                    ],
                  ),
                ),
              if (_selectedSkillNames.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.fromLTRB(12, 8, 12, 0),
                  child: Wrap(
                    spacing: 6,
                    runSpacing: 6,
                    children: _selectedSkillNames.map((name) => InputChip(
                      visualDensity: VisualDensity.compact,
                      avatar: const Icon(Icons.auto_awesome_outlined, size: 14),
                      label: Text('\$$name'),
                      onDeleted: () => setState(() => _selectedSkillNames.remove(name)),
                    )).toList(),
                  ),
                ),
              TapRegion(
                onTapOutside: (_) => _inputFocusNode.unfocus(),
                child: TextField(
                  controller: _controller,
                  focusNode: _inputFocusNode,
                  minLines: 1,
                  maxLines: 4,
                  textCapitalization: TextCapitalization.sentences,
                  onChanged: (value) => setState(() => _hasText = value.trim().isNotEmpty),
                  onSubmitted: (_) => _send(),
                  style: AppTypography.bodySmall(context).copyWith(fontSize: 16),
                  decoration: InputDecoration(
                    hintText: recipient.isEmpty ? '发消息…' : '给 $recipient 发消息…',
                    hintStyle: TextStyle(color: context.textTertiary, fontSize: 16),
                    isDense: true,
                    contentPadding: const EdgeInsets.fromLTRB(16, 14, 16, 8),
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
              Padding(
                padding: const EdgeInsets.fromLTRB(9, 0, 9, 8),
                child: SizedBox(
                  height: 48,
                  child: Row(
                    children: [
                      _ComposerRoundButton(
                        icon: Icons.add_rounded,
                        tooltip: '添加内容',
                        onTap: _showComposerTools,
                      ),
                      const SizedBox(width: 6),
                      _ClaudeStyleAgentChip(
                        isEnabled: widget.isAgentMode,
                        onTap: () =>
                            widget.onAgentModeChanged?.call(!widget.isAgentMode),
                      ),
                      const Spacer(),
                      _ComposerRoundButton(
                        icon: Icons.mic_none_outlined,
                        tooltip: '选择音频作为语音消息',
                        onTap: _showVoiceSheet,
                      ),
                      const SizedBox(width: 6),
                      GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onTap: (_hasText || _selectedSkillNames.isNotEmpty) ? _send : null,
                        child: Tooltip(
                          message: '发送消息',
                          child: Container(
                            width: 31,
                            height: 31,
                            decoration: BoxDecoration(
                              color: (_hasText || _selectedSkillNames.isNotEmpty)
                                  ? context.accentPrimary
                                  : context.borderPrimary,
                              shape: BoxShape.circle,
                            ),
                            alignment: Alignment.center,
                            child: Icon(
                              Icons.arrow_upward_rounded,
                              size: 18,
                              color: (_hasText || _selectedSkillNames.isNotEmpty)
                                  ? context.surfacePrimary
                                  : context.textTertiary,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ComposerRoundButton extends StatelessWidget {
  final IconData icon;
  final String tooltip;
  final VoidCallback onTap;

  const _ComposerRoundButton({
    required this.icon,
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
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: context.borderPrimary),
          ),
          alignment: Alignment.center,
          child: Icon(icon, size: 17, color: context.textPrimary),
        ),
      ),
    );
  }
}
class _ClaudeStyleAgentChip extends StatelessWidget {
  final bool isEnabled;
  final VoidCallback onTap;

  const _ClaudeStyleAgentChip({required this.isEnabled, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: onTap,
      child: Container(
        height: 31,
        padding: const EdgeInsets.symmetric(horizontal: 9),
        decoration: BoxDecoration(
          color: isEnabled ? context.accentSoft : context.surfaceSecondary,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(
            color: isEnabled ? context.accentPrimary : context.borderPrimary,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.auto_awesome_outlined,
              size: 14,
              color: isEnabled ? context.accentPrimary : context.textSecondary,
            ),
            const SizedBox(width: 5),
            Text(
              'Agent',
              style: AppTypography.label(context).copyWith(
                color: isEnabled ? context.accentPrimary : context.textSecondary,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
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
        _items = items.where((item) {
          final enabled = item['enabled'];
          return enabled == null || enabled == true || enabled == 1;
        }).toList(growable: false);
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
                decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)),
              ),
            ),
            const SizedBox(height: 14),
            Row(
              children: [
                Expanded(child: Text('角色表情', style: AppTypography.cardTitle(context))),
                IconButton(onPressed: _load, icon: const Icon(Icons.refresh, size: 20)),
              ],
            ),
            const SizedBox(height: 8),
            SizedBox(
              height: 240,
              child: _loading
                  ? const Center(child: CircularProgressIndicator())
                  : _error != null
                      ? Center(child: Text('加载表情失败：$_error', textAlign: TextAlign.center))
                      : _items.isEmpty
                          ? const Center(child: Text('暂无已启用的服务端表情，请先在“表情管理”中导入'))
                          : GridView.builder(
                              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                                crossAxisCount: 4,
                                mainAxisSpacing: 8,
                                crossAxisSpacing: 8,
                                childAspectRatio: .9,
                              ),
                              itemCount: _items.length,
                              itemBuilder: (context, index) {
                                final item = _items[index];
                                final id = (item['id'] ?? '').toString();
                                final name = (item['name'] ?? item['altText'] ?? '表情').toString();
                                final emoji = (item['emoji'] ?? '').toString();
                                final imageUrl = (item['imageUrl'] ?? item['url'] ?? item['path'] ?? '').toString();
                                return InkWell(
                                  borderRadius: AppRadius.brSmall,
                                  onTap: id.isEmpty ? null : () => widget.onSend(id, emoji.isNotEmpty ? emoji : name),
                                  child: Container(
                                    padding: const EdgeInsets.all(6),
                                    decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
                                    child: Column(
                                      mainAxisAlignment: MainAxisAlignment.center,
                                      children: [
                                        Expanded(
                                          child: imageUrl.startsWith('http') || imageUrl.startsWith('/')
                                              ? Image.network(imageUrl, fit: BoxFit.contain, cacheWidth: 64, errorBuilder: (_, __, ___) => Text(emoji.isNotEmpty ? emoji : '🙂', style: const TextStyle(fontSize: 28)))
                                              : Center(child: Text(emoji.isNotEmpty ? emoji : '🙂', style: const TextStyle(fontSize: 28))),
                                        ),
                                        const SizedBox(height: 4),
                                        Text(name, maxLines: 1, overflow: TextOverflow.ellipsis, style: const TextStyle(fontSize: 11)),
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
