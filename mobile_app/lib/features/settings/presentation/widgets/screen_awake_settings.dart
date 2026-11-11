import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/screen_awake_service.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/errors/original_error.dart';

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
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final enabled = await ref.read(screenAwakeServiceProvider).load();
      if (!mounted) return;
      setState(() {
        _enabled = enabled;
        _loaded = true;
      });
    } catch (error, stackTrace) {
      if (mounted) {
        setState(() {
          _loaded = false;
          _error = '$error\n${originalErrorText(error)}\n$stackTrace';
        });
      }
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
      if (mounted) {
        _showUnavailable(
          '屏幕常亮设置保存失败，请重试。\n\n原始错误：\n$error\n${originalErrorText(error)}\n$stackTrace',
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _showUnavailable(String message) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('屏幕常亮设置'),
        content: SingleChildScrollView(child: SelectableText(message)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('知道了'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) => Column(
    children: [
      GestureDetector(
        behavior: HitTestBehavior.opaque,
        onTap: !_loaded || _busy
            ? () => _showUnavailable(
                _busy
                    ? '正在处理，请稍后重试。'
                    : '无法读取屏幕设置，请重试。\n\n原始错误：\n${_error ?? '未返回错误详情'}',
              )
            : null,
        child: AmitiaSwitchTile(
          title: '屏幕常亮',
          subtitle: '开启后仅应用前台保持屏幕亮起，后台恢复系统息屏规则',
          value: _enabled,
          onChanged: !_loaded || _busy ? null : _update,
        ),
      ),
      if (!_loaded && !_busy)
        TextButton(onPressed: _load, child: const Text('重试读取屏幕设置')),
    ],
  );
}
