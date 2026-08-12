import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
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
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _workspaces.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.md),
        itemBuilder: (context, i) {
          final w = _workspaces[i];
          final kindLabel = _kindLabel(w.kind);
          final statusLabel = _statusLabel(w.status);
          return Container(
            padding: const EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Row(
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
          );
        },
      ),
    );
  }
}
