import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/services/providers.dart';

final _healthProvider = FutureProvider<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(systemServiceProvider);
  return svc.health();
});

class DevicesPage extends ConsumerWidget {
  const DevicesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final healthAsync = ref.watch(_healthProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '我的设备', navigation: AmitiaAppBarNavigation.back),
      body: healthAsync.when(
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
                  '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(
                  label: '重试',
                  onPressed: () => ref.invalidate(_healthProvider),
                ),
              ],
            ),
          ),
        ),
        data: (health) {
          final devices = health?['devices'] as List<dynamic>?;
          final deviceItems = devices != null && devices.isNotEmpty
              ? devices.map((d) {
                  final m = d as Map<String, dynamic>;
                  return _DeviceItem(
                    name: (m['name'] ?? '').toString(),
                    type: (m['type'] ?? 'desktop').toString(),
                    isCurrent: m['isCurrent'] == true,
                    lastActive: (m['lastActive'] ?? '').toString(),
                  );
                }).toList()
              : [
                  _DeviceItem(name: 'Windows 桌面端', type: 'desktop', isCurrent: true, lastActive: '当前设备'),
                ];

          return ListView(
            padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
            children: [
              Padding(
                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                child: Text(
                  '已登录的设备 (${deviceItems.length})',
                  style: AppTypography.caption(context),
                ),
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
                    for (int i = 0; i < deviceItems.length; i++) ...[
                      _DeviceTile(item: deviceItems[i]),
                      if (i < deviceItems.length - 1)
                        Padding(
                          padding: const EdgeInsets.only(left: 56),
                          child: Divider(height: 1, color: context.borderSecondary),
                        ),
                    ],
                  ],
                ),
              ),
            ],
          );
        },
      ),
    );
  }
}

class _DeviceItem {
  final String name;
  final String type;
  final bool isCurrent;
  final String lastActive;

  const _DeviceItem({required this.name, required this.type, required this.isCurrent, required this.lastActive});
}

class _DeviceTile extends StatelessWidget {
  final _DeviceItem item;

  const _DeviceTile({required this.item});

  IconData get _icon {
    switch (item.type) {
      case 'mobile':
        return Icons.smartphone_outlined;
      case 'tablet':
        return Icons.tablet_mac_outlined;
      default:
        return Icons.computer_outlined;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: BorderRadius.circular(12),
            ),
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
                    Text(
                      item.name,
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w500,
                        color: context.textPrimary,
                      ),
                    ),
                    if (item.isCurrent) ...[
                      const SizedBox(width: 6),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          '当前',
                          style: TextStyle(
                            fontSize: 10,
                            color: context.accentPrimary,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
                const SizedBox(height: 2),
                Text(
                  item.lastActive,
                  style: TextStyle(
                    fontSize: 12,
                    color: context.textTertiary,
                  ),
                ),
              ],
            ),
          ),
          if (!item.isCurrent)
            IconButton(
              icon: Icon(Icons.logout, size: 18, color: context.textTertiary),
              onPressed: null,
              visualDensity: VisualDensity.compact,
            ),
        ],
      ),
    );
  }
}
