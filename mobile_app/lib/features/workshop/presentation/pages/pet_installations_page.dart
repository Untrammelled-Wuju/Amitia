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

class PetInstallationsPage extends ConsumerStatefulWidget {
  const PetInstallationsPage({super.key});

  @override
  ConsumerState<PetInstallationsPage> createState() => _PetInstallationsPageState();
}

class _PetInstallationsPageState extends ConsumerState<PetInstallationsPage> {
  List<Map<String, dynamic>> _installations = [];
  List<Map<String, dynamic>> _availableTasks = [];
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
      final results = await Future.wait([
        svc.plugins(),
        svc.workshopSessions(),
      ]);
      if (mounted) {
        setState(() {
          _installations = List<Map<String, dynamic>>.from(results[0]);
          _availableTasks = List<Map<String, dynamic>>.from(results[1]);
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
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
              onPressed: _loading ? null : _showInstallDialog,
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
    if (_installations.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.install_desktop,
        title: '暂无已安装桌宠',
        subtitle: '点击右上角安装新桌宠',
        actionText: '安装桌宠',
        onAction: _showInstallDialog,
      );
    }
    return ListView.builder(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      itemCount: _installations.length,
      itemBuilder: (context, index) => _buildInstallationCard(context, _installations[index]),
    );
  }

  Widget _buildInstallationCard(BuildContext context, Map<String, dynamic> pet) {
    final name = pet['name']?.toString() ?? '';
    final characterName = pet['character']?.toString() ?? pet['characterName']?.toString() ?? '';
    final scale = (pet['scale'] is num) ? (pet['scale'] as num).toDouble() : 1.0;
    final isEnabled = pet['enabled'] == true;
    final isRunning = pet['isRunning'] == true || (isEnabled && pet['status'] == 'active');
    final defaultAction = pet['defaultAction']?.toString() ?? 'idle';
    final actions = pet['actions'] is List ? (pet['actions'] as List).map((e) => e.toString()).toList() : <String>[];
    final id = pet['id']?.toString() ?? '';
    final pluginId = pet['pluginId']?.toString() ?? id;

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
                      characterName.isNotEmpty ? characterName.substring(0, 1) : '?',
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
                      Text(name, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(
                        '$characterName · 缩放 ${(scale * 100).round()}%',
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                if (isRunning)
                  AmitiaStatusBadge(label: '运行中', type: BadgeType.success)
                else if (isEnabled)
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
                  Text(_actionLabel(defaultAction), style: AppTypography.bodySmall(context)),
                  const SizedBox(width: AppSpacing.xs),
                  GestureDetector(
                    onTap: () => _showDefaultActionSheet(id, defaultAction, actions),
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
              children: actions.map((action) {
                final a = action.toString();
                return Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: a == defaultAction ? context.accentSoft : context.surfaceSecondary,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(
                    _actionLabel(a),
                    style: TextStyle(
                      fontSize: 12,
                      color: a == defaultAction ? context.accentPrimary : context.textSecondary,
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
                    label: isEnabled ? '停用' : '启用',
                    isSecondary: true,
                    height: 38,
                    icon: isEnabled ? Icons.pause_circle_outline : Icons.play_circle_outline,
                    onPressed: () => _toggleEnabled(id, isEnabled),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '调整大小',
                    isSecondary: true,
                    height: 38,
                    icon: Icons.aspect_ratio,
                    onPressed: () => _showResizeDialog(id, scale),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '动作',
                    isSecondary: true,
                    height: 38,
                    icon: Icons.list,
                    onPressed: () => _showActionListSheet(id, defaultAction, actions),
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
              onPressed: () => _showUninstallConfirm(id, pluginId),
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

  Future<void> _toggleEnabled(String id, bool currentlyEnabled) async {
    if (currentlyEnabled) {
      _showDisableConfirm(id);
    } else {
      try {
        final svc = ref.read(extensionServiceProvider);
        await svc.enablePlugin(id);
        await _load();
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('已启用')),
          );
        }
      } catch (e) {
        if (mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('启用失败: $e')),
          );
        }
      }
    }
  }

  void _showDisableConfirm(String id) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('停用桌宠', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认停用该桌宠？停用后桌宠将不再显示在桌面上。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                try {
                  final svc = ref.read(extensionServiceProvider);
                  await svc.disablePlugin(id);
                  await _load();
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('已停用')),
                    );
                  }
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('停用失败: $e')),
                    );
                  }
                }
              },
              child: Text('停用', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }

  void _showResizeDialog(String id, double currentScale) {
    double tempScale = currentScale;
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
                  Text('当前缩放：${(tempScale * 100).round()}%', style: AppTypography.body(context)),
                  const SizedBox(height: AppSpacing.md),
                  Slider(
                    value: tempScale,
                    min: 0.5,
                    max: 2.0,
                    divisions: 15,
                    activeColor: context.accentPrimary,
                    onChanged: (value) {
                      setDialogState(() { tempScale = value; });
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
                  onPressed: () async {
                    Navigator.pop(dialogContext);
                    _updateScale(id, tempScale);
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

  Future<void> _updateScale(String id, double scale) async {
    try {
      final svc = ref.read(extensionServiceProvider);
      await svc.updatePluginConfig(id, {'scale': scale});
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('缩放已调整为 ${(scale * 100).round()}%')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('调整失败: $e')),
        );
      }
    }
  }

  void _showActionListSheet(String id, String defaultAction, List<String> actions) {
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
                  child: Text('动作列表', style: AppTypography.sectionTitle(context)),
                ),
                const SizedBox(height: AppSpacing.md),
                ...actions.map((action) {
                  final a = action.toString();
                  return AmitiaListTile(
                    leading: Icon(
                      a == defaultAction ? Icons.star : Icons.play_circle_outline,
                      size: 22,
                      color: a == defaultAction ? context.accentPrimary : context.textSecondary,
                    ),
                    title: _actionLabel(a),
                    subtitle: a == defaultAction ? '当前默认动作' : '点击设为默认',
                    trailing: a == defaultAction
                        ? Icon(Icons.check, size: 20, color: context.accentPrimary)
                        : null,
                    onTap: () {
                      Navigator.pop(sheetContext);
                      _setDefaultAction(id, a);
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

  void _showDefaultActionSheet(String id, String currentDefault, List<String> actions) {
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
                ...actions.map((action) {
                  final a = action.toString();
                  final isSelected = a == currentDefault;
                  return GestureDetector(
                    onTap: () {
                      Navigator.pop(sheetContext);
                      _setDefaultAction(id, a);
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
                            child: Text(_actionLabel(a), style: AppTypography.body(context)),
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

  Future<void> _setDefaultAction(String id, String action) async {
    try {
      final svc = ref.read(extensionServiceProvider);
      await svc.updatePluginConfig(id, {'defaultAction': action});
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('默认动作已更换为「${_actionLabel(action)}」')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('更换失败: $e')),
        );
      }
    }
  }

  void _showUninstallConfirm(String id, String pluginId) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('卸载桌宠', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认卸载该桌宠？卸载后所有动作和配置将被删除，无法恢复。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                try {
                  final svc = ref.read(extensionServiceProvider);
                  await svc.disablePlugin(id);
                  await _load();
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('已卸载')),
                    );
                  }
                } catch (e) {
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text('卸载失败: $e')),
                    );
                  }
                }
              },
              child: Text('卸载', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showInstallDialog() {
    String? selectedTask;
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
                  ..._availableTasks.map((task) {
                    final taskName = task['name']?.toString() ?? '';
                    final isInstalled = _installations.any((p) => p['name']?.toString() == taskName);
                    final isSelected = taskName == selectedTask;
                    return GestureDetector(
                      onTap: isInstalled
                          ? null
                          : () {
                              setDialogState(() { selectedTask = taskName; });
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
                                    taskName,
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
                  onPressed: selectedTask == null
                      ? null
                      : () {
                          Navigator.pop(dialogContext);
                          _installPet(selectedTask!);
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

  Future<void> _installPet(String taskName) async {
    try {
      final svc = ref.read(extensionServiceProvider);
      await svc.enablePlugin(taskName);
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('「$taskName」安装成功')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('安装失败: $e')),
        );
      }
    }
  }
}
