import 'dart:io';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/models/voice.dart';
import '../../../../core/native_bridge/providers/native_bridge_relay_provider.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class CharacterVoicePage extends ConsumerStatefulWidget {
  final String characterId;
  const CharacterVoicePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterVoicePage> createState() => _CharacterVoicePageState();
}

class _CharacterVoicePageState extends ConsumerState<CharacterVoicePage> {
  bool _loading = true;
  bool _saving = false;
  String? _error;
  String _voiceMode = 'preset';
  String _voiceType = 'zh_female_vv_uranus_bigtts';
  String _customVoiceId = '';
  String _voiceConfigId = '';
  String _emotion = '';
  int _emotionScale = 4;
  double _speed = 1;
  double _pitch = 1;
  double _volume = 1;
  int _silenceDuration = 0;
  List<Map<String, dynamic>> _voices = const [];
  List<VoiceConfigDto> _voiceConfigs = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait<dynamic>([
        ref.read(characterDetailServiceProvider).character(widget.characterId),
        ref.read(ttsServiceProvider).voices(),
        ref.read(ttsServiceProvider).listConfigs(),
      ]);
      final character = values[0] as Map<String, dynamic>? ?? <String, dynamic>{};
      if (!mounted) return;
      setState(() {
        _voices = values[1] as List<Map<String, dynamic>>;
        _voiceConfigs = values[2] as List<VoiceConfigDto>;
        _voiceMode = (character['voiceMode'] ?? 'preset').toString();
        _voiceType = (character['voiceType'] ?? _voiceType).toString();
        _customVoiceId = (character['customVoiceId'] ?? '').toString();
        _voiceConfigId = (character['voiceConfigId'] ?? '').toString();
        _speed = (character['voiceSpeed'] as num?)?.toDouble() ?? 1;
        _pitch = (character['voicePitch'] as num?)?.toDouble() ?? 1;
        _volume = (character['voiceVolume'] as num?)?.toDouble() ?? 1;
        _emotion = (character['emotion'] ?? '').toString();
        _emotionScale = (character['emotionScale'] as num?)?.toInt() ?? 4;
        _silenceDuration = (character['silenceDuration'] as num?)?.toInt() ?? 0;
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

  Future<bool> _save() async {
    if (_saving) return false;
    if (_voiceMode == 'clone' && _customVoiceId.trim().isEmpty) {
      _show('请先完成声音复刻或填写克隆音色 ID', error: true);
      return false;
    }
    setState(() => _saving = true);
    try {
      await ref.read(characterDetailServiceProvider).updateCharacter(widget.characterId, {
        'voiceMode': _voiceMode,
        'voiceType': _voiceType,
        'customVoiceId': _customVoiceId.trim(),
        'voiceConfigId': _voiceConfigId,
        'voiceSpeed': _speed,
        'voicePitch': _pitch,
        'voiceVolume': _volume,
        'emotion': _emotion,
        'emotionScale': _emotionScale,
        'silenceDuration': _silenceDuration,
      });
      _show('当前角色语音设置已保存');
      return true;
    } catch (e) {
      _show('保存失败：$e', error: true);
      return false;
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _preview() async {
    File? tempFile;
    Dio? dio;
    try {
      if (!await _save()) return;
      final result = await ref.read(ttsServiceProvider).synthesizeForCharacter(
            widget.characterId,
            '你好，这是当前角色的语音试听。',
          );
      final url = (result?['audioUrl'] ?? '').toString().trim();
      if (url.isEmpty) throw StateError('后端未返回试听音频地址');
      final platform = switch (defaultTargetPlatform) {
        TargetPlatform.android => 'android',
        TargetPlatform.iOS => 'ios',
        TargetPlatform.windows => 'windows',
        _ => null,
      };
      if (kIsWeb || platform == null) {
        throw UnsupportedError('当前平台尚未接入 Flutter 角色语音本地播放桥');
      }
      dio = await _dio();
      final tempDir = await Directory.systemTemp.createTemp('amitia_voice_preview_');
      tempFile = File('${tempDir.path}/preview_audio${_previewAudioExtension(url)}');
      await dio.download(url, tempFile.path);
      if (!await tempFile.exists() || await tempFile.length() == 0) {
        throw StateError('试听音频下载失败');
      }
      final dispatcher = ref.read(nativeBridgePlatformDispatcherProvider);
      final nativeResult = await dispatcher.execute({
        'protocolVersion': 1,
        'requestId': 'character-voice-preview-${DateTime.now().microsecondsSinceEpoch}',
        'platform': platform,
        'operation': 'media.audio.play_file',
        'payload': {'path': tempFile.path},
      });
      if (!const {'success', 'ok'}.contains((nativeResult['status'] ?? '').toString())) {
        final error = nativeResult['error'];
        final message = error is Map ? (error['message'] ?? error['code'])?.toString() : null;
        throw StateError(message?.isNotEmpty == true ? message! : '系统音频播放失败');
      }
      _show('试听已开始播放');
      // MediaPlayer opens the file before start(), so the temporary file can be
      // removed after a short grace period without keeping stale previews.
      final cleanupDir = tempFile.parent;
      Future<void>.delayed(const Duration(seconds: 30), () async {
        try {
          if (await cleanupDir.exists()) await cleanupDir.delete(recursive: true);
        } catch (_) {}
      });
      tempFile = null;
    } catch (e) {
      _show('试听失败：$e', error: true);
    } finally {
      dio?.close(force: true);
      if (tempFile != null) {
        try {
          final parent = tempFile.parent;
          if (await parent.exists()) await parent.delete(recursive: true);
        } catch (_) {}
      }
    }
  }


  String _previewAudioExtension(String url) {
    final lower = Uri.tryParse(url)?.path.toLowerCase() ?? url.toLowerCase();
    for (final extension in const ['.mp3', '.wav', '.m4a', '.aac', '.ogg', '.flac']) {
      if (lower.endsWith(extension)) return extension;
    }
    return '.mp3';
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) throw StateError('后端当前不可用');
    return createAuthenticatedDio(availability.config);
  }

  Future<void> _cloneVoice() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['wav', 'mp3', 'm4a', 'aac', 'ogg', 'pcm'],
    );
    if (picked == null || picked.files.isEmpty || picked.files.first.path == null) return;
    final nameController = TextEditingController(text: '角色专属音色');
    final name = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('声音复刻'),
        content: TextField(controller: nameController, decoration: const InputDecoration(labelText: '音色名称')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, nameController.text.trim()), child: const Text('开始复刻')),
        ],
      ),
    );
    if (name == null || name.isEmpty) return;
    Dio? dio;
    try {
      final configs = await ref.read(ttsServiceProvider).listConfigs();
      VoiceConfigDto? active;
      for (final cfg in configs) {
        if (cfg.isActive == 1) {
          active = cfg;
          break;
        }
      }
      active ??= configs.isEmpty ? null : configs.first;
      final apiKey = active?.apiKey ?? '';
      if (apiKey.isEmpty && (active?.realtimeAccessToken ?? '').isEmpty) {
        throw StateError('请先在 TTS 配置中设置 API Key');
      }
      dio = await _dio();
      final file = picked.files.first;
      final form = FormData.fromMap({
        'name': name,
        'language': 'cn',
        'audio': await MultipartFile.fromFile(file.path!, filename: file.name),
      });
      final response = await dio.post(
        '/api/tts/voice-clone',
        queryParameters: {'apiKey': apiKey.isNotEmpty ? apiKey : active!.realtimeAccessToken},
        data: form,
      );
      final body = response.data;
      final data = body is Map ? body['data'] : null;
      final speakerId = data is Map ? (data['speakerId'] ?? '').toString() : '';
      if (speakerId.isEmpty) throw StateError('后端未返回 speakerId');
      setState(() {
        _customVoiceId = speakerId;
        _voiceMode = 'clone';
      });
      await _save();
      _show('声音复刻完成并已绑定到当前角色');
    } catch (e) {
      _show('声音复刻失败：$e', error: true);
    } finally {
      dio?.close(force: true);
    }
  }

  void _show(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message), backgroundColor: error ? context.error : null));
  }

  @override
  Widget build(BuildContext context) {
    final presetValues = _voices.map((item) => (item['name'] ?? '').toString()).where((value) => value.isNotEmpty).toSet().toList();
    if (!presetValues.contains(_voiceType) && _voiceType.isNotEmpty) presetValues.insert(0, _voiceType);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色语音',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load)],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const Center(child: CircularProgressIndicator())
            : _error != null
                ? Center(child: Text('加载失败：$_error'))
                : ListView(
                    padding: EdgeInsets.all(AppSpacing.pagePadding),
                    children: [
                      AmitiaSectionHeader(title: '角色专属音色'),
                      SizedBox(height: AppSpacing.sm),
                      AmitiaCard(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            SegmentedButton<String>(
                              segments: const [
                                ButtonSegment(value: 'preset', label: Text('预设音色'), icon: Icon(Icons.record_voice_over_outlined)),
                                ButtonSegment(value: 'clone', label: Text('复刻音色'), icon: Icon(Icons.graphic_eq)),
                              ],
                              selected: {_voiceMode},
                              onSelectionChanged: (value) => setState(() => _voiceMode = value.first),
                            ),
                            SizedBox(height: AppSpacing.lg),
                            if (_voiceMode == 'preset')
                              DropdownButtonFormField<String>(
                                value: presetValues.contains(_voiceType) ? _voiceType : null,
                                decoration: const InputDecoration(labelText: '预设音色'),
                                items: presetValues.map((value) {
                                  final item = _voices.cast<Map<String, dynamic>?>().firstWhere(
                                    (row) => (row?['name'] ?? '').toString() == value,
                                    orElse: () => null,
                                  );
                                  return DropdownMenuItem(value: value, child: Text((item?['label'] ?? value).toString(), overflow: TextOverflow.ellipsis));
                                }).toList(),
                                onChanged: (value) => setState(() => _voiceType = value ?? _voiceType),
                              )
                            else ...[
                              Text('当前 speakerId：${_customVoiceId.isEmpty ? '尚未复刻' : _customVoiceId}', style: AppTypography.bodySmall(context)),
                              SizedBox(height: AppSpacing.sm),
                              AmitiaButton(label: '上传语音样本并复刻', icon: Icons.upload_file, isSecondary: true, onPressed: _cloneVoice),
                            ],
                          ],
                        ),
                      ),
                      SizedBox(height: AppSpacing.sectionGap),
                      AmitiaSectionHeader(title: '角色语音参数'),
                      SizedBox(height: AppSpacing.sm),
                      AmitiaCard(
                        child: Column(
                          children: [
                            DropdownButtonFormField<String>(
                              value: _voiceConfigs.any((item) => item.id == _voiceConfigId) ? _voiceConfigId : '',
                              decoration: const InputDecoration(labelText: 'TTS 配置'),
                              items: [
                                const DropdownMenuItem(value: '', child: Text('跟随当前全局配置')),
                                ..._voiceConfigs.map((item) => DropdownMenuItem(
                                      value: item.id,
                                      child: Text(item.name.isEmpty ? '${item.provider} · ${item.id}' : item.name),
                                    )),
                              ],
                              onChanged: (value) => setState(() => _voiceConfigId = value ?? ''),
                            ),
                            SizedBox(height: AppSpacing.sm),
                            _slider('语速', _speed, 0.5, 2, (v) => setState(() => _speed = v)),
                            _slider('音调', _pitch, 0.5, 2, (v) => setState(() => _pitch = v)),
                            _slider('音量', _volume, 0.2, 2, (v) => setState(() => _volume = v)),
                            DropdownButtonFormField<String>(
                              value: const ['', 'happy', 'sad', 'angry', 'fearful', 'surprised', 'neutral'].contains(_emotion) ? _emotion : '',
                              decoration: const InputDecoration(labelText: '情绪'),
                              items: const ['', 'happy', 'sad', 'angry', 'fearful', 'surprised', 'neutral'].map((value) => DropdownMenuItem(value: value, child: Text(value.isEmpty ? '无' : value))).toList(),
                              onChanged: (value) => setState(() => _emotion = value ?? ''),
                            ),
                            _slider('情绪强度', _emotionScale.toDouble(), 1, 5, (v) => setState(() => _emotionScale = v.round()), divisions: 4),
                            _slider('句尾静音(ms)', _silenceDuration.toDouble(), 0, 5000, (v) => setState(() => _silenceDuration = v.round()), divisions: 50),
                          ],
                        ),
                      ),
                      SizedBox(height: AppSpacing.sectionGap),
                      Row(children: [
                        Expanded(child: AmitiaButton(label: '试听', icon: Icons.volume_up_outlined, isSecondary: true, onPressed: _preview)),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(child: AmitiaButton(label: _saving ? '保存中...' : '保存', icon: Icons.save_outlined, onPressed: _saving ? null : _save)),
                      ]),
                      SizedBox(height: AppSpacing.xxl),
                    ],
                  ),
      ),
    );
  }

  Widget _slider(String label, double value, double min, double max, ValueChanged<double> onChanged, {int? divisions}) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('$label：${value.toStringAsFixed(1)}', style: AppTypography.label(context)),
        Slider(value: value.clamp(min, max).toDouble(), min: min, max: max, divisions: divisions ?? 30, onChanged: onChanged),
      ],
    );
  }
}
