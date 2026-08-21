import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
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
  late final TextEditingController _maxTasks;
  late final TextEditingController _timeout;

  @override
  void initState() {
    super.initState();
    _maxTasks = TextEditingController();
    _timeout = TextEditingController();
    _load();
  }

  @override
  void dispose() {
    _maxTasks.dispose();
    _timeout.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final values = await Future.wait([
        api.get<Map<String, dynamic>>('/api/runtime/long-running/status'),
        api.get<Map<String, dynamic>>('/api/runtime/long-running/config'),
        api.get<Map<String, dynamic>>('/api/runtime/modules/health'),
        api.get<Map<String, dynamic>>('/api/runtime/health-history'),
      ]);
      if (!mounted) return;
      setState(() {
        _status = values[0] ?? const {};
        _config = values[1] ?? const {};
        _modules = values[2] ?? const {};
        _history = values[3] ?? const {};
        _maxTasks.text = '${_config['maxTasks'] ?? 5}';
        _timeout.text = '${_config['timeoutMinutes'] ?? 30}';
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

  Future<void> _saveConfig() async {
    final maxTasks = int.tryParse(_maxTasks.text.trim());
    final timeout = int.tryParse(_timeout.text.trim());
    if (maxTasks == null || maxTasks < 1 || maxTasks > 20 || timeout == null || timeout < 5 || timeout > 120) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('最大任务数需为 1-20，超时时间需为 5-120 分钟')));
      return;
    }
    await _run('保存配置', '/api/runtime/long-running/config', method: 'put', data: {'maxTasks': maxTasks, 'timeoutMinutes': timeout});
  }

  Future<void> _run(String label, String path, {String method = 'post', Map<String, dynamic>? data}) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      final api = ref.read(backendServiceProvider);
      if (method == 'put') {
        await api.put<Map<String, dynamic>>(path, data: data);
      } else {
        await api.post<Map<String, dynamic>>(path, data: data);
      }
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$label完成')));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$label失败：$e'), backgroundColor: context.error));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final tasks = _status['tasks'] is List ? _status['tasks'] as List : const [];
    final modulesRaw = _modules['modules'];
    final modules = modulesRaw is List ? modulesRaw : const [];
    final history = _history['history'] is List ? _history['history'] as List : const [];
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '长期运行维护', showBackButton: true, fallbackRoute: AppRoutes.settings, actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load)]),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text('加载失败：$_error'))
              : ListView(
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  children: [
                    AmitiaSectionHeader(title: '运行状态'),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaCard(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Row(children: [Expanded(child: Text(_status['running'] == true ? '存在长期运行任务' : '当前空闲', style: AppTypography.cardTitle(context))), AmitiaStatusBadge(label: _status['running'] == true ? '运行中' : '空闲', type: _status['running'] == true ? BadgeType.success : BadgeType.neutral)]),
                      Text('任务数：${tasks.length}', style: AppTypography.caption(context)),
                      ...tasks.whereType<Map>().take(10).map((task) => Padding(padding: EdgeInsets.only(top: AppSpacing.xs), child: Text('${task['title'] ?? '未命名任务'} · ${task['updated_at'] ?? task['updatedAt'] ?? ''}', style: AppTypography.bodySmall(context)))),
                    ])),
                    SizedBox(height: AppSpacing.sectionGap),
                    AmitiaSectionHeader(title: '运行时健康'),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaCard(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Text('模块数：${modules.length} · 健康历史：${history.length}', style: AppTypography.bodySmall(context)),
                      ...modules.whereType<Map>().take(10).map((item) => Padding(padding: EdgeInsets.only(top: AppSpacing.xs), child: Text('${item['module'] ?? item['name'] ?? 'module'} · ${item['status'] ?? 'unknown'}', style: AppTypography.caption(context)))),
                    ])),
                    SizedBox(height: AppSpacing.sectionGap),
                    AmitiaSectionHeader(title: '配置'),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaCard(child: Column(children: [
                      TextField(controller: _maxTasks, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '最大任务数 (1-20)')),
                      TextField(controller: _timeout, keyboardType: TextInputType.number, decoration: const InputDecoration(labelText: '超时时间/分钟 (5-120)')),
                      SizedBox(height: AppSpacing.md),
                      AmitiaButton(label: '保存长期运行配置', isFullWidth: true, onPressed: _busy ? null : _saveConfig),
                    ])),
                    SizedBox(height: AppSpacing.sectionGap),
                    AmitiaSectionHeader(title: '手动维护'),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaButton(label: '立即执行健康检查', isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('健康检查', '/api/runtime/check-now')),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaButton(label: '清理临时文件', isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('临时文件清理', '/api/runtime/cleanup-temp')),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaButton(label: '轮转日志', isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('日志轮转', '/api/runtime/rotate-logs')),
                    SizedBox(height: AppSpacing.sm),
                    AmitiaButton(label: '数据库完整性检查', isFullWidth: true, isSecondary: true, onPressed: _busy ? null : () => _run('数据库完整性检查', '/api/runtime/check-db-integrity')),
                    SizedBox(height: AppSpacing.xl),
                  ],
                ),
    );
  }
}
