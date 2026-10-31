import 'dart:async';
import 'dart:convert';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/amitia_page_route.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/knowledge.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import 'knowledge_diagnostics_page.dart';

class KnowledgePage extends ConsumerStatefulWidget {
  const KnowledgePage({super.key});

  @override
  ConsumerState<KnowledgePage> createState() => _KnowledgePageState();
}

class _KnowledgePageState extends ConsumerState<KnowledgePage> {
  final _queryController = TextEditingController();
  List<KnowledgeSourceDto> _sources = const [];
  List<KnowledgeJobDto> _jobs = const [];
  KnowledgeSearchResultDto? _searchResult;
  bool _loading = true;
  bool _uploading = false;
  bool _searching = false;
  bool _reconciling = false;
  bool _collecting = false;
  bool _refreshing = false;
  String _readingEvidenceId = '';
  String _temporalMode = 'current';
  String _retrievalLevel = 'auto';
  DateTime? _historicalAsOf;
  String? _error;
  Timer? _refreshTimer;

  @override
  void initState() {
    super.initState();
    unawaited(_load());
    _refreshTimer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (mounted &&
          (_sources.any((item) => item.isBusy) ||
              _jobs.any(
                (item) => item.status == 'queued' || item.status == 'running',
              ))) {
        unawaited(_load(showLoading: false));
      }
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    _queryController.dispose();
    super.dispose();
  }

  Future<void> _load({bool showLoading = true}) async {
    if (_refreshing) return;
    _refreshing = true;
    if (showLoading && mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final service = ref.read(knowledgeServiceProvider);
      final results = await Future.wait<Object>([
        service.listSources(),
        service.listJobs(),
      ]);
      if (!mounted) return;
      setState(() {
        _sources = results[0] as List<KnowledgeSourceDto>;
        _jobs = results[1] as List<KnowledgeJobDto>;
        _loading = false;
        _error = null;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = error.toString();
      });
    } finally {
      _refreshing = false;
    }
  }

  Future<void> _upload() async {
    if (_uploading) return;
    final result = await FilePicker.platform.pickFiles(
      allowMultiple: false,
      withData: false,
      type: FileType.custom,
      allowedExtensions: const [
        'txt',
        'md',
        'csv',
        'json',
        'jsonl',
        'ndjson',
        'pdf',
        'doc',
        'docx',
        'xlsx',
      ],
    );
    if (result == null || result.files.isEmpty || !mounted) return;
    final file = result.files.single;
    final path = file.path;
    if (path == null || path.trim().isEmpty) {
      amitiaSnackBar(context, '无法取得文件路径，请先将文件保存到本机后重试');
      return;
    }
    final logicalName = await _askLogicalName(file.name);
    if (logicalName == null || !mounted) return;
    setState(() => _uploading = true);
    try {
      await ref
          .read(knowledgeServiceProvider)
          .upload(filePath: path, logicalName: logicalName);
      if (!mounted) return;
      amitiaSnackBar(context, '知识源已进入解析队列');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _uploading = false);
    }
  }

  Future<void> _uploadRevision(KnowledgeSourceDto source) async {
    final result = await FilePicker.platform.pickFiles(
      allowMultiple: false,
      withData: false,
      type: FileType.custom,
      allowedExtensions: const [
        'txt',
        'md',
        'csv',
        'json',
        'jsonl',
        'ndjson',
        'pdf',
        'doc',
        'docx',
        'xlsx',
      ],
    );
    if (result == null || result.files.isEmpty || !mounted) return;
    final path = result.files.single.path;
    if (path == null || path.trim().isEmpty) {
      amitiaSnackBar(context, '无法取得文件路径，请先将文件保存到本机后重试');
      return;
    }
    try {
      await ref
          .read(knowledgeServiceProvider)
          .uploadRevision(sourceId: source.id, filePath: path);
      if (!mounted) return;
      amitiaSnackBar(context, '新版本已进入解析队列；当前活动版本会保持可用直到切换完成');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _reindex(KnowledgeSourceDto source) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('重建索引'),
        content: Text(
          '重新生成“${source.logicalName.isEmpty ? source.filename : source.logicalName}”活动版本 r${source.revision} 的向量索引？',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('重建'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref.read(knowledgeServiceProvider).reindexSource(source.id);
      if (!mounted) return;
      amitiaSnackBar(context, '重建索引任务已入队');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _showSourceDetail(KnowledgeSourceDto source) async {
    try {
      final detail = await ref
          .read(knowledgeServiceProvider)
          .getSourceDetail(source.id);
      if (!mounted) return;
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (context) => SafeArea(
          child: FractionallySizedBox(
            heightFactor: 0.82,
            child: Column(
              children: [
                ListTile(
                  title: Text(
                    source.logicalName.isEmpty
                        ? source.filename
                        : source.logicalName,
                  ),
                  subtitle: Text(
                    '活动版本 r${detail.source.revision} · ${detail.unitCount} units · ${detail.chunkCount} chunks · ${detail.indexHealth}',
                  ),
                ),
                const Divider(height: 1),
                Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 12),
                  child: Align(
                    alignment: Alignment.centerRight,
                    child: TextButton.icon(
                      onPressed: () {
                        Navigator.pop(context);
                        unawaited(
                          _showSourceOutline(
                            detail.source,
                            detail.source.revision,
                          ),
                        );
                      },
                      icon: const Icon(Icons.toc_outlined),
                      label: const Text('Canonical Outline'),
                    ),
                  ),
                ),
                Expanded(
                  child: ListView.separated(
                    itemCount: detail.revisions.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (context, index) {
                      final revision = detail.revisions[index];
                      final active =
                          revision.revision == detail.source.revision;
                      return ListTile(
                        leading: CircleAvatar(
                          child: Text('r${revision.revision}'),
                        ),
                        title: Text(revision.filename),
                        subtitle: Text(
                          '${revision.status} · ${revision.processedUnits}/${revision.expectedUnits} units · ${revision.indexedChunks} indexed'
                          '${active ? '' : '\n${_revisionRetentionLabel(revision)}'}'
                          '${revision.lastError.isEmpty ? '' : '\n${revision.lastError}'}',
                        ),
                        trailing: active
                            ? const Chip(label: Text('活动'))
                            : PopupMenuButton<String>(
                                tooltip: '版本操作',
                                onSelected: (value) {
                                  Navigator.pop(context);
                                  if (value == 'rollback') {
                                    unawaited(
                                      _rollbackRevision(
                                        detail.source,
                                        revision,
                                      ),
                                    );
                                  } else if (value == 'purge') {
                                    unawaited(
                                      _purgeRevision(detail.source, revision),
                                    );
                                  }
                                },
                                itemBuilder: (context) => [
                                  PopupMenuItem(
                                    value: 'rollback',
                                    enabled: revision.canRollback,
                                    child: const Text('回滚到此版本'),
                                  ),
                                  PopupMenuItem(
                                    value: 'purge',
                                    enabled: _canPurgeRevision(revision),
                                    child: const Text('永久清理'),
                                  ),
                                ],
                              ),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        ),
      );
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _showSourceOutline(
    KnowledgeSourceDto source,
    int revision,
  ) async {
    final items = <KnowledgeOutlineItemDto>[];
    int? nextAfterOrdinal;
    bool loading = false;
    bool started = false;
    String? error;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (context) => StatefulBuilder(
        builder: (context, setSheetState) {
          Future<void> load({required bool append}) async {
            if (loading) return;
            setSheetState(() {
              loading = true;
              error = null;
            });
            try {
              final result = await ref
                  .read(knowledgeServiceProvider)
                  .sourceOutline(
                    sourceId: source.id,
                    revision: revision,
                    afterOrdinal: append ? nextAfterOrdinal : null,
                  );
              if (!context.mounted) return;
              setSheetState(() {
                if (!append) items.clear();
                items.addAll(result.items);
                nextAfterOrdinal = result.nextAfterOrdinal;
              });
            } catch (value) {
              if (context.mounted) {
                setSheetState(() => error = value.toString());
              }
            } finally {
              if (context.mounted) setSheetState(() => loading = false);
            }
          }

          if (!started) {
            started = true;
            Future.microtask(() => load(append: false));
          }
          return SafeArea(
            child: FractionallySizedBox(
              heightFactor: 0.88,
              child: Column(
                children: [
                  ListTile(
                    title: Text(
                      source.logicalName.isEmpty
                          ? source.filename
                          : source.logicalName,
                    ),
                    subtitle: Text('Canonical Outline · r$revision'),
                    trailing: IconButton(
                      onPressed: () => Navigator.pop(context),
                      icon: const Icon(Icons.close),
                    ),
                  ),
                  const Divider(height: 1),
                  if (loading && items.isEmpty)
                    const Expanded(
                      child: Center(child: CircularProgressIndicator()),
                    )
                  else if (error != null && items.isEmpty)
                    Expanded(
                      child: AmitiaErrorState(
                        message: error!,
                        onRetry: () => load(append: false),
                      ),
                    )
                  else
                    Expanded(
                      child: ListView.separated(
                        itemCount:
                            items.length + (nextAfterOrdinal == null ? 0 : 1),
                        separatorBuilder: (_, _) => const Divider(height: 1),
                        itemBuilder: (context, index) {
                          if (index >= items.length) {
                            return Center(
                              child: TextButton(
                                onPressed: loading
                                    ? null
                                    : () => load(append: true),
                                child: const Text('加载更多'),
                              ),
                            );
                          }
                          final item = items[index];
                          return ListTile(
                            leading: Text('#${item.ordinal}'),
                            title: Text(
                              item.title.isEmpty ? item.unitType : item.title,
                            ),
                            subtitle: Text(
                              '${item.unitType} · ${const JsonEncoder().convert(item.locator)} · ${_short(item.checksum)}',
                            ),
                            trailing: TextButton(
                              onPressed: () => _readOutlineUnit(item),
                              child: const Text('读取'),
                            ),
                          );
                        },
                      ),
                    ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Future<void> _readOutlineUnit(KnowledgeOutlineItemDto item) async {
    try {
      final result = await ref
          .read(knowledgeServiceProvider)
          .read(
            KnowledgeReadRequestDto(
              unitId: item.id,
              sourceId: item.sourceId,
              sourceRevision: item.sourceRevision,
              beforeUnits: 2,
              afterUnits: 2,
              maxUnits: 8,
              maxRunes: 18000,
            ),
          );
      if (!mounted) return;
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (context) => SafeArea(
          child: FractionallySizedBox(
            heightFactor: 0.82,
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                Text(
                  '${item.title.isEmpty ? item.unitType : item.title} · r${item.sourceRevision}',
                  style: AppTypography.sectionTitle(context),
                ),
                const SizedBox(height: 8),
                Text(
                  '${result.sourceId} · ${result.totalRunes} runes${result.truncated ? ' · 已截断' : ''}',
                  style: AppTypography.caption(context),
                ),
                const Divider(),
                ...result.units.map(
                  (unit) => Card(
                    child: Padding(
                      padding: const EdgeInsets.all(12),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '${unit.title.isEmpty ? unit.unitType : unit.title} · #${unit.ordinal}',
                            style: AppTypography.bodySmall(context),
                          ),
                          const SizedBox(height: 6),
                          SelectableText(unit.content),
                        ],
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      );
    } catch (error) {
      if (mounted) amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _rollbackRevision(
    KnowledgeSourceDto source,
    KnowledgeSourceRevisionDto revision,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('回滚到 r${revision.revision}'),
        content: Text(
          '回滚目标会先重建并校验二级索引。切换完成前，当前活动版本 r${source.revision} 仍继续提供检索。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('开始回滚'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref
          .read(knowledgeServiceProvider)
          .rollbackRevision(source.id, revision.revision);
      if (!mounted) return;
      amitiaSnackBar(context, '回滚 r${revision.revision} 任务已入队');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _purgeRevision(
    KnowledgeSourceDto source,
    KnowledgeSourceRevisionDto revision,
  ) async {
    if (!_canPurgeRevision(revision)) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('永久清理 r${revision.revision}'),
        content: const Text('该操作会删除此历史版本的解析数据、原文件和二级索引，完成后不能再回滚。'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('永久清理'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref
          .read(knowledgeServiceProvider)
          .purgeRevision(source.id, revision.revision);
      if (!mounted) return;
      amitiaSnackBar(context, 'r${revision.revision} 已进入清理流程');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _garbageCollect() async {
    if (_collecting) return;
    setState(() => _collecting = true);
    try {
      final purged = await ref.read(knowledgeServiceProvider).garbageCollect();
      if (!mounted) return;
      amitiaSnackBar(
        context,
        purged > 0 ? '已清理 $purged 个过期历史版本' : '没有达到清理条件的历史版本',
      );
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _collecting = false);
    }
  }

  bool _canPurgeRevision(KnowledgeSourceRevisionDto revision) =>
      revision.canPurge;

  String _revisionRetentionLabel(KnowledgeSourceRevisionDto revision) {
    if (revision.purgedAt.isNotEmpty || revision.status == 'purged') {
      return '已清理 ${_formatIso(revision.purgedAt.isEmpty ? revision.updatedAt : revision.purgedAt)}';
    }
    final retainUntil = DateTime.tryParse(revision.retainUntil);
    if (retainUntil != null) {
      final expired = !retainUntil.isAfter(DateTime.now().toUtc());
      return '${expired ? '可清理' : '保留至'} ${_formatIso(revision.retainUntil)}';
    }
    return '保留中';
  }

  Future<String?> _askLogicalName(String filename) async {
    final controller = TextEditingController(text: filename);
    final value = await showDialog<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('添加知识源'),
        content: TextField(
          controller: controller,
          autofocus: true,
          decoration: const InputDecoration(
            labelText: '显示名称',
            hintText: '留空则使用原文件名',
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, controller.text.trim()),
            child: const Text('上传'),
          ),
        ],
      ),
    );
    controller.dispose();
    return value;
  }

  Future<void> _delete(KnowledgeSourceDto source) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('删除知识源'),
        content: Text(
          '确认删除“${source.logicalName.isEmpty ? source.filename : source.logicalName}”？\n删除后将从聊天检索范围移除。',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('删除'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;
    try {
      await ref.read(knowledgeServiceProvider).deleteSource(source.id);
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _retry(KnowledgeJobDto job) async {
    try {
      await ref.read(knowledgeServiceProvider).retryJob(job.id);
      if (!mounted) return;
      amitiaSnackBar(context, '任务已重新入队');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _cancel(KnowledgeJobDto job) async {
    try {
      await ref.read(knowledgeServiceProvider).cancelJob(job.id);
      if (!mounted) return;
      amitiaSnackBar(context, '任务已取消');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    }
  }

  Future<void> _reconcile() async {
    if (_reconciling) return;
    setState(() => _reconciling = true);
    try {
      await ref.read(knowledgeServiceProvider).reconcile();
      if (!mounted) return;
      amitiaSnackBar(context, '一致性检查已完成');
      await _load(showLoading: false);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _reconciling = false);
    }
  }

  Future<void> _search() async {
    final query = _queryController.text.trim();
    if (query.isEmpty || _searching) return;
    if (_temporalMode == 'historical' && _historicalAsOf == null) {
      amitiaSnackBar(context, '请选择历史查询时点');
      return;
    }
    setState(() => _searching = true);
    try {
      final result = await ref
          .read(knowledgeServiceProvider)
          .search(
            query,
            retrievalLevel: _retrievalLevel,
            temporalMode: _temporalMode,
            asOf: _temporalMode == 'historical' ? _historicalAsOf : null,
          );
      if (!mounted) return;
      setState(() => _searchResult = result);
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _searching = false);
    }
  }

  Future<void> _showEvidenceRead(KnowledgeEvidenceDto item) async {
    if (_readingEvidenceId.isNotEmpty) return;
    setState(() => _readingEvidenceId = item.id);
    try {
      final result = await ref
          .read(knowledgeServiceProvider)
          .read(
            KnowledgeReadRequestDto(
              evidenceId: item.id,
              unitId: item.unitId,
              sourceId: item.sourceId,
              sourceRevision: item.sourceRevision,
              beforeUnits: 2,
              afterUnits: 2,
              maxUnits: 8,
              maxRunes: 18000,
            ),
          );
      if (!mounted) return;
      await showModalBottomSheet<void>(
        context: context,
        isScrollControlled: true,
        builder: (context) => SafeArea(
          child: FractionallySizedBox(
            heightFactor: 0.88,
            child: Column(
              children: [
                ListTile(
                  title: Text(item.title.isEmpty ? '知识原文' : item.title),
                  subtitle: Text(
                    'source ${result.sourceId} · r${result.sourceRevision} · ${result.totalRunes} runes'
                    '${result.truncated ? ' · 已按预算截断' : ''}',
                  ),
                  trailing: IconButton(
                    tooltip: '关闭',
                    icon: const Icon(Icons.close),
                    onPressed: () => Navigator.pop(context),
                  ),
                ),
                const Divider(height: 1),
                Expanded(
                  child: result.units.isEmpty
                      ? const Center(child: Text('没有可读取的原文单元'))
                      : ListView.builder(
                          padding: const EdgeInsets.all(12),
                          itemCount: result.units.length,
                          itemBuilder: (context, index) {
                            final unit = result.units[index];
                            final anchor = unit.id == result.anchorUnitId;
                            return Card(
                              margin: const EdgeInsets.only(bottom: 10),
                              color: anchor
                                  ? Theme.of(context)
                                        .colorScheme
                                        .primaryContainer
                                        .withValues(alpha: 0.28)
                                  : null,
                              child: Padding(
                                padding: const EdgeInsets.all(12),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Row(
                                      children: [
                                        Expanded(
                                          child: Text(
                                            unit.title.isEmpty
                                                ? (unit.unitType.isEmpty
                                                      ? '原文单元'
                                                      : unit.unitType)
                                                : unit.title,
                                            style: const TextStyle(
                                              fontWeight: FontWeight.w600,
                                            ),
                                          ),
                                        ),
                                        if (anchor)
                                          const Chip(label: Text('命中')),
                                      ],
                                    ),
                                    const SizedBox(height: 4),
                                    Text(
                                      '#${unit.ordinal} · ${const JsonEncoder().convert(unit.locator)}'
                                      '${unit.truncated ? ' · 单元已截断' : ''}',
                                      style: Theme.of(
                                        context,
                                      ).textTheme.bodySmall,
                                    ),
                                    const SizedBox(height: 8),
                                    SelectableText(unit.content),
                                  ],
                                ),
                              ),
                            );
                          },
                        ),
                ),
              ],
            ),
          ),
        ),
      );
    } catch (error) {
      if (!mounted) return;
      amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _readingEvidenceId = '');
    }
  }

  Future<void> _pickHistoricalAsOf() async {
    final now = DateTime.now();
    final initial = _historicalAsOf ?? now;
    final date = await showDatePicker(
      context: context,
      initialDate: initial,
      firstDate: DateTime(1970),
      lastDate: DateTime(now.year + 20),
    );
    if (date == null || !mounted) return;
    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay.fromDateTime(initial),
    );
    if (time == null || !mounted) return;
    setState(() {
      _historicalAsOf = DateTime(
        date.year,
        date.month,
        date.day,
        time.hour,
        time.minute,
      );
    });
  }

  void _showJobs() {
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (context) => SafeArea(
        child: FractionallySizedBox(
          heightFactor: 0.78,
          child: Column(
            children: [
              const ListTile(
                title: Text('入库任务'),
                subtitle: Text('失败任务可从 checkpoint 重新入队'),
              ),
              const Divider(height: 1),
              Expanded(
                child: _jobs.isEmpty
                    ? const Center(child: Text('暂无任务'))
                    : ListView.separated(
                        itemCount: _jobs.length,
                        separatorBuilder: (_, _) => const Divider(height: 1),
                        itemBuilder: (context, index) {
                          final job = _jobs[index];
                          return ListTile(
                            title: Text('${job.stage} · ${job.status}'),
                            subtitle: Text(
                              '${job.kind} · source ${_short(job.sourceId)} · r${job.sourceRevision} · ${job.processedUnits}/${job.expectedUnits} · indexed ${job.indexedChunks}'
                              '${job.lastError.isEmpty ? '' : '\n${job.lastError}'}',
                            ),
                            trailing:
                                (job.status == 'failed' ||
                                    job.status == 'cancelled')
                                ? IconButton(
                                    tooltip: '重试',
                                    onPressed: () {
                                      Navigator.pop(context);
                                      unawaited(_retry(job));
                                    },
                                    icon: const Icon(Icons.refresh),
                                  )
                                : (job.status == 'queued' ||
                                      job.status == 'running')
                                ? IconButton(
                                    tooltip: '取消任务',
                                    onPressed: () {
                                      Navigator.pop(context);
                                      unawaited(_cancel(job));
                                    },
                                    icon: const Icon(
                                      Icons.stop_circle_outlined,
                                    ),
                                  )
                                : null,
                          );
                        },
                      ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '全局知识库',
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: Icons.receipt_long_outlined,
            tooltip: '入库任务',
            onPressed: _showJobs,
          ),
          AmitiaIconButton(
            icon: Icons.delete_sweep_outlined,
            tooltip: '清理过期版本',
            onPressed: _collecting ? null : _garbageCollect,
          ),
          AmitiaIconButton(
            icon: Icons.build_circle_outlined,
            tooltip: '一致性检查',
            onPressed: _reconciling ? null : _reconcile,
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: () => _load(showLoading: false),
          ),
          AmitiaIconButton(
            icon: Icons.monitor_heart_outlined,
            tooltip: '运行诊断',
            onPressed: () => Navigator.of(context).push(
              amitiaSlideRoute(
                builder: (_) => const KnowledgeDiagnosticsPage(),
              ),
            ),
          ),
          AmitiaIconButton(
            icon: Icons.upload_file,
            tooltip: '上传知识源',
            onPressed: _uploading ? null : _upload,
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: _loading
            ? const AmitiaLoadingState(message: '加载知识库…')
            : _error != null
            ? AmitiaErrorState(message: _error!, onRetry: _load)
            : RefreshIndicator(
                onRefresh: () => _load(showLoading: false),
                child: ListView(
                  padding: EdgeInsets.fromLTRB(
                    AppSpacing.pagePadding,
                    AppSpacing.sm,
                    AppSpacing.pagePadding,
                    AppSpacing.xl,
                  ),
                  children: [
                    _BoundaryCard(),
                    const SizedBox(height: 12),
                    _buildSearchCard(),
                    const SizedBox(height: 12),
                    if (_sources.isEmpty)
                      const Padding(
                        padding: EdgeInsets.symmetric(vertical: 48),
                        child: Center(child: Text('暂无知识源，点击右上角上传文件')),
                      )
                    else
                      ..._sources.map(_buildSourceCard),
                  ],
                ),
              ),
      ),
    );
  }

  Widget _buildSearchCard() {
    final result = _searchResult;
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('检索验证', style: TextStyle(fontWeight: FontWeight.w600)),
            const SizedBox(height: 8),
            Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _queryController,
                    textInputAction: TextInputAction.search,
                    onSubmitted: (_) => unawaited(_search()),
                    decoration: const InputDecoration(
                      hintText: '输入问题，验证聊天使用的同一检索运行时',
                      prefixIcon: Icon(Icons.search),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                FilledButton(
                  onPressed: _searching ? null : _search,
                  child: _searching
                      ? const SizedBox.square(
                          dimension: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('检索'),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                DropdownButton<String>(
                  value: _retrievalLevel,
                  items: const [
                    DropdownMenuItem(value: 'auto', child: Text('聊天自动路由')),
                    DropdownMenuItem(value: 'none', child: Text('NONE')),
                    DropdownMenuItem(value: 'fast', child: Text('FAST')),
                    DropdownMenuItem(value: 'deep', child: Text('DEEP')),
                    DropdownMenuItem(value: 'agentic', child: Text('AGENTIC')),
                  ],
                  onChanged: _searching
                      ? null
                      : (value) {
                          if (value != null) {
                            setState(() => _retrievalLevel = value);
                          }
                        },
                ),
                DropdownButton<String>(
                  value: _temporalMode,
                  items: const [
                    DropdownMenuItem(value: 'current', child: Text('当前有效')),
                    DropdownMenuItem(value: 'historical', child: Text('历史时点')),
                    DropdownMenuItem(value: 'all', child: Text('全部时段')),
                  ],
                  onChanged: _searching
                      ? null
                      : (value) {
                          if (value != null) {
                            setState(() => _temporalMode = value);
                          }
                        },
                ),
                if (_temporalMode == 'historical')
                  OutlinedButton.icon(
                    onPressed: _searching ? null : _pickHistoricalAsOf,
                    icon: const Icon(Icons.schedule),
                    label: Text(
                      _historicalAsOf == null
                          ? '选择历史时点'
                          : _formatDateTime(_historicalAsOf!),
                    ),
                  ),
              ],
            ),
            if (result != null) ...[
              const SizedBox(height: 12),
              Text(
                '命中 ${result.evidence.length} 条 · ${result.routingMode == 'chat' ? '聊天路由' : '诊断指定'} · ${result.retrievalLevel.toUpperCase()} · policy=${result.policyMode} · ${result.queryComplexity} · ${result.answerState == 'ambiguous' ? '证据冲突' : (result.sufficient ? '证据充分' : '允许无证据/证据不足')}'
                '${result.sufficiencyReason.isEmpty ? '' : ' · gate=${result.sufficiencyReason}'}'
                '${result.degraded ? ' · 降级检索' : ''}'
                '${result.routeDegraded ? ' · 路由降级(${result.routeDegradationReason})' : ''}'
                '${result.requestedRetrievalLevel == result.retrievalLevel ? '' : ' · 请求 ${result.requestedRetrievalLevel} → 实际 ${result.retrievalLevel}'}'
                '${result.shadowLevel.isEmpty ? '' : ' · shadow ${result.shadowLevel}'}'
                '${result.ambiguous ? ' · 存在冲突证据' : ''}'
                ' · ${_temporalModeLabel(result.temporalMode)}'
                '${result.asOf.isEmpty ? '' : ' · asOf ${_formatIso(result.asOf)}'}',
              ),
              if (result.warnings.isNotEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(
                    result.warnings.join('；'),
                    style: Theme.of(context).textTheme.bodySmall,
                  ),
                ),
              ...result.evidence.map(_buildEvidence),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildEvidence(KnowledgeEvidenceDto item) {
    final locator = const JsonEncoder().convert(item.locator);
    return Container(
      margin: const EdgeInsets.only(top: 10),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        border: Border.all(color: Theme.of(context).dividerColor),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            item.title.isEmpty ? '未命名证据' : item.title,
            style: const TextStyle(fontWeight: FontWeight.w600),
          ),
          const SizedBox(height: 4),
          Text(
            'revision ${item.sourceRevision}'
            '${item.conflictGroup.isEmpty ? '' : ' · conflict ${item.conflictGroup}'}'
            '${item.validFrom.isEmpty ? '' : ' · from ${_formatIso(item.validFrom)}'}'
            '${item.validTo.isEmpty ? '' : ' · to ${_formatIso(item.validTo)}'}'
            ' · $locator',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 6),
          Text(item.content, maxLines: 6, overflow: TextOverflow.ellipsis),
          const SizedBox(height: 6),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton.icon(
              onPressed: _readingEvidenceId.isNotEmpty
                  ? null
                  : () => _showEvidenceRead(item),
              icon: _readingEvidenceId == item.id
                  ? const SizedBox.square(
                      dimension: 14,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Icon(Icons.article_outlined),
              label: const Text('读取原文'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSourceCard(KnowledgeSourceDto source) {
    final title = source.logicalName.isEmpty
        ? source.filename
        : source.logicalName;
    final job = _latestJob(source.id);
    return Card(
      margin: const EdgeInsets.only(bottom: 10),
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Icon(Icons.description_outlined),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        title,
                        style: const TextStyle(fontWeight: FontWeight.w600),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        '${source.filename} · rev ${source.revision} · ${_formatBytes(source.sizeBytes)}',
                        style: Theme.of(context).textTheme.bodySmall,
                      ),
                    ],
                  ),
                ),
                _StatusChip(status: source.status),
                PopupMenuButton<String>(
                  tooltip: '知识源操作',
                  onSelected: (value) {
                    switch (value) {
                      case 'detail':
                        unawaited(_showSourceDetail(source));
                        break;
                      case 'revision':
                        unawaited(_uploadRevision(source));
                        break;
                      case 'reindex':
                        unawaited(_reindex(source));
                        break;
                      case 'delete':
                        unawaited(_delete(source));
                        break;
                    }
                  },
                  itemBuilder: (context) => [
                    const PopupMenuItem(value: 'detail', child: Text('版本详情')),
                    const PopupMenuItem(
                      value: 'revision',
                      child: Text('上传新版本'),
                    ),
                    PopupMenuItem(
                      value: 'reindex',
                      enabled: source.isReady,
                      child: const Text('重建索引'),
                    ),
                    const PopupMenuDivider(),
                    const PopupMenuItem(value: 'delete', child: Text('删除知识源')),
                  ],
                ),
              ],
            ),
            if (source.isBusy || source.expectedUnits > 0) ...[
              const SizedBox(height: 10),
              LinearProgressIndicator(
                value: source.expectedUnits > 0 ? source.progress : null,
              ),
              const SizedBox(height: 4),
              Text(
                'units ${source.processedUnits}/${source.expectedUnits} · indexed ${source.indexedChunks}',
                style: Theme.of(context).textTheme.bodySmall,
              ),
            ],
            if (source.status == 'ready_degraded')
              const Padding(
                padding: EdgeInsets.only(top: 8),
                child: Text('向量索引暂不可用，当前仍可使用 SQLite 词法检索。'),
              ),
            if (source.lastError.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(source.lastError),
              ),
            if (job != null &&
                job.sourceRevision != source.revision &&
                (job.status == 'queued' || job.status == 'running'))
              Padding(
                padding: const EdgeInsets.only(top: 8),
                child: Text(
                  '后台准备新版本 r${job.sourceRevision} · ${job.stage}；r${source.revision} 继续提供检索',
                ),
              ),
            if (job != null &&
                (job.status == 'failed' || job.status == 'cancelled'))
              Align(
                alignment: Alignment.centerRight,
                child: TextButton.icon(
                  onPressed: () => _retry(job),
                  icon: const Icon(Icons.refresh),
                  label: const Text('重试失败任务'),
                ),
              ),
            if (job != null &&
                (job.status == 'queued' || job.status == 'running'))
              Align(
                alignment: Alignment.centerRight,
                child: TextButton.icon(
                  onPressed: () => _cancel(job),
                  icon: const Icon(Icons.stop_circle_outlined),
                  label: const Text('取消任务'),
                ),
              ),
          ],
        ),
      ),
    );
  }

  KnowledgeJobDto? _latestJob(String sourceId) {
    for (final job in _jobs) {
      if (job.sourceId == sourceId) return job;
    }
    return null;
  }

  String _temporalModeLabel(String value) {
    switch (value) {
      case 'historical':
        return '历史时点';
      case 'all':
        return '全部时段';
      default:
        return '当前有效';
    }
  }

  String _formatDateTime(DateTime value) {
    final local = value.toLocal();
    String two(int n) => n.toString().padLeft(2, '0');
    return '${local.year}-${two(local.month)}-${two(local.day)} ${two(local.hour)}:${two(local.minute)}';
  }

  String _formatIso(String value) {
    final parsed = DateTime.tryParse(value);
    return parsed == null ? value : _formatDateTime(parsed);
  }

  String _short(String value) =>
      value.length <= 10 ? value : value.substring(0, 10);

  String _formatBytes(int bytes) {
    if (bytes < 1024) return '$bytes B';
    final kb = bytes / 1024;
    if (kb < 1024) return '${kb.toStringAsFixed(1)} KB';
    final mb = kb / 1024;
    if (mb < 1024) return '${mb.toStringAsFixed(1)} MB';
    return '${(mb / 1024).toStringAsFixed(1)} GB';
  }
}

class _BoundaryCard extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(14),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Icon(
              Icons.info_outline,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(width: 10),
            const Expanded(
              child: Text('知识库在当前账户/空间内全局共享，不按角色隔离；它与角色世界书、长期记忆是三个独立领域。'),
            ),
          ],
        ),
      ),
    );
  }
}

class _StatusChip extends StatelessWidget {
  const _StatusChip({required this.status});

  final String status;

  @override
  Widget build(BuildContext context) {
    final label = switch (status) {
      'pending' => '排队中',
      'processing' => '处理中',
      'ready' => '可用',
      'ready_degraded' => '降级可用',
      'failed' => '失败',
      'cancelled' => '已取消',
      'purged' => '已清理',
      'deleted' => '已删除',
      _ => status,
    };
    return Chip(label: Text(label), visualDensity: VisualDensity.compact);
  }
}
