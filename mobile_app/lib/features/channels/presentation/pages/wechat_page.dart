import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

enum _WechatStatus { connected, disconnected, connecting, expired }

class WechatPage extends ConsumerStatefulWidget {
  const WechatPage({super.key});

  @override
  ConsumerState<WechatPage> createState() => _WechatPageState();
}

class _WechatPageState extends ConsumerState<WechatPage> {
  _WechatStatus _status = _WechatStatus.disconnected;
  bool _loading = true;
  bool _operating = false;
  Map<String, dynamic> _statusData = const {};
  Map<String, dynamic> _qrData = const {};
  List<Map<String, dynamic>> _events = const [];
  Map<String, dynamic> _riskData = const {};

  @override
  void initState() {
    super.initState();
    Future.microtask(_loadAll);
  }

  bool get _isConnected => _status == _WechatStatus.connected;

  Future<void> _loadAll() async {
    if (mounted) setState(() => _loading = true);
    final svc = ref.read(wechatServiceProvider);
    Map<String, dynamic>? status;
    Map<String, dynamic>? qr;
    Map<String, dynamic>? risk;
    List<Map<String, dynamic>> events = const [];
    Object? failure;
    try {
      status = await svc.status();
      try {
        qr = await svc.qrCode();
      } catch (_) {}
      try {
        events = await svc.events();
      } catch (_) {}
      try {
        risk = await svc.riskSummary();
      } catch (_) {}
    } catch (e) {
      failure = e;
    }
    if (!mounted) return;
    setState(() {
      _statusData = status ?? const {};
      _qrData = qr ?? const {};
      _events = events;
      _riskData = risk ?? const {};
      _status = _parseStatus(_statusData);
      _loading = false;
    });
    if (failure != null) _snack('读取微信状态失败：${_message(failure)}');
  }

  _WechatStatus _parseStatus(Map<String, dynamic> data) {
    if (data['connected'] == true) return _WechatStatus.connected;
    final raw = (data['status'] ?? '').toString().toLowerCase();
    if (raw == 'connected' || raw == 'ready' || raw == 'online') return _WechatStatus.connected;
    if (raw.contains('connect') || raw == 'starting' || raw == 'waiting') return _WechatStatus.connecting;
    if (raw.contains('expired') || raw.contains('invalid')) return _WechatStatus.expired;
    return _WechatStatus.disconnected;
  }

  Future<void> _operate(Future<Map<String, dynamic>?> Function() action, String success) async {
    if (_operating) return;
    setState(() => _operating = true);
    try {
      await action();
      _snack(success, color: context.accentPrimary);
      await _loadAll();
    } catch (e) {
      _snack('操作失败：${_message(e)}', color: context.error);
    } finally {
      if (mounted) setState(() => _operating = false);
    }
  }

  Future<void> _startLogin() async {
    setState(() => _status = _WechatStatus.connecting);
    await _operate(ref.read(wechatServiceProvider).startLogin, '登录流程已启动');
  }

  Future<void> _rescan() async {
    await _operate(ref.read(wechatServiceProvider).rescan, '已重新生成登录二维码');
  }

  Future<void> _reconnect() async {
    await _operate(ref.read(wechatServiceProvider).reconnect, '已请求重新连接微信');
  }

  Future<void> _recover() async {
    await _operate(ref.read(wechatServiceProvider).recoverBridge, '微信桥接恢复已执行');
  }

  Future<void> _runCloudCheck() async {
    await _operate(ref.read(wechatServiceProvider).runCloudCheck, '云端风险检查已启动');
  }

  void _snack(String message, {Color? color}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), duration: const Duration(seconds: 2), backgroundColor: color),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '微信连接',
        showBackButton: true,
        fallbackRoute: AppRoutes.channels,
        actions: [
          IconButton(onPressed: _loading || _operating ? null : _loadAll, icon: const Icon(Icons.refresh)),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : RefreshIndicator(
              onRefresh: _loadAll,
              child: ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  _buildStatusCard(),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildQRCard(),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildAccountInfo(),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildMessageStatus(),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildActions(),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildLogs(),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildRiskWarning(),
                ],
              ),
            ),
    );
  }

  Widget _buildStatusCard() {
    return AmitiaCard(
      child: Row(
        children: [
          Container(
            width: 52,
            height: 52,
            decoration: BoxDecoration(
              color: (_isConnected ? context.success : context.warning).withValues(alpha: 0.12),
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(_isConnected ? Icons.wechat : Icons.wechat_outlined,
                size: 28, color: _isConnected ? context.success : context.warning),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('微信渠道', style: AppTypography.cardTitle(context)),
                Text(
                  _statusData['running'] == true ? '微信桥接服务运行中' : '微信桥接服务未运行或不可达',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          AmitiaStatusBadge(label: _statusLabel(), type: _badgeType()),
        ],
      ),
    );
  }

  Widget _buildQRCard() {
    final image = _qrImage();
    return AmitiaCard(
      child: Column(
        children: [
          Text('登录二维码', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.lg),
          Container(
            width: 210,
            height: 210,
            padding: const EdgeInsets.all(8),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary),
            ),
            child: _isConnected
                ? Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(Icons.check_circle, size: 56, color: context.success),
                      SizedBox(height: AppSpacing.sm),
                      Text('已登录', style: AppTypography.body(context).copyWith(color: context.success)),
                    ],
                  )
                : image ??
                    Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(Icons.qr_code_2, size: 76, color: context.textTertiary),
                        SizedBox(height: AppSpacing.sm),
                        Text('启动登录后显示二维码', style: AppTypography.caption(context)),
                      ],
                    ),
          ),
          SizedBox(height: AppSpacing.md),
          if (!_isConnected)
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: '启动登录',
                    icon: Icons.login,
                    isSecondary: true,
                    onPressed: _operating ? null : _startLogin,
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '刷新二维码',
                    icon: Icons.refresh,
                    isSecondary: true,
                    onPressed: _operating ? null : _rescan,
                  ),
                ),
              ],
            ),
        ],
      ),
    );
  }

  Widget? _qrImage() {
    final raw = _findString(_qrData, const ['qrCode', 'qrcode', 'qr', 'qrUrl', 'qrcodeUrl', 'url', 'image']);
    if (raw == null || raw.isEmpty) return null;
    if (raw.startsWith('data:image/')) {
      final comma = raw.indexOf(',');
      if (comma > 0) {
        try {
          final Uint8List bytes = base64Decode(raw.substring(comma + 1));
          return Image.memory(bytes, fit: BoxFit.contain, errorBuilder: (_, __, ___) => const SizedBox.shrink());
        } catch (_) {}
      }
    }
    if (raw.startsWith('http://') || raw.startsWith('https://')) {
      return Image.network(raw, fit: BoxFit.contain, cacheWidth: 360, errorBuilder: (_, __, ___) => const SizedBox.shrink());
    }
    try {
      final bytes = base64Decode(raw);
      return Image.memory(bytes, fit: BoxFit.contain, errorBuilder: (_, __, ___) => const SizedBox.shrink());
    } catch (_) {
      return null;
    }
  }

  Widget _buildAccountInfo() {
    final account = _findString(_statusData, const ['nickname', 'nickName', 'account', 'username', 'userName', 'wxid']) ?? '未登录';
    final heartbeat = _findString(_statusData, const ['lastHeartbeat', 'heartbeatAt', 'updatedAt']) ?? '未知';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('账号信息', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              _InfoLine(label: '当前账号', value: account),
              Divider(height: 1, color: context.borderSecondary),
              _InfoLine(label: '连接状态', value: _statusLabel()),
              Divider(height: 1, color: context.borderSecondary),
              _InfoLine(label: '最近心跳', value: heartbeat),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildMessageStatus() {
    final messageCount = (_statusData['messageCount'] ?? 0).toString();
    final replyCount = (_statusData['replyCount'] ?? 0).toString();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('消息状态', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              _InfoLine(label: '收到消息', value: messageCount),
              Divider(height: 1, color: context.borderSecondary),
              _InfoLine(label: '已回复', value: replyCount),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildActions() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('连接操作', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '重新连接',
                icon: Icons.sync,
                isSecondary: true,
                onPressed: _operating ? null : _reconnect,
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '恢复桥接',
                icon: Icons.build_circle_outlined,
                isSecondary: true,
                onPressed: _operating ? null : _recover,
              ),
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
        Text('最近事件', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: _events.isEmpty
              ? Text('暂无微信事件', style: AppTypography.caption(context))
              : Column(
                  children: _events.take(10).map((event) {
                    final title = _findString(event, const ['message', 'type', 'event', 'status']) ?? jsonEncode(event);
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 5),
                      child: Align(
                        alignment: Alignment.centerLeft,
                        child: Text(title, style: AppTypography.caption(context)),
                      ),
                    );
                  }).toList(),
                ),
        ),
      ],
    );
  }

  Widget _buildRiskWarning() {
    final risks = _riskData['risks'];
    final riskCount = risks is List ? risks.length : 0;
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(riskCount == 0 ? Icons.verified_user_outlined : Icons.warning_amber_rounded,
                  color: riskCount == 0 ? context.success : context.warning),
              SizedBox(width: AppSpacing.sm),
              Expanded(
                child: Text(
                  riskCount == 0 ? '未发现已报告的连接风险' : '检测到 $riskCount 项连接风险',
                  style: AppTypography.bodySmall(context),
                ),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          AmitiaButton(
            label: '运行云端风险检查',
            isSecondary: true,
            isFullWidth: true,
            onPressed: _operating ? null : _runCloudCheck,
          ),
        ],
      ),
    );
  }

  String _statusLabel() {
    switch (_status) {
      case _WechatStatus.connected:
        return '已连接';
      case _WechatStatus.disconnected:
        return '未连接';
      case _WechatStatus.connecting:
        return '连接中';
      case _WechatStatus.expired:
        return '已过期';
    }
  }

  BadgeType _badgeType() {
    switch (_status) {
      case _WechatStatus.connected:
        return BadgeType.success;
      case _WechatStatus.disconnected:
        return BadgeType.neutral;
      case _WechatStatus.connecting:
        return BadgeType.warning;
      case _WechatStatus.expired:
        return BadgeType.error;
    }
  }

  static String? _findString(Map<String, dynamic> map, List<String> keys) {
    for (final key in keys) {
      final value = map[key];
      if (value != null && value.toString().trim().isNotEmpty) return value.toString();
    }
    final data = map['data'];
    if (data is Map) return _findString(Map<String, dynamic>.from(data), keys);
    return null;
  }

  static String _message(Object error) => error.toString().replaceFirst('Exception: ', '');
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
          Expanded(child: Text(label, style: AppTypography.bodySmall(context))),
          const SizedBox(width: 12),
          Flexible(child: Text(value, textAlign: TextAlign.right, style: AppTypography.caption(context))),
        ],
      ),
    );
  }
}
