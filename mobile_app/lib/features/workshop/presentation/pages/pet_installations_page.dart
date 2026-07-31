import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class PetInstallationsPage extends ConsumerStatefulWidget {
  const PetInstallationsPage({super.key});

  @override
  ConsumerState<PetInstallationsPage> createState() => _PetInstallationsPageState();
}

class _PetInstallationsPageState extends ConsumerState<PetInstallationsPage> {
  late List<PetInstallation> _installations;

  @override
  void initState() {
    super.initState();
    _installations = List.from(MockWorkshop.installations);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '安装管理',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          Padding(
            padding: const EdgeInsets.only(right: AppSpacing.sm),
            child: AmitiaIconButton(
              icon: Icons.add,
              color: context.accentPrimary,
              onPressed: _showInstallDialog,
            ),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _installations.isEmpty
            ? AmitiaEmptyState(
                icon: Icons.install_desktop,
                title: '暂无已安装桌宠',
                subtitle: '点击右上角安装新桌宠',
                actionText: '安装桌宠',
                onAction: _showInstallDialog,
              )
            : ListView.builder(
                padding: const EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: _installations.length,
                itemBuilder: (context, index) => _buildInstallationCard(context, _installations[index]),
              ),
      ),
    );
  }

  Widget _buildInstallationCard(BuildContext context, PetInstallation pet) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
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
                    color: context.accentPrimary,
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: Text(
                      pet.characterName.substring(0, 1),
                      style: const TextStyle(
                        color: Colors.white,
                        fontSize: 18,
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
                        '${pet.characterName} · 缩放 ${(pet.scale * 100).round()}%',
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                if (pet.isRunning)
                  AmitiaStatusBadge(label: '运行中', type: BadgeType.success)
                else if (pet.isEnabled)
                  AmitiaStatusBadge(label: '已启用', type: BadgeType.accent)
                else
                  AmitiaStatusBadge(label: '已停用', type: BadgeType.neutral),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Container(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Row(
                children: [
                  Icon(Icons.play_circle_outline, size: 16, color: context.textSecondary),
                  const SizedBox(width: AppSpacing.xs),
                  Text('默认动作', style: AppTypography.label(context)),
                  const Spacer(),
                  Text(_actionLabel(pet.defaultAction), style: AppTypography.bodySmall(context)),
                  const SizedBox(width: AppSpacing.xs),
                  GestureDetector(
                    onTap: () => _showDefaultActionSheet(pet),
                    child: Text(
                      '更换',
                      style: TextStyle(fontSize: 13, color: context.accentPrimary, fontWeight: FontWeight.w500),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.xs,
              runSpacing: AppSpacing.xs,
              children: pet.actions.map((action) {
                return Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: action == pet.defaultAction ? context.accentSoft : context.surfaceSecondary,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(
                    _actionLabel(action),
                    style: TextStyle(
                      fontSize: 12,
                      color: action == pet.defaultAction ? context.accentPrimary : context.textSecondary,
                    ),
                  ),
                );
              }).toList(),
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: pet.isEnabled ? '停用' : '启用',
                    isSecondary: true,
                    height: 38,
                    icon: pet.isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline,
                    onPressed: () => _toggleEnabled(pet),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '调整大小',
                    isSecondary: true,
                    height: 38,
                    icon: Icons.aspect_ratio,
                    onPressed: () => _showResizeDialog(pet),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '动作',
                    isSecondary: true,
                    height: 38,
                    icon: Icons.list,
                    onPressed: () => _showActionListSheet(pet),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.sm),
            AmitiaButton(
              label: '卸载桌宠',
              isDestructive: true,
              isFullWidth: true,
              icon: Icons.delete_outline,
              height: 38,
              onPressed: () => _showUninstallConfirm(pet),
            ),
          ],
        ),
      ),
    );
  }

  String _actionLabel(String action) {
    final labels = {
      'idle': '待机',
      'wave': '招手',
      'happy': '开心',
      'speaking': '说话',
      'thinking': '思考',
      'sleeping': '睡觉',
    };
    return labels[action] ?? action;
  }

  void _toggleEnabled(PetInstallation pet) {
    if (pet.isEnabled) {
      _showDisableConfirm(pet);
    } else {
      setState(() {
        final idx = _installations.indexWhere((p) => p.id == pet.id);
        if (idx >= 0) {
          _installations[idx] = PetInstallation(
            id: pet.id,
            name: pet.name,
            characterName: pet.characterName,
            isEnabled: true,
            isRunning: true,
            scale: pet.scale,
            defaultAction: pet.defaultAction,
            actions: pet.actions,
          );
        }
      });
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('「${pet.name}」已启用')),
      );
    }
  }

  void _showDisableConfirm(PetInstallation pet) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('停用桌宠', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认停用「${pet.name}」？停用后桌宠将不再显示在桌面上。',
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
                  final idx = _installations.indexWhere((p) => p.id == pet.id);
                  if (idx >= 0) {
                    _installations[idx] = PetInstallation(
                      id: pet.id,
                      name: pet.name,
                      characterName: pet.characterName,
                      isEnabled: false,
                      isRunning: false,
                      scale: pet.scale,
                      defaultAction: pet.defaultAction,
                      actions: pet.actions,
                    );
                  }
                });
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('「${pet.name}」已停用')),
                );
              },
              child: Text('停用', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }

  void _showResizeDialog(PetInstallation pet) {
    double tempScale = pet.scale;
    showDialog(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
              title: Text('调整大小', style: AppTypography.cardTitle(context)),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text('「${pet.name}」当前缩放：${(tempScale * 100).round()}%', style: AppTypography.body(context)),
                  const SizedBox(height: AppSpacing.md),
                  Slider(
                    value: tempScale,
                    min: 0.5,
                    max: 2.0,
                    divisions: 15,
                    activeColor: context.accentPrimary,
                    onChanged: (value) {
                      setDialogState(() {
                        tempScale = value;
                      });
                    },
                  ),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.spaceBetween,
                    children: [
                      Text('50%', style: AppTypography.label(context)),
                      Text('200%', style: AppTypography.label(context)),
                    ],
                  ),
                ],
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
                      final idx = _installations.indexWhere((p) => p.id == pet.id);
                      if (idx >= 0) {
                        _installations[idx] = PetInstallation(
                          id: pet.id,
                          name: pet.name,
                          characterName: pet.characterName,
                          isEnabled: pet.isEnabled,
                          isRunning: pet.isRunning,
                          scale: tempScale,
                          defaultAction: pet.defaultAction,
                          actions: pet.actions,
                        );
                      }
                    });
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('「${pet.name}」缩放已调整为 ${(tempScale * 100).round()}%')),
                    );
                  },
                  child: Text('确认', style: TextStyle(color: context.accentPrimary)),
                ),
              ],
            );
          },
        );
      },
    );
  }

  void _showActionListSheet(PetInstallation pet) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: Text('动作列表 - ${pet.name}', style: AppTypography.sectionTitle(context)),
                ),
                const SizedBox(height: AppSpacing.md),
                ...pet.actions.map((action) {
                  return AmitiaListTile(
                    leading: Icon(
                      action == pet.defaultAction ? Icons.star : Icons.play_circle_outline,
                      size: 22,
                      color: action == pet.defaultAction ? context.accentPrimary : context.textSecondary,
                    ),
                    title: _actionLabel(action),
                    subtitle: action == pet.defaultAction ? '当前默认动作' : '点击设为默认',
                    trailing: action == pet.defaultAction
                        ? Icon(Icons.check, size: 20, color: context.accentPrimary)
                        : null,
                    onTap: () {
                      Navigator.pop(sheetContext);
                      _setDefaultAction(pet, action);
                    },
                  );
                }),
                const SizedBox(height: AppSpacing.sm),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: AmitiaButton(
                    label: '关闭',
                    isSecondary: true,
                    isFullWidth: true,
                    onPressed: () => Navigator.pop(sheetContext),
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showDefaultActionSheet(PetInstallation pet) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(vertical: AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: Text('更换默认待机动作', style: AppTypography.sectionTitle(context)),
                ),
                const SizedBox(height: AppSpacing.md),
                ...pet.actions.map((action) {
                  final isSelected = action == pet.defaultAction;
                  return GestureDetector(
                    onTap: () {
                      Navigator.pop(sheetContext);
                      _setDefaultAction(pet, action);
                    },
                    behavior: HitTestBehavior.opaque,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: 12),
                      child: Row(
                        children: [
                          Icon(
                            isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
                            size: 20,
                            color: isSelected ? context.accentPrimary : context.textTertiary,
                          ),
                          const SizedBox(width: AppSpacing.md),
                          Expanded(
                            child: Text(_actionLabel(action), style: AppTypography.body(context)),
                          ),
                          if (isSelected)
                            Icon(Icons.check, size: 18, color: context.accentPrimary),
                        ],
                      ),
                    ),
                  );
                }),
              ],
            ),
          ),
        );
      },
    );
  }

  void _setDefaultAction(PetInstallation pet, String action) {
    setState(() {
      final idx = _installations.indexWhere((p) => p.id == pet.id);
      if (idx >= 0) {
        _installations[idx] = PetInstallation(
          id: pet.id,
          name: pet.name,
          characterName: pet.characterName,
          isEnabled: pet.isEnabled,
          isRunning: pet.isRunning,
          scale: pet.scale,
          defaultAction: action,
          actions: pet.actions,
        );
      }
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('默认动作已更换为「${_actionLabel(action)}」')),
    );
  }

  void _showUninstallConfirm(PetInstallation pet) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('卸载桌宠', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认卸载「${pet.name}」？卸载后所有动作和配置将被删除，无法恢复。',
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
                  _installations.removeWhere((p) => p.id == pet.id);
                });
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('「${pet.name}」已卸载')),
                );
              },
              child: Text('卸载', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showInstallDialog() {
    final availablePets = MockWorkshop.petTasks.map((t) => t.name).toList();
    String? selectedPet;
    showDialog(
      context: context,
      builder: (dialogContext) {
        return StatefulBuilder(
          builder: (context, setDialogState) {
            return AlertDialog(
              shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
              title: Text('安装桌宠', style: AppTypography.cardTitle(context)),
              content: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('选择要安装的桌宠', style: AppTypography.caption(context)),
                  const SizedBox(height: AppSpacing.sm),
                  ...availablePets.map((name) {
                    final isInstalled = _installations.any((p) => p.name == name);
                    final isSelected = name == selectedPet;
                    return GestureDetector(
                      onTap: isInstalled
                          ? null
                          : () {
                              setDialogState(() {
                                selectedPet = name;
                              });
                            },
                      behavior: HitTestBehavior.opaque,
                      child: Container(
                        padding: const EdgeInsets.symmetric(vertical: 10),
                        child: Row(
                          children: [
                            Icon(
                              isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
                              size: 20,
                              color: isSelected
                                  ? context.accentPrimary
                                  : isInstalled
                                      ? context.textTertiary
                                      : context.textSecondary,
                            ),
                            const SizedBox(width: AppSpacing.md),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(
                                    name,
                                    style: AppTypography.body(context).copyWith(
                                      color: isInstalled ? context.textTertiary : context.textPrimary,
                                    ),
                                  ),
                                  const SizedBox(height: 2),
                                  Text(
                                    isInstalled ? '已安装' : '可安装',
                                    style: AppTypography.label(context),
                                  ),
                                ],
                              ),
                            ),
                          ],
                        ),
                      ),
                    );
                  }),
                ],
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext),
                  child: Text('取消', style: TextStyle(color: context.textSecondary)),
                ),
                TextButton(
                  onPressed: selectedPet == null
                      ? null
                      : () {
                          Navigator.pop(dialogContext);
                          final task = MockWorkshop.petTasks.firstWhere((t) => t.name == selectedPet);
                          setState(() {
                            _installations.add(PetInstallation(
                              id: 'pi${DateTime.now().millisecondsSinceEpoch}',
                              name: task.name,
                              characterName: task.characterName,
                              isEnabled: true,
                              isRunning: false,
                              scale: 1.0,
                              defaultAction: 'idle',
                              actions: const ['idle', 'wave', 'happy', 'speaking'],
                            ));
                          });
                          ScaffoldMessenger.of(context).showSnackBar(
                            SnackBar(content: Text('「$selectedPet」安装成功')),
                          );
                        },
                  child: Text('安装', style: TextStyle(color: context.accentPrimary)),
                ),
              ],
            );
          },
        );
      },
    );
  }
}
