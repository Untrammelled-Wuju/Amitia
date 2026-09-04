import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class SchedulesPage extends ConsumerStatefulWidget {
  const SchedulesPage({super.key});

  @override
  ConsumerState<SchedulesPage> createState() => _SchedulesPageState();
}

class _SchedulesPageState extends ConsumerState<SchedulesPage> {
  bool _loading = true;
  String? _error;
  int _tab = 0;
  List<Map<String, dynamic>> _schedules = [];
  List<Map<String, dynamic>> _quarantines = [];

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (mounted) setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait([
        api.get<Map<String, dynamic>>('/api/extensions/schedules', fromJson: _map),
        api.get<Map<String, dynamic>>('/api/extensions/schedules/quarantines', fromJson: _map),
      ]);
      final schedules = (results[0]?['items'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
      final quarantines = (results[1]?['items'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList();
      if (!mounted) return;
      setState(() { _schedules = schedules; _quarantines = quarantines; _loading = false; });
    } catch (e) {
      if (!mounted) return;
      setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Map<String, dynamic> _map(dynamic value) => Map<String, dynamic>.from(value as Map);

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '调度中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(icon: Icons.add, tooltip: '创建调度', onPressed: _createSchedule),
          AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Padding(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: AmitiaSegmentedControl(
                segments: const ['调度', '隔离队列'],
                selectedIndex: _tab,
                onChanged: (value) => setState(() => _tab = value),
              ),
            ),
            Expanded(child: _body()),
          ],
        ),
      ),
    );
  }

  Widget _body() {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_tab == 1) return _quarantineList();
    if (_schedules.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.schedule_outlined,
        title: '暂无调度',
        subtitle: '可以创建 Kernel Schedule，或等待扩展安装调度贡献。',
        actionText: '创建调度',
        onAction: _createSchedule,
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.only(bottom: AppSpacing.xl),
        itemCount: _schedules.length,
        itemBuilder: (_, index) => _scheduleCard(_schedules[index]),
      ),
    );
  }

  Widget _scheduleCard(Map<String, dynamic> item) {
    final definition = item['definition'] is Map ? Map<String, dynamic>.from(item['definition'] as Map) : <String, dynamic>{};
    final state = item['state'] is Map ? Map<String, dynamic>.from(item['state'] as Map) : <String, dynamic>{};
    final id = (definition['scheduleId'] ?? state['scheduleId'] ?? '').toString();
    final name = (definition['name'] ?? id).toString();
    final status = (state['status'] ?? 'created').toString();
    final disabled = const {'disabled', 'paused', 'uninstalled', 'expired'}.contains(status.toLowerCase());
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: () => _showScheduleActions(item),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(color: disabled ? context.surfaceSecondary : context.accentSoft, borderRadius: AppRadius.brSmall),
                  child: Icon(Icons.schedule_outlined, color: disabled ? context.textTertiary : context.accentPrimary),
                ),
                const SizedBox(width: 12),
                Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(name, style: AppTypography.cardTitle(context)),
                  Text(id, style: AppTypography.label(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                ])),
                AmitiaStatusBadge(label: status, type: _statusType(status)),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            Text(
              '下次：${state['nextEffectiveAt'] ?? state['nextScheduledAt'] ?? '暂无'}  ·  最近：${state['lastFinishedAt'] ?? state['lastTriggeredAt'] ?? '暂无'}',
              style: AppTypography.caption(context),
            ),
            SizedBox(height: AppSpacing.md),
            Row(children: [
              Expanded(child: AmitiaButton(label: '立即执行', icon: Icons.play_arrow, isSecondary: true, onPressed: disabled ? null : () => _postAction(id, 'run-now', '已触发执行'))),
              SizedBox(width: AppSpacing.sm),
              Expanded(child: AmitiaButton(label: status == 'paused' ? '恢复' : '暂停', icon: status == 'paused' ? Icons.play_circle_outline : Icons.pause_circle_outline, isSecondary: true, onPressed: () => _postAction(id, status == 'paused' ? 'resume' : 'pause', status == 'paused' ? '调度已恢复' : '调度已暂停'))),
            ]),
          ],
        ),
      ),
    );
  }

  BadgeType _statusType(String status) {
    switch (status.toLowerCase()) {
      case 'enabled': return BadgeType.success;
      case 'paused': return BadgeType.warning;
      case 'quarantined': return BadgeType.error;
      case 'disabled': return BadgeType.neutral;
      default: return BadgeType.info;
    }
  }

  Widget _quarantineList() {
    if (_quarantines.isEmpty) return const AmitiaEmptyState(icon: Icons.verified_outlined, title: '没有隔离调度');
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.only(bottom: AppSpacing.xl),
        itemCount: _quarantines.length,
        itemBuilder: (_, index) {
          final item = _quarantines[index];
          return Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
            child: AmitiaCard(
              onTap: () => _showJson('隔离详情', item),
              child: Row(children: [
                Icon(Icons.gpp_bad_outlined, color: AppColors.error),
                const SizedBox(width: 12),
                Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text((item['scheduleId'] ?? item['ScheduleID'] ?? '隔离调度').toString(), style: AppTypography.cardTitle(context)),
                  Text((item['reason'] ?? item['Reason'] ?? item['detail'] ?? '').toString(), style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
                ])),
              ]),
            ),
          );
        },
      ),
    );
  }

  Future<void> _showScheduleActions(Map<String, dynamic> item) async {
    final definition = item['definition'] is Map ? Map<String, dynamic>.from(item['definition'] as Map) : <String, dynamic>{};
    final state = item['state'] is Map ? Map<String, dynamic>.from(item['state'] as Map) : <String, dynamic>{};
    final id = (definition['scheduleId'] ?? state['scheduleId'] ?? '').toString();
    final status = (state['status'] ?? '').toString();
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(sheetContext).height * .88,
          child: ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              Row(children: [Expanded(child: Text((definition['name'] ?? id).toString(), style: AppTypography.pageTitle(sheetContext))), IconButton(onPressed: () => Navigator.pop(sheetContext), icon: const Icon(Icons.close))]),
              Text(id, style: AppTypography.caption(sheetContext)),
              SizedBox(height: AppSpacing.md),
              _actionTile(Icons.code, '查看完整定义与状态', '检查 Kernel 保存的原始数据', () => _showJson('调度详情', item)),
              _actionTile(Icons.edit_outlined, '编辑定义', '通过完整 JSON 更新调度定义', () => _editSchedule(id, definition)),
              _actionTile(Icons.play_arrow, '立即执行', '绕过下一次计划时间立即触发', () => _postAction(id, 'run-now', '已触发执行')),
              _actionTile(Icons.skip_next, '跳过下一次', '将下一次触发标记为跳过', () => _postAction(id, 'skip-next', '已跳过下一次')),
              _actionTile(Icons.calculate_outlined, '重新计算', '重新计算下一次有效执行时间', () => _postAction(id, 'recalculate', '已重新计算')),
              _actionTile(status == 'enabled' ? Icons.toggle_off_outlined : Icons.toggle_on_outlined, status == 'enabled' ? '禁用' : '启用', '改变调度启用状态', () => _postAction(id, status == 'enabled' ? 'disable' : 'enable', status == 'enabled' ? '调度已禁用' : '调度已启用')),
              _actionTile(status == 'paused' ? Icons.play_circle_outline : Icons.pause_circle_outline, status == 'paused' ? '恢复' : '暂停', '暂停或恢复调度器处理', () => _postAction(id, status == 'paused' ? 'resume' : 'pause', status == 'paused' ? '调度已恢复' : '调度已暂停')),
              const Divider(),
              _actionTile(Icons.bolt_outlined, 'Triggers', '查看触发记录', () => _fetchSubresource(id, 'triggers', 'Triggers')),
              _actionTile(Icons.history, 'Runs', '查看执行历史', () => _fetchSubresource(id, 'runs', 'Runs')),
              _actionTile(Icons.warning_amber_outlined, 'Misfires', '查看错过触发记录', () => _fetchSubresource(id, 'misfires', 'Misfires')),
              _actionTile(Icons.electric_bolt_outlined, 'Circuit', '查看熔断器状态', () => _fetchSubresource(id, 'circuit', 'Circuit')),
              _actionTile(Icons.restart_alt, '重置 Circuit', '清除该调度熔断状态', () => _postAction(id, 'circuit/reset', 'Circuit 已重置')),
              const Divider(),
              _actionTile(Icons.delete_outline, '卸载调度', '删除该 Schedule Contribution', () => _deleteSchedule(id), destructive: true),
            ],
          ),
        ),
      ),
    );
  }

  Widget _actionTile(IconData icon, String title, String subtitle, VoidCallback onTap, {bool destructive = false}) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      leading: Icon(icon, color: destructive ? AppColors.error : context.accentPrimary),
      title: Text(title, style: AppTypography.body(context).copyWith(color: destructive ? AppColors.error : null)),
      subtitle: Text(subtitle, style: AppTypography.caption(context)),
      onTap: onTap,
    );
  }

  Future<void> _postAction(String id, String action, String message) async {
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/extensions/schedules/$id/$action',
        data: const <String, dynamic>{},
        fromJson: _map,
      );
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    }
  }

  Future<void> _fetchSubresource(String id, String resource, String title) async {
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        '/api/extensions/schedules/$id/$resource',
        fromJson: _map,
      );
      if (mounted) await _showJson(title, data ?? <String, dynamic>{});
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取失败：$e')));
    }
  }

  Future<void> _deleteSchedule(String id) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('卸载调度'),
        content: Text('确定卸载 $id？'),
        actions: [TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')), FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('卸载'))],
      ),
    );
    if (ok != true) return;
    await ref.read(backendServiceProvider).delete('/api/extensions/schedules/$id');
    if (mounted) Navigator.of(context).maybePop();
    await _load();
  }

  Future<void> _createSchedule() async {
    final stamp = DateTime.now().millisecondsSinceEpoch;
    final template = <String, dynamic>{
      'contributionId': 'manual.schedule.$stamp',
      'extensionId': '',
      'moduleId': 'manual',
      'scheduleId': 'schedule-$stamp',
      'name': '',
      'description': '',
      'trigger': {'type': 'cron', 'cron': {'expression': '0 * * * *', 'seconds': false}},
      'target': {'type': 'task', 'targetId': '', 'inputTemplate': <String, dynamic>{}, 'idempotencyMode': 'idempotent'},
      'timezone': 'UTC',
      'enabledByDefault': true,
      'misfirePolicy': {'policy': 'fire_once', 'maxCatchUp': 1},
      'overlapPolicy': {'policy': 'forbid'},
      'retryPolicy': {'maxAttempts': 3, 'initialBackoff': 1000000000, 'maxBackoff': 30000000000, 'multiplier': 2.0, 'jitter': 0.1},
      'jitterPolicy': {'enabled': false, 'maxDelay': 0, 'seedMode': 'schedule'},
      'concurrencyPolicy': {'maxConcurrentRuns': 1, 'perExtensionLimit': 1, 'perTargetLimit': 1},
      'scopeRule': {'scopeType': 'global'},
      'executionOwner': 'backend',
      'version': '1.0.0',
    };
    final data = await _jsonEditor('创建调度', template);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/schedules', data: data, fromJson: _map);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('调度已创建')));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('创建失败：$e')));
    }
  }

  Future<void> _editSchedule(String id, Map<String, dynamic> definition) async {
    final data = await _jsonEditor('编辑调度定义', definition);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).put<Map<String, dynamic>>('/api/extensions/schedules/$id', data: data, fromJson: _map);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('调度定义已更新')));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('更新失败：$e')));
    }
  }

  Future<Map<String, dynamic>?> _jsonEditor(String title, Map<String, dynamic> initial) async {
    final controller = TextEditingController(text: const JsonEncoder.withIndent('  ').convert(initial));
    String? validationError;
    return showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          title: Text(title),
          content: SizedBox(
            width: 720,
            child: Column(mainAxisSize: MainAxisSize.min, children: [
              Flexible(child: TextField(controller: controller, minLines: 14, maxLines: 24, keyboardType: TextInputType.multiline, style: const TextStyle(fontFamily: 'monospace', fontSize: 12), decoration: const InputDecoration(border: OutlineInputBorder()))),
              if (validationError != null) Padding(padding: const EdgeInsets.only(top: 8), child: Text(validationError!, style: TextStyle(color: AppColors.error))),
            ]),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
            FilledButton(onPressed: () {
              try {
                final decoded = jsonDecode(controller.text);
                if (decoded is! Map) throw const FormatException('顶层必须是 JSON 对象');
                Navigator.pop(dialogContext, Map<String, dynamic>.from(decoded));
              } catch (e) {
                setDialogState(() => validationError = 'JSON 无效：$e');
              }
            }, child: const Text('提交')),
          ],
        ),
      ),
    );
  }

  Future<void> _showJson(String title, Map<String, dynamic> data) async {
    final text = const JsonEncoder.withIndent('  ').convert(data);
    if (!mounted) return;
    await showDialog<void>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: SizedBox(width: 720, child: SingleChildScrollView(child: SelectableText(text, style: const TextStyle(fontFamily: 'monospace', fontSize: 12)))),
        actions: [
          TextButton(onPressed: () async { await Clipboard.setData(ClipboardData(text: text)); if (context.mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已复制'))); }, child: const Text('复制')),
          FilledButton(onPressed: () => Navigator.pop(context), child: const Text('关闭')),
        ],
      ),
    );
  }
}
