import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/voice_preview_player.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';

class VoiceCloneManager extends ConsumerStatefulWidget {
  const VoiceCloneManager({super.key});

  @override
  ConsumerState<VoiceCloneManager> createState() => _VoiceCloneManagerState();
}

class _VoiceCloneManagerState extends ConsumerState<VoiceCloneManager> {
  List<Map<String, dynamic>> _voices = const <Map<String, dynamic>>[];
  bool _loading = true;
  String _busySpeakerId = '';
  bool _creating = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final voices = await ref.read(ttsServiceProvider).listClonedVoices();
      if (!mounted) return;
      setState(() {
        _voices = voices;
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

  Future<void> _createVoice() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const <String>['wav', 'mp3', 'm4a', 'aac', 'ogg', 'pcm'],
    );
    if (picked == null || picked.files.isEmpty || picked.files.first.path == null) return;

    final nameController = TextEditingController();
    final speakerIdController = TextEditingController();
    final refTextController = TextEditingController();
    var language = 'cn';
    final request = await showDialog<Map<String, String>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('复刻新音色', style: AppTypography.cardTitle(context)),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: <Widget>[
                Text('显示名称', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                AmitiaTextField(controller: nameController, hintText: '例如：我的专属音色'),
                SizedBox(height: AppSpacing.md),
                Text('复刻槽位 / Speaker ID（可选）', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                AmitiaTextField(
                  controller: speakerIdController,
                  hintText: '例如 S_xxxxxxxx；按服务商控制台要求填写',
                ),
                const SizedBox(height: 4),
                Text('MegaTTS V1 需填写已购买槽位；V3 可留空，由 Core 生成 provider ID。', style: AppTypography.caption(context)),
                SizedBox(height: AppSpacing.md),
                Text('语言', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                DropdownButtonFormField<String>(
                  value: language,
                  isExpanded: true,
                  decoration: const InputDecoration(border: OutlineInputBorder(), isDense: true),
                  items: const <DropdownMenuItem<String>>[
                    DropdownMenuItem(value: 'cn', child: Text('中文')),
                    DropdownMenuItem(value: 'en', child: Text('英文')),
                    DropdownMenuItem(value: 'ja', child: Text('日语')),
                  ],
                  onChanged: (value) {
                    if (value != null) setDialogState(() => language = value);
                  },
                ),
                SizedBox(height: AppSpacing.md),
                Text('参考文本（可选）', style: AppTypography.label(context)),
                const SizedBox(height: 4),
                AmitiaTextField(
                  controller: refTextController,
                  hintText: '音频中说的话，可提高复刻质量',
                  maxLines: 3,
                ),
                SizedBox(height: AppSpacing.sm),
                Text('已选择：${picked.files.first.name}', style: AppTypography.caption(context)),
              ],
            ),
          ),
          actions: <Widget>[
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
            TextButton(
              onPressed: () {
                final name = nameController.text.trim();
                if (name.isEmpty) return;
                Navigator.pop(dialogContext, <String, String>{
                  'name': name,
                  'speakerId': speakerIdController.text.trim(),
                  'language': language,
                  'refText': refTextController.text.trim(),
                });
              },
              child: const Text('开始复刻'),
            ),
          ],
        ),
      ),
    );
    nameController.dispose();
    speakerIdController.dispose();
    refTextController.dispose();
    if (request == null || !mounted) return;

    setState(() => _creating = true);
    try {
      final result = await ref.read(ttsServiceProvider).cloneVoice(
            filePath: picked.files.first.path!,
            name: request['name']!,
            speakerId: request['speakerId'] ?? '',
            language: request['language'] ?? 'cn',
            refText: request['refText'] ?? '',
          );
      final speakerId = (result?['speakerId'] ?? '').toString().trim();
      if (speakerId.isEmpty) throw StateError('后端未返回 speakerId');
      await _load();
      _toast('声音复刻完成');
    } catch (e) {
      _toast('声音复刻失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _creating = false);
    }
  }

  Future<void> _preview(Map<String, dynamic> voice) async {
    final speakerId = (voice['speakerId'] ?? '').toString().trim();
    if (speakerId.isEmpty) return;
    setState(() => _busySpeakerId = speakerId);
    try {
      final result = await ref.read(ttsServiceProvider).synthesizeWithSpeaker(speakerId, '测试');
      final url = (result?['audioUrl'] ?? '').toString();
      await playBackendVoicePreview(ref, url, requestIdPrefix: 'cloned-voice-preview');
      _toast('试听已开始播放');
    } catch (e) {
      _toast('试听失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busySpeakerId = '');
    }
  }

  Future<void> _delete(Map<String, dynamic> voice) async {
    final speakerId = (voice['speakerId'] ?? '').toString().trim();
    if (speakerId.isEmpty) return;
    final name = (voice['name'] ?? speakerId).toString();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('删除复刻音色', style: AppTypography.cardTitle(context)),
        content: Text('确定删除「$name」吗？此操作会同步删除服务商侧音色。', style: AppTypography.body(context)),
        actions: <Widget>[
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    setState(() => _busySpeakerId = speakerId);
    try {
      await ref.read(ttsServiceProvider).deleteClonedVoice(speakerId);
      await _load();
      _toast('复刻音色已删除');
    } catch (e) {
      _toast('删除失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busySpeakerId = '');
    }
  }

  void _toast(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), backgroundColor: error ? context.error : null),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Container(
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
              Expanded(child: Text('声音复刻', style: AppTypography.sectionTitle(context))),
              AmitiaButton(
                label: _creating ? '复刻中...' : '复刻新音色',
                icon: Icons.add,
                height: 36,
                onPressed: _creating ? null : _createVoice,
              ),
            ],
          ),
          const SizedBox(height: 4),
          Text('音色元数据由 Core 统一保存，云端模式下可在不同设备之间保持一致。', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.md),
          if (_loading)
            const Center(child: Padding(padding: EdgeInsets.all(16), child: CircularProgressIndicator()))
          else if (_error != null)
            Row(
              children: <Widget>[
                Expanded(child: Text('加载失败：$_error', style: AppTypography.caption(context).copyWith(color: context.error))),
                TextButton(onPressed: _load, child: const Text('重试')),
              ],
            )
          else if (_voices.isEmpty)
            Text('暂无复刻音色', style: AppTypography.caption(context))
          else
            ..._voices.map((voice) {
              final speakerId = (voice['speakerId'] ?? '').toString();
              final name = (voice['name'] ?? speakerId).toString();
              final busy = _busySpeakerId == speakerId;
              return Container(
                margin: const EdgeInsets.only(bottom: 8),
                padding: const EdgeInsets.all(10),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Row(
                  children: <Widget>[
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: <Widget>[
                          Text(name, style: AppTypography.body(context)),
                          const SizedBox(height: 2),
                          Text(speakerId, style: AppTypography.caption(context), overflow: TextOverflow.ellipsis),
                        ],
                      ),
                    ),
                    const SizedBox(width: 8),
                    TextButton(onPressed: busy ? null : () => _preview(voice), child: const Text('试听')),
                    TextButton(
                      onPressed: busy ? null : () => _delete(voice),
                      child: Text('删除', style: TextStyle(color: context.error)),
                    ),
                  ],
                ),
              );
            }),
        ],
      ),
    );
  }
}
