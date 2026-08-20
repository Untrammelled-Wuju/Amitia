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
      final configs = await ref.read(asrServiceProvider).configs();
      if (!mounted) return;
      setState(() {
        _configs = configs;
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
                      AmitiaCard(child: Text('暂无 ASR 配置，请先在模型配置中添加语音识别 Provider。', style: AppTypography.caption(context)))
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
          TextButton(onPressed: _busy ? null : () => _test(config), child: const Text('测试')),
          if (!active) TextButton(onPressed: _busy ? null : () => _activate(config), child: const Text('启用')),
        ],
      ),
    );
  }
}
