import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../widgets/voice_clone_manager.dart';

class ModelConfigPage extends ConsumerStatefulWidget {
  final String modelType;

  const ModelConfigPage({super.key, required this.modelType});

  @override
  ConsumerState<ModelConfigPage> createState() => _ModelConfigPageState();
}

class _ModelConfigPageState extends ConsumerState<ModelConfigPage> {
  static const Map<String, String> _typeLabels = <String, String>{
    'text': '文本模型',
    'vision': '视觉模型',
    'voice': '语音模型',
    'vector': '向量模型',
    'image': '图像生成',
  };

  final Map<String, int> _testStates = <String, int>{};
  List<Map<String, dynamic>> _configs = const <Map<String, dynamic>>[];
  List<Map<String, dynamic>> _providers = const <Map<String, dynamic>>[];
  bool _loading = true;
  bool _busy = false;
  String? _error;

  String get _typeName => _typeLabels[widget.modelType] ?? '模型配置';
  bool get _isText => widget.modelType == 'text';

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void didUpdateWidget(covariant ModelConfigPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.modelType != widget.modelType) {
      _testStates.clear();
      _load();
    }
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final configs = await _loadConfigs();
      final providers = await _loadProviders();
      if (!mounted) return;
      setState(() {
        _configs = configs;
        _providers = providers;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.toString();
      });
    }
  }

  Future<List<Map<String, dynamic>>> _loadConfigs() async {
    switch (widget.modelType) {
      case 'vision':
        return ref.read(visionServiceProvider).configs();
      case 'voice':
        final items = await ref.read(ttsServiceProvider).listConfigs();
        return items.map((item) => item.toJson()).toList(growable: false);
      case 'vector':
        return ref.read(embeddingServiceProvider).configs();
      case 'image':
        return ref.read(imageGenServiceProvider).configs();
      case 'text':
      default:
        final items = await ref.read(modelConfigServiceProvider).list();
        return items.map((item) => item.toJson()).toList(growable: false);
    }
  }

  Future<List<Map<String, dynamic>>> _loadProviders() async {
    switch (widget.modelType) {
      case 'vision':
        return ref.read(visionServiceProvider).providers();
      case 'voice':
        return ref.read(ttsServiceProvider).providers();
      case 'vector':
        return ref.read(embeddingServiceProvider).providers();
      case 'image':
        return ref.read(imageGenServiceProvider).providers();
      case 'text':
      default:
        return ref.read(modelConfigServiceProvider).providers();
    }
  }

  String _idOf(Map<String, dynamic> item) => (item['id'] ?? '').toString();
  String _providerOf(Map<String, dynamic> item) =>
      (item['apiType'] ?? item['provider'] ?? '').toString();
  String _modelOf(Map<String, dynamic> item) {
    if (widget.modelType == 'voice') {
      final voice = (item['voiceType'] ?? '').toString();
      final resource = (item['resourceId'] ?? '').toString();
      return voice.isNotEmpty ? voice : resource;
    }
    return (item['modelName'] ?? item['model'] ?? '').toString();
  }

  bool _activeOf(Map<String, dynamic> item) {
    final active = item['isActive'];
    return active == true || active == 1 || active == '1';
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _typeName,
        showBackButton: true,
        actions: <Widget>[
          IconButton(
            tooltip: '新建',
            onPressed: _busy ? null : () => _showConfigSheet(null),
            icon: const Icon(Icons.add),
          ),
          IconButton(
            tooltip: '刷新',
            onPressed: _busy ? null : _load,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? _buildError()
          : _buildContent(),
    );
  }

  Widget _buildError() {
    return Center(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.error_outline, size: 48, color: context.error),
            SizedBox(height: AppSpacing.md),
            Text('加载失败', style: AppTypography.body(context)),
            const SizedBox(height: 4),
            Text(
              _error ?? '',
              textAlign: TextAlign.center,
              style: AppTypography.caption(context),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaButton(label: '重试', isSecondary: true, onPressed: _load),
          ],
        ),
      ),
    );
  }

  Widget _buildContent() {
    return ListView(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.xxxl,
      ),
      children: <Widget>[
        Row(
          children: <Widget>[
            Icon(
              Icons.psychology_outlined,
              size: 20,
              color: context.accentPrimary,
            ),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                '已配置 ${_configs.length} 个$_typeName',
                style: AppTypography.caption(context),
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        if (_configs.isEmpty)
          const AmitiaEmptyState(
            icon: Icons.inbox_outlined,
            title: '暂无配置',
            subtitle: '点击右上角新建配置',
          )
        else
          ..._configs.map(_buildConfigCard),
        if (widget.modelType == 'voice') ...<Widget>[
          SizedBox(height: AppSpacing.sectionGap),
          const VoiceCloneManager(),
        ],
      ],
    );
  }

  Widget _buildConfigCard(Map<String, dynamic> config) {
    final id = _idOf(config);
    final testState = _testStates[id] ?? 0;
    final name = (config['name'] ?? '未命名配置').toString();
    return Container(
      margin: EdgeInsets.only(bottom: AppSpacing.md),
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(
                child: Text(name, style: AppTypography.cardTitle(context)),
              ),
              AmitiaStatusBadge(
                label: _activeOf(config) ? '已激活' : '未激活',
                type: _activeOf(config) ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          _InfoRow(
            label: '提供商',
            value: _providerOf(config).isEmpty ? '-' : _providerOf(config),
          ),
          _InfoRow(
            label: widget.modelType == 'voice' ? '声音' : '模型名',
            value: _modelOf(config).isEmpty ? '-' : _modelOf(config),
          ),
          if ((config['baseUrl'] ?? '').toString().isNotEmpty)
            _InfoRow(
              label: 'API 地址',
              value: (config['baseUrl'] ?? '').toString(),
            ),
          if (config['hasApiKey'] == true)
            const _InfoRow(label: 'API Key', value: '已配置'),
          if (_isText) ...<Widget>[
            _InfoRow(
              label: '温度',
              value: ((config['temperature'] as num?)?.toDouble() ?? 0.7)
                  .toStringAsFixed(2),
            ),
            _InfoRow(
              label: '最大 Token',
              value: ((config['maxTokens'] as num?)?.toInt() ?? 4096)
                  .toString(),
            ),
            _InfoRow(
              label: '超时',
              value: '${(config['timeoutSeconds'] as num?)?.toInt() ?? 60}s',
            ),
            _InfoRow(
              label: '重试',
              value: ((config['retryCount'] as num?)?.toInt() ?? 1).toString(),
            ),
          ],
          if (widget.modelType == 'voice') ...<Widget>[
            _InfoRow(
              label: '语速',
              value: ((config['speed'] as num?)?.toDouble() ?? 1)
                  .toStringAsFixed(1),
            ),
            _InfoRow(
              label: '音调',
              value: ((config['pitch'] as num?)?.toDouble() ?? 1)
                  .toStringAsFixed(1),
            ),
            _InfoRow(
              label: '音量',
              value: ((config['volume'] as num?)?.toDouble() ?? 1)
                  .toStringAsFixed(1),
            ),
          ],
          SizedBox(height: AppSpacing.md),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.sm,
            children: <Widget>[
              AmitiaButton(
                label: testState == 1 ? '测试中...' : '测试连接',
                isSecondary: true,
                icon: Icons.bolt,
                height: 36,
                onPressed: testState == 1 || id.isEmpty
                    ? null
                    : () => _testConnection(config),
              ),
              AmitiaButton(
                label: '编辑',
                isSecondary: true,
                icon: Icons.edit_outlined,
                height: 36,
                onPressed: _busy ? null : () => _showConfigSheet(config),
              ),
              if (!_activeOf(config))
                AmitiaButton(
                  label: '设为默认',
                  isSecondary: true,
                  icon: Icons.check_circle_outline,
                  height: 36,
                  onPressed: _busy || id.isEmpty
                      ? null
                      : () => _activate(config),
                ),
              AmitiaButton(
                label: '删除',
                isDestructive: true,
                icon: Icons.delete_outline,
                height: 36,
                onPressed: _busy || id.isEmpty
                    ? null
                    : () => _confirmDelete(config),
              ),
            ],
          ),
          if (testState == 2) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Row(
              children: <Widget>[
                Icon(Icons.check_circle, size: 14, color: context.success),
                const SizedBox(width: 4),
                Text(
                  '连接成功',
                  style: AppTypography.label(
                    context,
                  ).copyWith(color: context.success),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }

  Future<void> _testConnection(Map<String, dynamic> config) async {
    final id = _idOf(config);
    setState(() => _testStates[id] = 1);
    try {
      switch (widget.modelType) {
        case 'vision':
          await ref.read(visionServiceProvider).test(id);
          break;
        case 'voice':
          await ref.read(ttsServiceProvider).test(id);
          break;
        case 'vector':
          await ref.read(embeddingServiceProvider).test(id);
          break;
        case 'image':
          await ref.read(imageGenServiceProvider).test(id);
          break;
        case 'text':
        default:
          await ref.read(modelConfigServiceProvider).test(id);
          break;
      }
      if (mounted) setState(() => _testStates[id] = 2);
    } catch (e) {
      if (mounted) setState(() => _testStates[id] = 0);
      _toast('测试失败：$e', error: true);
    }
  }

  Future<void> _activate(Map<String, dynamic> config) async {
    final id = _idOf(config);
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      switch (widget.modelType) {
        case 'vision':
          await ref.read(visionServiceProvider).activate(id);
          break;
        case 'voice':
          await ref.read(ttsServiceProvider).activate(id);
          break;
        case 'vector':
          await ref.read(embeddingServiceProvider).activate(id);
          break;
        case 'image':
          await ref.read(imageGenServiceProvider).activate(id);
          break;
        case 'text':
        default:
          await ref.read(modelConfigServiceProvider).activate(id);
          break;
      }
      await _load();
      _toast('已设为默认配置');
    } catch (e) {
      _toast('激活失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showConfigSheet(Map<String, dynamic>? existing) async {
    final nameCtrl = TextEditingController(
      text: (existing?['name'] ?? '').toString(),
    );
    final providerCtrl = TextEditingController(
      text: _providerOf(existing ?? const <String, dynamic>{}),
    );
    final modelCtrl = TextEditingController(
      text: _modelOf(existing ?? const <String, dynamic>{}),
    );
    final baseUrlCtrl = TextEditingController(
      text: (existing?['baseUrl'] ?? '').toString(),
    );
    final apiKeyCtrl = TextEditingController();
    final resourceCtrl = TextEditingController(
      text: (existing?['resourceId'] ?? '').toString(),
    );
    final voiceCtrl = TextEditingController(
      text: (existing?['voiceType'] ?? '').toString(),
    );
    final temperatureCtrl = TextEditingController(
      text: ((existing?['temperature'] as num?)?.toDouble() ?? 0.7).toString(),
    );
    final maxTokensCtrl = TextEditingController(
      text: ((existing?['maxTokens'] as num?)?.toInt() ?? 4096).toString(),
    );
    final timeoutCtrl = TextEditingController(
      text: ((existing?['timeoutSeconds'] as num?)?.toInt() ?? 60).toString(),
    );
    final retryCtrl = TextEditingController(
      text: ((existing?['retryCount'] as num?)?.toInt() ?? 1).toString(),
    );
    final speedCtrl = TextEditingController(
      text: ((existing?['speed'] as num?)?.toDouble() ?? 1.0).toString(),
    );
    final pitchCtrl = TextEditingController(
      text: ((existing?['pitch'] as num?)?.toDouble() ?? 1.0).toString(),
    );
    final volumeCtrl = TextEditingController(
      text: ((existing?['volume'] as num?)?.toDouble() ?? 1.0).toString(),
    );
    final cloneResourceCtrl = TextEditingController(
      text: (existing?['cloneResourceId'] ?? 'volc.megatts.timbre').toString(),
    );
    final realtimeAppIdCtrl = TextEditingController(
      text: (existing?['realtimeAppId'] ?? '').toString(),
    );
    final realtimeAccessTokenCtrl = TextEditingController();
    final realtimeSecretKeyCtrl = TextEditingController();
    bool isActive = existing == null ? _configs.isEmpty : _activeOf(existing);
    bool supportsReasoning = existing == null
        ? true
        : existing['supportsReasoning'] == true;
    String defaultReasoningEffort =
        (existing?['defaultReasoningEffort'] ?? 'high').toString();
    bool detecting = false;
    List<Map<String, dynamic>> detectedModels = const <Map<String, dynamic>>[];

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) {
          final selectedProvider = _providers
              .where((p) => (p['id'] ?? '').toString() == providerCtrl.text)
              .firstOrNull;
          return SafeArea(
            child: Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.lg,
                AppSpacing.lg,
                AppSpacing.lg,
                MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
              ),
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(
                      existing == null ? '新建$_typeName配置' : '编辑$_typeName配置',
                      style: AppTypography.sectionTitle(context),
                    ),
                    SizedBox(height: AppSpacing.lg),
                    _SheetField(
                      label: '配置名称',
                      controller: nameCtrl,
                      hint: '用于识别此配置',
                    ),
                    SizedBox(height: AppSpacing.md),
                    Text('提供商', style: AppTypography.label(context)),
                    const SizedBox(height: 4),
                    if (_providers.isEmpty)
                      _SheetField(
                        label: '',
                        controller: providerCtrl,
                        hint: 'Provider / API Type',
                      )
                    else
                      DropdownButtonFormField<String>(
                        value:
                            _providers.any(
                              (p) =>
                                  (p['id'] ?? '').toString() ==
                                  providerCtrl.text,
                            )
                            ? providerCtrl.text
                            : null,
                        isExpanded: true,
                        decoration: const InputDecoration(
                          border: OutlineInputBorder(),
                          isDense: true,
                          hintText: '选择提供商',
                        ),
                        items: _providers
                            .map(
                              (provider) => DropdownMenuItem<String>(
                                value: (provider['id'] ?? '').toString(),
                                child: Text(
                                  (provider['name'] ?? provider['id'] ?? '')
                                      .toString(),
                                ),
                              ),
                            )
                            .toList(growable: false),
                        onChanged: (value) {
                          if (value == null) return;
                          final provider = _providers.firstWhere(
                            (p) => (p['id'] ?? '').toString() == value,
                          );
                          setSheetState(() {
                            providerCtrl.text = value;
                            final defaultBase =
                                (provider['defaultBaseUrl'] ?? '').toString();
                            final defaultModel =
                                (provider['defaultModel'] ?? '').toString();
                            if (baseUrlCtrl.text.trim().isEmpty &&
                                defaultBase.isNotEmpty)
                              baseUrlCtrl.text = defaultBase;
                            if (modelCtrl.text.trim().isEmpty &&
                                defaultModel.isNotEmpty)
                              modelCtrl.text = defaultModel;
                          });
                        },
                      ),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(
                      label: 'API Key',
                      controller: apiKeyCtrl,
                      hint: existing == null ? '输入 API Key' : '留空则保持原 Key',
                      obscure: true,
                    ),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(
                      label: 'API 地址',
                      controller: baseUrlCtrl,
                      hint:
                          (selectedProvider?['defaultBaseUrl'] ?? 'https://...')
                              .toString(),
                    ),
                    SizedBox(height: AppSpacing.md),
                    if (widget.modelType == 'voice') ...<Widget>[
                      _SheetField(
                        label: 'Resource ID',
                        controller: resourceCtrl,
                        hint: '例如 seed-tts-2.0',
                      ),
                      SizedBox(height: AppSpacing.md),
                      _SheetField(
                        label: 'Voice Type',
                        controller: voiceCtrl,
                        hint: '声音/音色标识',
                      ),
                      SizedBox(height: AppSpacing.md),
                      Row(
                        children: <Widget>[
                          Expanded(
                            child: _SheetField(
                              label: '语速',
                              controller: speedCtrl,
                              hint: '0.5 - 2.0',
                              keyboardType:
                                  const TextInputType.numberWithOptions(
                                    decimal: true,
                                  ),
                            ),
                          ),
                          SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: _SheetField(
                              label: '音调',
                              controller: pitchCtrl,
                              hint: '0.5 - 2.0',
                              keyboardType:
                                  const TextInputType.numberWithOptions(
                                    decimal: true,
                                  ),
                            ),
                          ),
                          SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: _SheetField(
                              label: '音量',
                              controller: volumeCtrl,
                              hint: '0.1 - 2.0',
                              keyboardType:
                                  const TextInputType.numberWithOptions(
                                    decimal: true,
                                  ),
                            ),
                          ),
                        ],
                      ),
                      SizedBox(height: AppSpacing.md),
                      _SheetField(
                        label: '复刻 Resource ID',
                        controller: cloneResourceCtrl,
                        hint: 'volc.megatts.timbre',
                      ),
                      SizedBox(height: AppSpacing.md),
                      _SheetField(
                        label: 'Realtime App ID',
                        controller: realtimeAppIdCtrl,
                        hint: '端到端实时语音 App ID',
                      ),
                      SizedBox(height: AppSpacing.md),
                      _SheetField(
                        label: 'Realtime Access Token',
                        controller: realtimeAccessTokenCtrl,
                        hint: existing == null
                            ? '输入实时语音 Access Token'
                            : '留空则保持原 Token',
                        obscure: true,
                      ),
                      SizedBox(height: AppSpacing.md),
                      _SheetField(
                        label: 'Realtime Secret Key',
                        controller: realtimeSecretKeyCtrl,
                        hint: existing == null
                            ? '输入实时语音 Secret Key'
                            : '留空则保持原 Secret',
                        obscure: true,
                      ),
                    ] else ...<Widget>[
                      _SheetField(
                        label: '模型名',
                        controller: modelCtrl,
                        hint: (selectedProvider?['defaultModel'] ?? '模型 ID')
                            .toString(),
                      ),
                      if (_isText) ...<Widget>[
                        SizedBox(height: AppSpacing.sm),
                        Row(
                          children: <Widget>[
                            Expanded(
                              child: AmitiaButton(
                                label: detecting ? '探测中...' : '自动探测模型',
                                isSecondary: true,
                                icon: Icons.travel_explore,
                                onPressed:
                                    detecting || baseUrlCtrl.text.trim().isEmpty
                                    ? null
                                    : () async {
                                        setSheetState(() => detecting = true);
                                        try {
                                          final models = await ref
                                              .read(modelConfigServiceProvider)
                                              .detectModels(
                                                baseUrl: baseUrlCtrl.text
                                                    .trim(),
                                                apiKey: apiKeyCtrl.text.trim(),
                                                apiType: providerCtrl.text
                                                    .trim(),
                                              );
                                          setSheetState(() {
                                            detectedModels = models;
                                            detecting = false;
                                          });
                                        } catch (e) {
                                          setSheetState(
                                            () => detecting = false,
                                          );
                                          _toast('模型探测失败：$e', error: true);
                                        }
                                      },
                              ),
                            ),
                          ],
                        ),
                        if (detectedModels.isNotEmpty) ...<Widget>[
                          SizedBox(height: AppSpacing.sm),
                          DropdownButtonFormField<String>(
                            isExpanded: true,
                            decoration: const InputDecoration(
                              border: OutlineInputBorder(),
                              isDense: true,
                              labelText: '探测结果',
                            ),
                            items: detectedModels
                                .map(
                                  (item) => (item['id'] ?? item['name'] ?? '')
                                      .toString(),
                                )
                                .where((id) => id.isNotEmpty)
                                .map(
                                  (id) => DropdownMenuItem<String>(
                                    value: id,
                                    child: Text(
                                      id,
                                      overflow: TextOverflow.ellipsis,
                                    ),
                                  ),
                                )
                                .toList(growable: false),
                            onChanged: (value) {
                              if (value != null)
                                setSheetState(() => modelCtrl.text = value);
                            },
                          ),
                        ],
                        SizedBox(height: AppSpacing.md),
                        AmitiaSwitchTile(
                          title: '支持思考强度',
                          value: supportsReasoning,
                          onChanged: (value) =>
                              setSheetState(() => supportsReasoning = value),
                        ),
                        const SizedBox(height: 4),
                        DropdownButtonFormField<String>(
                          value:
                              const <String>[
                                'low',
                                'medium',
                                'high',
                                'xhigh',
                              ].contains(defaultReasoningEffort)
                              ? defaultReasoningEffort
                              : 'high',
                          decoration: const InputDecoration(
                            border: OutlineInputBorder(),
                            isDense: true,
                            labelText: '默认思考强度',
                          ),
                          items: const <DropdownMenuItem<String>>[
                            DropdownMenuItem(value: 'low', child: Text('低')),
                            DropdownMenuItem(value: 'medium', child: Text('中')),
                            DropdownMenuItem(value: 'high', child: Text('高')),
                            DropdownMenuItem(value: 'xhigh', child: Text('极高')),
                          ],
                          onChanged: supportsReasoning
                              ? (value) {
                                  if (value == null) return;
                                  setSheetState(
                                    () => defaultReasoningEffort = value,
                                  );
                                }
                              : null,
                        ),
                        SizedBox(height: AppSpacing.md),
                        Row(
                          children: <Widget>[
                            Expanded(
                              child: _SheetField(
                                label: '温度',
                                controller: temperatureCtrl,
                                hint: '0.0 - 2.0',
                                keyboardType:
                                    const TextInputType.numberWithOptions(
                                      decimal: true,
                                    ),
                              ),
                            ),
                            SizedBox(width: AppSpacing.sm),
                            Expanded(
                              child: _SheetField(
                                label: '最大 Token',
                                controller: maxTokensCtrl,
                                hint: '4096',
                                keyboardType: TextInputType.number,
                              ),
                            ),
                          ],
                        ),
                        SizedBox(height: AppSpacing.md),
                        Row(
                          children: <Widget>[
                            Expanded(
                              child: _SheetField(
                                label: '超时（秒）',
                                controller: timeoutCtrl,
                                hint: '60',
                                keyboardType: TextInputType.number,
                              ),
                            ),
                            SizedBox(width: AppSpacing.sm),
                            Expanded(
                              child: _SheetField(
                                label: '重试次数',
                                controller: retryCtrl,
                                hint: '1',
                                keyboardType: TextInputType.number,
                              ),
                            ),
                          ],
                        ),
                      ],
                    ],
                    SizedBox(height: AppSpacing.md),
                    AmitiaSwitchTile(
                      title: '激活此配置',
                      value: isActive,
                      onChanged: (value) =>
                          setSheetState(() => isActive = value),
                    ),
                    SizedBox(height: AppSpacing.lg),
                    AmitiaButton(
                      label: _busy ? '保存中...' : '保存',
                      isFullWidth: true,
                      onPressed: _busy
                          ? null
                          : () async {
                              if (nameCtrl.text.trim().isEmpty) {
                                _toast('配置名称不能为空', error: true);
                                return;
                              }
                              Navigator.of(sheetContext).pop();
                              await _saveConfig(
                                existing,
                                name: nameCtrl.text.trim(),
                                provider: providerCtrl.text.trim(),
                                model: modelCtrl.text.trim(),
                                baseUrl: baseUrlCtrl.text.trim(),
                                apiKey: apiKeyCtrl.text.trim(),
                                resourceId: resourceCtrl.text.trim(),
                                voiceType: voiceCtrl.text.trim(),
                                temperature: temperatureCtrl.text.trim(),
                                maxTokens: maxTokensCtrl.text.trim(),
                                timeoutSeconds: timeoutCtrl.text.trim(),
                                retryCount: retryCtrl.text.trim(),
                                supportsReasoning: supportsReasoning,
                                defaultReasoningEffort: defaultReasoningEffort,
                                speed: speedCtrl.text.trim(),
                                pitch: pitchCtrl.text.trim(),
                                volume: volumeCtrl.text.trim(),
                                cloneResourceId: cloneResourceCtrl.text.trim(),
                                realtimeAppId: realtimeAppIdCtrl.text.trim(),
                                realtimeAccessToken: realtimeAccessTokenCtrl
                                    .text
                                    .trim(),
                                realtimeSecretKey: realtimeSecretKeyCtrl.text
                                    .trim(),
                                isActive: isActive,
                              );
                            },
                    ),
                  ],
                ),
              ),
            ),
          );
        },
      ),
    );

    nameCtrl.dispose();
    providerCtrl.dispose();
    modelCtrl.dispose();
    baseUrlCtrl.dispose();
    apiKeyCtrl.dispose();
    resourceCtrl.dispose();
    voiceCtrl.dispose();
    temperatureCtrl.dispose();
    maxTokensCtrl.dispose();
    timeoutCtrl.dispose();
    retryCtrl.dispose();
    speedCtrl.dispose();
    pitchCtrl.dispose();
    volumeCtrl.dispose();
    cloneResourceCtrl.dispose();
    realtimeAppIdCtrl.dispose();
    realtimeAccessTokenCtrl.dispose();
    realtimeSecretKeyCtrl.dispose();
  }

  Future<void> _saveConfig(
    Map<String, dynamic>? existing, {
    required String name,
    required String provider,
    required String model,
    required String baseUrl,
    required String apiKey,
    required String resourceId,
    required String voiceType,
    required String temperature,
    required String maxTokens,
    required String timeoutSeconds,
    required String retryCount,
    required bool supportsReasoning,
    required String defaultReasoningEffort,
    required String speed,
    required String pitch,
    required String volume,
    required String cloneResourceId,
    required String realtimeAppId,
    required String realtimeAccessToken,
    required String realtimeSecretKey,
    required bool isActive,
  }) async {
    final id = existing == null ? '' : _idOf(existing);
    final data = <String, dynamic>{
      'name': name,
      'apiType': provider,
      'baseUrl': baseUrl,
      'isActive': isActive ? 1 : 0,
      if (apiKey.isNotEmpty) 'apiKey': apiKey,
    };
    if (widget.modelType == 'voice') {
      final parsedSpeed =
          double.tryParse(speed) ??
          (existing?['speed'] as num?)?.toDouble() ??
          1.0;
      final parsedPitch =
          double.tryParse(pitch) ??
          (existing?['pitch'] as num?)?.toDouble() ??
          1.0;
      final parsedVolume =
          double.tryParse(volume) ??
          (existing?['volume'] as num?)?.toDouble() ??
          1.0;
      data['resourceId'] = resourceId;
      data['voiceType'] = voiceType;
      data['speed'] = parsedSpeed.clamp(0.5, 2.0).toDouble();
      data['pitch'] = parsedPitch.clamp(0.5, 2.0).toDouble();
      data['volume'] = parsedVolume.clamp(0.1, 2.0).toDouble();
      data['cloneResourceId'] = cloneResourceId.isEmpty
          ? 'volc.megatts.timbre'
          : cloneResourceId;
      data['realtimeAppId'] = realtimeAppId;
      if (realtimeAccessToken.isNotEmpty)
        data['realtimeAccessToken'] = realtimeAccessToken;
      if (realtimeSecretKey.isNotEmpty)
        data['realtimeSecretKey'] = realtimeSecretKey;
    } else {
      data['modelName'] = model;
      if (_isText) {
        data['temperature'] = (double.tryParse(temperature) ?? 0.7)
            .clamp(0.0, 2.0)
            .toDouble();
        data['maxTokens'] = (int.tryParse(maxTokens) ?? 4096).clamp(1, 1048576);
        data['timeoutSeconds'] = (int.tryParse(timeoutSeconds) ?? 60).clamp(
          1,
          3600,
        );
        data['retryCount'] = (int.tryParse(retryCount) ?? 1).clamp(0, 10);
        data['supportsReasoning'] = supportsReasoning;
        data['defaultReasoningEffort'] = defaultReasoningEffort;
      }
    }

    setState(() => _busy = true);
    try {
      switch (widget.modelType) {
        case 'vision':
          existing == null
              ? await ref.read(visionServiceProvider).createConfig(data)
              : await ref.read(visionServiceProvider).updateConfig(id, data);
          break;
        case 'voice':
          existing == null
              ? await ref.read(ttsServiceProvider).createConfig(data)
              : await ref.read(ttsServiceProvider).updateConfig(id, data);
          break;
        case 'vector':
          existing == null
              ? await ref.read(embeddingServiceProvider).createConfig(data)
              : await ref.read(embeddingServiceProvider).updateConfig(id, data);
          break;
        case 'image':
          existing == null
              ? await ref.read(imageGenServiceProvider).createConfig(data)
              : await ref.read(imageGenServiceProvider).updateConfig(id, data);
          break;
        case 'text':
        default:
          existing == null
              ? await ref.read(modelConfigServiceProvider).create(data)
              : await ref.read(modelConfigServiceProvider).update(id, data);
          break;
      }
      await _load();
      _toast(existing == null ? '已新建配置' : '已更新配置');
    } catch (e) {
      _toast('保存失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _confirmDelete(Map<String, dynamic> config) async {
    final id = _idOf(config);
    if (id.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('删除配置', style: AppTypography.cardTitle(context)),
        content: Text(
          '确定要删除「${config['name'] ?? _modelOf(config)}」吗？',
          style: AppTypography.body(context),
        ),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _busy = true);
    try {
      switch (widget.modelType) {
        case 'vision':
          await ref.read(visionServiceProvider).deleteConfig(id);
          break;
        case 'voice':
          await ref.read(ttsServiceProvider).deleteConfig(id);
          break;
        case 'vector':
          await ref.read(embeddingServiceProvider).deleteConfig(id);
          break;
        case 'image':
          await ref.read(imageGenServiceProvider).deleteConfig(id);
          break;
        case 'text':
        default:
          await ref.read(modelConfigServiceProvider).delete(id);
          break;
      }
      await _load();
      _toast('已删除');
    } catch (e) {
      _toast('删除失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _toast(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? context.error : null,
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
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          SizedBox(
            width: 70,
            child: Text(label, style: AppTypography.label(context)),
          ),
          const SizedBox(width: 8),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _SheetField extends StatelessWidget {
  final String label;
  final TextEditingController controller;
  final String hint;
  final bool obscure;
  final TextInputType? keyboardType;

  const _SheetField({
    required this.label,
    required this.controller,
    required this.hint,
    this.obscure = false,
    this.keyboardType,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: <Widget>[
        if (label.isNotEmpty) ...<Widget>[
          Text(label, style: AppTypography.label(context)),
          const SizedBox(height: 4),
        ],
        AmitiaTextField(
          hintText: hint,
          controller: controller,
          obscureText: obscure,
          keyboardType: keyboardType,
        ),
      ],
    );
  }
}

extension _FirstOrNullExtension<T> on Iterable<T> {
  T? get firstOrNull {
    final iterator = this.iterator;
    if (!iterator.moveNext()) return null;
    return iterator.current;
  }
}
