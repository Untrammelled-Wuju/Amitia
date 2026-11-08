import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/notification_runtime_bootstrap.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/backend_uri_builder.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/runtime/backend/mobile_backend_lifecycle.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/backend_topology_resolver.dart';
import '../../../../core/services/providers.dart';

bool deploymentSelectionRequiresChange(
  MobileDeploymentConfig current,
  MobileDeploymentMode selected,
  String? remoteUri,
) {
  if (selected != current.mode) return true;
  if (selected != MobileDeploymentMode.cloud) return false;
  final currentUri = current.remoteCoreUri?.trim() ?? '';
  final requestedUri = remoteUri?.trim() ?? '';
  if (currentUri.isEmpty || requestedUri.isEmpty) return false;
  try {
    return normalizeRemoteCoreUri(currentUri).origin !=
        normalizeRemoteCoreUri(requestedUri).origin;
  } catch (_) {
    return false;
  }
}

class DeploymentPage extends ConsumerStatefulWidget {
  const DeploymentPage({super.key});

  @override
  ConsumerState<DeploymentPage> createState() => _DeploymentPageState();
}

class _DeploymentPageState extends ConsumerState<DeploymentPage> {
  bool _loading = true;
  bool _testState = false;
  bool _discoveringPairing = false;
  MobileDeploymentMode _selectedMode = MobileDeploymentMode.local;
  final _remoteUriController = TextEditingController();

  static const _modes = <(MobileDeploymentMode, IconData, String)>[
    (MobileDeploymentMode.local, Icons.dns_outlined, '当前设备保存统筹数据和模型配置，并管理其他设备'),
    (MobileDeploymentMode.cloud, Icons.cloud_outlined, '当前设备加入另一台设备提供的统筹中心'),
  ];

  @override
  void dispose() {
    _remoteUriController.dispose();
    super.dispose();
  }

  @override
  void initState() {
    super.initState();
    _loadConfig();
  }

  Future<void> _loadConfig() async {
    await Future.delayed(Duration.zero);
    await ref.read(mobileDeploymentConfigProvider.notifier).init();
    final config = ref.read(mobileDeploymentConfigProvider);
    _remoteUriController.text = config.remoteCoreUri ?? '';
    _selectedMode = config.mode;
    if (mounted) {
      setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final connectionAsync = ref.watch(backendConnectionProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '运行模式',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
              children: [
                _SectionLabel(text: '选择统筹中心'),
                SizedBox(height: AppSpacing.sm),
                ..._modes.map(
                  (m) => Padding(
                    padding: EdgeInsets.only(
                      left: AppSpacing.pagePadding,
                      right: AppSpacing.pagePadding,
                      bottom: AppSpacing.md,
                    ),
                    child: _ModeCard(
                      mode: m.$1,
                      icon: m.$2,
                      description: m.$3,
                      isSelected: m.$1 == _selectedMode,
                      onTap: () {
                        if (m.$1 == MobileDeploymentMode.cloud) {
                          setState(
                            () => _selectedMode = MobileDeploymentMode.cloud,
                          );
                          return;
                        }
                        _confirmSwitch(MobileDeploymentMode.local);
                      },
                    ),
                  ),
                ),
                if (_selectedMode == MobileDeploymentMode.cloud) ...[
                  SizedBox(height: AppSpacing.sm),
                  _SectionLabel(text: '统筹中心地址'),
                  SizedBox(height: AppSpacing.sm),
                  Padding(
                    padding: EdgeInsets.symmetric(
                      horizontal: AppSpacing.pagePadding,
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        TextField(
                          controller: _remoteUriController,
                          keyboardType: TextInputType.url,
                          autocorrect: false,
                          decoration: InputDecoration(
                            hintText: '统筹中心地址或 192.168.1.10:端口',
                            helperText: '连接后使用该中心的数据和模型；设备 Runtime 仍保留在本机',
                            border: OutlineInputBorder(
                              borderRadius: AppRadius.brSmall,
                            ),
                            contentPadding: EdgeInsets.symmetric(
                              horizontal: AppSpacing.md,
                              vertical: AppSpacing.sm,
                            ),
                          ),
                        ),
                        SizedBox(height: AppSpacing.sm),
                        AmitiaButton(
                          label: _discoveringPairing ? '检查中心身份...' : '输入地址并配对',
                          icon: Icons.link,
                          isFullWidth: true,
                          isSecondary: true,
                          onPressed: _discoveringPairing
                              ? null
                              : () =>
                                    _confirmSwitch(MobileDeploymentMode.cloud),
                        ),
                        SizedBox(height: AppSpacing.sm),
                        Row(
                          children: [
                            Expanded(
                              child: AmitiaButton(
                                label: '扫码',
                                icon: Icons.qr_code_scanner,
                                isSecondary: true,
                                onPressed: () =>
                                    _openCloudPairing(action: 'scan'),
                              ),
                            ),
                            SizedBox(width: AppSpacing.sm),
                            Expanded(
                              child: AmitiaButton(
                                label: '使用二维码图片',
                                icon: Icons.image_outlined,
                                isSecondary: true,
                                onPressed: () =>
                                    _openCloudPairing(action: 'image'),
                              ),
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
                SizedBox(height: AppSpacing.sm),
                _SectionLabel(text: '当前配置'),
                SizedBox(height: AppSpacing.sm),
                _buildConfigCard(connectionAsync),
                SizedBox(height: AppSpacing.sectionGap),
                Padding(
                  padding: EdgeInsets.symmetric(
                    horizontal: AppSpacing.pagePadding,
                  ),
                  child: AmitiaButton(
                    label: _testState ? '检查中...' : '检查运行时状态',
                    icon: Icons.wifi_protected_setup,
                    isFullWidth: true,
                    onPressed: _testState ? null : _checkRuntimeStatus,
                  ),
                ),
                SizedBox(height: AppSpacing.xl),
              ],
            ),
    );
  }

  Widget _buildConfigCard(
    AsyncValue<BackendConnectionAvailability> connectionAsync,
  ) {
    final statusAsync = ref.watch(mobileBackendStatusProvider);
    final deployment = ref.read(mobileDeploymentConfigProvider);
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        children: [
          _InfoRow(
            label: '当前中心',
            value: _modeDisplayName(
              ref.read(mobileDeploymentConfigProvider).mode,
            ),
          ),
          connectionAsync.when(
            data: (avail) {
              final remoteUri = deployment.remoteCoreUri?.trim() ?? '';
              if (deployment.mode == MobileDeploymentMode.cloud &&
                  remoteUri.isNotEmpty) {
                try {
                  return _InfoRow(
                    label: '统筹中心地址',
                    value: normalizeRemoteCoreUri(remoteUri).toString(),
                  );
                } on DeploymentConfigValidationError {
                  return _InfoRow(label: '统筹中心地址', value: remoteUri);
                }
              }
              if (avail is BackendConnectionAvailable) {
                final uri = BackendUriBuilder().httpBase(avail.config);
                return _InfoRow(label: '后端地址', value: uri.toString());
              }
              return const _InfoRow(label: '后端地址', value: '运行环境未就绪');
            },
            loading: () => const _InfoRow(label: '后端地址', value: '检查中...'),
            error: (_, __) => const _InfoRow(label: '后端地址', value: '运行环境未就绪'),
          ),
          statusAsync.when(
            data: (status) =>
                _InfoRow(label: '运行状态', value: _statusDisplayName(status)),
            loading: () => const _InfoRow(label: '运行状态', value: '检查中...'),
            error: (_, __) => const _InfoRow(label: '运行状态', value: '检查失败'),
          ),
        ],
      ),
    );
  }

  String _modeDisplayName(MobileDeploymentMode mode) {
    switch (mode) {
      case MobileDeploymentMode.local:
        return '以本机作为统筹中心';
      case MobileDeploymentMode.cloud:
        return '使用远程统筹中心';
    }
  }

  String _statusDisplayName(MobileBackendStatus status) {
    switch (status.state) {
      case RuntimeDeploymentState.unavailable:
        return '未就绪';
      case RuntimeDeploymentState.starting:
        return '启动中...';
      case RuntimeDeploymentState.ready:
        return '已就绪';
      case RuntimeDeploymentState.stopping:
        return '停止中...';
      case RuntimeDeploymentState.failed:
        return '连接失败';
    }
  }

  String? _normalizedRemoteUri({bool showError = true}) {
    final raw = _remoteUriController.text.trim();
    if (raw.isEmpty) {
      if (showError && mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('请输入统筹中心地址')));
      }
      return null;
    }
    try {
      return normalizeRemoteCoreUri(raw).toString();
    } on DeploymentConfigValidationError catch (error) {
      if (showError && mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('统筹中心地址无效：${error.message}')));
      }
      return null;
    } catch (error) {
      if (showError && mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('统筹中心地址无效：$error')));
      }
      return null;
    }
  }

  Future<void> _confirmSwitch(MobileDeploymentMode newMode) async {
    if (_discoveringPairing) return;
    final current = ref.read(mobileDeploymentConfigProvider);
    final remoteUri = newMode == MobileDeploymentMode.cloud
        ? _normalizedRemoteUri()
        : current.remoteCoreUri;
    if (!deploymentSelectionRequiresChange(current, newMode, remoteUri)) {
      if (newMode == MobileDeploymentMode.cloud &&
          remoteUri != null &&
          (current.remoteCoreUri?.trim() ?? '').isNotEmpty) {
        _show('当前已使用该统筹中心地址');
      }
      return;
    }
    if (newMode == MobileDeploymentMode.cloud) {
      if (remoteUri == null) {
        _show('请先填写统筹中心地址');
        return;
      }
      setState(() => _discoveringPairing = true);
      Map<String, dynamic> discovery;
      try {
        discovery = await ref
            .read(onboardingServiceProvider)
            .discoverPairingAt(remoteUri);
      } catch (error) {
        if (mounted) _show('检查统筹中心身份失败：$error');
        return;
      } finally {
        if (mounted) setState(() => _discoveringPairing = false);
      }
      if (!mounted) return;
      final coreId = (discovery['spaceId'] ?? '').toString().trim();
      if (coreId.isEmpty) {
        _show('统筹中心身份缺失');
        return;
      }
      final fingerprint = (discovery['fingerprint'] ?? '').toString().trim();
      var existingCoreId = '';
      if (current.mode == MobileDeploymentMode.cloud) {
        final localService = ref.read(deviceMeshLocalServiceProvider);
        if (localService != null) {
          try {
            final status = await localService.status();
            existingCoreId = (status['coreId'] ?? '').toString().trim();
          } catch (error, stackTrace) {
            print("[original-error] $error\n$stackTrace");
          }
        }
      }
      if (!mounted) return;
      showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('确认加入统筹中心'),
          content: Text(
            '中心地址\n$remoteUri\n\n'
            '中心标识\n$coreId\n\n'
            '${fingerprint.isEmpty ? '中心证书由系统信任。' : '中心证书指纹\n$fingerprint'}\n\n'
            '二维码只是自动填入相同信息。发起后仍需由统筹中心确认。',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('取消'),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(ctx);
                if (existingCoreId == coreId) {
                  await _updateKnownCenterEndpoint(
                    remoteUri: remoteUri,
                    fingerprint: fingerprint,
                    coreId: coreId,
                  );
                } else {
                  _openCloudPairing(
                    uri: remoteUri,
                    action: 'address',
                    fingerprint: fingerprint,
                    coreId: coreId,
                  );
                }
              },
              child: const Text('发起配对'),
            ),
          ],
        ),
      );
      return;
    }

    showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('切换统筹中心', style: AppTypography.cardTitle(context)),
        content: Text(
          '确定要切换为「${_modeDisplayName(newMode)}」吗？业务连接会重新建立，当前运行数据不迁移。',
          style: AppTypography.body(context),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              await _applyModeChange(newMode, remoteUri: remoteUri);
            },
            child: Text('确定', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  Future<void> _updateKnownCenterEndpoint({
    required String remoteUri,
    required String fingerprint,
    required String coreId,
  }) async {
    final service = ref.read(deviceMeshLocalServiceProvider);
    if (service == null) {
      _show('本机 Runtime 当前不可用，无法更新统筹中心地址');
      return;
    }
    try {
      await service.updateEndpoint(
        cloudBaseUrl: remoteUri,
        fingerprint: fingerprint,
        coreId: coreId,
      );
      await _applyModeChange(MobileDeploymentMode.cloud, remoteUri: remoteUri);
      _show('统筹中心地址已更新');
    } catch (error) {
      _show('更新统筹中心地址失败：$error');
    }
  }

  void _openCloudPairing({
    String? uri,
    String? action,
    String? fingerprint,
    String? coreId,
  }) {
    var cloudUri = uri?.trim() ?? '';
    if (cloudUri.isEmpty && (action == null || action.isEmpty)) {
      cloudUri = _normalizedRemoteUri() ?? '';
      if (cloudUri.isEmpty) return;
    }
    final query = <String, String>{
      if (cloudUri.isNotEmpty) 'endpoint': cloudUri,
      if (action != null && action.isNotEmpty) 'action': action,
      if (fingerprint != null && fingerprint.isNotEmpty)
        'fingerprint': fingerprint,
      if (coreId != null && coreId.isNotEmpty) 'coreId': coreId,
    };
    context.push(
      Uri(path: AppRoutes.settingsDeviceAdd, queryParameters: query).toString(),
    );
  }

  void _show(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  Future<void> _applyModeChange(
    MobileDeploymentMode newMode, {
    String? remoteUri,
  }) async {
    final config = ref.read(mobileDeploymentConfigProvider);
    final newConfig = MobileDeploymentConfig(
      mode: newMode,
      remoteCoreUri: remoteUri ?? config.remoteCoreUri,
    );
    final notificationCoordinator = ref.read(notificationCoordinatorProvider);
    await notificationCoordinator
        .revokeCurrentRegistrationForDeploymentTransition();
    try {
      await ref.read(mobileDeploymentConfigProvider.notifier).update(newConfig);
      ref.invalidate(backendConnectionProvider);
      await ref.read(mobileBackendLifecycleProvider).reconcile(newConfig);
      ref.invalidate(mobileBackendStatusProvider);
      await ref.read(mobileBackendStatusProvider.future);
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      await ref.read(mobileDeploymentConfigProvider.notifier).update(config);
      ref.invalidate(backendConnectionProvider);
      try {
        await ref.read(mobileBackendLifecycleProvider).reconcile(config);
      } catch (error, stackTrace) {
        print("[original-error] $error\n$stackTrace");
      }
      rethrow;
    } finally {
      notificationCoordinator.resumeRegistrationAfterDeploymentTransition();
    }
    if (!mounted) return;
    setState(() {
      _testState = false;
      _selectedMode = newMode;
    });
    _remoteUriController.text = newConfig.remoteCoreUri ?? '';
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('已切换为${_modeDisplayName(newMode)}模式'),
        duration: const Duration(seconds: 1),
      ),
    );
  }

  Future<void> _checkRuntimeStatus() async {
    setState(() => _testState = true);
    try {
      final config = ref.read(mobileDeploymentConfigProvider);
      await ref.read(mobileBackendLifecycleProvider).reconcile(config);
      ref.invalidate(mobileBackendStatusProvider);
      await ref.read(mobileBackendStatusProvider.future);
      ref.invalidate(backendConnectionProvider);
      await ref.read(backendConnectionProvider.future);
    } finally {
      if (mounted) setState(() => _testState = false);
    }
  }
}

class _ModeCard extends StatelessWidget {
  final MobileDeploymentMode mode;
  final IconData icon;
  final String description;
  final bool isSelected;
  final VoidCallback? onTap;

  const _ModeCard({
    required this.mode,
    required this.icon,
    required this.description,
    required this.isSelected,
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
          border: Border.all(
            color: isSelected ? context.accentPrimary : context.borderPrimary,
            width: isSelected ? 2 : 0.5,
          ),
        ),
        child: Row(
          children: [
            Container(
              width: 48,
              height: 48,
              decoration: BoxDecoration(
                color: isSelected
                    ? context.accentSoft
                    : context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(
                icon,
                size: 24,
                color: isSelected
                    ? context.accentPrimary
                    : context.textSecondary,
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    mode == MobileDeploymentMode.local
                        ? '以本机作为统筹中心'
                        : '使用远程统筹中心',
                    style: AppTypography.cardTitle(context),
                  ),
                  const SizedBox(height: 2),
                  Text(description, style: AppTypography.caption(context)),
                ],
              ),
            ),
            Icon(
              isSelected ? Icons.check_circle : Icons.radio_button_off,
              size: 22,
              color: isSelected ? context.accentPrimary : context.textTertiary,
            ),
          ],
        ),
      ),
    );
  }
}

class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  const _InfoRow({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 7),
      child: Row(
        children: [
          SizedBox(
            width: 72,
            child: Text(label, style: AppTypography.label(context)),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Text(
              value,
              style: AppTypography.bodySmall(context),
              textAlign: TextAlign.end,
            ),
          ),
        ],
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.sm,
        AppSpacing.pagePadding,
        AppSpacing.sm,
      ),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
