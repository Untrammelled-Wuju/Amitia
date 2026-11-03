import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class VirtualDisplaySettingsPage extends ConsumerStatefulWidget {
  const VirtualDisplaySettingsPage({super.key});

  @override
  ConsumerState<VirtualDisplaySettingsPage> createState() =>
      _VirtualDisplaySettingsPageState();
}

class _VirtualDisplaySettingsPageState
    extends ConsumerState<VirtualDisplaySettingsPage> {
  bool _loading = true;
  String _error = '';
  Map<String, dynamic> _status = const <String, dynamic>{};

  @override
  void initState() {
    super.initState();
    Future<void>.microtask(_load);
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final response = await dispatcher.execute(<String, dynamic>{
        'protocolVersion': 1,
        'requestId':
            'virtual_display_preview_status_${DateTime.now().microsecondsSinceEpoch}',
        'platform': 'ios',
        'operation': 'virtual_display.preview.status',
        'payload': const <String, dynamic>{},
      });
      if (response['status']?.toString() != 'success') {
        final error = response['error'];
        final message = error is Map
            ? (error['message']?.toString() ?? error.toString())
            : error?.toString();
        throw StateError(message ?? '虚拟屏状态不可用');
      }
      final result = response['result'];
      if (!mounted) return;
      setState(() {
        _status = result is Map
            ? Map<String, dynamic>.from(result)
            : const <String, dynamic>{};
        _error = '';
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final unsupported = _status['supported'] == false;
    final reason = _status['reason']?.toString().trim().isNotEmpty == true
        ? _status['reason']!.toString()
        : 'iOS 不提供 Android VirtualDisplay 等价能力';
    final badgeLabel = _loading
        ? '检查中'
        : unsupported
        ? '不支持'
        : '已检查';
    final detail = _loading
        ? '正在检查系统能力'
        : _error.isNotEmpty
        ? _error
        : reason;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '虚拟屏显示', showBackButton: true),
      body: RefreshIndicator(
        onRefresh: _load,
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            Container(
              padding: EdgeInsets.all(AppSpacing.cardPadding),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.5),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.splitscreen_outlined,
                        color: context.accentPrimary,
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Text(
                          '虚拟屏实时画面',
                          style: AppTypography.cardTitle(context),
                        ),
                      ),
                      AmitiaStatusBadge(
                        label: badgeLabel,
                        type: unsupported || _error.isNotEmpty
                            ? BadgeType.warning
                            : BadgeType.neutral,
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.md),
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          '显示虚拟屏实时画面',
                          style: AppTypography.bodySmall(context),
                        ),
                      ),
                      Switch.adaptive(value: false, onChanged: null),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  Text(
                    detail,
                    style: AppTypography.bodySmall(context).copyWith(
                      color: _error.isEmpty
                          ? context.textSecondary
                          : context.error,
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: AppSpacing.md),
            Container(
              padding: EdgeInsets.all(AppSpacing.cardPadding),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.5),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('iOS 限制', style: AppTypography.cardTitle(context)),
                  SizedBox(height: AppSpacing.sm),
                  Text(
                    '普通 iOS 应用只能在自身窗口内显示内容，不能像 Android 那样在桌面或其他应用上方创建任意系统悬浮窗。'
                    '画中画仅支持符合条件的视频播放，不能作为通用虚拟屏显示通道。',
                    style: AppTypography.bodySmall(context),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
