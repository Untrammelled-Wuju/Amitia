import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/models/attention.dart';
import '../../../../core/services/mcp_interaction_events.dart';
import '../../../../core/services/providers.dart';
import '../../runtime/conversation_runtime_controller.dart';

class AgentApprovalGuard extends ConsumerStatefulWidget {
  final String conversationId;

  const AgentApprovalGuard({super.key, required this.conversationId});

  @override
  ConsumerState<AgentApprovalGuard> createState() => _AgentApprovalGuardState();
}

class _AgentApprovalGuardState extends ConsumerState<AgentApprovalGuard>
    with WidgetsBindingObserver {
  bool _dialogOpen = false;
  String _activeApprovalId = '';
  String _activeAttentionId = '';
  List<AttentionItem> _attentionItems = const [];
  final Set<String> _dismissedAttentionIds = <String>{};
  StreamSubscription<void>? _interactionSubscription;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    unawaited(_loadAttention());
    _interactionSubscription = MCPInteractionEvents.pending.listen((_) {
      if (mounted && ModalRoute.of(context)?.isCurrent == true) {
        unawaited(_loadAttention());
      }
    });
  }

  @override
  void didUpdateWidget(covariant AgentApprovalGuard oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.conversationId != widget.conversationId) {
      unawaited(_loadAttention());
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      unawaited(_loadAttention());
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    unawaited(_interactionSubscription?.cancel());
    super.dispose();
  }

  Future<void> _loadAttention() async {
    try {
      final items = await ref.read(attentionServiceProvider).items();
      final liveIds = items.map((item) => item.id).toSet();
      _dismissedAttentionIds.removeWhere((id) => !liveIds.contains(id));
      if (mounted) setState(() => _attentionItems = items);
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
    }
  }

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
                  Text(
                    toolName,
                    style: AppTypography.sectionTitle(dialogContext),
                  ),
                  SizedBox(height: AppSpacing.xs),
                  Text(
                    '本次批准仅允许当前工具调用继续执行，不会更改会话的权限模式。',
                    style: AppTypography.bodySmall(dialogContext),
                  ),
                  SizedBox(height: AppSpacing.md),
                  Text(
                    '风险等级：$riskLevel',
                    style: AppTypography.caption(dialogContext),
                  ),
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
                        style: AppTypography.caption(
                          dialogContext,
                        ).copyWith(fontFamily: 'monospace', height: 1.5),
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
          .read(conversationRuntimeRegistryProvider)
          .sessionForConversation(widget.conversationId)
          .resolveApproval(id, approved);
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      ref
          .read(conversationRuntimeRegistryProvider)
          .sessionForConversation(widget.conversationId)
          .dismissPendingApproval(id);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(error.toString()),
            duration: const Duration(seconds: 10),
          ),
        );
      }
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
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      return raw.length > 8000 ? raw.substring(0, 8000) : raw;
    }
  }

  Future<void> _showAttention(AttentionItem item) async {
    if (!mounted || _dialogOpen) return;
    _dialogOpen = true;
    _activeAttentionId = item.id;
    try {
      final approved = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) {
          final isTaskApproval = item.kind == 'task_approval';
          return AlertDialog(
            title: Text(item.title),
            content: SizedBox(
              width: 560,
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (item.summary.isNotEmpty)
                      Text(
                        item.summary,
                        style: AppTypography.sectionTitle(dialogContext),
                      ),
                    SizedBox(height: AppSpacing.xs),
                    Text(
                      item.reason,
                      style: AppTypography.bodySmall(dialogContext),
                    ),
                    SizedBox(height: AppSpacing.md),
                    Text(
                      '事项类型：${_attentionKindLabel(item.kind)}',
                      style: AppTypography.caption(dialogContext),
                    ),
                  ],
                ),
              ),
            ),
            actions: [
              TextButton(
                onPressed: () => Navigator.pop(dialogContext),
                child: const Text('稍后'),
              ),
              if (item.threadId.isNotEmpty ||
                  item.kind == 'task_failed' ||
                  item.kind == 'task_blocked')
                TextButton(
                  onPressed: () {
                    Navigator.pop(dialogContext);
                    _openAttentionTarget(item);
                  },
                  child: const Text('打开详情'),
                ),
              if (isTaskApproval) ...[
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext, false),
                  child: Text(
                    '拒绝',
                    style: TextStyle(color: dialogContext.error),
                  ),
                ),
                FilledButton(
                  onPressed: () => Navigator.pop(dialogContext, true),
                  child: const Text('批准本次执行'),
                ),
              ],
            ],
          );
        },
      );
      if (approved != null && item.kind == 'task_approval') {
        final approvals = await ref
            .read(extensionTaskServiceProvider)
            .listSourceTaskApprovals();
        final approval = approvals
            .where((candidate) => candidate['id'] == item.approvalId)
            .firstOrNull;
        if (approval == null) throw StateError('设备审批已不存在或已处理');
        await ref
            .read(extensionTaskServiceProvider)
            .decideSourceTaskApproval(approval, approved);
      }
      _clearAttentionDeepLink(item.id);
      _dismissedAttentionIds.add(item.id);
      await _loadAttention();
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.toString())));
      }
    } finally {
      _dialogOpen = false;
      _activeAttentionId = '';
      if (mounted) setState(() {});
    }
  }

  String _attentionKindLabel(String kind) {
    return switch (kind) {
      'agent_approval' => '工具执行审批',
      'task_approval' => '设备任务审批',
      'continuity' => '持续事项等待',
      'task_failed' => '任务失败恢复',
      'task_blocked' => '任务等待处理',
      'automation_failed' => '自动化失败',
      'automation_approval' => '自动化确认',
      _ => '需要处理',
    };
  }

  void _openAttentionTarget(AttentionItem item) {
    if (item.workflowId.isNotEmpty) {
      final deviceId = (item.payload['deviceId'] ?? '').toString();
      context.push(
        AppRoutes.workflowEditor(
          item.workflowId,
          location: deviceId.isEmpty ? 'cloud' : 'device',
          deviceId: deviceId,
        ),
      );
      return;
    }
    if (item.threadId.isNotEmpty) {
      context.push('/continuity/${Uri.encodeComponent(item.threadId)}');
      return;
    }
    context.push('/agent');
  }

  void _clearAttentionDeepLink(String itemId) {
    if (!mounted) return;
    final state = GoRouterState.of(context);
    if (state.uri.queryParameters['attentionId'] != itemId) return;
    final query = Map<String, String>.from(state.uri.queryParameters)
      ..remove('attentionId');
    context.go(Uri(path: state.uri.path, queryParameters: query).toString());
  }

  @override
  Widget build(BuildContext context) {
    final conversationId = widget.conversationId.trim();
    final registry = ref.watch(conversationRuntimeRegistryProvider);
    final runtime = conversationId.isEmpty
        ? null
        : registry.sessionForConversation(conversationId);
    final approvals =
        (runtime?.pendingApprovals ?? const <Map<String, dynamic>>[])
            .where(
              (item) =>
                  conversationId.isNotEmpty &&
                  (item['conversationId'] ?? '').toString().trim() ==
                      conversationId,
            )
            .toList(growable: false);
    final currentAttentionId =
        GoRouterState.of(context).uri.queryParameters['attentionId'] ?? '';
    final linkedAttentionItem = currentAttentionId.isEmpty
        ? null
        : _attentionItems
              .where(
                (item) =>
                    item.id == currentAttentionId &&
                    item.conversationId == conversationId,
              )
              .firstOrNull;
    final attention =
        _attentionItems
            .where(
              (item) =>
                  item.requiresAction &&
                  item.kind != 'agent_approval' &&
                  item.conversationId == conversationId &&
                  (!_dismissedAttentionIds.contains(item.id) ||
                      item.id == currentAttentionId),
            )
            .toList(growable: false)
          ..sort((left, right) => right.priority.compareTo(left.priority));
    final linkedAttention = currentAttentionId.isEmpty
        ? null
        : attention.where((item) => item.id == currentAttentionId).firstOrNull;
    final linkedApproval = linkedAttentionItem?.kind == 'agent_approval'
        ? approvals
              .where(
                (item) =>
                    (item['id'] ?? '').toString() ==
                    (linkedAttentionItem?.approvalId ?? ''),
              )
              .firstOrNull
        : null;
    if (_dialogOpen &&
        _activeApprovalId.isNotEmpty &&
        !approvals.any(
          (item) => (item['id'] ?? '').toString() == _activeApprovalId,
        )) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted || !_dialogOpen) return;
        Navigator.of(context, rootNavigator: true).maybePop();
      });
    } else if (_dialogOpen &&
        _activeAttentionId.isNotEmpty &&
        !attention.any((item) => item.id == _activeAttentionId)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (!mounted || !_dialogOpen) return;
        Navigator.of(context, rootNavigator: true).maybePop();
      });
    } else if (!_dialogOpen && linkedAttention != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) unawaited(_showAttention(linkedAttention));
      });
    } else if (!_dialogOpen && linkedApproval != null) {
      final item = Map<String, dynamic>.from(linkedApproval);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _showApproval(item);
      });
    } else if (!_dialogOpen && approvals.isNotEmpty) {
      final item = Map<String, dynamic>.from(approvals.first);
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _showApproval(item);
      });
    } else if (!_dialogOpen && attention.isNotEmpty) {
      final item = attention.first;
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) unawaited(_showAttention(item));
      });
    }
    return const SizedBox.shrink();
  }
}
