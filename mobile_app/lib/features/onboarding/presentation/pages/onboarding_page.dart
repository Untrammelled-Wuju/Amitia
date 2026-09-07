import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class OnboardingPage extends ConsumerStatefulWidget {
  const OnboardingPage({super.key});

  @override
  ConsumerState<OnboardingPage> createState() => _OnboardingPageState();
}

class _OnboardingPageState extends ConsumerState<OnboardingPage> {
  int _currentStep = 0;

  static const _steps = [
    '欢迎',
    '运行环境检查',
    '部署模式选择',
    '管理员初始化',
    '使用边界确认',
    '文本模型配置',
    '视觉模型配置',
    '语音模型配置',
    '向量模型配置',
    '角色头像',
    '角色名字',
    '角色身份',
    '角色性格',
    '初始记忆',
    '设置汇总',
    '进入Amitia',
  ];

  final _adminUserController = TextEditingController();
  final _adminPassController = TextEditingController();
  final _setupTokenController = TextEditingController();
  final _textProviderCtrl = TextEditingController(text: 'OpenAI');
  final _textModelCtrl = TextEditingController(text: 'GPT-4o');
  final _textKeyCtrl = TextEditingController();
  final _visionProviderCtrl = TextEditingController(text: 'OpenAI');
  final _visionModelCtrl = TextEditingController(text: 'GPT-4o-mini');
  final _visionKeyCtrl = TextEditingController();
  final _voiceProviderCtrl = TextEditingController(text: 'Volcengine');
  final _voiceModelCtrl = TextEditingController(text: 'seed-tts-2.0');
  final _voiceKeyCtrl = TextEditingController();
  final _vectorProviderCtrl = TextEditingController(text: '火山方舟');
  final _vectorModelCtrl = TextEditingController(text: 'Doubao Embedding');
  final _vectorKeyCtrl = TextEditingController();
  final _remoteCoreCtrl = TextEditingController();
  final _charNameCtrl = TextEditingController();
  final _charIdentityCtrl = TextEditingController();
  final _initMemoryCtrl = TextEditingController();

  int _deployMode = 0;
  bool _envChecked = false;
  bool _envChecking = false;
  List<bool> _envResults = [];
  final List<bool> _boundaryAgreed = [false, false, false];
  int _selectedAvatarColor = 0;
  final List<bool> _selectedTraits = List.filled(8, false);
  bool _submitting = false;
  bool _adminInitialized = false;
  bool _adminExists = false;
  String? _textConfigId;
  String? _visionConfigId;
  String? _ttsConfigId;
  String? _embeddingConfigId;
  String? _createdCharacterId;

  static const _avatarColors = ['#8A5728', '#52B788', '#6C8FEA', '#E9A23B', '#E66767', '#9C91F5'];
  static const _personalityTraits = ['温柔', '理性', '活泼', '冷静', '幽默', '严谨', '热情', '内敛'];

  @override
  void dispose() {
    _adminUserController.dispose();
    _adminPassController.dispose();
    _setupTokenController.dispose();
    _textProviderCtrl.dispose();
    _textModelCtrl.dispose();
    _textKeyCtrl.dispose();
    _visionProviderCtrl.dispose();
    _visionModelCtrl.dispose();
    _visionKeyCtrl.dispose();
    _voiceProviderCtrl.dispose();
    _voiceModelCtrl.dispose();
    _voiceKeyCtrl.dispose();
    _vectorProviderCtrl.dispose();
    _vectorModelCtrl.dispose();
    _vectorKeyCtrl.dispose();
    _remoteCoreCtrl.dispose();
    _charNameCtrl.dispose();
    _charIdentityCtrl.dispose();
    _initMemoryCtrl.dispose();
    super.dispose();
  }

  Future<void> _next() async {
    if (_submitting) return;
    setState(() => _submitting = true);
    try {
      await _persistCurrentStep();
      if (!mounted) return;
      if (_currentStep < _steps.length - 1) {
        setState(() => _currentStep++);
      } else {
        await _completeOnboarding();
      }
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, '该步骤未完成：$error');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  void _prev() {
    if (_currentStep > 0 && !_submitting) {
      setState(() => _currentStep--);
    }
  }

  Future<void> _runEnvCheck() async {
    if (_envChecking) return;
    setState(() {
      _envChecking = true;
      _envChecked = false;
      _envResults = const [];
    });
    try {
      final onboarding = ref.read(onboardingServiceProvider);
      const localRuntimeUri = 'http://127.0.0.1:18899';
      final results = await Future.wait<dynamic>(<Future<dynamic>>[
        onboarding.livenessAt(localRuntimeUri),
        onboarding.readinessAt(localRuntimeUri),
        onboarding.runtimeCapabilitiesAt(localRuntimeUri),
      ]);
      final live = results[0] == true;
      final ready = results[1] == true;
      final capabilities = results[2] is Map
          ? Map<String, dynamic>.from(results[2] as Map)
          : const <String, dynamic>{};
      if (!mounted) return;
      final profile = (capabilities['runtimeProfile'] ?? '').toString();
      final capabilityMap = capabilities['capabilities'] is Map
          ? Map<String, dynamic>.from(capabilities['capabilities'] as Map)
          : const <String, dynamic>{};
      final profileReady = switch (profile) {
        'local' => capabilityMap['localUIEndpoints'] == true,
        'device-agent' =>
          capabilityMap['localUIEndpoints'] == true &&
              capabilityMap['deviceExecutionPlane'] == true,
        _ => false,
      };
      final checked = [live, ready, profileReady];
      setState(() {
        _envResults = checked;
        _envChecked = checked.every((value) => value);
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _envResults = [false, false, false];
        _envChecked = false;
      });
    } finally {
      if (mounted) {
        setState(() => _envChecking = false);
      }
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

    final deploymentNotifier = ref.read(mobileDeploymentConfigProvider.notifier);
    final previousConfig = ref.read(mobileDeploymentConfigProvider);
    try {
      await deploymentNotifier.update(config);
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      await ref.read(mobileBackendLifecycleProvider).reconcile(config);
      if (_deployMode == 0) {
        await ref.read(backendConnectionProvider.future);
        await ref.read(backendTransportProvider.future);
      }

      final onboarding = ref.read(onboardingServiceProvider);
      final auth = ref.read(authServiceProvider);
      final health = _deployMode == 0
          ? await onboarding.health()
          : await onboarding.healthAt(_remoteCoreCtrl.text.trim());
      if (health.isEmpty) {
        throw StateError(_deployMode == 0 ? '本地 Business Core 不可用' : 'Cloud Core 不可用');
      }
      final adminExists = _deployMode == 0
          ? await auth.hasAdmin()
          : await auth.hasAdminAt(_remoteCoreCtrl.text.trim());
      if (mounted) {
        setState(() {
          _adminExists = adminExists;
          _adminInitialized = false;
          _setupTokenController.clear();
        });
      }
    } catch (_) {
      await deploymentNotifier.update(previousConfig);
      ref.invalidate(backendConnectionProvider);
      ref.invalidate(backendTransportProvider);
      try {
        await ref.read(mobileBackendLifecycleProvider).reconcile(previousConfig);
      } catch (_) {}
      rethrow;
    }
  }

  Future<void> _persistCurrentStep() async {
    switch (_currentStep) {
      case 2:
        if (_deployMode == 1 && _remoteCoreCtrl.text.trim().isEmpty) {
          throw StateError('云端模式必须填写 Cloud Core 地址');
        }
        await _applyDeploymentSelection();
        return;
      case 3:
        if (_adminInitialized) return;
        final auth = ref.read(authServiceProvider);
        final username = _adminUserController.text.trim();
        final password = _adminPassController.text;
        final isCloud = _deployMode == 1;
        final remoteCore = _remoteCoreCtrl.text.trim();
        final adminExists = isCloud
            ? await auth.hasAdminAt(remoteCore)
            : await auth.hasAdmin();
        if (adminExists) {
          if (isCloud) {
            await auth.loginAt(remoteCore, username, password);
          } else {
            await auth.login(username, password);
          }
        } else {
          final setupToken = _setupTokenController.text.trim();
          if (isCloud) {
            await auth.setupAndLoginAt(
              remoteCore,
              username,
              password,
              setupToken: setupToken,
            );
          } else {
            await auth.setupAndLogin(username, password);
          }
        }
        if (isCloud) {
          ref.invalidate(backendConnectionProvider);
          ref.invalidate(backendTransportProvider);
          await ref.read(backendConnectionProvider.future);
          await ref.read(backendTransportProvider.future);
          await auth.fetchProfile();
        }
        _adminExists = true;
        _adminInitialized = true;
        ref.invalidate(currentUserProvider);
        return;
      case 5:
        await _persistTextModel();
        return;
      case 6:
        await _persistVisionModel();
        return;
      case 7:
        await _persistVoiceModel();
        return;
      case 8:
        await _persistEmbeddingModel();
        return;
      default:
        return;
    }
  }

  Future<void> _persistTextModel() async {
    final provider = _textProviderCtrl.text.trim();
    final model = _textModelCtrl.text.trim();
    final key = _textKeyCtrl.text.trim();
    final baseUrl = _baseUrlFor(provider, 'text');
    final detected = await ref.read(onboardingServiceProvider).detectModels(
          baseUrl: baseUrl,
          apiKey: key,
          apiType: _apiTypeFor(provider),
        );
    if (detected.isEmpty) {
      throw StateError('文本模型连接检测失败');
    }
    final payload = <String, dynamic>{
      'name': '默认文本模型',
      'apiType': _apiTypeFor(provider),
      'baseUrl': baseUrl,
      'apiKey': key,
      'modelName': model,
      'isActive': 1,
    };
    final svc = ref.read(modelConfigServiceProvider);
    if (_textConfigId == null) {
      final created = await svc.create(payload);
      _textConfigId = created?.id;
    } else {
      await svc.update(_textConfigId!, payload);
    }
  }

  Future<void> _persistVisionModel() async {
    final provider = _visionProviderCtrl.text.trim();
    final model = _visionModelCtrl.text.trim();
    final key = _visionKeyCtrl.text.trim();
    final baseUrl = _baseUrlFor(provider, 'vision');
    final detected = await ref.read(onboardingServiceProvider).detectModels(
          baseUrl: baseUrl,
          apiKey: key,
          apiType: _apiTypeFor(provider),
        );
    if (detected.isEmpty) throw StateError('视觉模型连接检测失败');
    final payload = <String, dynamic>{
      'name': '默认视觉模型',
      'apiType': _apiTypeFor(provider),
      'baseUrl': baseUrl,
      'apiKey': key,
      'modelName': model,
      'isActive': 1,
    };
    final svc = ref.read(visionServiceProvider);
    if (_visionConfigId == null) {
      final created = await svc.createConfig(payload);
      _visionConfigId = created?['id']?.toString();
    } else {
      await svc.updateConfig(_visionConfigId!, payload);
    }
  }

  Future<void> _persistVoiceModel() async {
    final provider = _voiceProviderCtrl.text.trim();
    final resource = _voiceModelCtrl.text.trim();
    final key = _voiceKeyCtrl.text.trim();
    final payload = <String, dynamic>{
      'name': '默认语音模型',
      'apiType': _apiTypeFor(provider, tts: true),
      'baseUrl': _baseUrlFor(provider, 'tts'),
      'apiKey': key,
      'resourceId': resource,
      'voiceType': 'zh_female_vv_uranus_bigtts',
      'speed': 1.0,
      'pitch': 1.0,
      'volume': 1.0,
      'isActive': 1,
    };
    final svc = ref.read(ttsServiceProvider);
    await svc.testConnection(payload);
    if (_ttsConfigId == null) {
      final created = await svc.createConfig(payload);
      _ttsConfigId = created?.id.toString();
    } else {
      await svc.updateConfig(_ttsConfigId!, payload);
    }
  }

  Future<void> _persistEmbeddingModel() async {
    final provider = _vectorProviderCtrl.text.trim();
    final model = _vectorModelCtrl.text.trim();
    final key = _vectorKeyCtrl.text.trim();
    final baseUrl = _baseUrlFor(provider, 'embedding');
    final detected = await ref.read(onboardingServiceProvider).detectModels(
          baseUrl: baseUrl,
          apiKey: key,
          apiType: _apiTypeFor(provider),
        );
    if (detected.isEmpty) throw StateError('向量模型连接检测失败');
    final payload = <String, dynamic>{
      'name': '默认向量模型',
      'apiType': _apiTypeFor(provider),
      'baseUrl': baseUrl,
      'apiKey': key,
      'modelName': model,
      'isActive': 1,
    };
    final svc = ref.read(embeddingServiceProvider);
    if (_embeddingConfigId == null) {
      final created = await svc.createConfig(payload);
      _embeddingConfigId = created?['id']?.toString();
    } else {
      await svc.updateConfig(_embeddingConfigId!, payload);
    }
  }

  Future<void> _completeOnboarding() async {
    final traits = _personalityTraits
        .asMap()
        .entries
        .where((entry) => _selectedTraits[entry.key])
        .map((entry) => entry.value)
        .join('、');
    var characterId = _createdCharacterId;
    if (characterId == null || characterId.isEmpty) {
      final character = await ref.read(characterServiceProvider).create({
        'name': _charNameCtrl.text.trim(),
        'identity': _charIdentityCtrl.text.trim(),
        'personality': traits,
        'description': _charIdentityCtrl.text.trim(),
        'isDefault': true,
      });
      if (character == null || character.id.isEmpty) {
        throw StateError('角色创建失败');
      }
      characterId = character.id;
      _createdCharacterId = characterId;
    }
    await ref.read(characterServiceProvider).setActive(characterId);
    final memory = _initMemoryCtrl.text.trim();
    if (memory.isNotEmpty) {
      final user = await ref.read(authServiceProvider).currentUser;
      await ref.read(profileServiceProvider).create({
        'category': 'memory',
        'attributeName': '初始记忆',
        'attributeValue': memory,
        if (user?.id.isNotEmpty == true) 'userId': user!.id,
        'characterId': characterId,
        'confidence': 1.0,
        'source': 'onboarding',
      });
    }
    await ref.read(onboardingServiceProvider).complete(
          deployMode: _deployMode == 0 ? 'mobile-local' : 'cloud-web',
          username: _adminUserController.text.trim(),
        );
    ref.invalidate(characterListProvider);
    ref.read(currentCharacterIdProvider.notifier).state = characterId;
    if (!mounted) return;
    context.go(AppRoutes.chat);
  }

  String _apiTypeFor(String provider, {bool tts = false}) {
    final p = provider.toLowerCase();
    if (tts && (p.contains('volc') || p.contains('火山'))) return 'volcengine';
    if (p.contains('anthropic')) return 'anthropic';
    return 'openai-compatible';
  }

  String _baseUrlFor(String provider, String kind) {
    final p = provider.toLowerCase();
    if (kind == 'tts') {
      if (p.contains('volc') || p.contains('火山')) {
        return 'https://openspeech.bytedance.com/api/v1';
      }
      return 'https://openspeech.bytedance.com/api/v1';
    }
    if (p.contains('deepseek')) return 'https://api.deepseek.com/v1';
    if (p.contains('volc') || p.contains('火山')) {
      return 'https://ark.cn-beijing.volces.com/api/v3';
    }
    if (p.contains('anthropic')) return 'https://api.anthropic.com';
    return 'https://api.openai.com/v1';
  }

  void _toggleTrait(int index) {
    setState(() {
      _selectedTraits[index] = !_selectedTraits[index];
    });
  }

  bool get _canProceed {
    switch (_currentStep) {
      case 1:
        return _envChecked;
      case 2:
        return _deployMode == 0 || _remoteCoreCtrl.text.trim().isNotEmpty;
      case 3:
        final credentialsReady =
            _adminUserController.text.isNotEmpty && _adminPassController.text.isNotEmpty;
        if (!credentialsReady) return false;
        if (_deployMode == 1 && !_adminExists) {
          return _setupTokenController.text.trim().length >= 32;
        }
        return true;
      case 4:
        return _boundaryAgreed.every((v) => v);
      case 5:
        return _textKeyCtrl.text.isNotEmpty;
      case 6:
        return _visionKeyCtrl.text.isNotEmpty;
      case 7:
        return _voiceKeyCtrl.text.isNotEmpty;
      case 8:
        return _vectorKeyCtrl.text.isNotEmpty;
      case 10:
        return _charNameCtrl.text.isNotEmpty;
      case 11:
        return _charIdentityCtrl.text.isNotEmpty;
      case 12:
        return _selectedTraits.any((v) => v);
      default:
        return true;
    }
  }

  Color _parseColor(String hex) {
    final cleaned = hex.replaceAll('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '初始化向导',
        leading: _currentStep > 0
            ? IconButton(
                icon: const Icon(Icons.arrow_back_ios_new, size: 20),
                onPressed: _prev,
              )
            : null,
      ),
      body: Column(
        children: [
          _buildProgress(),
          Expanded(
            child: SingleChildScrollView(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: _buildStepContent(),
            ),
          ),
          _buildBottomBar(),
        ],
      ),
    );
  }

  Widget _buildProgress() {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: Row(
        children: [
          Text(
            '步骤 ${_currentStep + 1}/${_steps.length}',
            style: AppTypography.label(context),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: ClipRRect(
              borderRadius: BorderRadius.circular(3),
              child: LinearProgressIndicator(
                value: (_currentStep + 1) / _steps.length,
                minHeight: 6,
                backgroundColor: context.accentSoft,
                color: context.accentPrimary,
              ),
            ),
          ),
          SizedBox(width: AppSpacing.md),
          Text(
            _steps[_currentStep],
            style: AppTypography.caption(context).copyWith(color: context.accentPrimary),
          ),
        ],
      ),
    );
  }

  Widget _buildBottomBar() {
    final isLast = _currentStep == _steps.length - 1;
    return Container(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(top: BorderSide(color: context.borderSecondary, width: 1)),
      ),
      child: SafeArea(
        top: false,
        child: Row(
          children: [
            if (_currentStep > 0)
              Expanded(
                child: AmitiaButton(
                  label: '上一步',
                  isSecondary: true,
                  onPressed: _submitting ? null : _prev,
                ),
              )
            else
              const Spacer(),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: AmitiaButton(
                label: _submitting ? '正在保存…' : (isLast ? '进入 Amitia' : '下一步'),
                icon: isLast ? Icons.rocket_launch : Icons.arrow_forward,
                onPressed: _canProceed && !_submitting ? _next : null,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStepContent() {
    switch (_currentStep) {
      case 0:
        return _buildWelcome();
      case 1:
        return _buildEnvCheck();
      case 2:
        return _buildDeployMode();
      case 3:
        return _buildAdminInit();
      case 4:
        return _buildBoundary();
      case 5:
        return _buildModelConfig(
          '文本模型',
          '用于对话生成和文本理解',
          _textProviderCtrl,
          _textModelCtrl,
          _textKeyCtrl,
          Icons.text_fields,
        );
      case 6:
        return _buildModelConfig(
          '视觉模型',
          '用于图片识别和理解',
          _visionProviderCtrl,
          _visionModelCtrl,
          _visionKeyCtrl,
          Icons.image_outlined,
        );
      case 7:
        return _buildModelConfig(
          '语音模型',
          '用于语音合成和识别',
          _voiceProviderCtrl,
          _voiceModelCtrl,
          _voiceKeyCtrl,
          Icons.record_voice_over_outlined,
        );
      case 8:
        return _buildModelConfig(
          '向量模型',
          '用于记忆向量化和检索',
          _vectorProviderCtrl,
          _vectorModelCtrl,
          _vectorKeyCtrl,
          Icons.memory,
        );
      case 9:
        return _buildAvatar();
      case 10:
        return _buildCharName();
      case 11:
        return _buildCharIdentity();
      case 12:
        return _buildPersonality();
      case 13:
        return _buildInitMemory();
      case 14:
        return _buildSummary();
      case 15:
        return _buildFinish();
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildWelcome() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(height: AppSpacing.xxl),
        Center(
          child: Container(
            width: 100,
            height: 100,
            decoration: BoxDecoration(
              color: context.accentSoft,
              shape: BoxShape.circle,
            ),
            child: Icon(Icons.auto_awesome, size: 50, color: context.accentPrimary),
          ),
        ),
        SizedBox(height: AppSpacing.xl),
        Center(
          child: Text('欢迎使用 Amitia', style: AppTypography.pageLargeTitle(context)),
        ),
        SizedBox(height: AppSpacing.md),
        Center(
          child: Text(
            '你的专属 AI 伙伴平台',
            style: AppTypography.body(context).copyWith(color: context.textSecondary),
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('接下来我们将引导你完成以下配置：', style: AppTypography.body(context)),
              SizedBox(height: AppSpacing.md),
              ...[
                '运行环境与部署模式',
                '管理员账号初始化',
                '文本 / 视觉 / 语音 / 向量模型',
                'AI 角色头像、名字与性格',
                '初始记忆设定',
              ].map((item) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: Row(
                  children: [
                    Icon(Icons.check_circle_outline, size: 18, color: context.accentPrimary),
                    SizedBox(width: AppSpacing.sm),
                    Text(item, style: AppTypography.bodySmall(context)),
                  ],
                ),
              )),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          backgroundColor: context.accentSoft,
          child: Row(
            children: [
              Icon(Icons.info_outline, size: 20, color: context.accentPrimary),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(
                  '整个过程大约需要 5 分钟，你可以随时返回上一步修改配置。',
                  style: AppTypography.caption(context).copyWith(color: context.accentPrimary),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildEnvCheck() {
    final checks = ['本地 Runtime 进程', 'Runtime 就绪状态', 'Runtime Profile / 本地能力'];
    final results = ['进程已响应', '编排已就绪', '能力声明有效'];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('运行环境检查', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('请确认以下组件状态正常，以确保 Amitia 正常运行。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        if (_envChecking)
          AmitiaCard(
            child: Center(
              child: Column(
                children: [
                  CircularProgressIndicator(strokeWidth: 2.5, color: context.accentPrimary),
                  SizedBox(height: AppSpacing.md),
                  Text('正在检查环境...', style: AppTypography.caption(context)),
                ],
              ),
            ),
          )
        else if (_envResults.isEmpty)
          AmitiaCard(
            child: Column(
              children: [
                Icon(Icons.search, size: 40, color: context.textTertiary),
                SizedBox(height: AppSpacing.md),
                Text('点击下方按钮开始检查', style: AppTypography.caption(context)),
                SizedBox(height: AppSpacing.md),
                AmitiaButton(
                  label: '开始检查',
                  icon: Icons.play_arrow,
                  isFullWidth: true,
                  onPressed: _runEnvCheck,
                ),
              ],
            ),
          )
        else ...[
          ...List.generate(checks.length, (i) {
            final ok = _envResults[i];
            return Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                child: Row(
                  children: [
                    Container(
                      width: 40,
                      height: 40,
                      decoration: BoxDecoration(
                        color: (ok ? context.success : context.warning).withValues(alpha: 0.12),
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Icon(
                        ok ? Icons.check_circle : Icons.warning_amber,
                        size: 22,
                        color: ok ? context.success : context.warning,
                      ),
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(checks[i], style: AppTypography.body(context)),
                          Text(ok ? results[i] : '检查失败', style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                    AmitiaStatusBadge(
                      label: ok ? '正常' : '需关注',
                      type: ok ? BadgeType.success : BadgeType.warning,
                    ),
                  ],
                ),
              ),
            );
          }),
          if (_envResults.any((v) => !v))
            AmitiaCard(
              backgroundColor: context.warning.withValues(alpha: 0.08),
              child: Row(
                children: [
                  Icon(Icons.shield_outlined, size: 18, color: context.warning),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Text(
                      '本地 Runtime/Device Agent 尚未就绪。请修复后重新检查，全部通过后再继续。',
                      style: AppTypography.caption(context).copyWith(color: context.warning),
                    ),
                  ),
                ],
              ),
            ),
          if (_envResults.any((v) => !v)) ...[
            SizedBox(height: AppSpacing.sm),
            AmitiaButton(
              label: '重新检查',
              icon: Icons.refresh,
              isSecondary: true,
              isFullWidth: true,
              onPressed: _runEnvCheck,
            ),
          ],
        ],
      ],
    );
  }

  Widget _buildDeployMode() {
    final modes = [
      ('本地部署', '所有数据存储在本地设备，隐私安全，无需网络', Icons.laptop, true),
      ('云端部署', '数据存储在云端服务器，可多设备同步', Icons.cloud_outlined, false),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('选择部署模式', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('不同的部署模式影响数据存储和访问方式。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        ...List.generate(modes.length, (i) {
          final mode = modes[i];
          final isSelected = _deployMode == i;
          return Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.md),
            child: GestureDetector(
              onTap: () => setState(() => _deployMode = i),
              child: Container(
                padding: EdgeInsets.all(AppSpacing.cardPadding),
                decoration: BoxDecoration(
                  color: isSelected ? context.accentSoft : context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(
                    color: isSelected ? context.accentPrimary : context.borderPrimary,
                    width: isSelected ? 1.5 : 0.5,
                  ),
                ),
                child: Row(
                  children: [
                    Container(
                      width: 48,
                      height: 48,
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.accentSoft,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Icon(mode.$3, size: 24, color: isSelected ? Colors.white : context.accentPrimary),
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Row(
                            children: [
                              Text(mode.$1, style: AppTypography.cardTitle(context)),
                              if (mode.$4) ...[
                                SizedBox(width: AppSpacing.sm),
                                AmitiaStatusBadge(label: '推荐', type: BadgeType.accent),
                              ],
                            ],
                          ),
                          const SizedBox(height: 4),
                          Text(mode.$2, style: AppTypography.caption(context)),
                        ],
                      ),
                    ),
                    Icon(
                      isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
                      color: isSelected ? context.accentPrimary : context.textTertiary,
                      size: 24,
                    ),
                  ],
                ),
              ),
            ),
          );
        }),
        if (_deployMode == 1) ...[
          SizedBox(height: AppSpacing.lg),
          AmitiaCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('Cloud Core 地址', style: AppTypography.label(context)),
                SizedBox(height: AppSpacing.xs),
                AmitiaTextField(
                  controller: _remoteCoreCtrl,
                  hintText: 'https://core.example.com',
                  prefixIcon: Icon(Icons.link, size: 20, color: context.textTertiary),
                  onChanged: (_) => setState(() {}),
                ),
                SizedBox(height: AppSpacing.sm),
                Text(
                  '点击下一步后会立即切换 Business Core 到该 Cloud Core；设备本地 Runtime / Device Agent 仍会保留。',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
        ],
      ],
    );
  }

  Widget _buildAdminInit() {
    final creatingRemoteAdmin = _deployMode == 1 && !_adminExists;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          _adminExists ? '管理员登录' : '管理员账号初始化',
          style: AppTypography.sectionTitle(context),
        ),
        SizedBox(height: AppSpacing.sm),
        Text(
          _adminExists
              ? '目标 Business Core 已存在管理员，请登录后继续。'
              : creatingRemoteAdmin
                  ? '该 Cloud Core 尚无管理员，需要使用服务器配置的初始化令牌创建首个管理员。'
                  : '创建管理员账号用于管理 Amitia 平台。',
          style: AppTypography.caption(context),
        ),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('用户名', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: _adminExists ? '请输入管理员用户名' : '设置管理员用户名',
                controller: _adminUserController,
                prefixIcon: Icon(Icons.person_outline, size: 20, color: context.textTertiary),
                onChanged: (_) => setState(() {}),
              ),
              SizedBox(height: AppSpacing.lg),
              Text('密码', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: _adminExists ? '请输入密码' : '设置管理员密码',
                controller: _adminPassController,
                obscureText: true,
                prefixIcon: Icon(Icons.lock_outline, size: 20, color: context.textTertiary),
                onChanged: (_) => setState(() {}),
              ),
              if (creatingRemoteAdmin) ...[
                SizedBox(height: AppSpacing.lg),
                Text('Cloud 初始化令牌', style: AppTypography.label(context)),
                SizedBox(height: AppSpacing.xs),
                AmitiaTextField(
                  hintText: 'AMITIA_SETUP_TOKEN（至少 32 位）',
                  controller: _setupTokenController,
                  obscureText: true,
                  prefixIcon: Icon(Icons.key_outlined, size: 20, color: context.textTertiary),
                  onChanged: (_) => setState(() {}),
                ),
                SizedBox(height: AppSpacing.sm),
                Text(
                  '该值必须与 Cloud Core 环境变量 AMITIA_SETUP_TOKEN 完全一致。',
                  style: AppTypography.caption(context),
                ),
              ],
              SizedBox(height: AppSpacing.lg),
              Container(
                padding: EdgeInsets.all(AppSpacing.md),
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: [
                    Icon(Icons.security, size: 18, color: context.accentPrimary),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Text(
                        _adminExists
                            ? '登录成功后，后续模型配置和初始化完成操作都会使用该管理员会话。'
                            : '首管理员创建完成后会直接保存服务端返回的会话，不再额外重复登录。',
                        style: AppTypography.label(context).copyWith(color: context.accentPrimary),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildBoundary() {
    final boundaries = [
      '我已了解 Amitia 将在本地处理我的数据，并拥有相应的访问权限。',
      '我同意 Amitia 在使用过程中调用 AI 模型进行推理，相关数据将发送至模型服务商。',
      '我理解在对话中让 Amitia 执行任务时，可能涉及文件操作、系统命令等高风险操作，并按权限策略要求确认。',
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('使用边界确认', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('请仔细阅读并确认以下使用边界。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        ...List.generate(boundaries.length, (i) {
          return Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.md),
            child: GestureDetector(
              onTap: () => setState(() => _boundaryAgreed[i] = !_boundaryAgreed[i]),
              child: AmitiaCard(
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Icon(
                      _boundaryAgreed[i] ? Icons.check_box : Icons.check_box_outline_blank,
                      size: 22,
                      color: _boundaryAgreed[i] ? context.accentPrimary : context.textTertiary,
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Text(boundaries[i], style: AppTypography.bodySmall(context)),
                    ),
                  ],
                ),
              ),
            ),
          );
        }),
      ],
    );
  }

  Widget _buildModelConfig(
    String title,
    String desc,
    TextEditingController providerCtrl,
    TextEditingController modelCtrl,
    TextEditingController keyCtrl,
    IconData icon,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 22, color: context.accentPrimary),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.sectionTitle(context)),
                  Text(desc, style: AppTypography.caption(context)),
                ],
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('服务商', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '如 OpenAI / Anthropic / DeepSeek',
                controller: providerCtrl,
                prefixIcon: Icon(Icons.business, size: 20, color: context.textTertiary),
              ),
              SizedBox(height: AppSpacing.lg),
              Text('模型名称', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '如 GPT-4o / Claude 3.5 Sonnet',
                controller: modelCtrl,
                prefixIcon: Icon(Icons.psychology_outlined, size: 20, color: context.textTertiary),
              ),
              SizedBox(height: AppSpacing.lg),
              Text('API Key', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '请输入 API Key',
                controller: keyCtrl,
                obscureText: true,
                prefixIcon: Icon(Icons.key, size: 20, color: context.textTertiary),
              ),
              SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Icon(Icons.info_outline, size: 14, color: context.textTertiary),
                  SizedBox(width: AppSpacing.xs),
                  Expanded(
                    child: Text(
                      'API Key 将加密存储，不会明文显示。',
                      style: AppTypography.label(context),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAvatar() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('选择角色头像颜色', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('为你的 AI 角色选择一个标识颜色。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.xl),
        Center(
          child: Container(
            width: 100,
            height: 100,
            decoration: BoxDecoration(
              color: _parseColor(_avatarColors[_selectedAvatarColor]),
              shape: BoxShape.circle,
              boxShadow: [
                BoxShadow(
                  color: _parseColor(_avatarColors[_selectedAvatarColor]).withValues(alpha: 0.3),
                  blurRadius: 20,
                  offset: const Offset(0, 8),
                ),
              ],
            ),
            child: Center(
              child: Text(
                _charNameCtrl.text.isNotEmpty ? _charNameCtrl.text.characters.first : 'A',
                style: const TextStyle(color: Colors.white, fontSize: 40, fontWeight: FontWeight.w600),
              ),
            ),
          ),
        ),
        SizedBox(height: AppSpacing.xxl),
        Text('选择颜色', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: AppSpacing.md,
          runSpacing: AppSpacing.md,
          children: List.generate(_avatarColors.length, (i) {
            final isSelected = _selectedAvatarColor == i;
            final color = _parseColor(_avatarColors[i]);
            return GestureDetector(
              onTap: () => setState(() => _selectedAvatarColor = i),
              child: Container(
                width: 52,
                height: 52,
                decoration: BoxDecoration(
                  color: color,
                  shape: BoxShape.circle,
                  border: Border.all(
                    color: isSelected ? context.surfacePrimary : Colors.transparent,
                    width: 3,
                  ),
                  boxShadow: isSelected
                      ? [BoxShadow(color: color.withValues(alpha: 0.4), blurRadius: 8, spreadRadius: 2)]
                      : null,
                ),
                child: isSelected
                    ? const Icon(Icons.check, color: Colors.white, size: 24)
                    : null,
              ),
            );
          }),
        ),
      ],
    );
  }

  Widget _buildCharName() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('角色名字', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('为你的 AI 角色取一个名字。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        Center(
          child: Container(
            width: 80,
            height: 80,
            decoration: BoxDecoration(
              color: _parseColor(_avatarColors[_selectedAvatarColor]),
              shape: BoxShape.circle,
            ),
            child: Center(
              child: Text(
                _charNameCtrl.text.isNotEmpty ? _charNameCtrl.text.characters.first : '?',
                style: const TextStyle(color: Colors.white, fontSize: 32, fontWeight: FontWeight.w600),
              ),
            ),
          ),
        ),
        SizedBox(height: AppSpacing.xl),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('角色名称', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '如 Amitia / 小雨 / Epsilon',
                controller: _charNameCtrl,
                prefixIcon: Icon(Icons.badge_outlined, size: 20, color: context.textTertiary),
                onChanged: (_) => setState(() {}),
              ),
              SizedBox(height: AppSpacing.md),
              Text('推荐名称', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.sm),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.sm,
                children: ['Amitia', '小雨', 'Epsilon', 'Karin', 'Nova'].map((name) {
                  return GestureDetector(
                    onTap: () {
                      _charNameCtrl.text = name;
                      setState(() {});
                    },
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                      decoration: BoxDecoration(
                        color: context.accentSoft,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(name, style: TextStyle(fontSize: 13, color: context.accentPrimary)),
                    ),
                  );
                }).toList(),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildCharIdentity() {
    final identities = [
      ('专属 AI 伙伴', '陪伴你日常的 AI 朋友'),
      ('效率助手', '帮你处理工作和任务'),
      ('技术顾问', '解答技术问题和提供方案'),
      ('创意搭档', '激发灵感和创意'),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('角色身份', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('定义 ${_charNameCtrl.text.isNotEmpty ? _charNameCtrl.text : '角色'} 的身份定位。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('自定义身份', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '输入角色身份描述',
                controller: _charIdentityCtrl,
                maxLines: 2,
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        Text('或选择预设身份', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.md),
        ...List.generate(identities.length, (i) {
          final id = identities[i];
          final isSelected = _charIdentityCtrl.text == id.$1;
          return Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.sm),
            child: GestureDetector(
              onTap: () {
                _charIdentityCtrl.text = id.$1;
                setState(() {});
              },
              child: AmitiaCard(
                backgroundColor: isSelected ? context.accentSoft : null,
                border: Border.all(
                  color: isSelected ? context.accentPrimary : context.borderPrimary,
                  width: isSelected ? 1.5 : 0.5,
                ),
                child: Row(
                  children: [
                    Icon(
                      isSelected ? Icons.radio_button_checked : Icons.radio_button_off,
                      size: 22,
                      color: isSelected ? context.accentPrimary : context.textTertiary,
                    ),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(id.$1, style: AppTypography.body(context)),
                          Text(id.$2, style: AppTypography.caption(context)),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          );
        }),
      ],
    );
  }

  Widget _buildPersonality() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('角色性格', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('选择 ${_charNameCtrl.text.isNotEmpty ? _charNameCtrl.text : '角色'} 的性格特质（可多选）。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        Wrap(
          spacing: AppSpacing.md,
          runSpacing: AppSpacing.md,
          children: List.generate(_personalityTraits.length, (i) {
            final isSelected = _selectedTraits[i];
            return GestureDetector(
              onTap: () => _toggleTrait(i),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                decoration: BoxDecoration(
                  color: isSelected ? context.accentPrimary : context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(
                    color: isSelected ? context.accentPrimary : context.borderPrimary,
                    width: 1,
                  ),
                ),
                child: Text(
                  _personalityTraits[i],
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            );
          }),
        ),
        SizedBox(height: AppSpacing.lg),
        if (_selectedTraits.asMap().entries.where((e) => e.value).isNotEmpty)
          AmitiaCard(
            backgroundColor: context.accentSoft,
            child: Row(
              children: [
                Icon(Icons.check_circle, size: 18, color: context.accentPrimary),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: Text(
                    '已选择 ${_selectedTraits.where((v) => v).length} 个性格特质',
                    style: AppTypography.caption(context).copyWith(color: context.accentPrimary),
                  ),
                ),
              ],
            ),
          ),
      ],
    );
  }

  Widget _buildInitMemory() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('初始记忆', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('为 ${_charNameCtrl.text.isNotEmpty ? _charNameCtrl.text : '角色'} 设定一些初始记忆，让它更了解你。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('记忆内容', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(
                hintText: '例如：我喜欢喝咖啡，主要做全栈开发...',
                controller: _initMemoryCtrl,
                maxLines: 5,
              ),
              SizedBox(height: AppSpacing.md),
              Text('也可以在后续使用中逐步添加记忆。', style: AppTypography.label(context)),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        Text('快速添加', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.sm),
        Wrap(
          spacing: AppSpacing.sm,
          runSpacing: AppSpacing.sm,
          children: ['我喜欢喝咖啡', '我是程序员', '我喜欢科幻电影', '养了一只猫'].map((item) {
            return GestureDetector(
              onTap: () {
                final current = _initMemoryCtrl.text;
                if (current.isEmpty) {
                  _initMemoryCtrl.text = item;
                } else if (!current.contains(item)) {
                  _initMemoryCtrl.text = '$current；$item';
                }
                setState(() {});
              },
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(item, style: TextStyle(fontSize: 13, color: context.accentPrimary)),
              ),
            );
          }).toList(),
        ),
      ],
    );
  }

  Widget _buildSummary() {
    final traits = _personalityTraits
        .asMap()
        .entries
        .where((e) => _selectedTraits[e.key])
        .map((e) => e.value)
        .join('、');
    final items = <(String, String)>[
      ('部署模式', _deployMode == 0 ? '本地部署' : '云端部署'),
      ('管理员账号', _adminUserController.text.isNotEmpty ? _adminUserController.text : '未设置'),
      ('文本模型', '${_textProviderCtrl.text} / ${_textModelCtrl.text}'),
      ('视觉模型', '${_visionProviderCtrl.text} / ${_visionModelCtrl.text}'),
      ('语音模型', '${_voiceProviderCtrl.text} / ${_voiceModelCtrl.text}'),
      ('向量模型', '${_vectorProviderCtrl.text} / ${_vectorModelCtrl.text}'),
      ('角色名称', _charNameCtrl.text.isNotEmpty ? _charNameCtrl.text : '未设置'),
      ('角色身份', _charIdentityCtrl.text.isNotEmpty ? _charIdentityCtrl.text : '未设置'),
      ('角色性格', traits.isNotEmpty ? traits : '未选择'),
      ('初始记忆', _initMemoryCtrl.text.isNotEmpty ? '${_initMemoryCtrl.text.length} 字' : '无'),
    ];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('设置汇总', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('请确认以下配置信息，确认无误后即可进入 Amitia。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        Center(
          child: Container(
            width: 72,
            height: 72,
            decoration: BoxDecoration(
              color: _parseColor(_avatarColors[_selectedAvatarColor]),
              shape: BoxShape.circle,
            ),
            child: Center(
              child: Text(
                _charNameCtrl.text.isNotEmpty ? _charNameCtrl.text.characters.first : 'A',
                style: const TextStyle(color: Colors.white, fontSize: 28, fontWeight: FontWeight.w600),
              ),
            ),
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          child: Column(
            children: [
              for (int i = 0; i < items.length; i++) ...[
                Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      SizedBox(
                        width: 90,
                        child: Text(items[i].$1, style: AppTypography.label(context)),
                      ),
                      SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Text(
                          items[i].$2,
                          style: AppTypography.bodySmall(context),
                          textAlign: TextAlign.end,
                        ),
                      ),
                    ],
                  ),
                ),
                if (i < items.length - 1)
                  Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildFinish() {
    return Column(
      children: [
        SizedBox(height: AppSpacing.xxl),
        Center(
          child: Container(
            width: 100,
            height: 100,
            decoration: BoxDecoration(
              color: context.accentSoft,
              shape: BoxShape.circle,
            ),
            child: Icon(Icons.celebration, size: 50, color: context.accentPrimary),
          ),
        ),
        SizedBox(height: AppSpacing.xl),
        Center(
          child: Text('一切就绪！', style: AppTypography.pageLargeTitle(context)),
        ),
        SizedBox(height: AppSpacing.md),
        Center(
          child: Text(
            '${_charNameCtrl.text.isNotEmpty ? _charNameCtrl.text : '你的 AI 伙伴'} 正在等你',
            style: AppTypography.body(context).copyWith(color: context.textSecondary),
          ),
        ),
        SizedBox(height: AppSpacing.sectionGap),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('你可以随时在设置中修改以上配置。', style: AppTypography.bodySmall(context)),
              SizedBox(height: AppSpacing.md),
              ...[
                '在对话页面与角色聊天',
                '直接在对话中让 AI 调用工具并执行任务',
                '在设置中管理模型和权限',
                '在角色页面自定义角色属性',
              ].map((item) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: Row(
                  children: [
                    Icon(Icons.arrow_right, size: 18, color: context.accentPrimary),
                    SizedBox(width: AppSpacing.xs),
                    Text(item, style: AppTypography.bodySmall(context)),
                  ],
                ),
              )),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        AmitiaButton(
          label: '进入 Amitia',
          icon: Icons.rocket_launch,
          isFullWidth: true,
          onPressed: () => context.go(AppRoutes.chat),
        ),
      ],
    );
  }
}
