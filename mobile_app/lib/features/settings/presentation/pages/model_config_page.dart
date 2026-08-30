import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/model_config.dart';
import '../../../../core/models/voice.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ModelConfigPage extends ConsumerStatefulWidget {
  final String modelType;

  const ModelConfigPage({super.key, required this.modelType});

  @override
  ConsumerState<ModelConfigPage> createState() => _ModelConfigPageState();
}

class _ModelConfigPageState extends ConsumerState<ModelConfigPage> {
  final Map<String, int> _testStates = <String, int>{};
  late Future<List<_ModelConfigRecord>> _configsFuture;
  late Future<List<Map<String, dynamic>>> _providersFuture;

  static const Map<String, String> _typeLabels = <String, String>{
    'text': '文本模型',
    'vision': '视觉模型',
    'voice': '语音合成模型',
    'vector': '向量模型',
    'image': '图像生成',
  };

  @override
  void initState() {
    super.initState();
    _reload();
  }

  String get _typeName => _typeLabels[widget.modelType] ?? '模型配置';

  void _reload() {
    _configsFuture = _loadConfigs();
    _providersFuture = _loadProviders();
  }

  Future<List<_ModelConfigRecord>> _loadConfigs() async {
    switch (widget.modelType) {
      case 'vision':
        final rows = await ref.read(visionServiceProvider).configs();
        return rows.map(_ModelConfigRecord.fromMap).toList();
      case 'voice':
        final rows = await ref.read(ttsServiceProvider).listConfigs();
        return rows.map(_ModelConfigRecord.fromVoice).toList();
      case 'vector':
        final rows = await ref.read(embeddingServiceProvider).configs();
        return rows.map(_ModelConfigRecord.fromMap).toList();
      case 'image':
        final rows = await ref.read(imageGenServiceProvider).configs();
        return rows.map(_ModelConfigRecord.fromMap).toList();
      case 'text':
      default:
        final rows = await ref.read(modelConfigServiceProvider).list();
        return rows.map(_ModelConfigRecord.fromText).toList();
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

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: _typeName, showBackButton: true),
      body: FutureBuilder<List<_ModelConfigRecord>>(
        future: _configsFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState != ConnectionState.done) {
            return const AmitiaLoadingState(message: '正在读取模型配置');
          }
          if (snapshot.hasError) {
            return AmitiaErrorState(
              message: '加载失败：${_errorText(snapshot.error!)}',
              onRetry: () => setState(_reload),
            );
          }
          final configs = snapshot.data ?? const <_ModelConfigRecord>[];
          return RefreshIndicator(
            onRefresh: () async {
              setState(_reload);
              await _configsFuture;
            },
            child: ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
              children: <Widget>[
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: Row(
                    children: <Widget>[
                      Icon(_typeIcon, size: 20, color: context.accentPrimary),
                      const SizedBox(width: 8),
                      Text('已配置 ${configs.length} 个$_typeName', style: AppTypography.caption(context)),
                      const Spacer(),
                      AmitiaButton(
                        label: '新建',
                        icon: Icons.add,
                        height: 36,
                        width: 92,
                        onPressed: () => _showConfigSheet(null),
                      ),
                    ],
                  ),
                ),
                SizedBox(height: AppSpacing.md),
                if (configs.isEmpty)
                  AmitiaEmptyState(
                    icon: _typeIcon,
                    title: '暂无$_typeName配置',
                    subtitle: '这里直接读取后端真实配置，不使用本地模拟数据。',
                    actionText: '新建配置',
                    onAction: () => _showConfigSheet(null),
                  )
                else
                  ...configs.map(_buildConfigCard),
                SizedBox(height: AppSpacing.xl),
              ],
            ),
          );
        },
      ),
    );
  }

  IconData get _typeIcon {
    switch (widget.modelType) {
      case 'vision':
        return Icons.visibility_outlined;
      case 'voice':
        return Icons.graphic_eq_outlined;
      case 'vector':
        return Icons.hub_outlined;
      case 'image':
        return Icons.image_outlined;
      default:
        return Icons.psychology_outlined;
    }
  }

  Widget _buildConfigCard(_ModelConfigRecord config) {
    final testState = _testStates[config.id] ?? 0;
    return Container(
      margin: EdgeInsets.only(
        left: AppSpacing.pagePadding,
        right: AppSpacing.pagePadding,
        bottom: AppSpacing.md,
      ),
      padding: EdgeInsets.all(AppSpacing.cardPadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.6),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          Row(
            children: <Widget>[
              Expanded(child: Text(config.name.isEmpty ? '未命名配置' : config.name, style: AppTypography.cardTitle(context))),
              AmitiaStatusBadge(
                label: config.isActive ? '当前使用' : '未激活',
                type: config.isActive ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          _InfoRow(label: '提供商', value: config.provider.isEmpty ? '未设置' : config.provider),
          _InfoRow(label: widget.modelType == 'voice' ? '资源模型' : '模型', value: config.model.isEmpty ? '未设置' : config.model),
          if (widget.modelType == 'voice' && config.voiceType.isNotEmpty)
            _InfoRow(label: '默认音色', value: config.voiceType),
          if (config.baseUrl.isNotEmpty) _InfoRow(label: 'API 地址', value: config.baseUrl),
          SizedBox(height: AppSpacing.md),
          Row(
            children: <Widget>[
              Expanded(
                child: AmitiaButton(
                  label: testState == 1 ? '测试中…' : '测试连接',
                  isSecondary: true,
                  icon: Icons.bolt_outlined,
                  onPressed: testState == 1 ? null : () => _testConnection(config.id),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AmitiaButton(
                  label: '编辑',
                  isSecondary: true,
                  icon: Icons.edit_outlined,
                  onPressed: () => _showConfigSheet(config),
                ),
              ),
            ],
          ),
          if (testState == 2) ...<Widget>[
            SizedBox(height: AppSpacing.sm),
            Row(
              children: <Widget>[
                Icon(Icons.check_circle_outline, size: 15, color: context.success),
                const SizedBox(width: 5),
                Text('后端连接测试通过', style: AppTypography.label(context).copyWith(color: context.success)),
              ],
            ),
          ],
          SizedBox(height: AppSpacing.sm),
          Row(
            children: <Widget>[
              if (!config.isActive) ...<Widget>[
                Expanded(
                  child: AmitiaButton(
                    label: '设为当前',
                    isSecondary: true,
                    icon: Icons.check_circle_outline,
                    onPressed: () => _activate(config.id),
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
              ],
              Expanded(
                child: AmitiaButton(
                  label: '删除',
                  isDestructive: true,
                  icon: Icons.delete_outline,
                  onPressed: () => _confirmDelete(config),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _testConnection(String id) async {
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
    } catch (error) {
      if (mounted) {
        setState(() => _testStates[id] = 0);
        _snack('测试失败：${_errorText(error)}', error: true);
      }
    }
  }

  Future<void> _activate(String id) async {
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
      if (!mounted) return;
      setState(_reload);
      _snack('已设为当前$_typeName');
    } catch (error) {
      if (mounted) _snack('激活失败：${_errorText(error)}', error: true);
    }
  }

  Future<void> _showConfigSheet(_ModelConfigRecord? existing) async {
    List<Map<String, dynamic>> providers;
    try {
      providers = await _providersFuture;
    } catch (_) {
      providers = const <Map<String, dynamic>>[];
    }
    if (!mounted) return;

    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final providerCtrl = TextEditingController(text: existing?.provider ?? '');
    final modelCtrl = TextEditingController(text: existing?.model ?? '');
    final baseUrlCtrl = TextEditingController(text: existing?.baseUrl ?? '');
    final apiKeyCtrl = TextEditingController();
    final voiceTypeCtrl = TextEditingController(text: existing?.voiceType ?? '');
    final temperatureCtrl = TextEditingController(text: existing == null ? '0.7' : existing.temperature.toString());
    final maxTokensCtrl = TextEditingController(text: existing == null ? '4096' : existing.maxTokens.toString());
    final speedCtrl = TextEditingController(text: existing == null ? '1.0' : existing.speed.toString());
    final pitchCtrl = TextEditingController(text: existing == null ? '1.0' : existing.pitch.toString());
    final volumeCtrl = TextEditingController(text: existing == null ? '1.0' : existing.volume.toString());
    var isActive = existing?.isActive ?? false;
    var selectedProvider = providerCtrl.text.trim();

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) {
          void chooseProvider(String providerId) {
            setSheetState(() {
              selectedProvider = providerId;
              providerCtrl.text = providerId;
              Map<String, dynamic>? provider;
              for (final item in providers) {
                if ((item['id'] ?? '').toString() == providerId) {
                  provider = item;
                  break;
                }
              }
              if (provider != null) {
                if (baseUrlCtrl.text.trim().isEmpty) baseUrlCtrl.text = (provider['defaultBaseUrl'] ?? '').toString();
                if (modelCtrl.text.trim().isEmpty) modelCtrl.text = (provider['defaultModel'] ?? '').toString();
              }
            });
          }

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
                    Text(existing == null ? '新建$_typeName配置' : '编辑$_typeName配置', style: AppTypography.sectionTitle(context)),
                    SizedBox(height: AppSpacing.lg),
                    _SheetField(label: '配置名称', controller: nameCtrl, hint: '用于区分多个配置'),
                    SizedBox(height: AppSpacing.md),
                    if (providers.isNotEmpty) ...<Widget>[
                      Text('提供商', style: AppTypography.label(context)),
                      const SizedBox(height: 7),
                      Wrap(
                        spacing: 7,
                        runSpacing: 7,
                        children: providers.map((provider) {
                          final id = (provider['id'] ?? '').toString();
                          final label = (provider['name'] ?? id).toString();
                          final selected = id == selectedProvider;
                          return GestureDetector(
                            onTap: () => chooseProvider(id),
                            child: Container(
                              padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 7),
                              decoration: BoxDecoration(
                                color: selected ? context.accentSoft : context.surfaceSecondary,
                                borderRadius: AppRadius.brTag,
                                border: Border.all(color: selected ? context.accentSecondary : context.borderPrimary, width: 0.6),
                              ),
                              child: Text(label, style: AppTypography.caption(context).copyWith(color: selected ? context.accentPrimary : context.textSecondary)),
                            ),
                          );
                        }).toList(),
                      ),
                      SizedBox(height: AppSpacing.md),
                    ],
                    _SheetField(label: 'API 类型', controller: providerCtrl, hint: '后端 apiType'),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(
                      label: widget.modelType == 'voice' ? '资源 / 模型 ID' : '模型名称',
                      controller: modelCtrl,
                      hint: widget.modelType == 'voice' ? '例如 seed-tts-2.0' : '填写服务商模型 ID',
                    ),
                    if (widget.modelType == 'voice') ...<Widget>[
                      SizedBox(height: AppSpacing.md),
                      _SheetField(label: '默认音色', controller: voiceTypeCtrl, hint: '例如 zh_female_vv_uranus_bigtts'),
                    ],
                    SizedBox(height: AppSpacing.md),
                    _SheetField(label: 'API 地址', controller: baseUrlCtrl, hint: '服务商 Base URL'),
                    SizedBox(height: AppSpacing.md),
                    _SheetField(
                      label: 'API Key',
                      controller: apiKeyCtrl,
                      hint: existing == null ? '按服务商要求填写；本地服务可留空' : '留空则保留现有 Key',
                      obscure: true,
                    ),
                    if (widget.modelType == 'text') ...<Widget>[
                      SizedBox(height: AppSpacing.md),
                      Row(
                        children: <Widget>[
                          Expanded(child: _SheetField(label: 'Temperature', controller: temperatureCtrl, hint: '0.7', keyboardType: const TextInputType.numberWithOptions(decimal: true))),
                          SizedBox(width: AppSpacing.sm),
                          Expanded(child: _SheetField(label: 'Max Tokens', controller: maxTokensCtrl, hint: '4096', keyboardType: TextInputType.number)),
                        ],
                      ),
                    ],
                    if (widget.modelType == 'voice') ...<Widget>[
                      SizedBox(height: AppSpacing.md),
                      Row(
                        children: <Widget>[
                          Expanded(child: _SheetField(label: '语速', controller: speedCtrl, hint: '1.0', keyboardType: const TextInputType.numberWithOptions(decimal: true))),
                          SizedBox(width: AppSpacing.sm),
                          Expanded(child: _SheetField(label: '音调', controller: pitchCtrl, hint: '1.0', keyboardType: const TextInputType.numberWithOptions(decimal: true))),
                          SizedBox(width: AppSpacing.sm),
                          Expanded(child: _SheetField(label: '音量', controller: volumeCtrl, hint: '1.0', keyboardType: const TextInputType.numberWithOptions(decimal: true))),
                        ],
                      ),
                    ],
                    SizedBox(height: AppSpacing.md),
                    AmitiaSwitchTile(
                      title: '设为当前配置',
                      subtitle: '保存后调用对应后端激活接口，确保同类配置只有一个当前项。',
                      value: isActive,
                      onChanged: (value) => setSheetState(() => isActive = value),
                    ),
                    SizedBox(height: AppSpacing.lg),
                    AmitiaButton(
                      label: '保存',
                      isFullWidth: true,
                      onPressed: () async {
                        final name = nameCtrl.text.trim();
                        if (name.isEmpty) {
                          _snack('请输入配置名称', error: true);
                          return;
                        }
                        try {
                          final data = <String, dynamic>{
                            'name': name,
                            'apiType': providerCtrl.text.trim(),
                            'baseUrl': baseUrlCtrl.text.trim(),
                            'isActive': isActive ? 1 : 0,
                            if (apiKeyCtrl.text.trim().isNotEmpty) 'apiKey': apiKeyCtrl.text.trim(),
                          };
                          if (widget.modelType == 'voice') {
                            data['resourceId'] = modelCtrl.text.trim();
                            data['voiceType'] = voiceTypeCtrl.text.trim();
                            data['speed'] = double.tryParse(speedCtrl.text.trim()) ?? 1.0;
                            data['pitch'] = double.tryParse(pitchCtrl.text.trim()) ?? 1.0;
                            data['volume'] = double.tryParse(volumeCtrl.text.trim()) ?? 1.0;
                          } else {
                            data['modelName'] = modelCtrl.text.trim();
                          }
                          if (widget.modelType == 'text') {
                            data['temperature'] = double.tryParse(temperatureCtrl.text.trim()) ?? 0.7;
                            data['maxTokens'] = int.tryParse(maxTokensCtrl.text.trim()) ?? 4096;
                          }

                          final savedId = await _saveConfig(existing?.id, data);
                          if (isActive && savedId.isNotEmpty) await _activateRaw(savedId);
                          if (!mounted) return;
                          Navigator.of(sheetContext).pop();
                          setState(_reload);
                          _snack(existing == null ? '已创建$_typeName配置' : '已更新$_typeName配置');
                        } catch (error) {
                          if (mounted) _snack('保存失败：${_errorText(error)}', error: true);
                        }
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
    voiceTypeCtrl.dispose();
    temperatureCtrl.dispose();
    maxTokensCtrl.dispose();
    speedCtrl.dispose();
    pitchCtrl.dispose();
    volumeCtrl.dispose();
  }

  Future<String> _saveConfig(String? existingId, Map<String, dynamic> data) async {
    switch (widget.modelType) {
      case 'vision':
        final result = existingId == null
            ? await ref.read(visionServiceProvider).createConfig(data)
            : await ref.read(visionServiceProvider).updateConfig(existingId, data);
        return (result?['id'] ?? existingId ?? '').toString();
      case 'voice':
        final result = existingId == null
            ? await ref.read(ttsServiceProvider).createConfig(data)
            : await ref.read(ttsServiceProvider).updateConfig(existingId, data);
        return result?.id ?? existingId ?? '';
      case 'vector':
        final result = existingId == null
            ? await ref.read(embeddingServiceProvider).createConfig(data)
            : await ref.read(embeddingServiceProvider).updateConfig(existingId, data);
        return (result?['id'] ?? existingId ?? '').toString();
      case 'image':
        final result = existingId == null
            ? await ref.read(imageGenServiceProvider).createConfig(data)
            : await ref.read(imageGenServiceProvider).updateConfig(existingId, data);
        return (result?['id'] ?? existingId ?? '').toString();
      case 'text':
      default:
        final result = existingId == null
            ? await ref.read(modelConfigServiceProvider).create(data)
            : await ref.read(modelConfigServiceProvider).update(existingId, data);
        return result?.id ?? existingId ?? '';
    }
  }

  Future<void> _activateRaw(String id) async {
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
  }

  void _confirmDelete(_ModelConfigRecord config) {
    showDialog<void>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除配置'),
        content: Text('确定删除「${config.name}」吗？此操作会调用后端删除接口。'),
        actions: <Widget>[
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(dialogContext);
              try {
                switch (widget.modelType) {
                  case 'vision':
                    await ref.read(visionServiceProvider).deleteConfig(config.id);
                    break;
                  case 'voice':
                    await ref.read(ttsServiceProvider).deleteConfig(config.id);
                    break;
                  case 'vector':
                    await ref.read(embeddingServiceProvider).deleteConfig(config.id);
                    break;
                  case 'image':
                    await ref.read(imageGenServiceProvider).deleteConfig(config.id);
                    break;
                  case 'text':
                  default:
                    await ref.read(modelConfigServiceProvider).delete(config.id);
                    break;
                }
                if (!mounted) return;
                setState(_reload);
                _snack('已删除配置');
              } catch (error) {
                if (mounted) _snack('删除失败：${_errorText(error)}', error: true);
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _snack(String message, {bool error = false}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  static String _errorText(Object error) => error
      .toString()
      .replaceFirst('Bad state: ', '')
      .replaceFirst('Exception: ', '');
}

class _ModelConfigRecord {
  final String id;
  final String name;
  final String provider;
  final String model;
  final String baseUrl;
  final bool isActive;
  final String voiceType;
  final double temperature;
  final int maxTokens;
  final double speed;
  final double pitch;
  final double volume;

  const _ModelConfigRecord({
    required this.id,
    required this.name,
    required this.provider,
    required this.model,
    required this.baseUrl,
    required this.isActive,
    this.voiceType = '',
    this.temperature = 0.7,
    this.maxTokens = 4096,
    this.speed = 1.0,
    this.pitch = 1.0,
    this.volume = 1.0,
  });

  factory _ModelConfigRecord.fromText(ModelConfigDto dto) {
    return _ModelConfigRecord(
      id: dto.id,
      name: dto.name,
      provider: dto.provider,
      model: dto.model,
      baseUrl: dto.baseUrl,
      isActive: dto.isActive == 1,
      temperature: dto.temperature,
      maxTokens: dto.maxTokens,
    );
  }

  factory _ModelConfigRecord.fromVoice(VoiceConfigDto dto) {
    return _ModelConfigRecord(
      id: dto.id,
      name: dto.name,
      provider: dto.provider,
      model: dto.resourceId,
      baseUrl: dto.baseUrl,
      isActive: dto.isActive == 1,
      voiceType: dto.voiceId,
      speed: dto.speed,
      pitch: dto.pitch,
      volume: dto.volume,
    );
  }

  factory _ModelConfigRecord.fromMap(Map<String, dynamic> json) {
    final activeRaw = json['isActive'];
    final active = activeRaw == true || (activeRaw is num && activeRaw.toInt() == 1) || activeRaw?.toString() == '1';
    return _ModelConfigRecord(
      id: (json['id'] ?? '').toString(),
      name: (json['name'] ?? '').toString(),
      provider: (json['apiType'] ?? json['provider'] ?? '').toString(),
      model: (json['modelName'] ?? json['model'] ?? json['resourceId'] ?? '').toString(),
      baseUrl: (json['baseUrl'] ?? '').toString(),
      isActive: active,
      voiceType: (json['voiceType'] ?? '').toString(),
      temperature: (json['temperature'] as num?)?.toDouble() ?? 0.7,
      maxTokens: (json['maxTokens'] as num?)?.toInt() ?? 4096,
      speed: (json['speed'] as num?)?.toDouble() ?? 1.0,
      pitch: (json['pitch'] as num?)?.toDouble() ?? 1.0,
      volume: (json['volume'] as num?)?.toDouble() ?? 1.0,
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
      padding: const EdgeInsets.symmetric(vertical: 3),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: <Widget>[
          SizedBox(width: 64, child: Text(label, style: AppTypography.label(context))),
          SizedBox(width: AppSpacing.sm),
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
        Text(label, style: AppTypography.label(context)),
        const SizedBox(height: 5),
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
