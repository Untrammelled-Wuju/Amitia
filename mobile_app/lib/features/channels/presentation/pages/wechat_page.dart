import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class WechatPage extends ConsumerStatefulWidget {
  const WechatPage({super.key});

  @override
  ConsumerState<WechatPage> createState() => _WechatPageState();
}

class _WechatPageState extends ConsumerState<WechatPage> {
  late ChannelStatus _status;
  late bool _receiving;
  late bool _sending;
  late List<String> _logs;
  late String _lastHeartbeat;

  @override
  void initState() {
    super.initState();
    final wechat = MockChannels.wechat;
    _status = wechat.status;
    _receiving = wechat.receivingMessages;
    _sending = wechat.sendingMessages;
    _logs = List.of(wechat.logs);
    _lastHeartbeat = wechat.lastHeartbeat ?? '未知';
  }

  bool get _isConnected => _status == ChannelStatus.connected;

  void _snack(String message, {Color? color}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        duration: const Duration(seconds: 2),
        backgroundColor: color,
      ),
    );
  }

  void _showReconnectConfirm() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('重新连接', style: AppTypography.cardTitle(context)),
        content: Text('确定要重新连接微信吗？当前连接将被断开并重新建立。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text('取消', style: TextStyle(color: context.textSecondary)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _doReconnect();
            },
            child: Text('确定', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  Future<void> _doReconnect() async {
    setState(() {
      _status = ChannelStatus.connecting;
      _receiving = false;
      _sending = false;
    });
    _snack('正在重新连接微信...', color: context.accentPrimary);

    await Future.delayed(const Duration(milliseconds: 1500));
    if (!mounted) return;

    setState(() {
      _status = ChannelStatus.connected;
      _receiving = true;
      _sending = true;
      _lastHeartbeat = '刚刚';
      _logs.insert(0, '[${DateTime.now().toString().substring(11, 19)}] 重新连接成功');
    });

    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.check_circle, size: 48, color: context.success),
            const SizedBox(height: AppSpacing.md),
            Text('连接成功', style: AppTypography.cardTitle(context)),
            const SizedBox(height: AppSpacing.xs),
            Text('微信已成功连接', style: AppTypography.caption(context)),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text('好的', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showDisconnectConfirm() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('断开连接', style: AppTypography.cardTitle(context)),
        content: Text('确定要断开微信连接吗？断开后将无法收发微信消息。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text('取消', style: TextStyle(color: context.textSecondary)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _doDisconnect();
            },
            child: Text('断开', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _doDisconnect() {
    setState(() {
      _status = ChannelStatus.disconnected;
      _receiving = false;
      _sending = false;
      _logs.insert(0, '[${DateTime.now().toString().substring(11, 19)}] 已断开连接');
    });
    _snack('微信已断开连接', color: context.warning);
  }

  void _regenerateQR() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('重新生成二维码', style: AppTypography.cardTitle(context)),
        content: Text('将生成新的登录二维码，当前二维码将失效。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: Text('取消', style: TextStyle(color: context.textSecondary)),
          ),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _snack('已生成新的二维码', color: context.accentPrimary);
            },
            child: Text('生成', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final wechat = MockChannels.wechat;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '微信连接', showBackButton: true, fallbackRoute: AppRoutes.channels),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          _buildStatusCard(),
          const SizedBox(height: AppSpacing.sectionGap),
          _buildQRCard(),
          const SizedBox(height: AppSpacing.sectionGap),
          _buildAccountInfo(wechat),
          const SizedBox(height: AppSpacing.sectionGap),
          _buildMessageStatus(),
          const SizedBox(height: AppSpacing.sectionGap),
          _buildActions(),
          const SizedBox(height: AppSpacing.sectionGap),
          _buildLogs(),
          const SizedBox(height: AppSpacing.sectionGap),
          _buildRiskWarning(),
        ],
      ),
    );
  }

  Widget _buildStatusCard() {
    final statusLabel = switch (_status) {
      ChannelStatus.connected => '已连接',
      ChannelStatus.disconnected => '未连接',
      ChannelStatus.connecting => '连接中...',
      ChannelStatus.expired => '已过期',
    };
    final badgeType = switch (_status) {
      ChannelStatus.connected => BadgeType.success,
      ChannelStatus.disconnected => BadgeType.neutral,
      ChannelStatus.connecting => BadgeType.warning,
      ChannelStatus.expired => BadgeType.error,
    };
    return AmitiaCard(
      child: Row(
        children: [
          Container(
            width: 52,
            height: 52,
            decoration: BoxDecoration(
              color: _isConnected
                  ? context.success.withValues(alpha: 0.12)
                  : context.warning.withValues(alpha: 0.12),
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              _isConnected ? Icons.wechat : Icons.wechat_outlined,
              size: 28,
              color: _isConnected ? context.success : context.warning,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('微信渠道', style: AppTypography.cardTitle(context)),
                Text('通过微信网页版协议连接', style: AppTypography.caption(context)),
              ],
            ),
          ),
          AmitiaStatusBadge(label: statusLabel, type: badgeType),
        ],
      ),
    );
  }

  Widget _buildQRCard() {
    return AmitiaCard(
      child: Column(
        children: [
          Text('登录二维码', style: AppTypography.cardTitle(context)),
          const SizedBox(height: AppSpacing.lg),
          Container(
            width: 200,
            height: 200,
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 1),
            ),
            child: _isConnected
                ? Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(Icons.check_circle, size: 56, color: context.success),
                      const SizedBox(height: AppSpacing.sm),
                      Text('已登录', style: AppTypography.body(context).copyWith(color: context.success)),
                    ],
                  )
                : Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(Icons.qr_code_2, size: 80, color: context.textTertiary),
                      const SizedBox(height: AppSpacing.sm),
                      Text('扫描二维码登录', style: AppTypography.caption(context)),
                    ],
                  ),
          ),
          const SizedBox(height: AppSpacing.md),
          if (!_isConnected)
            AmitiaButton(
              label: '重新生成二维码',
              icon: Icons.refresh,
              isSecondary: true,
              isFullWidth: true,
              onPressed: _regenerateQR,
            ),
        ],
      ),
    );
  }

  Widget _buildAccountInfo(ChannelConnection wechat) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('账号信息', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              _InfoLine(label: '当前账号', value: wechat.account ?? '未登录'),
              Divider(height: 1, color: context.borderSecondary),
              _InfoLine(label: '最后心跳', value: _lastHeartbeat),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildMessageStatus() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('消息状态', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        Row(
          children: [
            Expanded(
              child: _MessageStatusCard(
                icon: Icons.download,
                label: '接收消息',
                isActive: _receiving,
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: _MessageStatusCard(
                icon: Icons.upload,
                label: '发送消息',
                isActive: _sending,
              ),
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildActions() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('操作', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        Wrap(
          spacing: AppSpacing.md,
          runSpacing: AppSpacing.md,
          children: [
            AmitiaButton(
              label: '重新连接',
              icon: Icons.refresh,
              isSecondary: true,
              onPressed: _isConnected ? _showReconnectConfirm : _doReconnect,
            ),
            AmitiaButton(
              label: '重新生成二维码',
              icon: Icons.qr_code,
              isSecondary: true,
              onPressed: _regenerateQR,
            ),
            AmitiaButton(
              label: '断开连接',
              icon: Icons.link_off,
              isDestructive: true,
              onPressed: _isConnected ? _showDisconnectConfirm : null,
            ),
          ],
        ),
      ],
    );
  }

  Widget _buildLogs() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('日志摘要', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        Container(
          padding: const EdgeInsets.all(AppSpacing.cardPadding),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: _logs.take(8).map((log) {
              final isError = log.contains('失败') || log.contains('断开');
              return Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.xs),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 6,
                      height: 6,
                      margin: const EdgeInsets.only(top: 6),
                      decoration: BoxDecoration(
                        color: isError ? context.error : context.success,
                        shape: BoxShape.circle,
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Text(
                        log,
                        style: AppTypography.label(context).copyWith(
                          fontFamily: 'monospace',
                          color: isError ? context.error : context.textSecondary,
                        ),
                      ),
                    ),
                  ],
                ),
              );
            }).toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildRiskWarning() {
    return AmitiaCard(
      backgroundColor: context.warning.withValues(alpha: 0.08),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.warning_amber, size: 20, color: context.warning),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('风险提示', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600, color: context.warning)),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  '微信渠道使用网页版协议，存在被封号风险。建议仅在必要时使用，不要频繁重新连接或发送大量消息。',
                  style: AppTypography.caption(context).copyWith(color: context.warning, height: 1.5),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _InfoLine extends StatelessWidget {
  final String label;
  final String value;

  const _InfoLine({required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Text(label, style: AppTypography.caption(context)),
          const Spacer(),
          Text(value, style: AppTypography.bodySmall(context)),
        ],
      ),
    );
  }
}

class _MessageStatusCard extends StatelessWidget {
  final IconData icon;
  final String label;
  final bool isActive;

  const _MessageStatusCard({required this.icon, required this.label, required this.isActive});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(
          color: isActive ? context.success.withValues(alpha: 0.3) : context.borderPrimary,
          width: 0.5,
        ),
      ),
      child: Column(
        children: [
          Icon(icon, size: 28, color: isActive ? context.success : context.textTertiary),
          const SizedBox(height: AppSpacing.sm),
          Text(label, style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.xs),
          AmitiaStatusBadge(
            label: isActive ? '正常' : '暂停',
            type: isActive ? BadgeType.success : BadgeType.neutral,
          ),
        ],
      ),
    );
  }
}
