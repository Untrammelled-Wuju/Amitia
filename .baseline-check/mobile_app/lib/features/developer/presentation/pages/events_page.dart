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

class EventsPage extends ConsumerStatefulWidget {
  const EventsPage({super.key});

  @override
  ConsumerState<EventsPage> createState() => _EventsPageState();
}

class _EventsPageState extends ConsumerState<EventsPage> {
  bool _loading = true;
  String? _error;
  int _tab = 0;
  List<Map<String, dynamic>> _deliveries = [];
  List<Map<String, dynamic>> _deadLetters = [];
  List<Map<String, dynamic>> _types = [];
  Map<String, dynamic> _stats = {};
  List<Map<String, dynamic>> _subscriptions = [];
  List<Map<String, dynamic>> _outbox = [];
  List<Map<String, dynamic>> _audit = [];
  final _subscriptionFilter = TextEditingController();
  final _outboxFilter = TextEditingController();

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _subscriptionFilter.dispose();
    _outboxFilter.dispose();
    super.dispose();
  }

  Map<String, dynamic> _map(dynamic value) => Map<String, dynamic>.from(value as Map);
  List<Map<String, dynamic>> _items(Map<String, dynamic>? value) =>
      (value?['items'] as List<dynamic>? ?? const []).whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();

  Future<void> _load() async {
    if (mounted) setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait([
        api.get<Map<String, dynamic>>('/api/extensions/events/deliveries', queryParameters: {'limit': 200}, fromJson: _map),
        api.get<Map<String, dynamic>>('/api/extensions/events/dead-letters', queryParameters: {'limit': 200}, fromJson: _map),
        api.get<Map<String, dynamic>>('/api/extensions/events/types', fromJson: _map),
        api.get<Map<String, dynamic>>('/api/extensions/events/stats', fromJson: _map),
        api.get<Map<String, dynamic>>('/api/extensions/events/audit', fromJson: _map),
      ]);
      if (!mounted) return;
      setState(() {
        _deliveries = _items(results[0]);
        _deadLetters = _items(results[1]);
        _types = _items(results[2]);
        _stats = results[3] ?? {};
        _audit = _items(results[4]);
        _loading = false;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '事件中心',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(icon: Icons.send_outlined, tooltip: '发布事件', onPressed: _publishEvent),
          AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(children: [
          Padding(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            child: AmitiaSegmentedControl(
              segments: const ['投递', '死信', '类型', '管理'],
              selectedIndex: _tab,
              onChanged: (value) => setState(() => _tab = value),
            ),
          ),
          Expanded(child: _body()),
        ]),
      ),
    );
  }

  Widget _body() {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    switch (_tab) {
      case 0: return _deliveryList();
      case 1: return _deadList();
      case 2: return _typeList();
      default: return _admin();
    }
  }

  Widget _deliveryList() {
    if (_deliveries.isEmpty) return const AmitiaEmptyState(icon: Icons.bolt_outlined, title: '暂无事件投递');
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.only(bottom: AppSpacing.xl),
        itemCount: _deliveries.length,
        itemBuilder: (_, index) {
          final item = _deliveries[index];
          final id = (item['deliveryId'] ?? item['DeliveryID'] ?? '').toString();
          final status = (item['status'] ?? item['Status'] ?? 'unknown').toString();
          return _card(
            icon: Icons.bolt,
            title: (item['eventId'] ?? item['EventID'] ?? id).toString(),
            subtitle: '${item['subscriptionId'] ?? item['SubscriptionID'] ?? ''} · ${item['finishedAt'] ?? item['FinishedAt'] ?? item['createdAt'] ?? item['CreatedAt'] ?? ''}',
            badge: status,
            badgeType: status == 'succeeded' ? BadgeType.success : status == 'failed' ? BadgeType.error : BadgeType.info,
            onTap: () => _fetchDetail('/api/extensions/events/deliveries/$id', '投递详情'),
          );
        },
      ),
    );
  }

  Widget _deadList() {
    if (_deadLetters.isEmpty) return const AmitiaEmptyState(icon: Icons.inbox_outlined, title: '暂无死信', subtitle: '没有处理失败后进入 Dead Letter Queue 的事件。');
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView.builder(
        padding: EdgeInsets.only(bottom: AppSpacing.xl),
        itemCount: _deadLetters.length,
        itemBuilder: (_, index) {
          final item = _deadLetters[index];
          final id = (item['deadLetterId'] ?? item['DeadLetterID'] ?? '').toString();
          return Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
            child: AmitiaCard(
              onTap: () => _fetchDetail('/api/extensions/events/dead-letters/$id', '死信详情'),
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Row(children: [
                  Icon(Icons.error_outline, color: AppColors.error),
                  const SizedBox(width: 12),
                  Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Text((item['eventTypeId'] ?? item['EventTypeID'] ?? item['eventId'] ?? 'Dead Letter').toString(), style: AppTypography.cardTitle(context)),
                    Text((item['reason'] ?? item['Reason'] ?? item['errorMessage'] ?? item['ErrorMessage'] ?? id).toString(), style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
                  ])),
                  const AmitiaStatusBadge(label: '死信', type: BadgeType.error),
                ]),
                SizedBox(height: AppSpacing.md),
                Row(children: [
                  Expanded(child: AmitiaButton(label: '重放', icon: Icons.replay, isSecondary: true, onPressed: () => _deadAction(id, 'replay', '死信已重放'))),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(child: AmitiaButton(label: '丢弃', icon: Icons.delete_outline, isDestructive: true, onPressed: () => _deadAction(id, 'discard', '死信已丢弃'))),
                ]),
              ]),
            ),
          );
        },
      ),
    );
  }

  Widget _typeList() {
    return Column(children: [
      Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaButton(label: '注册事件类型', isSecondary: true, icon: Icons.add, isFullWidth: true, onPressed: _registerType),
      ),
      SizedBox(height: AppSpacing.sm),
      Expanded(
        child: _types.isEmpty
            ? const AmitiaEmptyState(icon: Icons.category_outlined, title: '暂无事件类型')
            : ListView.builder(
                padding: EdgeInsets.only(bottom: AppSpacing.xl),
                itemCount: _types.length,
                itemBuilder: (_, index) {
                  final item = _types[index];
                  final id = (item['eventTypeId'] ?? item['EventTypeID'] ?? '').toString();
                  final version = item['version'] ?? item['Version'] ?? 1;
                  return _card(
                    icon: Icons.category_outlined,
                    title: id,
                    subtitle: (item['description'] ?? item['Description'] ?? '').toString(),
                    badge: 'v$version',
                    badgeType: BadgeType.info,
                    onTap: () => _fetchDetail('/api/extensions/events/types/$id/$version', '事件类型'),
                  );
                },
              ),
      ),
    ]);
  }

  Widget _admin() {
    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.xl),
        children: [
          Text('统计', style: AppTypography.cardTitle(context)),
          SizedBox(height: AppSpacing.sm),
          AmitiaCard(onTap: () => _showJson('Event Stats', _stats), child: SelectableText(_stats.entries.map((e) => '${e.key}: ${e.value}').join('\n'), style: AppTypography.bodySmall(context))),
          SizedBox(height: AppSpacing.lg),
          Text('订阅', style: AppTypography.cardTitle(context)),
          Text('后端要求按 Extension ID 或 Event Type ID 查询。', style: AppTypography.caption(context)),
          SizedBox(height: AppSpacing.sm),
          Row(children: [
            Expanded(child: TextField(controller: _subscriptionFilter, decoration: const InputDecoration(labelText: 'Extension ID / Event Type ID'))),
            const SizedBox(width: 8),
            IconButton(onPressed: _loadSubscriptions, icon: const Icon(Icons.search)),
            IconButton(onPressed: _registerSubscription, icon: const Icon(Icons.add)),
          ]),
          ..._subscriptions.map((item) {
            final id = (item['contributionId'] ?? item['ContributionID'] ?? '').toString();
            return ListTile(
              contentPadding: EdgeInsets.zero,
              title: Text(id),
              subtitle: Text('${item['eventTypeId'] ?? item['EventTypeID'] ?? ''} · ${item['extensionId'] ?? item['ExtensionID'] ?? ''}'),
              onTap: () => _fetchDetail('/api/extensions/events/subscriptions/$id', '订阅详情'),
              trailing: PopupMenuButton<String>(
                onSelected: (value) => value == 'reset' ? _resetCircuit(id) : _deleteSubscription(id),
                itemBuilder: (_) => const [
                  PopupMenuItem(value: 'reset', child: Text('重置 Circuit')),
                  PopupMenuItem(value: 'delete', child: Text('删除订阅')),
                ],
              ),
            );
          }),
          SizedBox(height: AppSpacing.lg),
          Text('Outbox', style: AppTypography.cardTitle(context)),
          Text('输入 Extension ID；也可输入 status:pending 之类的状态过滤。', style: AppTypography.caption(context)),
          Row(children: [
            Expanded(child: TextField(controller: _outboxFilter, decoration: const InputDecoration(labelText: 'Extension ID 或 status:<状态>'))),
            IconButton(onPressed: _loadOutbox, icon: const Icon(Icons.search)),
          ]),
          if (_outbox.isEmpty) Padding(padding: EdgeInsets.symmetric(vertical: AppSpacing.sm), child: Text('暂无已加载 Outbox 记录', style: AppTypography.caption(context))),
          ..._outbox.take(30).map((item) => ListTile(contentPadding: EdgeInsets.zero, dense: true, title: Text((item['eventId'] ?? item['EventID'] ?? item['id'] ?? 'Outbox').toString()), subtitle: Text((item['status'] ?? item['Status'] ?? '').toString()), onTap: () => _showJson('Outbox', item))),
          SizedBox(height: AppSpacing.lg),
          Row(children: [Expanded(child: Text('Audit', style: AppTypography.cardTitle(context))), Text('${_audit.length} 条', style: AppTypography.caption(context))]),
          ..._audit.take(50).map((item) => ListTile(contentPadding: EdgeInsets.zero, dense: true, title: Text((item['action'] ?? item['Action'] ?? 'audit').toString()), subtitle: Text((item['extensionId'] ?? item['ExtensionID'] ?? item['eventId'] ?? item['EventID'] ?? '').toString()), onTap: () => _showJson('Audit', item))),
        ],
      ),
    );
  }

  Widget _card({required IconData icon, required String title, required String subtitle, required String badge, required BadgeType badgeType, required VoidCallback onTap}) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
      child: AmitiaCard(
        onTap: onTap,
        child: Row(children: [
          Container(width: 38, height: 38, decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brSmall), child: Icon(icon, size: 19, color: context.accentPrimary)),
          const SizedBox(width: 12),
          Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [Text(title, style: AppTypography.cardTitle(context), maxLines: 1, overflow: TextOverflow.ellipsis), if (subtitle.isNotEmpty) Text(subtitle, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis)])),
          AmitiaStatusBadge(label: badge, type: badgeType),
        ]),
      ),
    );
  }

  Future<void> _deadAction(String id, String action, String message) async {
    try {
      await ref.read(backendServiceProvider).post('/api/extensions/events/dead-letters/$id/$action');
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    }
  }

  Future<void> _fetchDetail(String path, String title) async {
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>(path, fromJson: _map);
      if (mounted) await _showJson(title, data ?? {});
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取失败：$e')));
    }
  }

  Future<void> _loadSubscriptions() async {
    final filter = _subscriptionFilter.text.trim();
    if (filter.isEmpty) return;
    try {
      final api = ref.read(backendServiceProvider);
      Map<String, dynamic>? data;
      try {
        data = await api.get<Map<String, dynamic>>('/api/extensions/events/subscriptions', queryParameters: {'extensionId': filter}, fromJson: _map);
        if (_items(data).isEmpty) {
          data = await api.get<Map<String, dynamic>>('/api/extensions/events/subscriptions', queryParameters: {'eventTypeId': filter}, fromJson: _map);
        }
      } catch (_) {
        data = await api.get<Map<String, dynamic>>('/api/extensions/events/subscriptions', queryParameters: {'eventTypeId': filter}, fromJson: _map);
      }
      if (mounted) setState(() => _subscriptions = _items(data));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('查询失败：$e')));
    }
  }

  Future<void> _loadOutbox() async {
    final filter = _outboxFilter.text.trim();
    if (filter.isEmpty) return;
    final query = filter.startsWith('status:') ? {'status': filter.substring(7)} : {'extensionId': filter};
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>('/api/extensions/events/outbox', queryParameters: query, fromJson: _map);
      if (mounted) setState(() => _outbox = _items(data));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('查询失败：$e')));
    }
  }

  Future<void> _resetCircuit(String id) async {
    await ref.read(backendServiceProvider).post('/api/extensions/events/circuits/$id/reset');
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Circuit 已重置')));
  }

  Future<void> _deleteSubscription(String id) async {
    await ref.read(backendServiceProvider).delete('/api/extensions/events/subscriptions/$id');
    await _loadSubscriptions();
  }

  Future<void> _publishEvent() async {
    final template = <String, dynamic>{
      'eventTypeId': '', 'version': 1, 'payload': <String, dynamic>{},
      'producerId': 'mobile-app', 'producerType': 'system',
      'aggregateType': '', 'aggregateId': '', 'partitionKey': '', 'orderingKey': '',
      'traceId': '', 'operationId': '', 'parentEventId': '', 'parentDepth': 0, 'metadata': <String, dynamic>{},
    };
    final data = await _jsonEditor('发布事件', template, includeTxChoice: true);
    if (data == null) return;
    final tx = data.remove('__transactional') == true;
    try {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(tx ? '/api/extensions/events/publish-tx' : '/api/extensions/events/publish', data: data, fromJson: _map);
      if (mounted) await _showJson('发布结果', result ?? {});
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('发布失败：$e')));
    }
  }

  Future<void> _registerType() async {
    final template = <String, dynamic>{'eventTypeId': '', 'version': 1, 'description': '', 'riskLevel': 'low'};
    final data = await _jsonEditor('注册事件类型', template);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/events/types', data: data, fromJson: _map);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('事件类型已注册')));
      await _load();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('注册失败：$e')));
    }
  }

  Future<void> _registerSubscription() async {
    final template = <String, dynamic>{'contributionId': '', 'extensionId': '', 'moduleId': '', 'eventTypeId': '', 'minVersion': 1, 'maxVersion': 1, 'handlerId': '', 'enabledByDefault': true};
    final data = await _jsonEditor('注册事件订阅', template);
    if (data == null) return;
    try {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/extensions/events/subscriptions', data: [data], fromJson: _map);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('订阅已注册')));
      if (_subscriptionFilter.text.trim().isNotEmpty) await _loadSubscriptions();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('注册失败：$e')));
    }
  }

  Future<Map<String, dynamic>?> _jsonEditor(String title, Map<String, dynamic> initial, {bool includeTxChoice = false}) async {
    final controller = TextEditingController(text: const JsonEncoder.withIndent('  ').convert(initial));
    bool transactional = false;
    String? validationError;
    return showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => AlertDialog(
          title: Text(title),
          content: SizedBox(width: 720, child: Column(mainAxisSize: MainAxisSize.min, children: [
            Flexible(child: TextField(controller: controller, minLines: 12, maxLines: 22, style: const TextStyle(fontFamily: 'monospace', fontSize: 12), decoration: const InputDecoration(border: OutlineInputBorder()))),
            if (includeTxChoice) SwitchListTile(contentPadding: EdgeInsets.zero, title: const Text('事务发布（publish-tx）'), value: transactional, onChanged: (v) => setDialogState(() => transactional = v)),
            if (validationError != null) Text(validationError!, style: TextStyle(color: AppColors.error)),
          ])),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
            FilledButton(onPressed: () {
              try {
                final decoded = jsonDecode(controller.text);
                if (decoded is! Map) throw const FormatException('顶层必须为 JSON 对象');
                final result = Map<String, dynamic>.from(decoded);
                if (includeTxChoice) result['__transactional'] = transactional;
                Navigator.pop(dialogContext, result);
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
