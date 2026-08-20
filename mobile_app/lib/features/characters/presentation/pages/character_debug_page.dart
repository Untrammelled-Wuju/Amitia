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

class CharacterDebugPage extends ConsumerWidget {
  final String characterId;

  const CharacterDebugPage({super.key, required this.characterId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final companionAsync = ref.watch(companionStateProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '调试模式',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.warning_amber,
            color: context.warning,
            onPressed: () {
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('开发者模式'), duration: Duration(seconds: 2)),
              );
            },
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildWarningBanner(context),
            SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '角色状态'),
            SizedBox(height: AppSpacing.sm),
            companionAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) => Text('加载失败: $err', style: AppTypography.bodySmall(context)),
              data: (state) {
                if (state == null) {
                  return Text('暂无状态数据', style: AppTypography.caption(context));
                }
                return Column(
                  children: [
                    _buildStatusRow(context, '状态', state['state']?.toString() ?? '-', Icons.info_outline),
                    _buildStatusRow(context, '睡眠中', (state['isSleeping'] == true) ? '是' : '否', Icons.bedtime_outlined),
                    _buildStatusRow(context, '当前活动', state['currentActivity']?.toString() ?? '-', Icons.play_circle_outline),
                    _buildStatusRow(context, '下次活动', state['nextActivity']?.toString() ?? '-', Icons.schedule),
                    _buildStatusRow(context, '醒来时间', state['wakeTime']?.toString() ?? '-', Icons.wb_sunny_outlined),
                    _buildStatusRow(context, '睡眠时间', state['sleepTime']?.toString() ?? '-', Icons.nights_stay_outlined),
                  ],
                );
              },
            ),
            SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(title: '调试操作'),
            SizedBox(height: AppSpacing.sm),
            _buildDebugAction(
              context,
              '测试角色对话',
              '发送测试消息验证角色响应',
              Icons.chat_bubble_outline,
              context.accentPrimary,
              () => _testCharacter(context, ref),
            ),
            _buildDebugAction(
              context,
              '重新生成今日作息',
              '根据当前时间和规则重新生成今日作息计划',
              Icons.refresh,
              context.accentPrimary,
              () => _regenerateSchedule(context, ref),
            ),
            _buildDebugAction(
              context,
              '重置所有状态',
              '重置角色的运行状态',
              Icons.restart_alt,
              context.warning,
              () => _showResetConfirm(context, ref),
            ),
            SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildStatusRow(BuildContext context, String label, String value, IconData icon) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Icon(icon, size: 20, color: context.accentPrimary),
            SizedBox(width: AppSpacing.md),
            Text(label, style: AppTypography.body(context)),
            const Spacer(),
            Text(value, style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary)),
          ],
        ),
      ),
    );
  }

  Widget _buildWarningBanner(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: context.warning.withValues(alpha: 0.08),
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.warning.withValues(alpha: 0.3), width: 1),
      ),
      child: Row(
        children: [
          Icon(Icons.warning_amber_rounded, size: 24, color: context.warning),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('开发者模式', style: AppTypography.cardTitle(context).copyWith(color: context.warning)),
                const SizedBox(height: 2),
                Text('以下操作将直接影响角色运行状态，请谨慎操作。所有操作需二次确认。', style: AppTypography.caption(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildDebugAction(
    BuildContext context,
    String title,
    String description,
    IconData icon,
    Color color,
    VoidCallback onTap,
  ) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: color.withValues(alpha: 0.12),
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 22, color: color),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(description, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
          ],
        ),
        onTap: onTap,
      ),
    );
  }

  Future<void> _testCharacter(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '测试角色对话', '将发送一条测试消息验证角色响应。确定继续吗？');
    if (confirmed != true) return;
    try {
      final svc = ref.read(characterDetailServiceProvider);
      final result = await svc.test(characterId);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('测试成功: ${result?.name ?? characterId}')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('测试失败: $e')),
        );
      }
    }
  }

  Future<void> _regenerateSchedule(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '重新生成今日作息', '这将覆盖当前的作息计划。确定继续吗？');
    if (confirmed != true) return;
    try {
      final svc = ref.read(companionServiceProvider);
      await svc.regenerateSchedule();
      ref.invalidate(companionStateProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('已重新生成今日作息')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: $e')),
        );
      }
    }
  }

  Future<void> _showResetConfirm(BuildContext context, WidgetRef ref) async {
    final confirmed = await _showConfirmDialog(context, '重置所有状态', '将重置角色所有运行状态。此操作不可撤销，确定继续吗？');
    if (confirmed != true) return;
    try {
      final svc = ref.read(companionServiceProvider);
      await svc.regenerateAll();
      ref.invalidate(companionStateProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('已重置所有状态')),
        );
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: $e')),
        );
      }
    }
  }

  Future<bool?> _showConfirmDialog(BuildContext context, String title, String message) {
    return showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.warning_amber, color: context.warning, size: 22),
            SizedBox(width: AppSpacing.sm),
            Text(title, style: AppTypography.cardTitle(context)),
          ],
        ),
        content: Text(message, style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text('确认执行', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }
}
