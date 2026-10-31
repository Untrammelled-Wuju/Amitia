import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/knowledge.dart';
import '../../../../core/services/knowledge_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class KnowledgeDiagnosticsPage extends ConsumerStatefulWidget {
  const KnowledgeDiagnosticsPage({super.key});

  @override
  ConsumerState<KnowledgeDiagnosticsPage> createState() =>
      _KnowledgeDiagnosticsPageState();
}

class _KnowledgeDiagnosticsPageState
    extends ConsumerState<KnowledgeDiagnosticsPage> {
  bool _loading = true;
  bool _busy = false;
  String? _error;
  KnowledgeRuntimeStatusDto? _runtime;
  KnowledgeIndexHealthDto? _indexHealth;
  KnowledgeSloSummaryDto? _slo;
  List<KnowledgeRetrievalTraceDto> _traces = const [];
  List<KnowledgeBenchmarkRunDto> _benchmarks = const [];
  KnowledgeVectorAuditDto? _vectorAudit;
  KnowledgeOrphanAuditDto? _orphanAudit;
  KnowledgeOrphanAuditDto? _orphanPage;
  KnowledgeOrphanAuditDto? _recheck;
  List<KnowledgeOrphanAuditDto> _sessions = const [];
  List<KnowledgeRecoveryReviewDto> _reviews = const [];
  Map<String, dynamic>? _reviewPlan;
  Map<String, dynamic>? _reviewEvents;
  Map<String, dynamic>? _receipts;
  List<Map<String, dynamic>> _recheckHistory = const [];
  String _selectedAuditId = '';
  String _selectedReviewId = '';
  final Map<String, String> _requestKeys = {};

  KnowledgeService get _service => ref.read(knowledgeServiceProvider);

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final runtime = await _service.runtimeStatus();
      final indexHealth = await _service.indexHealth();
      final slo = await _service.sloSummary();
      final traces = await _service.traces();
      final benchmarks = await _service.benchmarkRuns();
      if (!mounted) return;
      setState(() {
        _runtime = runtime;
        _indexHealth = indexHealth;
        _slo = slo;
        _traces = traces;
        _benchmarks = benchmarks;
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = error.toString();
      });
    }
  }

  Future<void> _run(Future<void> Function() operation) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await operation();
    } catch (error) {
      if (mounted) _toast(error.toString(), error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _auditVectors() async {
    await _run(() async {
      final result = await _service.auditVectors();
      if (!mounted) return;
      setState(() => _vectorAudit = result);
    });
  }

  Future<void> _auditOrphans() async {
    await _run(() async {
      final result = await _service.auditOrphans();
      if (!mounted) return;
      setState(() => _orphanAudit = result);
    });
  }

  Future<void> _pageOrphans({required bool next}) async {
    if (next && (_orphanPage?.nextCursor ?? '').isEmpty) return;
    await _run(() async {
      final result = await _service.auditOrphanPage(
        cursor: next ? _orphanPage?.nextCursor ?? '' : '',
      );
      final sessions = await _service.orphanAuditSessions();
      if (!mounted) return;
      setState(() {
        _orphanPage = result;
        _sessions = sessions;
        _selectedAuditId = result.auditId;
      });
    });
  }

  Future<void> _loadSessions() async {
    await _run(() async {
      final sessions = await _service.orphanAuditSessions();
      if (!mounted) return;
      setState(() => _sessions = sessions);
    });
  }

  Future<void> _selectAudit(KnowledgeOrphanAuditDto audit) async {
    setState(() {
      _selectedAuditId = audit.auditId;
      _reviews = const [];
      _reviewPlan = null;
      _reviewEvents = null;
      _receipts = null;
      _recheckHistory = const [];
    });
  }

  Future<void> _loadReceipts() async {
    if (_selectedAuditId.isEmpty) return;
    await _run(() async {
      final value = await _service.orphanAuditReceipts(_selectedAuditId);
      if (!mounted) return;
      setState(() => _receipts = value);
    });
  }

  Future<void> _loadRechecks() async {
    if (_selectedAuditId.isEmpty) return;
    await _run(() async {
      final value = await _service.orphanRecheckHistory(_selectedAuditId);
      if (!mounted) return;
      setState(() => _recheckHistory = value);
    });
  }

  Future<void> _recheckLatest() async {
    if (_selectedAuditId.isEmpty) return;
    await _run(() async {
      final value = await _service.recheckOrphanLatest(_selectedAuditId);
      if (!mounted) return;
      setState(() => _recheck = value);
    });
  }

  Future<void> _loadReviews() async {
    if (_selectedAuditId.isEmpty) return;
    await _run(_fetchReviews);
  }

  Future<void> _fetchReviews() async {
    if (mounted) {
      setState(() {
        _reviews = const [];
        _reviewPlan = null;
        _reviewEvents = null;
      });
    }
    final value = await _service.recoveryReviews(_selectedAuditId);
    if (!mounted) return;
    setState(() => _reviews = value);
  }

  Future<void> _requestReview() async {
    final recheckId = _recheck?.recheckId ?? '';
    if (_selectedAuditId.isEmpty || recheckId.isEmpty) return;
    final requestKey = _requestKeys.putIfAbsent(
      recheckId,
      KnowledgeService.newReviewRequestKey,
    );
    await _run(() async {
      await _service.requestRecoveryReview(
        auditId: _selectedAuditId,
        recheckId: recheckId,
        requestKey: requestKey,
      );
      await _fetchReviews();
      if (mounted) _toast('人工审查申请已登记');
    });
  }

  Future<void> _showReviewPlan(KnowledgeRecoveryReviewDto review) async {
    await _run(() async {
      final authority = await _service.recoveryAuthorityKey();
      final value = await _service.recoveryReviewPlan(review.id);
      if (!mounted) return;
      if (await _service.recoveryAuthorityKey() != authority) {
        throw StateError('Core 或管理员权限已变化，请重新读取');
      }
      setState(() {
        _selectedReviewId = review.id;
        _reviewPlan = value;
      });
    });
  }

  Future<void> _showReviewEvents(KnowledgeRecoveryReviewDto review) async {
    await _run(() async {
      final authority = await _service.recoveryAuthorityKey();
      final value = await _service.recoveryReviewEvents(review.id);
      if (!mounted) return;
      if (await _service.recoveryAuthorityKey() != authority) {
        throw StateError('Core 或管理员权限已变化，请重新读取');
      }
      setState(() {
        _selectedReviewId = review.id;
        _reviewEvents = value;
      });
    });
  }

  Future<void> _transitionReview(
    KnowledgeRecoveryReviewDto review,
    String action,
  ) async {
    final authority = await _service.recoveryAuthorityKey();
    if (action == 'approve') {
      final plan = await _service.recoveryReviewPlan(review.id);
      final historyStatus = (plan['historyStatus'] ?? '').toString();
      final evidenceStatus = (plan['evidenceStatus'] ?? '').toString();
      if (plan['status'] != 'pending' ||
          historyStatus != 'verified' ||
          evidenceStatus != 'valid' ||
          plan['manualReviewReady'] != true ||
          plan['canApprove'] != true) {
        throw StateError('审查证据、权限或操作轨迹已失效');
      }
      if (await _service.recoveryAuthorityKey() != authority) {
        throw StateError('Core 或管理员权限已变化，请重新读取');
      }
    }
    if (!mounted) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(action == 'approve' ? '人工确认审查' : '撤销审查'),
        content: Text(
          action == 'approve'
              ? '该操作只记录人工审查结论，不执行向量删除或重建。'
              : '撤销后，该人工审查记录不能继续用于核查。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('确认'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    await _run(() async {
      if (await _service.recoveryAuthorityKey() != authority) {
        throw StateError('Core 或管理员权限已变化，请重新读取');
      }
      await _service.transitionRecoveryReview(
        reviewId: review.id,
        action: action,
      );
      await _fetchReviews();
      if (mounted) _toast(action == 'approve' ? '人工审查已记录' : '审查已撤销');
    });
  }

  void _toast(String message, {bool error = false}) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? Theme.of(context).colorScheme.error : null,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '知识库诊断',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          IconButton(
            onPressed: _loading || _busy ? null : _load,
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const AmitiaLoadingState(message: '读取知识库诊断…')
            : _error != null
            ? AmitiaErrorState(message: _error!, onRetry: _load)
            : RefreshIndicator(
                onRefresh: _load,
                child: ListView(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.pagePadding,
                    AppSpacing.sm,
                    AppSpacing.pagePadding,
                    AppSpacing.xl,
                  ),
                  children: [
                    _runtimeCard(context),
                    const SizedBox(height: 12),
                    _indexCard(context),
                    const SizedBox(height: 12),
                    _auditCard(context),
                    const SizedBox(height: 12),
                    _sloCard(context),
                    const SizedBox(height: 12),
                    _traceCard(context),
                    const SizedBox(height: 12),
                    _benchmarkCard(context),
                    if (_selectedAuditId.isNotEmpty) ...[
                      const SizedBox(height: 12),
                      _reviewCard(context),
                    ],
                  ],
                ),
              ),
      ),
    );
  }

  Widget _runtimeCard(BuildContext context) {
    final value = _runtime;
    if (value == null) return const SizedBox.shrink();
    return _card(context, '运行策略', [
      Wrap(
        spacing: 6,
        runSpacing: 6,
        children: [
          Chip(
            label: Text(
              value.retrievalPolicyEnabled ? 'POLICY ACTIVE' : 'POLICY LEGACY',
            ),
          ),
          if (value.retrievalPolicyShadow) const Chip(label: Text('SHADOW')),
          Chip(
            label: Text(
              value.canonicalReaderEnabled ? 'READER ON' : 'READER OFF',
            ),
          ),
          Chip(
            label: Text(
              value.agenticInvestigationEnabled ? 'AGENTIC ON' : 'AGENTIC OFF',
            ),
          ),
        ],
      ),
      Text(
        'readerWorkers=${value.readerWorkers} · trace=${value.retrievalTraceRetentionHours}h · benchmark=${value.benchmarkRetentionHours}h · cleanupBatch=${value.diagnosticCleanupBatchSize}',
        style: AppTypography.caption(context),
      ),
      Text(
        'config=${_short(value.configHash)}',
        style: AppTypography.caption(context),
      ),
      if (_traces.isNotEmpty)
        Text(
          '最近 ${_traces.length} 条：actual=${_counts(_traces.map((item) => item.policyMode))} · shadow=${_counts(_traces.where((item) => item.shadowLevel.isNotEmpty).map((item) => item.shadowLevel))}',
          style: AppTypography.caption(context),
        ),
    ]);
  }

  Widget _indexCard(BuildContext context) {
    final value = _indexHealth;
    if (value == null) return const SizedBox.shrink();
    return _card(context, '向量索引任务健康度', [
      Chip(label: Text(value.status)),
      Text(
        '等待写入 ${value.pendingUpserts} · 等待删除 ${value.pendingDeletes} · 重试 ${value.retryingUpserts + value.retryingDeletes} · 耗尽 ${value.exhaustedUpserts + value.exhaustedDeletes}',
        style: AppTypography.bodySmall(context),
      ),
      Text(
        '租约 ${value.activeLeases} · 逾期 ${value.overdueOperations} · 最久积压 ${value.oldestOutstandingSeconds} 秒',
        style: AppTypography.caption(context),
      ),
      Text(
        '统计只反映当前空间的持久化 Outbox，不代表 Qdrant 已完成全量核验。',
        style: AppTypography.caption(context),
      ),
    ]);
  }

  Widget _auditCard(BuildContext context) {
    final sample = _vectorAudit;
    final orphan = _orphanAudit;
    final page = _orphanPage;
    return _card(context, 'Qdrant 向量核验', [
      Wrap(
        spacing: 8,
        runSpacing: 8,
        children: [
          OutlinedButton(
            onPressed: _busy ? null : _auditVectors,
            child: const Text('抽样核验'),
          ),
          OutlinedButton(
            onPressed: _busy ? null : _auditOrphans,
            child: const Text('反向孤儿检查'),
          ),
          OutlinedButton(
            onPressed: _busy ? null : () => _pageOrphans(next: false),
            child: const Text('开始分页巡检'),
          ),
          if ((page?.nextCursor ?? '').isNotEmpty)
            OutlinedButton(
              onPressed: _busy ? null : () => _pageOrphans(next: true),
              child: const Text('下一页'),
            ),
          OutlinedButton(
            onPressed: _busy ? null : _loadSessions,
            child: const Text('巡检历史'),
          ),
        ],
      ),
      if (sample != null)
        Text(
          '样本 ${sample.status} · 已检 ${sample.sampled} · 缺失 ${sample.missing} · 元数据不符 ${sample.mismatched} · 集合 ${sample.collectionsChecked}',
          style: AppTypography.bodySmall(context),
        ),
      if (orphan != null)
        Text(
          '孤儿检查 ${orphan.status} · 已检 ${orphan.sampled} · 疑似 ${orphan.suspectedOrphans} · 元数据差异 ${orphan.metadataMismatches} · 在途 ${orphan.inFlight}',
          style: AppTypography.bodySmall(context),
        ),
      if (page != null)
        Text(
          '巡检 ${_short(page.auditId)} · 第 ${page.page} 页 · 已查 ${page.sampled} · 疑似 ${page.suspectedOrphans} · ${page.finished ? '已到末页' : '仍有未覆盖数据'}',
          style: AppTypography.caption(context),
        ),
      ..._sessions.map(
        (item) => ListTile(
          dense: true,
          contentPadding: EdgeInsets.zero,
          title: Text('${_short(item.auditId)} · ${item.status}'),
          subtitle: Text(
            '${item.page} 页 · ${item.sampled} 点 · 疑似 ${item.suspectedOrphans} · 差异 ${item.metadataMismatches}',
          ),
          trailing: const Icon(Icons.chevron_right),
          onTap: () => _selectAudit(item),
        ),
      ),
      if (_selectedAuditId.isNotEmpty)
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            OutlinedButton(
              onPressed: _busy ? null : _recheckLatest,
              child: const Text('二次核验'),
            ),
            OutlinedButton(
              onPressed: _busy ? null : _loadReceipts,
              child: const Text('页回执'),
            ),
            OutlinedButton(
              onPressed: _busy ? null : _loadRechecks,
              child: const Text('复核历史'),
            ),
            OutlinedButton(
              onPressed: _busy ? null : _loadReviews,
              child: const Text('人工审查记录'),
            ),
          ],
        ),
      if (_recheck != null)
        SelectableText(
          '复核 ${_recheck!.status} · 原疑似 ${_recheck!.suspectedOrphans} · recheckId=${_short(_recheck!.recheckId)}',
          style: AppTypography.caption(context),
        ),
      if (_recheck?.manualRecoveryAllowed == true &&
          (_recheck?.recheckId ?? '').isNotEmpty)
        Align(
          alignment: Alignment.centerRight,
          child: FilledButton.tonal(
            onPressed: _busy ? null : _requestReview,
            child: const Text('提交人工核查申请'),
          ),
        ),
      if (_receipts != null) _jsonBlock(context, '页回执', _receipts!),
      if (_recheckHistory.isNotEmpty)
        ..._recheckHistory.map(
          (item) => Text(
            '复核 ${_short((item['id'] ?? '').toString())} · 第 ${item['page']} 页 · ${item['status']}',
            style: AppTypography.caption(context),
          ),
        ),
    ]);
  }

  Widget _sloCard(BuildContext context) {
    final value = _slo;
    if (value == null) return const SizedBox.shrink();
    return _card(
      context,
      '检索 SLO',
      value.levels
          .map(
            (item) => Text(
              '${item.level.toUpperCase()} · P50 ${item.p50Ms}ms · P95 ${item.p95Ms}ms · target ${item.targetMs}ms · ${item.enoughSamples ? (item.targetMet ? '达标' : '超标') : '样本不足'} · samples=${item.samples}',
              style: AppTypography.bodySmall(context),
            ),
          )
          .toList(growable: false),
    );
  }

  Widget _traceCard(BuildContext context) {
    return _card(
      context,
      '检索 Trace',
      _traces.isEmpty
          ? [Text('暂无检索 Trace', style: AppTypography.caption(context))]
          : _traces
                .map(
                  (item) => ExpansionTile(
                    tilePadding: EdgeInsets.zero,
                    title: Text(
                      '${item.level.toUpperCase()} · ${item.policyMode} · ${item.durationMs}ms',
                    ),
                    subtitle: Text(
                      '候选 ${item.candidateCount} / 选中 ${item.selectedCount} · ${item.sufficient ? 'sufficient' : 'insufficient'} · ${item.routeReason}${item.shadowLevel.isEmpty ? '' : ' · shadow ${item.shadowLevel}'}',
                    ),
                    children: [
                      Container(
                        width: double.infinity,
                        padding: const EdgeInsets.all(10),
                        color: Theme.of(
                          context,
                        ).colorScheme.surfaceContainerHighest,
                        child: SelectableText(
                          const JsonEncoder.withIndent('  ').convert(item.raw),
                          style: AppTypography.caption(context),
                        ),
                      ),
                    ],
                  ),
                )
                .toList(growable: false),
    );
  }

  Widget _benchmarkCard(BuildContext context) {
    return _card(
      context,
      'Benchmark 趋势',
      _benchmarks.isEmpty
          ? [Text('暂无 Benchmark 记录', style: AppTypography.caption(context))]
          : _benchmarks
                .map(
                  (item) => ListTile(
                    dense: true,
                    contentPadding: EdgeInsets.zero,
                    title: Text(item.passed ? 'PASS' : 'FAIL'),
                    subtitle: Text(
                      'Recall@K ${_score(item.recallAtK)} · MRR ${_score(item.mrr)} · NDCG ${_score(item.ndcgAtK)} · NoAnswer ${_score(item.noAnswerAccuracy)} · P50/P95 ${item.p50LatencyMs}/${item.p95LatencyMs}ms',
                    ),
                  ),
                )
                .toList(growable: false),
    );
  }

  Widget _reviewCard(BuildContext context) {
    return _card(context, '人工恢复审查', [
      Text(
        '巡检 ${_short(_selectedAuditId)}',
        style: AppTypography.caption(context),
      ),
      if (_reviews.isEmpty)
        Text('暂无人工审查记录', style: AppTypography.caption(context)),
      ..._reviews.map(
        (review) => Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '${_short(review.id)} · ${review.displayStatus} · 到期 ${_formatTime(review.expiresAt)}',
              style: AppTypography.bodySmall(context),
            ),
            Wrap(
              spacing: 6,
              children: [
                TextButton(
                  onPressed: _busy ? null : () => _showReviewPlan(review),
                  child: const Text('计划检查'),
                ),
                TextButton(
                  onPressed: _busy ? null : () => _showReviewEvents(review),
                  child: const Text('操作轨迹'),
                ),
                if (review.displayStatus == 'needs_recheck')
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () => _transitionReview(review, 'approve'),
                    child: const Text('人工通过'),
                  ),
                if (review.status != 'revoked')
                  TextButton(
                    onPressed: _busy
                        ? null
                        : () => _transitionReview(review, 'revoke'),
                    child: const Text('撤销'),
                  ),
              ],
            ),
          ],
        ),
      ),
      if (_selectedReviewId.isNotEmpty)
        Text(
          '当前审查 ${_short(_selectedReviewId)}',
          style: AppTypography.caption(context),
        ),
      if (_reviewPlan != null) _jsonBlock(context, '只读计划', _reviewPlan!),
      if (_reviewEvents != null) _jsonBlock(context, '操作轨迹', _reviewEvents!),
    ]);
  }

  Widget _card(BuildContext context, String title, List<Widget> children) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: AppTypography.sectionTitle(context)),
            const SizedBox(height: 10),
            ...children.expand((child) => [child, const SizedBox(height: 6)]),
          ],
        ),
      ),
    );
  }

  Widget _jsonBlock(
    BuildContext context,
    String title,
    Map<String, dynamic> value,
  ) {
    return ExpansionTile(
      tilePadding: EdgeInsets.zero,
      title: Text(title, style: AppTypography.bodySmall(context)),
      children: [
        Container(
          width: double.infinity,
          padding: const EdgeInsets.all(10),
          color: Theme.of(context).colorScheme.surfaceContainerHighest,
          child: SelectableText(
            const JsonEncoder.withIndent('  ').convert(value),
            style: AppTypography.caption(context),
          ),
        ),
      ],
    );
  }

  String _short(String value) =>
      value.length <= 10 ? value : value.substring(0, 10);

  String _score(double value) => value.toStringAsFixed(3);

  String _counts(Iterable<String> values) {
    final result = <String, int>{};
    for (final value in values) {
      if (value.isEmpty) continue;
      result[value] = (result[value] ?? 0) + 1;
    }
    return result.isEmpty
        ? '-'
        : result.entries
              .map((entry) => '${entry.key}:${entry.value}')
              .join(',');
  }

  String _formatTime(String value) {
    final parsed = DateTime.tryParse(value);
    return parsed == null ? value : parsed.toLocal().toString();
  }
}
