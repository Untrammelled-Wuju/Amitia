import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DevicesPage extends ConsumerWidget {
  const DevicesPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final devices = [
      _DeviceItem(name: 'Windows 桌面端', type: 'desktop', isCurrent: true, lastActive: '当前设备'),
      _DeviceItem(name: 'iPhone 15 Pro', type: 'mobile', isCurrent: false, lastActive: '2小时前'),
      _DeviceItem(name: 'iPad Air', type: 'tablet', isCurrent: false, lastActive: '昨天'),
    ];

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '我的设备', navigation: AmitiaAppBarNavigation.back),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Text(
              '已登录的设备',
              style: AppTypography.caption(context),
            ),
          ),
          const SizedBox(height: AppSpacing.sm),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                for (int i = 0; i < devices.length; i++) ...[
                  _DeviceTile(item: devices[i]),
                  if (i < devices.length - 1)
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
              onPressed: () {},
              visualDensity: VisualDensity.compact,
            ),
        ],
      ),
    );
  }
}
