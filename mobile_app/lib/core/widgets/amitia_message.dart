import 'package:flutter/material.dart';
import '../../app/theme/app_colors.dart';
import '../../app/theme/app_spacing.dart';
import '../../app/theme/app_radius.dart';
import '../../app/theme/app_typography.dart';
import '../../shared/models/models.dart';
import 'amitia_button.dart';
import 'amitia_misc.dart';

class AmitiaMessageBubble extends StatelessWidget {
  final ChatMessage message;
  final bool showAvatar;
  final String? avatarInitial;
  final String? avatarColor;

  const AmitiaMessageBubble({
    super.key,
    required this.message,
    this.showAvatar = false,
    this.avatarInitial,
    this.avatarColor,
  });

  @override
  Widget build(BuildContext context) {
    if (message.type == MessageType.systemNotice) {
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.sm),
        child: Center(
          child: Text(
            message.content,
            style: TextStyle(fontSize: 11, color: context.textTertiary),
          ),
        ),
      );
    }

    if (message.type == MessageType.agentTask) {
      return _AgentTaskMessage(message: message);
    }

    if (message.type == MessageType.toolCall) {
      return _ToolCallMessage(message: message);
    }

    final isUser = message.role == MessageRole.user;
    return Padding(
      padding: EdgeInsets.only(
        left: isUser ? 60 : AppSpacing.lg,
        right: isUser ? AppSpacing.lg : 60,
        bottom: AppSpacing.sm,
      ),
      child: Row(
        mainAxisAlignment: isUser ? MainAxisAlignment.end : MainAxisAlignment.start,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (!isUser && showAvatar) ...[
            _MiniAvatar(initial: avatarInitial ?? '阿', colorHex: avatarColor ?? '#7668EE'),
            const SizedBox(width: 8),
          ] else if (!isUser)
            const SizedBox(width: 40),
          Flexible(
            child: Column(
              crossAxisAlignment: isUser ? CrossAxisAlignment.end : CrossAxisAlignment.start,
              children: [
                if (message.type == MessageType.file)
                  _FileMessage(fileName: message.fileName ?? '', fileSizeKB: message.fileSizeKB ?? 0, isUser: isUser)
                else
                  Container(
                    constraints: BoxConstraints(maxWidth: MediaQuery.sizeOf(context).width * (isUser ? 0.78 : 0.82)),
                    padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                    decoration: BoxDecoration(
                      color: isUser ? context.accentSoft : context.surfacePrimary,
                      borderRadius: BorderRadius.only(
                        topLeft: const Radius.circular(16),
                        topRight: const Radius.circular(16),
                        bottomLeft: Radius.circular(isUser ? 16 : 4),
                        bottomRight: Radius.circular(isUser ? 4 : 16),
                      ),
                      border: isUser ? null : Border.all(color: context.borderPrimary, width: 0.5),
                    ),
                    child: Text(
                      message.content,
                      style: AppTypography.bodySmall(context).copyWith(
                        color: isUser ? context.accentPressed : context.textPrimary,
                        height: 1.45,
                      ),
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
                          child: Text('重试', style: TextStyle(fontSize: 12, color: context.error)),
                        ),
                      ],
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
    final color = Color(int.parse('FF${colorHex.replaceAll('#', '')}', radix: 16));
    return Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(color: color, shape: BoxShape.circle),
      child: Center(
        child: Text(initial, style: const TextStyle(color: Colors.white, fontSize: 14, fontWeight: FontWeight.w600)),
      ),
    );
  }
}

class _FileMessage extends StatelessWidget {
  final String fileName;
  final int fileSizeKB;
  final bool isUser;

  const _FileMessage({required this.fileName, required this.fileSizeKB, required this.isUser});

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: BoxConstraints(maxWidth: MediaQuery.sizeOf(context).width * 0.7),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: isUser ? context.accentSoft : context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: isUser ? null : Border.all(color: context.borderPrimary, width: 0.5),
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
            child: Icon(Icons.description_outlined, color: context.accentPrimary, size: 22),
          ),
          const SizedBox(width: 12),
          Flexible(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(fileName, style: AppTypography.bodySmall(context), overflow: TextOverflow.ellipsis),
                const SizedBox(height: 2),
                Text('${(fileSizeKB / 1024).toStringAsFixed(1)} MB', style: AppTypography.label(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _AgentTaskMessage extends StatelessWidget {
  final ChatMessage message;
  const _AgentTaskMessage({required this.message});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(left: AppSpacing.lg, right: 60, bottom: AppSpacing.sm),
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
                      Icon(Icons.auto_awesome, size: 16, color: context.accentPrimary),
                      const SizedBox(width: 6),
                      Text('正在执行：${message.agentTaskTitle ?? ''}', style: AppTypography.cardTitle(context).copyWith(fontSize: 14)),
                    ],
                  ),
                  const SizedBox(height: 12),
                  ...(message.agentTaskSteps ?? []).map((step) => Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: Row(
                      children: [
                        Container(
                          width: 6,
                          height: 6,
                          decoration: BoxDecoration(color: context.accentPrimary, shape: BoxShape.circle),
                        ),
                        const SizedBox(width: 8),
                        Text(step, style: AppTypography.caption(context)),
                      ],
                    ),
                  )),
                  const SizedBox(height: 10),
                  AmitiaProgressBar(progress: (message.agentTaskProgress ?? 0) / 100),
                  const SizedBox(height: 8),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('进度 ${message.agentTaskProgress ?? 0}%', style: AppTypography.label(context)),
                      Text('已用时 ${message.agentTaskElapsed ?? '00:00'}', style: AppTypography.label(context)),
                    ],
                  ),
                  const SizedBox(height: 12),
                  Row(
                    children: [
                      GestureDetector(
                        child: Text('查看详情', style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500)),
                      ),
                      const Spacer(),
                      GestureDetector(
                        child: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Text('暂停', style: TextStyle(fontSize: 12, color: context.accentPrimary)),
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
      padding: const EdgeInsets.only(left: AppSpacing.lg, right: 60, bottom: AppSpacing.sm),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const SizedBox(width: 40),
          Flexible(
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.build_outlined, size: 16, color: context.textTertiary),
                  const SizedBox(width: 8),
                  Expanded(
                    child: RichText(
                      text: TextSpan(
                        style: AppTypography.caption(context),
                        children: [
                          TextSpan(text: '${message.toolName ?? '工具'}: ', style: TextStyle(color: context.accentPrimary, fontWeight: FontWeight.w500)),
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

class AmitiaChatInput extends StatefulWidget {
  final ValueChanged<String> onSend;
  final bool isAgentMode;
  final ValueChanged<bool>? onAgentModeChanged;

  const AmitiaChatInput({
    super.key,
    required this.onSend,
    this.isAgentMode = false,
    this.onAgentModeChanged,
  });

  @override
  State<AmitiaChatInput> createState() => _AmitiaChatInputState();
}

class _AmitiaChatInputState extends State<AmitiaChatInput> {
  final _controller = TextEditingController();
  bool _hasText = false;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  void _send() {
    final text = _controller.text.trim();
    if (text.isEmpty) return;
    widget.onSend(text);
    _controller.clear();
    setState(() => _hasText = false);
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Container(
        padding: const EdgeInsets.fromLTRB(12, 8, 12, 8),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          border: Border(top: BorderSide(color: context.borderSecondary, width: 1)),
        ),
        child: Column(
          children: [
            Row(
              children: [
                _InputAction(icon: Icons.attach_file_outlined, onTap: () {}),
                const SizedBox(width: 6),
                _InputAction(icon: Icons.image_outlined, onTap: () {}),
                const SizedBox(width: 6),
                _InputAction(icon: Icons.code, onTap: () {}),
                const SizedBox(width: 6),
                _InputAction(icon: Icons.mic_outlined, onTap: () {}),
                const SizedBox(width: 8),
                Expanded(
                  child: Container(
                    constraints: const BoxConstraints(maxHeight: 120),
                    child: TextField(
                      controller: _controller,
                      maxLines: null,
                      onChanged: (v) => setState(() => _hasText = v.trim().isNotEmpty),
                      style: AppTypography.bodySmall(context),
                      decoration: InputDecoration(
                        hintText: '输入消息……',
                        hintStyle: TextStyle(color: context.textTertiary, fontSize: 14),
                        isDense: true,
                        contentPadding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                        filled: true,
                        fillColor: context.surfaceSecondary,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brLarge,
                          borderSide: BorderSide.none,
                        ),
                      ),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                GestureDetector(
                  onTap: _send,
                  child: Container(
                    width: 40,
                    height: 40,
                    decoration: BoxDecoration(
                      color: _hasText ? context.accentPrimary : context.accentPrimary.withValues(alpha: 0.4),
                      shape: BoxShape.circle,
                    ),
                    child: const Icon(Icons.arrow_upward, color: Colors.white, size: 20),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 6),
            Row(
              children: [
                GestureDetector(
                  onTap: () => widget.onAgentModeChanged?.call(!widget.isAgentMode),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: widget.isAgentMode ? context.accentSoft : Colors.transparent,
                      borderRadius: AppRadius.brTag,
                      border: widget.isAgentMode ? null : Border.all(color: context.borderPrimary, width: 1),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.auto_awesome, size: 14, color: widget.isAgentMode ? context.accentPrimary : context.textTertiary),
                        const SizedBox(width: 4),
                        Text('Agent 模式', style: TextStyle(fontSize: 12, color: widget.isAgentMode ? context.accentPrimary : context.textTertiary, fontWeight: FontWeight.w500)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _InputAction extends StatelessWidget {
  final IconData icon;
  final VoidCallback onTap;
  const _InputAction({required this.icon, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Icon(icon, size: 20, color: context.textTertiary),
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
      padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('Amitia 想执行以下操作', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 4),
          Text(taskTitle, style: AppTypography.caption(context)),
          const SizedBox(height: 20),
          ...permissions.map((p) => Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: Row(
              children: [
                Icon(Icons.check_circle_outline, size: 20, color: context.accentPrimary),
                const SizedBox(width: 12),
                Expanded(child: Text(p, style: AppTypography.body(context))),
              ],
            ),
          )),
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
                Expanded(child: Text('风险等级：中等 · 操作范围：本地文件', style: AppTypography.label(context).copyWith(color: context.warning))),
              ],
            ),
          ),
          const SizedBox(height: 20),
          AmitiaButton(label: '此次允许', isFullWidth: true, onPressed: onAllowOnce),
          const SizedBox(height: 8),
          AmitiaButton(label: '始终允许此工具', isFullWidth: true, isSecondary: true, onPressed: onAllowAlways),
          const SizedBox(height: 8),
          AmitiaButton(label: '拒绝', isFullWidth: true, isDestructive: true, onPressed: onDeny),
        ],
      ),
    );
  }
}
