import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/center_navigation.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../sections/desktop_pet_plugin_section.dart';

class _PetAction {
  final String name;
  final String trigger;
  bool enabled;
  _PetAction({required this.name, required this.trigger, this.enabled = true});
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

  final _petColors = ['#7668EE', '#52B788', '#E9A23B', '#6C8FEA'];

  final List<_PetAction> _actions = [
    _PetAction(name: '待机', trigger: '无操作 5 秒'),
    _PetAction(name: '招手', trigger: '用户进入'),
    _PetAction(name: '开心', trigger: '收到消息'),
    _PetAction(name: '说话', trigger: '语音交互'),
    _PetAction(name: '吃饭', trigger: '定时 12:00'),
    _PetAction(name: '睡觉', trigger: '定时 23:00'),
  ];

  static const _defaultPetNames = ['Amitia桌宠', '小雨桌宠', '自定义桌宠'];

  @override
  Widget build(BuildContext context) {
    final companionAsync = ref.watch(companionStateProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠中心',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: companionAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (_, __) => _buildStaticContent(context),
          data: (state) {
            final petName = state?['currentActivity']?.toString() ?? _defaultPetNames[_currentPetIndex % _defaultPetNames.length];
            return _buildContentWithState(context, petName);
          },
        ),
      ),
    );
  }

  Widget _buildStaticContent(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
      children: [
        const SizedBox(height: AppSpacing.sm),
        _buildCurrentPetCard(context, _defaultPetNames[_currentPetIndex % _defaultPetNames.length]),
        const SizedBox(height: AppSpacing.sm),
        _buildActionEntry(context),
        const SizedBox(height: AppSpacing.sectionGap),
        const DesktopPetPluginSection(),
        const SizedBox(height: AppSpacing.sectionGap),
        const AmitiaSectionHeader(title: '显示设置'),
        const SizedBox(height: AppSpacing.sm),
        _buildSettingsCard(context),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaCard(
            onTap: () => CenterNavigation.openExtensionCenter(context),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.extension_outlined, size: 18, color: context.accentPrimary),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('扩展中心', style: AppTypography.label(context)),
                      Text('管理所有扩展', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, size: 16, color: context.textSecondary),
              ],
            ),
          ),
        ),
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
    );
  }

  Widget _buildContentWithState(BuildContext context, String petName) {
    return ListView(
      padding: const EdgeInsets.only(bottom: AppSpacing.xxl),
      children: [
        const SizedBox(height: AppSpacing.sm),
        _buildCurrentPetCard(context, petName),
        const SizedBox(height: AppSpacing.sm),
        _buildActionEntry(context),
        const SizedBox(height: AppSpacing.sectionGap),
        const DesktopPetPluginSection(),
        const SizedBox(height: AppSpacing.sectionGap),
        const AmitiaSectionHeader(title: '显示设置'),
        const SizedBox(height: AppSpacing.sm),
        _buildSettingsCard(context),
        const SizedBox(height: AppSpacing.sectionGap),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: AmitiaCard(
            onTap: () => CenterNavigation.openExtensionCenter(context),
            child: Row(
              children: [
                Container(
                  width: 32,
                  height: 32,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.extension_outlined, size: 18, color: context.accentPrimary),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('扩展中心', style: AppTypography.label(context)),
                      Text('管理所有扩展', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Icon(Icons.chevron_right, size: 16, color: context.textSecondary),
              ],
            ),
          ),
        ),
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
    );
  }

  Widget _buildCurrentPetCard(BuildContext context, String petName) {
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
                    Text('当前桌宠: ${_defaultPetNames[_currentPetIndex % _defaultPetNames.length]}', style: AppTypography.caption(ctx)),
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
