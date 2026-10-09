import 'dart:async';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:amitia_app/core/widgets/amitia_popup_menu.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/backend_transport/core_configuration_intent.dart';
import '../../../../core/services/core_configuration_guard.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class AsrPage extends ConsumerStatefulWidget {
  const AsrPage({super.key});

  @override
  ConsumerState<AsrPage> createState() => _AsrPageState();
}

class _AsrPageState extends ConsumerState<AsrPage> {
  List<Map<String, dynamic>> _configs = const [];
  List<Map<String, dynamic>> _providers = const [];
  bool _loading = true;
  bool _busy = false;
  String? _error;
  String _audioUrl = '';
  String _language = '';
  String _taskId = '';
  String _status = '';
  String _result = '';
  Timer? _pollTimer;
  CoreConfigurationIntent? _configurationIntent;
  CoreConfigurationIntent? _audioIntent;
  CoreConfigurationIntent? _taskIntent;
  int _loadEpoch = 0;

  @override
  void initState() {
    super.initState();
    _loadConfigs();
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    super.dispose();
  }

  Future<void> _loadConfigs() async {
    final epoch = ++_loadEpoch;
    _pollTimer?.cancel();
    _audioIntent = null;
    _taskIntent = null;
    _configurationIntent = null;
    setState(() {
      _audioUrl = '';
      _taskId = '';
      _result = '';
      _loading = true;
      _error = null;
    });
    try {
      final intent = await coreConfigurationGuardFor(ref).capture();
      if (!mounted || epoch != _loadEpoch) return;
      _configurationIntent = intent;
      if (!intent.canConfigure) {
        throw StateError('AI 服务由云端 Core 提供，当前设备不能配置语音识别模型');
      }
      final service = ref.read(asrServiceProvider);
      final configs = await intent.run(service.configs);
      final providers = await intent.run(service.providers);
      await coreConfigurationGuardFor(ref).validate(intent);
      if (!mounted || epoch != _loadEpoch) return;
      setState(() {
        _configs = configs;
        _providers = providers;
        _loading = false;
      });
    } catch (e) {
      if (!mounted || epoch != _loadEpoch) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  bool get _configured => _configs.any((item) {
    final active = item['isActive'];
    final isActive = active == 1 || active == true;
    return isActive && item['hasApiKey'] == true;
  });

  Future<void> _activate(Map<String, dynamic> config) async {
    final intent = _configurationIntent;
    if (!await _ensureConfiguration(intent)) return;
    if (CoreConfigurationIntent.current == null) {
      return intent!.run(() => _activate(config));
    }
    final id = (config['id'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      await ref.read(asrServiceProvider).activate(id);
      await _loadConfigs();
      _show('ASR 配置已启用');
    } catch (e) {
      _show('启用失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _test(Map<String, dynamic> config) async {
    final intent = _configurationIntent;
    if (!await _ensureConfiguration(intent)) return;
    if (CoreConfigurationIntent.current == null) {
      return intent!.run(() => _test(config));
    }
    final id = (config['id'] ?? '').toString();
    if (id.isEmpty) return;
    setState(() => _busy = true);
    try {
      final result = await ref.read(asrServiceProvider).test(id);
      _show((result?['message'] ?? result?['msg'] ?? '连接测试完成').toString());
    } catch (e) {
      _show('测试失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _showConfigSheet([Map<String, dynamic>? existing]) async {
    final intent = _configurationIntent;
    if (!await _ensureConfiguration(intent)) return;
    final nameCtrl = TextEditingController(
      text: (existing?['name'] ?? '').toString(),
    );
    final typeCtrl = TextEditingController(
      text: (existing?['apiType'] ?? '').toString(),
    );
    final keyCtrl = TextEditingController();
    final baseCtrl = TextEditingController(
      text: (existing?['baseUrl'] ?? '').toString(),
    );
    final resourceCtrl = TextEditingController(
      text: (existing?['resourceId'] ?? '').toString(),
    );
    bool active = existing == null
        ? _configs.isEmpty
        : (existing['isActive'] == 1 || existing['isActive'] == true);

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
          child: Padding(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.lg,
              0,
              AppSpacing.lg,
              MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
            ),
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(
                    existing == null ? '新建 ASR 配置' : '编辑 ASR 配置',
                    style: AppTypography.sectionTitle(context),
                  ),
                  SizedBox(height: AppSpacing.lg),
                  TextField(
                    controller: nameCtrl,
                    decoration: const InputDecoration(
                      labelText: '配置名称',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  if (_providers.isEmpty)
                    TextField(
                      controller: typeCtrl,
                      decoration: const InputDecoration(
                        labelText: 'Provider / API Type',
                        border: OutlineInputBorder(),
                      ),
                    )
                  else
                    DropdownButtonFormField<String>(
                      value:
                          _providers.any(
                            (p) => (p['id'] ?? '').toString() == typeCtrl.text,
                          )
                          ? typeCtrl.text
                          : null,
                      isExpanded: true,
                      decoration: const InputDecoration(
                        labelText: 'Provider',
                        border: OutlineInputBorder(),
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
                          (item) => (item['id'] ?? '').toString() == value,
                        );
                        setSheetState(() {
                          typeCtrl.text = value;
                          if (baseCtrl.text.trim().isEmpty)
                            baseCtrl.text = (provider['defaultBaseUrl'] ?? '')
                                .toString();
                          if (resourceCtrl.text.trim().isEmpty)
                            resourceCtrl.text = (provider['defaultModel'] ?? '')
                                .toString();
                        });
                      },
                    ),
                  SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: keyCtrl,
                    obscureText: true,
                    decoration: InputDecoration(
                      labelText: 'API Key',
                      hintText: existing == null ? '输入 API Key' : '留空则保持原 Key',
                      border: const OutlineInputBorder(),
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: baseCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Base URL',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  TextField(
                    controller: resourceCtrl,
                    decoration: const InputDecoration(
                      labelText: 'Resource / Model ID',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  SwitchListTile(
                    contentPadding: EdgeInsets.zero,
                    title: const Text('激活此配置'),
                    value: active,
                    onChanged: (value) => setSheetState(() => active = value),
                  ),
                  SizedBox(height: AppSpacing.md),
                  AmitiaButton(
                    label: '保存',
                    isFullWidth: true,
                    onPressed: () async {
                      if (nameCtrl.text.trim().isEmpty ||
                          typeCtrl.text.trim().isEmpty) {
                        _show('名称和 Provider 不能为空', error: true);
                        return;
                      }
                      final data = <String, dynamic>{
                        'name': nameCtrl.text.trim(),
                        'apiType': typeCtrl.text.trim(),
                        'baseUrl': baseCtrl.text.trim(),
                        'resourceId': resourceCtrl.text.trim(),
                        'isActive': active ? 1 : 0,
                        if (keyCtrl.text.trim().isNotEmpty)
                          'apiKey': keyCtrl.text.trim(),
                      };
                      Navigator.of(sheetContext).pop();
                      await _saveConfig(existing, data, intent!);
                    },
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );

    nameCtrl.dispose();
    typeCtrl.dispose();
    keyCtrl.dispose();
    baseCtrl.dispose();
    resourceCtrl.dispose();
  }

  Future<void> _saveConfig(
    Map<String, dynamic>? existing,
    Map<String, dynamic> data,
    CoreConfigurationIntent intent,
  ) async {
    setState(() => _busy = true);
    try {
      if (!await _ensureConfiguration(intent)) return;
      final service = ref.read(asrServiceProvider);
      await intent.run(() async {
        if (existing == null) {
          await service.createConfig(data);
        } else {
          final id = (existing['id'] ?? '').toString();
          if (id.isEmpty) throw StateError('ASR 配置 ID 无效');
          await service.updateConfig(id, data);
        }
      });
      await _loadConfigs();
      _show(existing == null ? 'ASR 配置已创建' : 'ASR 配置已更新');
    } catch (e) {
      _show('保存失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _deleteConfig(Map<String, dynamic> config) async {
    final intent = _configurationIntent;
    if (!await _ensureConfiguration(intent)) return;
    final id = (config['id'] ?? '').toString();
    if (id.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除 ASR 配置'),
        content: Text('确定删除「${config['name'] ?? id}」吗？'),
        actions: [
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
      if (!await _ensureConfiguration(intent)) return;
      await intent!.run(() => ref.read(asrServiceProvider).deleteConfig(id));
      await _loadConfigs();
      _show('ASR 配置已删除');
    } catch (e) {
      _show('删除失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _pickAndUpload() async {
    final intent = _configurationIntent;
    if (!await _ensureConfiguration(intent)) return;
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['mp3', 'wav', 'ogg', 'm4a', 'aac', 'pcm'],
    );
    if (picked == null || picked.files.isEmpty) return;
    if (!await _ensureConfiguration(intent)) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) {
      _show('无法读取所选音频文件', error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      final payload = await intent!.run(
        () => ref.read(asrServiceProvider).uploadAudio(file.path!),
      );
      if (!await _ensureConfiguration(intent)) return;
      final url = (payload?['url'] ?? '').toString();
      if (url.isEmpty) throw StateError('后端未返回音频地址');
      if (!mounted) return;
      setState(() {
        _audioUrl = url;
        _audioIntent = intent;
      });
      _show('音频已上传');
    } catch (e) {
      _show('上传失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<bool> _ensureConfiguration(CoreConfigurationIntent? intent) async {
    try {
      if (intent == null) throw StateError('语音识别配置归属无法确认，请重新加载');
      if (!identical(intent, _configurationIntent))
        throw StateError('语音识别原配置页面已变化，请重新选择音频');
      await coreConfigurationGuardFor(ref).validate(intent);
      return mounted;
    } catch (error) {
      if (mounted) {
        _pollTimer?.cancel();
        _audioIntent = null;
        _taskIntent = null;
        _configurationIntent = null;
        _loadEpoch++;
        _audioUrl = '';
        _taskId = '';
        _result = '';
        setState(() => _error = error.toString());
        _show('模型配置已禁用：$error', error: true);
      }
      return false;
    }
  }

  Future<void> _submit() async {
    final intent = _audioIntent;
    if (!await _ensureConfiguration(intent)) return;
    if (!_configured) {
      _show('请先配置并启用 ASR API Key', error: true);
      return;
    }
    if (_audioUrl.trim().isEmpty) {
      _show('请先选择音频文件', error: true);
      return;
    }
    setState(() => _busy = true);
    try {
      final url = _audioUrl.trim();
      final language = _language;
      final payload = await intent!.run(
        () => ref.read(asrServiceProvider).submitUrl(url, language: language),
      );
      if (!await _ensureConfiguration(intent)) return;
      final taskId = (payload?['taskId'] ?? '').toString();
      if (taskId.isEmpty) throw StateError('后端未返回任务 ID');
      if (!mounted) return;
      setState(() {
        _taskId = taskId;
        _taskIntent = intent;
        _status = 'processing';
        _result = '';
      });
      _startPolling();
    } catch (e) {
      _show('提交失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _startPolling() {
    _pollTimer?.cancel();
    _pollTimer = Timer.periodic(const Duration(seconds: 2), (_) => _poll());
    _poll();
  }

  Future<void> _poll() async {
    if (_taskId.isEmpty) return;
    final intent = _taskIntent;
    final taskId = _taskId;
    if (!await _ensureConfiguration(intent)) return;
    try {
      final response = await intent!.run(
        () => ref.read(asrServiceProvider).queryResult(taskId),
      );
      if (!await _ensureConfiguration(intent) ||
          taskId != _taskId ||
          !identical(intent, _taskIntent) ||
          response == null)
        return;
      final status = (response['status'] ?? '').toString();
      final result = (response['result'] ?? '').toString();
      setState(() {
        _status = status.isEmpty ? _status : status;
        if (result.isNotEmpty) _result = result;
      });
      if (status == 'success' ||
          status == 'completed' ||
          status == 'failed' ||
          status == 'error') {
        _pollTimer?.cancel();
        _pollTimer = null;
      }
    } catch (e) {
      _pollTimer?.cancel();
      _pollTimer = null;
      _show('查询识别结果失败：$e', error: true);
    }
  }

  void _show(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? context.error : null,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    void invalidate() {
      _loadEpoch++;
      _pollTimer?.cancel();
      _configurationIntent = null;
      _audioIntent = null;
      _taskIntent = null;
      if (mounted)
        setState(() {
          _configs = const [];
          _providers = const [];
          _audioUrl = '';
          _taskId = '';
          _result = '';
          _loading = false;
          _error = 'Core连接或设备模式已变化，原语音识别测试已取消，请重新加载';
        });
    }

    ref.listen(rawBackendServiceApiProvider, (_, __) => invalidate());
    ref.listen(mobileDeploymentConfigProvider, (_, __) => invalidate());
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '语音识别',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            onPressed: _busy || _configurationIntent?.canConfigure != true
                ? null
                : () => _showConfigSheet(),
            icon: const Icon(Icons.add),
            tooltip: '新建 ASR 配置',
          ),
          IconButton(
            onPressed: _busy ? null : _loadConfigs,
            icon: const Icon(Icons.refresh),
            tooltip: '刷新',
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(_error!, textAlign: TextAlign.center),
                  AmitiaButton(label: '重新加载', onPressed: _loadConfigs),
                ],
              ),
            )
          : ListView(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.md,
                AppSpacing.pagePadding,
                AppSpacing.xxxl,
              ),
              children: [
                _statusCard(context),
                SizedBox(height: AppSpacing.sectionGap),
                Text('ASR 配置', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.sm),
                if (_configs.isEmpty)
                  AmitiaCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.stretch,
                      children: [
                        Text(
                          '暂无 ASR 配置。',
                          style: AppTypography.caption(context),
                        ),
                        const SizedBox(height: 10),
                        AmitiaButton(
                          label: '新建 ASR 配置',
                          icon: Icons.add,
                          isSecondary: true,
                          onPressed: _busy ? null : () => _showConfigSheet(),
                        ),
                      ],
                    ),
                  )
                else
                  ..._configs.map(
                    (config) => Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: _configCard(context, config),
                    ),
                  ),
                SizedBox(height: AppSpacing.sectionGap),
                Text('音频识别', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.sm),
                AmitiaCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        _audioUrl.isEmpty ? '尚未选择音频' : _audioUrl,
                        style: AppTypography.caption(context),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                      const SizedBox(height: 12),
                      AmitiaButton(
                        label: '选择并上传音频',
                        icon: Icons.audio_file_outlined,
                        isSecondary: true,
                        onPressed: _busy ? null : _pickAndUpload,
                      ),
                      const SizedBox(height: 12),
                      DropdownButtonFormField<String>(
                        initialValue: _language,
                        decoration: const InputDecoration(labelText: '语言'),
                        items: const [
                          DropdownMenuItem(value: '', child: Text('自动识别')),
                          DropdownMenuItem(
                            value: 'zh-CN',
                            child: Text('中文普通话'),
                          ),
                          DropdownMenuItem(value: 'en-US', child: Text('英语')),
                          DropdownMenuItem(value: 'ja-JP', child: Text('日语')),
                          DropdownMenuItem(value: 'ko-KR', child: Text('韩语')),
                        ],
                        onChanged: _busy
                            ? null
                            : (value) =>
                                  setState(() => _language = value ?? ''),
                      ),
                      const SizedBox(height: 12),
                      AmitiaButton(
                        label: '提交识别',
                        icon: Icons.transcribe_outlined,
                        onPressed: _busy ? null : _submit,
                      ),
                    ],
                  ),
                ),
                if (_taskId.isNotEmpty) ...[
                  SizedBox(height: AppSpacing.md),
                  AmitiaCard(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          '任务 $_taskId',
                          style: AppTypography.caption(context),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        const SizedBox(height: 8),
                        Text(
                          '状态：${_status.isEmpty ? '等待查询' : _status}',
                          style: AppTypography.bodySmall(context),
                        ),
                        if (_result.isNotEmpty) ...[
                          const SizedBox(height: 12),
                          SelectableText(
                            _result,
                            style: AppTypography.body(context),
                          ),
                        ],
                      ],
                    ),
                  ),
                ],
              ],
            ),
    );
  }

  Widget _statusCard(BuildContext context) {
    return AmitiaCard(
      child: Row(
        children: [
          Icon(
            _configured
                ? Icons.check_circle_outline
                : Icons.warning_amber_rounded,
            color: _configured ? context.success : context.warning,
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Text(
              _configured ? 'ASR 已配置，可提交识别任务' : '尚未启用带 API Key 的 ASR 配置',
              style: AppTypography.bodySmall(context),
            ),
          ),
        ],
      ),
    );
  }

  Widget _configCard(BuildContext context, Map<String, dynamic> config) {
    final active = config['isActive'] == 1 || config['isActive'] == true;
    final hasKey = config['hasApiKey'] == true;
    return AmitiaCard(
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  (config['name'] ?? '未命名配置').toString(),
                  style: AppTypography.bodySmall(
                    context,
                  ).copyWith(fontWeight: FontWeight.w600),
                ),
                const SizedBox(height: 3),
                Text(
                  '${config['apiType'] ?? 'unknown'} · ${hasKey ? 'API Key 已配置' : '缺少 API Key'}${active ? ' · 当前启用' : ''}',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          AmitiaPopupMenuButton<String>(
            enabled: !_busy,
            onSelected: (value) {
              switch (value) {
                case 'test':
                  _test(config);
                  break;
                case 'activate':
                  _activate(config);
                  break;
                case 'edit':
                  _showConfigSheet(config);
                  break;
                case 'delete':
                  _deleteConfig(config);
                  break;
              }
            },
            itemBuilder: (_) => [
              const PopupMenuItem(value: 'test', child: Text('测试连接')),
              if (!active)
                const PopupMenuItem(value: 'activate', child: Text('设为默认')),
              const PopupMenuItem(value: 'edit', child: Text('编辑')),
              const PopupMenuItem(value: 'delete', child: Text('删除')),
            ],
          ),
        ],
      ),
    );
  }
}
