import 'dart:async';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/services/providers.dart';
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
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final service = ref.read(asrServiceProvider);
      final configs = await service.configs();
      final providers = await service.providers();
      if (!mounted) return;
      setState(() {
        _configs = configs;
        _providers = providers;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) {
      throw StateError('后端当前不可用');
    }
    return createAuthenticatedDio(availability.config);
  }

  bool get _configured => _configs.any((item) {
        final active = item['isActive'];
        final isActive = active == 1 || active == true;
        return isActive && item['hasApiKey'] == true;
      });

  Future<void> _activate(Map<String, dynamic> config) async {
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
    final nameCtrl = TextEditingController(text: (existing?['name'] ?? '').toString());
    final typeCtrl = TextEditingController(text: (existing?['apiType'] ?? '').toString());
    final keyCtrl = TextEditingController();
    final baseCtrl = TextEditingController(text: (existing?['baseUrl'] ?? '').toString());
    final resourceCtrl = TextEditingController(text: (existing?['resourceId'] ?? '').toString());
    bool active = existing == null ? _configs.isEmpty : (existing['isActive'] == 1 || existing['isActive'] == true);

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => SafeArea(
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
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: [
                  Text(existing == null ? '新建 ASR 配置' : '编辑 ASR 配置', style: AppTypography.sectionTitle(context)),
                  SizedBox(height: AppSpacing.lg),
                  TextField(controller: nameCtrl, decoration: const InputDecoration(labelText: '配置名称', border: OutlineInputBorder())),
                  SizedBox(height: AppSpacing.md),
                  if (_providers.isEmpty)
                    TextField(controller: typeCtrl, decoration: const InputDecoration(labelText: 'Provider / API Type', border: OutlineInputBorder()))
                  else
                    DropdownButtonFormField<String>(
                      value: _providers.any((p) => (p['id'] ?? '').toString() == typeCtrl.text) ? typeCtrl.text : null,
                      isExpanded: true,
                      decoration: const InputDecoration(labelText: 'Provider', border: OutlineInputBorder()),
                      items: _providers
                          .map((provider) => DropdownMenuItem<String>(
                                value: (provider['id'] ?? '').toString(),
                                child: Text((provider['name'] ?? provider['id'] ?? '').toString()),
                              ))
                          .toList(growable: false),
                      onChanged: (value) {
                        if (value == null) return;
                        final provider = _providers.firstWhere((item) => (item['id'] ?? '').toString() == value);
                        setSheetState(() {
                          typeCtrl.text = value;
                          if (baseCtrl.text.trim().isEmpty) baseCtrl.text = (provider['defaultBaseUrl'] ?? '').toString();
                          if (resourceCtrl.text.trim().isEmpty) resourceCtrl.text = (provider['defaultModel'] ?? '').toString();
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
                  TextField(controller: baseCtrl, decoration: const InputDecoration(labelText: 'Base URL', border: OutlineInputBorder())),
                  SizedBox(height: AppSpacing.md),
                  TextField(controller: resourceCtrl, decoration: const InputDecoration(labelText: 'Resource / Model ID', border: OutlineInputBorder())),
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
                      if (nameCtrl.text.trim().isEmpty || typeCtrl.text.trim().isEmpty) {
                        _show('名称和 Provider 不能为空', error: true);
                        return;
                      }
                      final data = <String, dynamic>{
                        'name': nameCtrl.text.trim(),
                        'apiType': typeCtrl.text.trim(),
                        'baseUrl': baseCtrl.text.trim(),
                        'resourceId': resourceCtrl.text.trim(),
                        'isActive': active ? 1 : 0,
                        if (keyCtrl.text.trim().isNotEmpty) 'apiKey': keyCtrl.text.trim(),
                      };
                      Navigator.of(sheetContext).pop();
                      await _saveConfig(existing, data);
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

  Future<void> _saveConfig(Map<String, dynamic>? existing, Map<String, dynamic> data) async {
    setState(() => _busy = true);
    try {
      final service = ref.read(asrServiceProvider);
      if (existing == null) {
        await service.createConfig(data);
      } else {
        final id = (existing['id'] ?? '').toString();
        if (id.isEmpty) throw StateError('ASR 配置 ID 无效');
        await service.updateConfig(id, data);
      }
      await _loadConfigs();
      _show(existing == null ? 'ASR 配置已创建' : 'ASR 配置已更新');
    } catch (e) {
      _show('保存失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _deleteConfig(Map<String, dynamic> config) async {
    final id = (config['id'] ?? '').toString();
    if (id.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除 ASR 配置'),
        content: Text('确定删除「${config['name'] ?? id}」吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('删除', style: TextStyle(color: context.error))),
        ],
      ),
    );
    if (confirmed != true) return;
    setState(() => _busy = true);
    try {
      await ref.read(asrServiceProvider).deleteConfig(id);
      await _loadConfigs();
      _show('ASR 配置已删除');
    } catch (e) {
      _show('删除失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _pickAndUpload() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['mp3', 'wav', 'ogg', 'm4a', 'aac', 'pcm'],
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) {
      _show('无法读取所选音频文件', error: true);
      return;
    }
    setState(() => _busy = true);
    final dio = await _dio();
    try {
      final form = FormData.fromMap({
        'audio': await MultipartFile.fromFile(file.path!, filename: file.name),
      });
      final response = await dio.post('/api/asr/upload', data: form);
      final body = response.data;
      final payload = body is Map ? body['data'] : null;
      final url = payload is Map ? (payload['url'] ?? '').toString() : '';
      if (url.isEmpty) throw StateError('后端未返回音频地址');
      if (!mounted) return;
      setState(() => _audioUrl = url);
      _show('音频已上传');
    } catch (e) {
      _show('上传失败：$e', error: true);
    } finally {
      dio.close(force: true);
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _submit() async {
    if (!_configured) {
      _show('请先配置并启用 ASR API Key', error: true);
      return;
    }
    if (_audioUrl.trim().isEmpty) {
      _show('请先选择音频文件', error: true);
      return;
    }
    setState(() => _busy = true);
    final dio = await _dio();
    try {
      final form = FormData.fromMap({
        'audioUrl': _audioUrl.trim(),
        if (_language.isNotEmpty) 'language': _language,
      });
      final response = await dio.post('/api/asr/submit', data: form);
      final body = response.data;
      final payload = body is Map ? body['data'] : null;
      final taskId = payload is Map ? (payload['taskId'] ?? '').toString() : '';
      if (taskId.isEmpty) throw StateError('后端未返回任务 ID');
      if (!mounted) return;
      setState(() {
        _taskId = taskId;
        _status = 'processing';
        _result = '';
      });
      _startPolling();
    } catch (e) {
      _show('提交失败：$e', error: true);
    } finally {
      dio.close(force: true);
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
    try {
      final response = await ref.read(asrServiceProvider).queryResult(_taskId);
      if (!mounted || response == null) return;
      final status = (response['status'] ?? '').toString();
      final result = (response['result'] ?? '').toString();
      setState(() {
        _status = status.isEmpty ? _status : status;
        if (result.isNotEmpty) _result = result;
      });
      if (status == 'success' || status == 'completed' || status == 'failed' || status == 'error') {
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
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '语音识别',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(onPressed: _busy ? null : () => _showConfigSheet(), icon: const Icon(Icons.add), tooltip: '新建 ASR 配置'),
          IconButton(onPressed: _busy ? null : _loadConfigs, icon: const Icon(Icons.refresh), tooltip: '刷新'),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: AmitiaButton(label: '重新加载', onPressed: _loadConfigs))
              : ListView(
                  padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.xxxl),
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
                            Text('暂无 ASR 配置。', style: AppTypography.caption(context)),
                            const SizedBox(height: 10),
                            AmitiaButton(label: '新建 ASR 配置', icon: Icons.add, isSecondary: true, onPressed: _busy ? null : () => _showConfigSheet()),
                          ],
                        ),
                      )
                    else
                      ..._configs.map((config) => Padding(
                            padding: const EdgeInsets.only(bottom: 8),
                            child: _configCard(context, config),
                          )),
                    SizedBox(height: AppSpacing.sectionGap),
                    Text('音频识别', style: AppTypography.sectionTitle(context)),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaCard(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          Text(_audioUrl.isEmpty ? '尚未选择音频' : _audioUrl, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
                          const SizedBox(height: 12),
                          AmitiaButton(label: '选择并上传音频', icon: Icons.audio_file_outlined, isSecondary: true, onPressed: _busy ? null : _pickAndUpload),
                          const SizedBox(height: 12),
                          DropdownButtonFormField<String>(
                            initialValue: _language,
                            decoration: const InputDecoration(labelText: '语言'),
                            items: const [
                              DropdownMenuItem(value: '', child: Text('自动识别')),
                              DropdownMenuItem(value: 'zh-CN', child: Text('中文普通话')),
                              DropdownMenuItem(value: 'en-US', child: Text('英语')),
                              DropdownMenuItem(value: 'ja-JP', child: Text('日语')),
                              DropdownMenuItem(value: 'ko-KR', child: Text('韩语')),
                            ],
                            onChanged: _busy ? null : (value) => setState(() => _language = value ?? ''),
                          ),
                          const SizedBox(height: 12),
                          AmitiaButton(label: '提交识别', icon: Icons.transcribe_outlined, onPressed: _busy ? null : _submit),
                        ],
                      ),
                    ),
                    if (_taskId.isNotEmpty) ...[
                      SizedBox(height: AppSpacing.md),
                      AmitiaCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text('任务 $_taskId', style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                            const SizedBox(height: 8),
                            Text('状态：${_status.isEmpty ? '等待查询' : _status}', style: AppTypography.bodySmall(context)),
                            if (_result.isNotEmpty) ...[
                              const SizedBox(height: 12),
                              SelectableText(_result, style: AppTypography.body(context)),
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
          Icon(_configured ? Icons.check_circle_outline : Icons.warning_amber_rounded, color: _configured ? context.success : context.warning),
          const SizedBox(width: 10),
          Expanded(child: Text(_configured ? 'ASR 已配置，可提交识别任务' : '尚未启用带 API Key 的 ASR 配置', style: AppTypography.bodySmall(context))),
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
                Text((config['name'] ?? '未命名配置').toString(), style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
                const SizedBox(height: 3),
                Text('${config['apiType'] ?? 'unknown'} · ${hasKey ? 'API Key 已配置' : '缺少 API Key'}${active ? ' · 当前启用' : ''}', style: AppTypography.caption(context)),
              ],
            ),
          ),
          PopupMenuButton<String>(
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
              if (!active) const PopupMenuItem(value: 'activate', child: Text('设为默认')),
              const PopupMenuItem(value: 'edit', child: Text('编辑')),
              const PopupMenuItem(value: 'delete', child: Text('删除')),
            ],
          ),
        ],
      ),
    );
  }
}
