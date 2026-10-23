import '../../../../core/services/usage_view.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/log_folder_button.dart';

class AdvancedSystemPage extends ConsumerStatefulWidget {
  const AdvancedSystemPage({super.key});

  @override
  ConsumerState<AdvancedSystemPage> createState() => _AdvancedSystemPageState();
}

class _AdvancedSystemPageState extends ConsumerState<AdvancedSystemPage> {
  bool _loading = true;
  bool _busy = false;
  String? _error;
  Map<String, dynamic> _space = const {};
  List<dynamic> _devices = const [];
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
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/space') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(() async {
          final value =
              await api.get<Map<String, dynamic>>(
                '/api/device-mesh/v1/devices',
              ) ??
              <String, dynamic>{};
          final raw = value['devices'];
          return raw is List ? raw : <dynamic>[];
        }, <dynamic>[]),
        _safe(
          () async =>
              await api.get<List<dynamic>>('/api/audit/actions') ?? <dynamic>[],
          <dynamic>[],
        ),
        _safe(
          () async =>
              await api.get<List<dynamic>>(
                '/api/audit/logs',
                queryParameters: {'limit': 200},
              ) ??
              <dynamic>[],
          <dynamic>[],
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/audit/settings') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/audit/stats') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>(
                '/api/config/mood-detection',
              ) ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>(
                '/api/runtime/modules/health',
              ) ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>(
                '/api/runtime/health-history',
              ) ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/logs/model-errors') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/logs/files') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        readUsageView(api, '/api/usage/daily'),
        readUsageView(api, '/api/usage/models'),
        readUsageView(api, '/api/usage/sources'),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>(
                '/api/security/access-config',
              ) ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>(
                '/api/security/access-status',
              ) ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/shadow/status') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/shadow/thresholds') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
        _safe(
          () async =>
              await api.get<Map<String, dynamic>>('/api/shadow/rollbacks') ??
              <String, dynamic>{},
          <String, dynamic>{},
        ),
      ]);
      if (!mounted) return;
      setState(() {
        _space = values[0] as Map<String, dynamic>;
        _devices = values[1] as List<dynamic>;
        _auditActions = values[2] as List<dynamic>;
        _auditLogs = values[3] as List<dynamic>;
        _auditSettings = values[4] as Map<String, dynamic>;
        _auditStats = values[5] as Map<String, dynamic>;
        _mood = values[6] as Map<String, dynamic>;
        _runtimeModules = values[7] as Map<String, dynamic>;
        _runtimeHistory = values[8] as Map<String, dynamic>;
        _modelErrors = values[9] as Map<String, dynamic>;
        _logFiles = values[10] as Map<String, dynamic>;
        _usageDaily = values[11] as Map<String, dynamic>;
        _usageModels = values[12] as Map<String, dynamic>;
        _usageSources = values[13] as Map<String, dynamic>;
        _accessConfig = values[14] as Map<String, dynamic>;
        _accessStatus = values[15] as Map<String, dynamic>;
        _shadowStatus = values[16] as Map<String, dynamic>;
        _shadowThresholds = values[17] as Map<String, dynamic>;
        _shadowRollbacks = values[18] as Map<String, dynamic>;
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
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(success)));
      await _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败：$e'), backgroundColor: context.error),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _editAudit() async {
    var enabled = _auditSettings['enabled'] != false;
    var actions = _auditSettings['logActions'] != false;
    final days = TextEditingController(
      text: '${_auditSettings['retentionDays'] ?? 90}',
    );
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => AlertDialog(
          title: const Text('审计设置'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('启用审计'),
                value: enabled,
                onChanged: (v) => setLocal(() => enabled = v),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('记录动作'),
                value: actions,
                onChanged: (v) => setLocal(() => actions = v),
              ),
              TextField(
                controller: days,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(labelText: '保留天数'),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(context, {
                'enabled': enabled,
                'logActions': actions,
                'retentionDays': int.tryParse(days.text) ?? 90,
              }),
              child: const Text('保存'),
            ),
          ],
        ),
      ),
    );
    days.dispose();
    if (result == null) return;
    await _run(() async {
      await ref
          .read(backendServiceProvider)
          .put<Map<String, dynamic>>('/api/audit/settings', data: result);
    }, '审计设置已保存');
  }

  Future<void> _editAccess() async {
    var auth = _accessConfig['requireAuth'] != false;
    var rate = _accessConfig['rateLimit'] != false;
    final origins = TextEditingController(
      text: '${_accessConfig['allowedOrigins'] ?? '*'}',
    );
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => StatefulBuilder(
        builder: (context, setLocal) => AlertDialog(
          title: const Text('访问安全'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('要求认证'),
                value: auth,
                onChanged: (v) => setLocal(() => auth = v),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('限流'),
                value: rate,
                onChanged: (v) => setLocal(() => rate = v),
              ),
              TextField(
                controller: origins,
                decoration: const InputDecoration(labelText: 'Allowed Origins'),
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(context, {
                'requireAuth': auth,
                'rateLimit': rate,
                'allowedOrigins': origins.text.trim(),
              }),
              child: const Text('保存'),
            ),
          ],
        ),
      ),
    );
    origins.dispose();
    if (result == null) return;
    await _run(() async {
      await ref
          .read(backendServiceProvider)
          .put<Map<String, dynamic>>(
            '/api/security/access-config',
            data: result,
          );
    }, '访问安全设置已保存');
  }

  Future<void> _shadowAction(String action, {Map<String, dynamic>? data}) =>
      _run(() async {
        await ref
            .read(backendServiceProvider)
            .post<Map<String, dynamic>>(
              '/api/shadow/$action',
              data: data ?? const {},
            );
      }, 'Shadow Mode 操作已执行');

  String _pretty(dynamic value) =>
      const JsonEncoder.withIndent('  ').convert(value ?? const {});

  Widget _jsonCard(
    String title,
    dynamic value, {
    List<Widget> actions = const [],
  }) => Card(
    margin: EdgeInsets.only(bottom: AppSpacing.md),
    child: Padding(
      padding: EdgeInsets.all(AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  title,
                  style: Theme.of(context).textTheme.titleMedium,
                ),
              ),
              ...actions,
            ],
          ),
          const SizedBox(height: 10),
          SelectableText(
            _pretty(value),
            style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
          ),
        ],
      ),
    ),
  );

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '高级系统',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [
          const LogFolderButton(),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _load,
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
          ? Center(child: Text('加载失败：$_error'))
          : DefaultTabController(
              length: 4,
              child: Column(
                children: [
                  const TabBar(
                    isScrollable: true,
                    tabs: [
                      Tab(text: 'Space / 设备'),
                      Tab(text: '审计'),
                      Tab(text: '观测'),
                      Tab(text: 'Bridge'),
                    ],
                  ),
                  Expanded(
                    child: TabBarView(
                      children: [
                        ListView(
                          padding: EdgeInsets.all(AppSpacing.md),
                          children: [
                            _jsonCard('个人空间', _space),
                            _jsonCard('可信设备', _devices),
                          ],
                        ),
                        ListView(
                          padding: EdgeInsets.all(AppSpacing.md),
                          children: [
                            _jsonCard(
                              '审计统计',
                              {
                                'stats': _auditStats,
                                'actions': _auditActions,
                                'settings': _auditSettings,
                              },
                              actions: [
                                TextButton(
                                  onPressed: _busy ? null : _editAudit,
                                  child: const Text('设置'),
                                ),
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _run(() async {
                                          await ref
                                              .read(backendServiceProvider)
                                              .delete('/api/audit/logs');
                                        }, '审计日志已清空'),
                                  child: const Text('清空'),
                                ),
                              ],
                            ),
                            _jsonCard('审计日志', _auditLogs),
                          ],
                        ),
                        ListView(
                          padding: EdgeInsets.all(AppSpacing.md),
                          children: [
                            Card(
                              child: Padding(
                                padding: EdgeInsets.all(AppSpacing.md),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        const Expanded(
                                          child: Text('情绪历史记录'),
                                        ),
                                        Switch(
                                          value: _mood['enabled'] == true,
                                          onChanged: _busy
                                              ? null
                                              : (value) => _run(() async {
                                                  await ref
                                                      .read(
                                                        backendServiceProvider,
                                                      )
                                                      .put<Map<String, dynamic>>(
                                                        '/api/config/mood-detection',
                                                        data: {
                                                          'enabled': value,
                                                          'threshold':
                                                              _mood['threshold'] ??
                                                              .5,
                                                        },
                                                      );
                                                }, '情绪历史记录设置已更新'),
                                        ),
                                      ],
                                    ),
                                    Text(
                                      '记录阈值 ${(_mood['threshold'] ?? .5).toString()}',
                                    ),
                                    Slider(
                                      value:
                                          ((_mood['threshold'] as num?) ?? .5)
                                              .toDouble()
                                              .clamp(0.0, 1.0)
                                              .toDouble(),
                                      min: 0,
                                      max: 1,
                                      divisions: 20,
                                      onChanged: _busy
                                          ? null
                                          : (value) => _run(() async {
                                              await ref
                                                  .read(
                                                    backendServiceProvider,
                                                  )
                                                  .put<Map<String, dynamic>>(
                                                    '/api/config/mood-detection',
                                                    data: {
                                                      'enabled':
                                                          _mood['enabled'] ==
                                                          true,
                                                      'threshold': value,
                                                    },
                                                  );
                                            }, '情绪记录阈值已更新'),
                                    ),
                                    Text(
                                      '仅在情绪变化强度达到阈值时写入 Owner 情绪历史。',
                                      style: Theme.of(
                                        context,
                                      ).textTheme.bodySmall,
                                    ),
                                  ],
                                ),
                              ),
                            ),
                            _jsonCard(
                              'Shadow Mode',
                              {
                                'status': _shadowStatus,
                                'thresholds': _shadowThresholds,
                                'rollbacks': _shadowRollbacks,
                              },
                              actions: [
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _shadowAction(
                                          'start',
                                          data: const {'phase': 'interaction'},
                                        ),
                                  child: const Text('启动'),
                                ),
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _shadowAction('phase/advance'),
                                  child: const Text('推进'),
                                ),
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _shadowAction(
                                          'load-sim',
                                          data: const {
                                            'profile': 'burst',
                                            'durationSeconds': 10,
                                            'burstRate': 50,
                                            'sustainedRps': 20,
                                          },
                                        ),
                                  child: const Text('负载模拟'),
                                ),
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _shadowAction('stop'),
                                  child: const Text('停止'),
                                ),
                              ],
                            ),
                            _jsonCard('Runtime 模块健康', _runtimeModules),
                            _jsonCard('Runtime 健康历史', _runtimeHistory),
                            _jsonCard('日志文件', _logFiles),
                            _jsonCard(
                              '模型错误',
                              _modelErrors,
                              actions: [
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _run(() async {
                                          await ref
                                              .read(backendServiceProvider)
                                              .delete('/api/logs/model-errors');
                                        }, '模型错误日志已清空'),
                                  child: const Text('清空'),
                                ),
                              ],
                            ),
                            _jsonCard('Usage · 按天', _usageDaily),
                            _jsonCard('Usage · 按模型', _usageModels),
                            _jsonCard(
                              'Usage · 按来源',
                              _usageSources,
                              actions: [
                                TextButton(
                                  onPressed: _busy
                                      ? null
                                      : () => _run(() async {
                                          await ref
                                              .read(backendServiceProvider)
                                              .delete(
                                                '/api/usage/clear',
                                                queryParameters: {
                                                  'scope':
                                                      (_usageDaily['scope']
                                                              as Map?)?['deviceId'] ==
                                                          null
                                                      ? 'core'
                                                      : 'device',
                                                  'expectedCoreId':
                                                      (_usageDaily['scope']
                                                          as Map?)?['coreId'],
                                                  'expectedDeviceId':
                                                      (_usageDaily['scope']
                                                          as Map?)?['deviceId'] ??
                                                      '',
                                                },
                                              );
                                        }, '统计起点已重置，历史记录保留'),
                                  child: const Text('重置统计起点'),
                                ),
                              ],
                            ),
                          ],
                        ),
                        ListView(
                          padding: EdgeInsets.all(AppSpacing.md),
                          children: [
                            _jsonCard(
                              '访问安全',
                              {
                                'config': _accessConfig,
                                'status': _accessStatus,
                              },
                              actions: [
                                TextButton(
                                  onPressed: _busy ? null : _editAccess,
                                  child: const Text('编辑'),
                                ),
                              ],
                            ),
                          ],
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
    );
  }
}
