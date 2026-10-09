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
  bool _refreshPending = false;

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
    if (state == AppLifecycleState.resumed) {
      if (_busy) {
        _refreshPending = true;
      } else {
        _load();
      }
    }
  }

  Future<void> _load() =>
      _run(() => ref.read(backgroundKeepAliveServiceProvider).load());
  Future<void> _update(bool value) => _run(
    () => ref.read(backgroundKeepAliveServiceProvider).setEnabled(value),
  );
  Future<void> _openBatterySettings() => _run(
    () => ref.read(backgroundKeepAliveServiceProvider).openBatterySettings(),
  );
  Future<void> _openNetworkSettings() => _run(
    () => ref.read(backgroundKeepAliveServiceProvider).openNetworkSettings(),
  );
  Future<void> _openAppSettings() => _run(
    () => ref.read(backgroundKeepAliveServiceProvider).openAppSettings(),
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
      if (mounted) {
        setState(() => _busy = false);
        if (_refreshPending) {
          _refreshPending = false;
          _load();
        }
      }
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
      if (_status != null) ...[
        ListTile(
          leading: Icon(
            _status!.active
                ? Icons.check_circle_outline
                : Icons.pause_circle_outline,
          ),
          title: Text(_status!.active ? '后台服务正在运行' : '后台服务未运行'),
          subtitle: Text(_status!.enabled ? '后台保活已开启' : '后台保活未开启'),
        ),
        ListTile(
          leading: const Icon(Icons.battery_saver_outlined),
          title: Text(_status!.batteryUnrestricted ? '电池优化：已豁免' : '电池优化：未豁免'),
          subtitle: Text(switch (_status!.powerSaveMode) {
            true => '系统省电模式已开启，后台运行仍可能受限',
            false => '系统省电模式未开启',
            null => '系统省电模式状态暂不可用',
          }),
        ),
        ListTile(
          leading: const Icon(Icons.data_usage),
          title: Text(switch (_status!.backgroundDataRestriction) {
            'disabled' => '省流量模式：未开启',
            'whitelisted' => '省流量模式：已允许本应用不受限用量',
            'restricted' => '省流量模式：后台流量受限',
            _ => '省流量模式：状态暂不可用',
          }),
          subtitle: const Text('此状态仅反映系统省流量限制；后台联网、自启动和锁屏断网策略需在系统设置中检查'),
        ),
      ],
      Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text(
              '锁屏继续生成：将 Amitia 的电池策略设为不优化或无限制，允许后台数据；如系统提供自启动、后台联网或锁屏断网选项，也请检查。返回此页后会自动刷新可查询的状态。',
            ),
            Wrap(
              spacing: 8,
              children: [
                TextButton(
                  onPressed: _busy ? null : _openBatterySettings,
                  child: const Text('打开系统省电设置'),
                ),
                TextButton(
                  onPressed: _busy ? null : _openNetworkSettings,
                  child: const Text('后台流量设置'),
                ),
                TextButton(
                  onPressed: _busy ? null : _openAppSettings,
                  child: const Text('应用系统设置'),
                ),
                TextButton(
                  onPressed: _busy ? null : _load,
                  child: const Text('刷新状态'),
                ),
              ],
            ),
          ],
        ),
      ),
    ],
  );
}
