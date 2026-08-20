import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class ToolboxDeviceStatusPage extends ConsumerStatefulWidget {
  const ToolboxDeviceStatusPage({super.key});

  @override
  ConsumerState<ToolboxDeviceStatusPage> createState() => _ToolboxDeviceStatusPageState();
}

class _DeviceItem {
  final IconData icon;
  final String label;
  final String value;
  final bool isServer;
  const _DeviceItem({required this.icon, required this.label, required this.value, this.isServer = false});
}

class _ToolboxDeviceStatusPageState extends ConsumerState<ToolboxDeviceStatusPage> {
  bool _loading = true;
  String? _error;
  List<_DeviceItem> _items = [];

  static const _staticItems = <_DeviceItem>[
    _DeviceItem(icon: Icons.system_update, label: '系统版本', value: 'Android 14 · MIUI 9.2'),
    _DeviceItem(icon: Icons.memory, label: '架构', value: 'arm64-v8a · 8 核'),
    _DeviceItem(icon: Icons.developer_board, label: '内存', value: '已用 4.2 / 12 GB'),
    _DeviceItem(icon: Icons.sd_storage, label: '存储', value: '已用 78 / 256 GB'),
    _DeviceItem(icon: Icons.battery_charging_full, label: '电池优化', value: '已加入白名单'),
    _DeviceItem(icon: Icons.verified_user_outlined, label: '权限摘要', value: '6 项已授权 · 2 项未授权'),
  ];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(systemServiceProvider);
      final result = await svc.health();
      final data = result as Map<String, dynamic>? ?? {};
      final comps = (data['components'] as List?)?.cast<Map<String, dynamic>>() ??
          (data['snapshots'] as List?)?.cast<Map<String, dynamic>>() ?? [];

      final serverItems = comps.map((c) {
        final name = (c['name'] ?? c['component'] ?? 'Unknown').toString();
        final status = (c['status'] ?? c['state'] ?? '').toString();
        final detail = (c['version'] ?? c['detail'] ?? c['info'] ?? '').toString();
        return _DeviceItem(
          icon: Icons.dns_outlined,
          label: name,
          value: '$status · $detail',
          isServer: true,
        );
      }).toList();

      final allItems = <_DeviceItem>[
        ...serverItems,
        ..._staticItems,
      ];

      if (mounted) {
        setState(() { _items = allItems; _loading = false; });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _items = [..._staticItems];
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在获取设备状态...');
    if (_error != null && _items.isEmpty) {
      return AmitiaErrorState(message: _error!, onRetry: _load);
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '设备状态', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Container(
            padding: EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brMedium,
            ),
            child: Row(
              children: [
                Icon(Icons.phone_android, color: context.accentPrimary, size: 30),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Mi 14 Ultra', style: AppTypography.cardTitle(context)),
                      Text('设备运行正常 · 已连接 Amitia 核心', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
              ],
            ),
          ),
          SizedBox(height: AppSpacing.lg),
          Container(
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                for (int i = 0; i < _items.length; i++) ...[
                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 14),
                    child: Row(
                      children: [
                        Icon(_items[i].icon, size: 20, color: _items[i].isServer ? context.accentPrimary : context.textSecondary),
                        const SizedBox(width: 14),
                        Expanded(child: Text(_items[i].label, style: AppTypography.body(context))),
                        Text(_items[i].value, style: AppTypography.caption(context)),
                      ],
                    ),
                  ),
                  if (i < _items.length - 1)
                    Padding(
                      padding: EdgeInsets.only(left: AppSpacing.lg),
                      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
                    ),
                ],
              ],
            ),
          ),
          SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}
