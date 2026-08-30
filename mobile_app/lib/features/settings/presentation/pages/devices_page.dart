import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DevicesPage extends ConsumerWidget {
  const DevicesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devicesAsync = ref.watch(deviceMeshDevicesProvider);
    final identityAsync = ref.watch(localDeviceMeshIdentityProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '我的设备',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            tooltip: '添加设备',
            onPressed: () => context.push(AppRoutes.settingsDeviceAdd),
            icon: const Icon(Icons.add),
          ),
          IconButton(
            tooltip: '设备设置',
            onPressed: () => context.push(AppRoutes.settingsDeviceSettings),
            icon: const Icon(Icons.settings_outlined),
          ),
        ],
      ),
      body: devicesAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => _LoadError(
          message: _message(err),
          onRetry: () => ref.invalidate(deviceMeshDevicesProvider),
        ),
        data: (rawDevices) {
          final items = rawDevices.map(_DeviceItem.fromJson).toList();
          final localId = identityAsync.asData?.value?['deviceId']?.toString();
          final currentIndex = localId == null ? -1 : items.indexWhere((item) => item.deviceId == localId);
          final current = currentIndex >= 0 ? items[currentIndex] : null;
          final others = <_DeviceItem>[
            for (var i = 0; i < items.length; i++)
              if (i != currentIndex) items[i],
          ];

          return RefreshIndicator(
            onRefresh: () async {
              ref.invalidate(localDeviceMeshIdentityProvider);
              ref.invalidate(localDeviceMeshStatusProvider);
              ref.invalidate(deviceMeshDevicesProvider);
              await ref.read(deviceMeshDevicesProvider.future);
            },
            child: ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.md,
                AppSpacing.pagePadding,
                AppSpacing.xl,
              ),
              children: [
                if (current != null) ...[
                  _CurrentDeviceCard(item: current),
                  SizedBox(height: AppSpacing.lg),
                ],
                if (others.isNotEmpty) ...[
                  Text('其它设备', style: AppTypography.caption(context)),
                  SizedBox(height: AppSpacing.sm),
                  Container(
                    decoration: BoxDecoration(
                      color: context.surfacePrimary,
                      borderRadius: AppRadius.brMedium,
                      border: Border.all(color: context.borderPrimary, width: 0.6),
                    ),
                    child: Column(
                      children: [
                        for (var i = 0; i < others.length; i++) ...[
                          _DeviceTile(
                            item: others[i],
                            onRevoke: () => _confirmRevoke(context, ref, others[i]),
                          ),
                          if (i < others.length - 1)
                            Padding(
                              padding: const EdgeInsets.only(left: 56),
                              child: Divider(height: 1, color: context.borderSecondary),
                            ),
                        ],
                      ],
                    ),
                  ),
                ],
                if (items.isEmpty)
                  _EmptyDevices(onAdd: () => context.push(AppRoutes.settingsDeviceAdd)),
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
      ref.invalidate(deviceMeshDevicesProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('设备已移除')));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('移除失败：${_message(e)}')));
      }
    }
  }

  static String _message(Object error) => error.toString().replaceFirst('Exception: ', '').replaceFirst('Bad state: ', '');
}

class _CurrentDeviceCard extends StatelessWidget {
  final _DeviceItem item;
  const _CurrentDeviceCard({required this.item});

  @override
  Widget build(BuildContext context) {
    final online = item.presence.toLowerCase() == 'online';
    return Container(
      padding: const EdgeInsets.all(15),
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [
            context.isDark ? const Color(0xFF1D1E20) : const Color(0xFF302B26),
            context.isDark ? const Color(0xFF252629) : const Color(0xFF4B3D32),
          ],
        ),
        borderRadius: BorderRadius.circular(24),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: Colors.white.withValues(alpha: 0.10),
                  borderRadius: BorderRadius.circular(15),
                  border: Border.all(color: Colors.white.withValues(alpha: 0.08)),
                ),
                child: const Icon(Icons.smartphone_outlined, color: Colors.white, size: 21),
              ),
              const SizedBox(width: 11),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(item.name, style: const TextStyle(color: Colors.white, fontSize: 13, fontWeight: FontWeight.w600)),
                    const SizedBox(height: 3),
                    Text('${item.platform} · 本机', style: TextStyle(color: Colors.white.withValues(alpha: 0.58), fontSize: 10)),
                  ],
                ),
              ),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
                decoration: BoxDecoration(
                  color: online ? const Color(0xFF4D715D).withValues(alpha: 0.25) : Colors.white.withValues(alpha: 0.08),
                  borderRadius: BorderRadius.circular(99),
                ),
                child: Text(online ? '在线' : '离线', style: const TextStyle(color: Colors.white, fontSize: 9)),
              ),
            ],
          ),
          const SizedBox(height: 13),
          Container(height: 1, color: Colors.white.withValues(alpha: 0.08)),
          const SizedBox(height: 11),
          Row(
            children: [
              Container(width: 6, height: 6, decoration: const BoxDecoration(shape: BoxShape.circle, color: Color(0xFF8FBE9D))),
              const SizedBox(width: 7),
              Expanded(
                child: Text(
                  item.runtimeCount > 0 ? '${item.runtimeCount} 个 Runtime 已注册' : '设备已注册，暂无 Runtime 信息',
                  style: TextStyle(color: Colors.white.withValues(alpha: 0.68), fontSize: 10),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }
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
    if (platform.contains('android') || platform.contains('ios') || platform.contains('mobile')) return Icons.smartphone_outlined;
    if (platform.contains('ipad') || platform.contains('tablet')) return Icons.tablet_mac_outlined;
    return Icons.computer_outlined;
  }

  @override
  Widget build(BuildContext context) {
    final online = item.presence.toLowerCase() == 'online';
    final heartbeat = item.lastHeartbeat == null ? '暂无心跳' : _relative(item.lastHeartbeat!);
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(color: context.accentSoft, borderRadius: BorderRadius.circular(13)),
            child: Icon(_icon, size: 19, color: context.accentPrimary),
          ),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Flexible(child: Text(item.name, overflow: TextOverflow.ellipsis, style: AppTypography.body(context))),
                    const SizedBox(width: 6),
                    Container(width: 6, height: 6, decoration: BoxDecoration(shape: BoxShape.circle, color: online ? context.success : context.textTertiary)),
                  ],
                ),
                const SizedBox(height: 3),
                Text(
                  '${item.platform} · ${online ? '在线' : '离线'} · $heartbeat${item.runtimeCount > 0 ? ' · ${item.runtimeCount} 个 Runtime' : ''}',
                  style: AppTypography.caption(context),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
          PopupMenuButton<String>(
            tooltip: '设备操作',
            onSelected: (value) {
              if (value == 'remove') onRevoke();
            },
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'remove', child: Text('移除设备')),
            ],
            icon: Icon(Icons.more_horiz, size: 20, color: context.textTertiary),
          ),
        ],
      ),
    );
  }

  static String _relative(DateTime time) {
    final diff = DateTime.now().difference(time);
    if (diff.inMinutes < 1) return '刚刚';
    if (diff.inMinutes < 60) return '${diff.inMinutes} 分钟前';
    if (diff.inHours < 24) return '${diff.inHours} 小时前';
    return '${diff.inDays} 天前';
  }
}

class _EmptyDevices extends StatelessWidget {
  final VoidCallback onAdd;
  const _EmptyDevices({required this.onAdd});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 80),
      child: Column(
        children: [
          Icon(Icons.devices_other_outlined, size: 48, color: context.textTertiary),
          const SizedBox(height: 14),
          Text('暂无已绑定设备', style: AppTypography.cardTitle(context)),
          const SizedBox(height: 6),
          Text('将本机或其它客户端加入 Cloud Core 后会显示在这里。', style: AppTypography.caption(context), textAlign: TextAlign.center),
          const SizedBox(height: 18),
          SizedBox(width: 160, child: AmitiaButton(label: '添加设备', onPressed: onAdd)),
        ],
      ),
    );
  }
}

class _LoadError extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _LoadError({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 46, color: context.textSecondary),
            const SizedBox(height: 14),
            Text('加载失败：$message', style: AppTypography.body(context).copyWith(color: context.error), textAlign: TextAlign.center),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: onRetry),
          ],
        ),
      ),
    );
  }
}
