import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

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
      final svc = ref.read(systemServiceProvider);
      final data = await svc.diagnostics();
      if (mounted) {
        if (data != null) {
          final ws = data['workspaces'];
          if (ws is List) {
            _workspaces = ws.map((e) => Map<String, dynamic>.from(e as Map)).toList();
          } else {
            _workspaces = [
              {
                'id': 'default',
                'name': data['app_name'] as String? ?? '工作区',
                'version': data['version'] as String? ?? '1.0.0',
                'status': data['status'] as String? ?? '已注册',
              },
            ];
          }
        }
        setState(() {
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '开发模式',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: _registerWorkspace,
            color: context.accentPrimary,
            tooltip: '注册工作区',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _workspaces.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.developer_mode_outlined,
                title: '暂无开发工作区',
                subtitle: '点击右上角注册新工作区',
                actionText: '注册工作区',
                onAction: _registerWorkspace,
              )
            : ListView.builder(
                padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
                itemCount: _workspaces.length,
                itemBuilder: (context, index) => _buildWorkspaceCard(context, _workspaces[index]),
              ),
      ),
    );
  }

  Widget _buildWorkspaceCard(BuildContext context, Map<String, dynamic> workspace) {
    final name = workspace['name'] as String? ?? '未命名';
    final version = workspace['version'] as String? ?? '0.1.0';
    final status = workspace['status'] as String? ?? '已注册';
    final id = workspace['id'] as String? ?? '';

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showVersionDetailSheet(context, workspace),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    gradient: LinearGradient(colors: [context.accentPrimary, context.accentSecondary]),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: const Icon(Icons.code, size: 24, color: Colors.white),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('v$version', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                _buildStatusBadge(status),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Container(
              padding: const EdgeInsets.all(AppSpacing.sm),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.timeline, size: 16, color: context.textSecondary),
                  const SizedBox(width: 8),
                  Text('版本历史', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
                  const Spacer(),
                  Text('3 个版本', style: AppTypography.label(context)),
                  const SizedBox(width: 8),
                  Icon(Icons.chevron_right, size: 16, color: context.textTertiary),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
            Container(
              padding: const EdgeInsets.all(AppSpacing.sm),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.bolt_outlined, size: 16, color: context.textSecondary),
                  const SizedBox(width: 8),
                  Text('开发会话', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
                  const Spacer(),
                  Text(status == '开发中' ? '活跃' : '无活跃会话', style: AppTypography.label(context)),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: '构建',
                    isSecondary: true,
                    icon: Icons.build_outlined,
                    onPressed: () => _buildWorkspace(context, workspace),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '热重载',
                    icon: Icons.refresh,
                    onPressed: status == '开发中' ? () => _hotReload(context, workspace) : null,
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '删除',
                    isDestructive: true,
                    icon: Icons.delete_outline,
                    onPressed: () => _showDeleteConfirm(context, workspace),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  AmitiaStatusBadge _buildStatusBadge(String status) {
    switch (status) {
      case '已注册':
        return const AmitiaStatusBadge(label: '已注册', type: BadgeType.accent);
      case '开发中':
        return const AmitiaStatusBadge(label: '开发中', type: BadgeType.success);
      case '已构建':
        return const AmitiaStatusBadge(label: '已构建', type: BadgeType.info);
      default:
        return AmitiaStatusBadge(label: status, type: BadgeType.neutral);
    }
  }

  void _registerWorkspace() {
    setState(() {
      _workspaces.add({
        'id': 'dw${_workspaces.length + 1}',
        'name': '新工作区 ${_workspaces.length + 1}',
        'version': '0.1.0',
        'status': '已注册',
      });
    });
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已注册新工作区')),
    );
  }

  void _buildWorkspace(BuildContext context, Map<String, dynamic> workspace) {
    setState(() {
      final idx = _workspaces.indexWhere((w) => w['id'] == workspace['id']);
      if (idx >= 0) {
        _workspaces[idx] = {
          'id': workspace['id'],
          'name': workspace['name'] ?? '未命名',
          'version': workspace['version'] ?? '0.1.0',
          'status': '已构建',
        };
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('正在构建：${workspace['name'] ?? '未命名'}')),
    );
  }

  void _hotReload(BuildContext context, Map<String, dynamic> workspace) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('已触发热重载：${workspace['name'] ?? '未命名'}')),
    );
  }

  void _showVersionDetailSheet(BuildContext context, Map<String, dynamic> workspace) {
    final name = workspace['name'] as String? ?? '未命名';
    final version = workspace['version'] as String? ?? '0.1.0';
    final status = workspace['status'] as String? ?? '已注册';
    final id = workspace['id'] as String? ?? '';

    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (context) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2))),
                ),
                const SizedBox(height: 20),
                Text('版本详情', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 4),
                Text(name, style: AppTypography.caption(context)),
                const SizedBox(height: 16),
                _buildDetailRow(context, '工作区', name),
                _buildDetailRow(context, '当前版本', 'v$version'),
                _buildDetailRow(context, '状态', status),
                _buildDetailRow(context, '工作区 ID', id),
                const SizedBox(height: 16),
                Text('版本历史', style: AppTypography.cardTitle(context).copyWith(fontSize: 14)),
                const SizedBox(height: AppSpacing.sm),
                ..._buildVersionHistory(context, version),
                const SizedBox(height: 20),
                AmitiaButton(
                  label: '关闭',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  List<Widget> _buildVersionHistory(BuildContext context, String version) {
    final versions = [
      {'version': version, 'date': '2026/07/30', 'note': '当前版本'},
      {'version': _decrementVersion(version), 'date': '2026/07/25', 'note': '修复已知问题'},
      {'version': _decrementVersion(_decrementVersion(version)), 'date': '2026/07/20', 'note': '初始版本'},
    ];

    return versions.map((v) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 8),
        child: Row(
          children: [
            Container(
              width: 8,
              height: 8,
              decoration: BoxDecoration(
                color: v['note'] == '当前版本' ? context.accentPrimary : context.textTertiary,
                shape: BoxShape.circle,
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('v${v['version']}', style: AppTypography.bodySmall(context)),
                  Text(v['date']!, style: AppTypography.label(context)),
                ],
              ),
            ),
            Text(v['note']!, style: AppTypography.label(context).copyWith(color: v['note'] == '当前版本' ? context.accentPrimary : context.textTertiary)),
          ],
        ),
      );
    }).toList();
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        children: [
          SizedBox(
            width: 80,
            child: Text(label, style: AppTypography.label(context).copyWith(color: context.textTertiary)),
          ),
          Expanded(child: Text(value, style: AppTypography.body(context))),
        ],
      ),
    );
  }

  String _decrementVersion(String version) {
    final parts = version.split('.');
    if (parts.length != 3) return '0.0.0';
    final major = int.tryParse(parts[0]) ?? 0;
    final minor = int.tryParse(parts[1]) ?? 0;
    final patch = int.tryParse(parts[2]) ?? 0;
    if (patch > 0) {
      return '$major.$minor.${patch - 1}';
    } else if (minor > 0) {
      return '$major.${minor - 1}.9';
    } else if (major > 0) {
      return '${major - 1}.9.9';
    }
    return '0.0.0';
  }

  void _showDeleteConfirm(BuildContext context, Map<String, dynamic> workspace) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('删除工作区', style: AppTypography.cardTitle(context)),
          content: Text('确定要删除工作区「${workspace['name'] ?? '未命名'}」吗？所有版本和会话数据将被清除，此操作不可恢复。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  _workspaces.removeWhere((w) => w['id'] == workspace['id']);
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已删除工作区：${workspace['name'] ?? '未命名'}')));
              },
              child: Text('删除', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }
}
