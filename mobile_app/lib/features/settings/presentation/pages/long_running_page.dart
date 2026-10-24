import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/core_configuration_guard.dart';
import '../../../../core/services/core_configuration_session.dart';
import '../../../../core/services/task_maintenance_contract.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class LongRunningPage extends ConsumerStatefulWidget {
  const LongRunningPage({super.key});

  @override
  ConsumerState<LongRunningPage> createState() => _LongRunningPageState();
}

class _LongRunningPageState extends ConsumerState<LongRunningPage> {
  bool _loading = true;
  bool _busy = false;
  String? _error;
  Map<String, dynamic> _status = const {};
  Map<String, dynamic> _config = const {};
  Map<String, dynamic> _modules = const {};
  Map<String, dynamic> _history = const {};
  late final CoreConfigurationSession _configuration;
  int _loadEpoch = 0;
  bool _timeoutDisabled = false;
  late final TextEditingController _maxTasks;
  late final TextEditingController _timeout;

  @override
  void initState() {
    super.initState();
    _configuration = CoreConfigurationSession(
      coreConfigurationGuardFor(ref),
      onInvalidated: (reason) {
        if (!mounted) return;
        _loadEpoch++;
        setState(() {
          _error = reason.toString();
          _loading = false;
        });
      },
    );
    _maxTasks = TextEditingController();
    _timeout = TextEditingController();
    _load();
  }

  @override
  void dispose() {
    _loadEpoch++;
    _configuration.close();
    _maxTasks.dispose();
    _timeout.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    final epoch = ++_loadEpoch;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final values = await _configuration.load(
        () => Future.wait([
          api.get<Map<String, dynamic>>('/api/runtime/tasks/status'),
          api.get<Map<String, dynamic>>('/api/runtime/tasks/config'),
          api.get<Map<String, dynamic>>('/api/runtime/modules/health'),
          api.get<Map<String, dynamic>>('/api/runtime/health-history'),
        ]),
      );
      final config = requireAppliedTaskConfig(values[1]);
      if (values[0]?['source'] != 'task-runtime' ||
          values[0]?['tasks'] is! List ||
          values[0]?['activeCount'] is! int) {
        throw StateError('任务执行状态无效');
      }
      if (!mounted || epoch != _loadEpoch) return;
      setState(() {
        _status = values[0] ?? const {};
        _config = config;
        _modules = values[2] ?? const {};
        _history = values[3] ?? const {};
        _maxTasks.text = '${_config['maxTasks'] ?? 5}';
        _timeout.text = '${_config['timeout']['seconds']}';
        _timeoutDisabled = _config['timeout']['disabled'] == true;
        _loading = false;
      });
    } catch (e) {
      if (!mounted || epoch != _loadEpoch) return;
      setState(() {
        _error = e.toString();
        _loading = false;
      });
    }
  }

  Future<void> _saveConfig() async {
    final maxTasks = int.tryParse(_maxTasks.text.trim());
    final timeout = int.tryParse(_timeout.text.trim());
    if (maxTasks == null ||
        maxTasks < 1 ||
        maxTasks > 20 ||
        timeout == null ||
        timeout < 30 ||
        timeout > 1800 ||
        timeout % 30 != 0) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('并发任务数须为 1-20，统一超时须为 30-1800 秒且为 30 秒的倍数'),
        ),
      );
      return;
    }
    await _run(
      '执行器应用配置',
      '/api/runtime/tasks/config',
      method: 'put',
      data: {
        'maxTasks': maxTasks,
        'timeout': {'disabled': _timeoutDisabled, 'seconds': timeout},
        'expectedRevision': _config['revision'],
        'expectedTimeoutRevision': _config['timeoutRevision'],
      },
    );
  }

  Future<void> _run(
    String label,
    String path, {
    String method = 'post',
    Map<String, dynamic>? data,
  }) async {
    if (_busy) return;
    final intent = _configuration.intent;
    setState(() => _busy = true);
    try {
      final api = ref.read(backendServiceProvider);
      await _configuration.write(intent, () async {
        if (method == 'put') {
          final result = await api.put<Map<String, dynamic>>(path, data: data);
          requireSavedTaskConfig(requireAppliedTaskConfig(result), data!);
        } else {
          await api.post<Map<String, dynamic>>(path, data: data);
        }
      });
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('$label完成')));
      await _load();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('$label失败：$e'),
            backgroundColor: context.error,
          ),
        );
      }
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(
      rawBackendServiceApiProvider,
      (_, __) => _configuration.invalidate(StateError('Core 连接已变化，请重新加载')),
    );
    ref.listen(
      mobileDeploymentConfigProvider,
      (_, __) => _configuration.invalidate(StateError('设备模式已变化，请重新加载')),
    );
    final tasks = _status['tasks'] is List
        ? _status['tasks'] as List
        : const [];
    final modulesRaw = _modules['modules'];
    final modules = modulesRaw is List ? modulesRaw : const [];
    final history = _history['history'] is List
        ? _history['history'] as List
        : const [];
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '长期运行维护',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [
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
          : ListView(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                AmitiaSectionHeader(title: '运行状态'),
                SizedBox(height: AppSpacing.sm),
                AmitiaCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Expanded(
                            child: Text(
                              _status['running'] == true ? '存在长期运行任务' : '当前空闲',
                              style: AppTypography.cardTitle(context),
                            ),
                          ),
                          AmitiaStatusBadge(
                            label: _status['running'] == true ? '运行中' : '空闲',
                            type: _status['running'] == true
                                ? BadgeType.success
                                : BadgeType.neutral,
                          ),
                        ],
                      ),
                      Text(
                        '执行中：${_status['activeCount']} · 本机进程：${_status['localProcessCount']} · 最近任务：${tasks.length}',
                        style: AppTypography.caption(context),
                      ),
                      ...tasks
                          .whereType<Map>()
                          .take(10)
                          .map(
                            (task) => Padding(
                              padding: EdgeInsets.only(top: AppSpacing.xs),
                              child: Text(
                                '${task['title'] ?? task['id']} · ${task['status']} · ${task['placement']} · ${task['updatedAt'] ?? ''}',
                                style: AppTypography.bodySmall(context),
                              ),
                            ),
                          ),
                    ],
                  ),
                ),
                SizedBox(height: AppSpacing.sectionGap),
                AmitiaSectionHeader(title: '运行时健康'),
                SizedBox(height: AppSpacing.sm),
                AmitiaCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '模块数：${modules.length} · 健康历史：${history.length}',
                        style: AppTypography.bodySmall(context),
                      ),
                      ...modules
                          .whereType<Map>()
                          .take(10)
                          .map(
                            (item) => Padding(
                              padding: EdgeInsets.only(top: AppSpacing.xs),
                              child: Text(
                                '${item['module'] ?? item['name'] ?? 'module'} · ${item['status'] ?? 'unknown'}',
                                style: AppTypography.caption(context),
                              ),
                            ),
                          ),
                    ],
                  ),
                ),
                SizedBox(height: AppSpacing.sectionGap),
                AmitiaSectionHeader(title: '配置'),
                SizedBox(height: AppSpacing.sm),
                AmitiaCard(
                  child: Column(
                    children: [
                      TextField(
                        controller: _maxTasks,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(
                          labelText: '最大并发任务数 (1-20)',
                        ),
                      ),
                      TextField(
                        controller: _timeout,
                        enabled: !_timeoutDisabled,
                        keyboardType: TextInputType.number,
                        decoration: const InputDecoration(
                          labelText: '统一超时/秒 (30-1800，步长30)',
                        ),
                      ),
                      SwitchListTile(
                        title: const Text('禁用统一超时'),
                        value: _timeoutDisabled,
                        onChanged: _busy
                            ? null
                            : (value) =>
                                  setState(() => _timeoutDisabled = value),
                      ),
                      const Text(
                        '并发限制管理当前服务实例，降低限制不终止已运行任务。统一超时同时影响其他操作，新建任务采用新策略。',
                      ),
                      SizedBox(height: AppSpacing.md),
                      AmitiaButton(
                        label: '保存长期运行配置',
                        isFullWidth: true,
                        onPressed: _busy ? null : _saveConfig,
                      ),
                    ],
                  ),
                ),
                SizedBox(height: AppSpacing.sectionGap),
                AmitiaSectionHeader(title: '手动维护'),
                SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '立即执行健康检查',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _busy
                      ? null
                      : () => _run('健康检查', '/api/runtime/check-now'),
                ),
                SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '清理临时文件',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _busy
                      ? null
                      : () => _run('临时文件清理', '/api/runtime/cleanup-temp'),
                ),
                SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '轮转日志',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _busy
                      ? null
                      : () => _run('日志轮转', '/api/runtime/rotate-logs'),
                ),
                SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '数据库完整性检查',
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _busy
                      ? null
                      : () =>
                            _run('数据库完整性检查', '/api/runtime/check-db-integrity'),
                ),
                SizedBox(height: AppSpacing.xl),
              ],
            ),
    );
  }
}
