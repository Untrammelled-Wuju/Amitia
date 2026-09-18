import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
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
  final _pairingController = TextEditingController();
  bool _submitting = false;
  bool _generating = false;
  String _offerPayload = '';

  @override
  void dispose() {
    _labelController.dispose();
    _pairingController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final deployment = ref.watch(mobileDeploymentConfigProvider);
    final identity = ref.watch(localDeviceMeshIdentityProvider);
    final status = ref.watch(localDeviceMeshStatusProvider);
    final cloudUri = deployment.remoteCoreUri?.trim() ?? '';
    final cloudReady = deployment.mode == MobileDeploymentMode.cloud && cloudUri.isNotEmpty;
    final localState = (status.asData?.value?['state'] ?? 'unknown').toString().toLowerCase();
    final isTrusted = localState == 'connected' || localState == 'connecting';

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '设备配对',
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
            icon: Icons.hub_outlined,
            title: isTrusted ? '为新设备生成配对 Offer' : '将本机加入 Cloud Core',
            description: isTrusted
                ? '当前设备已受信任，可生成一次性 Offer。新设备扫描二维码或粘贴 Offer 后即可完成 Device Credential 签发。'
                : 'Amitia 不使用产品账号。未配对设备通过一次性 Offer 或首次设置码加入 Cloud Core。',
          ),
          SizedBox(height: AppSpacing.lg),
          Text('本机身份', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          _AsyncIdentityCard(identity: identity, status: status),
          SizedBox(height: AppSpacing.lg),
          _ConnectionCard(
            cloudReady: cloudReady,
            cloudUri: cloudUri,
            onOpenDeployment: () => context.push(AppRoutes.settingsDeployment),
          ),
          if (isTrusted) ...[
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: _generating ? '正在生成…' : '生成一次性配对 Offer',
              icon: Icons.qr_code_2,
              isFullWidth: true,
              onPressed: !_generating && cloudReady ? _generateOffer : null,
            ),
            if (_offerPayload.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              _CompactCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('二维码内容 / Offer', style: AppTypography.label(context)),
                    const SizedBox(height: 8),
                    SelectableText(_offerPayload, style: AppTypography.bodySmall(context)),
                    const SizedBox(height: 8),
                    Text('将这段内容交给待加入设备；Offer 过期或使用后即失效。', style: AppTypography.caption(context)),
                  ],
                ),
              ),
            ],
          ] else ...[
            SizedBox(height: AppSpacing.lg),
            Text('设备名称', style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.sm),
            AmitiaTextField(
              controller: _labelController,
              hintText: '例如：我的 Android 手机',
              prefixIcon: const Icon(Icons.edit_outlined),
            ),
            SizedBox(height: AppSpacing.lg),
            Text('配对信息', style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.sm),
            AmitiaTextField(
              controller: _pairingController,
              hintText: '粘贴 amitia://pair?...、Offer Token 或首设备设置码',
              prefixIcon: const Icon(Icons.qr_code_scanner_outlined),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: _submitting ? '正在配对…' : '配对并加入云端',
              icon: Icons.link_outlined,
              isFullWidth: true,
              onPressed: !_submitting && cloudReady ? _bindCurrentDevice : null,
            ),
            SizedBox(height: AppSpacing.sm),
            Text(
              '首次设备使用 Cloud Core 本机显示的设置码；后续设备使用任意已信任设备生成的一次性 Offer。Device ID 不是密码。',
              style: AppTypography.caption(context),
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _generateOffer() async {
    if (_generating) return;
    setState(() => _generating = true);
    try {
      final offer = await ref.read(deviceMeshServiceProvider).createPairingOffer();
      final token = (offer['offerToken'] ?? '').toString().trim();
      if (token.isEmpty) throw StateError('Cloud Core 未返回有效配对 Offer');
      final cloudUri = ref.read(mobileDeploymentConfigProvider).remoteCoreUri?.trim() ?? '';
      if (cloudUri.isEmpty) throw StateError('Cloud Core 地址未配置');
      final endpoint = Uri.parse(cloudUri).replace(path: '', query: null, fragment: null).toString().replaceFirst(RegExp(r'/$'), '');
      final payload = Uri(
        scheme: 'amitia',
        host: 'pair',
        queryParameters: <String, String>{'endpoint': endpoint, 'offer': token},
      ).toString();
      if (!mounted) return;
      setState(() => _offerPayload = payload);
    } catch (error) {
      if (mounted) _show('生成失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _generating = false);
    }
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

      final status = await ref.read(onboardingServiceProvider).pairingStatusAt(cloudUri);
      final first = status['firstDeviceSetupRequired'] == true;
      final raw = _pairingController.text.trim();
      if (raw.isEmpty) throw StateError(first ? '请输入首设备设置码' : '请粘贴配对 Offer');
      final offerToken = first ? '' : _extractOffer(raw, cloudUri);
      final setupCode = first ? raw : '';
      if (!first && offerToken.isEmpty) throw StateError('配对 Offer 无效');

      final claim = await ref.read(onboardingServiceProvider).claimPairingAt(
            cloudUri,
            deviceId: deviceId,
            runtimeId: runtimeId,
            platform: platform,
            label: _labelController.text.trim(),
            offerToken: offerToken,
            setupCode: setupCode,
          );
      final ticket = (claim['ticket'] ?? '').toString().trim();
      if (ticket.isEmpty) throw StateError('Cloud Core 未返回 Bootstrap Ticket');

      await localService.bootstrap(cloudBaseUrl: cloudUri, bootstrapTicket: ticket);
      ref.invalidate(localDeviceMeshStatusProvider);
      ref.invalidate(localDeviceMeshIdentityProvider);
      ref.invalidate(deviceMeshDevicesProvider);
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      await ref.read(backendConnectionProvider.future);
      await ref.read(backendTransportProvider.future);
      if (!mounted) return;
      _show('设备已加入 Cloud Core');
      context.pop();
    } catch (error) {
      if (mounted) _show('配对失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  String _extractOffer(String raw, String cloudUri) {
    final value = raw.trim();
    if (!value.startsWith('amitia://')) return value;
    final uri = Uri.tryParse(value);
    if (uri == null) return '';
    final endpoint = uri.queryParameters['endpoint']?.trim() ?? '';
    if (endpoint.isNotEmpty) {
      final expected = Uri.tryParse(cloudUri);
      final offered = Uri.tryParse(endpoint);
      if (expected == null || offered == null || expected.scheme != offered.scheme || expected.host != offered.host || expected.port != offered.port) {
        throw StateError('配对 Offer 属于另一个 Cloud Core');
      }
    }
    return uri.queryParameters['offer']?.trim() ?? '';
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
          return _CompactCard(child: Text('本机 Runtime 尚未就绪。', style: AppTypography.caption(context)));
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
      case 'connected': return '已连接';
      case 'connecting': return '连接中';
      case 'unprovisioned': return '未配对';
      default: return raw.isEmpty ? '未知' : raw;
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
                Text(cloudReady ? 'Cloud Core 已配置' : '需要云端模式', style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(cloudReady ? cloudUri : '先填写 Cloud Core 服务地址', style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
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
