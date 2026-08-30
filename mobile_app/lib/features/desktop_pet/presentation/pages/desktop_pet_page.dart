import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/center_navigation.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../runtime/desktop_pet_mobile_runtime.dart';
import '../sections/desktop_pet_plugin_section.dart';

class DesktopPetPage extends ConsumerStatefulWidget {
  const DesktopPetPage({super.key});

  @override
  ConsumerState<DesktopPetPage> createState() => _DesktopPetPageState();
}

class _DesktopPetPageState extends ConsumerState<DesktopPetPage>
    with WidgetsBindingObserver {
  static const _defaultPetColor = '#7668EE';
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      ref.read(desktopPetMobileRuntimeProvider.notifier).refreshStatus();
    });
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      unawaited(
        ref.read(desktopPetMobileRuntimeProvider.notifier).refreshStatus(),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final runtime = ref.watch(desktopPetMobileRuntimeProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠中心',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: RefreshIndicator(
          onRefresh: () =>
              ref.read(desktopPetMobileRuntimeProvider.notifier).refreshStatus(),
          child: ListView(
            physics: const AlwaysScrollableScrollPhysics(),
            padding: EdgeInsets.only(bottom: AppSpacing.xxl),
            children: [
              SizedBox(height: AppSpacing.sm),
              _buildCurrentPetCard(context, runtime),
              if (runtime.supported && !runtime.permissionGranted) ...[
                SizedBox(height: AppSpacing.sm),
                _buildPermissionCard(context),
              ],
              if (runtime.error.isNotEmpty) ...[
                SizedBox(height: AppSpacing.sm),
                _buildErrorCard(context, runtime.error),
              ],
              SizedBox(height: AppSpacing.sectionGap),
              const DesktopPetPluginSection(),
              SizedBox(height: AppSpacing.sectionGap),
              const AmitiaSectionHeader(title: '显示设置'),
              SizedBox(height: AppSpacing.sm),
              _buildSettingsCard(context, runtime),
              SizedBox(height: AppSpacing.sectionGap),
              _buildExtensionCenterCard(context),
              SizedBox(height: AppSpacing.sectionGap),
              Padding(
                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
      ),
    );
  }

  Widget _buildCurrentPetCard(
    BuildContext context,
    DesktopPetMobileRuntimeState runtime,
  ) {
    final petName = runtime.petName.trim().isNotEmpty ? runtime.petName : '未启用桌宠';
    final color = _parseColor(_defaultPetColor);
    final status = _runtimeStatus(runtime);

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              child: Center(
                child: Text(
                  _getInitial(petName),
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(petName, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(
                    _runtimeDetail(runtime),
                    style: AppTypography.caption(context),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            AmitiaStatusBadge(label: status.$1, type: status.$2),
          ],
        ),
      ),
    );
  }

  Widget _buildPermissionCard(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Icon(Icons.picture_in_picture_alt_outlined, color: context.accentPrimary),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('需要悬浮窗权限', style: AppTypography.label(context)),
                  const SizedBox(height: 2),
                  Text(
                    'Android 需要允许 Amitia 显示在其他应用上层。',
                    style: AppTypography.caption(context),
                  ),
                ],
              ),
            ),
            TextButton(
              onPressed: _busy ? null : _requestPermission,
              child: const Text('去授权'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildErrorCard(BuildContext context, String message) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(Icons.error_outline, color: Theme.of(context).colorScheme.error),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Text(
                message,
                style: AppTypography.caption(context),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSettingsCard(
    BuildContext context,
    DesktopPetMobileRuntimeState runtime,
  ) {
    final canControl = runtime.rendererLoaded && !_busy;
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
                      Text(
                        runtime.rendererLoaded
                            ? '控制真实 Android 桌宠悬浮窗'
                            : '启用桌宠后才能控制悬浮窗',
                        style: AppTypography.label(context),
                      ),
                    ],
                  ),
                ),
                Switch(
                  value: runtime.rendererLoaded && runtime.visible,
                  onChanged: canControl
                      ? (value) => _setVisible(value)
                      : null,
                ),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                Expanded(child: Text('透明度', style: AppTypography.body(context))),
                Text(
                  '${(runtime.alpha * 100).round()}%',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
            Slider(
              value: runtime.alpha.clamp(0.2, 1.0).toDouble(),
              min: 0.2,
              max: 1.0,
              divisions: 8,
              activeColor: context.accentPrimary,
              onChanged: canControl
                  ? (value) => ref
                      .read(desktopPetMobileRuntimeProvider.notifier)
                      .setAlpha(value)
                  : null,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildExtensionCenterCard(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
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
              child: Icon(
                Icons.extension_outlined,
                size: 18,
                color: context.accentPrimary,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
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
    );
  }

  (String, BadgeType) _runtimeStatus(DesktopPetMobileRuntimeState runtime) {
    if (!runtime.supported) return ('不可用', BadgeType.neutral);
    if (!runtime.permissionGranted) return ('待授权', BadgeType.warning);
    if (!runtime.rendererLoaded) {
      return runtime.connected
          ? ('未启用', BadgeType.neutral)
          : ('Runtime 离线', BadgeType.warning);
    }
    if (!runtime.visible) return ('已隐藏', BadgeType.info);
    if (runtime.paused) return ('已暂停', BadgeType.warning);
    return ('运行中', BadgeType.success);
  }

  String _runtimeDetail(DesktopPetMobileRuntimeState runtime) {
    if (!runtime.supported) return '当前版本仅在 Android 提供系统级桌宠悬浮窗';
    if (!runtime.permissionGranted) return '尚未获得系统悬浮窗权限';
    if (!runtime.rendererLoaded) {
      return runtime.connected ? '当前设备没有启用的桌宠' : '正在等待本机 Device Agent';
    }
    if (runtime.currentActionKey.isNotEmpty) {
      return '动作：${runtime.currentActionKey}';
    }
    return runtime.visible ? '已连接本机 Runtime V2' : '悬浮窗当前隐藏';
  }

  Future<void> _requestPermission() async {
    await _runControl(() async {
      await ref
          .read(desktopPetMobileRuntimeProvider.notifier)
          .requestOverlayPermission();
    });
  }

  Future<void> _setVisible(bool visible) async {
    await _runControl(() async {
      await ref
          .read(desktopPetMobileRuntimeProvider.notifier)
          .setVisible(visible);
    });
  }

  Future<void> _runControl(Future<void> Function() operation) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await operation();
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(error.toString())),
      );
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Color _parseColor(String hex) {
    final cleaned = hex.replaceAll('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  }

  String _getInitial(String name) => name.isNotEmpty ? name.substring(0, 1) : '?';
}
