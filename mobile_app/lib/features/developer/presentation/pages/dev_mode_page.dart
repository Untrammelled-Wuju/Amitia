import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DevModePage extends ConsumerStatefulWidget {
  const DevModePage({super.key});

  @override
  ConsumerState<DevModePage> createState() => _DevModePageState();
}

class _DevModePageState extends ConsumerState<DevModePage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _workspaces = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final data = await api.get<Map<String, dynamic>>(
        '/api/extensions/dev-mode/workspaces',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      final raw = data?['workspaces'] as List<dynamic>? ?? const [];
      final items = raw.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList();
      if (!mounted) return;
      setState(() {
        _workspaces = items;
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

  Future<void> _mutate(
    String path, {
    Object? data,
    bool delete = false,
    String success = '操作成功',
  }) async {
    try {
      final api = ref.read(backendServiceProvider);
      if (delete) {
        await api.delete(path);
      } else {
        await api.post<Map<String, dynamic>>(
          path,
          data: data ?? const <String, dynamic>{},
          fromJson: (value) => Map<String, dynamic>.from(value as Map),
        );
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(success)));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '开发模式',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, onPressed: _load, tooltip: '刷新'),
          AmitiaIconButton(icon: Icons.add, onPressed: _showRegisterDialog, tooltip: '注册工作区'),
        ],
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) {
      return AmitiaErrorState(
        message: _error!,
        onRetry: _load,
      );
    }
    if (_workspaces.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.developer_mode_outlined,
        title: '暂无开发工作区',
        subtitle: '仅在后端启用 AMITIA_EXTENSION_DEV_MODE 后可使用',
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        itemCount: _workspaces.length,
        itemBuilder: (context, index) => _workspaceCard(context, _workspaces[index]),
      ),
    );
  }

  Widget _workspaceCard(BuildContext context, Map<String, dynamic> workspace) {
    final id = (workspace['workspaceId'] ?? '').toString();
    final extensionId = (workspace['extensionId'] ?? '').toString();
    final status = (workspace['status'] ?? 'unknown').toString();
    final path = (workspace['path'] ?? '').toString();
    final trusted = workspace['devTrust'] == true;
    final watching = workspace['watchEnabled'] == true;
    final autoReload = workspace['autoReload'] == true;
    final revision = (workspace['currentRevision'] ?? '').toString();
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showWorkspaceDetails(workspace),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                  child: Icon(Icons.code_outlined, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(extensionId.isEmpty ? id : extensionId, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(path, maxLines: 1, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(
                  label: status,
                  type: status == 'active' || status == 'ready' ? BadgeType.success : BadgeType.neutral,
                ),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                _chip(context, trusted ? '已信任' : '未信任'),
                _chip(context, watching ? '监听开启' : '监听关闭'),
                _chip(context, autoReload ? '自动重载' : '手动重载'),
                if (revision.isNotEmpty) _chip(context, 'Revision $revision'),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                AmitiaButton(
                  label: '构建',
                  isSecondary: true,
                  icon: Icons.build_outlined,
                  onPressed: () => _mutate('/api/extensions/dev-mode/workspaces/$id/build', success: '构建完成'),
                ),
                AmitiaButton(
                  label: '重载',
                  isSecondary: true,
                  icon: Icons.refresh,
                  onPressed: () => _mutate(
                    '/api/extensions/dev-mode/workspaces/$id/reload',
                    data: const {'reason': 'manual_mobile_reload'},
                    success: '重载完成',
                  ),
                ),
                AmitiaButton(
                  label: trusted ? '撤销信任' : '授予信任',
                  isSecondary: true,
                  icon: trusted ? Icons.lock_open_outlined : Icons.verified_user_outlined,
                  onPressed: () => trusted
                      ? _deleteTrust(id)
                      : _mutate('/api/extensions/dev-mode/workspaces/$id/trust', success: '已授予开发信任'),
                ),
                AmitiaButton(
                  label: watching ? '停止监听' : '开始监听',
                  isSecondary: true,
                  icon: watching ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                  onPressed: () => _mutate(
                    '/api/extensions/dev-mode/workspaces/$id/watch/${watching ? 'stop' : 'start'}',
                    success: watching ? '监听已停止' : '监听已启动',
                  ),
                ),
                AmitiaButton(
                  label: '删除',
                  isDestructive: true,
                  icon: Icons.delete_outline,
                  onPressed: () => _confirmDelete(workspace),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _chip(BuildContext context, String text) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brTag),
      child: Text(text, style: AppTypography.label(context)),
    );
  }

  Future<void> _deleteTrust(String id) async {
    try {
      await ref.read(backendServiceProvider).delete('/api/extensions/dev-mode/workspaces/$id/trust');
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已撤销开发信任')));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _showRegisterDialog() async {
    final extensionController = TextEditingController();
    final pathController = TextEditingController();
    final manifestController = TextEditingController(text: 'manifest.json');
    var watch = true;
    var autoReload = true;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('注册开发工作区'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                TextField(controller: extensionController, decoration: const InputDecoration(labelText: 'Extension ID')),
                TextField(controller: pathController, decoration: const InputDecoration(labelText: '工作区路径')),
                TextField(controller: manifestController, decoration: const InputDecoration(labelText: 'Manifest 路径')),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('监听文件变更'),
                  value: watch,
                  onChanged: (value) => setDialogState(() => watch = value),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('自动重载'),
                  value: autoReload,
                  onChanged: (value) => setDialogState(() => autoReload = value),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('注册')),
          ],
        ),
      ),
    );
    if (confirmed != true) return;
    if (extensionController.text.trim().isEmpty || pathController.text.trim().isEmpty || manifestController.text.trim().isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Extension ID、路径和 Manifest 路径不能为空')));
      return;
    }
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/extensions/dev-mode/workspaces',
        data: {
          'extensionId': extensionController.text.trim(),
          'path': pathController.text.trim(),
          'manifestPath': manifestController.text.trim(),
          'watchEnabled': watch,
          'autoReload': autoReload,
        },
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('开发工作区已注册')));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _showWorkspaceDetails(Map<String, dynamic> workspace) async {
    final id = (workspace['workspaceId'] ?? '').toString();
    try {
      final api = ref.read(backendServiceProvider);
      final detail = await api.get<Map<String, dynamic>>(
        '/api/extensions/dev-mode/workspaces/$id',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      final revisions = await api.get<Map<String, dynamic>>(
        '/api/extensions/dev-mode/workspaces/$id/revisions',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted) return;
      final payload = {
        'workspace': detail ?? workspace,
        'revisions': revisions?['revisions'] ?? const [],
      };
      showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('工作区详情'),
          content: SizedBox(
            width: 520,
            child: SingleChildScrollView(
              child: SelectableText(const JsonEncoder.withIndent('  ').convert(payload)),
            ),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _confirmDelete(Map<String, dynamic> workspace) async {
    final id = (workspace['workspaceId'] ?? '').toString();
    final extensionId = (workspace['extensionId'] ?? id).toString();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('删除开发工作区'),
        content: Text('确定删除 $extensionId？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('删除')),
        ],
      ),
    );
    if (confirmed == true) {
      await _mutate('/api/extensions/dev-mode/workspaces/$id', delete: true, success: '开发工作区已删除');
    }
  }
}
