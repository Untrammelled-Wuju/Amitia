import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class SchedulesPage extends ConsumerStatefulWidget {
  const SchedulesPage({super.key});

  @override
  ConsumerState<SchedulesPage> createState() => _SchedulesPageState();
}

class _SchedulesPageState extends ConsumerState<SchedulesPage> {
  late List<ScheduleEntry> _schedules;

  @override
  void initState() {
    super.initState();
    _schedules = List.from(MockKernel.schedules);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '调度中心',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: _schedules.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.schedule_outlined,
                title: '暂无调度',
                subtitle: '调度将在配置后自动生成',
              )
            : ListView.builder(
                padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
                itemCount: _schedules.length,
                itemBuilder: (context, index) => _buildScheduleCard(context, _schedules[index]),
              ),
      ),
    );
  }

  Widget _buildScheduleCard(BuildContext context, ScheduleEntry schedule) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showScheduleDetailSheet(context, schedule),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: schedule.isEnabled ? context.accentSoft : context.surfaceSecondary,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(
                    Icons.schedule_outlined,
                    size: 22,
                    color: schedule.isEnabled ? context.accentPrimary : context.textTertiary,
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(schedule.name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text('ID: ${schedule.id}', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(
                  label: schedule.isEnabled ? '已启用' : '已停用',
                  type: schedule.isEnabled ? BadgeType.success : BadgeType.neutral,
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            _buildTimeRow(context, '下次执行', schedule.nextRun, Icons.schedule),
            if (schedule.lastRun != null)
              _buildTimeRow(context, '最近执行', schedule.lastRun, Icons.history),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: '立即执行',
                    isSecondary: true,
                    icon: Icons.play_arrow,
                    onPressed: schedule.isEnabled ? () => _showExecuteConfirm(context, schedule) : null,
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '跳过',
                    isSecondary: true,
                    icon: Icons.skip_next,
                    onPressed: schedule.isEnabled ? () => _showSkipConfirm(context, schedule) : null,
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTimeRow(BuildContext context, String label, DateTime? time, IconData icon) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Row(
        children: [
          Icon(icon, size: 16, color: context.textTertiary),
          const SizedBox(width: 6),
          Text(label, style: AppTypography.label(context).copyWith(color: context.textSecondary)),
          const SizedBox(width: 8),
          Text(
            time != null ? _formatDateTime(time) : '暂无',
            style: AppTypography.bodySmall(context).copyWith(fontSize: 13),
          ),
        ],
      ),
    );
  }

  String _formatDateTime(DateTime time) {
    return '${time.month}/${time.day} ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }

  void _showScheduleDetailSheet(BuildContext context, ScheduleEntry schedule) {
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
                Text('调度详情', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                _buildDetailRow(context, '调度名称', schedule.name),
                _buildDetailRow(context, '调度 ID', schedule.id),
                _buildDetailRow(context, '状态', schedule.isEnabled ? '已启用' : '已停用'),
                _buildDetailRow(context, '下次执行', schedule.nextRun != null ? _formatDateTime(schedule.nextRun!) : '暂无'),
                _buildDetailRow(context, '最近执行', schedule.lastRun != null ? _formatDateTime(schedule.lastRun!) : '暂无'),
                const SizedBox(height: 20),
                Row(
                  children: [
                    Expanded(
                      child: AmitiaButton(
                        label: '立即执行',
                        icon: Icons.play_arrow,
                        onPressed: schedule.isEnabled
                            ? () {
                                Navigator.pop(context);
                                _showExecuteConfirm(context, schedule);
                              }
                            : null,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: AmitiaButton(
                        label: '跳过',
                        isSecondary: true,
                        icon: Icons.skip_next,
                        onPressed: schedule.isEnabled
                            ? () {
                                Navigator.pop(context);
                                _showSkipConfirm(context, schedule);
                              }
                            : null,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 8),
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
        children: [
          SizedBox(
            width: 70,
            child: Text(label, style: AppTypography.label(context).copyWith(color: context.textTertiary)),
          ),
          Expanded(child: Text(value, style: AppTypography.body(context))),
        ],
      ),
    );
  }

  void _showExecuteConfirm(BuildContext context, ScheduleEntry schedule) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('立即执行', style: AppTypography.cardTitle(context)),
          content: Text('确定要立即执行调度「${schedule.name}」吗？', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已触发执行：${schedule.name}')));
              },
              child: Text('执行', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showSkipConfirm(BuildContext context, ScheduleEntry schedule) {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('跳过调度', style: AppTypography.cardTitle(context)),
          content: Text('确定要跳过调度「${schedule.name}」的本次执行吗？', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  final idx = _schedules.indexWhere((s) => s.id == schedule.id);
                  if (idx >= 0) {
                    _schedules[idx] = ScheduleEntry(
                      id: schedule.id,
                      name: schedule.name,
                      nextRun: schedule.nextRun,
                      lastRun: DateTime.now(),
                      isEnabled: schedule.isEnabled,
                    );
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已跳过调度：${schedule.name}')));
              },
              child: Text('跳过', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }
}
