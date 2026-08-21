import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
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
  const ToolboxWorkspacePage({super.key});

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
    bool loading = true;

    Future<void> refresh(StateSetter setSheetState) async {
      setSheetState(() { loading = true; error = null; });
      try {
        final values = await Future.wait([
          svc.gitStatus(workspace.rootUri),
          svc.gitBranches(workspace.rootUri),
          svc.gitRemotes(workspace.rootUri),
          svc.gitLog(workspace.rootUri, limit: 30),
        ]);
        setSheetState(() {
          status = values[0];
          branches = values[1];
          remotes = values[2];
          log = values[3];
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
          return SafeArea(
            child: SizedBox(
              height: MediaQuery.sizeOf(sheetContext).height * .9,
              child: Column(
                children: [
                  Padding(
                    padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.sm),
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
                              Expanded(child: AmitiaButton(label: '暂存全部', isSecondary: true, icon: Icons.add_task, onPressed: workspace.readOnly ? null : () async { await svc.gitAdd(workspace.rootUri, all: true); await refresh(setSheetState); })),
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
                                trailing: PopupMenuButton<String>(
                                  onSelected: (value) async {
                                    if (value == 'diff') {
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
                          Text('分支', style: AppTypography.cardTitle(sheetContext)),
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
    if (_loading) return const AmitiaLoadingState(message: '正在加载工作区...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_workspaces.isEmpty) {
      return const AmitiaEmptyState(
        icon: Icons.work_outline,
        title: '暂无工作区',
        subtitle: '添加本地目录或 Android 文件夹后会显示在这里',
      );
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '工作区', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView.separated(
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
                Row(
                  children: [
                    Expanded(
                      child: AmitiaButton(
                        label: 'Git 管理',
                        isSecondary: true,
                        icon: Icons.account_tree_outlined,
                        onPressed: w.available ? () => _openGitPanel(w) : null,
                      ),
                    ),
                    if (w.kind == 'isolated') ...[
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: AmitiaButton(
                          label: '隔离信息',
                          isSecondary: true,
                          icon: Icons.info_outline,
                          onPressed: () => _showIsolatedInfo(w),
                        ),
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
