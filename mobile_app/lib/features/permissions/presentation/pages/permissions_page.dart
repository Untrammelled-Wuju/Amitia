import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';

class PermissionsPage extends ConsumerStatefulWidget {
  const PermissionsPage({super.key});

  @override
  ConsumerState<PermissionsPage> createState() => _PermissionsPageState();
}

class _PermissionsPageState extends ConsumerState<PermissionsPage> {
  late List<PermissionItem> _permissions;

  @override
  void initState() {
    super.initState();
    _permissions = [
      PermissionItem(
        name: '无障碍服务',
        icon: Icons.accessibility_new,
        status: '已授权',
        description: '允许 Amitia 读取屏幕内容并提供辅助',
      ),
      PermissionItem(
        name: '通知读取',
        icon: Icons.notifications_outlined,
        status: '已授权',
        description: '读取系统通知以提供智能提醒',
      ),
      PermissionItem(
        name: '悬浮窗',
        icon: Icons.picture_in_picture,
        status: '已授权',
        description: '在其他应用上方显示悬浮窗',
      ),
      PermissionItem(
        name: '文件访问',
        icon: Icons.folder_outlined,
        status: '需要设置',
        description: '访问设备存储中的文件',
      ),
      PermissionItem(
        name: '麦克风',
        icon: Icons.mic_outlined,
        status: '已授权',
        description: '语音输入和通话',
      ),
      PermissionItem(
        name: '相机',
        icon: Icons.camera_alt_outlined,
        status: '未授权',
        description: '拍照和扫描功能',
      ),
      PermissionItem(
        name: '位置',
        icon: Icons.location_on_outlined,
        status: '未授权',
        description: '获取设备位置信息',
      ),
      PermissionItem(
        name: '电池优化',
        icon: Icons.battery_std,
        status: '已授权',
        description: '忽略电池优化以保持后台运行',
      ),
      PermissionItem(
        name: 'Shizuku',
        icon: Icons.security,
        status: '不可用',
        description: '提供高级系统操作能力',
      ),
    ];
  }

  BadgeType _badgeType(String status) {
    switch (status) {
      case '已授权':
        return BadgeType.success;
      case '需要设置':
        return BadgeType.warning;
      case '不可用':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  void _showGuide(PermissionItem item) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(
          top: Radius.circular(AppRadius.large),
        ),
      ),
      builder: (ctx) => _PermissionGuideSheet(item: item),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '系统权限', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView.separated(
        padding: const EdgeInsets.symmetric(
          vertical: AppSpacing.md,
          horizontal: AppSpacing.pagePadding,
        ),
        itemCount: _permissions.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
        itemBuilder: (context, index) {
          final item = _permissions[index];
          return _PermissionCard(
            item: item,
            badgeType: _badgeType(item.status),
            onTap: () => _showGuide(item),
          );
        },
      ),
    );
  }
}

class _PermissionCard extends StatelessWidget {
  final PermissionItem item;
  final BadgeType badgeType;
  final VoidCallback onTap;

  const _PermissionCard({
    required this.item,
    required this.badgeType,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(AppSpacing.cardPadding),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(item.icon, size: 20, color: context.accentPrimary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(item.name, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(
                    item.description,
                    style: AppTypography.label(context),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ),
            ),
            const SizedBox(width: AppSpacing.sm),
            AmitiaStatusBadge(label: item.status, type: badgeType),
          ],
        ),
      ),
    );
  }
}

class _PermissionGuideSheet extends StatelessWidget {
  final PermissionItem item;

  const _PermissionGuideSheet({required this.item});

  @override
  Widget build(BuildContext context) {
    final steps = <String>[
      '打开系统设置',
      '找到「应用管理」',
      '选择 Amitia',
      '找到「${item.name}」并开启',
    ];
    return SafeArea(
      child: Padding(
        padding: const EdgeInsets.fromLTRB(
          AppSpacing.lg,
          AppSpacing.lg,
          AppSpacing.lg,
          AppSpacing.xxl,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Center(
              child: Container(
                width: 36,
                height: 4,
                decoration: BoxDecoration(
                  color: context.borderPrimary,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            Text(item.name, style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            Text(item.description, style: AppTypography.caption(context)),
            const SizedBox(height: AppSpacing.lg),
            ...steps.asMap().entries.map(
              (entry) {
                return Padding(
                  padding: const EdgeInsets.only(bottom: AppSpacing.md),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Container(
                        width: 22,
                        height: 22,
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          shape: BoxShape.circle,
                        ),
                        child: Center(
                          child: Text(
                            '${entry.key + 1}',
                            style: TextStyle(
                              fontSize: 12,
                              color: context.accentPrimary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: AppSpacing.md),
                      Expanded(
                        child: Text(
                          entry.value,
                          style: AppTypography.bodySmall(context),
                        ),
                      ),
                    ],
                  ),
                );
              },
            ),
          ],
        ),
      ),
    );
  }
}
