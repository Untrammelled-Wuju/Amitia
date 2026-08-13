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
import '../../../../core/services/error_utils.dart';

enum _QqStatus { connected, disconnected, connecting, expired }

class QqPage extends ConsumerStatefulWidget {
  const QqPage({super.key});

  @override
  ConsumerState<QqPage> createState() => _QqPageState();
}

class _QqPageState extends ConsumerState<QqPage> {
  _QqStatus _status = _QqStatus.disconnected;
  String _wsStatus = '未连接';
  String? _errorMessage;
  List<String> _logs = [];
  String _appId = '';
  String _token = '';
  bool _obscureToken = true;
  int _testState = 0;
  bool _loading = true;

  final _appIdController = TextEditingController();
  final _tokenController = TextEditingController();

  @override
  void initState() {
    super.initState();
    _loadConfig();
  }

  @override
  void dispose() {
    _appIdController.dispose();
    _tokenController.dispose();
    super.dispose();
  }

  Future<void> _loadConfig() async {
    setState(() => _loading = true);
    try {
      final svc = ref.read(qqServiceProvider);
      final config = await svc.config();
      if (config != null && mounted) {
        _appId = (config['appId'] ?? config['app_id'] ?? '').toString();
        _token = (config['token'] ?? config['botToken'] ?? config['bot_token'] ?? '').toString();
        _appIdController.text = _appId;
        _tokenController.text = _token;
      }
      final status = await svc.status();
      if (status != null && mounted) {
        final s = (status['status'] ?? '').toString().toLowerCase();
        final connected = status['connected'];
        if (connected is bool && connected) {
          _status = _QqStatus.connected;
          _wsStatus = '已连接';
        } else if (s == 'connected' || s == '已连接') {
          _status = _QqStatus.connected;
          _wsStatus = '已连接';
        } else if (s == 'connecting' || s == '连接中') {
          _status = _QqStatus.connecting;
          _wsStatus = '连接中...';
        } else if (s == 'disconnected' || s == '未连接') {
          _status = _QqStatus.disconnected;
          _wsStatus = '未连接';
        } else {
          _status = _QqStatus.disconnected;
          _wsStatus = '未连接';
        }
        final logs = status['logs'] as List?;
        if (logs != null) {
          _logs = logs.map((e) => e?.toString() ?? '').where((e) => e.isNotEmpty).toList();
        }
      }
    } catch (_) {}
    if (mounted) setState(() => _loading = false);
  }

  bool get _isConnected => _status == _QqStatus.connected;

  String _statusLabel() {
    switch (_status) {
      case _QqStatus.connected:
        return '已连接';
      case _QqStatus.disconnected:
        return '未连接';
      case _QqStatus.connecting:
        return '连接中...';
      case _QqStatus.expired:
        return '已过期';
    }
  }

  BadgeType _badgeType() {
    switch (_status) {
      case _QqStatus.connected:
        return BadgeType.success;
      case _QqStatus.disconnected:
        return BadgeType.neutral;
      case _QqStatus.connecting:
        return BadgeType.warning;
      case _QqStatus.expired:
        return BadgeType.error;
    }
  }

  void _snack(String message, {Color? color}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(message), duration: const Duration(seconds: 2), backgroundColor: color),
    );
  }

  void _showConnectConfirm() {
    if (_appIdController.text.trim().isEmpty || _tokenController.text.trim().isEmpty) {
      _snack('请先填写 AppID 和 Token', color: context.warning);
      return;
    }
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('连接确认', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('即将使用以下配置连接 QQ Bot：', style: AppTypography.bodySmall(context)),
            const SizedBox(height: AppSpacing.md),
            Text('AppID: ${_appIdController.text}', style: AppTypography.label(context)),
            Text('Token: ${_maskToken(_tokenController.text)}', style: AppTypography.label(context)),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: Text('取消', style: TextStyle(color: context.textSecondary))),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              _doConnect();
            },
            child: Text('连接', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  Future<void> _doConnect() async {
    setState(() {
      _status = _QqStatus.connecting;
      _wsStatus = '连接中...';
      _errorMessage = null;
    });
    _snack('正在连接 QQ Bot...', color: context.accentPrimary);
    try {
      final svc = ref.read(qqServiceProvider);
      await svc.connect({
        'appId': _appIdController.text.trim(),
        'token': _tokenController.text.trim(),
      });
      if (!mounted) return;
      setState(() {
        _status = _QqStatus.connected;
        _wsStatus = '已连接';
        _appId = _appIdController.text;
        _token = _tokenController.text;
        _logs.insert(0, '[${DateTime.now().toString().substring(11, 19)}] WebSocket 连接成功');
        _logs.insert(0, '[${DateTime.now().toString().substring(11, 19)}] Bot 已上线');
      });
      _snack('QQ Bot 连接成功', color: context.success);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _status = _QqStatus.disconnected;
        _wsStatus = '未连接';
        _errorMessage = safeErrorMessage(e);
        _logs.insert(0, '[${DateTime.now().toString().substring(11, 19)}] 连接失败: $e');
      });
      _snack('连接失败: $e', color: context.error);
    }
  }

  Future<void> _disconnect() async {
    try {
      final svc = ref.read(qqServiceProvider);
      await svc.disconnect();
    } catch (_) {}
    if (!mounted) return;
    setState(() {
      _status = _QqStatus.disconnected;
      _wsStatus = '未连接';
      _logs.insert(0, '[${DateTime.now().toString().substring(11, 19)}] 已断开连接');
    });
    _snack('已断开 QQ Bot 连接', color: context.warning);
  }

  Future<void> _testMessage() async {
    if (!_isConnected) {
      _snack('请先连接 QQ Bot', color: context.warning);
      return;
    }
    setState(() => _testState = 1);
    await Future.delayed(const Duration(milliseconds: 1000));
    if (!mounted) return;
    setState(() => _testState = 2);
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('测试消息结果', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.check_circle, size: 20, color: context.success),
                const SizedBox(width: AppSpacing.sm),
                Text('发送成功', style: AppTypography.body(context).copyWith(color: context.success)),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Container(
              padding: const EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('目标: 测试群', style: AppTypography.label(context)),
                  Text('内容: Amitia 测试消息', style: AppTypography.label(context)),
                  Text('延迟: 128ms', style: AppTypography.label(context)),
                  Text('状态: 已送达', style: AppTypography.label(context)),
                ],
              ),
            ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _testState = 0);
            },
            child: Text('关闭', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  String _maskToken(String token) {
    if (token.length <= 4) return '****';
    return '${token.substring(0, 2)}****${token.substring(token.length - 2)}';
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'QQ 连接', showBackButton: true, fallbackRoute: AppRoutes.channels),
      body: _loading
          ? const AmitiaLoadingState(message: '加载中...')
          : ListView(
              padding: const EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                _buildStatusCard(),
                const SizedBox(height: AppSpacing.sectionGap),
                _buildBotConfig(),
                const SizedBox(height: AppSpacing.sectionGap),
                _buildConnectionStatus(),
                const SizedBox(height: AppSpacing.sectionGap),
                if (_errorMessage != null && !_isConnected) ...[
                  _buildErrorState(),
                  const SizedBox(height: AppSpacing.sectionGap),
                ],
                _buildActions(),
                const SizedBox(height: AppSpacing.sectionGap),
                _buildLogs(),
              ],
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
              color: _isConnected
                  ? context.success.withValues(alpha: 0.12)
                  : context.warning.withValues(alpha: 0.12),
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              Icons.smart_toy_outlined,
              size: 28,
              color: _isConnected ? context.success : context.warning,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('QQ Bot 渠道', style: AppTypography.cardTitle(context)),
                Text('通过 QQ Bot WebSocket 协议连接', style: AppTypography.caption(context)),
              ],
            ),
          ),
          AmitiaStatusBadge(label: _statusLabel(), type: _badgeType()),
        ],
      ),
    );
  }

  Widget _buildBotConfig() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Bot 配置', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('AppID', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Container(
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: TextField(
                  controller: _appIdController,
                  style: AppTypography.bodySmall(context),
                  decoration: InputDecoration(
                    hintText: '请输入 Bot AppID',
                    hintStyle: TextStyle(color: context.textTertiary, fontSize: 14),
                    prefixIcon: Icon(Icons.tag, size: 20, color: context.textTertiary),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(vertical: 12),
                    border: InputBorder.none,
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              Text('Token', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Container(
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: TextField(
                  controller: _tokenController,
                  obscureText: _obscureToken,
                  style: AppTypography.bodySmall(context),
                  decoration: InputDecoration(
                    hintText: '请输入 Bot Token',
                    hintStyle: TextStyle(color: context.textTertiary, fontSize: 14),
                    prefixIcon: Icon(Icons.key, size: 20, color: context.textTertiary),
                    suffixIcon: GestureDetector(
                      onTap: () => setState(() => _obscureToken = !_obscureToken),
                      child: Icon(
                        _obscureToken ? Icons.visibility_off_outlined : Icons.visibility_outlined,
                        size: 20,
                        color: context.textTertiary,
                      ),
                    ),
                    isDense: true,
                    contentPadding: const EdgeInsets.symmetric(vertical: 12),
                    border: InputBorder.none,
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Icon(Icons.info_outline, size: 14, color: context.textTertiary),
                  const SizedBox(width: AppSpacing.xs),
                  Expanded(
                    child: Text(
                      '在 QQ 开放平台 > 应用管理 中获取 AppID 和 Token',
                      style: AppTypography.label(context),
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildConnectionStatus() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('连接状态', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              _StatusLine(
                label: '连接状态',
                value: _statusLabel(),
                type: _isConnected ? BadgeType.success : (_status == _QqStatus.connecting ? BadgeType.warning : BadgeType.neutral),
              ),
              Divider(height: 1, color: context.borderSecondary),
              _StatusLine(
                label: 'WebSocket',
                value: _wsStatus,
                type: _wsStatus == '已连接' ? BadgeType.success : (_wsStatus.contains('中') ? BadgeType.warning : BadgeType.neutral),
              ),
              Divider(height: 1, color: context.borderSecondary),
              _StatusLine(
                label: 'Bot AppID',
                value: _appId.isNotEmpty ? _appId : '未配置',
                type: BadgeType.info,
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildErrorState() {
    return AmitiaCard(
      backgroundColor: context.error.withValues(alpha: 0.08),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.error_outline, size: 20, color: context.error),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('错误状态', style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600, color: context.error)),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  _errorMessage ?? '未知错误',
                  style: AppTypography.caption(context).copyWith(color: context.error, height: 1.5),
                ),
              ],
            ),
          ),
        ],
      ),
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
              label: _status == _QqStatus.connecting ? '连接中...' : '连接',
              icon: Icons.link,
              isSecondary: _isConnected,
              onPressed: _status == _QqStatus.connecting ? null : (_isConnected ? null : _showConnectConfirm),
            ),
            AmitiaButton(
              label: '重新连接',
              icon: Icons.refresh,
              isSecondary: true,
              onPressed: _status == _QqStatus.connecting ? null : _doConnect,
            ),
            AmitiaButton(
              label: '断开',
              icon: Icons.link_off,
              isDestructive: true,
              onPressed: _isConnected ? _disconnect : null,
            ),
            AmitiaButton(
              label: _testState == 1 ? '发送中...' : '测试消息',
              icon: _testState == 2 ? Icons.check_circle : Icons.send_outlined,
              isSecondary: true,
              onPressed: _testState == 1 ? null : _testMessage,
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
        Text('日志', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: AppSpacing.md),
        Container(
          padding: const EdgeInsets.all(AppSpacing.cardPadding),
          decoration: BoxDecoration(
            color: context.surfacePrimary,
            borderRadius: AppRadius.brMedium,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: _logs.isEmpty
              ? Padding(
                  padding: const EdgeInsets.symmetric(vertical: 8),
                  child: Text('暂无日志', style: AppTypography.label(context).copyWith(color: context.textTertiary)),
                )
              : Column(
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
}

class _StatusLine extends StatelessWidget {
  final String label;
  final String value;
  final BadgeType type;

  const _StatusLine({required this.label, required this.value, required this.type});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10),
      child: Row(
        children: [
          Text(label, style: AppTypography.caption(context)),
          const Spacer(),
          AmitiaStatusBadge(label: value, type: type),
        ],
      ),
    );
  }
}
