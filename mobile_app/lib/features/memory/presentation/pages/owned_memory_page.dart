import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

Widget? ownedMemoryGate(WidgetRef ref, {String kind = 'memory', String? characterId}) {
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
  if (ownership.value == true) return OwnedMemoryPage(initialKind: kind, characterId: characterId);
  return null;
}

class OwnedMemoryPage extends ConsumerStatefulWidget {
  const OwnedMemoryPage({super.key, this.initialKind = 'memory', this.characterId});
  final String initialKind;
  final String? characterId;

  @override
  ConsumerState<OwnedMemoryPage> createState() => _OwnedMemoryPageState();
}

class _OwnedMemoryPageState extends ConsumerState<OwnedMemoryPage> {
  static const _layers = {
    'memory': '原始记忆',
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
      final result = await owned.data(
        _kind,
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
      final cursor = (snapshot['nextCursors']?[_kind] ?? '').toString();
      final legacyKind = {
        'memory': 'legacyMemory',
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
          final indexScope = Map<String, dynamic>.from(value['executionScope'] as Map)
            ..remove('requestId')
            ..remove('turnId')
            ..remove('executionId');
          if (jsonEncode(indexScope) != stamp) throw StateError('索引数据归属已变化，请刷新后重试');
          if (mounted && ticket == _generation) setState(() => _projection = value);
        } catch (cause) {
          if (mounted && ticket == _generation) setState(() => _projectionNotice = cause.toString());
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
      final value = await ref.read(chatServiceProvider).owned.projections(_role, expectedScope: Map<String, dynamic>.from(_projection!['executionScope'] as Map));
      if (mounted && ticket == _generation) {
        setState(() {
          _projection = value;
          _projectionNotice = '已加入所有者的重建队列，处理完成后刷新查看状态。';
        });
      }
    } catch (cause) {
      if (mounted && ticket == _generation) setState(() => _projectionNotice = cause.toString());
    } finally {
      if (mounted) setState(() => _rebuilding = false);
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
      appBar: AmitiaAppBar(title: '记忆与数据归属', showBackButton: true),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
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
                  if (owned.policy?['coordinated'] == true)
                    DropdownButtonFormField<String>(
                      initialValue: _source,
                      decoration: const InputDecoration(labelText: '记忆数据来源'),
                      items: [
                        const DropdownMenuItem(
                          value: 'current',
                          child: Text('当前 Core 的数据'),
                        ),
                        ..._historicalRoles.map(
                          (row) => DropdownMenuItem(
                            value: 'history:${row['id']}',
                            child: Text('原设备历史 · ${row['name'] ?? row['id']}'),
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
                    initialValue: owned.roles.any((row) => row['id'] == _role)
                        ? _role
                        : null,
                    decoration: const InputDecoration(labelText: '记忆所属角色'),
                    items: owned.roles
                        .map(
                          (row) => DropdownMenuItem(
                            value: row['id'].toString(),
                            child: Text((row['name'] ?? row['id']).toString()),
                          ),
                        )
                        .toList(),
                    onChanged: _loading
                        ? null
                        : (value) {
                            _role = value ?? '';
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
                  if (_source == 'current' && const ['vector', 'graph'].contains(_kind)) ...[
                    for (final layer in (_projection?['status']?['layers'] as List? ?? []).whereType<Map>())
                      Text('${layer['kind'] == 'vector' ? '向量' : '图谱'}：已处理 ${layer['current']}/${layer['total']} · 待更新 ${layer['pending']} · 重试 ${layer['retrying']}'),
                    if (_projectionNotice.isNotEmpty) Text(_projectionNotice),
                    TextButton(onPressed: _projection == null || _loading || _rebuilding ? null : _rebuild, child: Text(_rebuilding ? '正在提交' : '重建当前角色索引')),
                  ],
                ],
              ),
            ),
            if (_loading) const LinearProgressIndicator(),
            Expanded(
              child: ListView(
                children: [
                  for (final row in _rows.values)
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
                            SelectableText(_content(row)),
                            Text(
                              '版本 ${row['revision']}${row['body']?['allowContextUse'] == false ? ' · 已停用' : ''}',
                            ),
                            if (_kind == 'memory' && _source == 'current' && (row['revision'] as num? ?? 0) > 0)
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
