import 'dart:convert';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/memory_service.dart';
import '../../../../core/models/memory.dart';

class _GlobalSearchRow {
  final String layer;
  final String title;
  final String detail;

  const _GlobalSearchRow({required this.layer, required this.title, required this.detail});
}

class MemoryManagerPage extends ConsumerStatefulWidget {
  const MemoryManagerPage({super.key});

  @override
  ConsumerState<MemoryManagerPage> createState() => _MemoryManagerPageState();
}

class _MemoryManagerPageState extends ConsumerState<MemoryManagerPage> {
  bool _batchMode = false;
  final Set<String> _selected = {};
  String _typeFilter = '';
  String _importanceFilter = '全部';
  String _scopeFilter = '';
  int _retentionFilter = 0;
  String _decayFilter = '';
  Map<String, dynamic>? _pipelineStatus;
  bool _searchVisible = false;
  final _searchController = TextEditingController();
  String _searchQuery = '';
  String _searchMode = '混合';
  bool _searching = false;
  List<MemoryDto>? _remoteResults;

  static const Map<String, String> _memoryTypeLabels = {
    '': '全部',
    'personal_info': '个人信息',
    'hobby': '爱好',
    'preference': '偏好',
    'fact': '事实',
    'plan': '计划',
    'habit': '习惯',
    'relationship': '关系',
    'custom': '自定义',
  };
  final _types = const ['', 'personal_info', 'hobby', 'preference', 'fact', 'plan', 'habit', 'relationship', 'custom'];
  final _importances = ['全部', '高', '较高', '中', '低'];
  static const _scopes = <String, String>{'': '全部', 'character': '角色', 'user': '用户全局', 'world': '世界'};
  static const _retentions = <int, String>{0: '全部', 1: 'L1 核心', 2: 'L2 稳定', 3: 'L3 普通', 4: 'L4 弱记忆', 5: 'L5 短暂'};
  static const _decayStates = <String, String>{'': '全部', 'active': '活跃', 'fading': '淡化', 'archived': '已归档'};

  @override
  void initState() {
    super.initState();
    _loadPipelineStatus();
  }

  Future<void> _loadPipelineStatus() async {
    try {
      final status = await ref.read(systemServiceProvider).pipelineStatus();
      if (mounted) setState(() => _pipelineStatus = status);
    } catch (_) {}
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final memoriesAsync = ref.watch(memoryListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆总览',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(
            icon: _searchVisible ? Icons.close : Icons.search,
            onPressed: () => setState(() {
              _searchVisible = !_searchVisible;
              if (!_searchVisible) { _searchController.clear(); _searchQuery = ''; }
            }),
          ),
          AmitiaIconButton(
            icon: _batchMode ? Icons.check : Icons.checklist,
            onPressed: () => setState(() {
              _batchMode = !_batchMode;
              if (!_batchMode) _selected.clear();
            }),
          ),
          AmitiaIconButton(
            icon: Icons.tune,
            tooltip: '高级记忆工具',
            onPressed: () => _showAdvancedTools(context),
          ),
          AmitiaIconButton(
            icon: Icons.account_tree_outlined,
            tooltip: '记忆图谱',
            onPressed: () => context.push(AppRoutes.memoryGraph),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: memoriesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                  const SizedBox(height: 16),
                  Text('加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                    style: AppTypography.body(context).copyWith(color: context.error),
                    textAlign: TextAlign.center),
                  const SizedBox(height: 16),
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(memoryListProvider)),
                ],
              ),
            ),
          ),
          data: (memories) {
            final filtered = _filterMemories(memories);
            return Column(
              children: [
                if (_searchVisible)
                  Padding(
                    padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xs),
                    child: Row(
                      children: [
                        PopupMenuButton<String>(
                          initialValue: _searchMode,
                          onSelected: (v) => setState(() => _searchMode = v),
                          itemBuilder: (_) => const [
                            PopupMenuItem(value: '混合', child: Text('混合搜索')),
                            PopupMenuItem(value: '向量', child: Text('向量搜索')),
                            PopupMenuItem(value: '关键词', child: Text('关键词搜索')),
                            PopupMenuItem(value: '本地', child: Text('本地过滤')),
                          ],
                          child: Container(
                            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 10),
                            decoration: BoxDecoration(
                              color: context.surfaceSecondary,
                              borderRadius: AppRadius.brTag,
                              border: Border.all(color: context.borderPrimary, width: 0.5),
                            ),
                            child: Text(_searchMode, style: AppTypography.label(context)),
                          ),
                        ),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: TextField(
                            controller: _searchController,
                            onChanged: (v) {
                              setState(() => _searchQuery = v);
                              if (v.trim().isEmpty) setState(() => _remoteResults = null);
                            },
                            onSubmitted: (_) => _performSearch(),
                            decoration: InputDecoration(
                              hintText: _searchMode == '本地' ? '过滤当前记忆...' : '搜索记忆...',
                              prefixIcon: const Icon(Icons.search),
                              suffixIcon: _searching
                                  ? const Padding(padding: EdgeInsets.all(12), child: SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)))
                                  : IconButton(icon: const Icon(Icons.arrow_forward), onPressed: _performSearch),
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                if (_pipelineStatus != null)
                  Padding(
                    padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.xs, AppSpacing.pagePadding, 0),
                    child: AmitiaCard(
                      child: Row(
                        children: [
                          Icon(Icons.hub_outlined, size: 18, color: context.accentPrimary),
                          SizedBox(width: AppSpacing.sm),
                          Expanded(
                            child: Text(
                              _pipelineSummary(_pipelineStatus!),
                              style: AppTypography.caption(context),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          IconButton(icon: const Icon(Icons.refresh, size: 18), tooltip: '刷新记忆管线状态', onPressed: _loadPipelineStatus),
                        ],
                      ),
                    ),
                  ),
                _buildFilters(context),
                if (_batchMode) _buildBatchBar(context),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.memory,
                          title: '暂无记忆',
                          subtitle: '与角色对话后记忆会自动生成',
                          actionText: '新建记忆',
                          onAction: () => _showMemoryEditor(context, null),
                        )
                      : ListView.separated(
                          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
                          itemCount: filtered.length,
                          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) => _buildMemoryCard(context, filtered[index]),
                        ),
                ),
              ],
            );
          },
        ),
      ),
      floatingActionButton: FloatingActionButton(
        onPressed: () => _showMemoryEditor(context, null),
        backgroundColor: context.accentPrimary,
        child: const Icon(Icons.add, color: Colors.white),
      ),
    );
  }

  List<MemoryDto> _filterMemories(List<MemoryDto> memories) {
    final source = _remoteResults ?? memories;
    return source.where((m) {
      if (_typeFilter.isNotEmpty && m.type != _typeFilter) return false;
      if (_scopeFilter.isNotEmpty && m.scope != _scopeFilter) return false;
      if (_retentionFilter != 0 && m.retentionLevel != _retentionFilter) return false;
      if (_decayFilter.isNotEmpty && m.decayState != _decayFilter) return false;
      if (_importanceFilter != '全部') {
        final impStr = _importanceIntToString(m.importance);
        if (impStr != _importanceFilter) return false;
      }
      if (_searchMode == '本地' && _searchQuery.isNotEmpty && !m.content.toLowerCase().contains(_searchQuery.toLowerCase())) return false;
      return true;
    }).toList();
  }

  String _pipelineSummary(Map<String, dynamic> status) {
    final layers = status['layers'];
    if (layers is List && layers.isNotEmpty) {
      final completed = layers.whereType<Map>().where((row) => (row['status'] ?? '').toString() == 'completed').length;
      final failed = layers.whereType<Map>().where((row) => (row['status'] ?? '').toString() == 'failed').length;
      return '记忆管线：$completed/${layers.length} 层完成${failed > 0 ? ' · $failed 层失败' : ''}';
    }
    final state = (status['status'] ?? status['state'] ?? '可用').toString();
    return '记忆管线：$state';
  }

  String _scopeLabel(String scope) => _scopes[scope] ?? (scope.isEmpty ? '角色' : scope);

  String _importanceIntToString(int importance) {
    if (importance >= 9) return '高';
    if (importance >= 7) return '较高';
    if (importance >= 4) return '中';
    return '低';
  }

  int _importanceStringToInt(String importance) {
    switch (importance) {
      case '高': return 10;
      case '较高': return 8;
      case '中': return 5;
      default: return 2;
    }
  }

  Widget _buildFilters(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: [
            _buildFilterChip(
              context,
              '类型: ${_memoryTypeLabel(_typeFilter)}',
              _types,
              (v) => setState(() => _typeFilter = v),
              optionLabel: _memoryTypeLabel,
            ),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(context, '重要度: $_importanceFilter', _importances, (v) => setState(() => _importanceFilter = v)),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(
              context,
              '范围: ${_scopes[_scopeFilter] ?? _scopeFilter}',
              _scopes.keys.toList(growable: false),
              (v) => setState(() => _scopeFilter = v),
              optionLabel: (value) => _scopes[value] ?? value,
            ),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(
              context,
              '层级: ${_retentions[_retentionFilter] ?? '全部'}',
              _retentions.keys.map((e) => e.toString()).toList(growable: false),
              (v) => setState(() => _retentionFilter = int.tryParse(v) ?? 0),
              optionLabel: (value) => _retentions[int.tryParse(value) ?? 0] ?? value,
            ),
            SizedBox(width: AppSpacing.sm),
            _buildFilterChip(
              context,
              '状态: ${_decayStates[_decayFilter] ?? '全部'}',
              _decayStates.keys.toList(growable: false),
              (v) => setState(() => _decayFilter = v),
              optionLabel: (value) => _decayStates[value] ?? value,
            ),
            SizedBox(width: AppSpacing.sm),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterChip(
    BuildContext context,
    String label,
    List<String> options,
    ValueChanged<String> onSelected, {
    String Function(String value)? optionLabel,
  }) {
    return GestureDetector(
      onTap: () => _showFilterMenu(context, label, options, onSelected, optionLabel: optionLabel),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(label, style: TextStyle(fontSize: 12, color: context.textSecondary)),
            const SizedBox(width: 4),
            Icon(Icons.arrow_drop_down, size: 16, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Future<void> _performSearch() async {
    final query = _searchController.text.trim();
    setState(() => _searchQuery = query);
    if (query.isEmpty || _searchMode == '本地') {
      setState(() => _remoteResults = null);
      return;
    }
    setState(() => _searching = true);
    try {
      final svc = ref.read(memoryServiceProvider);
      final results = switch (_searchMode) {
        '向量' => await svc.vectorSearch(query),
        '关键词' => await svc.search(query),
        _ => await svc.hybridSearch(query),
      };
      if (mounted) setState(() => _remoteResults = results);
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('搜索失败: ${e.toString().replaceFirst('Exception: ', '')}')));
    } finally {
      if (mounted) setState(() => _searching = false);
    }
  }

  Future<void> _showAdvancedTools(BuildContext context) async {
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: EdgeInsets.all(AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('高级记忆工具', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              ListTile(leading: const Icon(Icons.manage_search_outlined), title: const Text('跨记忆层全局搜索'), onTap: () { Navigator.pop(ctx); _showGlobalSearch(); }),
              ListTile(leading: const Icon(Icons.analytics_outlined), title: const Text('向量与检索诊断'), onTap: () { Navigator.pop(ctx); _showDiagnostics(); }),
              ListTile(leading: const Icon(Icons.pending_actions_outlined), title: const Text('候选记忆管理'), onTap: () { Navigator.pop(ctx); _showCandidates(); }),
              ListTile(leading: const Icon(Icons.auto_awesome_outlined), title: const Text('从会话生成候选'), onTap: () async { Navigator.pop(ctx); await _generateCandidates(); }),
              ListTile(leading: const Icon(Icons.download_outlined), title: const Text('导出当前记忆'), onTap: () async { Navigator.pop(ctx); await _exportMemories(); }),
              ListTile(leading: const Icon(Icons.sort), title: const Text('查看检索排序结果'), onTap: () { Navigator.pop(ctx); _showRanked(); }),
              ListTile(leading: const Icon(Icons.replay), title: const Text('重建向量嵌入'), onTap: () async { Navigator.pop(ctx); await _runMaintenance('正在重建向量嵌入', () => ref.read(memoryServiceProvider).rebuildEmbeddings()); }),
              ListTile(leading: const Icon(Icons.reorder), title: const Text('重建记忆索引'), onTap: () async { Navigator.pop(ctx); await _runMaintenance('正在重建记忆索引', () => ref.read(memoryServiceProvider).rebuildIndex()); }),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _showGlobalSearch() async {
    final controller = TextEditingController(text: _searchController.text.trim());
    final query = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('跨记忆层全局搜索'),
        content: SizedBox(
          width: 560,
          child: TextField(
            controller: controller,
            autofocus: true,
            decoration: const InputDecoration(
              hintText: '同时搜索长期记忆、用户画像、情景记忆和世界书',
              prefixIcon: Icon(Icons.search),
            ),
            onSubmitted: (value) => Navigator.pop(ctx, value.trim()),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(ctx, controller.text.trim()), child: const Text('搜索')),
        ],
      ),
    );
    controller.dispose();
    if (query == null || query.trim().isEmpty || !mounted) return;

    try {
      final q = query.trim();
      final memories = await ref.read(memoryServiceProvider).hybridSearch(q, limit: 5);
      final profiles = await ref.read(profileServiceProvider).list(keyword: q, pageSize: 5);
      final episodics = await ref.read(episodicServiceProvider).list(keyword: q, pageSize: 5);
      final worldBooks = await ref.read(worldBookServiceProvider).list(keyword: q, pageSize: 5);
      if (!mounted) return;

      final rows = <_GlobalSearchRow>[
        ...memories.map((item) => _GlobalSearchRow(
              layer: '长期记忆',
              title: item.key.isEmpty ? item.content : item.key,
              detail: item.content,
            )),
        ...profiles.map((item) => _GlobalSearchRow(
              layer: '用户画像',
              title: item.attributeName,
              detail: item.attributeValue,
            )),
        ...episodics.map((item) => _GlobalSearchRow(
              layer: '情景记忆',
              title: item.title,
              detail: item.content,
            )),
        ...worldBooks.map((item) => _GlobalSearchRow(
              layer: '世界书',
              title: item.matchPattern,
              detail: item.injectContent,
            )),
      ];

      showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
          title: Text('“$q” 的全局搜索结果'),
          content: SizedBox(
            width: 680,
            height: 520,
            child: rows.isEmpty
                ? const Center(child: Text('没有匹配结果'))
                : ListView.separated(
                    itemCount: rows.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (_, index) {
                      final row = rows[index];
                      return ListTile(
                        dense: true,
                        leading: Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Text(row.layer, style: AppTypography.caption(context).copyWith(color: context.accentPrimary)),
                        ),
                        title: Text(row.title.isEmpty ? '未命名' : row.title, maxLines: 1, overflow: TextOverflow.ellipsis),
                        subtitle: Text(row.detail, maxLines: 3, overflow: TextOverflow.ellipsis),
                      );
                    },
                  ),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('全局搜索失败: ${e.toString().replaceFirst('Exception: ', '')}')),
        );
      }
    }
  }

  Future<void> _showDiagnostics() async {
    try {
      final svc = ref.read(memoryServiceProvider);
      final status = await svc.vectorStatus() ?? <String, dynamic>{};
      final stats = await svc.retrievalStats();
      if (!mounted) return;
      showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('记忆诊断'),
          content: SingleChildScrollView(child: SelectableText('向量状态\n${_prettyMap(status)}\n\n检索统计\n${_prettyMap(stats)}')),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('诊断加载失败: $e')));
    }
  }

  Future<void> _showRanked() async {
    try {
      final rows = await ref.read(memoryServiceProvider).ranked(query: _searchController.text.trim(), limit: 30);
      if (!mounted) return;
      showDialog(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('检索排序'),
          content: SizedBox(
            width: 620,
            child: rows.isEmpty
                ? const Text('暂无排序结果')
                : ListView.separated(
                    shrinkWrap: true,
                    itemCount: rows.length,
                    separatorBuilder: (_, _) => const Divider(height: 1),
                    itemBuilder: (_, i) {
                      final row = rows[i];
                      final memory = row['memory'] is Map ? Map<String, dynamic>.from(row['memory'] as Map) : <String, dynamic>{};
                      return ListTile(
                        dense: true,
                        title: Text((memory['value'] ?? memory['key'] ?? '').toString(), maxLines: 2, overflow: TextOverflow.ellipsis),
                        subtitle: Text('score=${row['finalScore'] ?? '-'} · vector=${row['vectorScore'] ?? '-'} · keyword=${row['keywordScore'] ?? '-'}'),
                      );
                    },
                  ),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭'))],
        ),
      );
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('排序加载失败: $e')));
    }
  }

  Future<void> _generateCandidates() async {
    try {
      final conversations = await ref.read(chatServiceProvider).listConversations();
      if (!mounted) return;
      if (conversations.isEmpty) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('暂无可用于提取记忆的会话')));
        return;
      }
      final conversationId = await showDialog<String>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('选择来源会话'),
          content: SizedBox(
            width: 620,
            height: 440,
            child: ListView.separated(
              itemCount: conversations.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (_, index) {
                final conversation = conversations[index];
                final title = conversation.title.trim().isEmpty ? '未命名会话' : conversation.title.trim();
                return ListTile(
                  leading: const Icon(Icons.chat_bubble_outline),
                  title: Text(title, maxLines: 1, overflow: TextOverflow.ellipsis),
                  subtitle: Text('${conversation.messageCount} 条消息${conversation.updatedAt.isEmpty ? '' : ' · ${conversation.updatedAt}'}'),
                  onTap: () => Navigator.pop(ctx, conversation.id),
                );
              },
            ),
          ),
          actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消'))],
        ),
      );
      if (conversationId == null || conversationId.isEmpty) return;
      final candidates = await ref.read(memoryServiceProvider).generateCandidates(conversationId);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已从会话生成 ${candidates.length} 条候选记忆')));
      await _showCandidates();
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('候选生成失败: $e')));
    }
  }

  Future<void> _exportMemories() async {
    try {
      final memories = await ref.read(memoryListProvider.future);
      if (memories.isEmpty) {
        if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('当前没有可导出的记忆')));
        return;
      }
      final now = DateTime.now();
      final stamp = '${now.year}${now.month.toString().padLeft(2, '0')}${now.day.toString().padLeft(2, '0')}_${now.hour.toString().padLeft(2, '0')}${now.minute.toString().padLeft(2, '0')}';
      final path = await FilePicker.platform.saveFile(
        dialogTitle: '导出当前记忆',
        fileName: 'amitia_memories_$stamp.json',
        type: FileType.custom,
        allowedExtensions: const ['json'],
      );
      if (path == null || path.trim().isEmpty) return;
      final payload = <String, dynamic>{
        'schemaVersion': 1,
        'exportedAt': now.toUtc().toIso8601String(),
        'count': memories.length,
        'memories': memories.map((memory) => memory.toJson()).toList(growable: false),
      };
      await File(path).writeAsString(const JsonEncoder.withIndent('  ').convert(payload), flush: true);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已导出 ${memories.length} 条记忆')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('记忆导出失败: $e')));
    }
  }

  Future<void> _showCandidates() async {
    try {
      final svc = ref.read(memoryServiceProvider);
      var candidates = await svc.listCandidates();
      if (!mounted) return;
      await showDialog(
        context: context,
        builder: (ctx) => StatefulBuilder(
          builder: (ctx, setDialogState) => AlertDialog(
            title: Text('候选记忆 (${candidates.length})'),
            content: SizedBox(
              width: 680,
              height: 480,
              child: candidates.isEmpty
                  ? const Center(child: Text('暂无候选记忆'))
                  : ListView.separated(
                      itemCount: candidates.length,
                      separatorBuilder: (_, _) => const Divider(height: 1),
                      itemBuilder: (_, index) {
                        final c = candidates[index];
                        return ListTile(
                          title: Text(c.content, maxLines: 2, overflow: TextOverflow.ellipsis),
                          subtitle: Text('${c.memoryType} · 重要度 ${c.importance} · ${(c.confidence * 100).toStringAsFixed(0)}%${c.reason.isEmpty ? '' : ' · ${c.reason}'}'),
                          trailing: Wrap(
                            spacing: 2,
                            children: [
                              IconButton(icon: const Icon(Icons.edit_outlined), tooltip: '编辑', onPressed: () async { await _editCandidate(c); candidates = await svc.listCandidates(); if (ctx.mounted) setDialogState(() {}); }),
                              IconButton(icon: const Icon(Icons.check), tooltip: '接受', onPressed: () async { await svc.acceptCandidate(c.id); candidates = await svc.listCandidates(); ref.invalidate(memoryListProvider); if (ctx.mounted) setDialogState(() {}); }),
                              IconButton(icon: const Icon(Icons.close), tooltip: '拒绝', onPressed: () async { await svc.rejectCandidate(c.id); candidates = await svc.listCandidates(); if (ctx.mounted) setDialogState(() {}); }),
                              IconButton(icon: const Icon(Icons.delete_outline), tooltip: '删除', onPressed: () async { await svc.deleteCandidate(c.id); candidates = await svc.listCandidates(); if (ctx.mounted) setDialogState(() {}); }),
                            ],
                          ),
                        );
                      },
                    ),
            ),
            actions: [
              if (candidates.isNotEmpty) TextButton(onPressed: () async { await svc.batchAcceptCandidates(candidates.map((e) => e.id).toList()); ref.invalidate(memoryListProvider); candidates = await svc.listCandidates(); if (ctx.mounted) setDialogState(() {}); }, child: const Text('全部接受')),
              TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
            ],
          ),
        ),
      );
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('候选加载失败: $e')));
    }
  }

  Future<void> _editCandidate(MemoryCandidateDto candidate) async {
    final key = TextEditingController(text: candidate.key);
    final value = TextEditingController(text: candidate.value);
    final result = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('编辑候选记忆'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextField(controller: key, decoration: const InputDecoration(labelText: 'Key')),
            TextField(controller: value, maxLines: 4, decoration: const InputDecoration(labelText: '内容')),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('保存')),
        ],
      ),
    );
    if (result == true) await ref.read(memoryServiceProvider).updateCandidate(candidate.id, key: key.text.trim(), value: value.text.trim());
    key.dispose();
    value.dispose();
  }

  Future<void> _runMaintenance(String label, Future<dynamic> Function() action) async {
    if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('$label...')));
    try {
      await action();
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('操作完成')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: $e')));
    }
  }

  Future<void> _batchVerifySelected() async {
    try {
      await ref.read(memoryServiceProvider).batchVerify(_selected.toList());
      ref.invalidate(memoryListProvider);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('已确认选中记忆')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('批量确认失败: $e')));
    }
  }

  Future<void> _showBatchImportance(BuildContext context) async {
    final picked = await showModalBottomSheet<int>(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(title: const Text('高'), trailing: const Text('10'), onTap: () => Navigator.pop(ctx, 10)),
            ListTile(title: const Text('较高'), trailing: const Text('8'), onTap: () => Navigator.pop(ctx, 8)),
            ListTile(title: const Text('中'), trailing: const Text('5'), onTap: () => Navigator.pop(ctx, 5)),
            ListTile(title: const Text('低'), trailing: const Text('2'), onTap: () => Navigator.pop(ctx, 2)),
          ],
        ),
      ),
    );
    if (picked != null) await _setSelectedImportance(picked);
  }

  Future<void> _setSelectedImportance(int importance) async {
    try {
      await ref.read(memoryServiceProvider).batchSetImportance(_selected.toList(), importance);
      ref.invalidate(memoryListProvider);
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('重要度已更新')));
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('设置失败: $e')));
    }
  }

  Future<bool> _createWithConflictCheck(
    MemoryService svc, {
    required String content,
    required String type,
    required int importance,
    required String scope,
    required int retentionLevel,
    required bool pinned,
  }) async {
    final key = content.replaceAll(RegExp(r'\s+'), ' ').trim();
    final normalizedKey = key.length <= 60 ? key : key.substring(0, 60);
    final check = await svc.checkConflict(key: normalizedKey, value: content, memoryType: type, importance: importance);
    final hasConflict = check['hasConflict'] == true;
    final conflictsRaw = check['conflicts'];
    final conflicts = conflictsRaw is List ? conflictsRaw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];
    if (!hasConflict || conflicts.isEmpty) {
      await svc.create({
        'key': normalizedKey,
        'value': content,
        'memoryType': type,
        'importance': importance,
        'verifiedStatus': 'user_verified',
        'scope': scope,
        if (retentionLevel > 0) 'retentionLevel': retentionLevel,
        'pinned': pinned,
      });
      return true;
    }
    if (!mounted) return false;
    final first = conflicts.first;
    final memory = first['memory'] is Map ? Map<String, dynamic>.from(first['memory'] as Map) : <String, dynamic>{};
    final action = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('发现记忆冲突'),
        content: Text('现有记忆：${memory['value'] ?? memory['key'] ?? ''}\n\n新记忆：$content\n\n原因：${first['reason'] ?? ''}'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(ctx, 'keep_existing'), child: const Text('保留旧记忆')),
          TextButton(onPressed: () => Navigator.pop(ctx, 'keep_both'), child: const Text('两条都保留')),
          TextButton(onPressed: () => Navigator.pop(ctx, 'merge'), child: const Text('合并')),
          FilledButton(onPressed: () => Navigator.pop(ctx, 'replace'), child: const Text('用新记忆替换')),
        ],
      ),
    );
    if (action == null) return false;
    final resolved = await svc.resolveConflict(
      action: action,
      newKey: normalizedKey,
      newValue: content,
      newType: type,
      importance: importance,
      conflictId: (memory['id'] ?? '').toString(),
      characterId: (memory['characterId'] ?? '').toString(),
    );
    final resolvedId = (resolved['memoryId'] ?? resolved['memoryID'] ?? '').toString().trim();
    if (resolvedId.isNotEmpty) {
      await svc.update(resolvedId, {
        if (scope != 'character') 'scope': scope,
        if (retentionLevel > 0) 'retentionLevel': retentionLevel,
        'pinned': pinned,
      });
    }
    return true;
  }

  String _prettyMap(Map<String, dynamic> map) {
    if (map.isEmpty) return '无数据';
    return map.entries.map((e) => '${e.key}: ${e.value}').join('\n');
  }

  String _memoryTypeLabel(String value) => _memoryTypeLabels[value] ?? value;

  void _showFilterMenu(
    BuildContext context,
    String title,
    List<String> options,
    ValueChanged<String> onSelected, {
    String Function(String value)? optionLabel,
  }) {
    showModalBottomSheet(
      context: context,
      builder: (ctx) => Container(
        padding: EdgeInsets.all(AppSpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: options.map((o) => GestureDetector(
                onTap: () {
                  onSelected(o);
                  Navigator.pop(ctx);
                },
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(optionLabel?.call(o) ?? o, style: TextStyle(fontSize: 14, color: context.accentPrimary)),
                ),
              )).toList(),
            ),
            SizedBox(height: AppSpacing.xl),
          ],
        ),
      ),
    );
  }

  Widget _buildBatchBar(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      color: context.accentSoft,
      child: Row(
        children: [
          Text('已选 ${_selected.length} 项', style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
          const Spacer(),
          if (_selected.isNotEmpty) ...[
            TextButton(onPressed: _batchVerifySelected, child: const Text('确认')),
            TextButton(onPressed: () => _showBatchImportance(context), child: const Text('重要度')),
            GestureDetector(
              onTap: () => _showBatchDeleteConfirm(context),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                decoration: BoxDecoration(color: context.error, borderRadius: AppRadius.brTag),
                child: const Text('批量删除', style: TextStyle(fontSize: 13, color: Colors.white)),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildMemoryCard(BuildContext context, MemoryDto memory) {
    final isSelected = _selected.contains(memory.id);
    return AmitiaCard(
      border: Border.all(
        color: _batchMode && isSelected ? context.accentPrimary : context.borderPrimary,
        width: _batchMode && isSelected ? 1.5 : 0.5,
      ),
      onTap: () {
        if (_batchMode) {
          setState(() {
            if (isSelected) {
              _selected.remove(memory.id);
            } else {
              _selected.add(memory.id);
            }
          });
        } else {
          _showMemoryEditor(context, memory);
        }
      },
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (_batchMode)
            Padding(
              padding: EdgeInsets.only(top: 2, right: AppSpacing.sm),
              child: Icon(
                isSelected ? Icons.check_circle : Icons.radio_button_unchecked,
                size: 20,
                color: isSelected ? context.accentPrimary : context.textTertiary,
              ),
            ),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(child: Text(memory.content, style: AppTypography.body(context), maxLines: 2, overflow: TextOverflow.ellipsis)),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                Wrap(
                  spacing: AppSpacing.sm,
                  runSpacing: AppSpacing.xs,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [
                    AmitiaStatusBadge(label: _importanceIntToString(memory.importance), type: _importanceToBadgeType(memory.importance)),
                    AmitiaStatusBadge(label: _memoryTypeLabel(memory.type), type: BadgeType.neutral),
                    AmitiaStatusBadge(
                      label: 'L${memory.retentionLevel} · ${(memory.memoryStrength * 100).round()}%${memory.pinned ? ' · 固定' : memory.decayState == 'archived' ? ' · 归档' : memory.decayState == 'fading' ? ' · 淡化' : ''}',
                      type: memory.decayState == 'archived' ? BadgeType.neutral : memory.retentionLevel <= 2 ? BadgeType.success : BadgeType.neutral,
                    ),
                    AmitiaStatusBadge(label: _scopeLabel(memory.scope), type: memory.scope == 'world' || memory.scope == 'user' ? BadgeType.success : BadgeType.neutral),
                    Text(memory.status, style: AppTypography.label(context)),
                    Text(_formatTimeString(memory.createdAt), style: AppTypography.label(context)),
                  ],
                ),
                SizedBox(height: AppSpacing.xs),
                Text(
                  '强化 ${memory.reinforceCount} 次 · 召回 ${memory.retrievedCount} · 注入 ${memory.injectedCount}${memory.lastReinforcedAt != null && memory.lastReinforcedAt!.isNotEmpty ? ' · 上次强化 ${_formatTimeString(memory.lastReinforcedAt!)}' : ''}',
                  style: AppTypography.caption(context).copyWith(color: context.textTertiary),
                ),
                if (!_batchMode) ...[
                  SizedBox(height: AppSpacing.sm),
                  Wrap(
                    spacing: AppSpacing.sm,
                    runSpacing: AppSpacing.xs,
                    children: [
                      GestureDetector(
                        onTap: () => _showMemoryEditor(context, memory),
                        child: _buildMiniButton(context, '编辑', context.accentPrimary),
                      ),
                      GestureDetector(
                        onTap: () => _togglePinned(memory),
                        child: _buildMiniButton(context, memory.pinned ? '取消固定' : '固定', context.accentPrimary),
                      ),
                      if (memory.decayState == 'archived')
                        GestureDetector(
                          onTap: () => _restoreMemory(memory),
                          child: _buildMiniButton(context, '恢复', context.success),
                        ),
                      GestureDetector(
                        onTap: () => _showDeleteConfirm(context, memory),
                        child: _buildMiniButton(context, '删除', context.error),
                      ),
                    ],
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMiniButton(BuildContext context, String label, Color color) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 3),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Text(label, style: TextStyle(fontSize: 12, color: color)),
    );
  }

  void _showMemoryEditor(BuildContext context, MemoryDto? existing) {
    final isEdit = existing != null;
    final contentCtrl = TextEditingController(text: existing?.content ?? '');
    String importance = existing != null ? _importanceIntToString(existing.importance) : '中';
    String type = _memoryTypeLabels.containsKey(existing?.type) ? (existing?.type ?? 'fact') : 'fact';
    String scope = _scopes.containsKey(existing?.scope) ? (existing?.scope ?? 'character') : 'character';
    int retentionLevel = existing?.retentionLevel ?? 3;
    bool pinned = existing?.pinned ?? false;

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.xl, AppSpacing.lg, AppSpacing.xl, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑记忆' : '新建记忆', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('记忆内容', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: contentCtrl, maxLines: 4, hintText: '输入记忆内容'),
              SizedBox(height: AppSpacing.md),
              Text('重要程度', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['高', '较高', '中', '低'].map((i) {
                  final isSelected = importance == i;
                  return GestureDetector(
                    onTap: () => setSheetState(() => importance = i),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(i, style: TextStyle(fontSize: 13, color: isSelected ? Colors.white : context.textSecondary)),
                    ),
                  );
                }).toList(),
              ),
              SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: _memoryTypeLabels.entries.where((entry) => entry.key.isNotEmpty).map((entry) {
                  final c = entry.key;
                  final isSelected = type == c;
                  return GestureDetector(
                    onTap: () => setSheetState(() => type = c),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(entry.value, style: TextStyle(fontSize: 13, color: isSelected ? Colors.white : context.textSecondary)),
                    ),
                  );
                }).toList(),
              ),
              SizedBox(height: AppSpacing.md),
              Text('自然遗忘层级', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                runSpacing: AppSpacing.xs,
                children: _retentions.entries.where((entry) => entry.key != 0).map((entry) {
                  final selected = retentionLevel == entry.key;
                  return ChoiceChip(
                    label: Text(entry.value),
                    selected: selected,
                    onSelected: (_) => setSheetState(() => retentionLevel = entry.key),
                  );
                }).toList(growable: false),
              ),
              SizedBox(height: AppSpacing.sm),
              SwitchListTile.adaptive(
                contentPadding: EdgeInsets.zero,
                dense: true,
                title: const Text('固定记忆'),
                subtitle: Text(
                  pinned ? '固定后不会参与自然遗忘归档' : '未固定时按 L1-L5 规则自然衰减',
                  style: AppTypography.caption(context).copyWith(color: context.textTertiary),
                ),
                value: pinned,
                onChanged: (value) => setSheetState(() => pinned = value),
              ),
              if (existing != null) ...[
                SizedBox(height: AppSpacing.sm),
                Container(
                  width: double.infinity,
                  padding: EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: context.surfaceSecondary,
                    borderRadius: AppRadius.brCard,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        '当前强度 ${(existing.memoryStrength * 100).round()}% · ${_decayStates[existing.decayState] ?? existing.decayState}',
                        style: AppTypography.bodySmall(context),
                      ),
                      SizedBox(height: AppSpacing.xs),
                      Text(
                        '强化 ${existing.reinforceCount} 次 · 召回 ${existing.retrievedCount} · 注入 ${existing.injectedCount}',
                        style: AppTypography.caption(context).copyWith(color: context.textTertiary),
                      ),
                      if (existing.lastReinforcedAt != null && existing.lastReinforcedAt!.isNotEmpty) ...[
                        SizedBox(height: AppSpacing.xs),
                        Text(
                          '上次强化：${_formatTimeString(existing.lastReinforcedAt!)}',
                          style: AppTypography.caption(context).copyWith(color: context.textTertiary),
                        ),
                      ],
                    ],
                  ),
                ),
              ],
              SizedBox(height: AppSpacing.md),
              Text('记忆范围', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: _scopes.entries.where((entry) => entry.key.isNotEmpty).map((entry) {
                  final selected = scope == entry.key;
                  return ChoiceChip(
                    label: Text(entry.value),
                    selected: selected,
                    onSelected: (_) => setSheetState(() => scope = entry.key),
                  );
                }).toList(growable: false),
              ),
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (contentCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(memoryServiceProvider);
                  final normalizedContent = contentCtrl.text.trim();
                  final normalizedKey = normalizedContent.replaceAll(RegExp(r'\s+'), ' ').trim();
                  final data = {
                    'key': normalizedKey.length <= 60 ? normalizedKey : normalizedKey.substring(0, 60),
                    'value': normalizedContent,
                    'memoryType': type,
                    'importance': _importanceStringToInt(importance),
                    'verifiedStatus': 'user_verified',
                    'scope': scope,
                    'retentionLevel': retentionLevel,
                    'pinned': pinned,
                  };
                  try {
                    if (isEdit) {
                      await svc.update(existing.id, data);
                    } else {
                      final handled = await _createWithConflictCheck(
                        svc,
                        content: contentCtrl.text.trim(),
                        type: type,
                        importance: _importanceStringToInt(importance),
                        scope: scope,
                        retentionLevel: retentionLevel,
                        pinned: pinned,
                      );
                      if (!handled) return;
                    }
                    ref.invalidate(memoryListProvider);
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '记忆已更新' : '记忆已创建'), duration: const Duration(seconds: 1)));
                    }
                  } catch (e) {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                    }
                  }
                },
              ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _togglePinned(MemoryDto memory) async {
    try {
      await ref.read(memoryServiceProvider).update(memory.id, {'pinned': !memory.pinned});
      ref.invalidate(memoryListProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(memory.pinned ? '已取消固定' : '记忆已固定'), duration: const Duration(seconds: 1)),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败: ${e.toString().replaceFirst('Exception: ', '')}')),
        );
      }
    }
  }

  Future<void> _restoreMemory(MemoryDto memory) async {
    try {
      await ref.read(memoryServiceProvider).restore(memory.id);
      ref.invalidate(memoryListProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('记忆已恢复'), duration: Duration(seconds: 1)),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('恢复失败: ${e.toString().replaceFirst('Exception: ', '')}')),
        );
      }
    }
  }

  void _showDeleteConfirm(BuildContext context, MemoryDto memory) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除记忆', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除这条记忆吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(memoryServiceProvider);
                await svc.delete(memory.id);
                ref.invalidate(memoryListProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('记忆已删除'), duration: Duration(seconds: 1)));
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                }
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _showBatchDeleteConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('批量删除', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除选中的 ${_selected.length} 条记忆吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(memoryServiceProvider);
                for (final id in _selected) {
                  await svc.delete(id);
                }
                ref.invalidate(memoryListProvider);
                if (mounted) {
                  setState(() { _selected.clear(); _batchMode = false; });
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('批量删除完成'), duration: Duration(seconds: 1)));
                }
              } catch (e) {
                ref.invalidate(memoryListProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                }
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  BadgeType _importanceToBadgeType(int importance) {
    if (importance >= 9) return BadgeType.error;
    if (importance >= 7) return BadgeType.warning;
    if (importance >= 4) return BadgeType.info;
    return BadgeType.neutral;
  }

  String _formatTimeString(String timeStr) {
    if (timeStr.isEmpty) return '';
    try {
      final time = DateTime.parse(timeStr);
      final now = DateTime.now();
      final diff = now.difference(time);
      if (diff.inHours < 1) return '刚刚';
      if (diff.inDays == 0) return '${diff.inHours}小时前';
      if (diff.inDays < 7) return '${diff.inDays}天前';
      return '${time.month}/${time.day}';
    } catch (_) {
      return timeStr;
    }
  }
}
