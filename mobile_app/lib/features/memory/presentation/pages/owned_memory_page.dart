import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/device_owned_memory_service.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import 'owned_memory_timeline.dart';

Widget? ownedMemoryGate(
  WidgetRef ref, {
  String kind = 'memory',
  String? characterId,
  bool timeline = false,
}) {
  final ownership = ref.watch(ownedMemoryModeProvider);
  if (ownership.isLoading) {
    return const Scaffold(body: Center(child: CircularProgressIndicator()));
  }
  if (ownership.hasError) {
    return Scaffold(
      appBar: AppBar(title: const Text('记忆与数据归属')),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(ownership.error.toString()),
            TextButton(
              onPressed: () => ref.invalidate(ownedMemoryModeProvider),
              child: const Text('重试'),
            ),
          ],
        ),
      ),
    );
  }
  if (ownership.value == true)
    return OwnedMemoryPage(
      initialKind: kind,
      characterId: characterId,
      timeline: timeline,
    );
  return null;
}

class OwnedMemoryPage extends ConsumerStatefulWidget {
  const OwnedMemoryPage({
    super.key,
    this.initialKind = 'memory',
    this.characterId,
    this.timeline = false,
  });
  final String initialKind;
  final String? characterId;
  final bool timeline;

  @override
  ConsumerState<OwnedMemoryPage> createState() => _OwnedMemoryPageState();
}

class _OwnedMemoryPageState extends ConsumerState<OwnedMemoryPage> {
  static const _layers = {
    'memory': '原始记忆',
    'candidate': '候选记忆',
    'legacy': '已有记忆（旧格式）',
    'working': '工作记忆',
    'profile': '用户画像',
    'episodic': '情节记忆',
    'fact': '事实记忆',
    'vector': '向量索引',
    'graph': '关系图谱',
    'summary': '对话摘要',
  };
  String _role = '';
  String _kind = 'memory';
  String _owner = '';
  String _next = '';
  String _nextLegacy = '';
  String _source = 'current';
  List<Map<String, dynamic>> _historicalRoles = [];
  String _scope = '';
  String _error = '';
  String _query = '';
  String _searchMode = 'keyword';
  String _memoryType = '';
  String _memorySource = '';
  String _memorySort = '';
  Map<String, dynamic>? _managementPage;
  bool _managing = false;
  ({
    Map<String, dynamic> original,
    Map<String, dynamic> input,
    bool candidates,
    String id,
    int generation,
  })?
  _pendingMemory;
  Map<String, dynamic>? _projection;
  String _projectionNotice = '';
  bool _rebuilding = false;
  bool _loading = false;
  int _generation = 0;
  final _rows = <String, Map<String, dynamic>>{};

  @override
  void initState() {
    super.initState();
    _kind = widget.initialKind;
    final owned = ref.read(chatServiceProvider).owned;
    try {
      _role = owned.selectRole(widget.characterId);
    } catch (_) {}
    if (_role.isNotEmpty) _load();
  }

  Future<void> _load({bool older = false}) async {
    final ticket = ++_generation;
    setState(() {
      _loading = true;
      _error = '';
      if (!older) {
        _managementPage = null;
        _projection = null;
        _projectionNotice = '';
        _rows.clear();
        _next = '';
        _nextLegacy = '';
        _scope = '';
      }
    });
    try {
      final owned = ref.read(chatServiceProvider).owned;
      if (!await owned.refresh()) throw StateError('记忆服务尚未就绪');
      if (owned.policy?['coordinated'] == true && _historicalRoles.isEmpty) {
        _historicalRoles = await owned.historicalRoles(_role);
      }
      final historical = _source.startsWith('history:');
      if (!historical && const ['memory', 'candidate'].contains(_kind)) {
        final page = await ref
            .read(deviceOwnedMemoryServiceProvider)
            .query(
              _role,
              candidates: _kind == 'candidate',
              query: _query,
              mode: _searchMode,
              memoryType: _memoryType,
              source: _memorySource,
              sort: _memorySort,
              cursor: older ? _next : '',
              original: older ? _managementPage : null,
            );
        if (!mounted || ticket != _generation) return;
        setState(() {
          _managementPage = page;
          _owner = (page['executionScope'] as Map)['resourceOwnerId']
              .toString();
          _scope = DeviceOwnedMemoryService.scopeStamp(
            page['executionScope'] as Map,
          );
          _next = (page['nextCursor'] ?? '').toString();
          for (final row in page['resources'] as List<Map<String, dynamic>>)
            _rows[row['id'].toString()] = row;
        });
        return;
      }
      final result = await owned.data(
        _kind == 'legacy' ? 'memory' : _kind,
        characterId: _role,
        cursor: !historical && older ? _next : '',
        legacyCursor: !historical && older ? _nextLegacy : '',
        historicalRoleId: historical ? _source.substring(8) : '',
        historicalCursor: historical && older ? _next : '',
        historicalLegacyCursor: historical && older ? _nextLegacy : '',
      );
      if (!mounted || ticket != _generation) return;
      final scope = Map<String, dynamic>.from(result['executionScope'] as Map)
        ..remove('requestId')
        ..remove('turnId')
        ..remove('executionId');
      final stamp = jsonEncode(scope);
      if (older && _scope != stamp) throw StateError('数据来源已变化，请刷新后重试');
      final rawSnapshot = historical
          ? result['historicalSnapshot']
          : result['snapshot'];
      if (rawSnapshot is! Map) throw StateError('原设备历史数据暂不可用');
      final snapshot = rawSnapshot;
      final cursor = _kind == 'legacy'
          ? ''
          : (snapshot['nextCursors']?[_kind] ?? '').toString();
      final legacyKind = {
        'memory': 'legacyMemory',
        'legacy': 'legacyMemory',
        'profile': 'legacyProfile',
        'episodic': 'legacyEpisode',
      }[_kind];
      final legacyCursor = legacyKind == null
          ? ''
          : (snapshot['nextCursors']?[legacyKind] ?? '').toString();
      if (older && cursor.isNotEmpty && cursor == _next) {
        throw StateError('记忆分页游标重复');
      }
      if (older && legacyCursor.isNotEmpty && legacyCursor == _nextLegacy) {
        throw StateError('记忆分页游标重复');
      }
      setState(() {
        _scope = stamp;
        _owner = snapshot['ownerId'].toString();
        _next = cursor;
        _nextLegacy = legacyCursor;
        for (final row
            in (snapshot['resources'] as List? ?? []).whereType<Map>()) {
          if (row['kind'] == _kind) {
            _rows[row['id'].toString()] = Map<String, dynamic>.from(row)
              ..['executionScope'] = result['executionScope'];
          }
        }
        final legacyField = {
          'memory': 'legacyMemories',
          'legacy': 'legacyMemories',
          'profile': 'legacyProfiles',
          'episodic': 'legacyEpisodes',
        }[_kind];
        for (final row
            in (legacyField == null
                    ? <dynamic>[]
                    : snapshot[legacyField] as List? ?? [])
                .whereType<Map>()) {
          final id = 'legacy/${row['id']}';
          _rows[id] = {
            'id': id,
            'kind': _kind,
            'ownerId': _owner,
            'revision': 0,
            'body': {
              'key': row['key'] ?? row['title'] ?? row['fieldName'],
              'content': row,
              'allowContextUse': row['allowContextUse'],
              'expiresAt': row['expiresAt'],
            },
          };
        }
      });
      if (_source == 'current' && const ['vector', 'graph'].contains(_kind)) {
        try {
          final value = await owned.projections(_role);
          final indexScope =
              Map<String, dynamic>.from(value['executionScope'] as Map)
                ..remove('requestId')
                ..remove('turnId')
                ..remove('executionId');
          if (jsonEncode(indexScope) != stamp)
            throw StateError('索引数据归属已变化，请刷新后重试');
          if (mounted && ticket == _generation)
            setState(() => _projection = value);
        } catch (cause) {
          if (mounted && ticket == _generation)
            setState(() => _projectionNotice = cause.toString());
        }
      }
    } catch (cause) {
      if (mounted && ticket == _generation) {
        setState(() => _error = cause.toString());
      }
    } finally {
      if (mounted && ticket == _generation) setState(() => _loading = false);
    }
  }

  String _content(Map row) {
    final body = row['body'] as Map? ?? {};
    final content = body['content'];
    if (_kind == 'candidate') return (body['value'] ?? '').toString();
    if (row['kind'] == 'vector' && content is Map) {
      return '${(content['values'] as List? ?? []).length} 维 · 模型 ${content['modelFingerprint'] ?? '待重建'}';
    }
    if (content is String) return content;
    if (content is Map) {
      return (content['value'] ??
              content['text'] ??
              content['summary'] ??
              jsonEncode(content))
          .toString();
    }
    return jsonEncode(body);
  }

  Future<void> _rebuild() async {
    if (_projection == null || _rebuilding) return;
    final ticket = _generation;
    setState(() => _rebuilding = true);
    try {
      final value = await ref
          .read(chatServiceProvider)
          .owned
          .projections(
            _role,
            expectedScope: Map<String, dynamic>.from(
              _projection!['executionScope'] as Map,
            ),
          );
      if (mounted && ticket == _generation) {
        setState(() {
          _projection = value;
          _projectionNotice = '已加入所有者的重建队列，处理完成后刷新查看状态。';
        });
      }
    } catch (cause) {
      if (mounted && ticket == _generation)
        setState(() => _projectionNotice = cause.toString());
    } finally {
      if (mounted) setState(() => _rebuilding = false);
    }
  }

  Future<Map<String, dynamic>?> _memoryForm({Map<String, dynamic>? row}) async {
    final body = row?['body'] as Map? ?? {};
    final key = TextEditingController(text: (body['key'] ?? '').toString());
    final value = TextEditingController(text: (body['value'] ?? '').toString());
    final type = TextEditingController(
      text: (body['memoryType'] ?? 'fact').toString(),
    );
    final importance = TextEditingController(
      text: (body['importance'] ?? 5).toString(),
    );
    final result = await showDialog<Map<String, dynamic>>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(row == null ? '新增记忆' : '编辑候选记忆'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: key,
                decoration: const InputDecoration(labelText: '记忆键'),
              ),
              TextField(
                controller: value,
                minLines: 3,
                maxLines: 8,
                decoration: const InputDecoration(labelText: '内容'),
              ),
              TextField(
                controller: type,
                decoration: const InputDecoration(labelText: '类型'),
              ),
              TextField(
                controller: importance,
                keyboardType: TextInputType.number,
                decoration: const InputDecoration(labelText: '重要程度（0—10）'),
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () {
              final number = int.tryParse(importance.text);
              if (key.text.trim().isEmpty ||
                  value.text.trim().isEmpty ||
                  number == null ||
                  number < 0 ||
                  number > 10)
                return;
              Navigator.pop(context, {
                'key': key.text.trim(),
                'value': value.text.trim(),
                'memoryType': type.text.trim(),
                'importance': number,
              });
            },
            child: const Text('保存'),
          ),
        ],
      ),
    );
    key.dispose();
    value.dispose();
    type.dispose();
    importance.dispose();
    return result;
  }

  Future<void> _manage(
    Map<String, dynamic> original,
    Map<String, dynamic> input, {
    bool candidates = false,
    String? operationId,
  }) async {
    if (_managing) return;
    final ticket = _generation;
    final service = ref.read(deviceOwnedMemoryServiceProvider);
    final id = operationId ?? DeviceOwnedMemoryService.requestId();
    _pendingMemory = (
      original: original,
      input: Map<String, dynamic>.unmodifiable(input),
      candidates: candidates,
      id: id,
      generation: ticket,
    );
    setState(() {
      _managing = true;
      _error = '';
    });
    try {
      await service.manage(
        original,
        input,
        candidates: candidates,
        operationId: id,
      );
      _pendingMemory = null;
      if (mounted && ticket == _generation) await _load();
    } on OwnedMemoryConflict catch (conflict) {
      _pendingMemory = null;
      if (!mounted || ticket != _generation || conflict.resources.length != 1)
        return;
      final row = conflict.resources.single;
      final resolution = await showDialog<String>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('同键记忆冲突'),
          content: Text(
            '原记录版本 ${row['revision']}\n${_content(row)}\n请选择本次处理方式',
          ),
          actions: [
            for (final option in const {
              'replace': '替换',
              'merge': '合并',
              'keep_both': '同时保留',
            }.entries)
              TextButton(
                onPressed: () => Navigator.pop(context, option.key),
                child: Text(option.value),
              ),
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('取消'),
            ),
          ],
        ),
      );
      if (resolution == null || !mounted || ticket != _generation) return;
      try {
        final resolved = {
          ...input,
          'action': candidates ? input['action'] : 'resolve',
          'conflictId': row['id'],
          'expectedConflictRevision': row['revision'],
          'resolution': resolution,
        };
        final resolvedId = DeviceOwnedMemoryService.requestId();
        _pendingMemory = (
          original: original,
          input: Map<String, dynamic>.unmodifiable(resolved),
          candidates: candidates,
          id: resolvedId,
          generation: ticket,
        );
        await service.manage(
          original,
          resolved,
          candidates: candidates,
          operationId: resolvedId,
        );
        _pendingMemory = null;
        if (mounted && ticket == _generation) await _load();
      } catch (cause) {
        if (mounted && ticket == _generation)
          setState(() => _error = cause.toString());
      }
    } catch (cause) {
      if (mounted && ticket == _generation)
        setState(() => _error = cause.toString());
    } finally {
      if (mounted) setState(() => _managing = false);
    }
  }

  Future<void> _createMemory() async {
    final page = _managementPage;
    final ticket = _generation;
    if (page == null) return;
    final input = await _memoryForm();
    if (input == null || !mounted || ticket != _generation) return;
    await _manage(page, {...input, 'action': 'create'});
  }

  Future<void> _candidate(Map<String, dynamic> row, String action) async {
    final ticket = _generation;
    Map<String, dynamic> changes = {};
    if (action == 'update') {
      final input = await _memoryForm(row: row);
      if (input == null || !mounted || ticket != _generation) return;
      changes = input;
    }
    await _manage(row, {
      ...changes,
      'action': action,
      'id': row['id'],
      'expectedRevision': row['revision'],
    }, candidates: true);
  }

  Future<void> _generateCandidates() async {
    final page = _managementPage;
    final ticket = _generation;
    final role = _role;
    if (page == null) return;
    try {
      final rows = await ref
          .read(chatServiceProvider)
          .owned
          .conversations(characterId: role);
      if (!mounted || ticket != _generation) return;
      final selected = await showDialog<int>(
        context: context,
        builder: (context) => SimpleDialog(
          title: const Text('从已保存对话生成候选记忆'),
          children: [
            for (var index = 0; index < rows.length; index++)
              SimpleDialogOption(
                onPressed: () => Navigator.pop(context, index),
                child: Text(
                  '${rows[index].title}\n${rows[index].sourceOwnerId} · ${rows[index].messageCount} 条消息',
                ),
              ),
          ],
        ),
      );
      if (selected == null || !mounted || ticket != _generation) return;
      final row = rows[selected];
      await _manage(page, {
        'action': 'generate',
        'conversationId': row.sourceResourceId,
        if (row.conversationOrigin != null)
          'conversationOrigin': row.conversationOrigin,
        if (row.sourceOwnerId !=
            (page['executionScope'] as Map)['resourceOwnerId'])
          'historicalRoleId': row.characterId,
      }, candidates: true);
    } catch (cause) {
      if (mounted && ticket == _generation)
        setState(() => _error = cause.toString());
    }
  }

  Future<void> _edit(Map row, {bool deleted = false}) async {
    if (deleted) {
      final accepted = await showDialog<bool>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('删除记忆'),
          content: const Text('删除原始记忆会同时停用其事实、向量和图谱数据。'),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('取消'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(context, true),
              child: const Text('删除'),
            ),
          ],
        ),
      );
      if (accepted != true || !mounted) return;
    }
    try {
      await ref
          .read(chatServiceProvider)
          .owned
          .edit(
            'memory',
            row['id'].toString(),
            characterId: _role,
            expectedScope: Map<String, dynamic>.from(
              row['executionScope'] as Map,
            ),
            expectedOwnerId: row['ownerId'].toString(),
            expectedRevision: (row['revision'] as num).toInt(),
            deleted: deleted,
            changes: deleted
                ? {}
                : {'allowContextUse': row['body']?['allowContextUse'] == false},
          );
      if (mounted) await _load();
    } catch (cause) {
      if (mounted) setState(() => _error = cause.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    final owned = ref.watch(chatServiceProvider).owned;
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: widget.timeline ? '记忆时间线' : '记忆与数据归属',
        showBackButton: true,
      ),
      body: SafeArea(
        child: Column(
          children: [
            Flexible(
              flex: 2,
              fit: FlexFit.loose,
              child: SingleChildScrollView(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        owned.policy?['coordinated'] == true
                            ? '新记忆由当前 Core 保存和管理'
                            : '新记忆由设备保存和管理，Core 负责计算',
                      ),
                      Text('服务：${owned.coreId}\n数据所有者：$_owner'),
                      if (owned.policy?['coordinated'] == true &&
                          _kind != 'candidate')
                        DropdownButtonFormField<String>(
                          initialValue: _source,
                          decoration: const InputDecoration(
                            labelText: '记忆数据来源',
                          ),
                          items: [
                            const DropdownMenuItem(
                              value: 'current',
                              child: Text('当前 Core 的数据'),
                            ),
                            ..._historicalRoles.map(
                              (row) => DropdownMenuItem(
                                value: 'history:${row['id']}',
                                child: Text(
                                  '原设备历史 · ${row['name'] ?? row['id']}',
                                ),
                              ),
                            ),
                          ],
                          onChanged: _loading
                              ? null
                              : (value) {
                                  _source = value ?? 'current';
                                  if (_role.isNotEmpty) _load();
                                },
                        ),
                      if (_source != 'current')
                        const Text('原设备历史数据可供 Core 调用；请在原设备管理这些历史记录。'),
                      DropdownButtonFormField<String>(
                        initialValue:
                            owned.roles.any((row) => row['id'] == _role)
                            ? _role
                            : null,
                        decoration: const InputDecoration(labelText: '记忆所属角色'),
                        items: owned.roles
                            .map(
                              (row) => DropdownMenuItem(
                                value: row['id'].toString(),
                                child: Text(
                                  (row['name'] ?? row['id']).toString(),
                                ),
                              ),
                            )
                            .toList(),
                        onChanged: _loading
                            ? null
                            : (value) {
                                _role = value ?? '';
                                _historicalRoles = [];
                                _source = 'current';
                                if (_role.isNotEmpty) _load();
                              },
                      ),
                      DropdownButtonFormField<String>(
                        initialValue: _kind,
                        decoration: const InputDecoration(labelText: '记忆层'),
                        items: _layers.entries
                            .map(
                              (row) => DropdownMenuItem(
                                value: row.key,
                                child: Text(row.value),
                              ),
                            )
                            .toList(),
                        onChanged: _loading
                            ? null
                            : (value) {
                                _kind = value ?? 'memory';
                                if (_kind == 'candidate') _source = 'current';
                                if (_role.isNotEmpty) _load();
                              },
                      ),
                      if (_error.isNotEmpty)
                        Text(
                          _error,
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                          ),
                        ),
                      if (_source == 'current' &&
                          const ['memory', 'candidate'].contains(_kind)) ...[
                        TextFormField(
                          initialValue: _query,
                          decoration: const InputDecoration(labelText: '搜索记忆'),
                          onFieldSubmitted: (value) {
                            _query = value.trim();
                            _load();
                          },
                        ),
                        if (_kind == 'memory') ...[
                          TextFormField(
                            initialValue: _memoryType,
                            decoration: const InputDecoration(
                              labelText: '按类型过滤（留空为全部）',
                            ),
                            onFieldSubmitted: (value) {
                              _memoryType = value.trim();
                              _load();
                            },
                          ),
                          TextFormField(
                            initialValue: _memorySource,
                            decoration: const InputDecoration(
                              labelText: '按来源过滤（留空为全部）',
                            ),
                            onFieldSubmitted: (value) {
                              _memorySource = value.trim();
                              _load();
                            },
                          ),
                          DropdownButtonFormField<String>(
                            initialValue: _memorySort,
                            decoration: const InputDecoration(labelText: '排序'),
                            items: const [
                              DropdownMenuItem(value: '', child: Text('默认顺序')),
                              DropdownMenuItem(
                                value: 'importance_desc',
                                child: Text('重要程度优先'),
                              ),
                            ],
                            onChanged: _loading || _managing
                                ? null
                                : (value) {
                                    _memorySort = value ?? '';
                                    _load();
                                  },
                          ),
                        ],
                        if (_kind == 'memory')
                          DropdownButtonFormField<String>(
                            initialValue: _searchMode,
                            decoration: const InputDecoration(
                              labelText: '检索方式',
                            ),
                            items: const [
                              DropdownMenuItem(
                                value: 'keyword',
                                child: Text('关键词'),
                              ),
                              DropdownMenuItem(
                                value: 'hybrid',
                                child: Text('混合检索'),
                              ),
                              DropdownMenuItem(
                                value: 'vector',
                                child: Text('语义检索'),
                              ),
                            ],
                            onChanged: _loading || _managing
                                ? null
                                : (value) {
                                    _searchMode = value ?? 'keyword';
                                    _load();
                                  },
                          ),
                        TextButton(
                          onPressed:
                              _managementPage == null || _loading || _managing
                              ? null
                              : _kind == 'candidate'
                              ? _generateCandidates
                              : _createMemory,
                          child: Text(
                            _kind == 'candidate' ? '从已保存对话生成候选' : '新增记忆',
                          ),
                        ),
                        if (_managing) const Text('等待数据所有者确认保存'),
                        if (_pendingMemory != null &&
                            _pendingMemory!.generation == _generation &&
                            !_managing)
                          TextButton(
                            onPressed: () {
                              final pending = _pendingMemory!;
                              _manage(
                                pending.original,
                                pending.input,
                                candidates: pending.candidates,
                                operationId: pending.id,
                              );
                            },
                            child: const Text('使用原请求重试保存'),
                          ),
                      ],
                      if (_source == 'current' &&
                          const ['vector', 'graph'].contains(_kind)) ...[
                        for (final layer
                            in (_projection?['status']?['layers'] as List? ??
                                    [])
                                .whereType<Map>())
                          Text(
                            '${layer['kind'] == 'vector' ? '向量' : '图谱'}：已处理 ${layer['current']}/${layer['total']} · 待更新 ${layer['pending']} · 重试 ${layer['retrying']}',
                          ),
                        if (_projectionNotice.isNotEmpty)
                          Text(_projectionNotice),
                        TextButton(
                          onPressed:
                              _projection == null || _loading || _rebuilding
                              ? null
                              : _rebuild,
                          child: Text(_rebuilding ? '正在提交' : '重建当前角色索引'),
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            ),
            if (_loading) const LinearProgressIndicator(),
            Expanded(
              child: ListView(
                children: [
                  if (widget.timeline)
                    const Padding(
                      padding: EdgeInsets.all(16),
                      child: Text(
                        '按当前已加载资源的真实保存时间排列；可继续加载更多数据。此视图展示记忆快照，不代表完整修改历史。',
                      ),
                    ),
                  for (final row
                      in widget.timeline
                          ? ownedMemoryTimelineRows(_rows.values)
                          : _rows.values)
                    Card(
                      child: Padding(
                        padding: const EdgeInsets.all(16),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              (row['body']?['key'] ?? row['id']).toString(),
                              style: Theme.of(context).textTheme.titleSmall,
                            ),
                            const SizedBox(height: 8),
                            if (widget.timeline)
                              Text(
                                ownedMemoryResourceTime(
                                      row,
                                    )?.toLocal().toString() ??
                                    '该记录未保存时间',
                              ),
                            SelectableText(_content(row)),
                            Text(
                              '版本 ${row['revision']}${row['body']?['allowContextUse'] == false ? ' · 已停用' : ''}',
                            ),
                            if (_kind == 'candidate' && _source == 'current')
                              Wrap(
                                children: [
                                  TextButton(
                                    onPressed: _loading || _managing
                                        ? null
                                        : () => _candidate(row, 'update'),
                                    child: const Text('编辑'),
                                  ),
                                  TextButton(
                                    onPressed: _loading || _managing
                                        ? null
                                        : () => _candidate(row, 'accept'),
                                    child: const Text('接受并保存'),
                                  ),
                                  TextButton(
                                    onPressed: _loading || _managing
                                        ? null
                                        : () => _candidate(row, 'reject'),
                                    child: const Text('拒绝'),
                                  ),
                                ],
                              ),
                            if (_kind == 'memory' &&
                                _source == 'current' &&
                                (row['revision'] as num? ?? 0) > 0)
                              Wrap(
                                children: [
                                  TextButton(
                                    onPressed: _loading
                                        ? null
                                        : () => _edit(row),
                                    child: Text(
                                      row['body']?['allowContextUse'] == false
                                          ? '允许用于对话'
                                          : '停用',
                                    ),
                                  ),
                                  TextButton(
                                    onPressed: _loading
                                        ? null
                                        : () => _edit(row, deleted: true),
                                    child: const Text('删除'),
                                  ),
                                ],
                              ),
                          ],
                        ),
                      ),
                    ),
                  if (!_loading && _rows.isEmpty)
                    const Padding(
                      padding: EdgeInsets.all(24),
                      child: Text('当前角色在此数据来源中暂无记录'),
                    ),
                ],
              ),
            ),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                TextButton(
                  onPressed: _loading || _role.isEmpty ? null : () => _load(),
                  child: const Text('刷新'),
                ),
                if (_next.isNotEmpty || _nextLegacy.isNotEmpty)
                  TextButton(
                    onPressed: _loading ? null : () => _load(older: true),
                    child: const Text('加载更多'),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
