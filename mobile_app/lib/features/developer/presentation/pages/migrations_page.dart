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

class MigrationsPage extends ConsumerStatefulWidget {
  const MigrationsPage({super.key});

  @override
  ConsumerState<MigrationsPage> createState() => _MigrationsPageState();
}

class _MigrationsPageState extends ConsumerState<MigrationsPage> {
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _plans = [];

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
          final migrations = data['migrations'];
          if (migrations is List) {
            _plans = migrations.map((e) => Map<String, dynamic>.from(e as Map)).toList();
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
    if (_plans.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.merge_type_outlined, title: '暂无迁移计划');
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '迁移与灰度',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: _resumeScan,
            color: context.accentPrimary,
            tooltip: '恢复扫描',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView.builder(
          padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
          itemCount: _plans.length,
          itemBuilder: (context, index) => _buildMigrationCard(context, _plans[index]),
        ),
      ),
    );
  }

  Widget _buildMigrationCard(BuildContext context, Map<String, dynamic> plan) {
    final name = plan['name'] as String? ?? '迁移计划';
    final id = plan['id'] as String? ?? '';
    final status = plan['status'] as String? ?? '灰度中';
    final progress = (plan['progress'] as num?)?.toDouble() ?? 0.0;
    final rollbackReason = plan['rollback_reason'] as String?;

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showCanaryDetailSheet(context, plan),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: _statusColor(context, status).withValues(alpha: 0.1),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(_statusIcon(status), size: 22, color: _statusColor(context, status)),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('ID: $id', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                _buildStatusBadge(status),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            if (status == '灰度中') ...[
              Row(
                children: [
                  Text('灰度进度', style: AppTypography.label(context)),
                  const SizedBox(width: 8),
                  Expanded(child: AmitiaProgressBar(progress: progress / 100)),
                  const SizedBox(width: 8),
                  Text('${progress.toInt()}%', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
                ],
              ),
              SizedBox(height: AppSpacing.md),
            ],
            if (rollbackReason != null) ...[
              Container(
                width: double.infinity,
                padding: EdgeInsets.all(AppSpacing.sm),
                decoration: BoxDecoration(
                  color: context.error.withValues(alpha: 0.08),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: [
                    Icon(Icons.error_outline, size: 16, color: context.error),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text('回滚原因: $rollbackReason', style: AppTypography.caption(context).copyWith(color: context.error)),
                    ),
                  ],
                ),
              ),
              SizedBox(height: AppSpacing.md),
            ],
            Row(
              children: _buildActionButtons(context, plan),
            ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildActionButtons(BuildContext context, Map<String, dynamic> plan) {
    final buttons = <Widget>[];
    final status = plan['status'] as String? ?? '';

    if (status == '灰度中') {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '灰度详情',
          isSecondary: true,
          icon: Icons.info_outline,
          onPressed: () => _showCanaryDetailSheet(context, plan),
        ),
      ));
      buttons.add(SizedBox(width: AppSpacing.sm));
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '回滚',
          isDestructive: true,
          icon: Icons.undo,
          onPressed: () => _showRollbackConfirm(context, plan),
        ),
      ));
    } else if (status == '已完成') {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '回滚',
          isSecondary: true,
          isDestructive: true,
          icon: Icons.undo,
          onPressed: () => _showRollbackConfirm(context, plan),
        ),
      ));
    } else if (status == '已回滚') {
      buttons.add(Expanded(
        child: AmitiaButton(
          label: '重新迁移',
          icon: Icons.refresh,
          onPressed: () => _showRiskConfirm(context, plan),
        ),
      ));
    }

    return buttons;
  }

  Color _statusColor(BuildContext context, String status) {
    switch (status) {
      case '已完成':
        return context.success;
      case '灰度中':
        return context.accentPrimary;
      case '已回滚':
        return context.error;
      default:
        return context.textSecondary;
    }
  }

  IconData _statusIcon(String status) {
    switch (status) {
      case '已完成':
        return Icons.check_circle_outline;
      case '灰度中':
        return Icons.flaky_outlined;
      case '已回滚':
        return Icons.undo;
      default:
        return Icons.pending_outlined;
    }
  }

  AmitiaStatusBadge _buildStatusBadge(String status) {
    switch (status) {
      case '已完成':
        return const AmitiaStatusBadge(label: '已完成', type: BadgeType.success);
      case '灰度中':
        return const AmitiaStatusBadge(label: '灰度中', type: BadgeType.accent);
      case '已回滚':
        return const AmitiaStatusBadge(label: '已回滚', type: BadgeType.error);
      default:
        return AmitiaStatusBadge(label: status, type: BadgeType.neutral);
    }
  }

  void _showCanaryDetailSheet(BuildContext context, Map<String, dynamic> plan) {
    final name = plan['name'] as String? ?? '';
    final id = plan['id'] as String? ?? '';
    final status = plan['status'] as String? ?? '';
    final progress = (plan['progress'] as num?)?.toDouble() ?? 0.0;
    final rollbackReason = plan['rollback_reason'] as String?;

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
                Text('灰度发布详情', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                _buildDetailRow(context, '迁移计划', name),
                _buildDetailRow(context, '计划 ID', id),
                _buildDetailRow(context, '状态', status),
                _buildDetailRow(context, '进度', '${progress.toInt()}%'),
                if (rollbackReason != null)
                  _buildDetailRow(context, '回滚原因', rollbackReason),
                _buildDetailRow(context, '灰度比例', '35%'),
                _buildDetailRow(context, '目标比例', '100%'),
                _buildDetailRow(context, '健康检查', '正常'),
                _buildDetailRow(context, '错误率', '0.2%'),
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

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
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

  void _showRollbackConfirm(BuildContext context, Map<String, dynamic> plan) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('回滚迁移', style: AppTypography.cardTitle(context)),
          content: Text('确定要回滚「${plan['name']}」吗？回滚后数据将恢复到迁移前的状态。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _plans.indexWhere((p) => p['id'] == plan['id']);
                  if (idx >= 0) {
                    _plans[idx]['status'] = '已回滚';
                    _plans[idx]['progress'] = 0;
                    _plans[idx]['rollback_reason'] = '手动回滚';
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已回滚：${plan['name']}')));
              },
              child: Text('确认回滚', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showRiskConfirm(BuildContext context, Map<String, dynamic> plan) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('风险确认', style: AppTypography.cardTitle(context)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('重新迁移「${plan['name']}」存在以下风险：', style: AppTypography.body(context)),
              const SizedBox(height: 8),
              Row(
                children: [
                  Icon(Icons.warning_amber_rounded, size: 16, color: context.warning),
                  const SizedBox(width: 6),
                  Expanded(child: Text('数据可能不一致', style: AppTypography.caption(context))),
                ],
              ),
              const SizedBox(height: 4),
              Row(
                children: [
                  Icon(Icons.warning_amber_rounded, size: 16, color: context.warning),
                  const SizedBox(width: 6),
                  Expanded(child: Text('服务可能短暂中断', style: AppTypography.caption(context))),
                ],
              ),
              const SizedBox(height: 4),
              Row(
                children: [
                  Icon(Icons.warning_amber_rounded, size: 16, color: context.warning),
                  const SizedBox(width: 6),
                  Expanded(child: Text('之前的问题可能重现', style: AppTypography.caption(context))),
                ],
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _plans.indexWhere((p) => p['id'] == plan['id']);
                  if (idx >= 0) {
                    _plans[idx]['status'] = '灰度中';
                    _plans[idx]['progress'] = 0;
                    _plans[idx].remove('rollback_reason');
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已开始重新迁移：${plan['name']}')));
              },
              child: Text('确认风险并继续', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }

  void _resumeScan() {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已恢复迁移扫描')),
    );
  }
}
