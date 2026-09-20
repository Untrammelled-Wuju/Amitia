import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';

class AgentApprovalGuard extends ConsumerStatefulWidget {
  final String conversationId;

  const AgentApprovalGuard({super.key, required this.conversationId});

  @override
  ConsumerState<AgentApprovalGuard> createState() => _AgentApprovalGuardState();
}

class _AgentApprovalGuardState extends ConsumerState<AgentApprovalGuard> {
  Timer? _timer;
  bool _loading = false;
  bool _dialogOpen = false;

  @override
  void initState() {
    super.initState();
    _restart();
  }

  @override
  void didUpdateWidget(covariant AgentApprovalGuard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.conversationId != widget.conversationId) {
      _restart();
    }
  }

  void _restart() {
    _timer?.cancel();
    _timer = null;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      unawaited(_poll());
      _timer = Timer.periodic(const Duration(milliseconds: 1200), (_) {
        unawaited(_poll());
      });
    });
  }

  Future<void> _poll() async {
    final conversationId = widget.conversationId.trim();
    if (!mounted || _loading || _dialogOpen) return;
    _loading = true;
    try {
      final items = await ref
          .read(chatServiceProvider)
          .listApprovals(conversationId);
      if (!mounted || items.isEmpty || _dialogOpen) return;
      _dialogOpen = true;
      await _showApproval(items.first);
    } catch (_) {
      return;
    } finally {
      _loading = false;
      _dialogOpen = false;
    }
  }

  Future<void> _showApproval(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString().trim();
    if (id.isEmpty || !mounted) return;
    final toolName = (item['toolName'] ?? '工具调用').toString();
    final riskLevel = (item['riskLevel'] ?? '未知').toString();
    final arguments = _prettyArguments(item['arguments']);
    final approved = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => AlertDialog(
        title: const Text('工具执行需要批准'),
        content: SizedBox(
          width: 560,
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(toolName, style: AppTypography.sectionTitle(dialogContext)),
                SizedBox(height: AppSpacing.xs),
                Text(
                  '本次批准仅允许当前工具调用继续执行，不会更改会话的权限模式。',
                  style: AppTypography.bodySmall(dialogContext),
                ),
                SizedBox(height: AppSpacing.md),
                Text('风险等级：$riskLevel', style: AppTypography.caption(dialogContext)),
                SizedBox(height: AppSpacing.sm),
                Text('参数', style: AppTypography.label(dialogContext)),
                SizedBox(height: AppSpacing.xs),
                Container(
                  width: double.infinity,
                  constraints: const BoxConstraints(maxHeight: 260),
                  padding: const EdgeInsets.all(10),
                  decoration: BoxDecoration(
                    color: dialogContext.surfaceSecondary,
                    borderRadius: BorderRadius.circular(10),
                  ),
                  child: SingleChildScrollView(
                    child: SelectableText(
                      arguments,
                      style: AppTypography.caption(dialogContext).copyWith(
                        fontFamily: 'monospace',
                        height: 1.5,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: Text('拒绝', style: TextStyle(color: dialogContext.error)),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('本次允许'),
          ),
        ],
      ),
    );
    if (approved == null) return;
    await ref.read(chatServiceProvider).resolveApproval(id, approved);
  }

  String _prettyArguments(dynamic value) {
    final raw = (value ?? '').toString().trim();
    if (raw.isEmpty) return '无参数';
    try {
      return const JsonEncoder.withIndent('  ').convert(jsonDecode(raw));
    } catch (_) {
      return raw.length > 8000 ? raw.substring(0, 8000) : raw;
    }
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => const SizedBox.shrink();
}
