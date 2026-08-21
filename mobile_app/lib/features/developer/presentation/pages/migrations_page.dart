import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class MigrationsPage extends ConsumerStatefulWidget {
  const MigrationsPage({super.key});

  @override
  ConsumerState<MigrationsPage> createState() => _MigrationsPageState();
}

class _MigrationsPageState extends ConsumerState<MigrationsPage> with SingleTickerProviderStateMixin {
  late final TabController _tabController;
  bool _loading = true;
  String? _error;
  List<Map<String, dynamic>> _migrations = [];
  List<Map<String, dynamic>> _rollbacks = [];
  List<Map<String, dynamic>> _recovery = [];
  final _canaryExtensionController = TextEditingController();
  List<Map<String, dynamic>> _canaryPolicies = [];
  List<Map<String, dynamic>> _canaryStates = [];
  List<Map<String, dynamic>> _canaryMetrics = [];
  Map<String, dynamic>? _canaryHealth;
  bool _canaryLoading = false;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 4, vsync: this);
    _load();
  }

  @override
  void dispose() {
    _tabController.dispose();
    _canaryExtensionController.dispose();
    super.dispose();
  }

  Future<Map<String, dynamic>?> _get(String path) {
    return ref.read(backendServiceProvider).get<Map<String, dynamic>>(
          path,
          fromJson: (value) => Map<String, dynamic>.from(value as Map),
        );
  }

  List<Map<String, dynamic>> _items(Map<String, dynamic>? data) {
    final raw = data?['items'] as List<dynamic>? ?? const [];
    return raw.whereType<Map>().map((item) => Map<String, dynamic>.from(item)).toList();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final results = await Future.wait([
        _get('/api/extensions/migrations'),
        _get('/api/extensions/rollbacks'),
        _get('/api/extensions/recovery/scan'),
      ]);
      if (!mounted) return;
      setState(() {
        _migrations = _items(results[0]);
        _rollbacks = _items(results[1]);
        _recovery = _items(results[2]);
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

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '迁移与恢复',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(icon: Icons.refresh, onPressed: _load, tooltip: '刷新'),
          AmitiaIconButton(icon: Icons.article_outlined, onPressed: _showOperationJournal, tooltip: '查询迁移操作与 Journal'),
          AmitiaIconButton(icon: Icons.playlist_add, onPressed: _showMigrationDialog, tooltip: '执行迁移'),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [Tab(text: '迁移定义'), Tab(text: '回滚'), Tab(text: '恢复'), Tab(text: 'Canary')],
        ),
      ),
      body: SafeArea(top: false, child: _buildBody(context)),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    return TabBarView(
      controller: _tabController,
      children: [
        _list(context, _migrations, _migrationCard, Icons.merge_type_outlined, '暂无迁移定义'),
        _list(context, _rollbacks, _rollbackCard, Icons.undo_outlined, '暂无回滚计划'),
        _list(context, _recovery, _recoveryCard, Icons.healing_outlined, '暂无待恢复操作'),
        _canaryTab(context),
      ],
    );
  }

  Widget _list(
    BuildContext context,
    List<Map<String, dynamic>> items,
    Widget Function(BuildContext, Map<String, dynamic>) builder,
    IconData icon,
    String emptyTitle,
  ) {
    if (items.isEmpty) return AmitiaEmptyState(icon: icon, title: emptyTitle);
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
        itemCount: items.length,
        itemBuilder: (context, index) => builder(context, items[index]),
      ),
    );
  }

  Widget _migrationCard(BuildContext context, Map<String, dynamic> item) {
    final id = (item['migration_id'] ?? '').toString();
    final extensionId = (item['extension_id'] ?? '').toString();
    final from = (item['from_version_range'] ?? '').toString();
    final to = (item['to_version'] ?? '').toString();
    final reversibility = (item['reversibility'] ?? '').toString();
    return _card(
      context,
      icon: Icons.merge_type_outlined,
      title: id.isEmpty ? '迁移定义' : id,
      subtitle: extensionId,
      chips: ['$from → $to', reversibility],
      raw: item,
      actions: [
        AmitiaButton(
          label: '预览计划',
          isSecondary: true,
          icon: Icons.route_outlined,
          onPressed: () => _showMigrationDialog(seed: item, planOnly: true),
        ),
        AmitiaButton(
          label: '按此定义迁移',
          isSecondary: true,
          icon: Icons.play_arrow,
          onPressed: () => _showMigrationDialog(seed: item),
        ),
      ],
    );
  }

  Widget _rollbackCard(BuildContext context, Map<String, dynamic> item) {
    final id = (item['rollback_id'] ?? item['rollbackId'] ?? item['id'] ?? '').toString();
    final extensionId = (item['extension_id'] ?? item['extensionId'] ?? '').toString();
    final status = (item['status'] ?? 'unknown').toString();
    return _card(
      context,
      icon: Icons.undo_outlined,
      title: id.isEmpty ? '回滚计划' : id,
      subtitle: extensionId,
      chips: [status],
      raw: item,
      actions: [
        AmitiaButton(
          label: '查看步骤',
          isSecondary: true,
          icon: Icons.format_list_numbered,
          onPressed: id.isEmpty ? null : () => _showRollbackSteps(id),
        ),
        AmitiaButton(
          label: '执行回滚',
          isDestructive: true,
          icon: Icons.undo,
          onPressed: id.isEmpty ? null : () => _post('/api/extensions/rollbacks/$id/execute', success: '回滚执行完成'),
        ),
        AmitiaButton(
          label: '恢复回滚',
          isSecondary: true,
          icon: Icons.restore,
          onPressed: id.isEmpty ? null : () => _post('/api/extensions/rollbacks/$id/recover', success: '回滚恢复完成'),
        ),
      ],
    );
  }

  Widget _recoveryCard(BuildContext context, Map<String, dynamic> item) {
    final operationId = (item['operation_id'] ?? item['operationId'] ?? '').toString();
    final strategy = (item['strategy'] ?? '').toString();
    final detail = (item['detail'] ?? '').toString();
    return _card(
      context,
      icon: Icons.healing_outlined,
      title: operationId.isEmpty ? '恢复操作' : operationId,
      subtitle: detail,
      chips: [if (strategy.isNotEmpty) strategy],
      raw: item,
      actions: [
        AmitiaButton(
          label: '执行恢复',
          icon: Icons.play_arrow,
          onPressed: operationId.isEmpty || strategy.isEmpty
              ? null
              : () => _post(
                    '/api/extensions/recovery/execute',
                    data: {'operationId': operationId, 'strategy': strategy, 'detail': detail},
                    success: '恢复操作已执行',
                  ),
        ),
      ],
    );
  }


  Widget _canaryTab(BuildContext context) {
    return RefreshIndicator(
      onRefresh: _loadCanary,
      child: ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text('Canary 发布控制', style: AppTypography.pageTitle(context)),
          const SizedBox(height: 4),
          Text('按 Extension ID 加载策略、当前状态、24 小时指标与健康评估。', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.md),
          Row(children: [
            Expanded(child: TextField(controller: _canaryExtensionController, decoration: const InputDecoration(labelText: 'Extension ID'))),
            const SizedBox(width: 8),
            IconButton(onPressed: _canaryLoading ? null : _loadCanary, icon: const Icon(Icons.search)),
          ]),
          SizedBox(height: AppSpacing.sm),
          Row(children: [
            Expanded(child: AmitiaButton(label: '创建策略', isSecondary: true, icon: Icons.policy_outlined, onPressed: _createCanaryPolicy)),
            SizedBox(width: AppSpacing.sm),
            Expanded(child: AmitiaButton(label: '启动 Canary', icon: Icons.rocket_launch_outlined, onPressed: _createCanaryState)),
          ]),
          SizedBox(height: AppSpacing.sm),
          AmitiaButton(label: '记录指标', isSecondary: true, icon: Icons.monitor_heart_outlined, isFullWidth: true, onPressed: _recordCanaryMetric),
          if (_canaryLoading) ...[SizedBox(height: AppSpacing.md), const LinearProgressIndicator(minHeight: 2)],
          if (_canaryHealth != null) ...[
            SizedBox(height: AppSpacing.lg),
            Text('健康评估', style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.sm),
            AmitiaCard(onTap: () => _showJson('Canary Health', _canaryHealth!), child: SelectableText(const JsonEncoder.withIndent('  ').convert(_canaryHealth), maxLines: 10)),
          ],
          SizedBox(height: AppSpacing.lg),
          Text('策略', style: AppTypography.cardTitle(context)),
          if (_canaryPolicies.isEmpty) Text('暂无已加载策略', style: AppTypography.caption(context)),
          ..._canaryPolicies.map((item) {
            final id = (item['policy_id'] ?? item['policyId'] ?? '').toString();
            return ListTile(
              contentPadding: EdgeInsets.zero,
              leading: const Icon(Icons.policy_outlined),
              title: Text(id.isEmpty ? 'Canary Policy' : id),
              subtitle: Text('${item['mode'] ?? ''} · stages=${(item['stages'] as List?)?.length ?? 0}'),
              onTap: () => _showJson('Canary Policy', item),
            );
          }),
          SizedBox(height: AppSpacing.lg),
          Text('状态', style: AppTypography.cardTitle(context)),
          if (_canaryStates.isEmpty) Text('暂无已加载 Canary 状态', style: AppTypography.caption(context)),
          ..._canaryStates.map(_canaryStateTile),
          SizedBox(height: AppSpacing.lg),
          Text('指标', style: AppTypography.cardTitle(context)),
          if (_canaryMetrics.isEmpty) Text('暂无指标', style: AppTypography.caption(context)),
          ..._canaryMetrics.take(50).map((item) => ListTile(
            contentPadding: EdgeInsets.zero,
            dense: true,
            title: Text((item['metric_name'] ?? item['metricName'] ?? 'metric').toString()),
            subtitle: Text('value=${item['metric_value'] ?? item['metricValue'] ?? ''} · samples=${item['sample_count'] ?? item['sampleCount'] ?? ''}'),
            trailing: Text((item['status'] ?? '').toString()),
            onTap: () => _showJson('Canary Metric', item),
          )),
          SizedBox(height: AppSpacing.lg),
          AmitiaButton(label: '查询 Cohort Route', isSecondary: true, icon: Icons.route_outlined, isFullWidth: true, onPressed: _queryCanaryRoute),
        ],
      ),
    );
  }

  Widget _canaryStateTile(Map<String, dynamic> item) {
    final id = (item['canary_id'] ?? item['canaryId'] ?? '').toString();
    final status = (item['status'] ?? '').toString();
    final generation = item['new_generation'] ?? item['newGeneration'] ?? 0;
    return AmitiaCard(
      onTap: () => _showJson('Canary State', item),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          const Icon(Icons.rocket_launch_outlined),
          const SizedBox(width: 10),
          Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(id, style: AppTypography.cardTitle(context)), Text('generation=$generation · stage=${item['current_stage'] ?? item['currentStage'] ?? 0}', style: AppTypography.caption(context))])),
          AmitiaStatusBadge(label: status, type: status == 'failed' || status == 'aborted' ? BadgeType.error : status == 'completed' ? BadgeType.success : status == 'paused' ? BadgeType.warning : BadgeType.info),
        ]),
        SizedBox(height: AppSpacing.md),
        Wrap(spacing: 8, runSpacing: 8, children: [
          AmitiaButton(label: '推进', isSecondary: true, icon: Icons.skip_next, onPressed: () => _canaryAction(id, 'advance', 'Canary 已推进')),
          AmitiaButton(label: status == 'paused' ? '恢复' : '暂停', isSecondary: true, icon: status == 'paused' ? Icons.play_arrow : Icons.pause, onPressed: () => _canaryAction(id, status == 'paused' ? 'resume' : 'pause', status == 'paused' ? 'Canary 已恢复' : 'Canary 已暂停')),
          AmitiaButton(label: '提交', icon: Icons.done_all, onPressed: () => _canaryAction(id, 'commit', 'Canary 已提交')),
          AmitiaButton(label: '终止', isDestructive: true, icon: Icons.stop_circle_outlined, onPressed: () => _abortCanary(id)),
          AmitiaButton(label: '健康', isSecondary: true, icon: Icons.monitor_heart_outlined, onPressed: () => _loadCanaryHealth(generation)),
        ]),
      ]),
    );
  }

  Future<void> _loadCanary() async {
    final extensionId = _canaryExtensionController.text.trim();
    if (extensionId.isEmpty) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请输入 Extension ID')));
      return;
    }
    setState(() => _canaryLoading = true);
    try {
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait([
        api.get<Map<String, dynamic>>('/api/extensions/canary/policies', queryParameters: {'extensionId': extensionId}, fromJson: (v) => Map<String, dynamic>.from(v as Map)),
        api.get<Map<String, dynamic>>('/api/extensions/canary/states', queryParameters: {'extensionId': extensionId}, fromJson: (v) => Map<String, dynamic>.from(v as Map)),
        api.get<Map<String, dynamic>>('/api/extensions/canary/metrics', queryParameters: {'extensionId': extensionId}, fromJson: (v) => Map<String, dynamic>.from(v as Map)),
      ]);
      if (!mounted) return;
      setState(() {
        _canaryPolicies = _items(results[0]);
        _canaryStates = _items(results[1]);
        _canaryMetrics = _items(results[2]);
        _canaryLoading = false;
      });
      final state = _canaryStates.isNotEmpty ? _canaryStates.first : null;
      if (state != null) await _loadCanaryHealth(state['new_generation'] ?? state['newGeneration'] ?? 0, silent: true);
    } catch (e) {
      if (!mounted) return;
      setState(() => _canaryLoading = false);
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Canary 加载失败：$e')));
    }
  }

  Future<void> _loadCanaryHealth(dynamic generation, {bool silent = false}) async {
    final extensionId = _canaryExtensionController.text.trim();
    if (extensionId.isEmpty || generation == null) return;
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        '/api/extensions/canary/health/$extensionId',
        queryParameters: {'generation': generation, 'baselineWindow': '1h'},
        fromJson: (v) => Map<String, dynamic>.from(v as Map),
      );
      if (!mounted) return;
      setState(() => _canaryHealth = data);
      if (!silent && data != null) _showJson('Canary Health', data);
    } catch (e) {
      if (!silent && mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('健康评估失败：$e')));
    }
  }

  Future<void> _canaryAction(String id, String action, String success) async {
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/canary/states/$id/$action', data: const <String, dynamic>{}, fromJson: (v) => Map<String, dynamic>.from(v as Map));
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(success)));
      await _loadCanary();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    }
  }

  Future<void> _abortCanary(String id) async {
    final reason = TextEditingController(text: 'manual_abort');
    final ok = await showDialog<bool>(context: context, builder: (context) => AlertDialog(
      title: const Text('终止 Canary'),
      content: TextField(controller: reason, decoration: const InputDecoration(labelText: 'Reason')),
      actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('终止'))],
    ));
    if (ok != true) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/canary/states/$id/abort', data: {'reason': reason.text.trim()}, fromJson: (v) => Map<String, dynamic>.from(v as Map));
      await _loadCanary();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('终止失败：$e')));
    }
  }

  Future<Map<String, dynamic>?> _editJson(String title, Map<String, dynamic> initial) async {
    final controller = TextEditingController(text: const JsonEncoder.withIndent('  ').convert(initial));
    String? error;
    return showDialog<Map<String, dynamic>>(context: context, builder: (dialogContext) => StatefulBuilder(builder: (dialogContext, setDialogState) => AlertDialog(
      title: Text(title),
      content: SizedBox(width: 720, child: Column(mainAxisSize: MainAxisSize.min, children: [
        Flexible(child: TextField(controller: controller, minLines: 14, maxLines: 24, style: const TextStyle(fontFamily: 'monospace', fontSize: 12), decoration: const InputDecoration(border: OutlineInputBorder()))),
        if (error != null) Text(error!, style: TextStyle(color: AppColors.error)),
      ])),
      actions: [TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')), FilledButton(onPressed: () {
        try { final value = jsonDecode(controller.text); if (value is! Map) throw const FormatException('顶层必须为对象'); Navigator.pop(dialogContext, Map<String, dynamic>.from(value)); }
        catch (e) { setDialogState(() => error = 'JSON 无效：$e'); }
      }, child: const Text('提交'))],
    )));
  }

  Future<void> _createCanaryPolicy() async {
    final extensionId = _canaryExtensionController.text.trim();
    final template = <String, dynamic>{
      'extension_id': extensionId,
      'mode': 'canary',
      'stages': [
        {'stage_id': 'canary-10', 'mode': 'canary', 'percentage': 10, 'min_duration': 60000000000, 'min_invocations': 10, 'auto_advance': false},
        {'stage_id': 'full', 'mode': 'full', 'percentage': 100, 'min_duration': 60000000000, 'min_invocations': 10, 'auto_advance': false},
      ],
      'cohort_key': 'invocation', 'stable_seed': extensionId, 'min_observations': 10,
      'min_duration': 60000000000, 'max_duration': 86400000000000,
      'health_policy': {'maximum_error_rate': 0.05, 'maximum_relative_error_rate': 0.02, 'maximum_p95_latency': 5000000000, 'maximum_latency_regression': 0.25, 'maximum_crash_count': 1, 'maximum_timeout_rate': 0.05, 'maximum_invalid_result_rate': 0.01, 'required_health_checks': <String>[]},
      'abort_policy': {'abort_on_signature_anomaly': true, 'abort_on_data_validation_fail': true, 'abort_on_error_rate_exceeded': true, 'abort_on_crash_exceeded': true, 'abort_on_latency_regression': true, 'abort_on_side_effect_mismatch': true, 'abort_on_permission_escalation': true, 'abort_on_scope_violation': true, 'abort_on_data_incompatible': true, 'abort_on_background_double_run': true, 'abort_on_migration_error': true},
      'write_strategy': 'single_write',
    };
    final data = await _editJson('创建 Canary Policy', template);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/canary/policies', data: data, fromJson: (v) => Map<String, dynamic>.from(v as Map));
      await _loadCanary();
    } catch (e) { if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('创建失败：$e'))); }
  }

  Future<void> _createCanaryState() async {
    final extensionId = _canaryExtensionController.text.trim();
    final policy = _canaryPolicies.isNotEmpty ? _canaryPolicies.first : <String, dynamic>{'extension_id': extensionId, 'mode': 'canary', 'stages': <dynamic>[], 'cohort_key': 'invocation'};
    final template = <String, dynamic>{'state': {'extension_id': extensionId, 'old_generation': 0, 'new_generation': 1, 'status': 'created', 'current_stage': 0}, 'policy': policy};
    final data = await _editJson('启动 Canary', template);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/canary/states', data: data, fromJson: (v) => Map<String, dynamic>.from(v as Map));
      await _loadCanary();
    } catch (e) { if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('启动失败：$e'))); }
  }

  Future<void> _recordCanaryMetric() async {
    final extensionId = _canaryExtensionController.text.trim();
    final state = _canaryStates.isNotEmpty ? _canaryStates.first : <String, dynamic>{};
    final now = DateTime.now().toUtc();
    final template = <String, dynamic>{
      'extension_id': extensionId, 'generation': state['new_generation'] ?? state['newGeneration'] ?? 0,
      'stage_id': '', 'metric_name': 'error_rate', 'metric_value': 0.0, 'sample_count': 1,
      'window_start': now.subtract(const Duration(minutes: 5)).toIso8601String(), 'window_end': now.toIso8601String(),
      'baseline_value': 0.0, 'status': 'normal',
    };
    final data = await _editJson('记录 Canary Metric', template);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/canary/metrics', data: data, fromJson: (v) => Map<String, dynamic>.from(v as Map));
      await _loadCanary();
    } catch (e) { if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('记录失败：$e'))); }
  }

  Future<void> _queryCanaryRoute() async {
    final extensionId = _canaryExtensionController.text.trim();
    if (extensionId.isEmpty) return;
    final cohortType = TextEditingController(text: 'invocation');
    final cohortId = TextEditingController();
    final ok = await showDialog<bool>(context: context, builder: (context) => AlertDialog(
      title: const Text('查询 Cohort Route'),
      content: Column(mainAxisSize: MainAxisSize.min, children: [TextField(controller: cohortType, decoration: const InputDecoration(labelText: 'Cohort Type')), TextField(controller: cohortId, decoration: const InputDecoration(labelText: 'Cohort ID'))]),
      actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('查询'))],
    ));
    if (ok != true || cohortId.text.trim().isEmpty) return;
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>('/api/extensions/canary/routes/$extensionId', queryParameters: {'cohortType': cohortType.text.trim(), 'cohortId': cohortId.text.trim()}, fromJson: (v) => Map<String, dynamic>.from(v as Map));
      if (data != null && mounted) _showJson('Generation Route', data);
    } catch (e) { if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('查询失败：$e'))); }
  }

  Widget _card(
    BuildContext context, {
    required IconData icon,
    required String title,
    required String subtitle,
    required List<String> chips,
    required Map<String, dynamic> raw,
    required List<Widget> actions,
  }) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showJson('详情', raw),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall),
                  child: Icon(icon, color: context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(title, style: AppTypography.cardTitle(context)),
                      if (subtitle.isNotEmpty)
                        Text(subtitle, maxLines: 2, overflow: TextOverflow.ellipsis, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
              ],
            ),
            if (chips.where((value) => value.isNotEmpty).isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: chips
                    .where((value) => value.isNotEmpty)
                    .map((value) => Container(
                          padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 4),
                          decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brTag),
                          child: Text(value, style: AppTypography.label(context)),
                        ))
                    .toList(),
              ),
            ],
            if (actions.isNotEmpty) ...[
              SizedBox(height: AppSpacing.md),
              Wrap(spacing: 8, runSpacing: 8, children: actions),
            ],
          ],
        ),
      ),
    );
  }

  Future<void> _post(String path, {Object? data, required String success}) async {
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        path,
        data: data ?? const <String, dynamic>{},
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(success)));
      await _load();
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString())));
    }
  }

  Future<void> _showRollbackSteps(String rollbackId) async {
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        '/api/extensions/rollbacks/$rollbackId/steps',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (data != null && mounted) _showJson('回滚步骤 · $rollbackId', data);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取回滚步骤失败：$e')));
    }
  }

  Future<void> _showOperationJournal() async {
    final controller = TextEditingController();
    final operationId = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('查询迁移操作'),
        content: TextField(controller: controller, decoration: const InputDecoration(labelText: 'Operation ID')),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, controller.text.trim()), child: const Text('查询')),
        ],
      ),
    );
    controller.dispose();
    if (operationId == null || operationId.isEmpty) return;
    try {
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait([
        api.get<dynamic>('/api/extensions/migrations/operations/$operationId'),
        api.get<dynamic>('/api/extensions/journal/$operationId'),
      ]);
      if (!mounted) return;
      _showJson('迁移操作与 Journal', <String, dynamic>{
        'operation': results[0],
        'journal': results[1],
      });
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('查询失败：$e')));
    }
  }

  Future<void> _showMigrationDialog({Map<String, dynamic>? seed, bool planOnly = false}) async {
    final extensionController = TextEditingController(text: (seed?['extension_id'] ?? '').toString());
    final fromController = TextEditingController(text: (seed?['from_version_range'] ?? '').toString());
    final toController = TextEditingController(text: (seed?['to_version'] ?? '').toString());
    final fromHashController = TextEditingController();
    final toHashController = TextEditingController(text: (seed?['definition_hash'] ?? '').toString());
    final result = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(planOnly ? '预览迁移计划' : '执行迁移'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(controller: extensionController, decoration: const InputDecoration(labelText: 'Extension ID')),
              TextField(controller: fromController, decoration: const InputDecoration(labelText: '当前版本')),
              TextField(controller: toController, decoration: const InputDecoration(labelText: '目标版本')),
              TextField(controller: fromHashController, decoration: const InputDecoration(labelText: '当前 Definition Hash（可选）')),
              TextField(controller: toHashController, decoration: const InputDecoration(labelText: '目标 Definition Hash（可选）')),
            ],
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: Text(planOnly ? '生成计划' : '执行')),
        ],
      ),
    );
    if (result != true) return;
    final extensionId = extensionController.text.trim();
    final fromVersion = fromController.text.trim();
    final toVersion = toController.text.trim();
    if (extensionId.isEmpty || fromVersion.isEmpty || toVersion.isEmpty) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Extension ID、当前版本、目标版本不能为空')));
      return;
    }
    final payload = <String, dynamic>{
      'extensionId': extensionId,
      'fromVersion': fromVersion,
      'toVersion': toVersion,
      'fromDefinitionHash': fromHashController.text.trim(),
      'toDefinitionHash': toHashController.text.trim(),
    };
    if (planOnly) {
      try {
        final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
          '/api/extensions/migrations/plan',
          data: payload,
          fromJson: (value) => Map<String, dynamic>.from(value as Map),
        );
        if (result != null && mounted) _showJson('迁移计划', result);
      } catch (e) {
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('生成计划失败：$e')));
      }
      return;
    }
    await _post('/api/extensions/migrations/execute', data: payload, success: '迁移操作已创建');
  }

  void _showJson(String title, Map<String, dynamic> data) {
    showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 520,
          child: SingleChildScrollView(
            child: SelectableText(const JsonEncoder.withIndent('  ').convert(data)),
          ),
        ),
        actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
      ),
    );
  }
}
