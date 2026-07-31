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
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class UpdatesPage extends ConsumerStatefulWidget {
  const UpdatesPage({super.key});

  @override
  ConsumerState<UpdatesPage> createState() => _UpdatesPageState();
}

class _UpdatesPageState extends ConsumerState<UpdatesPage> {
  double _downloadProgress = 0;
  bool _isDownloading = false;
  bool _isInstalling = false;
  bool _isInstalled = false;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '更新中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
          children: [
            _buildAvailableUpdateCard(context),
            const SizedBox(height: AppSpacing.xl),
            const AmitiaSectionHeader(title: '更新历史'),
            const SizedBox(height: AppSpacing.sm),
            ...MockKernel.updateHistory.map((u) => _buildHistoryItem(context, u)),
          ],
        ),
      ),
    );
  }

  Widget _buildAvailableUpdateCard(BuildContext context) {
    final update = MockKernel.availableUpdate;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 48,
                  height: 48,
                  decoration: BoxDecoration(
                    gradient: LinearGradient(
                      colors: [context.accentPrimary, context.accentSecondary],
                    ),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: const Icon(Icons.system_update, size: 26, color: Colors.white),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('v${update.version}', style: AppTypography.cardTitle(context).copyWith(fontSize: 18)),
                      const SizedBox(height: 2),
                      Text('发布于 ${_formatDate(update.date)}', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                if (!_isDownloading && !_isInstalling && !_isInstalled)
                  const AmitiaStatusBadge(label: '可更新', type: BadgeType.accent),
                if (_isInstalled)
                  const AmitiaStatusBadge(label: '已安装', type: BadgeType.success),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Text('更新内容', style: AppTypography.cardTitle(context).copyWith(fontSize: 14)),
            const SizedBox(height: AppSpacing.sm),
            ..._buildUpdateNotes(context),
            if (_isDownloading) ...[
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Text('下载中', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
                  const SizedBox(width: 8),
                  Expanded(child: AmitiaProgressBar(progress: _downloadProgress)),
                  const SizedBox(width: 8),
                  Text('${(_downloadProgress * 100).toInt()}%', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
                ],
              ),
            ],
            if (_isInstalling) ...[
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2, color: context.accentPrimary),
                  ),
                  const SizedBox(width: 8),
                  Text('正在安装...', style: AppTypography.label(context).copyWith(color: context.accentPrimary)),
                ],
              ),
            ],
            const SizedBox(height: AppSpacing.md),
            if (!_isDownloading && !_isInstalling && !_isInstalled)
              AmitiaButton(
                label: '下载并安装',
                isFullWidth: true,
                icon: Icons.download,
                onPressed: _startDownload,
              ),
            if (_isInstalled)
              AmitiaButton(
                label: '回滚到上个版本',
                isFullWidth: true,
                isSecondary: true,
                isDestructive: true,
                icon: Icons.undo,
                onPressed: () => _showRollbackConfirm(context),
              ),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildUpdateNotes(BuildContext context) {
    final notes = [
      '新增 WASM Runtime 模块管理功能',
      '优化 Hook 中心的熔断器恢复策略',
      '修复事件中心死信重放的问题',
      '提升调度中心的执行稳定性',
    ];
    return notes.map((note) {
      return Padding(
        padding: const EdgeInsets.only(bottom: 4),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              margin: const EdgeInsets.only(top: 6),
              width: 4,
              height: 4,
              decoration: BoxDecoration(color: context.accentPrimary, shape: BoxShape.circle),
            ),
            const SizedBox(width: 8),
            Expanded(child: Text(note, style: AppTypography.caption(context))),
          ],
        ),
      );
    }).toList();
  }

  Widget _buildHistoryItem(BuildContext context, UpdateInfo update) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: _historyColor(context, update.status).withValues(alpha: 0.1),
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(_historyIcon(update.status), size: 22, color: _historyColor(context, update.status)),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('v${update.version}', style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(update.date != null ? _formatDate(update.date) : '未知日期', style: AppTypography.label(context)),
                ],
              ),
            ),
            AmitiaStatusBadge(
              label: update.status,
              type: update.status == '已安装'
                  ? BadgeType.success
                  : update.status == '已回滚'
                      ? BadgeType.warning
                      : BadgeType.neutral,
            ),
          ],
        ),
      ),
    );
  }

  Color _historyColor(BuildContext context, String status) {
    switch (status) {
      case '已安装':
        return context.success;
      case '已回滚':
        return context.warning;
      default:
        return context.textSecondary;
    }
  }

  IconData _historyIcon(String status) {
    switch (status) {
      case '已安装':
        return Icons.check_circle_outline;
      case '已回滚':
        return Icons.undo;
      default:
        return Icons.history;
    }
  }

  String _formatDate(DateTime? date) {
    if (date == null) return '未知';
    return '${date.year}/${date.month}/${date.day}';
  }

  void _startDownload() {
    setState(() {
      _isDownloading = true;
    });
    Future.delayed(const Duration(milliseconds: 500), () {
      if (!mounted) return;
      _simulateDownload();
    });
  }

  void _simulateDownload() {
    if (!mounted) return;
    setState(() {
      _downloadProgress += 0.15;
    });
    if (_downloadProgress >= 1.0) {
      setState(() {
        _isDownloading = false;
        _isInstalling = true;
      });
      Future.delayed(const Duration(seconds: 2), () {
        if (!mounted) return;
        setState(() {
          _isInstalling = false;
          _isInstalled = true;
        });
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('更新已安装完成')),
        );
      });
    } else {
      Future.delayed(const Duration(milliseconds: 300), () {
        if (!mounted) return;
        _simulateDownload();
      });
    }
  }

  void _showRollbackConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('回滚版本', style: AppTypography.cardTitle(context)),
          content: Text('确定要回滚到上个版本吗？回滚后当前版本的功能将不可用。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  _isInstalled = false;
                  _downloadProgress = 0;
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('已回滚到上个版本')),
                );
              },
              child: Text('确认回滚', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }
}
