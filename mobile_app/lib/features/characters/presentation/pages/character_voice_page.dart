import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/voice.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/voice_preview_player.dart';
import '../../../../core/services/owned_speech_player.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/backend_transport/core_configuration_intent.dart';
import '../../../../core/services/core_configuration_guard.dart';
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
  String _roleAuthority = '';
  CoreConfigurationIntent? _configurationIntent;
  int _loadEpoch = 0;
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
  List<Map<String, dynamic>> _clonedVoices = const [];
  List<VoiceConfigDto> _voiceConfigs = const [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final epoch = ++_loadEpoch;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final intent = await coreConfigurationGuardFor(ref).capture();
      if (!mounted || epoch != _loadEpoch) return;
      _configurationIntent = intent;
      final values = await Future.wait<dynamic>([
        ref.read(characterDetailServiceProvider).character(widget.characterId),
        ref.read(ttsServiceProvider).voices(),
        ref.read(ttsServiceProvider).listConfigSummaries(),
        ref.read(ttsServiceProvider).listClonedVoices(),
      ]);
      final character =
          values[0] as Map<String, dynamic>? ?? <String, dynamic>{};
      if (!mounted || epoch != _loadEpoch) return;
      setState(() {
        _roleAuthority = (character['roleAuthority'] ?? '').toString();
        _voices = values[1] as List<Map<String, dynamic>>;
        _voiceConfigs = values[2] as List<VoiceConfigDto>;
        _clonedVoices = values[3] as List<Map<String, dynamic>>;
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
      if (!mounted || epoch != _loadEpoch) return;
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
      await ref
          .read(characterDetailServiceProvider)
          .updateCharacter(widget.characterId, {
            'voiceMode': _voiceMode,
            'voiceType': _voiceType,
            'customVoiceId': _customVoiceId.trim(),
            if (ref.read(mobileDeploymentConfigProvider).mode !=
                MobileDeploymentMode.cloud)
              'voiceConfigId': _voiceConfigId,
            'voiceSpeed': _speed,
            'voicePitch': _pitch,
            'voiceVolume': _volume,
            'emotion': _emotion,
            'emotionScale': _emotionScale,
            'silenceDuration': _silenceDuration,
          }, roleAuthority: _roleAuthority);
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
    try {
      if (ref.read(mobileDeploymentConfigProvider).mode ==
          MobileDeploymentMode.cloud) {
        final audio = await ref
            .read(deviceOwnedSpeechServiceProvider)
            .synthesize(widget.characterId, '你好，这是当前角色已保存设置的语音试听。');
        if (!mounted) return;
        await ref.read(ownedSpeechPlayerProvider).play(audio);
        _show('正在试听当前角色已保存的语音设置');
        return;
      }
      if (!await _save()) return;
      final result = await ref
          .read(ttsServiceProvider)
          .synthesizeForCharacter(widget.characterId, '你好，这是当前角色的语音试听。');
      final url = (result?['audioUrl'] ?? '').toString().trim();
      await playBackendVoicePreview(
        ref,
        url,
        requestIdPrefix: 'character-voice-preview',
      );
      _show('试听已开始播放');
    } catch (e) {
      _show('试听失败：$e', error: true);
    }
  }

  Future<void> _cloneVoice() async {
    final intent = _configurationIntent;
    try {
      if (intent == null) throw StateError('音色配置归属无法确认，请重新加载');
      await coreConfigurationGuardFor(ref).validate(intent);
    } catch (error) {
      _show('声音复刻不可用：$error', error: true);
      return;
    }
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['wav', 'mp3', 'm4a', 'aac', 'ogg', 'pcm'],
    );
    if (picked == null ||
        picked.files.isEmpty ||
        picked.files.first.path == null)
      return;
    final nameController = TextEditingController(text: '角色专属音色');
    final speakerIdController = TextEditingController();
    final request = await showDialog<Map<String, String>>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('声音复刻'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(
              controller: nameController,
              decoration: const InputDecoration(labelText: '显示名称'),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: speakerIdController,
              decoration: const InputDecoration(
                labelText: '复刻槽位 / Speaker ID（V1 必填，V3 可选）',
                hintText: '按服务商控制台要求填写',
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () {
              final name = nameController.text.trim();
              if (name.isEmpty) return;
              Navigator.pop(dialogContext, <String, String>{
                'name': name,
                'speakerId': speakerIdController.text.trim(),
              });
            },
            child: const Text('开始复刻'),
          ),
        ],
      ),
    );
    nameController.dispose();
    speakerIdController.dispose();
    if (request == null || (request['name'] ?? '').isEmpty) return;
    try {
      await coreConfigurationGuardFor(ref).validate(intent);
      final file = picked.files.first;
      final data = await intent.run(
        () => ref
            .read(ttsServiceProvider)
            .cloneVoice(
              filePath: file.path!,
              name: request['name']!,
              speakerId: request['speakerId'] ?? '',
              voiceConfigId: _voiceConfigId,
              language: 'cn',
            ),
      );
      final speakerId = (data?['speakerId'] ?? '').toString().trim();
      if (speakerId.isEmpty) throw StateError('后端未返回 speakerId');
      final clones = await intent.run(
        () => ref.read(ttsServiceProvider).listClonedVoices(),
      );
      if (!mounted) return;
      setState(() {
        _customVoiceId = speakerId;
        _voiceMode = 'clone';
        _clonedVoices = clones;
      });
      if (!await _save()) throw StateError('声音已复刻，但角色尚未确认保存');
      _show('声音复刻完成并已绑定到当前角色');
    } catch (e) {
      _show('声音复刻失败：$e', error: true);
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

  Set<String> get _cloneSpeakerIds => _clonedVoices
      .map((voice) => (voice['speakerId'] ?? '').toString())
      .where((value) => value.isNotEmpty)
      .toSet();

  @override
  Widget build(BuildContext context) {
    final bound =
        ref.watch(mobileDeploymentConfigProvider).mode ==
        MobileDeploymentMode.cloud;
    final presetValues = _voices
        .map((item) => (item['name'] ?? '').toString())
        .where((value) => value.isNotEmpty)
        .toSet()
        .toList();
    if (!presetValues.contains(_voiceType) && _voiceType.isNotEmpty)
      presetValues.insert(0, _voiceType);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色语音',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _load,
          ),
        ],
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
                            ButtonSegment(
                              value: 'preset',
                              label: Text('预设音色'),
                              icon: Icon(Icons.record_voice_over_outlined),
                            ),
                            ButtonSegment(
                              value: 'clone',
                              label: Text('复刻音色'),
                              icon: Icon(Icons.graphic_eq),
                            ),
                          ],
                          selected: {_voiceMode},
                          onSelectionChanged: (value) =>
                              setState(() => _voiceMode = value.first),
                        ),
                        SizedBox(height: AppSpacing.lg),
                        if (_voiceMode == 'preset')
                          DropdownButtonFormField<String>(
                            value: presetValues.contains(_voiceType)
                                ? _voiceType
                                : null,
                            decoration: const InputDecoration(
                              labelText: '预设音色',
                            ),
                            items: presetValues.map((value) {
                              final item = _voices
                                  .cast<Map<String, dynamic>?>()
                                  .firstWhere(
                                    (row) =>
                                        (row?['name'] ?? '').toString() ==
                                        value,
                                    orElse: () => null,
                                  );
                              return DropdownMenuItem(
                                value: value,
                                child: Text(
                                  (item?['label'] ?? value).toString(),
                                  overflow: TextOverflow.ellipsis,
                                ),
                              );
                            }).toList(),
                            onChanged: (value) => setState(
                              () => _voiceType = value ?? _voiceType,
                            ),
                          )
                        else ...[
                          DropdownButtonFormField<String>(
                            value: _cloneSpeakerIds.contains(_customVoiceId)
                                ? _customVoiceId
                                : '',
                            isExpanded: true,
                            decoration: const InputDecoration(
                              labelText: '已复刻音色',
                            ),
                            items: <DropdownMenuItem<String>>[
                              const DropdownMenuItem(
                                value: '',
                                child: Text('请选择复刻音色'),
                              ),
                              ..._clonedVoices
                                  .map((voice) {
                                    final speakerId = (voice['speakerId'] ?? '')
                                        .toString();
                                    final name = (voice['name'] ?? speakerId)
                                        .toString();
                                    return DropdownMenuItem<String>(
                                      value: speakerId,
                                      child: Text(
                                        '$name · $speakerId',
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                    );
                                  })
                                  .where(
                                    (item) => item.value?.isNotEmpty == true,
                                  ),
                            ],
                            onChanged: (value) =>
                                setState(() => _customVoiceId = value ?? ''),
                          ),
                          if (_customVoiceId.isNotEmpty &&
                              !_cloneSpeakerIds.contains(_customVoiceId)) ...[
                            const SizedBox(height: 4),
                            Text(
                              '当前角色绑定了未登记到 Core 列表的旧 speakerId：$_customVoiceId',
                              style: AppTypography.caption(context),
                            ),
                          ],
                          SizedBox(height: AppSpacing.sm),
                          AmitiaButton(
                            label: '上传语音样本并复刻',
                            icon: Icons.upload_file,
                            isSecondary: true,
                            onPressed:
                                _configurationIntent?.canConfigure == true
                                ? _cloneVoice
                                : null,
                          ),
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
                        if (bound)
                          const Text('语音模型和服务配置由当前 Core 提供，请在 Core 控制页面修改。')
                        else
                          DropdownButtonFormField<String>(
                            value:
                                _voiceConfigs.any(
                                  (item) => item.id == _voiceConfigId,
                                )
                                ? _voiceConfigId
                                : '',
                            decoration: const InputDecoration(
                              labelText: 'TTS 配置',
                            ),
                            items: [
                              const DropdownMenuItem(
                                value: '',
                                child: Text('跟随当前全局配置'),
                              ),
                              ..._voiceConfigs.map(
                                (item) => DropdownMenuItem(
                                  value: item.id,
                                  child: Text(
                                    item.name.isEmpty
                                        ? '${item.provider} · ${item.id}'
                                        : item.name,
                                  ),
                                ),
                              ),
                            ],
                            onChanged: (value) =>
                                setState(() => _voiceConfigId = value ?? ''),
                          ),
                        SizedBox(height: AppSpacing.sm),
                        _slider(
                          '语速',
                          _speed,
                          0.5,
                          2,
                          (v) => setState(() => _speed = v),
                        ),
                        _slider(
                          '音调',
                          _pitch,
                          0.5,
                          2,
                          (v) => setState(() => _pitch = v),
                        ),
                        _slider(
                          '音量',
                          _volume,
                          0.2,
                          2,
                          (v) => setState(() => _volume = v),
                        ),
                        DropdownButtonFormField<String>(
                          value:
                              const [
                                '',
                                'happy',
                                'sad',
                                'angry',
                                'fearful',
                                'surprised',
                                'neutral',
                              ].contains(_emotion)
                              ? _emotion
                              : '',
                          decoration: const InputDecoration(labelText: '情绪'),
                          items:
                              const [
                                    '',
                                    'happy',
                                    'sad',
                                    'angry',
                                    'fearful',
                                    'surprised',
                                    'neutral',
                                  ]
                                  .map(
                                    (value) => DropdownMenuItem(
                                      value: value,
                                      child: Text(value.isEmpty ? '无' : value),
                                    ),
                                  )
                                  .toList(),
                          onChanged: (value) =>
                              setState(() => _emotion = value ?? ''),
                        ),
                        _slider(
                          '情绪强度',
                          _emotionScale.toDouble(),
                          1,
                          5,
                          (v) => setState(() => _emotionScale = v.round()),
                          divisions: 4,
                        ),
                        _slider(
                          '句尾静音(ms)',
                          _silenceDuration.toDouble(),
                          0,
                          5000,
                          (v) => setState(() => _silenceDuration = v.round()),
                          divisions: 50,
                        ),
                      ],
                    ),
                  ),
                  SizedBox(height: AppSpacing.sectionGap),
                  Row(
                    children: [
                      Expanded(
                        child: AmitiaButton(
                          label: '试听',
                          icon: Icons.volume_up_outlined,
                          isSecondary: true,
                          onPressed: _preview,
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: AmitiaButton(
                          label: _saving ? '保存中...' : '保存',
                          icon: Icons.save_outlined,
                          onPressed: _saving ? null : _save,
                        ),
                      ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.xxl),
                ],
              ),
      ),
    );
  }

  Widget _slider(
    String label,
    double value,
    double min,
    double max,
    ValueChanged<double> onChanged, {
    int? divisions,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          '$label：${value.toStringAsFixed(1)}',
          style: AppTypography.label(context),
        ),
        Slider(
          value: value.clamp(min, max).toDouble(),
          min: min,
          max: max,
          divisions: divisions ?? 30,
          onChanged: onChanged,
        ),
      ],
    );
  }
}
