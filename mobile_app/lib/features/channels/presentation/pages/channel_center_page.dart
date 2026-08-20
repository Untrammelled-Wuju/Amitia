import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class ChannelCenterPage extends ConsumerStatefulWidget {
  const ChannelCenterPage({super.key});

  @override
  ConsumerState<ChannelCenterPage> createState() => _ChannelCenterPageState();
}

class _ChannelCenterPageState extends ConsumerState<ChannelCenterPage> {
  Map<String, dynamic>? _qqStatus;
  Map<String, dynamic>? _wechatStatus;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _loadStatus();
  }

  Future<void> _loadStatus() async {
    setState(() => _loading = true);
    try {
      final qqSvc = ref.read(qqServiceProvider);
      final qqData = await qqSvc.status();
      if (mounted) setState(() { _qqStatus = qqData; });
    } catch (_) {}
    try {
      final mcpSvc = ref.read(mcpServiceProvider);
      final servers = await mcpSvc.servers();
      if (mounted) {
        setState(() {
          _wechatStatus = servers.isNotEmpty ? {'connected' : true} : {'connected': false};
        });
      }
    } catch (_) {}
    if (mounted) setState(() => _loading = false);
  }

  bool _isQqConnected() {
    if (_qqStatus == null) return false;
    final status = (_qqStatus!['status'] ?? _qqStatus!['connected'] ?? false);
    if (status is bool) return status;
    if (status is String) return status.toLowerCase() == 'connected' || status.toLowerCase() == '已连接';
    return status == 1;
  }

  bool _isWechatConnected() {
    if (_wechatStatus == null) return false;
    final connected = _wechatStatus!['connected'];
    if (connected is bool) return connected;
    return false;
  }

  String _qqSyncTime() {
    if (_qqStatus == null) return '';
    return (_qqStatus!['lastHeartbeat'] ?? _qqStatus!['wsStatus'] ?? _qqStatus!['syncTime'] ?? '').toString();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '渠道中心',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const AmitiaLoadingState(message: '加载中...')
            : ListView(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  _ChannelCard(
                    icon: Icons.chat_bubble,
                    iconColor: const Color(0xFF07C160),
                    title: '微信',
                    subtitle: '连接状态：${_isWechatConnected() ? "已连接" : "未连接"}',
                    syncTime: null,
                    onTap: () => context.push(AppRoutes.channelsWechat),
                  ),
                  SizedBox(height: AppSpacing.sm),
                  _ChannelCard(
                    icon: Icons.account_circle,
                    iconColor: const Color(0xFF12B7F5),
                    title: 'QQ',
                    subtitle: '连接状态：${_isQqConnected() ? "已连接" : "未连接"}',
                    syncTime: _qqSyncTime(),
                    onTap: () => context.push(AppRoutes.channelsQq),
                  ),
                  SizedBox(height: AppSpacing.sm),
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
                  if (syncTime != null && syncTime!.isNotEmpty) ...[
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
