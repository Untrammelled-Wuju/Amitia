import 'dart:async';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';
import 'package:mobile_scanner/mobile_scanner.dart';
import 'package:qr_flutter/qr_flutter.dart';
import 'device_pair_scan_page.dart';

import '../../../../app/amitia_page_route.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/notification_runtime_bootstrap.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/errors/original_error.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/device_management_intent.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DeviceAddPage extends ConsumerStatefulWidget {
  const DeviceAddPage({super.key});

  @override
  ConsumerState<DeviceAddPage> createState() => _DeviceAddPageState();
}

String pairingEndpointFor(MobileDeploymentConfig deployment, Uri routeUri) {
  final routed = routeUri.queryParameters['endpoint']?.trim() ?? '';
  return routed.isNotEmpty ? routed : (deployment.remoteCoreUri?.trim() ?? '');
}

({String endpoint, String fingerprint, String coreId}) pairingIdentityFor(
  MobileDeploymentConfig deployment,
  Uri routeUri,
  String raw,
) {
  var endpoint = pairingEndpointFor(deployment, routeUri);
  final scanned = raw.startsWith('amitia://') ? Uri.tryParse(raw) : null;
  final fingerprint =
      scanned?.queryParameters['fingerprint'] ??
      routeUri.queryParameters['fingerprint'] ??
      '';
  final coreId =
      scanned?.queryParameters['core'] ??
      routeUri.queryParameters['coreId'] ??
      routeUri.queryParameters['core'] ??
      '';
  if (fingerprint.isNotEmpty) {
    if (!RegExp(r'^[0-9a-f]{64}$').hasMatch(fingerprint) || coreId.isEmpty) {
      throw const FormatException('二维码身份信息无效');
    }
    endpoint = scanned?.queryParameters['endpoint'] ?? endpoint;
  }
  return (endpoint: endpoint, fingerprint: fingerprint, coreId: coreId);
}

bool isPairingRouteAction(String? action) {
  return action == 'address' || action == 'scan' || action == 'image';
}

bool isDevicePairingCenter(
  MobileDeploymentConfig deployment,
  String? routeAction,
) {
  return deployment.mode == MobileDeploymentMode.local &&
      !isPairingRouteAction(routeAction);
}

class _DeviceAddPageState extends ConsumerState<DeviceAddPage> {
  final _labelController = TextEditingController();
  final _pairingController = TextEditingController();
  bool _submitting = false;
  bool _generating = false;
  String _offerPayload = '';
  List<Map<String, dynamic>> _approvals = const [];
  StreamSubscription<List<Map<String, dynamic>>>? _approvalSubscription;
  CancelToken? _approvalCancel;
  Timer? _approvalReconnectTimer;
  int _approvalReconnectAttempt = 0;
  bool _refreshingApprovals = false;
  bool _routeActionOpened = false;
  final _approvalNotifier = ValueNotifier<List<Map<String, dynamic>>>(const []);
  final _approvalBusyNotifier = ValueNotifier<String>('');
  bool _approvalDialogOpen = false;
  BuildContext? _approvalDialogContext;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      unawaited(_refreshApprovals());
      unawaited(_startApprovalEvents());
      unawaited(_runRouteAction());
    });
  }

  Future<void> _runRouteAction() async {
    if (_routeActionOpened || !mounted) return;
    final action = GoRouterState.of(context).uri.queryParameters['action'];
    if (!isPairingRouteAction(action)) return;
    _routeActionOpened = true;
    if (action == 'address') {
      await _bindCurrentDevice();
    } else if (action == 'scan') {
      await _scanPairing();
    } else {
      await _pickPairingImage();
    }
  }

  Future<void> _startApprovalEvents() async {
    if (!mounted || _approvalSubscription != null) return;
    final service = ref.read(deviceMeshServiceProvider);
    final api = ref.read(rawBackendServiceApiProvider);
    final apiGeneration = api?.generation;
    final deployment = ref.read(mobileDeploymentConfigProvider);
    final policy = await service.coordination();
    final intent = DeviceManagementIntent(
      policy,
      isCurrent: () =>
          mounted &&
          api != null &&
          api.generation == apiGeneration &&
          identical(ref.read(rawBackendServiceApiProvider), api) &&
          ref.read(mobileDeploymentConfigProvider) == deployment,
    );
    if (!intent.canAdminister || !mounted) return;
    final cancel = CancelToken();
    _approvalCancel = cancel;
    _approvalSubscription = service
        .approvalEvents(cancel)
        .listen(
          (approvals) {
            if (!mounted) return;
            _setApprovals(
              approvals
                  .map(
                    (row) => Map<String, dynamic>.unmodifiable({
                      ...row,
                      '_managementIntent': intent,
                    }),
                  )
                  .toList(growable: false),
            );
          },
          onError: (Object error, StackTrace stackTrace) {
            print("[original-error] $error\n$stackTrace");
            _finishApprovalStream();
            _scheduleApprovalReconnect();
          },
          onDone: () {
            _finishApprovalStream();
            _scheduleApprovalReconnect();
          },
          cancelOnError: true,
        );
    _approvalReconnectAttempt = 0;
  }

  void _finishApprovalStream() {
    final subscription = _approvalSubscription;
    _approvalSubscription = null;
    if (subscription != null) unawaited(subscription.cancel());
    _approvalCancel?.cancel();
    _approvalCancel = null;
  }

  void _scheduleApprovalReconnect() {
    if (!mounted || _approvalReconnectTimer != null) return;
    final exponent = _approvalReconnectAttempt > 5
        ? 5
        : _approvalReconnectAttempt;
    final seconds = 1 << exponent;
    final delay = Duration(seconds: seconds > 30 ? 30 : seconds);
    _approvalReconnectAttempt++;
    _approvalReconnectTimer = Timer(delay, () {
      _approvalReconnectTimer = null;
      unawaited(_startApprovalEvents());
    });
  }

  Future<void> _refreshApprovals() async {
    if (_refreshingApprovals || !mounted) return;
    _refreshingApprovals = true;
    try {
      final service = ref.read(deviceMeshServiceProvider);
      final api = ref.read(rawBackendServiceApiProvider);
      final apiGeneration = api?.generation;
      final deployment = ref.read(mobileDeploymentConfigProvider);
      final policy = await service.coordination();
      final intent = DeviceManagementIntent(
        policy,
        isCurrent: () =>
            mounted &&
            api != null &&
            api.generation == apiGeneration &&
            identical(ref.read(rawBackendServiceApiProvider), api) &&
            ref.read(mobileDeploymentConfigProvider) == deployment,
      );
      final approvals = intent.canAdminister
          ? await service.pendingApprovals()
          : const <Map<String, dynamic>>[];
      intent.validate(await service.coordination());
      if (mounted)
        _setApprovals(
          approvals
              .map(
                (row) => Map<String, dynamic>.unmodifiable({
                  ...row,
                  '_managementIntent': intent,
                }),
              )
              .toList(growable: false),
        );
      if (mounted && intent.canAdminister) {
        unawaited(_startApprovalEvents());
      }
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      if (mounted) _setApprovals(const []);
    } finally {
      _refreshingApprovals = false;
    }
  }

  Future<void> _decideApproval(Map<String, dynamic> request, bool allow) async {
    final intent = request['_managementIntent'];
    final requestId = request['requestId']?.toString() ?? '';
    _setApprovalBusy(requestId);
    try {
      if (intent is! DeviceManagementIntent) throw StateError('原审批范围缺失，请重新加载');
      intent.requireAdministrator();
      intent.validate(await ref.read(deviceMeshServiceProvider).coordination());
      await ref
          .read(deviceMeshServiceProvider)
          .decideApproval(
            requestId,
            allow: allow,
            expectedRevision: (request['revision'] as num).toInt(),
            headers: intent.headers,
          );
      intent.validate(await ref.read(deviceMeshServiceProvider).coordination());
      await _refreshApprovals();
      if (mounted) _show(allow ? '已批准设备配对' : '已拒绝设备配对');
    } catch (error) {
      if (mounted) _show(_message(error));
    } finally {
      if (mounted) _setApprovalBusy('');
    }
  }

  @override
  void dispose() {
    _approvalReconnectTimer?.cancel();
    _finishApprovalStream();
    _labelController.dispose();
    _pairingController.dispose();
    _approvalNotifier.dispose();
    _approvalBusyNotifier.dispose();
    super.dispose();
  }

  void _setApprovals(List<Map<String, dynamic>> approvals) {
    if (!mounted) return;
    final next = List<Map<String, dynamic>>.unmodifiable(approvals);
    setState(() => _approvals = next);
    _approvalNotifier.value = next;
    if (next.isEmpty) {
      final dialogContext = _approvalDialogContext;
      if (dialogContext != null && _approvalDialogOpen) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted && Navigator.of(dialogContext).canPop()) {
            Navigator.of(dialogContext).pop();
          }
        });
      }
      return;
    }
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _openApprovalDialog();
    });
  }

  void _setApprovalBusy(String requestId) {
    _approvalBusyNotifier.value = requestId;
  }

  void _openApprovalDialog() {
    if (!mounted ||
        _approvalDialogOpen ||
        _approvals.isEmpty ||
        ref.read(mobileDeploymentConfigProvider).mode !=
            MobileDeploymentMode.local) {
      return;
    }
    _approvalDialogOpen = true;
    showDialog<void>(
      context: context,
      builder: (dialogContext) {
        _approvalDialogContext = dialogContext;
        return AlertDialog(
          title: const Text('子设备配对请求'),
          content: SizedBox(
            width: 420,
            child: AnimatedBuilder(
              animation: Listenable.merge([
                _approvalNotifier,
                _approvalBusyNotifier,
              ]),
              builder: (context, _) {
                final requests = _approvalNotifier.value;
                if (requests.isEmpty) {
                  return const SizedBox.shrink();
                }
                return Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      '以下子设备正在请求加入当前统筹中心，请逐台确认。',
                      style: AppTypography.caption(context),
                    ),
                    const SizedBox(height: 12),
                    Flexible(
                      child: ListView.separated(
                        shrinkWrap: true,
                        itemCount: requests.length,
                        separatorBuilder: (_, __) => const Divider(height: 20),
                        itemBuilder: (context, index) {
                          final request = requests[index];
                          final requestId =
                              request['requestId']?.toString() ?? '';
                          final label =
                              request['label']?.toString().trim() ?? '';
                          final deviceId =
                              request['deviceId']?.toString() ?? '';
                          final busy = _approvalBusyNotifier.value.isNotEmpty;
                          return Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                label.isEmpty ? deviceId : label,
                                style: AppTypography.body(context),
                              ),
                              const SizedBox(height: 2),
                              Text(
                                '${request['platform'] ?? ''} · $deviceId',
                                style: AppTypography.caption(context),
                              ),
                              const SizedBox(height: 8),
                              Row(
                                mainAxisAlignment: MainAxisAlignment.end,
                                children: [
                                  TextButton(
                                    onPressed: busy
                                        ? null
                                        : () => _decideApproval(request, false),
                                    child: const Text('拒绝'),
                                  ),
                                  FilledButton(
                                    onPressed: busy
                                        ? null
                                        : () => _decideApproval(request, true),
                                    child: Text(
                                      _approvalBusyNotifier.value == requestId
                                          ? '处理中…'
                                          : '批准配对',
                                    ),
                                  ),
                                ],
                              ),
                            ],
                          );
                        },
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.of(dialogContext).pop(),
              child: const Text('关闭'),
            ),
          ],
        );
      },
    ).whenComplete(() {
      _approvalDialogOpen = false;
      _approvalDialogContext = null;
    });
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(rawBackendServiceApiProvider, (_, __) {
      if (mounted) {
        _finishApprovalStream();
        _setApprovals(const []);
        unawaited(_startApprovalEvents());
      }
    });
    ref.listen(mobileDeploymentConfigProvider, (_, __) {
      if (mounted) {
        _finishApprovalStream();
        _setApprovals(const []);
        unawaited(_startApprovalEvents());
      }
    });
    final deployment = ref.watch(mobileDeploymentConfigProvider);
    final identity = ref.watch(localDeviceMeshIdentityProvider);
    final status = ref.watch(localDeviceMeshStatusProvider);
    final cloudUri = pairingEndpointFor(
      deployment,
      GoRouterState.of(context).uri,
    );
    final cloudReady = cloudUri.isNotEmpty;
    final routeAction = GoRouterState.of(context).uri.queryParameters['action'];
    final isCenter = isDevicePairingCenter(deployment, routeAction);
    final pairingTitle = isCenter
        ? '邀请子设备加入统筹中心'
        : _submitting
        ? '等待统筹中心确认'
        : '扫描统筹中心配对码';
    final pairingDescription = isCenter
        ? '当前设备是统筹中心。生成二维码后，由子设备扫描加入；中心设备不会扫描自己的配对码。'
        : _submitting
        ? '配对请求已发送到统筹中心。请在中心设备批准；批准后本机会自动完成绑定并切换连接。'
        : '当前设备是子设备。请扫描中心设备生成的二维码，或从相册选择二维码图片。';

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
            icon: isCenter
                ? Icons.hub_outlined
                : _submitting
                ? Icons.hourglass_top_outlined
                : Icons.qr_code_scanner,
            title: pairingTitle,
            description: pairingDescription,
          ),
          SizedBox(height: AppSpacing.lg),
          Text('本机身份', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          _AsyncIdentityCard(
            identity: identity,
            status: status,
            isCenter: isCenter,
          ),
          if (!isCenter && !_submitting) ...[
            SizedBox(height: AppSpacing.lg),
            _ConnectionCard(
              cloudReady: cloudReady,
              cloudUri: cloudUri,
              onOpenDeployment: () =>
                  context.push(AppRoutes.settingsDeployment),
            ),
          ],
          if (isCenter) ...[
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: _generating ? '正在生成…' : '生成配对新设备二维码',
              icon: Icons.qr_code_2,
              isFullWidth: true,
              onPressed: _generating ? null : _generateOffer,
            ),
            if (_offerPayload.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              _CompactCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('二维码内容 / Offer', style: AppTypography.label(context)),
                    Center(
                      child: QrImageView(
                        data: _offerPayload,
                        size: 280,
                        backgroundColor: Colors.white,
                      ),
                    ),
                    const SizedBox(height: 8),
                    SelectableText(
                      _offerPayload,
                      style: AppTypography.bodySmall(context),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      '二维码在 30 分钟内可让多台子设备扫描；每台设备仍需单独确认审批。',
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
            ],
          ],
          if (!isCenter && !_submitting) ...[
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
            AmitiaButton(
              label: '扫描另一台设备的配对码',
              icon: Icons.qr_code_scanner,
              isFullWidth: true,
              onPressed: _submitting ? null : _scanPairing,
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaButton(
              label: '使用二维码图片',
              icon: Icons.image_outlined,
              isFullWidth: true,
              isSecondary: true,
              onPressed: _submitting ? null : _pickPairingImage,
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaTextField(
              controller: _pairingController,
              hintText: '粘贴 amitia://pair?...、Offer Token 或首设备设置码',
              prefixIcon: const Icon(Icons.qr_code_scanner_outlined),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(
              label: _submitting ? '正在配对…' : '配对并加入统筹中心',
              icon: Icons.link_outlined,
              isFullWidth: true,
              onPressed: !_submitting ? _bindCurrentDevice : null,
            ),
            SizedBox(height: AppSpacing.sm),
            Text(
              '首次设备使用统筹中心本机显示的设置码；后续设备使用中心生成的 30 分钟有效配对二维码。Device ID 不是密码。',
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
      if (ref.read(mobileDeploymentConfigProvider).mode !=
          MobileDeploymentMode.local) {
        throw StateError('子设备不能生成统筹中心二维码');
      }
      var endpoint = '';
      final endpoints =
          await ref.read(deviceMeshLocalServiceProvider)?.lanEndpoints() ??
          const <Map<String, dynamic>>[];
      if (endpoints.isEmpty) throw StateError('当前设备没有可用的局域网加密地址');
      endpoint = endpoints.first['url']?.toString() ?? '';
      final offer = await ref
          .read(deviceMeshServiceProvider)
          .createPairingOffer(endpoint: endpoint, ttlSeconds: 1800);
      final token = (offer['offerToken'] ?? '').toString().trim();
      if (token.isEmpty) throw StateError('统筹中心未返回有效配对 Offer');
      final payload = (offer['qrPayload'] ?? '').toString();
      if (payload.isEmpty) throw StateError('Core 未返回设备配对二维码');
      if (!mounted) return;
      setState(() => _offerPayload = payload);
    } catch (error) {
      if (mounted) _show('生成失败：${_message(error)}');
    } finally {
      if (mounted) setState(() => _generating = false);
    }
  }

  Future<void> _scanPairing() async {
    final raw = await Navigator.of(context).push<String>(
      amitiaSlideRoute(builder: (_) => const DevicePairScanPage()),
    );
    if (!mounted || raw == null || raw.isEmpty) return;
    await _acceptDetectedPairing(raw);
  }

  Future<void> _pickPairingImage() async {
    final image = await ImagePicker().pickImage(source: ImageSource.gallery);
    if (!mounted || image == null) return;
    final scanner = MobileScannerController(
      formats: const [BarcodeFormat.qrCode],
    );
    try {
      final capture = await scanner.analyzeImage(image.path);
      String raw = '';
      for (final barcode in capture?.barcodes ?? const <Barcode>[]) {
        final value = barcode.rawValue?.trim() ?? '';
        final uri = Uri.tryParse(value);
        if (uri?.scheme == 'amitia' && uri?.host == 'pair') {
          raw = value;
          break;
        }
      }
      if (raw.isEmpty) throw StateError('图片中没有有效的 Amitia 配对二维码');
      if (!mounted) return;
      await _acceptDetectedPairing(raw);
    } catch (error) {
      if (mounted) _show('图片识别失败：${_message(error)}');
    } finally {
      await scanner.dispose();
    }
  }

  Future<void> _acceptDetectedPairing(String raw) async {
    _pairingController.text = raw;
    final uri = Uri.tryParse(raw);
    final endpoint = uri?.queryParameters['endpoint']?.trim() ?? '';
    if (uri?.scheme != 'amitia' || uri?.host != 'pair' || endpoint.isEmpty) {
      _show('二维码身份信息无效');
      return;
    }
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('确认连接统筹中心'),
        content: Text('确认使用以下统筹中心完成配对？\n\n$endpoint'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: const Text('配对'),
          ),
        ],
      ),
    );
    if (!mounted || confirmed != true) return;
    await _bindCurrentDevice();
  }

  Future<void> _bindCurrentDevice() async {
    if (_submitting) return;
    final deployment = ref.read(mobileDeploymentConfigProvider);
    final routeUri = GoRouterState.of(context).uri;
    final raw = _pairingController.text.trim();
    late final ({String endpoint, String fingerprint, String coreId}) identity;
    try {
      identity = pairingIdentityFor(deployment, routeUri, raw);
    } on FormatException catch (error) {
      _show(error.message);
      return;
    }
    final cloudUri = identity.endpoint;
    final fingerprint = identity.fingerprint;
    final coreId = identity.coreId;
    if (fingerprint.isEmpty && cloudUri.isEmpty) {
      _show('请先填写统筹中心地址并完成配对');
      return;
    }
    setState(() => _submitting = true);
    var deploymentConfigChanged = false;
    var notificationHandoff = false;
    final notificationCoordinator = ref.read(notificationCoordinatorProvider);
    try {
      final localService = ref.read(deviceMeshLocalServiceProvider);
      final identity = await localService?.identity();
      if (identity == null) {
        throw StateError('本机 Runtime 当前不可用，无法读取 Device Mesh 身份');
      }
      final deviceId = (identity['deviceId'] ?? '').toString().trim();
      final runtimeId = (identity['runtimeId'] ?? '').toString().trim();
      final platform = (identity['platform'] ?? '').toString().trim();
      if (deviceId.isEmpty || runtimeId.isEmpty || platform.isEmpty) {
        throw StateError('本机 Device Mesh 身份不完整');
      }

      final status = fingerprint.isEmpty
          ? await ref.read(onboardingServiceProvider).pairingStatusAt(cloudUri)
          : <String, dynamic>{'firstDeviceSetupRequired': false};
      final first = status['firstDeviceSetupRequired'] == true;
      if (first && raw.isEmpty) throw StateError('请输入首设备设置码');
      final offerToken = first ? '' : _extractOffer(raw, cloudUri);
      final setupCode = first ? raw : '';
      if (!first && raw.isNotEmpty && offerToken.isEmpty) {
        throw StateError('配对 Offer 无效');
      }

      final claim = await ref
          .read(onboardingServiceProvider)
          .claimPairingAt(
            cloudUri,
            deviceId: deviceId,
            runtimeId: runtimeId,
            platform: platform,
            label: _labelController.text.trim(),
            offerToken: offerToken,
            setupCode: setupCode,
            fingerprint: fingerprint,
            coreId: coreId,
          );
      final ticket = (claim['ticket'] ?? '').toString().trim();
      if (ticket.isEmpty) throw StateError('统筹中心未返回 Bootstrap Ticket');

      if (localService == null) {
        throw StateError('本机 Device Agent 不可用，无法保存设备凭据');
      }
      await localService.bootstrap(
        cloudBaseUrl: cloudUri,
        bootstrapTicket: ticket,
        fingerprint: fingerprint,
        coreId: coreId,
      );
      final previous = ref.read(mobileDeploymentConfigProvider);
      final next = MobileDeploymentConfig(
        mode: MobileDeploymentMode.cloud,
        remoteCoreUri: cloudUri,
      );
      if (previous.mode != next.mode ||
          previous.remoteCoreUri?.trim() != next.remoteCoreUri?.trim()) {
        await notificationCoordinator
            .revokeCurrentRegistrationForDeploymentTransition();
        notificationHandoff = true;
      }
      await ref.read(mobileDeploymentConfigProvider.notifier).update(next);
      deploymentConfigChanged = true;
      ref.invalidate(localDeviceMeshStatusProvider);
      ref.invalidate(localDeviceMeshIdentityProvider);
      ref.invalidate(deviceMeshDevicesProvider);
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      await ref.read(backendConnectionProvider.future);
      await ref.read(backendTransportProvider.future);
      await ref.read(mobileBackendLifecycleProvider).reconcile(next);
      ref.invalidate(mobileBackendStatusProvider);
      if (!mounted) return;
      _show('设备已加入统筹中心');
      context.pop();
    } catch (error) {
      if (deploymentConfigChanged) {
        await ref
            .read(mobileDeploymentConfigProvider.notifier)
            .update(deployment);
        ref.invalidate(backendConnectionProvider);
        ref.invalidate(backendTransportProvider);
        try {
          await ref.read(mobileBackendLifecycleProvider).reconcile(deployment);
        } catch (error, stackTrace) {
          print("[original-error] $error\n$stackTrace");
        }
      }
      if (mounted) _show('配对失败：${_message(error)}');
    } finally {
      if (notificationHandoff) {
        notificationCoordinator.resumeRegistrationAfterDeploymentTransition();
      }
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
      if (expected == null ||
          offered == null ||
          expected.scheme != offered.scheme ||
          expected.host != offered.host ||
          expected.port != offered.port) {
        throw StateError('配对 Offer 属于另一个统筹中心');
      }
    }
    return uri.queryParameters['offer']?.trim() ?? '';
  }

  void _show(String message) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(message)));
  }

  static String _message(Object error) => originalErrorText(
    error,
  ).replaceFirst('Bad state: ', '').replaceFirst('Exception: ', '');
}

class _AsyncIdentityCard extends StatelessWidget {
  final AsyncValue<Map<String, dynamic>?> identity;
  final AsyncValue<Map<String, dynamic>?> status;
  final bool isCenter;

  const _AsyncIdentityCard({
    required this.identity,
    required this.status,
    required this.isCenter,
  });

  @override
  Widget build(BuildContext context) {
    return identity.when(
      loading: () => const _CompactCard(
        child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
      ),
      error: (error, _) => _CompactCard(
        child: Text(
          '无法读取本机身份：${error.toString().replaceFirst('Exception: ', '')}',
          style: AppTypography.caption(context).copyWith(color: context.error),
        ),
      ),
      data: (data) {
        if (data == null) {
          return _CompactCard(
            child: Text(
              '本机 Runtime 尚未就绪。',
              style: AppTypography.caption(context),
            ),
          );
        }
        final state = status.asData?.value?['state']?.toString() ?? 'unknown';
        return _CompactCard(
          child: Column(
            children: [
              _ValueRow(
                label: 'Device ID',
                value: (data['deviceId'] ?? '-').toString(),
              ),
              _ValueRow(
                label: 'Runtime ID',
                value: (data['runtimeId'] ?? '-').toString(),
              ),
              _ValueRow(
                label: '平台',
                value: (data['platform'] ?? '-').toString(),
              ),
              _ValueRow(
                label: 'Mesh 状态',
                value: isCenter ? '本机统筹中心' : _stateLabel(state),
                isLast: true,
              ),
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
        return '未配对';
      default:
        return raw.isEmpty ? '未知' : raw;
    }
  }
}

class _ConnectionCard extends StatelessWidget {
  final bool cloudReady;
  final String cloudUri;
  final VoidCallback onOpenDeployment;

  const _ConnectionCard({
    required this.cloudReady,
    required this.cloudUri,
    required this.onOpenDeployment,
  });

  @override
  Widget build(BuildContext context) {
    return _CompactCard(
      child: Row(
        children: [
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Icon(
              Icons.cloud_outlined,
              color: context.accentPrimary,
              size: 19,
            ),
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  cloudReady ? '远程统筹中心已配置' : '需要选择远程统筹中心',
                  style: AppTypography.body(context),
                ),
                const SizedBox(height: 2),
                Text(
                  cloudReady ? cloudUri : '先填写统筹中心服务地址',
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
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

  const _InfoCard({
    required this.icon,
    required this.title,
    required this.description,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(15),
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brMedium,
        border: Border.all(
          color: context.accentPrimary.withValues(alpha: 0.14),
        ),
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

  const _ValueRow({
    required this.label,
    required this.value,
    this.isLast = false,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 8),
      decoration: BoxDecoration(
        border: isLast
            ? null
            : Border(
                bottom: BorderSide(color: context.borderSecondary, width: 0.6),
              ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 82,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          Expanded(
            child: Text(
              value,
              style: AppTypography.body(context),
              textAlign: TextAlign.right,
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }
}
