import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ToolboxDeviceStatusPage extends StatelessWidget {
  const ToolboxDeviceStatusPage({super.key});

  static const _items = <(IconData, String, String)>[
    (Icons.system_update, '系统版本', 'Android 14 · MIUI 9.2'),
    (Icons.memory, '架构', 'arm64-v8a · 8 核'),
    (Icons.developer_board, '内存', '已用 4.2 / 12 GB'),
    (Icons.sd_storage, '存储', '已用 78 / 256 GB'),
    (Icons.battery_charging_full, '电池优化', '已加入白名单'),
    (Icons.verified_user_outlined, '权限摘要', '6 项已授权 · 2 项未授权'),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '设备状态', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Container(
            padding: const EdgeInsets.all(AppSpacing.cardPadding),
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
          const SizedBox(height: AppSpacing.lg),
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
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 14),
                    child: Row(
                      children: [
                        Icon(_items[i].$1, size: 20, color: context.textSecondary),
                        const SizedBox(width: 14),
                        Expanded(child: Text(_items[i].$2, style: AppTypography.body(context))),
                        Text(_items[i].$3, style: AppTypography.caption(context)),
                      ],
                    ),
                  ),
                  if (i < _items.length - 1)
                    Padding(
                      padding: const EdgeInsets.only(left: AppSpacing.lg),
                      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
                    ),
                ],
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}
