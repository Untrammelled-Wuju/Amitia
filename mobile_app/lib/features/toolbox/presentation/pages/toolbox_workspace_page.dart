import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/widgets/amitia_popup_menu.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/workspace_service.dart';

class ToolboxWorkspacePage extends ConsumerStatefulWidget {
  final bool embedded;
  final void Function(String resourceUri, String workspaceUri, String title)? onOpenDiff;
  const ToolboxWorkspacePage({super.key, this.embedded = false, this.onOpenDiff});

  @override
  ConsumerState<ToolboxWorkspacePage> createState() => _ToolboxWorkspacePageState();
}

class _ToolboxWorkspacePageState extends ConsumerState<ToolboxWorkspacePage> {
  List<WorkspaceMountDto> _workspaces = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(workspaceServiceProvider);
      final mounts = await svc.list();
      if (mounted) {
        setState(() { _workspaces = mounts; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _kindLabel(String kind) {
    switch (kind) {
      case 'saf':
        return 'Android 目录';
      case 'isolated':
        return '隔离工作区';
      case 'local':
      default:
        return '本地';
    }
  }

  String _statusLabel(String status) {
    switch (status) {
      case 'ready':
        return '可用';
      case 'read_only':
        return '只读';
      case 'permission_revoked':
        return '权限已撤销';
      case 'missing':
        return '不存在';
      case 'unavailable':
        return '不可用';
      default:
        return status;
    }
  }

  IconData _statusIcon(String status) {
    switch (status) {
      case 'ready':
        return Icons.check_circle_outline;
      case 'read_only':
        return Icons.lock_outline;
      case 'permission_revoked':
        return Icons.block_outlined;
      default:
        return Icons.error_outline;
    }
  }

  Color _statusColor(String status) {
    switch (status) {
      case 'ready':
        return AppColors.success;
      case 'read_only':
        return AppColors.warning;
      default:
        return AppColors.error;
    }
  }

  IconData _iconForKind(String kind) {
    switch (kind) {
      case 'saf':
        return Icons.folder_shared_outlined;
      case 'local':
      default:
        return Icons.folder_outlined;
    }
  }



  Future<void> _createGitWorktree(
    WorkspaceMountDto workspace, {
    required Future<void> Function() refresh,
  }) async {
    final nameController = TextEditingController();
    final branchController = TextEditingController();
    final refController = TextEditingController(text: 'HEAD');
    final taskController = TextEditingController();
    final conversationController = TextEditingController();
    try {
      final confirm = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('创建 Git Worktree'),
          content: SingleChildScrollView(
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              TextField(controller: nameController, maxLength: 100, decoration: const InputDecoration(labelText: '显示名称（可选）')),
              TextField(controller: branchController, maxLength: 80, decoration: const InputDecoration(labelText: '新分支（可选）', helperText: '留空则创建 detached HEAD 工作树')),
              TextField(controller: refController, maxLength: 160, decoration: const InputDecoration(labelText: '起始提交')),
              TextField(controller: taskController, maxLength: 128, decoration: const InputDecoration(labelText: '关联任务 ID（可选）')),
              TextField(controller: conversationController, maxLength: 128, decoration: const InputDecoration(labelText: '对应对话 ID（绑定时必填）')),
            ]),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('创建')),
          ],
        ),
      );
      if (confirm != true || !mounted) return;
      if (taskController.text.trim().isEmpty != conversationController.text.trim().isEmpty) {
        throw StateError('任务 ID 和对话 ID 必须同时填写');
      }
      await ref.read(workspaceServiceProvider).createGitWorktree(
        workspace.rootUri,
        name: nameController.text.trim(),
        branch: branchController.text.trim(),
        refName: refController.text.trim().isEmpty ? 'HEAD' : refController.text.trim(),
        taskId: taskController.text.trim(),
        conversationId: conversationController.text.trim(),
      );
      await refresh();
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已创建并注册 Git Worktree')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('创建失败：$e')));
    } finally {
      nameController.dispose(); branchController.dispose(); refController.dispose(); taskController.dispose(); conversationController.dispose();
    }
  }

  Future<void> _removeGitWorktree(
    WorkspaceMountDto workspace,
    String id, {
    required Future<void> Function() refresh,
  }) async {
    if (id.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('安全删除 Worktree'),
        content: const Text('仅能删除 U-Ai 创建的 Worktree；如果有未提交变更或锁定，后端会拒绝删除。不会执行强制删除。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('删除')),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref.read(workspaceServiceProvider).removeGitWorktree(workspace.rootUri, id);
      await refresh();
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Worktree 已删除')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除被拒绝：$e')));
    }
  }

  Future<void> _reviewGitWorktree(WorkspaceMountDto workspace, Map<String, dynamic> tree) async {
    final binding = tree['taskBinding'];
    if (binding is! Map || tree['id'] == null) return;
    try {
      final result = await ref.read(workspaceServiceProvider).reviewGitWorktree(
        workspace.rootUri,
        tree['id'].toString(),
        (binding['taskId'] ?? '').toString(),
        (binding['conversationId'] ?? '').toString(),
        (tree['head'] ?? '').toString(),
      );
      if (!mounted) return;
      final diff = result['diff'] is Map ? Map<String, dynamic>.from(result['diff'] as Map) : <String, dynamic>{};
      final stats = diff['stats'] is Map ? Map<String, dynamic>.from(diff['stats'] as Map) : <String, dynamic>{};
      final files = (diff['files'] as List<dynamic>? ?? const []).whereType<Map>();
      final lines = files.map((file) => (file['patch'] ?? '[文本 Diff 不可用]').toString()).join('\n').split('\n');
      final preview = lines.take(1200).join('\n');
      await showDialog<void>(context: context, builder: (dialogContext) => AlertDialog(
        title: Text('任务 ${binding['taskId']} · Worktree 审查'),
        content: SizedBox(width: double.maxFinite, child: SingleChildScrollView(child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Text('${stats['filesChanged'] ?? 0} 个文件 · +${stats['insertions'] ?? 0} / -${stats['deletions'] ?? 0}'),
            if (result['reviewIncomplete'] == true) const Text('注意：未跟踪文件、旧绑定缺少起点或截断内容未完整覆盖，不可直接自动合并。'),
            if ((result['conflictedCount'] as num? ?? 0) > 0) Text('存在 ${result['conflictedCount']} 个冲突'),
            const SizedBox(height: 12),
            SelectableText(preview.isEmpty ? '没有可展示的文本差异' : preview),
          ],
        ))),
        actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭'))],
      ));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('审查失败：$e')));
    }
  }

  Future<void> _checkpointGitWorktree(WorkspaceMountDto workspace, Map<String, dynamic> tree) async {
    final binding = tree['taskBinding'];
    final id = (tree['id'] ?? '').toString();
    if (binding is! Map || id.isEmpty) return;
    final taskId = (binding['taskId'] ?? '').toString();
    final conversationId = (binding['conversationId'] ?? '').toString();
    try {
      final service = ref.read(workspaceServiceProvider);
      Map<String, dynamic> checkpoint = await service.prepareGitWorktreeCheckpoint(workspace.rootUri, id, taskId, conversationId);
      if (!mounted) return;
      await showDialog<void>(context: context, builder: (dialogContext) => StatefulBuilder(builder: (dialogContext, update) {
        final tests = (checkpoint['tests'] as List<dynamic>? ?? const []).whereType<Map>().toList();
        final approved = checkpoint['approvedAt'] != null;
        final passed = tests.isNotEmpty && tests.last['passed'] == true;
        return AlertDialog(
          title: Text('任务 $taskId · 提交验收'),
          content: SizedBox(width: double.maxFinite, child: SingleChildScrollView(child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Text('固定提交：${checkpoint['head']}'),
              Text(approved ? '已批准 · ${checkpoint['approvedBy'] ?? ''}' : '尚未批准'),
              const Text('执行测试前请确认仓库代码可信。测试在本机 Runtime 执行，最长 90 秒。'),
              ...tests.map((test) => Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Text('${test['profile']} · ${test['passed'] == true ? '通过' : '失败'} · exit ${test['exitCode']}'),
                SelectableText((test['logExcerpt'] ?? '').toString(), maxLines: 20),
              ])),
            ],
          ))),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('关闭')),
            if (!approved) TextButton(onPressed: () async {
              final allowed = await showDialog<bool>(context: dialogContext, builder: (confirmContext) => AlertDialog(
                title: const Text('运行 Go 验证'),
                content: const Text('将实际执行 go test ./... 并记录退出码与日志。确认工作树代码可信后继续。'),
                actions: [TextButton(onPressed: () => Navigator.pop(confirmContext, false), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(confirmContext, true), child: const Text('运行'))],
              ));
              if (allowed != true) return;
              try {
                final updated = await service.testGitWorktreeCheckpoint(workspace.rootUri, id, taskId, conversationId, (checkpoint['revision'] ?? '').toString(), 'go');
                if (dialogContext.mounted) update(() => checkpoint = updated);
              } catch (e) {
                if (dialogContext.mounted) ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('验证失败：$e')));
              }
            }, child: const Text('运行 Go 测试')),
            if (!approved && passed) FilledButton(onPressed: () async {
              final allowed = await showDialog<bool>(context: dialogContext, builder: (confirmContext) => AlertDialog(
                title: const Text('批准固定提交'),
                content: const Text('确认人工批准这一提交及测试证据？不会自动合并代码。'),
                actions: [TextButton(onPressed: () => Navigator.pop(confirmContext, false), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(confirmContext, true), child: const Text('批准'))],
              ));
              if (allowed != true) return;
              try {
                final updated = await service.approveGitWorktreeCheckpoint(workspace.rootUri, id, taskId, conversationId, (checkpoint['revision'] ?? '').toString());
                if (dialogContext.mounted) update(() => checkpoint = updated);
              } catch (e) {
                if (dialogContext.mounted) ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('批准失败：$e')));
              }
            }, child: const Text('人工批准')),
            if (approved) TextButton(onPressed: () async {
              try {
                final audit = await service.readGitWorktreeMergeAudit(workspace.rootUri, id, taskId, conversationId);
                if (dialogContext.mounted) {
                  ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('合并审计：${audit['result'] ?? '未知'} · ${audit['appliedHead'] ?? '目标提交待核对'}')));
                }
              } catch (e) {
                if (dialogContext.mounted) ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('读取合并审计失败：$e')));
              }
            }, child: const Text('查看合并记录')),
            if (approved) FilledButton(onPressed: () async {
              try {
                final plan = await service.previewGitWorktreeMerge(workspace.rootUri, id, taskId, conversationId);
                if (!dialogContext.mounted) return;
                final branch = (plan['targetBranch'] ?? '').toString();
                final source = (plan['sourceHead'] ?? '').toString();
                final target = (plan['targetHead'] ?? '').toString();
                final planId = (plan['planId'] ?? '').toString();
                if (branch.isEmpty || planId.isEmpty) throw StateError('预检结果不完整');
                final confirmed = await showDialog<bool>(context: dialogContext, builder: (confirmContext) => AlertDialog(
                  title: const Text('确认快进合并'),
                  content: SingleChildScrollView(child: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text('目标分支：$branch'),
                    Text('目标提交：$target'),
                    Text('来源提交：$source'),
                    Text('变更文件：${plan['filesChanged']}'),
                    const Text('本地目标工作树必须干净，且仅允许 fast-forward。此操作将修改目标分支和文件，不会推送远端。请确认已检查目标仓库。'),
                  ])),
                  actions: [
                    TextButton(onPressed: () => Navigator.pop(confirmContext, false), child: const Text('取消')),
                    FilledButton(onPressed: () => Navigator.pop(confirmContext, true), child: const Text('确认本地合并')),
                  ],
                ));
                if (confirmed != true) return;
                final result = await service.applyGitWorktreeMerge(workspace.rootUri, id, taskId, conversationId, planId);
                if (dialogContext.mounted) {
                  ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('合并审计：${result['result'] ?? '未知'}')));
                  Navigator.pop(dialogContext);
                }
              } catch (e) {
                if (dialogContext.mounted) ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('合并未确认成功：$e。请先核对审计与目标 HEAD，勿盲目重试。')));
              }
            }, child: const Text('预检并快进合并')),
          ],
        );
      }));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('无法创建检查点：$e')));
    }
  }

  Future<void> _releaseGitWorktree(WorkspaceMountDto workspace, String id, Map<String, dynamic> binding, {required Future<void> Function() refresh}) async {
    final taskID = (binding['taskId'] ?? '').toString();
    final conversationID = (binding['conversationId'] ?? '').toString();
    if (id.isEmpty || taskID.isEmpty || conversationID.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('解除任务绑定'),
        content: Text('解除任务 $taskID 的工作树绑定？不会删除工作树或 Git 分支。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('解绑')),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref.read(workspaceServiceProvider).releaseGitWorktree(workspace.rootUri, id, taskID, conversationID);
      await refresh();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('任务绑定已解除')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('解绑失败：$e')));
    }
  }

  Future<void> _showCreateIsolated() async {
    final nameCtrl = TextEditingController();
    final remoteCtrl = TextEditingController();
    final refCtrl = TextEditingController();
    final lifetimeCtrl = TextEditingController();
    String mode = 'snapshot';
    String sourceUri = _workspaces.where((item) => item.kind != 'isolated' && item.available).firstOrNull?.rootUri ?? '';
    bool readOnly = false;
    bool saving = false;
    String? localError;

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.lg,
              0,
              AppSpacing.lg,
              MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
            ),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text('创建隔离工作区', style: AppTypography.sectionTitle(context)),
                  SizedBox(height: AppSpacing.lg),
                  TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: '名称', border: OutlineInputBorder())),
                  SizedBox(height: AppSpacing.md),
                  DropdownButtonFormField<String>(
                    value: mode,
                    decoration: const InputDecoration(labelText: '创建方式', border: OutlineInputBorder()),
                    items: const [
                      DropdownMenuItem(value: 'snapshot', child: Text('当前工作区快照')),
                      DropdownMenuItem(value: 'git_clone', child: Text('Git Clone')),
                    ],
                    onChanged: saving ? null : (value) => setSheetState(() => mode = value ?? 'snapshot'),
                  ),
                  SizedBox(height: AppSpacing.md),
                  if (mode == 'snapshot')
                    DropdownButtonFormField<String>(
                      value: _workspaces.any((item) => item.rootUri == sourceUri) ? sourceUri : null,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: '源工作区', border: OutlineInputBorder()),
                      items: _workspaces
                          .where((item) => item.kind != 'isolated' && item.available)
                          .map((item) => DropdownMenuItem(value: item.rootUri, child: Text(item.name, overflow: TextOverflow.ellipsis)))
                          .toList(growable: false),
                      onChanged: saving ? null : (value) => setSheetState(() => sourceUri = value ?? ''),
                    )
                  else ...[
                    TextField(controller: remoteCtrl, decoration: const InputDecoration(labelText: 'Git Remote URL', border: OutlineInputBorder())),
                    SizedBox(height: AppSpacing.md),
                    TextField(controller: refCtrl, decoration: const InputDecoration(labelText: 'Ref / Branch（可选）', border: OutlineInputBorder())),
                  ],
                  SizedBox(height: AppSpacing.md),
                  TextField(controller: lifetimeCtrl, decoration: const InputDecoration(labelText: 'Lifetime（可选，例如 24h）', border: OutlineInputBorder())),
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('只读'),
                    value: readOnly,
                    onChanged: saving ? null : (value) => setSheetState(() => readOnly = value),
                  ),
                  if (localError != null) Text(localError!, style: AppTypography.caption(context).copyWith(color: context.error)),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: saving ? '创建中...' : '创建',
                    isFullWidth: true,
                    onPressed: saving
                        ? null
                        : () async {
                            final name = nameCtrl.text.trim();
                            if (name.isEmpty) {
                              setSheetState(() => localError = '名称不能为空');
                              return;
                            }
                            if (mode == 'snapshot' && sourceUri.isEmpty) {
                              setSheetState(() => localError = '请选择源工作区');
                              return;
                            }
                            if (mode == 'git_clone' && remoteCtrl.text.trim().isEmpty) {
                              setSheetState(() => localError = 'Git Remote URL 不能为空');
                              return;
                            }
                            setSheetState(() {
                              saving = true;
                              localError = null;
                            });
                            try {
                              await ref.read(workspaceServiceProvider).createIsolated(
                                    name: name,
                                    mode: mode,
                                    sourceWorkspaceUri: mode == 'snapshot' ? sourceUri : '',
                                    remoteUrl: mode == 'git_clone' ? remoteCtrl.text.trim() : '',
                                    refName: refCtrl.text.trim(),
                                    readOnly: readOnly,
                                    lifetime: lifetimeCtrl.text.trim(),
                                  );
                              if (!sheetContext.mounted) return;
                              Navigator.pop(sheetContext);
                              await _load();
                              if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('隔离工作区已创建')));
                            } catch (e) {
                              setSheetState(() {
                                saving = false;
                                localError = e.toString();
                              });
                            }
                          },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );

    nameCtrl.dispose();
    remoteCtrl.dispose();
    refCtrl.dispose();
    lifetimeCtrl.dispose();
  }

  Future<void> _deleteIsolated(WorkspaceMountDto workspace) async {
    final force = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除隔离工作区'),
        content: Text('确定删除「${workspace.name}」及其隔离文件吗？如果工作区存在未提交修改，普通删除会被后端拒绝。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, null), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('删除')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('强制删除', style: TextStyle(color: context.error))),
        ],
      ),
    );
    if (force == null) return;
    try {
      await ref.read(workspaceServiceProvider).deleteIsolated(workspace.rootUri, force: force);
      await _load();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('隔离工作区已删除')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败：$e'), backgroundColor: context.error));
    }
  }

  Future<void> _showIsolatedInfo(WorkspaceMountDto workspace) async {
    try {
      final info = await ref.read(workspaceServiceProvider).isolatedInfo(workspace.rootUri);
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: Text('隔离工作区 · ${workspace.name}'),
          content: SingleChildScrollView(
            child: SelectableText(info.entries.map((e) => '${e.key}: ${e.value}').join('\n')),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取失败：$e')));
    }
  }

  Future<void> _openGitPanel(WorkspaceMountDto workspace) async {
    final svc = ref.read(workspaceServiceProvider);
    Map<String, dynamic>? status;
    Map<String, dynamic>? branches;
    Map<String, dynamic>? remotes;
    Map<String, dynamic>? log;
    String? error;
    Map<String, dynamic>? worktrees;
    bool loading = true;

    Future<void> refresh(StateSetter setSheetState) async {
      setSheetState(() { loading = true; error = null; });
      try {
        final values = await Future.wait([
          svc.gitStatus(workspace.rootUri),
          svc.gitBranches(workspace.rootUri),
          svc.gitRemotes(workspace.rootUri),
          svc.gitLog(workspace.rootUri, limit: 30),
          svc.gitWorktrees(workspace.rootUri),
        ]);
        setSheetState(() {
          status = values[0];
          branches = values[1];
          remotes = values[2];
          log = values[3];
          worktrees = values[4];
          loading = false;
        });
      } catch (e) {
        setSheetState(() { error = e.toString(); loading = false; });
      }
    }

    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) {
          if (loading && status == null && error == null) {
            WidgetsBinding.instance.addPostFrameCallback((_) => refresh(setSheetState));
          }
          final entries = (status?['entries'] as List<dynamic>? ?? const []).whereType<Map>().toList();
          final branchItems = (branches?['branches'] as List<dynamic>? ?? const []).whereType<Map>().toList();
          final remoteItems = (remotes?['remotes'] as List<dynamic>? ?? remotes?['items'] as List<dynamic>? ?? const []).whereType<Map>().toList();
          final commits = (log?['entries'] as List<dynamic>? ?? const []).whereType<Map>().toList();
          final worktreeItems = (worktrees?['entries'] as List<dynamic>? ?? const []).whereType<Map>().toList();
          return SafeArea(
            child: SizedBox(
              height: MediaQuery.sizeOf(sheetContext).height * .9,
              child: Column(
                children: [
                  Padding(
                    padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.sm),
                    child: Row(
                      children: [
                        Expanded(child: Text('Git · ${workspace.name}', style: AppTypography.pageTitle(sheetContext))),
                        IconButton(onPressed: () => refresh(setSheetState), icon: const Icon(Icons.refresh)),
                        IconButton(onPressed: () => Navigator.pop(sheetContext), icon: const Icon(Icons.close)),
                      ],
                    ),
                  ),
                  if (loading) const LinearProgressIndicator(minHeight: 2),
                  if (error != null)
                    Padding(padding: EdgeInsets.all(AppSpacing.md), child: Text(error!, style: TextStyle(color: AppColors.error))),
                  Expanded(
                    child: ListView(
                      padding: EdgeInsets.all(AppSpacing.pagePadding),
                      children: [
                        if (status != null) ...[
                          Wrap(
                            spacing: 8,
                            runSpacing: 8,
                            children: [
                              AmitiaStatusBadge(label: (status!['branch'] ?? 'detached').toString(), type: BadgeType.info),
                              AmitiaStatusBadge(label: status!['clean'] == true ? '工作区干净' : '${entries.length} 项变更', type: status!['clean'] == true ? BadgeType.success : BadgeType.warning),
                              AmitiaStatusBadge(label: '↑${status!['ahead'] ?? 0} ↓${status!['behind'] ?? 0}', type: BadgeType.neutral),
                            ],
                          ),
                          SizedBox(height: AppSpacing.md),
                          Row(
                            children: [
                              Expanded(child: AmitiaButton(label: '暂存全部', isSecondary: true, icon: Icons.add_task, onPressed: workspace.readOnly ? null : () async { await svc.gitAdd(workspace.rootUri, all: true, force: true); await refresh(setSheetState); })),
                              SizedBox(width: AppSpacing.sm),
                              Expanded(child: AmitiaButton(label: '提交', icon: Icons.commit, onPressed: workspace.readOnly ? null : () => _gitCommitDialog(workspace, refresh: () => refresh(setSheetState)))),
                            ],
                          ),
                          SizedBox(height: AppSpacing.sm),
                          Row(
                            children: [
                              Expanded(child: AmitiaButton(label: 'Fetch', isSecondary: true, onPressed: () async { await svc.gitFetch(workspace.rootUri); await refresh(setSheetState); })),
                              SizedBox(width: AppSpacing.sm),
                              Expanded(child: AmitiaButton(label: 'Pull', isSecondary: true, onPressed: workspace.readOnly ? null : () async { await svc.gitPull(workspace.rootUri); await refresh(setSheetState); })),
                              SizedBox(width: AppSpacing.sm),
                              Expanded(child: AmitiaButton(label: 'Push', isSecondary: true, onPressed: workspace.readOnly ? null : () => _gitPushDialog(workspace, status ?? {}, remoteItems, refresh: () => refresh(setSheetState)))),
                            ],
                          ),
                          SizedBox(height: AppSpacing.lg),
                          Row(children: [
                            Expanded(child: Text('Git Worktree', style: AppTypography.cardTitle(sheetContext))),
                            AmitiaButton(label: '创建', isSecondary: true, icon: Icons.account_tree_outlined,
                              onPressed: workspace.readOnly ? null : () => _createGitWorktree(workspace, refresh: () => refresh(setSheetState))),
                          ]),
                          Text('每个 Worktree 为独立的本机工作区，不支持强制删除。', style: AppTypography.caption(sheetContext)),
                          ...worktreeItems.map((raw) {
                            final tree = Map<String, dynamic>.from(raw);
                            final managed = tree['managed'] == true;
                            final id = (tree['id'] ?? '').toString();
                            final binding = tree['taskBinding'] is Map ? Map<String, dynamic>.from(tree['taskBinding'] as Map) : null;
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              dense: true,
                              leading: Icon(managed ? Icons.account_tree : Icons.source_outlined),
                              title: Text((tree['branch'] ?? '').toString().isEmpty ? 'detached HEAD' : tree['branch'].toString()),
                              subtitle: Text('${tree['managed'] == true ? 'U-Ai 管理' : '外部 Worktree'} · ${binding != null ? '任务 ${binding['taskId']} · ' : ''}${tree['path'] ?? ''}', maxLines: 2, overflow: TextOverflow.ellipsis),
                              trailing: managed && id.isNotEmpty
                                  ? Wrap(children: [
                                      if (binding != null) IconButton(
                                        icon: const Icon(Icons.rate_review_outlined),
                                        tooltip: '只读审查 Git Diff',
                                        onPressed: () => _reviewGitWorktree(workspace, tree),
                                      ),
                                      if (binding != null && !workspace.readOnly) IconButton(
                                        tooltip: '提交验收与测试',
                                        icon: const Icon(Icons.fact_check_outlined),
                                        onPressed: () => _checkpointGitWorktree(workspace, tree),
                                      ),
                                      if (!workspace.readOnly && tree['locked'] != true && tree['prunable'] != true)
                                        IconButton(icon: Icon(binding == null ? Icons.delete_outline : Icons.link_off), tooltip: binding == null ? '安全删除' : '解除任务绑定',
                                          onPressed: () => binding == null ? _removeGitWorktree(workspace, id, refresh: () => refresh(setSheetState)) : _releaseGitWorktree(workspace, id, binding, refresh: () => refresh(setSheetState))),
                                    ])
                                  : null,
                            );
                          }),
                          SizedBox(height: AppSpacing.lg),
                          Text('变更', style: AppTypography.cardTitle(sheetContext)),
                          if (entries.isEmpty)
                            Padding(padding: EdgeInsets.symmetric(vertical: AppSpacing.md), child: Text('没有未提交变更', style: AppTypography.caption(sheetContext)))
                          else
                            ...entries.map((raw) {
                              final entry = Map<String, dynamic>.from(raw);
                              final uri = (entry['uri'] ?? '').toString();
                              return ListTile(
                                contentPadding: EdgeInsets.zero,
                                dense: true,
                                title: Text(uri, maxLines: 1, overflow: TextOverflow.ellipsis),
                                subtitle: Text('staging=${entry['staging'] ?? '-'} · worktree=${entry['worktree'] ?? '-'}${entry['conflict'] == true ? ' · 冲突' : ''}'),
                                trailing: AmitiaPopupMenuButton<String>(
                                  onSelected: (value) async {
                                    if (value == 'diff') {
                                      if (widget.onOpenDiff != null) {
                                        Navigator.of(sheetContext).pop();
                                        widget.onOpenDiff!(uri, workspace.rootUri, uri.split('/').last);
                                        return;
                                      }
                                      final diff = await svc.gitDiff(workspace.rootUri, paths: [uri]);
                                      if (mounted) await _showRawJson('Diff · $uri', diff);
                                    } else if (value == 'stage') {
                                      await svc.gitAdd(workspace.rootUri, paths: [uri]);
                                      await refresh(setSheetState);
                                    } else if (value == 'restore') {
                                      await svc.gitRestore(workspace.rootUri, paths: [uri]);
                                      await refresh(setSheetState);
                                    }
                                  },
                                  itemBuilder: (_) => [
                                    const PopupMenuItem(value: 'diff', child: Text('查看 Diff')),
                                    if (!workspace.readOnly) const PopupMenuItem(value: 'stage', child: Text('暂存')),
                                    if (!workspace.readOnly) const PopupMenuItem(value: 'restore', child: Text('还原工作区文件')),
                                  ],
                                ),
                              );
                            }),
                          SizedBox(height: AppSpacing.lg),
                          Row(
                            children: [
                              Expanded(child: Text('分支', style: AppTypography.cardTitle(sheetContext))),
                              AmitiaButton(
                                label: '新建分支',
                                isSecondary: true,
                                icon: Icons.call_split_outlined,
                                onPressed: workspace.readOnly
                                    ? null
                                    : () => _gitCreateBranchDialog(
                                          workspace,
                                          fromRef: (status!['branch'] ?? 'HEAD').toString(),
                                          refresh: () => refresh(setSheetState),
                                        ),
                              ),
                            ],
                          ),
                          ...branchItems.map((raw) {
                            final branch = Map<String, dynamic>.from(raw);
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              dense: true,
                              leading: Icon(branch['current'] == true ? Icons.radio_button_checked : Icons.radio_button_unchecked, size: 18),
                              title: Text((branch['name'] ?? '').toString()),
                              subtitle: Text((branch['commit'] ?? '').toString(), maxLines: 1, overflow: TextOverflow.ellipsis),
                              onTap: branch['current'] == true || workspace.readOnly ? null : () async { await svc.gitCheckout(workspace.rootUri, (branch['name'] ?? '').toString()); await refresh(setSheetState); },
                            );
                          }),
                          SizedBox(height: AppSpacing.lg),
                          Text('远程', style: AppTypography.cardTitle(sheetContext)),
                          if (remoteItems.isEmpty) Text('暂无远程', style: AppTypography.caption(sheetContext)),
                          ...remoteItems.map((raw) {
                            final remote = Map<String, dynamic>.from(raw);
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              dense: true,
                              title: Text((remote['name'] ?? '').toString()),
                              subtitle: Text((remote['fetchUrl'] ?? remote['pushUrl'] ?? '').toString()),
                              trailing: remote['hasCredential'] == true ? const Icon(Icons.key, size: 18) : null,
                            );
                          }),
                          SizedBox(height: AppSpacing.lg),
                          Text('提交历史', style: AppTypography.cardTitle(sheetContext)),
                          ...commits.map((raw) {
                            final commit = Map<String, dynamic>.from(raw);
                            return ListTile(
                              contentPadding: EdgeInsets.zero,
                              dense: true,
                              title: Text((commit['subject'] ?? '').toString(), maxLines: 1, overflow: TextOverflow.ellipsis),
                              subtitle: Text('${commit['authorName'] ?? ''} · ${((commit['hash'] ?? '').toString().length > 8 ? (commit['hash'] ?? '').toString().substring(0, 8) : (commit['hash'] ?? '').toString())}'),
                              onTap: () => _showRawJson('Commit', commit),
                            );
                          }),
                        ],
                      ],
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Future<void> _gitCreateBranchDialog(
    WorkspaceMountDto workspace, {
    required String fromRef,
    required Future<void> Function() refresh,
  }) async {
    final nameController = TextEditingController();
    final fromController = TextEditingController(text: fromRef.isEmpty ? 'HEAD' : fromRef);
    final create = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('新建 Git 分支'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: nameController,
              autofocus: true,
              decoration: const InputDecoration(labelText: '分支名称'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: fromController,
              decoration: const InputDecoration(labelText: '起点', hintText: 'HEAD / 分支 / Commit'),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('创建并切换')),
        ],
      ),
    );
    final branchName = nameController.text.trim();
    final branchFrom = fromController.text.trim().isEmpty ? 'HEAD' : fromController.text.trim();
    nameController.dispose();
    fromController.dispose();
    if (create != true || branchName.isEmpty) return;
    try {
      await ref.read(workspaceServiceProvider).gitCheckout(
            workspace.rootUri,
            branchName,
            create: true,
            fromRef: branchFrom,
          );
      await refresh();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已创建并切换到 $branchName')));
      }
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('新建分支失败：$error')));
      }
    }
  }

  Future<void> _gitCommitDialog(WorkspaceMountDto workspace, {required Future<void> Function() refresh}) async {
    final controller = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('创建提交'),
        content: TextField(controller: controller, autofocus: true, maxLines: 3, decoration: const InputDecoration(labelText: '提交信息')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('提交')),
        ],
      ),
    );
    if (ok != true || controller.text.trim().isEmpty) return;
    await ref.read(workspaceServiceProvider).gitCommit(workspace.rootUri, controller.text.trim());
    await refresh();
  }

  Future<void> _gitPushDialog(WorkspaceMountDto workspace, Map<String, dynamic> status, List<Map<dynamic, dynamic>> remotes, {required Future<void> Function() refresh}) async {
    final branch = (status['branch'] ?? '').toString();
    final remoteController = TextEditingController(text: remotes.isNotEmpty ? (remotes.first['name'] ?? 'origin').toString() : 'origin');
    final localController = TextEditingController(text: branch.isEmpty ? 'HEAD' : branch);
    final remoteRefController = TextEditingController(text: branch.isEmpty ? 'HEAD' : branch);
    bool upstream = false;
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => StatefulBuilder(builder: (context, setState) => AlertDialog(
        title: const Text('Push'),
        content: SingleChildScrollView(child: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(controller: remoteController, decoration: const InputDecoration(labelText: 'Remote')),
          TextField(controller: localController, decoration: const InputDecoration(labelText: 'Local ref')),
          TextField(controller: remoteRefController, decoration: const InputDecoration(labelText: 'Remote ref')),
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('设置 upstream'), value: upstream, onChanged: (v) => setState(() => upstream = v)),
        ])),
        actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('Push'))],
      )),
    );
    if (ok != true) return;
    await ref.read(workspaceServiceProvider).gitPush(workspace.rootUri, remote: remoteController.text.trim(), localRef: localController.text.trim(), remoteRef: remoteRefController.text.trim(), setUpstream: upstream);
    await refresh();
  }

  Future<void> _showRawJson(String title, Map<String, dynamic> data) async {
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: SizedBox(width: 640, child: SingleChildScrollView(child: SelectableText(data.entries.map((e) => '${e.key}: ${e.value}').join('\n')))),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '工作区',
        showBackButton: !widget.embedded,
        fallbackRoute: AppRoutes.settingsMaintenanceCategory,
        actions: [
          IconButton(onPressed: _loading ? null : _showCreateIsolated, icon: const Icon(Icons.add), tooltip: '创建隔离工作区'),
          IconButton(onPressed: _loading ? null : _load, icon: const Icon(Icons.refresh), tooltip: '刷新'),
        ],
      ),
      body: _loading
          ? const AmitiaLoadingState(message: '正在加载工作区...')
          : _error != null
              ? AmitiaErrorState(message: _error!, onRetry: _load)
              : _workspaces.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.work_outline,
                      title: '暂无工作区',
                      subtitle: '可从桌面/移动端添加工作目录，或创建 Git Clone 隔离工作区',
                      actionText: '创建隔离工作区',
                      onAction: _showCreateIsolated,
                    )
                  : ListView.separated(
                      padding: EdgeInsets.all(AppSpacing.pagePadding),
                      itemCount: _workspaces.length,
                      separatorBuilder: (_, _) => SizedBox(height: AppSpacing.md),
                      itemBuilder: (context, i) {
                        final w = _workspaces[i];
                        final kindLabel = _kindLabel(w.kind);
                        final statusLabel = _statusLabel(w.status);
                        return Container(
                          padding: EdgeInsets.all(AppSpacing.cardPadding),
                          decoration: BoxDecoration(
                            color: context.surfacePrimary,
                            borderRadius: AppRadius.brMedium,
                            border: Border.all(color: context.borderPrimary, width: 0.5),
                          ),
                          child: Column(
                            children: [
                              Row(
                                children: [
                                  Container(
                                    width: 44,
                                    height: 44,
                                    decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                                    child: Icon(_iconForKind(w.kind), size: 22, color: context.accentPrimary),
                                  ),
                                  const SizedBox(width: 12),
                                  Expanded(
                                    child: Column(
                                      crossAxisAlignment: CrossAxisAlignment.start,
                                      children: [
                                        Text(w.name, style: AppTypography.cardTitle(context)),
                                        const SizedBox(height: 2),
                                        Text('$kindLabel · $statusLabel', style: AppTypography.caption(context)),
                                        if (w.readOnly) Text('只读', style: AppTypography.caption(context).copyWith(color: AppColors.warning)),
                                      ],
                                    ),
                                  ),
                                  Icon(_statusIcon(w.status), size: 20, color: _statusColor(w.status)),
                                ],
                              ),
                              SizedBox(height: AppSpacing.md),
                              Wrap(
                                spacing: AppSpacing.sm,
                                runSpacing: AppSpacing.sm,
                                children: [
                                  AmitiaButton(
                                    label: 'Git 管理',
                                    isSecondary: true,
                                    icon: Icons.account_tree_outlined,
                                    onPressed: w.available ? () => _openGitPanel(w) : null,
                                  ),
                                  if (w.kind == 'isolated') ...[
                                    AmitiaButton(
                                      label: '隔离信息',
                                      isSecondary: true,
                                      icon: Icons.info_outline,
                                      onPressed: () => _showIsolatedInfo(w),
                                    ),
                                    AmitiaButton(
                                      label: '删除隔离区',
                                      isDestructive: true,
                                      icon: Icons.delete_outline,
                                      onPressed: () => _deleteIsolated(w),
                                    ),
                                  ],
                                ],
                              ),
                            ],
                          ),
                        );
                      },
                    ),
    );
  }

}


extension _WorkspaceFirstOrNull<T> on Iterable<T> {
  T? get firstOrNull {
    final iterator = this.iterator;
    if (!iterator.moveNext()) return null;
    return iterator.current;
  }
}
