import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/services/providers.dart';

class PetTasksPage extends ConsumerStatefulWidget {
  const PetTasksPage({super.key});

  @override
  ConsumerState<PetTasksPage> createState() => _PetTasksPageState();
}

class _PetTasksPageState extends ConsumerState<PetTasksPage> {
  List<Map<String, dynamic>> _tasks = [];
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
      final svc = ref.read(extensionServiceProvider);
      final sessions = await svc.workshopSessions();
      if (mounted) {
        setState(() { _tasks = sessions; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _statusLabel(String? status) {
    switch (status) {
      case 'pending':
        return '待处理';
      case 'processing':
        return '处理中';
      case 'completed':
        return '已完成';
      case 'cancelled':
        return '已取消';
      default:
        return status ?? '';
    }
  }

  BadgeType _statusBadgeType(String? status) {
    switch (status) {
      case 'pending':
        return BadgeType.neutral;
      case 'processing':
        return BadgeType.accent;
      case 'completed':
        return BadgeType.success;
      case 'cancelled':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '任务列表',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          Padding(
            padding: EdgeInsets.only(right: AppSpacing.sm),
            child: AmitiaIconButton(
              icon: Icons.add,
              color: context.accentPrimary,
              onPressed: () => context.push(AppRoutes.workshopPetCreate),
            ),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(context),
      ),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(child: Text('加载失败: $_error'));
    }
    if (_tasks.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.assignment_outlined,
        title: '暂无任务',
        subtitle: '点击右上角创建新桌宠',
        actionText: '创建桌宠',
        onAction: () => context.push(AppRoutes.workshopPetCreate),
      );
    }
    return ListView.builder(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      itemCount: _tasks.length,
      itemBuilder: (context, index) => _buildTaskCard(context, _tasks[index]),
    );
  }

  Widget _buildTaskCard(BuildContext context, Map<String, dynamic> task) {
    final name = task['name']?.toString() ?? '';
    final characterName = task['characterName']?.toString() ?? task['character']?.toString() ?? '';
    final status = task['status']?.toString();
    final createdAt = task['createdAt']?.toString() ?? '';
    final completedActions = (task['completedActions'] is num) ? (task['completedActions'] as num).toInt() : 0;
    final totalActions = (task['totalActions'] is num) ? (task['totalActions'] as num).toInt() : 0;
    final progress = (task['progress'] is num) ? (task['progress'] as num).toInt() : 0;
    final sessionId = task['id']?.toString() ?? '';

    final canCancel = status == 'pending' || status == 'processing';
    final canProcess = status == 'pending' || status == 'processing';

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: () => context.push(AppRoutes.petProcessing(sessionId)),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.pets_outlined, size: 22, color: context.accentPrimary),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(
                        '$characterName · $createdAt',
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Text('$completedActions/$totalActions 动作', style: AppTypography.caption(context)),
                SizedBox(width: AppSpacing.md),
                Expanded(child: AmitiaProgressBar(progress: (progress / 100.0).clamp(0.0, 1.0))),
                SizedBox(width: AppSpacing.sm),
                Text('$progress%', style: AppTypography.caption(context)),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                if (canProcess)
                  Expanded(
                    child: AmitiaButton(
                      label: '继续处理',
                      isSecondary: true,
                      height: 38,
                      icon: Icons.play_arrow,
                      onPressed: () => context.push(AppRoutes.petProcessing(sessionId)),
                    ),
                  )
                else
                  Expanded(
                    child: AmitiaButton(
                      label: '查看详情',
                      isSecondary: true,
                      height: 38,
                      onPressed: () => context.push(AppRoutes.petProcessing(sessionId)),
                    ),
                  ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '创建处理任务',
                    height: 38,
                    icon: Icons.add,
                    onPressed: () => _showCreateProcessingConfirm(sessionId, name),
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
                if (canCancel)
                  SizedBox(
                    width: 38,
                    height: 38,
                    child: AmitiaIconButton(
                      icon: Icons.close,
                      color: context.error,
                      backgroundColor: context.error.withValues(alpha: 0.1),
                      onPressed: () => _showCancelConfirm(sessionId, name),
                    ),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showCancelConfirm(String sessionId, String name) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('取消任务', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认取消任务「$name」？取消后无法恢复。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('保留', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                setState(() {
                  final idx = _tasks.indexWhere((t) => t['id']?.toString() == sessionId);
                  if (idx >= 0) {
                    _tasks[idx] = Map<String, dynamic>.from(_tasks[idx]);
                    _tasks[idx]['status'] = 'cancelled';
                  }
                });
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('任务「$name」已取消')),
                );
              },
              child: Text('取消任务', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showCreateProcessingConfirm(String sessionId, String name) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('创建处理任务', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认为任务「$name」创建处理任务？将为每个动作生成处理子任务。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('已为「$name」创建处理任务')),
                );
                context.push(AppRoutes.petProcessing(sessionId));
              },
              child: Text('创建', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }
}
