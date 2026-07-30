import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class PetProcessingPage extends ConsumerStatefulWidget {
  final String taskId;

  const PetProcessingPage({super.key, required this.taskId});

  @override
  ConsumerState<PetProcessingPage> createState() => _PetProcessingPageState();
}

class _PetProcessingPageState extends ConsumerState<PetProcessingPage> {
  late List<ProcessingTask> _processingTasks;
  late PetTask _petTask;
  int _selectedActionIndex = 0;
  int _selectedAttemptIndex = 0;

  @override
  void initState() {
    super.initState();
    _processingTasks = MockWorkshop.processingTasks(widget.taskId);
    PetTask? found;
    for (final t in MockWorkshop.petTasks) {
      if (t.id == widget.taskId) {
        found = t;
        break;
      }
    }
    _petTask = found ?? PetTask(id: widget.taskId, name: '未知任务', characterName: '未知', createdAt: DateTime.now());
    if (_processingTasks.isNotEmpty) {
      final selected = _processingTasks[_selectedActionIndex];
      for (int i = 0; i < selected.attempts.length; i++) {
        if (selected.attempts[i].isSelected) {
          _selectedAttemptIndex = i;
          break;
        }
      }
    }
  }

  BadgeType _statusBadgeType(ProcessingStatus status) {
    switch (status) {
      case ProcessingStatus.pending:
        return BadgeType.neutral;
      case ProcessingStatus.reviewing:
        return BadgeType.warning;
      case ProcessingStatus.approved:
        return BadgeType.success;
      case ProcessingStatus.rejected:
        return BadgeType.error;
    }
  }

  String _statusLabel(ProcessingStatus status) {
    switch (status) {
      case ProcessingStatus.pending:
        return '待处理';
      case ProcessingStatus.reviewing:
        return '审核中';
      case ProcessingStatus.approved:
        return '已通过';
      case ProcessingStatus.rejected:
        return '已拒绝';
    }
  }

  Color _frameColor(BuildContext context, FrameEntry frame) {
    if (frame.status == '等待中') return context.surfaceSecondary;
    if (frame.qualityLabel == '不合格') return context.error.withValues(alpha: 0.15);
    if (frame.qualityLabel == '高质量') return context.success.withValues(alpha: 0.15);
    return context.accentSoft;
  }

  Color _frameIconColor(BuildContext context, FrameEntry frame) {
    if (frame.status == '等待中') return context.textTertiary;
    if (frame.qualityLabel == '不合格') return context.error;
    if (frame.qualityLabel == '高质量') return context.success;
    return context.accentPrimary;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '处理审核',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: _processingTasks.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.assignment_late_outlined,
                title: '暂无处理任务',
                subtitle: '该任务还没有创建处理子任务',
              )
            : Column(
                children: [
                  _buildTaskHeader(context),
                  _buildActionBarTabs(context),
                  Expanded(
                    child: ListView(
                      padding: const EdgeInsets.all(AppSpacing.pagePadding),
                      children: [
                        _buildQualityBanner(context),
                        const SizedBox(height: AppSpacing.md),
                        _buildAttemptSelector(context),
                        const SizedBox(height: AppSpacing.md),
                        _buildFrameGrid(context),
                        const SizedBox(height: AppSpacing.md),
                        _buildOriginalResult(context),
                        const SizedBox(height: AppSpacing.md),
                        _buildActionButtons(context),
                        const SizedBox(height: AppSpacing.xxl),
                      ],
                    ),
                  ),
                ],
              ),
      ),
    );
  }

  Widget _buildTaskHeader(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(
          bottom: BorderSide(color: context.borderPrimary, width: 0.5),
        ),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(Icons.pets_outlined, size: 20, color: context.accentPrimary),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(_petTask.name, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(
                  '${_petTask.characterName} · ${_processingTasks.length} 个动作待处理',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActionBarTabs(BuildContext context) {
    return SizedBox(
      height: 44,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
        itemCount: _processingTasks.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.xs),
        itemBuilder: (context, index) {
          final task = _processingTasks[index];
          final isSelected = index == _selectedActionIndex;
          return GestureDetector(
            onTap: () {
              setState(() {
                _selectedActionIndex = index;
                _selectedAttemptIndex = 0;
                for (int i = 0; i < task.attempts.length; i++) {
                  if (task.attempts[i].isSelected) {
                    _selectedAttemptIndex = i;
                    break;
                  }
                }
              });
            },
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(
                  task.actionName,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildQualityBanner(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    final qualityType = task.qualityStatus == '高质量'
        ? BadgeType.success
        : task.qualityStatus == '部分不合格'
            ? BadgeType.error
            : task.qualityStatus == '待审核'
                ? BadgeType.warning
                : BadgeType.neutral;
    return AmitiaCard(
      child: Row(
        children: [
          Icon(Icons.verified_outlined, size: 22, color: context.accentPrimary),
          const SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(task.actionName, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(
                  '${task.completedFrames}/${task.totalFrames} 帧已完成',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              AmitiaStatusBadge(label: _statusLabel(task.status), type: _statusBadgeType(task.status)),
              const SizedBox(height: 4),
              AmitiaStatusBadge(label: task.qualityStatus, type: qualityType),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildAttemptSelector(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    if (task.attempts.isEmpty) {
      return AmitiaCard(
        child: Row(
          children: [
            Icon(Icons.history, size: 20, color: context.textTertiary),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Text('暂无 Attempt 记录', style: AppTypography.caption(context)),
            ),
          ],
        ),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Attempt 切换', style: AppTypography.label(context)),
        const SizedBox(height: AppSpacing.xs),
        AmitiaCard(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
          child: Row(
            children: task.attempts.asMap().entries.map((entry) {
              final i = entry.key;
              final attempt = entry.value;
              final isSelected = i == _selectedAttemptIndex;
              return Expanded(
                child: GestureDetector(
                  onTap: () => _switchAttempt(i),
                  child: Container(
                    padding: const EdgeInsets.symmetric(vertical: 10),
                    decoration: BoxDecoration(
                      color: isSelected ? context.accentSoft : Colors.transparent,
                      borderRadius: AppRadius.brSmall,
                    ),
                    child: Center(
                      child: Text(
                        attempt.label,
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                          color: isSelected ? context.accentPrimary : context.textSecondary,
                        ),
                      ),
                    ),
                  ),
                ),
              );
            }).toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildFrameGrid(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('帧列表', style: AppTypography.label(context)),
        const SizedBox(height: AppSpacing.xs),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 4,
            mainAxisSpacing: AppSpacing.sm,
            crossAxisSpacing: AppSpacing.sm,
            childAspectRatio: 0.8,
          ),
          itemCount: task.frames.length,
          itemBuilder: (context, index) {
            final frame = task.frames[index];
            return _buildFrameCell(context, frame);
          },
        ),
      ],
    );
  }

  Widget _buildFrameCell(BuildContext context, FrameEntry frame) {
    return GestureDetector(
      onTap: () => _showFramePreview(frame),
      child: Container(
        decoration: BoxDecoration(
          color: _frameColor(context, frame),
          borderRadius: AppRadius.brSmall,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              frame.status == '等待中' ? Icons.hourglass_empty : Icons.image,
              size: 24,
              color: _frameIconColor(context, frame),
            ),
            const SizedBox(height: AppSpacing.xs),
            Text(
              '帧 ${frame.index + 1}',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w500,
                color: frame.status == '等待中' ? context.textTertiary : context.textSecondary,
              ),
            ),
            if (frame.qualityLabel != null)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(
                  frame.qualityLabel!,
                  style: TextStyle(
                    fontSize: 9,
                    color: frame.qualityLabel == '不合格'
                        ? context.error
                        : frame.qualityLabel == '高质量'
                            ? context.success
                            : context.textTertiary,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildOriginalResult(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('原始结果', style: AppTypography.label(context)),
        const SizedBox(height: AppSpacing.xs),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildResultRow(context, '动作', task.actionName),
              const SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, '帧数', '${task.totalFrames} 帧'),
              const SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, '已完成', '${task.completedFrames} 帧'),
              const SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, 'Attempt', _selectedAttemptIndex < task.attempts.length ? task.attempts[_selectedAttemptIndex].label : '无'),
              const SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, '质量', task.qualityStatus),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildResultRow(BuildContext context, String label, String value) {
    return Row(
      children: [
        SizedBox(
          width: 64,
          child: Text(label, style: AppTypography.label(context)),
        ),
        Expanded(
          child: Text(value, style: AppTypography.bodySmall(context)),
        ),
      ],
    );
  }

  Widget _buildActionButtons(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '编辑动作',
                isSecondary: true,
                icon: Icons.edit_outlined,
                onPressed: () => context.push(AppRoutes.petActionEditor(widget.taskId, task.actionKey)),
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '重处理',
                isSecondary: true,
                icon: Icons.refresh,
                onPressed: _showReprocessConfirm,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.sm),
        Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '排除动作',
                isSecondary: true,
                icon: Icons.block,
                onPressed: _showExcludeConfirm,
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '打包',
                icon: Icons.archive_outlined,
                onPressed: _showPackageDialog,
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.sm),
        AmitiaButton(
          label: '安装到桌宠',
          icon: Icons.install_desktop,
          isFullWidth: true,
          onPressed: _showInstallDialog,
        ),
      ],
    );
  }

  void _showFramePreview(FrameEntry frame) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return Dialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('帧 ${frame.index + 1} 预览', style: AppTypography.cardTitle(context)),
                const SizedBox(height: AppSpacing.md),
                Container(
                  width: 200,
                  height: 200,
                  decoration: BoxDecoration(
                    color: _frameColor(context, frame),
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary),
                  ),
                  child: Icon(Icons.image, size: 64, color: _frameIconColor(context, frame)),
                ),
                const SizedBox(height: AppSpacing.md),
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    AmitiaStatusBadge(
                      label: frame.status,
                      type: frame.status == '已完成' ? BadgeType.success : BadgeType.neutral,
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    if (frame.qualityLabel != null)
                      AmitiaStatusBadge(
                        label: frame.qualityLabel!,
                        type: frame.qualityLabel == '不合格'
                            ? BadgeType.error
                            : frame.qualityLabel == '高质量'
                                ? BadgeType.success
                                : BadgeType.neutral,
                      ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: '关闭',
                  isSecondary: true,
                  isFullWidth: true,
                  onPressed: () => Navigator.pop(dialogContext),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _switchAttempt(int index) {
    setState(() {
      _selectedAttemptIndex = index;
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('已切换到 ${_processingTasks[_selectedActionIndex].attempts[index].label}')),
    );
  }

  void _showReprocessConfirm() {
    final task = _processingTasks[_selectedActionIndex];
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('重处理动作', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认重新处理动作「${task.actionName}」？将生成新的 Attempt。',
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
                  SnackBar(content: Text('「${task.actionName}」重处理已启动')),
                );
              },
              child: Text('确认', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showExcludeConfirm() {
    final task = _processingTasks[_selectedActionIndex];
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('排除动作', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认排除动作「${task.actionName}」？排除后该动作将不会安装到桌宠。',
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
                setState(() {
                  _processingTasks.removeAt(_selectedActionIndex);
                  if (_selectedActionIndex >= _processingTasks.length && _processingTasks.isNotEmpty) {
                    _selectedActionIndex = _processingTasks.length - 1;
                  }
                });
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('动作「${task.actionName}」已排除')),
                );
              },
              child: Text('排除', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showPackageDialog() {
    final task = _processingTasks[_selectedActionIndex];
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('打包资源', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认打包动作「${task.actionName}」的资源？打包后将生成可安装的资源包。',
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
                  SnackBar(content: Text('「${task.actionName}」资源打包完成')),
                );
              },
              child: Text('打包', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showInstallDialog() {
    final task = _processingTasks[_selectedActionIndex];
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('安装到桌宠', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认将动作「${task.actionName}」安装到桌宠「${_petTask.name}」？',
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
                  SnackBar(content: Text('动作「${task.actionName}」安装成功')),
                );
              },
              child: Text('安装', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }
}
