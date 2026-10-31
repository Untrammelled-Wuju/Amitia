import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/screen_awake_service.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ScreenAwakeSettings extends ConsumerStatefulWidget {
  const ScreenAwakeSettings({super.key});
  @override
  ConsumerState<ScreenAwakeSettings> createState() =>
      _ScreenAwakeSettingsState();
}

class _ScreenAwakeSettingsState extends ConsumerState<ScreenAwakeSettings> {
  bool _enabled = false;
  bool _busy = true;
  bool _loaded = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _busy = true);
    try {
      final enabled = await ref.read(screenAwakeServiceProvider).load();
      if (!mounted) return;
      setState(() {
        _enabled = enabled;
        _loaded = true;
      });
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
if (mounted) setState(() => _loaded = false);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _update(bool enabled) async {
    setState(() => _busy = true);
    try {
      final saved = await ref
          .read(screenAwakeServiceProvider)
          .setEnabled(enabled);
      if (mounted) setState(() => _enabled = saved);
    } catch (error, stackTrace) {
  print("[original-error] $error\n$stackTrace");
if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(error.toString())));
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    children: [
      AmitiaSwitchTile(
        title: '屏幕常亮',
        subtitle: '开启后仅应用前台保持屏幕亮起，后台恢复系统息屏规则',
        value: _enabled,
        onChanged: !_loaded || _busy ? null : _update,
      ),
      if (!_loaded && !_busy)
        TextButton(onPressed: _load, child: const Text('重试读取屏幕设置')),
    ],
  );
}
