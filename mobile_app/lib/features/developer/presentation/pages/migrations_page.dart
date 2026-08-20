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

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _load();
  }

  @override
  void dispose() {
    _tabController.dispose();
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
          AmitiaIconButton(icon: Icons.playlist_add, onPressed: _showMigrationDialog, tooltip: '执行迁移'),
        ],
        bottom: TabBar(
          controller: _tabController,
          tabs: const [Tab(text: '迁移定义'), Tab(text: '回滚'), Tab(text: '恢复')],
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

  Future<void> _showMigrationDialog({Map<String, dynamic>? seed}) async {
    final extensionController = TextEditingController(text: (seed?['extension_id'] ?? '').toString());
    final fromController = TextEditingController(text: (seed?['from_version_range'] ?? '').toString());
    final toController = TextEditingController(text: (seed?['to_version'] ?? '').toString());
    final fromHashController = TextEditingController();
    final toHashController = TextEditingController(text: (seed?['definition_hash'] ?? '').toString());
    final result = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('执行迁移'),
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
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('执行')),
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
    await _post(
      '/api/extensions/migrations/execute',
      data: {
        'extensionId': extensionId,
        'fromVersion': fromVersion,
        'toVersion': toVersion,
        'fromDefinitionHash': fromHashController.text.trim(),
        'toDefinitionHash': toHashController.text.trim(),
      },
      success: '迁移操作已创建',
    );
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
