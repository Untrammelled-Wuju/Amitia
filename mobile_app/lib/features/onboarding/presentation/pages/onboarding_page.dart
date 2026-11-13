import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/notification_runtime_bootstrap.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/core_configuration_intent.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/errors/original_error.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/services/core_configuration_guard.dart';
import '../../../../core/services/onboarding_service.dart';
import '../../../../core/services/providers.dart';
import '../steps/onboarding_center_step.dart';
import '../steps/onboarding_model_step.dart';
import '../steps/onboarding_runtime_step.dart';
import '../widgets/onboarding_consent_gate.dart';
import '../widgets/onboarding_controls.dart';
import '../widgets/onboarding_step_scaffold.dart';

class OnboardingPage extends ConsumerStatefulWidget {
  const OnboardingPage({super.key});

  @override
  ConsumerState<OnboardingPage> createState() => _OnboardingPageState();
}

class _OnboardingPageState extends ConsumerState<OnboardingPage> {
  static const _stepCount = 3;
  static const _stepNames = ['统筹中心', '运行环境', '语言模型'];
  static const _consentKey = 'onboarding_consent_accepted';

  int _currentStep = 0;
  bool _consentLoaded = false;
  bool _consentAccepted = false;
  bool _agreed = false;

  int _deployMode = 0;
  final _remoteCoreCtrl = TextEditingController();
  bool _remoteChecking = false;
  String _remoteStatus = '';

  List<OnboardingCheckState> _checkRows = const [];
  bool _envChecking = false;
  bool _connectionFailed = false;
  String _failureDetail = '';

  bool _devicePaired = false;
  bool _pairingRequired = false;
  bool _firstDeviceSetupRequired = false;
  bool _pairingBusy = false;
  String _pairingError = '';
  final _pairingCodeCtrl = TextEditingController();

  final _providerCtrl = TextEditingController(text: 'DeepSeek');
  final _baseUrlCtrl = TextEditingController(text: 'https://api.deepseek.com/v1');
  final _modelCtrl = TextEditingController(text: 'deepseek-chat');
  final _keyCtrl = TextEditingController();
  bool _detecting = false;
  bool _modelStatusOk = false;
  String _modelStatus = '';
  List<String> _detectedModels = const [];
  bool _showKey = false;
  String? _textConfigId;

  CoreConfigurationIntent? _modelIntent;
  bool _skipCoreModels = false;
  bool _submitting = false;
  String _stepError = '';

  @override
  void initState() {
    super.initState();
    _loadConsent();
  }

  @override
  void dispose() {
    _remoteCoreCtrl.dispose();
    _pairingCodeCtrl.dispose();
    _providerCtrl.dispose();
    _baseUrlCtrl.dispose();
    _modelCtrl.dispose();
    _keyCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadConsent() async {
    var accepted = false;
    try {
      final preferences = await SharedPreferences.getInstance();
      accepted = preferences.getBool(_consentKey) ?? false;
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
    }
    final config = ref.read(mobileDeploymentConfigProvider);
    if (!mounted) return;
    setState(() {
      _consentAccepted = accepted;
      _consentLoaded = true;
      _deployMode = config.mode == MobileDeploymentMode.cloud ? 1 : 0;
      _remoteCoreCtrl.text = config.remoteCoreUri ?? '';
    });
  }

  Future<void> _acceptConsent() async {
    if (!_agreed) return;
    try {
      final preferences = await SharedPreferences.getInstance();
      await preferences.setBool(_consentKey, true);
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
    }
    if (!mounted) return;
    setState(() => _consentAccepted = true);
  }

  List<String> _checkNames() {
    return _deployMode == 0
        ? const ['检查本地服务', '检查运行时就绪状态', '检查运行时能力']
        : const ['检查统筹中心', '读取统筹数据身份', '检查设备配对服务', '确认运行时兼容性'];
  }

  bool get _envAllDone {
    return _checkRows.isNotEmpty &&
        _checkRows.every((row) => row.state == OnboardingCheckPhase.done);
  }

  Future<void> _checkRemoteCore() async {
    if (_remoteChecking) return;
    final remoteCore = _remoteCoreCtrl.text.trim();
    if (remoteCore.isEmpty) {
      setState(() => _remoteStatus = '请填写统筹中心地址');
      return;
    }
    setState(() {
      _remoteChecking = true;
      _remoteStatus = '';
    });
    try {
      final health = await ref
          .read(onboardingServiceProvider)
          .healthAt(remoteCore);
      if (health.isEmpty) {
        throw StateError('远程统筹中心健康检查未返回数据');
      }
      if (!mounted) return;
      setState(() => _remoteStatus = '连接成功');
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() => _remoteStatus = originalErrorText(error));
    } finally {
      if (mounted) setState(() => _remoteChecking = false);
    }
  }

  Future<void> _applyDeploymentSelection() async {
    final config = _deployMode == 0
        ? MobileDeploymentConfig.local
        : MobileDeploymentConfig(
            mode: MobileDeploymentMode.cloud,
            remoteCoreUri: _remoteCoreCtrl.text.trim(),
          );
    final validationError = validateDeploymentConfigForSave(config);
    if (validationError != null) throw validationError;

    final deploymentNotifier = ref.read(
      mobileDeploymentConfigProvider.notifier,
    );
    final previousConfig = ref.read(mobileDeploymentConfigProvider);
    final deploymentChanged =
        previousConfig.mode != config.mode ||
        previousConfig.remoteCoreUri?.trim() != config.remoteCoreUri?.trim();
    final notificationCoordinator = ref.read(notificationCoordinatorProvider);
    try {
      if (deploymentChanged) {
        await notificationCoordinator
            .revokeCurrentRegistrationForDeploymentTransition();
      }
      await deploymentNotifier.update(config);
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      await ref.read(mobileBackendLifecycleProvider).reconcile(config);
      if (_deployMode == 0) {
        await ref.read(backendConnectionProvider.future);
        await ref.read(backendTransportProvider.future);
      }
      final onboarding = ref.read(onboardingServiceProvider);
      final health = _deployMode == 0
          ? await onboarding.health()
          : await onboarding.healthAt(_remoteCoreCtrl.text.trim());
      if (health.isEmpty) {
        throw StateError(_deployMode == 0 ? '本机统筹中心不可用' : '远程统筹中心不可用');
      }
      if (mounted) {
        setState(() {
          _devicePaired = _deployMode == 0;
          _pairingRequired = false;
          _pairingError = '';
          _pairingCodeCtrl.clear();
          _firstDeviceSetupRequired = false;
          _modelIntent = null;
          _skipCoreModels = false;
          _textConfigId = null;
          _connectionFailed = false;
          _failureDetail = '';
          _checkRows = const [];
        });
      }
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      await deploymentNotifier.update(previousConfig);
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      try {
        await ref
            .read(mobileBackendLifecycleProvider)
            .reconcile(previousConfig);
      } catch (error, stackTrace) {
        print('[original-error] $error\n$stackTrace');
      }
      rethrow;
    } finally {
      if (deploymentChanged) {
        notificationCoordinator.resumeRegistrationAfterDeploymentTransition();
      }
    }
  }

  Future<bool> _runtimeProfileReady(
    OnboardingService onboarding,
    String coreUri,
  ) async {
    final capabilities = await onboarding.runtimeCapabilitiesAt(coreUri);
    final profile = (capabilities['runtimeProfile'] ?? '').toString();
    final capabilityMap = capabilities['capabilities'] is Map
        ? Map<String, dynamic>.from(capabilities['capabilities'] as Map)
        : const <String, dynamic>{};
    return switch (profile) {
      'local' => capabilityMap['localUIEndpoints'] == true,
      'device-agent' =>
        capabilityMap['localUIEndpoints'] == true &&
            capabilityMap['deviceExecutionPlane'] == true,
      _ => false,
    };
  }

  Future<void> _runCheckAt(
    int index,
    OnboardingService onboarding,
    String coreUri,
  ) async {
    if (_deployMode == 0) {
      switch (index) {
        case 0:
          final live = await onboarding.livenessAt(coreUri);
          if (!live) throw StateError('/livez 未返回 HTTP 200');
          return;
        case 1:
          final ready = await onboarding.readinessAt(coreUri);
          if (!ready) throw StateError('/readyz 未返回 HTTP 200');
          return;
        default:
          final profileReady = await _runtimeProfileReady(onboarding, coreUri);
          if (!profileReady) {
            throw StateError('本机运行时能力不足，无法提供统筹服务');
          }
          return;
      }
    }
    switch (index) {
      case 0:
        final health = await onboarding.healthAt(coreUri);
        if (health.isEmpty) throw StateError('远程统筹中心健康检查未返回数据');
        return;
      case 1:
        final info = await onboarding.coreInfoAt(coreUri);
        final spaceId = (info['spaceId'] ?? '').toString().trim();
        final instanceId = (info['instanceId'] ?? '').toString().trim();
        if (spaceId.isEmpty || instanceId.isEmpty) {
          throw StateError('统筹数据身份不完整（spaceId 或 instanceId 为空）');
        }
        return;
      case 2:
        final pairing = await onboarding.pairingStatusAt(coreUri);
        if (!pairing.containsKey('firstDeviceSetupRequired')) {
          throw StateError('配对服务未返回首设备状态');
        }
        return;
      default:
        final profileReady = await _runtimeProfileReady(onboarding, coreUri);
        if (!profileReady) {
          throw StateError('远程运行时能力与当前版本不兼容');
        }
        return;
    }
  }

  Future<String> _localRuntimeUri() async {
    final endpoint = await ref
        .read(embeddedRuntimeControllerProvider)
        .getEndpoint();
    return endpoint.httpBaseUri.toString();
  }

  Future<void> _runEnvChecks() async {
    if (_envChecking) return;
    setState(() {
      _envChecking = true;
      _connectionFailed = false;
      _failureDetail = '';
      _checkRows = _checkNames().map(OnboardingCheckState.waiting).toList();
    });
    final onboarding = ref.read(onboardingServiceProvider);
    var failed = false;
    try {
      final coreUri = _deployMode == 0
          ? await _localRuntimeUri()
          : _remoteCoreCtrl.text.trim();
      for (var index = 0; index < _checkRows.length; index++) {
        if (!mounted) return;
        setState(() {
          _checkRows[index] = _checkRows[index].copyWith(
            state: OnboardingCheckPhase.running,
            stateText: '处理中',
          );
        });
        try {
          await _runCheckAt(index, onboarding, coreUri);
        } catch (error, stackTrace) {
          print('[original-error] $error\n$stackTrace');
          failed = true;
          if (!mounted) return;
          setState(() {
            _checkRows[index] = _checkRows[index].copyWith(
              state: OnboardingCheckPhase.failed,
              stateText: '失败',
            );
            _connectionFailed = true;
            _failureDetail = originalErrorText(error);
          });
          break;
        }
        if (!mounted) return;
        setState(() {
          _checkRows[index] = _checkRows[index].copyWith(
            state: OnboardingCheckPhase.done,
            stateText: '已完成',
          );
        });
      }
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      failed = true;
      if (mounted) {
        setState(() {
          _connectionFailed = true;
          _failureDetail = originalErrorText(error);
        });
      }
    } finally {
      if (mounted) setState(() => _envChecking = false);
    }
    if (failed || !mounted) return;
    if (_deployMode == 0) {
      setState(() => _devicePaired = true);
      return;
    }
    await _refreshPairingState();
  }

  String _originOf(String value) {
    final uri = Uri.tryParse(value.trim());
    if (uri == null || !uri.hasScheme) {
      return value.trim().replaceAll(RegExp(r'/+$'), '');
    }
    return '${uri.scheme}://${uri.host}:${uri.port}';
  }

  Future<void> _refreshPairingState() async {
    final remoteCore = _remoteCoreCtrl.text.trim();
    try {
      final pairing = await ref
          .read(onboardingServiceProvider)
          .pairingStatusAt(remoteCore);
      final status = await ref.read(deviceMeshLocalServiceProvider)?.status();
      final state = (status?['state'] ?? '').toString();
      final cloudBaseUrl = (status?['cloudBaseUrl'] ?? '').toString();
      final paired =
          state.isNotEmpty &&
          state != 'unprovisioned' &&
          cloudBaseUrl.isNotEmpty &&
          _originOf(cloudBaseUrl) == _originOf(remoteCore);
      if (!mounted) return;
      setState(() {
        _firstDeviceSetupRequired = pairing['firstDeviceSetupRequired'] == true;
        _devicePaired = paired;
        _pairingRequired = !paired;
        _pairingError = '';
      });
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() {
        _devicePaired = false;
        _pairingRequired = true;
        _pairingError = originalErrorText(error);
      });
    }
  }

  String _extractPairingOffer(String raw, String cloudUri) {
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

  Future<void> _pairDevice() async {
    if (_pairingBusy) return;
    final rawPairing = _pairingCodeCtrl.text.trim();
    if (rawPairing.isEmpty) {
      setState(() {
        _pairingError = _firstDeviceSetupRequired
            ? '请输入首设备设置码'
            : '请粘贴设备配对二维码内容或 Offer Token';
      });
      return;
    }
    setState(() {
      _pairingBusy = true;
      _pairingError = '';
    });
    try {
      final remoteCore = _remoteCoreCtrl.text.trim();
      final localMesh = ref.read(deviceMeshLocalServiceProvider);
      if (localMesh == null) {
        throw StateError('本机 Device Agent 不可用，无法完成统筹中心设备配对');
      }
      final identity = await localMesh.identity();
      final deviceId = (identity['deviceId'] ?? '').toString().trim();
      final runtimeId = (identity['runtimeId'] ?? '').toString().trim();
      final platform = (identity['platform'] ?? '').toString().trim();
      if (deviceId.isEmpty || runtimeId.isEmpty || platform.isEmpty) {
        throw StateError('本机 Device Mesh 身份不完整');
      }
      var offerToken = '';
      var setupCode = '';
      if (_firstDeviceSetupRequired) {
        setupCode = rawPairing;
      } else {
        offerToken = _extractPairingOffer(rawPairing, remoteCore);
      }
      final claimed = await ref
          .read(onboardingServiceProvider)
          .claimPairingAt(
            remoteCore,
            deviceId: deviceId,
            runtimeId: runtimeId,
            platform: platform,
            label: 'Mobile',
            offerToken: offerToken,
            setupCode: setupCode,
          );
      final ticket = (claimed['ticket'] ?? '').toString().trim();
      if (ticket.isEmpty) {
        throw StateError('统筹中心未返回 Bootstrap Ticket');
      }
      await localMesh.bootstrap(
        cloudBaseUrl: remoteCore,
        bootstrapTicket: ticket,
      );
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      await ref.read(backendConnectionProvider.future);
      await ref.read(backendTransportProvider.future);
      if (!mounted) return;
      setState(() {
        _devicePaired = true;
        _pairingRequired = false;
        _pairingCodeCtrl.clear();
      });
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() => _pairingError = originalErrorText(error));
    } finally {
      if (mounted) setState(() => _pairingBusy = false);
    }
  }

  Future<void> _goToRuntimeStep() async {
    if (_submitting) return;
    if (_deployMode == 1 && _remoteCoreCtrl.text.trim().isEmpty) {
      setState(() => _stepError = '使用远程统筹中心必须填写中心地址');
      return;
    }
    setState(() {
      _submitting = true;
      _stepError = '';
    });
    try {
      await _applyDeploymentSelection();
      if (!mounted) return;
      setState(() => _currentStep = 1);
      await _runEnvChecks();
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() => _stepError = originalErrorText(error));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Future<void> _goToModelStep() async {
    if (_submitting) return;
    if (!_envAllDone) {
      setState(() => _stepError = '请先完成环境检查');
      return;
    }
    if (_deployMode == 1 && !_devicePaired) {
      setState(() => _stepError = '请先完成设备配对');
      return;
    }
    setState(() {
      _submitting = true;
      _stepError = '';
    });
    try {
      final intent = await coreConfigurationGuardFor(ref).capture();
      if (!mounted) return;
      setState(() {
        _modelIntent = intent;
        _skipCoreModels = !intent.canConfigure;
        _currentStep = 2;
      });
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() => _stepError = originalErrorText(error));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  void _prevStep() {
    if (_submitting) return;
    setState(() {
      _stepError = '';
      _currentStep = _currentStep == 0 ? 0 : _currentStep - 1;
      if (_currentStep == 0) {
        _checkRows = const [];
        _connectionFailed = false;
        _failureDetail = '';
        _pairingRequired = false;
        _pairingError = '';
      }
    });
  }

  String _apiTypeFor(String provider) {
    final value = provider.toLowerCase();
    if (value.contains('anthropic')) return 'anthropic';
    return 'openai-compatible';
  }

  Future<void> _detectModel() async {
    if (_detecting) return;
    final baseUrl = _baseUrlCtrl.text.trim();
    final key = _keyCtrl.text.trim();
    setState(() {
      _detecting = true;
      _modelStatusOk = false;
      _modelStatus = '正在检测模型连接';
      _detectedModels = const [];
    });
    try {
      final detected = await ref
          .read(onboardingServiceProvider)
          .detectModels(
            baseUrl: baseUrl,
            apiKey: key,
            apiType: _apiTypeFor(_providerCtrl.text.trim()),
          );
      final models = detected
          .map((item) => (item['id'] ?? '').toString().trim())
          .where((value) => value.isNotEmpty)
          .toList();
      if (!mounted) return;
      setState(() {
        _detectedModels = models;
        _modelStatusOk = models.isNotEmpty;
        _modelStatus = models.isEmpty
            ? '未检测到可用模型'
            : '已检测到 ${models.length} 个模型';
        if (models.length == 1) {
          _modelCtrl.text = models.first;
        }
      });
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() {
        _detectedModels = const [];
        _modelStatusOk = false;
        _modelStatus = '检测失败\n原始错误：\n${originalErrorText(error)}';
      });
    } finally {
      if (mounted) setState(() => _detecting = false);
    }
  }

  Future<void> _persistTextModel() async {
    final baseUrl = _baseUrlCtrl.text.trim();
    final key = _keyCtrl.text.trim();
    final model = _modelCtrl.text.trim();
    if (baseUrl.isEmpty) throw StateError('请填写服务地址');
    if (key.isEmpty) throw StateError('请填写 API Key');
    if (model.isEmpty) throw StateError('请填写模型名称');
    final detected = await ref
        .read(onboardingServiceProvider)
        .detectModels(
          baseUrl: baseUrl,
          apiKey: key,
          apiType: _apiTypeFor(_providerCtrl.text.trim()),
        );
    if (detected.isEmpty) {
      throw StateError('文本模型连接检测失败');
    }
    final payload = <String, dynamic>{
      'name': '默认文本模型',
      'supportsVision': false,
      'apiType': _apiTypeFor(_providerCtrl.text.trim()),
      'baseUrl': baseUrl,
      'apiKey': key,
      'modelName': model,
      'isActive': 1,
    };
    final service = ref.read(modelConfigServiceProvider);
    if (_textConfigId == null) {
      final created = await service.create(payload);
      _textConfigId = created?.id;
    } else {
      await service.update(_textConfigId!, payload);
    }
  }

  Future<void> _completeOnboarding() async {
    await ref
        .read(onboardingServiceProvider)
        .complete(deployMode: _deployMode == 0 ? 'mobile-local' : 'cloud-web');
    if (!mounted) return;
    context.go(AppRoutes.chat);
  }

  Future<void> _finish() async {
    if (_submitting) return;
    setState(() {
      _submitting = true;
      _stepError = '';
    });
    try {
      if (!_skipCoreModels) {
        final intent = _modelIntent;
        if (intent == null) {
          throw StateError('模型配置归属无法确认，请返回上一步重新进入');
        }
        await coreConfigurationGuardFor(ref).validate(intent);
        if (CoreConfigurationIntent.current == null) {
          await intent.run(() async {
            await _persistTextModel();
            await _completeOnboarding();
          });
        } else {
          await _persistTextModel();
          await _completeOnboarding();
        }
      } else {
        await _completeOnboarding();
      }
    } catch (error, stackTrace) {
      print('[original-error] $error\n$stackTrace');
      if (!mounted) return;
      setState(() => _stepError = originalErrorText(error));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  String get _runtimeTitle {
    if (_connectionFailed) {
      return _deployMode == 0 ? '启动本机统筹中心失败' : '连接远程统筹中心失败';
    }
    if (_envChecking) {
      return _deployMode == 0 ? '正在启动本机统筹中心' : '正在连接远程统筹中心';
    }
    if (_envAllDone) return '统筹中心已就绪';
    return _deployMode == 0 ? '启动本机统筹中心' : '连接远程统筹中心';
  }

  String get _runtimeSubtitle {
    return _deployMode == 0
        ? '系统将检查本机统筹服务和运行时能力。'
        : '远程统筹中心使用设备凭证认证。检查完成后，当前设备必须先完成配对再继续。';
  }

  @override
  Widget build(BuildContext context) {
    if (!_consentLoaded) {
      return const SizedBox.shrink();
    }
    if (!_consentAccepted) {
      return OnboardingConsentGate(
        agreed: _agreed,
        onToggleAgreed: () => setState(() => _agreed = !_agreed),
        onContinue: _acceptConsent,
      );
    }
    return OnboardingStepScaffold(
      stepIndex: _currentStep,
      stepCount: _stepCount,
      stepName: _stepNames[_currentStep],
      title: switch (_currentStep) {
        0 => '连接统筹中心',
        1 => _runtimeTitle,
        _ => '连接语言模型',
      },
      subtitle: switch (_currentStep) {
        0 => '统筹中心保存设备、会话与模型配置。可以选择本机运行，也可以连接另一台设备提供的统筹中心。',
        1 => _runtimeSubtitle,
        _ => '语言模型负责理解对话、组织回复和完成推理。请选择服务并完成连接检测。',
      },
      nextLabel: _currentStep == _stepCount - 1
          ? (_submitting ? '正在保存' : '开始使用')
          : (_submitting ? '正在保存' : '下一步'),
      onNext: _submitting
          ? null
          : switch (_currentStep) {
              0 => _goToRuntimeStep,
              1 => _goToModelStep,
              _ => _finish,
            },
      onBack: _currentStep == 0 ? null : _prevStep,
      errorText: _stepError.isEmpty ? null : _stepError,
      child: switch (_currentStep) {
        0 => OnboardingCenterStep(
          deployMode: _deployMode,
          remoteCoreController: _remoteCoreCtrl,
          onModeChanged: (value) => setState(() {
            _deployMode = value;
            _remoteStatus = '';
            _stepError = '';
          }),
          onRemoteCoreChanged: (value) => setState(() {
            _remoteStatus = '';
            _stepError = '';
          }),
          detecting: _remoteChecking,
          statusText: _remoteStatus,
          onCheckRemote: _checkRemoteCore,
        ),
        1 => OnboardingRuntimeStep(
          checks: _checkRows,
          connectionFailed: _connectionFailed,
          failureDetail: _failureDetail,
          onRetry: _envChecking ? () {} : _runEnvChecks,
          pairingRequired: _pairingRequired,
          firstDeviceSetupRequired: _firstDeviceSetupRequired,
          pairingController: _pairingCodeCtrl,
          pairingBusy: _pairingBusy,
          pairingError: _pairingError,
          onPair: _pairDevice,
          pairingVerified: _deployMode == 1 && _devicePaired,
        ),
        _ => _skipCoreModels
            ? const _SkippedModelNotice()
            : OnboardingModelStep(
                providerController: _providerCtrl,
                baseUrlController: _baseUrlCtrl,
                modelController: _modelCtrl,
                keyController: _keyCtrl,
                detecting: _detecting,
                statusText: _modelStatus,
                statusOk: _modelStatusOk,
                detectedModels: _detectedModels,
                onSelectModel: (value) => setState(() {
                  _modelCtrl.text = value;
                }),
                onDetect: _detectModel,
                onFieldChanged: () => setState(() {
                  _modelStatus = '';
                  _modelStatusOk = false;
                  _detectedModels = const [];
                }),
                showKey: _showKey,
                onToggleKey: () => setState(() => _showKey = !_showKey),
              ),
      },
    );
  }
}

class _SkippedModelNotice extends StatelessWidget {
  const _SkippedModelNotice();

  @override
  Widget build(BuildContext context) {
    return Text(
      'AI 服务由统筹中心提供，本机无需配置模型。',
      style: AppTypography.bodySmall(context),
    );
  }
}
