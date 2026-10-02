import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/background_keep_alive_service.dart';
import '../../../../core/widgets/amitia_misc.dart';

class BackgroundKeepAliveSettings extends ConsumerStatefulWidget {
  const BackgroundKeepAliveSettings({super.key});
  @override
  ConsumerState<BackgroundKeepAliveSettings> createState() =>
      _BackgroundKeepAliveSettingsState();
}

class _BackgroundKeepAliveSettingsState
    extends ConsumerState<BackgroundKeepAliveSettings>
    with WidgetsBindingObserver {
  BackgroundKeepAliveStatus? _status;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _load();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    super.dispose();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed && !_busy) _load();
  }

  Future<void> _load() =>
      _run(() => ref.read(backgroundKeepAliveServiceProvider).load());
  Future<void> _update(bool value) => _run(
    () => ref.read(backgroundKeepAliveServiceProvider).setEnabled(value),
  );
  Future<void> _openBatterySettings() => _run(
    () => ref.read(backgroundKeepAliveServiceProvider).openBatterySettings(),
  );

  Future<void> _run(
    Future<BackgroundKeepAliveStatus> Function() operation,
  ) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final status = await operation();
      if (mounted) setState(() => _status = status);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(error.toString().replaceFirst('Bad state: ', '')),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    children: [
      AmitiaSwitchTile(
        title: '后台保活',
        subtitle: '通过持续通知维持后台运行，优先复用本地运行时；仍受系统省电策略限制',
        value: _status?.enabled ?? false,
        onChanged: _status == null || _busy ? null : _update,
      ),
      if (_status == null && !_busy)
        TextButton(onPressed: _load, child: const Text('重试读取保活设置')),
      if (_status?.enabled == true && _status?.active == false)
        TextButton(
          onPressed: _busy ? null : () => _update(true),
          child: const Text('重试启动后台保活'),
        ),
      if (_status?.enabled == true && _status?.batteryUnrestricted == false)
        TextButton(
          onPressed: _busy ? null : _openBatterySettings,
          child: const Text('打开系统省电设置'),
        ),
    ],
  );
}
