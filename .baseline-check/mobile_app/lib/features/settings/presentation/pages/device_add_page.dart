import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DeviceAddPage extends ConsumerStatefulWidget {
  const DeviceAddPage({super.key});

  @override
  ConsumerState<DeviceAddPage> createState() => _DeviceAddPageState();
}

class _DeviceAddPageState extends ConsumerState<DeviceAddPage> {
  final _labelController = TextEditingController();
  bool _submitting = false;

  @override
  void dispose() {
    _labelController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final deployment = ref.watch(mobileDeploymentConfigProvider);
    final identity = ref.watch(localDeviceMeshIdentityProvider);
    final status = ref.watch(localDeviceMeshStatusProvider);
    final cloudUri = deployment.remoteCoreUri?.trim() ?? '';
    final cloudReady = deployment.mode == MobileDeploymentMode.cloud && cloudUri.isNotEmpty;

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '添加设备',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: ListView(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          AppSpacing.xl,
        ),
        children: [
          _InfoCard(
            icon: Icons.devices_other_outlined,
            title: '将本机加入云端设备网格',
            description: '使用当前账号向 Cloud Core 申请一次性绑定凭据，再由本机 Runtime 完成 Device Mesh 注册。',
          ),
          SizedBox(height: AppSpacing.lg),
          Text('本机身份', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          _AsyncIdentityCard(identity: identity, status: status),
          SizedBox(height: AppSpacing.lg),
          Text('设备名称', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          AmitiaTextField(
            controller: _labelController,
            hintText: '例如：我的 Android 手机',
            prefixIcon: const Icon(Icons.edit_outlined),
          ),
          SizedBox(height: AppSpacing.lg),
          _ConnectionCard(
            cloudReady: cloudReady,
            cloudUri: cloudUri,
            onOpenDeployment: () => context.push(AppRoutes.settingsDeployment),
          ),
          SizedBox(height: AppSpacing.lg),
          AmitiaButton(
            label: _submitting ? '正在绑定…' : '将本机加入云端',
            icon: Icons.link_outlined,
            isFullWidth: true,
            onPressed: !_submitting && cloudReady ? _bindCurrentDevice : null,
          ),
          SizedBox(height: AppSpacing.sm),
          Text(
            '添加其它设备时，请在对应设备上登录同一账号并执行本流程。绑定凭据由后端生成且具有有效期。',
            style: AppTypography.caption(context),
          ),
        ],
      ),
    );
  }

  Future<void> _bindCurrentDevice() async {
    if (_submitting) return;
    final deployment = ref.read(mobileDeploymentConfigProvider);
    final cloudUri = deployment.remoteCoreUri?.trim() ?? '';
    if (deployment.mode != MobileDeploymentMode.cloud || cloudUri.isEmpty) {
      _show('请先配置并启用云端模式');
      return;
    }
    setState(() => _submitting = true);
    try {
      final localService = ref.read(deviceMeshLocalServiceProvider);
      if (localService == null) {
        throw StateError('本机 Runtime 当前不可用，无法读取 Device Mesh 身份');
      }
      final identity = await localService.identity();
      final deviceId = (identity['deviceId'] ?? '').toString().trim();
      final runtimeId = (identity['runtimeId'] ?? '').toString().trim();
      final platform = (identity['platform'] ?? '').toString().trim();
      if (deviceId.isEmpty || runtimeId.isEmpty || platform.isEmpty) {
        throw StateError('本机 Device Mesh 身份不完整');
      }

      final ticket = await ref.read(deviceMeshServiceProvider).createBootstrapTicket(
            deviceId: deviceId,
            runtimeId: runtimeId,
            platform: platform,
            label: _labelController.text.trim(),
          );
      final rawTicket = (ticket['ticket'] ?? '').toString();
      if (rawTicket.isEmpty) throw StateError('云端未返回有效绑定票据');

      await localService.bootstrap(
        cloudBaseUrl: cloudUri,
        bootstrapTicket: rawTicket,
      );
      ref.invalidate(localDeviceMeshStatusProvider);
      ref.invalidate(localDeviceMeshIdentityProvider);
      ref.invalidate(deviceMeshDevicesProvider);
      if (!mounted) return;
      _show('设备已加入云端协同');
      context.pop();
    } catch (e) {
      if (mounted) _show('绑定失败：${_message(e)}');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  void _show(String message) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  static String _message(Object error) => error.toString().replaceFirst('Bad state: ', '').replaceFirst('Exception: ', '');
}

class _AsyncIdentityCard extends StatelessWidget {
  final AsyncValue<Map<String, dynamic>?> identity;
  final AsyncValue<Map<String, dynamic>?> status;

  const _AsyncIdentityCard({required this.identity, required this.status});

  @override
  Widget build(BuildContext context) {
    return identity.when(
      loading: () => const _CompactCard(child: Center(child: CircularProgressIndicator(strokeWidth: 2))),
      error: (error, _) => _CompactCard(
        child: Text('无法读取本机身份：${error.toString().replaceFirst('Exception: ', '')}', style: AppTypography.caption(context).copyWith(color: context.error)),
      ),
      data: (data) {
        if (data == null) {
          return _CompactCard(
            child: Text('本机 Runtime 尚未就绪。请先启动 Runtime。', style: AppTypography.caption(context)),
          );
        }
        final state = status.asData?.value?['state']?.toString() ?? 'unknown';
        return _CompactCard(
          child: Column(
            children: [
              _ValueRow(label: 'Device ID', value: (data['deviceId'] ?? '-').toString()),
              _ValueRow(label: 'Runtime ID', value: (data['runtimeId'] ?? '-').toString()),
              _ValueRow(label: '平台', value: (data['platform'] ?? '-').toString()),
              _ValueRow(label: 'Mesh 状态', value: _stateLabel(state), isLast: true),
            ],
          ),
        );
      },
    );
  }

  static String _stateLabel(String raw) {
    switch (raw.toLowerCase()) {
      case 'connected':
        return '已连接';
      case 'connecting':
        return '连接中';
      case 'unprovisioned':
        return '未绑定';
      default:
        return raw.isEmpty ? '未知' : raw;
    }
  }
}

class _ConnectionCard extends StatelessWidget {
  final bool cloudReady;
  final String cloudUri;
  final VoidCallback onOpenDeployment;

  const _ConnectionCard({required this.cloudReady, required this.cloudUri, required this.onOpenDeployment});

  @override
  Widget build(BuildContext context) {
    return _CompactCard(
      child: Row(
        children: [
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(color: context.accentSoft, borderRadius: BorderRadius.circular(12)),
            child: Icon(Icons.cloud_outlined, color: context.accentPrimary, size: 19),
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(cloudReady ? '云端配置已就绪' : '需要云端模式', style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(cloudReady ? cloudUri : '添加设备依赖 Cloud Core 的真实绑定接口', style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
              ],
            ),
          ),
          TextButton(onPressed: onOpenDeployment, child: const Text('配置')),
        ],
      ),
    );
  }
}

class _InfoCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String description;

  const _InfoCard({required this.icon, required this.title, required this.description});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(15),
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.accentPrimary.withValues(alpha: 0.14)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(icon, size: 20, color: context.accentPrimary),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 5),
                Text(description, style: AppTypography.caption(context)),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _CompactCard extends StatelessWidget {
  final Widget child;
  const _CompactCard({required this.child});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(13),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: child,
    );
  }
}

class _ValueRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isLast;

  const _ValueRow({required this.label, required this.value, this.isLast = false});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        border: isLast ? null : Border(bottom: BorderSide(color: context.borderSecondary, width: 0.6)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(width: 82, child: Text(label, style: AppTypography.caption(context))),
          Expanded(child: Text(value, style: AppTypography.body(context), textAlign: TextAlign.right, maxLines: 2, overflow: TextOverflow.ellipsis)),
        ],
      ),
    );
  }
}
