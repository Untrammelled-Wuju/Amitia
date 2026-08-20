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

class _DeviceItem {
  final String deviceId;
  final String name;
  final String platform;
  final String trustState;
  final String presence;
  final DateTime? lastHeartbeat;
  final int runtimeCount;

  const _DeviceItem({
    required this.deviceId,
    required this.name,
    required this.platform,
    required this.trustState,
    required this.presence,
    required this.lastHeartbeat,
    required this.runtimeCount,
  });

  factory _DeviceItem.fromJson(Map<String, dynamic> json) {
    final deviceId = (json['deviceId'] ?? '').toString();
    final label = (json['label'] ?? '').toString().trim();
    final runtimes = json['runtimes'];
    return _DeviceItem(
      deviceId: deviceId,
      name: label.isEmpty ? (deviceId.isEmpty ? '未命名设备' : deviceId) : label,
      platform: (json['platform'] ?? 'unknown').toString(),
      trustState: (json['trustState'] ?? '').toString(),
      presence: (json['presence'] ?? 'offline').toString(),
      lastHeartbeat: DateTime.tryParse((json['lastHeartbeat'] ?? '').toString())?.toLocal(),
      runtimeCount: runtimes is List ? runtimes.length : 0,
    );
  }
}

class _DeviceTile extends StatelessWidget {
  final _DeviceItem item;
  final VoidCallback onRevoke;

  const _DeviceTile({required this.item, required this.onRevoke});

  IconData get _icon {
    final platform = item.platform.toLowerCase();
    if (platform.contains('android') || platform.contains('ios') || platform.contains('mobile')) {
      return Icons.smartphone_outlined;
    }
    if (platform.contains('ipad') || platform.contains('tablet')) return Icons.tablet_mac_outlined;
    return Icons.computer_outlined;
  }

  @override
  Widget build(BuildContext context) {
    final online = item.presence.toLowerCase() == 'online';
    final heartbeat = item.lastHeartbeat == null ? '暂无心跳' : _relative(item.lastHeartbeat!);
    final detail = [
      item.platform,
      online ? '在线' : '离线',
      heartbeat,
      if (item.runtimeCount > 0) '${item.runtimeCount} 个运行时',
    ].join(' · ');

    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
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
              mainAxisSize: MainAxisSize.min,
              children: [
                Row(
                  children: [
                    Flexible(
                      child: Text(
                        item.name,
                        overflow: TextOverflow.ellipsis,
                        style: TextStyle(fontSize: 14, fontWeight: FontWeight.w500, color: context.textPrimary),
                      ),
                    ),
                    const SizedBox(width: 6),
                    Container(
                      width: 7,
                      height: 7,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: online ? context.success : context.textTertiary,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 2),
                Text(detail, style: TextStyle(fontSize: 12, color: context.textTertiary)),
                if (item.trustState.isNotEmpty) ...[
                  const SizedBox(height: 2),
                  Text('信任状态：${item.trustState}', style: TextStyle(fontSize: 11, color: context.textTertiary)),
                ],
              ],
            ),
          ),
          IconButton(
            tooltip: '移除设备',
            icon: Icon(Icons.logout, size: 18, color: context.textTertiary),
            onPressed: onRevoke,
            visualDensity: VisualDensity.compact,
          ),
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
