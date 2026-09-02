import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _devicesProvider = FutureProvider.autoDispose<List<Map<String, dynamic>>>((ref) async {
  return ref.read(deviceMeshServiceProvider).devices();
});

class DevicesPage extends ConsumerWidget {
  const DevicesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devicesAsync = ref.watch(_devicesProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '我的设备',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            tooltip: '刷新',
            onPressed: () => ref.invalidate(_devicesProvider),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: devicesAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                const SizedBox(height: 16),
                Text(
                  '加载失败：${_message(err)}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(label: '重试', onPressed: () => ref.invalidate(_devicesProvider)),
              ],
            ),
          ),
        ),
        data: (devices) {
          if (devices.isEmpty) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(32),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.devices_other_outlined, size: 52, color: context.textTertiary),
                    const SizedBox(height: 14),
                    Text('暂无已绑定设备', style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 6),
                    Text('云端设备完成绑定后会显示在这里。', style: AppTypography.caption(context)),
                  ],
                ),
              ),
            );
          }

          final items = devices.map(_DeviceItem.fromJson).toList();
          return RefreshIndicator(
            onRefresh: () async => ref.refresh(_devicesProvider.future),
            child: ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
              children: [
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  child: Text('已绑定设备 (${items.length})', style: AppTypography.caption(context)),
                ),
                SizedBox(height: AppSpacing.sm),
                Container(
                  margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Column(
                    children: [
                      for (var i = 0; i < items.length; i++) ...[
                        _DeviceTile(
                          item: items[i],
                          onRevoke: () => _confirmRevoke(context, ref, items[i]),
                          onLoadSync: () => ref.read(deviceMeshServiceProvider).syncStatus(items[i].deviceId),
                          onProbe: (runtimeId) async {
                            final result = await ref.read(deviceMeshServiceProvider).probeRuntime(items[i].deviceId, runtimeId);
                            if (context.mounted) {
                              final ok = result?['ok'] != false;
                              ScaffoldMessenger.of(context).showSnackBar(
                                SnackBar(content: Text(ok ? 'Runtime 探测成功' : 'Runtime 已返回探测结果')),
                              );
                            }
                            ref.invalidate(_devicesProvider);
                          },
                        ),
                        if (i < items.length - 1)
                          Padding(
                            padding: const EdgeInsets.only(left: 56),
                            child: Divider(height: 1, color: context.borderSecondary),
                          ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  Future<void> _confirmRevoke(BuildContext context, WidgetRef ref, _DeviceItem item) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('移除设备'),
        content: Text('确定移除「${item.name}」吗？该设备的云端凭据会立即失效。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(dialogContext, true),
            child: Text('移除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(deviceMeshServiceProvider).revokeDevice(item.deviceId);
      ref.invalidate(_devicesProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('设备已移除')));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('移除失败：${_message(e)}')));
      }
    }
  }

  static String _message(Object error) => error.toString().replaceFirst('Exception: ', '');
}

class _RuntimeItem {
  final String runtimeId;
  final String presence;
  final String runtimeSessionId;

  const _RuntimeItem({required this.runtimeId, required this.presence, required this.runtimeSessionId});

  factory _RuntimeItem.fromJson(Map<String, dynamic> json) => _RuntimeItem(
        runtimeId: (json['runtimeId'] ?? '').toString(),
        presence: (json['presence'] ?? 'offline').toString(),
        runtimeSessionId: (json['runtimeSessionId'] ?? '').toString(),
      );
}

class _DeviceItem {
  final String deviceId;
  final String name;
  final String platform;
  final String trustState;
  final String presence;
  final DateTime? lastHeartbeat;
  final List<_RuntimeItem> runtimes;

  const _DeviceItem({
    required this.deviceId,
    required this.name,
    required this.platform,
    required this.trustState,
    required this.presence,
    required this.lastHeartbeat,
    required this.runtimes,
  });

  factory _DeviceItem.fromJson(Map<String, dynamic> json) {
    final deviceId = (json['deviceId'] ?? '').toString();
    final label = (json['label'] ?? '').toString().trim();
    final rawRuntimes = json['runtimes'];
    final runtimes = rawRuntimes is List
        ? rawRuntimes.whereType<Map>().map((item) => _RuntimeItem.fromJson(Map<String, dynamic>.from(item))).toList()
        : <_RuntimeItem>[];
    return _DeviceItem(
      deviceId: deviceId,
      name: label.isEmpty ? (deviceId.isEmpty ? '未命名设备' : deviceId) : label,
      platform: (json['platform'] ?? 'unknown').toString(),
      trustState: (json['trustState'] ?? '').toString(),
      presence: (json['presence'] ?? 'offline').toString(),
      lastHeartbeat: DateTime.tryParse((json['lastHeartbeat'] ?? '').toString())?.toLocal(),
      runtimes: runtimes,
    );
  }
}

class _DeviceTile extends StatefulWidget {
  final _DeviceItem item;
  final VoidCallback onRevoke;
  final Future<Map<String, dynamic>?> Function() onLoadSync;
  final Future<void> Function(String runtimeId) onProbe;

  const _DeviceTile({
    required this.item,
    required this.onRevoke,
    required this.onLoadSync,
    required this.onProbe,
  });

  @override
  State<_DeviceTile> createState() => _DeviceTileState();
}

class _DeviceTileState extends State<_DeviceTile> {
  Map<String, dynamic>? _sync;
  bool _syncLoading = false;
  String _probeBusy = '';

  @override
  void initState() {
    super.initState();
    _loadSync(silent: true);
  }

  @override
  void didUpdateWidget(covariant _DeviceTile oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.item.deviceId != widget.item.deviceId) {
      _sync = null;
      _loadSync(silent: true);
    }
  }

  IconData get _icon {
    final platform = widget.item.platform.toLowerCase();
    if (platform.contains('android') || platform.contains('ios') || platform.contains('mobile')) {
      return Icons.smartphone_outlined;
    }
    if (platform.contains('ipad') || platform.contains('tablet')) return Icons.tablet_mac_outlined;
    return Icons.computer_outlined;
  }

  String get _syncLabel {
    if (_syncLoading && _sync == null) return '读取中';
    final value = _sync;
    if (value == null) return '暂不可用';
    if ((value['error'] ?? '').toString().isNotEmpty) return (value['error']).toString();
    final lastApplied = value['lastApplied'] ?? value['lastAppliedSequence'] ?? value['cursor'];
    final latest = value['latest'] ?? value['latestSequence'] ?? value['head'];
    if (lastApplied != null && latest != null) return '$lastApplied / $latest';
    if (lastApplied != null) return '已应用 $lastApplied';
    return (value['status'] ?? '正常').toString();
  }

  Future<void> _loadSync({bool silent = false}) async {
    if (_syncLoading) return;
    setState(() => _syncLoading = true);
    try {
      final value = await widget.onLoadSync();
      if (!mounted) return;
      setState(() => _sync = value ?? <String, dynamic>{});
      if (!silent) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('同步状态已刷新')));
    } catch (e) {
      if (!mounted) return;
      setState(() => _sync = <String, dynamic>{'error': DevicesPage._message(e)});
      if (!silent) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('同步状态暂不可用')));
    } finally {
      if (mounted) setState(() => _syncLoading = false);
    }
  }

  Future<void> _probe(String runtimeId) async {
    if (_probeBusy.isNotEmpty) return;
    setState(() => _probeBusy = runtimeId);
    try {
      await widget.onProbe(runtimeId);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Runtime 探测失败：${DevicesPage._message(e)}')));
    } finally {
      if (mounted) setState(() => _probeBusy = '');
    }
  }

  @override
  Widget build(BuildContext context) {
    final item = widget.item;
    final online = item.presence.toLowerCase() == 'online';
    final heartbeat = item.lastHeartbeat == null ? '暂无心跳' : _relative(item.lastHeartbeat!);

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(color: context.accentSoft, borderRadius: BorderRadius.circular(12)),
                child: Icon(_icon, size: 20, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(children: [
                      Flexible(child: Text(item.name, overflow: TextOverflow.ellipsis, style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500, color: context.textPrimary))),
                      const SizedBox(width: 6),
                      Container(width: 7, height: 7, decoration: BoxDecoration(shape: BoxShape.circle, color: online ? context.success : context.textTertiary)),
                    ]),
                    const SizedBox(height: 2),
                    Text('${item.platform} · ${online ? '在线' : '离线'} · $heartbeat · ${item.runtimes.length} 个运行时', style: TextStyle(fontSize: 12, color: context.textTertiary)),
                    if (item.trustState.isNotEmpty) Text('信任状态：${item.trustState}', style: TextStyle(fontSize: 11, color: context.textTertiary)),
                    Text('同步：$_syncLabel', style: TextStyle(fontSize: 11, color: context.textTertiary)),
                  ],
                ),
              ),
              IconButton(tooltip: '刷新同步状态', onPressed: _syncLoading ? null : _loadSync, icon: _syncLoading ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)) : const Icon(Icons.sync, size: 18)),
              IconButton(tooltip: '移除设备', icon: Icon(Icons.logout, size: 18, color: context.textTertiary), onPressed: widget.onRevoke, visualDensity: VisualDensity.compact),
            ],
          ),
          if (item.runtimes.isNotEmpty) ...[
            const SizedBox(height: 10),
            for (final runtime in item.runtimes)
              Container(
                margin: const EdgeInsets.only(top: 6),
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
                decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
                child: Row(children: [
                  Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text(runtime.runtimeId, maxLines: 1, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context).copyWith(color: context.textPrimary)),
                    Text(runtime.runtimeSessionId.isEmpty ? runtime.presence : '${runtime.presence} · ${runtime.runtimeSessionId}', maxLines: 1, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context)),
                  ])),
                  TextButton.icon(
                    onPressed: _probeBusy.isNotEmpty ? null : () => _probe(runtime.runtimeId),
                    icon: _probeBusy == runtime.runtimeId ? const SizedBox(width: 14, height: 14, child: CircularProgressIndicator(strokeWidth: 2)) : const Icon(Icons.radar, size: 16),
                    label: const Text('探测'),
                  ),
                ]),
              ),
          ],
        ],
      ),
    );
  }

  static String _relative(DateTime value) {
    final delta = DateTime.now().difference(value);
    if (delta.inMinutes < 1) return '刚刚';
    if (delta.inHours < 1) return '${delta.inMinutes} 分钟前';
    if (delta.inDays < 1) return '${delta.inHours} 小时前';
    return '${delta.inDays} 天前';
  }
}

