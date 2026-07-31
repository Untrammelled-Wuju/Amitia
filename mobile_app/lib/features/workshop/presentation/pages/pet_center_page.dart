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
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class PetCenterPage extends ConsumerWidget {
  const PetCenterPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final runningPet = MockWorkshop.installations.where((p) => p.isRunning).firstOrNull;
    final activeTasks = MockWorkshop.petTasks.where((t) => t.status != PetTaskStatus.completed).toList();
    final recentTasks = MockWorkshop.petTasks.take(3).toList();

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠制作',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
          children: [
            const SizedBox(height: AppSpacing.sm),
            if (runningPet != null) _buildRunningPetCard(context, runningPet),
            if (runningPet != null) const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '快速操作'),
            const SizedBox(height: AppSpacing.sm),
            _buildQuickActions(context),
            const SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(
              title: '生成任务',
              actionText: '查看全部',
              onAction: () => context.push(AppRoutes.workshopPetTasks),
            ),
            const SizedBox(height: AppSpacing.sm),
            _buildTaskListCard(context, activeTasks),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '最近记录'),
            const SizedBox(height: AppSpacing.sm),
            _buildRecentRecords(context, recentTasks),
          ],
        ),
      ),
    );
  }

  Widget _buildRunningPetCard(BuildContext context, PetInstallation pet) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: context.accentPrimary,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  pet.characterName.substring(0, 1),
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(pet.name, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(
                    '${pet.characterName} · ${pet.actions.length} 个动作 · 缩放 ${(pet.scale * 100).round()}%',
                    style: AppTypography.caption(context),
                  ),
                ],
              ),
            ),
            AmitiaStatusBadge(label: '运行中', type: BadgeType.success),
          ],
        ),
      ),
    );
  }

  Widget _buildQuickActions(BuildContext context) {
    final actions = [
      (Icons.add_circle_outline, '创建桌宠', () => context.push(AppRoutes.workshopPetCreate)),
      (Icons.list_alt, '任务列表', () => context.push(AppRoutes.workshopPetTasks)),
      (Icons.install_desktop, '安装管理', () => context.push(AppRoutes.workshopPetInstallations)),
    ];
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Row(
        children: actions.map((action) {
          return Expanded(
            child: Padding(
              padding: const EdgeInsets.only(right: AppSpacing.sm),
              child: GestureDetector(
                onTap: action.$3,
                child: AmitiaCard(
                  padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
                  child: Column(
                    children: [
                      Icon(action.$1, size: 26, color: context.accentPrimary),
                      const SizedBox(height: AppSpacing.xs),
                      Text(action.$2, style: AppTypography.bodySmall(context)),
                    ],
                  ),
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildTaskListCard(BuildContext context, List<PetTask> tasks) {
    if (tasks.isEmpty) {
      return Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaCard(
          child: AmitiaEmptyState(
            icon: Icons.check_circle_outline,
            title: '没有进行中的任务',
            subtitle: '所有生成任务已完成',
          ),
        ),
      );
    }
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < tasks.length; i++) ...[
              _buildTaskItem(context, tasks[i]),
              if (i < tasks.length - 1) Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTaskItem(BuildContext context, PetTask task) {
    return GestureDetector(
      onTap: () => context.push(AppRoutes.petProcessing(task.id)),
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(task.name, style: AppTypography.body(context)),
                ),
                AmitiaStatusBadge(label: _statusLabel(task.status), type: _statusBadgeType(task.status)),
              ],
            ),
            const SizedBox(height: AppSpacing.xs),
            Row(
              children: [
                Text(
                  '${task.completedActions}/${task.totalActions} 动作',
                  style: AppTypography.caption(context),
                ),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  child: AmitiaProgressBar(progress: task.progress / 100.0),
                ),
                const SizedBox(width: AppSpacing.sm),
                Text('${task.progress}%', style: AppTypography.caption(context)),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildRecentRecords(BuildContext context, List<PetTask> records) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < records.length; i++) ...[
              _buildRecordItem(context, records[i]),
              if (i < records.length - 1) Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildRecordItem(BuildContext context, PetTask task) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brExtraSmall,
            ),
            child: Icon(Icons.pets_outlined, size: 18, color: context.accentPrimary),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(task.name, style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(
                  '${task.characterName} · ${task.createdAt.month}/${task.createdAt.day}',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
          AmitiaStatusBadge(label: _statusLabel(task.status), type: _statusBadgeType(task.status)),
        ],
      ),
    );
  }

  String _statusLabel(PetTaskStatus status) {
    switch (status) {
      case PetTaskStatus.pending:
        return '待处理';
      case PetTaskStatus.processing:
        return '处理中';
      case PetTaskStatus.completed:
        return '已完成';
      case PetTaskStatus.cancelled:
        return '已取消';
    }
  }

  BadgeType _statusBadgeType(PetTaskStatus status) {
    switch (status) {
      case PetTaskStatus.pending:
        return BadgeType.neutral;
      case PetTaskStatus.processing:
        return BadgeType.accent;
      case PetTaskStatus.completed:
        return BadgeType.success;
      case PetTaskStatus.cancelled:
        return BadgeType.error;
    }
  }
}
