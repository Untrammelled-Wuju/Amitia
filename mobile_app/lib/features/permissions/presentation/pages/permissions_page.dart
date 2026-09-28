import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../shared/models/models.dart';

class PermissionsPage extends ConsumerStatefulWidget {
  const PermissionsPage({super.key});

  @override
  ConsumerState<PermissionsPage> createState() => _PermissionsPageState();
}

class _PermissionsPageState extends ConsumerState<PermissionsPage>
    with WidgetsBindingObserver {
  late List<PermissionItem> _permissions;
  Map<String, dynamic> _providerStatus = const <String, dynamic>{};
  Map<String, dynamic> _shizukuStatus = const <String, dynamic>{};
  bool _providerBusy = false;
  bool _shizukuBusy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _permissions = [
      PermissionItem(
        name: '无障碍服务',
        icon: Icons.accessibility_new,
        status: '需要设置',
        description: '允许 Amitia 读取屏幕内容并提供辅助',
      ),
      PermissionItem(
        name: '通知读取',
        icon: Icons.notifications_outlined,
        status: '需要设置',
        description: '读取系统通知以提供智能提醒',
      ),
      PermissionItem(
        name: '悬浮窗',
        icon: Icons.picture_in_picture,
        status: '已授权',
        description: '在其他应用上方显示悬浮窗',
      ),
      PermissionItem(
        name: '文件访问',
        icon: Icons.folder_outlined,
        status: '需要设置',
        description: '访问设备存储中的文件',
      ),
      PermissionItem(
        name: '麦克风',
        icon: Icons.mic_outlined,
        status: '已授权',
        description: '语音输入和通话',
      ),
      PermissionItem(
        name: '相机',
        icon: Icons.camera_alt_outlined,
        status: '未授权',
        description: '拍照和扫描功能',
      ),
      PermissionItem(
        name: '位置',
        icon: Icons.location_on_outlined,
        status: '未授权',
        description: '获取设备位置信息',
      ),
      PermissionItem(
        name: '电池优化',
        icon: Icons.battery_std,
        status: '已授权',
        description: '忽略电池优化以保持后台运行',
      ),
    ];
    Future<void>.microtask(() async {
      await _refreshShizuku();
      await _refreshProvider();
      await _refreshAccessibility();
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
      _refreshShizuku();
      _refreshProvider();
      _refreshAccessibility();
    }
  }

  Future<void> _refreshShizuku() async {
    try {
      final response = await ref
          .read(nativeBridgePlatformDispatcherProvider)
          .execute({
            'protocolVersion': 1,
            'requestId':
                'shizuku_status_${DateTime.now().microsecondsSinceEpoch}',
            'platform': 'android',
            'operation': 'shizuku.status',
            'payload': <String, dynamic>{},
          });
      if (!mounted || response['status'] != 'success') return;
      setState(() {
        _shizukuStatus = Map<String, dynamic>.from(
          response['result'] as Map? ?? {},
        );
      });
    } catch (_) {}
  }

  Future<Map<String, dynamic>> _executeShizuku(
    String operation, {
    Map<String, dynamic> payload = const <String, dynamic>{},
  }) async {
    return ref.read(nativeBridgePlatformDispatcherProvider).execute({
      'protocolVersion': 1,
      'requestId':
          'shizuku_${DateTime.now().microsecondsSinceEpoch}_$operation',
      'platform': 'android',
      'operation': operation,
      'payload': payload,
    });
  }

  Future<void> _handleShizukuToggle(bool enabled) async {
    if (_shizukuBusy) return;
    setState(() => _shizukuBusy = true);
    try {
      if (!enabled) {
        final response = await _executeShizuku(
          'shizuku.set_enabled',
          payload: const <String, dynamic>{'enabled': false},
        );
        if (response['status'] != 'success') {
          throw StateError(
            (response['error'] as Map?)?['message']?.toString() ??
                '关闭 Shizuku 失败',
          );
        }
      } else {
        final state = _shizukuStatus['state']?.toString() ?? '';
        if (state == 'not_installed' || state == 'not_running') {
          final response = await _executeShizuku('shizuku.open_manager');
          if (response['status'] != 'success') {
            throw StateError(
              (response['error'] as Map?)?['message']?.toString() ??
                  '无法打开 Shizuku',
            );
          }
          final result = response['result'] as Map?;
          final message = result?['openedInstallPage'] == true
              ? '已打开 Shizuku 下载页，安装并启动服务后返回'
              : '已打开 Shizuku，请启动服务后返回';
          if (mounted) {
            ScaffoldMessenger.of(
              context,
            ).showSnackBar(SnackBar(content: Text(message)));
          }
        } else {
          final response = await _executeShizuku(
            'shizuku.set_enabled',
            payload: const <String, dynamic>{'enabled': true},
          );
          if (response['status'] != 'success') {
            throw StateError(
              (response['error'] as Map?)?['message']?.toString() ??
                  '启用 Shizuku 失败',
            );
          }
        }
      }
      await Future<void>.delayed(const Duration(milliseconds: 300));
      await _refreshShizuku();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('操作失败：$error')));
      }
    } finally {
      if (mounted) setState(() => _shizukuBusy = false);
    }
  }

  Future<void> _runShizukuPrimaryAction() async {
    if (_shizukuBusy) return;
    final state = _shizukuStatus['state']?.toString() ?? '';
    if (state == 'not_installed' || state == 'not_running') {
      setState(() => _shizukuBusy = true);
      try {
        final response = await _executeShizuku('shizuku.open_manager');
        if (response['status'] != 'success') {
          throw StateError(
            (response['error'] as Map?)?['message']?.toString() ??
                '无法打开 Shizuku',
          );
        }
        final result = response['result'] as Map?;
        if (mounted) {
          final message = result?['openedInstallPage'] == true
              ? '已打开 Shizuku 下载页'
              : '已打开 Shizuku，请启动服务';
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text(message)));
        }
      } catch (error) {
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('操作失败：$error')));
        }
      } finally {
        if (mounted) setState(() => _shizukuBusy = false);
      }
      return;
    }
    if (state == 'ready') {
      setState(() => _shizukuBusy = true);
      try {
        final response = await _executeShizuku('shizuku.test');
        if (response['status'] != 'success') {
          throw StateError(
            (response['error'] as Map?)?['message']?.toString() ??
                'Shizuku 测试失败',
          );
        }
        final result = response['result'] as Map?;
        final stdout = result?['stdout']?.toString().trim() ?? '';
        final message = stdout.isEmpty
            ? 'Shizuku 测试通过'
            : 'Shizuku 测试通过：$stdout';
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text(message)));
        }
      } catch (error) {
        if (mounted) {
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('操作失败：$error')));
        }
      } finally {
        if (mounted) setState(() => _shizukuBusy = false);
      }
      return;
    }
    await _handleShizukuToggle(true);
  }

  String _shizukuStateLabel() {
    switch (_shizukuStatus['state']?.toString()) {
      case 'ready':
        return '已就绪';
      case 'disabled':
        return '已授权未启用';
      case 'permission_required':
        return '需授权';
      case 'not_running':
        return '服务未启动';
      case 'not_installed':
        return '未安装';
      case 'error':
        return '异常';
      default:
        return '检测中';
    }
  }

  BadgeType _shizukuBadgeType() {
    switch (_shizukuStatus['state']?.toString()) {
      case 'ready':
        return BadgeType.success;
      case 'disabled':
      case 'permission_required':
      case 'not_running':
        return BadgeType.warning;
      case 'error':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  String _shizukuPrimaryLabel() {
    switch (_shizukuStatus['state']?.toString()) {
      case 'not_installed':
        return '安装 Shizuku';
      case 'not_running':
        return '打开 Shizuku';
      case 'permission_required':
        return '申请授权';
      case 'disabled':
        return '启用';
      case 'ready':
        return '测试';
      default:
        return '重试';
    }
  }

  Future<void> _refreshProvider() async {
    try {
      final response = await ref
          .read(nativeBridgePlatformDispatcherProvider)
          .execute({
            'protocolVersion': 1,
            'requestId':
                'accessibility_provider_status_${DateTime.now().microsecondsSinceEpoch}',
            'platform': 'android',
            'operation': 'accessibility.provider.status',
            'payload': <String, dynamic>{},
          });
      if (!mounted || response['status'] != 'success') return;
      setState(() {
        _providerStatus = Map<String, dynamic>.from(
          response['result'] as Map? ?? {},
        );
      });
    } catch (_) {}
  }

  Future<void> _providerAction(String operation) async {
    if (_providerBusy) return;
    setState(() => _providerBusy = true);
    try {
      final response = await ref
          .read(nativeBridgePlatformDispatcherProvider)
          .execute({
            'protocolVersion': 1,
            'requestId':
                'accessibility_provider_${DateTime.now().microsecondsSinceEpoch}',
            'platform': 'android',
            'operation': operation,
            'payload': <String, dynamic>{},
          });
      if (response['status'] != 'success') {
        throw StateError(
          (response['error'] as Map?)?['message']?.toString() ?? '操作未成功执行',
        );
      }
      await Future<void>.delayed(const Duration(milliseconds: 400));
      await _refreshProvider();
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('操作失败：$error')));
      }
    } finally {
      if (mounted) setState(() => _providerBusy = false);
    }
  }

  Future<void> _refreshAccessibility() async {
    try {
      final response = await ref
          .read(nativeBridgePlatformDispatcherProvider)
          .execute({
            'protocolVersion': 1,
            'requestId':
                'accessibility_status_${DateTime.now().microsecondsSinceEpoch}',
            'platform': 'android',
            'operation': 'accessibility.status',
            'payload': <String, dynamic>{},
          });
      if (!mounted || response['status'] != 'success') return;
      final result = Map<String, dynamic>.from(
        response['result'] as Map? ?? {},
      );
      final connected = result['connected'] == true;
      final enabled = result['enabledInSettings'] == true;
      final ready = result['ready'] == true;
      final canRetrieve = result['canRetrieveWindowContent'] == true;
      final canPerformGestures = result['canPerformGestures'] == true;
      final canTakeScreenshot = result['canTakeScreenshot'] == true;
      final limitations = <String>[
        if (!canRetrieve) '界面读取',
        if (!canPerformGestures) '手势控制',
        if (!canTakeScreenshot) '屏幕截图',
      ];
      setState(() {
        _permissions[0] = PermissionItem(
          name: '无障碍服务',
          icon: Icons.accessibility_new,
          status: ready
              ? '已授权'
              : connected
              ? '能力受限'
              : enabled
              ? '连接中'
              : '需要设置',
          description: ready
              ? '已连接，可供 AI 读取并操作屏幕'
              : connected
              ? '已连接，但缺少：${limitations.join('、')}'
              : enabled
              ? '系统已开启，正在等待服务连接'
              : '允许 Amitia 读取和操作屏幕',
        );
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _permissions[0] = PermissionItem(
          name: '无障碍服务',
          icon: Icons.accessibility_new,
          status: '不可用',
          description: '无法读取无障碍服务状态',
        );
      });
    }
  }

  Future<void> _openAccessibilitySettings() async {
    try {
      final response = await ref
          .read(nativeBridgePlatformDispatcherProvider)
          .execute({
            'protocolVersion': 1,
            'requestId':
                'accessibility_settings_${DateTime.now().microsecondsSinceEpoch}',
            'platform': 'android',
            'operation': 'accessibility.open_settings',
            'payload': <String, dynamic>{},
          });
      if (response['status'] == 'success' &&
          (response['result'] as Map?)?['opened'] == true) {
        return;
      }
    } catch (_) {}
    if (mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('无法打开系统无障碍设置')));
    }
  }

  BadgeType _badgeType(String status) {
    switch (status) {
      case '已授权':
        return BadgeType.success;
      case '需要设置':
      case '连接中':
      case '能力受限':
        return BadgeType.warning;
      case '不可用':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  void _showGuide(PermissionItem item) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (ctx) => _PermissionGuideSheet(
        item: item,
        steps: _guideStepsFor(item.name),
        onOpenSettings: item.name == '无障碍服务'
            ? _openAccessibilitySettings
            : null,
      ),
    );
  }

  List<String> _guideStepsFor(String name) {
    if (name == '无障碍服务') {
      return const [
        '打开系统设置',
        '找到「无障碍」或「辅助功能」',
        '选择「Amitia Accessibility Service」',
        '开启服务开关并确认',
        '若开关不可用，请在应用详情中允许受限设置后重试',
      ];
    }
    if (name == '通知读取') {
      return const [
        '打开系统设置',
        '找到「无障碍」或「通知」',
        '选择「通知访问」或「Notification Listener」',
        '开启 Amitia 并确认',
      ];
    }
    return const ['打开系统设置', '找到「应用管理」', '选择 Amitia', '找到权限并开启'];
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '系统权限',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: ListView.separated(
        padding: EdgeInsets.symmetric(
          vertical: AppSpacing.md,
          horizontal: AppSpacing.pagePadding,
        ),
        itemCount: _permissions.length + 2,
        separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
        itemBuilder: (context, index) {
          if (index == 0) {
            return _ShizukuPermissionCard(
              status: _shizukuStatus,
              busy: _shizukuBusy,
              stateLabel: _shizukuStateLabel(),
              badgeType: _shizukuBadgeType(),
              primaryLabel: _shizukuPrimaryLabel(),
              onToggle: _handleShizukuToggle,
              onPrimaryAction: _runShizukuPrimaryAction,
            );
          }
          if (index == 1) {
            return _AccessibilityProviderCard(
              status: _providerStatus,
              busy: _providerBusy,
              onInstall: () =>
                  _providerAction('accessibility.provider.install'),
              onSettings: () =>
                  _providerAction('accessibility.provider.open_settings'),
            );
          }
          index -= 2;
          final item = _permissions[index];
          return _PermissionCard(
            item: item,
            badgeType: _badgeType(item.status),
            onTap: () => _showGuide(item),
          );
        },
      ),
    );
  }
}

class _ShizukuPermissionCard extends StatelessWidget {
  final Map<String, dynamic> status;
  final bool busy;
  final String stateLabel;
  final BadgeType badgeType;
  final String primaryLabel;
  final ValueChanged<bool> onToggle;
  final VoidCallback onPrimaryAction;

  const _ShizukuPermissionCard({
    required this.status,
    required this.busy,
    required this.stateLabel,
    required this.badgeType,
    required this.primaryLabel,
    required this.onToggle,
    required this.onPrimaryAction,
  });

  @override
  Widget build(BuildContext context) {
    final provider = status['provider']?.toString() ?? 'none';
    final version = status['version']?.toString() ?? '';
    final uid = status['uid']?.toString() ?? '';
    final reason = status['reason']?.toString().trim() ?? '';
    final enabled = status['enabled'] == true;
    final ready = status['state']?.toString() == 'ready';
    final providerLabel = switch (provider) {
      'shizuku' => 'Shizuku',
      'axmanager' => 'AXManager',
      'sui' => 'Sui',
      'none' => '未检测到',
      _ => provider,
    };
    final detail = ready
        ? '$providerLabel${version.isEmpty ? '' : ' v$version'}${uid.isEmpty ? '' : ' · uid $uid'}'
        : (reason.isEmpty ? '高权限系统执行通道' : reason);
    return Container(
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
              Icon(Icons.security_outlined, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text('Shizuku', style: AppTypography.cardTitle(context)),
              ),
              AmitiaStatusBadge(label: stateLabel, type: badgeType),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(detail, style: AppTypography.label(context)),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: Text(
                  '允许 AI 使用 Shizuku',
                  style: AppTypography.bodySmall(context),
                ),
              ),
              Switch.adaptive(
                value: enabled,
                onChanged: busy ? null : onToggle,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          SizedBox(
            width: double.infinity,
            child: OutlinedButton.icon(
              onPressed: busy ? null : onPrimaryAction,
              icon: Icon(
                status['state']?.toString() == 'ready'
                    ? Icons.play_arrow_outlined
                    : Icons.build_outlined,
              ),
              label: Text(primaryLabel),
            ),
          ),
        ],
      ),
    );
  }
}

class _AccessibilityProviderCard extends StatelessWidget {
  final Map<String, dynamic> status;
  final bool busy;
  final VoidCallback onInstall;
  final VoidCallback onSettings;

  const _AccessibilityProviderCard({
    required this.status,
    required this.busy,
    required this.onInstall,
    required this.onSettings,
  });

  @override
  Widget build(BuildContext context) {
    final installed = status['installed'] == true;
    final connected = status['connected'] == true;
    final version = status['versionName']?.toString().trim() ?? '';
    final stateText = connected
        ? '已连接'
        : installed
        ? '已安装未开启'
        : '未安装';
    final badgeType = connected
        ? BadgeType.success
        : installed
        ? BadgeType.warning
        : BadgeType.neutral;
    return Container(
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
              Icon(Icons.verified_user_outlined, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  '无障碍 Provider',
                  style: AppTypography.cardTitle(context),
                ),
              ),
              AmitiaStatusBadge(label: stateText, type: badgeType),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            installed
                ? '独立低权限辅助包${version.isEmpty ? '' : ' $version'}'
                : '安装独立辅助包，避免主应用风险策略影响无障碍服务',
            style: AppTypography.label(context),
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Expanded(
                child: FilledButton.icon(
                  onPressed: busy ? null : onInstall,
                  icon: const Icon(Icons.download_outlined),
                  label: Text(installed ? '更新 Provider' : '安装 Provider'),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              OutlinedButton.icon(
                onPressed: busy ? null : onSettings,
                icon: const Icon(Icons.settings_outlined),
                label: const Text('设置'),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _PermissionCard extends StatelessWidget {
  final PermissionItem item;
  final BadgeType badgeType;
  final VoidCallback onTap;

  const _PermissionCard({
    required this.item,
    required this.badgeType,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(item.icon, size: 20, color: context.accentPrimary),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(item.name, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(
                    item.description,
                    style: AppTypography.label(context),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            AmitiaStatusBadge(label: item.status, type: badgeType),
          ],
        ),
      ),
    );
  }
}

class _PermissionGuideSheet extends StatelessWidget {
  final PermissionItem item;
  final List<String> steps;
  final Future<void> Function()? onOpenSettings;

  const _PermissionGuideSheet({
    required this.item,
    required this.steps,
    this.onOpenSettings,
  });

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.lg,
          0,
          AppSpacing.lg,
          AppSpacing.xxl,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(item.name, style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            Text(item.description, style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.lg),
            ...steps.asMap().entries.map((entry) {
              return Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.md),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 22,
                      height: 22,
                      decoration: BoxDecoration(
                        color: context.accentSoft,
                        shape: BoxShape.circle,
                      ),
                      child: Center(
                        child: Text(
                          '${entry.key + 1}',
                          style: TextStyle(
                            fontSize: 12,
                            color: context.accentPrimary,
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                      ),
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Text(
                        entry.value,
                        style: AppTypography.bodySmall(context),
                      ),
                    ),
                  ],
                ),
              );
            }),
            if (onOpenSettings != null) ...[
              SizedBox(height: AppSpacing.md),
              SizedBox(
                width: double.infinity,
                child: FilledButton(
                  onPressed: () async {
                    Navigator.of(context).pop();
                    await onOpenSettings!();
                  },
                  child: const Text('前往开启无障碍服务'),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
