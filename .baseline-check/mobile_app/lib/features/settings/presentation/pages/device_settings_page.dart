import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DeviceSettingsPage extends ConsumerWidget {
  const DeviceSettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final deployment = ref.watch(mobileDeploymentConfigProvider);
    final status = ref.watch(localDeviceMeshStatusProvider);
    final devices = ref.watch(deviceMeshDevicesProvider);
    final state = status.asData?.value?['state']?.toString() ?? '';
    final connected = state.toLowerCase() == 'connected';
    final cloudBaseUrl = status.asData?.value?['cloudBaseUrl']?.toString().trim();
    final deviceCount = devices.asData?.value.length;

    return AmitiaScaffold(
      appBar: const AmitiaAppBar(
        title: '设备设置',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: RefreshIndicator(
        onRefresh: () async {
          ref.invalidate(localDeviceMeshStatusProvider);
          ref.invalidate(deviceMeshDevicesProvider);
          await Future.wait([
            ref.read(localDeviceMeshStatusProvider.future),
            ref.read(deviceMeshDevicesProvider.future),
          ]);
        },
        child: ListView(
          physics: const AlwaysScrollableScrollPhysics(),
          padding: EdgeInsets.fromLTRB(0, AppSpacing.sm, 0, AppSpacing.xl),
          children: [
            _sectionTitle(context, '云端协同'),
            _group(context, [
              _StatusTile(
                icon: Icons.cloud_done_outlined,
                title: '设备协同',
                subtitle: connected
                    ? (cloudBaseUrl == null || cloudBaseUrl.isEmpty ? 'Device Mesh 已连接' : cloudBaseUrl)
                    : _meshStatusText(status),
                value: connected ? '已连接' : '未连接',
                valueColor: connected ? context.success : context.textTertiary,
              ),
              _StatusTile(
                icon: Icons.route_outlined,
                title: '任务路由',
                subtitle: connected ? '云端可将设备任务路由到已连接 Runtime' : '连接云端协同后由 Device Mesh 自动路由',
                value: connected ? '自动' : '关闭',
              ),
              _StatusTile(
                icon: Icons.devices_outlined,
                title: '云端设备',
                subtitle: '来自 /api/device-mesh/v1/devices',
                value: deviceCount == null ? '—' : '$deviceCount 台',
              ),
            ]),
            _sectionTitle(context, '同步设置'),
            _group(context, [
              _StatusTile(
                icon: Icons.sync_outlined,
                title: '跨设备同步',
                subtitle: '当前后端没有独立同步开关；连接 Cloud Core 后由云端协同机制自动工作',
                value: connected ? '随云端协同' : '未启用',
                valueColor: connected ? context.success : context.textTertiary,
              ),
            ]),
            _sectionTitle(context, '设备管理'),
            _group(context, [
              _RouteTile(
                icon: Icons.admin_panel_settings_outlined,
                title: '设备权限',
                subtitle: '管理系统权限与工具访问',
                onTap: () => context.push(AppRoutes.settingsPermissions),
              ),
              _RouteTile(
                icon: Icons.monitor_heart_outlined,
                title: '本机运行状态',
                subtitle: '查看 Embedded Runtime 状态与诊断',
                onTap: () => context.push(AppRoutes.settingsRuntime),
              ),
              _RouteTile(
                icon: Icons.cloud_outlined,
                title: '部署配置',
                subtitle: deployment.remoteCoreUri?.trim().isNotEmpty == true
                    ? deployment.remoteCoreUri!.trim()
                    : '本地 / 云端运行模式与 Core 地址',
                onTap: () => context.push(AppRoutes.settingsDeployment),
              ),
            ]),
            if (connected) ...[
              SizedBox(height: AppSpacing.lg),
              Padding(
                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                child: AmitiaButton(
                  label: '解除本机云端绑定',
                  isSecondary: true,
                  isFullWidth: true,
                  onPressed: () => _unlink(context, ref),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }

  static Future<void> _unlink(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('解除云端绑定'),
        content: const Text('这会删除本机保存的 Device Mesh 云端凭据。云端设备记录仍可在“我的设备”中单独移除。'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('解除')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      final service = ref.read(deviceMeshLocalServiceProvider);
      if (service == null) throw StateError('本机 Runtime 当前不可用');
      await service.deleteCredential();
      ref.invalidate(localDeviceMeshStatusProvider);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('本机云端绑定已解除')));
      }
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('解除失败：${e.toString().replaceFirst('Bad state: ', '')}')));
      }
    }
  }

  static String _meshStatusText(AsyncValue<Map<String, dynamic>?> status) {
    if (status.isLoading) return '正在读取本机 Device Mesh 状态';
    if (status.hasError) return '本机 Runtime 当前不可用';
    final data = status.asData?.value;
    if (data == null) return '本机 Runtime 尚未就绪';
    final raw = (data['state'] ?? 'unprovisioned').toString();
    if (raw.toLowerCase() == 'unprovisioned') return '本机尚未加入 Cloud Core';
    final error = (data['lastErrorCode'] ?? '').toString();
    return error.isEmpty ? raw : '$raw · $error';
  }

  static Widget _sectionTitle(BuildContext context, String title) => Padding(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.sm),
        child: Text(title, style: AppTypography.caption(context)),
      );

  static Widget _group(BuildContext context, List<Widget> children) => Container(
        margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.6),
        ),
        child: Column(
          children: [
            for (var i = 0; i < children.length; i++) ...[
              children[i],
              if (i < children.length - 1)
                Padding(
                  padding: const EdgeInsets.only(left: 56),
                  child: Divider(height: 1, color: context.borderSecondary),
                ),
            ],
          ],
        ),
      );
}

class _StatusTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final String value;
  final Color? valueColor;

  const _StatusTile({required this.icon, required this.title, required this.subtitle, required this.value, this.valueColor});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          _leading(context, icon),
          const SizedBox(width: 11),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(subtitle, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
              ],
            ),
          ),
          const SizedBox(width: 8),
          Text(value, style: AppTypography.caption(context).copyWith(color: valueColor ?? context.textTertiary)),
        ],
      ),
    );
  }
}

class _RouteTile extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  const _RouteTile({required this.icon, required this.title, required this.subtitle, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 13, vertical: 12),
        child: Row(
          children: [
            _leading(context, icon),
            const SizedBox(width: 11),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.body(context)),
                  const SizedBox(height: 2),
                  Text(subtitle, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                ],
              ),
            ),
            Icon(Icons.chevron_right, size: 19, color: context.textTertiary),
          ],
        ),
      ),
    );
  }
}

Widget _leading(BuildContext context, IconData icon) => Container(
      width: 32,
      height: 32,
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(11)),
      child: Icon(icon, size: 17, color: context.textSecondary),
    );
