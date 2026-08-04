import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../shared/mock_data/mock_channels.dart';
import '../../../../shared/models/models.dart';

class ChannelCenterPage extends ConsumerWidget {
  const ChannelCenterPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '渠道中心',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _ChannelCard(
              icon: Icons.chat_bubble,
              iconColor: const Color(0xFF07C160),
              title: '微信',
              subtitle: '连接状态：${MockChannels.wechat.status == ChannelStatus.connected ? "已连接" : "未连接"}',
              syncTime: MockChannels.wechat.lastHeartbeat,
              onTap: () => context.push(AppRoutes.channelsWechat),
            ),
            const SizedBox(height: AppSpacing.sm),
            _ChannelCard(
              icon: Icons.account_circle,
              iconColor: const Color(0xFF12B7F5),
              title: 'QQ',
              subtitle: '连接状态：${MockChannels.qq.status == ChannelStatus.connected ? "已连接" : "未连接"}',
              syncTime: MockChannels.qq.wsStatus,
              onTap: () => context.push(AppRoutes.channelsQq),
            ),
            const SizedBox(height: AppSpacing.sm),
            _ChannelCard(
              icon: Icons.add_circle_outline,
              iconColor: context.textTertiary,
              title: '更多渠道',
              subtitle: '后续渠道占位',
              syncTime: null,
              onTap: () {
                ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('更多渠道即将上线')),
                );
              },
              isPlaceholder: true,
            ),
          ],
        ),
      ),
    );
  }
}

class _ChannelCard extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String title;
  final String subtitle;
  final String? syncTime;
  final VoidCallback onTap;
  final bool isPlaceholder;

  const _ChannelCard({
    required this.icon,
    required this.iconColor,
    required this.title,
    required this.subtitle,
    required this.syncTime,
    required this.onTap,
    this.isPlaceholder = false,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: iconColor.withValues(alpha: 0.12),
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(icon, size: 24, color: iconColor),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(subtitle, style: AppTypography.caption(context)),
                  if (syncTime != null) ...[
                    const SizedBox(height: 2),
                    Text('最近同步：$syncTime', style: AppTypography.label(context)),
                  ],
                ],
              ),
            ),
            Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
          ],
        ),
      ),
    );
  }
}
