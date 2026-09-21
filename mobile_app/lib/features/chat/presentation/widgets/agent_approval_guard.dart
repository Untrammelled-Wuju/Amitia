import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../runtime/conversation_runtime_controller.dart';

class AgentApprovalGuard extends ConsumerStatefulWidget {
  final String conversationId;

  const AgentApprovalGuard({super.key, required this.conversationId});

  @override
  ConsumerState<AgentApprovalGuard> createState() => _AgentApprovalGuardState();
}

class _AgentApprovalGuardState extends ConsumerState<AgentApprovalGuard> {
  bool _dialogOpen = false;
  String _activeApprovalId = '';

  Future<void> _showApproval(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString().trim();
    if (id.isEmpty || !mounted || _dialogOpen) return;
    _dialogOpen = true;
    _activeApprovalId = id;
    final toolName = (item['toolName'] ?? '工具调用').toString();
    final riskLevel = (item['riskLevel'] ?? '未知').toString();
    final arguments = _prettyArguments(item['arguments']);
    try {
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
      if (approved == null || !mounted) return;
      await ref
          .read(conversationRuntimeControllerProvider)
          .resolveApproval(id, approved);
    } finally {
      _dialogOpen = false;
      _activeApprovalId = '';
      if (mounted) setState(() {});
    }
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
  Widget build(BuildContext context) {
    final runtime = ref.watch(conversationRuntimeControllerProvider);
    final conversationId = widget.conversationId.trim();
    final approvals = runtime.pendingApprovals
        .where((item) =>
            conversationId.isNotEmpty &&
            (item['conversationId'] ?? '').toString().trim() == conversationId)
        .toList(growable: false);
    if (_dialogOpen &&
        _activeApprovalId.isNotEmpty &&
        !approvals.any((item) => (item['id'] ?? '').toString() == _activeApprovalId)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted || !_dialogOpen) return;
        Navigator.of(context, rootNavigator: true).maybePop();
      });
    } else if (!_dialogOpen && approvals.isNotEmpty) {
      final item = Map<String, dynamic>.from(approvals.first);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _showApproval(item);
      });
    }
    return const SizedBox.shrink();
  }
}
