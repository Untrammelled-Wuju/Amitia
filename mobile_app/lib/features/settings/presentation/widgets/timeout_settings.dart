import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';

class TimeoutSettings extends ConsumerStatefulWidget {
  const TimeoutSettings({super.key});

  @override
  ConsumerState<TimeoutSettings> createState() => _TimeoutSettingsState();
}

class _TimeoutSettingsState extends ConsumerState<TimeoutSettings> {
  bool _disabled = false;
  int _seconds = 180;
  bool _loading = true;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  String get _duration {
    final minutes = _seconds ~/ 60;
    final seconds = _seconds % 60;
    return minutes > 0
        ? '$minutes 分钟${seconds > 0 ? ' $seconds 秒' : ''}'
        : '$seconds 秒';
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final value = await ref
          .read(backendServiceProvider)
          .get<Map<String, dynamic>>('/api/runtime/timeout/config');
      if (value == null) throw StateError('无法加载超时设置');
      if (!mounted) return;
      setState(() {
        _disabled = value['disabled'] == true;
        _seconds = (value['seconds'] as num).toInt();
      });
    } catch (_) {
      if (mounted) setState(() => _error = '无法加载超时设置，请重试');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      await ref
          .read(backendServiceProvider)
          .put<Map<String, dynamic>>(
            '/api/runtime/timeout/config',
            data: {'disabled': _disabled, 'seconds': _seconds},
          );
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('超时设置已保存，对新调用生效')));
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('保存失败，请重试')));
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const Padding(
        padding: EdgeInsets.all(24),
        child: Center(child: CircularProgressIndicator()),
      );
    }
    if (_error != null) {
      return Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Text(_error!),
            TextButton(onPressed: _load, child: const Text('重新加载')),
          ],
        ),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        AmitiaSwitchTile(
          title: '停用超时',
          subtitle: '开启后，功能调用不再因执行时间过长而中止',
          value: _disabled,
          onChanged: _saving
              ? null
              : (value) => setState(() => _disabled = value),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('超时时间：$_duration'),
              Slider(
                value: _seconds.toDouble(),
                min: 30,
                max: 1800,
                divisions: 59,
                label: _duration,
                onChanged: _disabled || _saving
                    ? null
                    : (value) => setState(() => _seconds = value.round()),
              ),
              const Text('统一控制模型、工具与媒体处理等功能的执行时限。保存后对新调用生效；仍可主动取消。'),
              const SizedBox(height: 12),
              FilledButton(
                onPressed: _saving ? null : _save,
                child: Text(_saving ? '保存中…' : '保存'),
              ),
            ],
          ),
        ),
      ],
    );
  }
}
