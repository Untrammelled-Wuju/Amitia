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

class DesktopContributionsPage extends ConsumerStatefulWidget {
  const DesktopContributionsPage({super.key});

  @override
  ConsumerState<DesktopContributionsPage> createState() => _DesktopContributionsPageState();
}

class _DesktopContributionsPageState extends ConsumerState<DesktopContributionsPage> {
  late List<DesktopContribution> _contributions;

  @override
  void initState() {
    super.initState();
    _contributions = List.from(MockKernel.desktopContributions);
  }

  @override
  Widget build(BuildContext context) {
    final shortcuts = _contributions.where((c) => c.type == '快捷键').toList();
    final menus = _contributions.where((c) => c.type == '菜单').toList();
    final windows = _contributions.where((c) => c.type == '窗口').toList();
    final trays = _contributions.where((c) => c.type == '托盘').toList();

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '桌面贡献中心',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
          children: [
            _buildConflictWarning(context),
            if (shortcuts.isNotEmpty) ...[
              const AmitiaSectionHeader(title: '快捷键'),
              const SizedBox(height: AppSpacing.sm),
              ...shortcuts.map((c) => _buildContributionCard(context, c)),
              const SizedBox(height: AppSpacing.xl),
            ],
            if (menus.isNotEmpty) ...[
              const AmitiaSectionHeader(title: '菜单贡献'),
              const SizedBox(height: AppSpacing.sm),
              ...menus.map((c) => _buildContributionCard(context, c)),
              const SizedBox(height: AppSpacing.xl),
            ],
            if (windows.isNotEmpty) ...[
              const AmitiaSectionHeader(title: '桌面窗口'),
              const SizedBox(height: AppSpacing.sm),
              ...windows.map((c) => _buildContributionCard(context, c)),
              const SizedBox(height: AppSpacing.xl),
            ],
            if (trays.isNotEmpty) ...[
              const AmitiaSectionHeader(title: '托盘贡献'),
              const SizedBox(height: AppSpacing.sm),
              ...trays.map((c) => _buildContributionCard(context, c)),
            ],
            const SizedBox(height: AppSpacing.xl),
          ],
        ),
      ),
    );
  }

  Widget _buildConflictWarning(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        backgroundColor: context.warning.withValues(alpha: 0.06),
        border: Border.all(color: context.warning.withValues(alpha: 0.3), width: 0.5),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.warning_amber_rounded, size: 20, color: context.warning),
            const SizedBox(width: 8),
            Expanded(
              child: Text('检测到 1 个快捷键冲突：Ctrl+Shift+A 与系统快捷键冲突，建议修改。', style: AppTypography.caption(context).copyWith(color: context.warning)),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildContributionCard(BuildContext context, DesktopContribution contribution) {
    final isShortcut = contribution.type == '快捷键';
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(_getIcon(contribution.type), size: 22, color: context.accentPrimary),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(contribution.label, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(contribution.type, style: AppTypography.label(context)),
                ],
              ),
            ),
            if (isShortcut) ...[
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brTag,
                  border: Border.all(color: context.borderPrimary, width: 0.5),
                ),
                child: Text(
                  contribution.value,
                  style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace', fontSize: 13),
                ),
              ),
              const SizedBox(width: 8),
              AmitiaIconButton(
                icon: Icons.edit_outlined,
                onPressed: () => _showEditShortcutDialog(context, contribution),
                color: context.accentPrimary,
                tooltip: '修改快捷键',
              ),
            ] else
              AmitiaStatusBadge(
                label: contribution.value,
                type: BadgeType.accent,
              ),
          ],
        ),
      ),
    );
  }

  IconData _getIcon(String type) {
    switch (type) {
      case '快捷键':
        return Icons.keyboard_outlined;
      case '菜单':
        return Icons.menu_outlined;
      case '窗口':
        return Icons.desktop_windows_outlined;
      case '托盘':
        return Icons.apps_outlined;
      default:
        return Icons.widgets_outlined;
    }
  }

  void _showEditShortcutDialog(BuildContext context, DesktopContribution contribution) {
    final controller = TextEditingController(text: contribution.value);
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('修改快捷键', style: AppTypography.cardTitle(context)),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(contribution.label, style: AppTypography.caption(context)),
              const SizedBox(height: 12),
              Text('当前快捷键', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              const SizedBox(height: 4),
              Text(contribution.value, style: AppTypography.body(context).copyWith(fontFamily: 'monospace')),
              const SizedBox(height: 16),
              Text('新快捷键', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
              const SizedBox(height: 6),
              AmitiaTextField(
                hintText: '例如: Ctrl+Shift+B',
                controller: controller,
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
                final newValue = controller.text.trim();
                if (newValue.isEmpty) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请输入新快捷键')));
                  return;
                }
                setState(() {
                  final idx = _contributions.indexWhere((c) => c.id == contribution.id);
                  if (idx >= 0) {
                    _contributions[idx] = DesktopContribution(
                      id: contribution.id,
                      type: contribution.type,
                      label: contribution.label,
                      value: newValue,
                    );
                  }
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已修改快捷键：${contribution.label} -> $newValue')));
              },
              child: Text('保存', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }
}
