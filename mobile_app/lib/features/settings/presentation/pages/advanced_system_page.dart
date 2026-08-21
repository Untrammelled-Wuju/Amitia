import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class AdvancedSystemPage extends ConsumerStatefulWidget {
  const AdvancedSystemPage({super.key});

  @override
  ConsumerState<AdvancedSystemPage> createState() => _AdvancedSystemPageState();
}

class _AdvancedSystemPageState extends ConsumerState<AdvancedSystemPage> {
  bool _loading = true;
  bool _busy = false;
  String? _error;
  Map<String, dynamic> _currentSession = const {};
  List<dynamic> _loginHistory = const [];
  Map<String, dynamic> _recovery = const {};
  List<String> _newRecoveryCodes = const [];
  Map<String, dynamic> _sessionSettings = const {};
  List<dynamic> _auditLogs = const [];
  List<dynamic> _auditActions = const [];
  Map<String, dynamic> _auditSettings = const {};
  Map<String, dynamic> _auditStats = const {};
  Map<String, dynamic> _mood = const {};
  Map<String, dynamic> _runtimeModules = const {};
  Map<String, dynamic> _runtimeHistory = const {};
  Map<String, dynamic> _modelErrors = const {};
  Map<String, dynamic> _logFiles = const {};
  Map<String, dynamic> _usageDaily = const {};
  Map<String, dynamic> _usageModels = const {};
  Map<String, dynamic> _usageSources = const {};
  Map<String, dynamic> _accessConfig = const {};
  Map<String, dynamic> _accessStatus = const {};
  Map<String, dynamic> _accountCheck = const {};
  Map<String, dynamic> _wechatBridge = const {};
  Map<String, dynamic> _wechatEvents = const {};
  Map<String, dynamic> _wechatReplyTiming = const {};
  Map<String, dynamic> _qqBridge = const {};
  Map<String, dynamic> _qqEvents = const {};
  List<dynamic> _voiceSessions = const [];
  Map<String, dynamic> _shadowStatus = const {};
  Map<String, dynamic> _shadowThresholds = const {};
  Map<String, dynamic> _shadowRollbacks = const {};

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<T> _safe<T>(Future<T> Function() action, T fallback) async {
    try {
      return await action();
    } catch (_) {
      return fallback;
    }
  }

  Future<void> _load() async {
    if (!mounted) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final values = await Future.wait<dynamic>([
        _safe(() async => await api.get<Map<String, dynamic>>('/api/auth/current-session') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<List<dynamic>>('/api/auth/login-history') ?? <dynamic>[], <dynamic>[]),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/auth/recovery-codes/status') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/auth/session-settings') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<List<dynamic>>('/api/audit/actions') ?? <dynamic>[], <dynamic>[]),
        _safe(() async => await api.get<List<dynamic>>('/api/audit/logs', queryParameters: {'limit': 200}) ?? <dynamic>[], <dynamic>[]),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/audit/settings') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/audit/stats') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/config/mood-detection') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/runtime/modules/health') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/runtime/health-history') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/logs/model-errors') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/logs/files') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/usage/daily') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/usage/models') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/usage/sources') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/security/access-config') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/security/access-status') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/security/account-check') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/wechat/bridge/status-detail') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/wechat/bridge/events') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/wechat/reply-timing/status') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/qq/bridge/status-detail') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/qq/bridge/events') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/voice/sessions') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/shadow/status') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/shadow/thresholds') ?? <String, dynamic>{}, <String, dynamic>{}),
        _safe(() async => await api.get<Map<String, dynamic>>('/api/shadow/rollbacks') ?? <String, dynamic>{}, <String, dynamic>{}),
      ]);
      if (!mounted) return;
      final voice = values[24] as Map<String, dynamic>;
      setState(() {
        _currentSession = values[0] as Map<String, dynamic>;
        _loginHistory = values[1] as List<dynamic>;
        _recovery = values[2] as Map<String, dynamic>;
        _sessionSettings = values[3] as Map<String, dynamic>;
        _auditActions = values[4] as List<dynamic>;
        _auditLogs = values[5] as List<dynamic>;
        _auditSettings = values[6] as Map<String, dynamic>;
        _auditStats = values[7] as Map<String, dynamic>;
        _mood = values[8] as Map<String, dynamic>;
        _runtimeModules = values[9] as Map<String, dynamic>;
        _runtimeHistory = values[10] as Map<String, dynamic>;
        _modelErrors = values[11] as Map<String, dynamic>;
        _logFiles = values[12] as Map<String, dynamic>;
        _usageDaily = values[13] as Map<String, dynamic>;
        _usageModels = values[14] as Map<String, dynamic>;
        _usageSources = values[15] as Map<String, dynamic>;
        _accessConfig = values[16] as Map<String, dynamic>;
        _accessStatus = values[17] as Map<String, dynamic>;
        _accountCheck = values[18] as Map<String, dynamic>;
        _wechatBridge = values[19] as Map<String, dynamic>;
        _wechatEvents = values[20] as Map<String, dynamic>;
        _wechatReplyTiming = values[21] as Map<String, dynamic>;
        _qqBridge = values[22] as Map<String, dynamic>;
        _qqEvents = values[23] as Map<String, dynamic>;
        _voiceSessions = voice['sessions'] is List ? voice['sessions'] as List : const [];
        _shadowStatus = values[25] as Map<String, dynamic>;
        _shadowThresholds = values[26] as Map<String, dynamic>;
        _shadowRollbacks = values[27] as Map<String, dynamic>;
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _run(Future<void> Function() action, String success) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await action();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(success)));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e'), backgroundColor: context.error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _generateRecoveryCodes() async {
    final api = ref.read(backendServiceProvider);
    final result = await api.post<Map<String, dynamic>>('/api/auth/recovery-codes/generate', data: const {});
    final raw = result?['codes'];
    if (!mounted) return;
    setState(() => _newRecoveryCodes = raw is List ? raw.map((e) => '$e').toList() : const []);
    if (_newRecoveryCodes.isNotEmpty) {
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('新的恢复代码'),
          content: SizedBox(
            width: 420,
            child: SelectableText('这些代码只显示一次，请立即保存。\n\n${_newRecoveryCodes.join('\n')}'),
          ),
          actions: [
            TextButton(onPressed: () async { await Clipboard.setData(ClipboardData(text: _newRecoveryCodes.join('\n'))); }, child: const Text('复制')),
            TextButton(onPressed: () => Navigator.pop(context), child: const Text('完成')),
          ],
        ),
      );
    }
    await _load();
  }

  Future<void> _editSessionSettings() async {
    final timeout = TextEditingController(text: '${_sessionSettings['sessionTimeoutMinutes'] ?? 60}');
    final maxSessions = TextEditingController(text: '${_sessionSettings['maxSessionsPerUser'] ?? 10}');
    var tracking = _sessionSettings['enableDeviceTracking'] != false;
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => StatefulBuilder(builder: (context, setLocal) => AlertDialog(
        title: const Text('会话策略'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          TextField(controller: timeout, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '访问令牌时长（分钟）')),
          TextField(controller: maxSessions, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '最大会话数')),
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('设备追踪'), value: tracking, onChanged: (v) => setLocal(() => tracking = v)),
          const Text('修改后需重启后端，新的会话才会使用更新后的策略。'),
        ]),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, {'sessionTimeoutMinutes': int.tryParse(timeout.text) ?? 60, 'maxSessionsPerUser': int.tryParse(maxSessions.text) ?? 10, 'enableDeviceTracking': tracking}), child: const Text('保存'))],
      )),
    );
    timeout.dispose(); maxSessions.dispose();
    if (result == null) return;
    await _run(() async { await ref.read(backendServiceProvider).put<Map<String, dynamic>>('/api/auth/session-settings', data: result); }, '会话策略已保存，重启后端后应用');
  }

  Future<void> _editAudit() async {
    var enabled = _auditSettings['enabled'] != false;
    var actions = _auditSettings['logActions'] != false;
    final days = TextEditingController(text: '${_auditSettings['retentionDays'] ?? 90}');
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => StatefulBuilder(builder: (context, setLocal) => AlertDialog(
        title: const Text('审计设置'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('启用审计'), value: enabled, onChanged: (v) => setLocal(() => enabled = v)),
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('记录动作'), value: actions, onChanged: (v) => setLocal(() => actions = v)),
          TextField(controller: days, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '保留天数')),
        ]),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, {'enabled': enabled, 'logActions': actions, 'retentionDays': int.tryParse(days.text) ?? 90}), child: const Text('保存'))],
      )),
    );
    days.dispose(); if (result == null) return;
    await _run(() async { await ref.read(backendServiceProvider).put<Map<String, dynamic>>('/api/audit/settings', data: result); }, '审计设置已保存');
  }

  Future<void> _editAccess() async {
    var auth = _accessConfig['requireAuth'] != false;
    var rate = _accessConfig['rateLimit'] != false;
    final origins = TextEditingController(text: '${_accessConfig['allowedOrigins'] ?? '*'}');
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => StatefulBuilder(builder: (context, setLocal) => AlertDialog(
        title: const Text('访问安全'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('要求认证'), value: auth, onChanged: (v) => setLocal(() => auth = v)),
          SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('限流'), value: rate, onChanged: (v) => setLocal(() => rate = v)),
          TextField(controller: origins, decoration: const InputDecoration(labelText: 'Allowed Origins')),
        ]),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, {'requireAuth': auth, 'rateLimit': rate, 'allowedOrigins': origins.text.trim()}), child: const Text('保存'))],
      )),
    );
    origins.dispose(); if (result == null) return;
    await _run(() async { await ref.read(backendServiceProvider).put<Map<String, dynamic>>('/api/security/access-config', data: result); }, '访问安全设置已保存');
  }

  Future<void> _shadowAction(String action, {Map<String, dynamic>? data}) => _run(() async {
    await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/shadow/$action', data: data ?? const {});
  }, 'Shadow Mode 操作已执行');

  Future<void> _voiceAction(String sessionId, String action) => _run(() async {
    await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/voice/sessions/${Uri.encodeComponent(sessionId)}/$action', data: const {});
  }, 'Voice Session 操作已执行');

  String _pretty(dynamic value) => const JsonEncoder.withIndent('  ').convert(value ?? const {});

  Widget _jsonCard(String title, dynamic value, {List<Widget> actions = const []}) => Card(
    margin: EdgeInsets.only(bottom: AppSpacing.md),
    child: Padding(
      padding: EdgeInsets.all(AppSpacing.md),
      child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
        Row(children: [Expanded(child: Text(title, style: Theme.of(context).textTheme.titleMedium)), ...actions]),
        const SizedBox(height: 10),
        SelectableText(_pretty(value), style: const TextStyle(fontFamily: 'monospace', fontSize: 12)),
      ]),
    ),
  );

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '高级系统', showBackButton: true, fallbackRoute: AppRoutes.settings, actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load)]),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text('加载失败：$_error'))
              : DefaultTabController(
                  length: 4,
                  child: Column(children: [
                    const TabBar(isScrollable: true, tabs: [Tab(text: '账号'), Tab(text: '审计'), Tab(text: '观测'), Tab(text: 'Bridge')]),
                    Expanded(child: TabBarView(children: [
                      ListView(padding: EdgeInsets.all(AppSpacing.md), children: [
                        _jsonCard('当前会话', _currentSession),
                        _jsonCard('恢复代码状态', _recovery, actions: [TextButton(onPressed: _busy ? null : _generateRecoveryCodes, child: const Text('重新生成'))]),
                        _jsonCard('会话策略', _sessionSettings, actions: [TextButton(onPressed: _busy ? null : _editSessionSettings, child: const Text('编辑'))]),
                        _jsonCard('登录历史', _loginHistory),
                      ]),
                      ListView(padding: EdgeInsets.all(AppSpacing.md), children: [
                        _jsonCard('审计统计', {'stats': _auditStats, 'actions': _auditActions, 'settings': _auditSettings}, actions: [TextButton(onPressed: _busy ? null : _editAudit, child: const Text('设置')), TextButton(onPressed: _busy ? null : () => _run(() async { await ref.read(backendServiceProvider).delete('/api/audit/logs'); }, '审计日志已清空'), child: const Text('清空'))]),
                        _jsonCard('审计日志', _auditLogs),
                      ]),
                      ListView(padding: EdgeInsets.all(AppSpacing.md), children: [
                        _jsonCard('Mood Detection', _mood, actions: [Switch(value: _mood['enabled'] == true, onChanged: _busy ? null : (v) => _run(() async { await ref.read(backendServiceProvider).put<Map<String, dynamic>>('/api/config/mood-detection', data: {'enabled': v, 'threshold': _mood['threshold'] ?? .5}); }, 'Mood Detection 已更新'))]),
                        _jsonCard('Shadow Mode', {'status': _shadowStatus, 'thresholds': _shadowThresholds, 'rollbacks': _shadowRollbacks}, actions: [
                          TextButton(onPressed: _busy ? null : () => _shadowAction('start', data: const {'phase': 'interaction'}), child: const Text('启动')),
                          TextButton(onPressed: _busy ? null : () => _shadowAction('phase/advance'), child: const Text('推进')),
                          TextButton(onPressed: _busy ? null : () => _shadowAction('load-sim', data: const {'profile': 'burst', 'durationSeconds': 10, 'burstRate': 50, 'sustainedRps': 20}), child: const Text('负载模拟')),
                          TextButton(onPressed: _busy ? null : () => _shadowAction('stop'), child: const Text('停止')),
                        ]),
                        _jsonCard('Runtime 模块健康', _runtimeModules),
                        _jsonCard('Runtime 健康历史', _runtimeHistory),
                        _jsonCard('日志文件', _logFiles),
                        _jsonCard('模型错误', _modelErrors, actions: [TextButton(onPressed: _busy ? null : () => _run(() async { await ref.read(backendServiceProvider).delete('/api/logs/model-errors'); }, '模型错误日志已清空'), child: const Text('清空'))]),
                        _jsonCard('Usage · 按天', _usageDaily),
                        _jsonCard('Usage · 按模型', _usageModels),
                        _jsonCard('Usage · 按来源', _usageSources, actions: [TextButton(onPressed: _busy ? null : () => _run(() async { await ref.read(backendServiceProvider).delete('/api/usage/clear'); }, 'Usage 统计已清空'), child: const Text('清空统计'))]),
                      ]),
                      ListView(padding: EdgeInsets.all(AppSpacing.md), children: [
                        _jsonCard('访问安全', {'config': _accessConfig, 'status': _accessStatus, 'account': _accountCheck}, actions: [TextButton(onPressed: _busy ? null : _editAccess, child: const Text('编辑'))]),
                        _jsonCard('微信 Bridge', {'status': _wechatBridge, 'replyTiming': _wechatReplyTiming, 'events': _wechatEvents}, actions: [TextButton(onPressed: _busy ? null : () => _run(() async { final api=ref.read(backendServiceProvider); await api.post<Map<String,dynamic>>('/api/wechat/bridge/recover',data: const {}); await api.post<Map<String,dynamic>>('/api/wechat/reply-timing/recover',data: const {}); }, '微信 Bridge 恢复已执行'), child: const Text('恢复'))]),
                        _jsonCard('QQ Bridge', {'status': _qqBridge, 'events': _qqEvents}, actions: [TextButton(onPressed: _busy ? null : () => _run(() async { await ref.read(backendServiceProvider).post<Map<String,dynamic>>('/api/qq/bridge/recover',data: const {}); }, 'QQ Bridge 恢复已执行'), child: const Text('恢复'))]),
                        Card(margin: EdgeInsets.only(bottom: AppSpacing.md), child: Padding(padding: EdgeInsets.all(AppSpacing.md), child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Text('Voice Sessions', style: Theme.of(context).textTheme.titleMedium), const SizedBox(height: 8),
                          if (_voiceSessions.isEmpty) const Text('暂无活动 Voice Session') else for (final raw in _voiceSessions) if (raw is Map) ...[
                            ListTile(contentPadding: EdgeInsets.zero, title: Text('${raw['sessionId'] ?? ''}'), subtitle: Text('conversation=${raw['conversationId'] ?? '—'} · character=${raw['characterId'] ?? '—'}')),
                            Wrap(spacing: 6, children: [
                              TextButton(onPressed: _busy ? null : () => _voiceAction('${raw['sessionId']}', 'start'), child: const Text('启动')),
                              TextButton(onPressed: _busy ? null : () => _voiceAction('${raw['sessionId']}', 'interrupt'), child: const Text('打断')),
                              TextButton(onPressed: _busy ? null : () => _voiceAction('${raw['sessionId']}', 'wake/arm'), child: const Text('唤醒')),
                              TextButton(onPressed: _busy ? null : () => _voiceAction('${raw['sessionId']}', 'wake/disarm'), child: const Text('取消唤醒')),
                              TextButton(onPressed: _busy ? null : () => _voiceAction('${raw['sessionId']}', 'stop'), child: const Text('停止')),
                            ]), const Divider(),
                          ],
                        ]))),
                      ]),
                    ])),
                  ]),
                ),
    );
  }
}
