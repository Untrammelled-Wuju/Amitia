import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DecisionVizPage extends ConsumerStatefulWidget {
  const DecisionVizPage({super.key});

  @override
  ConsumerState<DecisionVizPage> createState() => _DecisionVizPageState();
}

class _DecisionVizPageState extends ConsumerState<DecisionVizPage> {
  bool _loading = true;
  String? _error;
  Map<String, dynamic> _snapshot = const {};

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>('/api/runtime/debug/snapshot');
      if (!mounted) return;
      setState(() {
        _snapshot = data ?? const {};
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
  void initState() {
    super.initState();
    Future.microtask(_load);
  }

  Widget _field(String label, dynamic value) => Padding(
        padding: const EdgeInsets.symmetric(vertical: 5),
        child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
          SizedBox(width: 120, child: Text(label, style: Theme.of(context).textTheme.bodySmall)),
          Expanded(child: SelectableText(value == null ? '—' : '$value')),
        ]),
      );

  Widget _planCard(String title, Map<String, dynamic> plan, List<String> fields) => Card(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.md),
          child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
            Text(title, style: Theme.of(context).textTheme.titleMedium),
            const SizedBox(height: 8),
            for (final field in fields) _field(field, plan[field]),
            const Divider(),
            ExpansionTile(
              tilePadding: EdgeInsets.zero,
              title: const Text('原始数据'),
              children: [SelectableText(const JsonEncoder.withIndent('  ').convert(plan), style: const TextStyle(fontFamily: 'monospace', fontSize: 12))],
            ),
          ]),
        ),
      );

  @override
  Widget build(BuildContext context) {
    final behavior = _snapshot['behaviorPlan'] is Map ? Map<String, dynamic>.from(_snapshot['behaviorPlan'] as Map) : <String, dynamic>{};
    final expression = _snapshot['expressionPlan'] is Map ? Map<String, dynamic>.from(_snapshot['expressionPlan'] as Map) : <String, dynamic>{};
    final meta = _snapshot['meta'] is Map ? Map<String, dynamic>.from(_snapshot['meta'] as Map) : <String, dynamic>{};
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'BDI 决策可视化',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
        actions: [AmitiaIconButton(icon: Icons.refresh, tooltip: '刷新', onPressed: _load)],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(child: Text('加载失败：$_error'))
              : ListView(
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  children: [
                    Card(
                      child: Padding(
                        padding: EdgeInsets.all(AppSpacing.md),
                        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
                          Text('运行状态', style: Theme.of(context).textTheme.titleMedium),
                          _field('降级', meta['degraded'] == true ? '是' : '否'),
                          _field('版本', meta['stateVersion'] ?? behavior['stateVersion']),
                        ]),
                      ),
                    ),
                    const SizedBox(height: 12),
                    if (behavior.isNotEmpty)
                      _planCard('BehaviorPlan', behavior, const ['intention', 'strategy', 'winnerCandidate', 'questionPolicy', 'advicePolicy', 'deliveryPolicy']),
                    if (behavior.isNotEmpty) const SizedBox(height: 12),
                    if (expression.isNotEmpty)
                      _planCard('ExpressionPlan', expression, const ['sentenceCount', 'maxLength', 'directness', 'warmth', 'emotionDisplay', 'useQuestion', 'voiceParams']),
                    if (behavior.isEmpty && expression.isEmpty)
                      const Card(child: Padding(padding: EdgeInsets.all(24), child: Center(child: Text('暂无决策数据')))),
                  ],
                ),
    );
  }
}
