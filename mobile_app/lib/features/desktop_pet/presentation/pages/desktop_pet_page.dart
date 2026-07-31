import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/mock_data/mock_data.dart';

class _MockPetAction {
  final String name;
  final String trigger;
  bool enabled;
  _MockPetAction({required this.name, required this.trigger, this.enabled = true});
}

class DesktopPetPage extends ConsumerStatefulWidget {
  const DesktopPetPage({super.key});

  @override
  ConsumerState<DesktopPetPage> createState() => _DesktopPetPageState();
}

class _DesktopPetPageState extends ConsumerState<DesktopPetPage> {
  bool _floatingWindow = true;
  double _transparency = 0.85;
  int _currentPetIndex = 0;

  final _petColors = ['#7668EE', '#52B788', '#E9A23B'];

  final List<_MockPetAction> _actions = [
    _MockPetAction(name: '待机', trigger: '无操作 5 秒'),
    _MockPetAction(name: '招手', trigger: '用户进入'),
    _MockPetAction(name: '开心', trigger: '收到消息'),
    _MockPetAction(name: '说话', trigger: '语音交互'),
    _MockPetAction(name: '吃饭', trigger: '定时 12:00'),
    _MockPetAction(name: '睡觉', trigger: '定时 23:00'),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠中心',
        navigation: AmitiaAppBarNavigation.drawer,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
          children: [
            const SizedBox(height: AppSpacing.sm),
            _buildCurrentPetCard(context),
            const SizedBox(height: AppSpacing.sm),
            _buildActionEntry(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '已安装桌宠插件'),
            const SizedBox(height: AppSpacing.sm),
            _buildPluginsCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '显示设置'),
            const SizedBox(height: AppSpacing.sm),
            _buildSettingsCard(context),
            const SizedBox(height: AppSpacing.sectionGap),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: AmitiaButton(
                label: '生成新桌宠',
                icon: Icons.auto_awesome_outlined,
                isFullWidth: true,
                onPressed: () => context.go(AppRoutes.workshopPetCreate),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCurrentPetCard(BuildContext context) {
    final petName = MockData.desktopPetPlugins[_currentPetIndex];
    final color = _parseColor(_petColors[_currentPetIndex % _petColors.length]);
    final initial = _getInitial(petName);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: color,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  initial,
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
                  Text(petName, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text('运行中 · 心情很好', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaStatusBadge(label: '运行中', type: BadgeType.success),
          ],
        ),
      ),
    );
  }

  Widget _buildActionEntry(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        onTap: () => _showActionManagementSheet(context),
        child: Row(
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brExtraSmall,
              ),
              child: Icon(Icons.touch_app_outlined, size: 18, color: context.accentPrimary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('动作管理', style: AppTypography.body(context)),
                  const SizedBox(height: 2),
                  Text('${_actions.where((a) => a.enabled).length} 个动作已启用', style: AppTypography.label(context)),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showActionManagementSheet(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return StatefulBuilder(
          builder: (ctx, setSheetState) {
            return SafeArea(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const SizedBox(height: 8),
                    Center(
                      child: Container(
                        width: 40,
                        height: 4,
                        decoration: BoxDecoration(
                          color: ctx.borderPrimary,
                          borderRadius: BorderRadius.circular(2),
                        ),
                      ),
                    ),
                    const SizedBox(height: 20),
                    Text('动作管理', style: AppTypography.pageTitle(ctx)),
                    const SizedBox(height: 4),
                    Text('当前桌宠: ${MockData.desktopPetPlugins[_currentPetIndex]}', style: AppTypography.caption(ctx)),
                    const SizedBox(height: 16),
                    Flexible(
                      child: ListView.builder(
                        shrinkWrap: true,
                        itemCount: _actions.length,
                        itemBuilder: (_, index) {
                          final action = _actions[index];
                          return Container(
                            margin: const EdgeInsets.only(bottom: 8),
                            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                            decoration: BoxDecoration(
                              color: ctx.surfaceSecondary,
                              borderRadius: AppRadius.brSmall,
                            ),
                            child: Row(
                              children: [
                                Container(
                                  width: 32,
                                  height: 32,
                                  decoration: BoxDecoration(
                                    color: action.enabled ? ctx.accentSoft : ctx.borderPrimary,
                                    borderRadius: AppRadius.brExtraSmall,
                                  ),
                                  child: Icon(
                                    Icons.play_circle_outline,
                                    size: 18,
                                    color: action.enabled ? ctx.accentPrimary : ctx.textTertiary,
                                  ),
                                ),
                                const SizedBox(width: 10),
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment: CrossAxisAlignment.start,
                                    children: [
                                      Text(action.name, style: AppTypography.bodySmall(ctx).copyWith(
                                        color: action.enabled ? ctx.textPrimary : ctx.textTertiary,
                                      )),
                                      const SizedBox(height: 2),
                                      Text('触发: ${action.trigger}', style: AppTypography.label(ctx)),
                                    ],
                                  ),
                                ),
                                GestureDetector(
                                  onTap: () {
                                    setSheetState(() {
                                      action.enabled = !action.enabled;
                                    });
                                  },
                                  child: Container(
                                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                                    decoration: BoxDecoration(
                                      color: action.enabled ? ctx.accentPrimary : ctx.borderPrimary,
                                      borderRadius: AppRadius.brTag,
                                    ),
                                    child: Text(
                                      action.enabled ? '已启用' : '已停用',
                                      style: TextStyle(
                                        fontSize: 11,
                                        color: action.enabled ? Colors.white : ctx.textTertiary,
                                        fontWeight: FontWeight.w500,
                                      ),
                                    ),
                                  ),
                                ),
                                const SizedBox(width: 4),
                                GestureDetector(
                                  onTap: () => amitiaSnackBar(ctx, '预览动作: ${action.name}'),
                                  child: Padding(
                                    padding: const EdgeInsets.all(4),
                                    child: Icon(Icons.visibility_outlined, size: 18, color: ctx.textSecondary),
                                  ),
                                ),
                                if (index > 0)
                                  GestureDetector(
                                    onTap: () {
                                      setSheetState(() {
                                        final temp = _actions[index];
                                        _actions[index] = _actions[index - 1];
                                        _actions[index - 1] = temp;
                                      });
                                    },
                                    child: Padding(
                                      padding: const EdgeInsets.all(4),
                                      child: Icon(Icons.arrow_upward, size: 16, color: ctx.textSecondary),
                                    ),
                                  ),
                                if (index < _actions.length - 1)
                                  GestureDetector(
                                    onTap: () {
                                      setSheetState(() {
                                        final temp = _actions[index];
                                        _actions[index] = _actions[index + 1];
                                        _actions[index + 1] = temp;
                                      });
                                    },
                                    child: Padding(
                                      padding: const EdgeInsets.all(4),
                                      child: Icon(Icons.arrow_downward, size: 16, color: ctx.textSecondary),
                                    ),
                                  ),
                              ],
                            ),
                          );
                        },
                      ),
                    ),
                    const SizedBox(height: 12),
                    AmitiaButton(
                      label: '保存动作配置',
                      icon: Icons.save_outlined,
                      isFullWidth: true,
                      onPressed: () {
                        Navigator.pop(ctx);
                        setState(() {});
                        amitiaSnackBar(context, '动作配置已保存');
                      },
                    ),
                  ],
                ),
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildPluginsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < MockData.desktopPetPlugins.length; i++) ...[
              _buildPluginItem(context, MockData.desktopPetPlugins[i], i),
              if (i < MockData.desktopPetPlugins.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildPluginItem(BuildContext context, String name, int index) {
    final colorHex = _petColors[index % _petColors.length];
    final color = _parseColor(colorHex);
    final initial = _getInitial(name);
    final isCurrent = index == _currentPetIndex;

    return GestureDetector(
      behavior: HitTestBehavior.opaque,
      onTap: () => _showPluginManagementSheet(context, name, index),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
        child: Row(
          children: [
            Container(
              width: 36,
              height: 36,
              decoration: BoxDecoration(
                color: color,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  initial,
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Text(name, style: AppTypography.body(context)),
            ),
            if (isCurrent)
              const AmitiaStatusBadge(label: '当前', type: BadgeType.accent)
            else
              Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  void _showPluginManagementSheet(BuildContext context, String name, int index) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text(name, style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: Icon(Icons.info_outline, size: 20, color: context.accentPrimary),
                  title: '查看详情',
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    amitiaSnackBar(context, '$name · 桌宠插件 v1.0.0');
                  },
                ),
                if (index != _currentPetIndex)
                  AmitiaListTile(
                    leading: Icon(Icons.star_outline, size: 20, color: context.accentPrimary),
                    title: '设为当前桌宠',
                    onTap: () {
                      Navigator.pop(sheetCtx);
                      setState(() {
                        _currentPetIndex = index;
                      });
                      amitiaSnackBar(context, '已切换到 $name');
                    },
                  ),
                AmitiaListTile(
                  leading: Icon(Icons.visibility_outlined, size: 20, color: context.accentPrimary),
                  title: '预览',
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    amitiaSnackBar(context, '正在预览 $name');
                  },
                ),
                AmitiaListTile(
                  leading: Icon(Icons.delete_outline, size: 20, color: context.error),
                  title: '卸载',
                  onTap: () {
                    Navigator.pop(sheetCtx);
                    _confirmUninstall(context, name, index);
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  void _confirmUninstall(BuildContext context, String name, int index) {
    showAmitiaConfirmDialog(
      context,
      title: '卸载桌宠插件',
      message: '确定要卸载 $name 吗？卸载后相关数据将被清除。',
      confirmLabel: '卸载',
      isDestructive: true,
    ).then((confirmed) {
      if (confirmed == true) {
        setState(() {
          MockData.desktopPetPlugins.removeAt(index);
          if (_currentPetIndex >= MockData.desktopPetPlugins.length) {
            _currentPetIndex = 0;
          }
        });
        amitiaSnackBar(context, '$name 已卸载');
      }
    });
  }

  Widget _buildSettingsCard(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('悬浮窗显示', style: AppTypography.body(context)),
                      const SizedBox(height: 2),
                      Text('在其他应用上方显示桌宠', style: AppTypography.label(context)),
                    ],
                  ),
                ),
                Switch(
                  value: _floatingWindow,
                  onChanged: (value) {
                    setState(() {
                      _floatingWindow = value;
                    });
                  },
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Expanded(
                  child: Text('透明度', style: AppTypography.body(context)),
                ),
                Text(
                  '${(_transparency * 100).round()}%',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
            Slider(
              value: _transparency,
              min: 0.2,
              max: 1.0,
              divisions: 8,
              activeColor: context.accentPrimary,
              onChanged: (value) {
                setState(() {
                  _transparency = value;
                });
              },
            ),
          ],
        ),
      ),
    );
  }

  Color _parseColor(String hex) {
    final cleaned = hex.replaceAll('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  }

  String _getInitial(String name) {
    return name.isNotEmpty ? name.substring(0, 1) : '?';
  }
}
