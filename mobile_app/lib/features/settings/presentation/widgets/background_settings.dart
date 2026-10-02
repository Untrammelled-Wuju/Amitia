import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../core/settings/background_preferences.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/background_media.dart';

class BackgroundSettings extends ConsumerStatefulWidget {
  const BackgroundSettings({super.key});
  @override
  ConsumerState<BackgroundSettings> createState() => _BackgroundSettingsState();
}

class _BackgroundSettingsState extends ConsumerState<BackgroundSettings> {
  bool _busy = true;
  String? _error;
  @override
  void initState() {
    super.initState();
    _run(() => ref.read(backgroundPreferencesProvider.notifier).init());
  }

  Future<void> _run(Future<void> Function() action) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await action();
    } catch (error) {
      if (mounted) {
        setState(
          () => _error = error is FormatException
              ? error.message
              : '无法保存或读取背景，请重试',
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _pick() => _run(() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: ['png', 'jpg', 'jpeg', 'webp', 'gif', 'mp4', 'webm'],
      withData: false,
    );
    if (result == null || !mounted) return;
    final file = result.files.single;
    if (file.path == null) throw StateError('无法读取文件');
    await ref
        .read(backgroundPreferencesProvider.notifier)
        .importFile(file.path!, file.name);
  });
  Future<void> _adjust(Future<void> Function() action) async {
    try {
      await action();
    } catch (_) {
      if (mounted) setState(() => _error = '保存背景设置失败，请重试');
    }
  }

  @override
  Widget build(BuildContext context) {
    final preferences = ref.watch(backgroundPreferencesProvider);
    final notifier = ref.read(backgroundPreferencesProvider.notifier);
    final hasMedia = preferences.path.isNotEmpty;
    return Container(
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: context.borderPrimary),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          AmitiaSwitchTile(
            title: '自定义背景',
            subtitle: '图片或静音循环视频，保存在当前设备',
            value: preferences.enabled,
            onChanged: _busy || !hasMedia
                ? null
                : (value) => _run(() => notifier.update(enabled: value)),
          ),
          Text(preferences.name.isEmpty ? '尚未选择背景' : preferences.name),
          const SizedBox(height: 8),
          Wrap(
            spacing: 8,
            children: [
              OutlinedButton(
                onPressed: _busy ? null : _pick,
                child: Text(hasMedia ? '更换背景' : '选择背景'),
              ),
              if (hasMedia)
                TextButton(
                  onPressed: _busy ? null : () => _run(notifier.clear),
                  child: const Text('移除'),
                ),
            ],
          ),
          const Text(
            '图片最大 20 MB，视频最大 150 MB。建议使用 MP4；编码支持由设备决定。视频在后台或减少动画时暂停。',
            style: TextStyle(fontSize: 12),
          ),
          if (_error != null)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 8),
              child: Text(_error!, style: TextStyle(color: context.error)),
            ),
          if (_error != null)
            TextButton(
              onPressed: _busy ? null : () => _run(notifier.init),
              child: const Text('重试读取'),
            ),
          const SizedBox(height: 16),
          Text('背景不透明度 ${(preferences.opacity * 100).round()}%'),
          Slider(
            value: preferences.opacity,
            min: 0,
            max: 1,
            divisions: 100,
            label: '${(preferences.opacity * 100).round()}%',
            onChanged: _busy || !hasMedia
                ? null
                : (value) => _adjust(() => notifier.update(opacity: value)),
          ),
          AmitiaSwitchTile(
            title: '背景模糊',
            value: preferences.blurEnabled,
            onChanged: _busy || !hasMedia
                ? null
                : (value) => _run(() => notifier.update(blurEnabled: value)),
          ),
          Text('模糊半径 ${preferences.blurRadius.round()} px'),
          Slider(
            value: preferences.blurRadius,
            min: 0,
            max: 30,
            divisions: 30,
            label: '${preferences.blurRadius.round()} px',
            onChanged: _busy || !hasMedia || !preferences.blurEnabled
                ? null
                : (value) => _adjust(() => notifier.update(blurRadius: value)),
          ),
          if (hasMedia)
            SizedBox(
              height: 210,
              child: ClipRRect(
                borderRadius: BorderRadius.circular(12),
                child: BackgroundMedia(
                  preferences: preferences,
                  baseColor: context.backgroundPrimary,
                  animate: false,
                  preview: true,
                  child: Padding(
                    padding: const EdgeInsets.all(16),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        const Text(
                          '背景效果预览',
                          style: TextStyle(fontWeight: FontWeight.w600),
                        ),
                        const SizedBox(height: 12),
                        const Text('正文内容保持清晰可读'),
                        const SizedBox(height: 12),
                        Container(
                          padding: const EdgeInsets.all(12),
                          decoration: BoxDecoration(
                            color: context.surfacePrimary,
                            borderRadius: BorderRadius.circular(12),
                          ),
                          child: const Text('内容卡片与输入区保留原有底色'),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}
