import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/device_owned_memory_service.dart';
import '../../../../core/services/owned_memory_scope.dart';
import '../../../../core/services/owned_memory_view_options.dart';
import '../../../../core/services/memory_scope_transfer_service.dart';
import '../widgets/memory_scope_transfer_dialog.dart';
import '../widgets/owned_memory_graph.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import 'owned_memory_timeline.dart';
import '../../../../app/theme/app_colors.dart';
import '../historical_memory_style.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../owned_memory_format.dart';
import '../widgets/historical_memory_card.dart';

Widget? ownedMemoryGate(
  WidgetRef ref, {
  String kind = 'memory',
  String? characterId,
  bool timeline = false,
  bool showTools = false,
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
      showTools: showTools,
    );
  return null;
}

class OwnedMemoryPage extends ConsumerStatefulWidget {
  const OwnedMemoryPage({
    super.key,
    this.initialKind = 'memory',
    this.characterId,
    this.timeline = false,
    this.showTools = false,
  });
  final String initialKind;
  final String? characterId;
  final bool timeline;
  final bool showTools;

  @override
  ConsumerState<OwnedMemoryPage> createState() => _OwnedMemoryPageState();
}

class _OwnedMemoryPageState extends ConsumerState<OwnedMemoryPage> {
  static const _layers = {
    'memory': '记忆总览',
    'candidate': '候选记忆',
    'legacy': '已有记忆（旧格式）',
    'working': '工作记忆',
    'profile': '用户画像',
    'episodic': '情景记忆',
    'fact': '事实记忆',
    'vector': '向量索引',
    'graph': '关系图谱',
    'summary': '对话摘要',
  };
  String _role = '';
  String _memoryView = 'all';
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
    if (_kind == 'profile') _memoryView = 'global';
    final owned = ref.read(memoryDataSourceProvider);
    try {
      _role = owned.selectRole(widget.characterId);
    } catch (_) {}
    if (_role.isEmpty && owned.roles.isNotEmpty)
      _role = owned.roles.first['id'].toString();
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
      final owned = ref.read(memoryDataSourceProvider);
      if (!await owned.refresh()) throw StateError('记忆服务尚未就绪');
      if (!owned.roles.any((role) => role['id'] == _role)) {
        if (_memoryView.startsWith('role:')) _memoryView = 'all';
        _role = owned.roles.isEmpty ? '' : owned.roles.first['id'].toString();
        if (_role.isEmpty) throw StateError('请先创建可用角色后管理记忆');
      }
      if (owned.policy?['coordinated'] == true && _historicalRoles.isEmpty) {
        _historicalRoles = await owned.historicalRoles(_role);
      }
      final historical = _source.startsWith('history:');
      if (!historical && const ['memory', 'candidate'].contains(_kind)) {
        final page = older && _next.isEmpty && _managementPage != null
            ? {..._managementPage!, 'resources': <Map<String, dynamic>>[]}
            : await ref
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
                    memoryView: _memoryView,
                  );
        final legacy = _kind == 'memory' && (!older || _nextLegacy.isNotEmpty)
            ? await owned.data(
                'memory',
                characterId: _role,
                cursor: '',
                legacyCursor: older ? _nextLegacy : '',
                memoryView: _memoryView,
              )
            : null;
        if (legacy != null &&
            DeviceOwnedMemoryService.scopeStamp(
                  legacy['executionScope'] as Map,
                ) !=
                DeviceOwnedMemoryService.scopeStamp(
                  page['executionScope'] as Map,
                ))
          throw StateError('记忆分区在加载期间发生变化，请重新加载');
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
          if (legacy != null) {
            _nextLegacy =
                (legacy['snapshot']['nextCursors']?['legacyMemory'] ?? '')
                    .toString();
            _appendLegacyRows(
              legacy['snapshot'] as Map,
              legacy['executionScope'] as Map,
            );
          }
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
        memoryView: historical
            ? ''
            : _kind == 'profile'
            ? 'global'
            : _memoryView,
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
        'graph': 'legacyGraph',
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
        if (_kind == 'graph') {
          for (final row
              in (snapshot['legacyGraphs'] as List? ?? []).whereType<Map>()) {
            if (row['ownerId'] != snapshot['ownerId'] ||
                row['roleId'] == null ||
                (result['executionScope']['memoryViewAll'] != true &&
                    row['roleId'] !=
                        memoryStorageRole(
                          result['executionScope'] as Map,
                          'graph',
                        )))
              throw StateError('历史图谱的分区或数据所有者已变化');
            _rows[row['id'].toString()] = Map<String, dynamic>.from(row)
              ..['executionScope'] = memoryRecordScope(
                result['executionScope'] as Map,
                row['roleId'].toString(),
              );
          }
        }
        for (final row
            in (snapshot['resources'] as List? ?? []).whereType<Map>()) {
          if (row['kind'] == _kind) {
            _rows[row['id'].toString()] = Map<String, dynamic>.from(row)
              ..['executionScope'] = memoryRecordScope(
                result['executionScope'] as Map,
                row['roleId'].toString(),
              );
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
          final partition = _kind == 'memory' || _kind == 'legacy'
              ? const ['user', 'user_global'].contains(row['scope'])
                    ? globalMemoryRole
                    : row['characterId'].toString()
              : _kind == 'profile' ||
                    (row['characterId'] ?? '').toString().isEmpty
              ? globalMemoryRole
              : row['characterId'].toString();
          _rows[id] = {
            'id': id,
            'kind': _kind,
            'ownerId': _owner,
            'revision': 0,
            'roleId': partition,
            'executionScope': memoryRecordScope(
              result['executionScope'] as Map,
              partition,
            ),
            'body': {
              'key': row['key'] ?? row['title'] ?? row['fieldName'],
              'content': row,
              'allowContextUse': row['allowContextUse'],
              'expiresAt': row['expiresAt'],
            },
          };
        }
      });
      if (_source == 'current' &&
          _memoryView != 'all' &&
          const ['vector', 'graph'].contains(_kind)) {
        try {
          final value = await owned.projections(_role, memoryView: _memoryView);
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

  void _appendLegacyRows(Map snapshot, Map scope) {
    final field = {
      'memory': 'legacyMemories',
      'profile': 'legacyProfiles',
      'episodic': 'legacyEpisodes',
    }[_kind];
    for (final row in (snapshot[field] as List? ?? []).whereType<Map>()) {
      final partition = _kind == 'memory'
          ? const ['user', 'user_global'].contains(row['scope'])
                ? globalMemoryRole
                : row['characterId'].toString()
          : _kind == 'profile' || (row['characterId'] ?? '').toString().isEmpty
          ? globalMemoryRole
          : row['characterId'].toString();
      final id = 'legacy/${row['id']}';
      _rows[id] = {
        'id': id,
        'kind': _kind,
        'ownerId': snapshot['ownerId'],
        'revision': 0,
        'roleId': partition,
        'executionScope': memoryRecordScope(scope, partition),
        'body': {
          'key': row['key'] ?? row['title'] ?? row['attributeName'],
          'content': row,
          'allowContextUse': row['allowContextUse'],
          'expiresAt': row['expiresAt'],
        },
      };
    }
  }

  Future<void> _transferScope(Map<String, dynamic> row) async {
    final original = Map<String, dynamic>.from(row);
    final roles = ref.read(memoryDataSourceProvider).roles;
    final target = await showMemoryScopeTransferDialog(
      context,
      roles,
      row['roleId'].toString(),
    );
    if (target == null || !mounted) return;
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) return;
    setState(() => _managing = true);
    try {
      await transferMemoryScope(api, original, target.scope, target.role);
      if (mounted) await _load();
    } catch (cause) {
      if (mounted) setState(() => _error = cause.toString());
    } finally {
      if (mounted) setState(() => _managing = false);
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
      if (_kind == 'graph')
        return '${(content['nodes'] as List? ?? []).length} 个节点 · ${(content['edges'] as List? ?? []).length} 条关系';
      if (_kind == 'profile')
        return '${content['attributeName'] ?? content['fieldName'] ?? body['key'] ?? ''}：${content['attributeValue'] ?? content['fieldValue'] ?? content['value'] ?? ''}';
      return (content['value'] ??
              content['text'] ??
              content['summary'] ??
              jsonEncode(content))
          .toString();
    }
    return jsonEncode(body);
  }

  Future<void> _openCreateProfile() async {
    final keyCtrl = TextEditingController();
    final valueCtrl = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('新增画像'),
        content: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: keyCtrl,
                decoration: const InputDecoration(labelText: '属性名'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: valueCtrl,
                decoration: const InputDecoration(labelText: '属性值'),
                maxLines: 3,
              ),
            ],
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    final key = keyCtrl.text.trim();
    final value = valueCtrl.text.trim();
    if (key.isEmpty || value.isEmpty) return;
    try {
      final service = ref.read(deviceOwnedMemoryServiceProvider);
      final page = await service.query(_role, memoryView: 'global');
      final scope = page['executionScope'];
      if (scope is! Map) throw StateError('记忆服务尚未就绪');
      await service.manage(
        {'executionScope': scope},
        {
          'action': 'create',
          'key': key,
          'value': value,
          'memoryType': 'personal_info',
          'importance': 5,
          'partition': 'profile',
        },
      );
      await _load();
    } catch (e) {
      if (mounted) setState(() => _error = e.toString());
    }
  }

  Future<void> _rebuild() async {
    if (_projection == null || _rebuilding) return;
    final ticket = _generation;
    setState(() => _rebuilding = true);
    try {
      final value = await ref
          .read(memoryDataSourceProvider)
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
    final route = DialogRoute<Map<String, dynamic>>(
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
    final result = await Navigator.of(context).push(route);
    await route.completed;
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
          .read(memoryDataSourceProvider)
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

  Future<void> _edit(
    Map row, {
    bool deleted = false,
    Map<String, dynamic>? changes,
  }) async {
    row = Map<String, dynamic>.from(jsonDecode(jsonEncode(row)) as Map);
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
          .read(memoryDataSourceProvider)
          .edit(
            row['kind'].toString(),
            row['id'].toString(),
            characterId: row['executionScope']['roleId'].toString(),
            expectedScope: Map<String, dynamic>.from(
              row['executionScope'] as Map,
            ),
            expectedOwnerId: row['ownerId'].toString(),
            expectedRevision: (row['revision'] as num).toInt(),
            deleted: deleted,
            changes: deleted
                ? {}
                : changes ??
                      {
                        'allowContextUse':
                            row['body']?['allowContextUse'] == false,
                      },
          );
      if (mounted) await _load();
    } catch (cause) {
      if (mounted) setState(() => _error = cause.toString());
    }
  }

  Future<void> _editContent(Map row) async {
    final original = Map<String, dynamic>.from(
      jsonDecode(jsonEncode(row)) as Map,
    );
    final content = original['body']?['content'];
    final field = content is Map
        ? const [
            'value',
            'text',
            'summary',
            'attributeValue',
            'fieldValue',
            'event',
          ].firstWhere(content.containsKey, orElse: () => 'value')
        : '';
    final value = await _textInput(
      '编辑记忆内容',
      content is Map
          ? (content[field] ?? '').toString()
          : (content ?? '').toString(),
    );
    if (value == null || !mounted) return;
    await _edit(
      original,
      changes: {
        'content': content is Map ? {...content, field: value} : value,
      },
    );
  }

  Future<String?> _textInput(String title, String initial) async {
    final controller = TextEditingController(text: initial);
    final route = DialogRoute<String>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: TextField(controller: controller, minLines: 1, maxLines: 6),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, controller.text),
            child: const Text('保存'),
          ),
        ],
      ),
    );
    final result = await Navigator.of(context).push(route);
    await route.completed;
    controller.dispose();
    return result;
  }

  Future<void> _setExpiry(Map row) async {
    final original = Map<String, dynamic>.from(
      jsonDecode(jsonEncode(row)) as Map,
    );
    final input = await _textInput(
      '到期时间（留空取消，填写日期或 ISO 时间）',
      (row['body']?['expiresAt'] ?? '').toString(),
    );
    if (input == null || !mounted) return;
    final date = input.trim().isEmpty ? null : DateTime.tryParse(input.trim());
    if (input.trim().isNotEmpty && date == null) {
      setState(() => _error = '到期时间格式无效');
      return;
    }
    await _edit(
      original,
      changes: {'expiresAt': date?.toUtc().toIso8601String()},
    );
  }

  bool _searchVisible = false;

  String get _pageTitle => widget.timeline
      ? '记忆时间线'
      : widget.showTools && _kind == 'memory'
      ? '记忆'
      : widget.characterId != null && _kind == 'memory'
      ? '角色记忆'
      : _layers[_kind] ?? '记忆';

  IconData get _pageIcon => switch (_kind) {
    'profile' => Icons.person_outline,
    'episodic' => Icons.psychology_outlined,
    'graph' => Icons.account_tree_outlined,
    'vector' => Icons.hub_outlined,
    'summary' => Icons.subject_outlined,
    'candidate' => Icons.fact_check_outlined,
    _ => widget.timeline ? Icons.timeline : Icons.memory_outlined,
  };

  @override
  Widget build(BuildContext context) {
    final rows = widget.timeline
        ? ownedMemoryTimelineRows(_rows.values)
        : _rows.values.toList();
    final categorized = _kind == 'profile' && _profileCategory != '全部'
        ? rows
              .where((row) => historicalMemoryCategory(row) == _profileCategory)
              .toList()
        : rows;
    final visible = _query.isEmpty
        ? categorized
        : categorized
              .where(
                (row) =>
                    '${ownedMemoryTitle(row)} ${ownedMemoryText(row, kind: _kind)}'
                        .toLowerCase()
                        .contains(_query.toLowerCase()),
              )
              .toList();
    final manager = !widget.showTools && !widget.timeline && _kind == 'memory';
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: _kind == 'graph' ? '记忆图谱' : _pageTitle,
        showBackButton: true,
        fallbackRoute: widget.characterId != null
            ? AppRoutes.characters
            : AppRoutes.memory,
        actions: [
          if (const ['memory', 'candidate'].contains(_kind))
            AmitiaIconButton(
              icon: _searchVisible ? Icons.close : Icons.search,
              tooltip: _searchVisible ? '退出搜索' : '搜索记忆',
              onPressed: () {
                setState(() {
                  _searchVisible = !_searchVisible;
                  if (!_searchVisible && _query.isNotEmpty) {
                    _searchController.clear();
                    _query = '';
                    _load();
                  }
                });
              },
            ),
          if (_kind == 'profile')
            AmitiaIconButton(
              icon: Icons.add,
              tooltip: '新增画像',
              onPressed: _loading || _source != 'current'
                  ? null
                  : _openCreateProfile,
            ),
          AmitiaIconButton(
            icon: Icons.tune,
            tooltip: '筛选与管理',
            onPressed: _showControls,
          ),
          if (manager)
            AmitiaIconButton(
              icon: Icons.account_tree_outlined,
              tooltip: '记忆图谱',
              onPressed: () => context.push(AppRoutes.memoryGraph),
            ),
          if (_kind == 'graph' || widget.timeline || _kind == 'episodic')
            AmitiaIconButton(
              icon: Icons.refresh,
              tooltip: '刷新',
              onPressed: _loading || _role.isEmpty ? null : () => _load(),
            ),
        ],
      ),
      floatingActionButton: manager && _source == 'current'
          ? FloatingActionButton(
              onPressed: _loading || _managing || _managementPage == null
                  ? null
                  : _createMemory,
              backgroundColor: context.accentPrimary,
              child: const Icon(Icons.add, color: Colors.white),
            )
          : null,
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            if (widget.showTools) _buildTools(context),
            if (_searchVisible)
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  AppSpacing.sm,
                  AppSpacing.pagePadding,
                  AppSpacing.xs,
                ),
                child: AmitiaSearchField(
                  key: ValueKey('search/$_kind'),
                  hintText: '搜索记忆',
                  controller: _searchController,
                  onChanged: (value) => setState(() => _query = value.trim()),
                ),
              ),
            if (widget.showTools && _kind == 'memory' || _kind == 'profile')
              _buildCategoryTabs(context),
            if (manager || widget.timeline)
              Padding(
                padding: EdgeInsets.symmetric(
                  horizontal: AppSpacing.pagePadding,
                  vertical: AppSpacing.sm,
                ),
                child: Row(
                  children: [
                    _historicalFilterChip(
                      context,
                      '类型：${_memoryType.isEmpty ? '全部' : _memoryType}',
                    ),
                    SizedBox(width: AppSpacing.sm),
                    _historicalFilterChip(
                      context,
                      _memoryView == 'all' ? '全部记忆' : '查看范围',
                    ),
                  ],
                ),
              ),
            if (_loading && rows.isNotEmpty) const LinearProgressIndicator(),
            SizedBox(height: AppSpacing.sm),
            Expanded(
              child: _loading && rows.isEmpty
                  ? const Center(child: CircularProgressIndicator())
                  : _error.isNotEmpty && rows.isEmpty
                  ? AmitiaErrorState(
                      message: _error,
                      onRetry: _role.isEmpty ? null : () => _load(),
                    )
                  : visible.isEmpty
                  ? AmitiaEmptyState(
                      icon: _pageIcon,
                      title: _kind == 'profile'
                          ? '暂无画像'
                          : _kind == 'episodic'
                          ? '暂无情景记忆'
                          : '暂无记忆',
                      subtitle: '与角色对话后，记忆会自动生成',
                    )
                  : widget.timeline
                  ? _buildTimeline(context, visible)
                  : ListView.separated(
                      padding: EdgeInsets.symmetric(
                        horizontal: AppSpacing.pagePadding,
                      ),
                      itemCount:
                          visible.length +
                          (_kind == 'graph' ? 1 : 0) +
                          (_next.isNotEmpty || _nextLegacy.isNotEmpty ? 1 : 0),
                      separatorBuilder: (_, _) =>
                          SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) {
                        if (_kind == 'graph' && index == 0)
                          return OwnedMemoryGraph(rows: rows);
                        final rowIndex = index - (_kind == 'graph' ? 1 : 0);
                        if (rowIndex >= visible.length)
                          return TextButton(
                            onPressed: _loading
                                ? null
                                : () => _load(older: true),
                            child: const Text('加载更多'),
                          );
                        final row = visible[rowIndex];
                        final current =
                            _source == 'current' && !_loading && !_managing;
                        final card = HistoricalMemoryCard(
                          key: ValueKey('owned-memory/${row['id']}'),
                          row: row,
                          kind: _kind,
                          manager: manager,
                          timeline: widget.timeline,
                          onDetail: () => _showRecordDetail(row),
                          onEdit: current && (row['revision'] as num? ?? 0) > 0
                              ? _kind == 'candidate'
                                    ? () => _candidate(row, 'update')
                                    : () => _editContent(row)
                              : null,
                          onDelete:
                              current &&
                                  const [
                                    'memory',
                                    'summary',
                                    'profile',
                                    'episodic',
                                  ].contains(_kind) &&
                                  (row['revision'] as num? ?? 0) > 0
                              ? () => _edit(row, deleted: true)
                              : null,
                        );
                        return card;
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildTimeline(BuildContext context, List<Map<String, dynamic>> rows) {
    final groups = <String, List<Map<String, dynamic>>>{};
    for (final row in rows) {
      final time = ownedMemoryResourceTime(row)?.toLocal();
      final date = time == null
          ? '未保存时间'
          : '${time.year}年${time.month}月${time.day}日';
      groups.putIfAbsent(date, () => []).add(row);
    }
    final entries = groups.entries.toList();
    return ListView.separated(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      itemCount:
          entries.length + (_next.isNotEmpty || _nextLegacy.isNotEmpty ? 1 : 0),
      separatorBuilder: (_, _) => SizedBox(height: AppSpacing.md),
      itemBuilder: (context, index) {
        if (index == entries.length)
          return TextButton(
            onPressed: _loading ? null : () => _load(older: true),
            child: const Text('加载更多'),
          );
        final group = entries[index];
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: EdgeInsets.only(
                bottom: AppSpacing.sm,
                top: AppSpacing.sm,
              ),
              child: Row(
                children: [
                  Container(
                    padding: EdgeInsets.symmetric(horizontal: 12, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Text(
                      group.key,
                      style: TextStyle(
                        fontSize: 13,
                        color: context.accentPrimary,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Stack(
              children: [
                Positioned(
                  left: 20,
                  top: 0,
                  bottom: 0,
                  child: Container(width: 1.5, color: context.borderPrimary),
                ),
                Column(
                  children: [
                    for (final row in group.value)
                      HistoricalMemoryCard(
                        row: row,
                        kind: (row['kind'] ?? 'memory').toString(),
                        timeline: true,
                        onDetail: () => _showRecordDetail(row),
                      ),
                  ],
                ),
              ],
            ),
          ],
        );
      },
    );
  }

  Widget _historicalFilterChip(BuildContext context, String label) {
    return GestureDetector(
      onTap: _showControls,
      child: Container(
        padding: EdgeInsets.symmetric(horizontal: 12, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
          border: Border.all(color: context.borderPrimary, width: .5),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              label,
              style: TextStyle(fontSize: 12, color: context.textSecondary),
            ),
            const SizedBox(width: 4),
            Icon(Icons.arrow_drop_down, size: 16, color: context.textTertiary),
          ],
        ),
      ),
    );
  }

  Future<void> _showControls() async {
    final owned = ref.read(memoryDataSourceProvider);
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => SingleChildScrollView(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.sm,
          AppSpacing.pagePadding,
          AppSpacing.xl + MediaQuery.viewInsetsOf(sheetContext).bottom,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisSize: MainAxisSize.min,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    '筛选与管理',
                    style: AppTypography.sectionTitle(context),
                  ),
                ),
                AmitiaIconButton(
                  icon: Icons.close,
                  tooltip: '关闭筛选',
                  onPressed: () => Navigator.pop(sheetContext),
                ),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (owned.policy?['coordinated'] == true &&
                    _kind != 'candidate')
                  Expanded(
                    child: DropdownButtonFormField<String>(
                      key: ValueKey('source/$_source'),
                      isExpanded: true,
                      initialValue: _source,
                      decoration: const InputDecoration(labelText: '数据来源'),
                      items: [
                        const DropdownMenuItem(
                          value: 'current',
                          child: Text('当前 Core 的数据'),
                        ),
                        ...(_kind == 'profile'
                                ? _historicalRoles.take(1)
                                : _historicalRoles)
                            .map(
                              (row) => DropdownMenuItem(
                                value: 'history:${row['id']}',
                                child: Text(
                                  _kind == 'profile'
                                      ? '原设备历史'
                                      : '原设备历史 · ${row['name'] ?? row['id']}',
                                ),
                              ),
                            ),
                      ],
                      onChanged: _loading
                          ? null
                          : (value) {
                              _source = value ?? 'current';
                              final options = ownedMemoryViewOptions(
                                _source,
                                owned.roles,
                                _historicalRoles,
                                kind: _kind,
                              );
                              if (!options.any(
                                (option) => option.value == _memoryView,
                              )) {
                                _memoryView = options.first.value;
                              }
                              if (_role.isNotEmpty) Navigator.pop(sheetContext);
                              _load();
                            },
                    ),
                  ),
                ...[
                  if (owned.policy?['coordinated'] == true &&
                      _kind != 'candidate')
                    const SizedBox(width: 12),
                  Expanded(
                    child: DropdownButtonFormField<String>(
                      key: ValueKey('view/$_memoryView/$_source'),
                      isExpanded: true,
                      initialValue: _memoryView,
                      decoration: const InputDecoration(labelText: '查看范围'),
                      items:
                          ownedMemoryViewOptions(
                                _source,
                                owned.roles,
                                _historicalRoles,
                                kind: _kind,
                              )
                              .map(
                                (option) => DropdownMenuItem(
                                  value: option.value,
                                  child: Text(option.label),
                                ),
                              )
                              .toList(),
                      onChanged: _loading
                          ? null
                          : (value) {
                              _memoryView = value ?? 'all';
                              if (_memoryView.startsWith('role:'))
                                _role = _memoryView.substring(5);
                              if (_source == 'current') _historicalRoles = [];
                              if (_role.isNotEmpty) Navigator.pop(sheetContext);
                              _load();
                            },
                    ),
                  ),
                ],
              ],
            ),
            if (_kind == 'profile') ...[
              SizedBox(height: AppSpacing.sm),
              Text('用户画像全局共享', style: AppTypography.caption(context)),
            ],
            if (_source != 'current') ...[
              SizedBox(height: AppSpacing.sm),
              Text(
                '原设备历史数据可供 Core 调用；请在原设备管理这些历史记录。',
                style: AppTypography.caption(context),
              ),
            ],
            if (_searchVisible &&
                _source == 'current' &&
                const ['memory', 'candidate'].contains(_kind)) ...[
              SizedBox(height: AppSpacing.md),
              TextFormField(
                key: ValueKey('search/$_kind'),
                initialValue: _query,
                decoration: const InputDecoration(
                  hintText: '搜索记忆',
                  prefixIcon: Icon(Icons.search),
                  border: OutlineInputBorder(),
                ),
                textInputAction: TextInputAction.search,
                onFieldSubmitted: (value) {
                  _query = value.trim();
                  Navigator.pop(sheetContext);
                  _load();
                },
              ),
            ],
            if (_source == 'current' && _kind == 'memory') ...[
              SizedBox(height: AppSpacing.sm),
              HistoricalMemorySurface(
                padding: EdgeInsets.zero,
                child: ExpansionTile(
                  key: const ValueKey('memory-filters'),
                  leading: Icon(Icons.tune, color: context.accentPrimary),
                  title: Text('筛选记忆', style: AppTypography.body(context)),
                  subtitle: Text(
                    '类型、来源与检索方式',
                    style: AppTypography.caption(context),
                  ),
                  childrenPadding: EdgeInsets.all(AppSpacing.md),
                  children: [
                    if (_kind == 'memory') ...[
                      TextFormField(
                        initialValue: _memoryType,
                        decoration: const InputDecoration(
                          labelText: '按类型过滤（留空为全部）',
                        ),
                        onFieldSubmitted: (value) {
                          _memoryType = value.trim();
                          Navigator.pop(sheetContext);
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
                          Navigator.pop(sheetContext);
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
                                Navigator.pop(sheetContext);
                                _load();
                              },
                      ),
                    ],
                    if (_kind == 'memory')
                      DropdownButtonFormField<String>(
                        initialValue: _searchMode,
                        decoration: const InputDecoration(labelText: '检索方式'),
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
                                Navigator.pop(sheetContext);
                                _load();
                              },
                      ),
                  ],
                ),
              ),
            ],
            SizedBox(height: AppSpacing.sm),
            if (_kind == 'profile' && _source == 'current' && _role.isNotEmpty)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton.icon(
                  onPressed: _loading ? null : _openCreateProfile,
                  icon: const Icon(Icons.add),
                  label: const Text('新增画像'),
                ),
              ),
            if (_source == 'current' &&
                const ['memory', 'candidate'].contains(_kind))
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  TextButton(
                    onPressed: _managementPage == null || _loading || _managing
                        ? null
                        : _kind == 'candidate'
                        ? _generateCandidates
                        : _createMemory,
                    child: Text(_kind == 'candidate' ? '从已保存对话生成候选' : '新增记忆'),
                  ),
                  if (_kind == 'memory')
                    Wrap(
                      children: [
                        TextButton(
                          onPressed: _loading || _managing
                              ? null
                              : _generateCandidates,
                          child: const Text('从对话生成候选'),
                        ),
                        TextButton(
                          onPressed: _loading || _managing
                              ? null
                              : () {
                                  setState(() => _kind = 'candidate');
                                  Navigator.pop(sheetContext);
                                  _load();
                                },
                          child: const Text('待审核候选'),
                        ),
                      ],
                    ),
                  if (_kind == 'candidate')
                    TextButton(
                      onPressed: _loading || _managing
                          ? null
                          : () {
                              setState(() => _kind = 'memory');
                              Navigator.pop(sheetContext);
                              _load();
                            },
                      child: const Text('返回记忆总览'),
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
              ),
            if (_source == 'current' &&
                _memoryView != 'all' &&
                const ['vector', 'graph'].contains(_kind)) ...[
              for (final layer
                  in (_projection?['status']?['layers'] as List? ?? [])
                      .whereType<Map>())
                Text(
                  '${layer['kind'] == 'vector' ? '向量' : '图谱'}：已处理 ${layer['current']}/${layer['total']} · 待更新 ${layer['pending']} · 重试 ${layer['retrying']}',
                ),
              if (_projectionNotice.isNotEmpty) Text(_projectionNotice),
              TextButton(
                onPressed: _projection == null || _loading || _rebuilding
                    ? null
                    : _rebuild,
                child: Text(_rebuilding ? '正在提交' : '重建当前范围索引'),
              ),
            ],

            if (_error.isNotEmpty)
              Text(
                _error,
                style: AppTypography.body(
                  context,
                ).copyWith(color: context.error),
              ),
          ],
        ),
      ),
    );
  }

  String _profileCategory = '全部';
  final _searchController = TextEditingController();

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  Widget _buildCategoryTabs(BuildContext context) {
    final categories = _kind == 'profile'
        ? const {
            '全部': '',
            '事实': 'fact',
            '偏好': 'preference',
            '习惯': 'habit',
            '关系': 'relationship',
          }
        : const {
            '全部': '',
            '长期记忆': 'fact',
            '个人信息': 'personal_info',
            '偏好记忆': 'preference',
            '关系记忆': 'relationship',
            '计划': 'plan',
          };
    return SizedBox(
      height: MediaQuery.textScalerOf(context).scale(13) + 25,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: categories.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final category = categories.entries.elementAt(index);
          final selected = _kind == 'profile'
              ? _profileCategory == category.key
              : _memoryType == category.value;
          return GestureDetector(
            onTap: _loading
                ? null
                : () {
                    setState(() {
                      if (_kind == 'profile') {
                        _profileCategory = category.key;
                      } else {
                        _memoryType = category.value;
                      }
                    });
                    if (_kind != 'profile') _load();
                  },
            child: Container(
              padding: EdgeInsets.symmetric(
                horizontal: AppSpacing.md,
                vertical: 8,
              ),
              decoration: BoxDecoration(
                color: selected
                    ? context.accentPrimary
                    : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
                border: Border.all(
                  color: selected ? context.accentPrimary : Colors.transparent,
                  width: .5,
                ),
              ),
              child: Center(
                child: Text(
                  category.key,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: selected ? FontWeight.w500 : FontWeight.w400,
                    color: selected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildTools(BuildContext context) {
    final tools = <_ToolEntry>[
      _ToolEntry(
        title: '记忆管理',
        subtitle: '查看与管理全部记忆',
        icon: Icons.folder_special_outlined,
        route: AppRoutes.memoryManager,
      ),
      _ToolEntry(
        title: '情景记忆',
        subtitle: '回顾情景与事件',
        icon: Icons.event_note_outlined,
        route: AppRoutes.memoryEpisodic,
      ),
      _ToolEntry(
        title: '记忆图谱',
        subtitle: '可视化记忆关联',
        icon: Icons.account_tree_outlined,
        route: AppRoutes.memoryGraph,
      ),
      _ToolEntry(
        title: '记忆时间线',
        subtitle: '按时间浏览记忆',
        icon: Icons.timeline,
        route: AppRoutes.memoryTimeline,
      ),
      _ToolEntry(
        title: '用户画像',
        subtitle: '用户偏好与特征',
        icon: Icons.person_outline,
        route: AppRoutes.memoryProfiles,
      ),
      _ToolEntry(
        title: '世界书',
        subtitle: '世界观与设定',
        icon: Icons.menu_book_outlined,
        route: AppRoutes.memoryWorldBook,
      ),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(height: AppSpacing.sm),
        const HistoricalMemorySectionHeader(title: '记忆工具'),
        SizedBox(height: AppSpacing.sm),
        Padding(
          padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 2,
              mainAxisSpacing: AppSpacing.sm,
              crossAxisSpacing: AppSpacing.sm,
              childAspectRatio: MediaQuery.textScalerOf(context).scale(14) > 20
                  ? 1.7
                  : 3.0,
            ),
            itemCount: tools.length,
            itemBuilder: (context, index) {
              final t = tools[index];
              return GestureDetector(
                onTap: () => context.push(t.route),
                child: HistoricalMemorySurface(
                  borderRadius: AppRadius.brMedium,
                  padding: EdgeInsets.symmetric(
                    horizontal: AppSpacing.sm,
                    vertical: AppSpacing.xs,
                  ),
                  child: Row(
                    children: [
                      Container(
                        width: 34,
                        height: 34,
                        decoration: BoxDecoration(
                          color: context.accentSoft,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: Icon(
                          t.icon,
                          size: 18,
                          color: context.accentPrimary,
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisAlignment: MainAxisAlignment.center,
                          children: [
                            Text(
                              t.title,
                              style: AppTypography.body(context),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                            SizedBox(height: 2),
                            Text(
                              t.subtitle,
                              style: AppTypography.label(context),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              );
            },
          ),
        ),
        SizedBox(height: AppSpacing.md),
      ],
    );
  }

  void _detailAction(BuildContext sheetContext, VoidCallback action) {
    Navigator.pop(sheetContext);
    action();
  }

  Future<void> _showRecordDetail(Map<String, dynamic> row) async {
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      showDragHandle: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => Consumer(
        builder: (context, ref, child) {
          final body = ownedMemoryBody(row);
          final fields = ownedMemoryFields(row, kind: _kind);
          return SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.sm,
              AppSpacing.pagePadding,
              AppSpacing.lg + MediaQuery.viewInsetsOf(context).bottom,
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        ownedMemoryTitle(row),
                        style: AppTypography.sectionTitle(context),
                      ),
                    ),
                    AmitiaIconButton(
                      icon: Icons.close,
                      tooltip: '关闭详情',
                      onPressed: () => Navigator.pop(sheetContext),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.md),
                if (fields.isEmpty)
                  SelectableText(
                    _content(row),
                    style: AppTypography.body(context),
                  ),
                for (final field in fields) ...[
                  Text(field.label, style: AppTypography.caption(context)),
                  const SizedBox(height: 4),
                  SelectableText(
                    field.value,
                    style: AppTypography.body(context),
                  ),
                  SizedBox(height: AppSpacing.md),
                ],
                Text(
                  '版本 ${row['revision'] ?? 0}${body['allowContextUse'] == false ? ' · 已停用' : ''}',
                  style: AppTypography.caption(context),
                ),
                SizedBox(height: AppSpacing.md),
                Wrap(
                  spacing: 8,
                  runSpacing: 8,
                  children: [
                    if (_source == 'current' &&
                        const [
                          'memory',
                          'legacy',
                          'episodic',
                          'fact',
                          'graph',
                          'vector',
                        ].contains(_kind) &&
                        row['executionScope'] is Map)
                      TextButton(
                        onPressed: _loading || _managing
                            ? null
                            : () => _detailAction(
                                sheetContext,
                                () => _transferScope(row),
                              ),
                        child: Text(
                          row['roleId'] == globalMemoryRole
                              ? '降级为角色记忆'
                              : '提升为全局记忆',
                        ),
                      ),
                    if (_kind == 'candidate' && _source == 'current')
                      Wrap(
                        children: [
                          TextButton(
                            onPressed: _loading || _managing
                                ? null
                                : () => _detailAction(
                                    sheetContext,
                                    () => _candidate(row, 'update'),
                                  ),
                            child: const Text('编辑'),
                          ),
                          TextButton(
                            onPressed: _loading || _managing
                                ? null
                                : () => _detailAction(
                                    sheetContext,
                                    () => _candidate(row, 'accept'),
                                  ),
                            child: const Text('接受并保存'),
                          ),
                          TextButton(
                            onPressed: _loading || _managing
                                ? null
                                : () => _detailAction(
                                    sheetContext,
                                    () => _candidate(row, 'reject'),
                                  ),
                            child: const Text('拒绝'),
                          ),
                        ],
                      ),
                    if (const [
                          'memory',
                          'summary',
                          'profile',
                          'episodic',
                        ].contains(_kind) &&
                        _source == 'current' &&
                        (row['revision'] as num? ?? 0) > 0)
                      Wrap(
                        children: [
                          TextButton(
                            onPressed: _loading
                                ? null
                                : () => _detailAction(
                                    sheetContext,
                                    () => _editContent(row),
                                  ),
                            child: const Text('编辑'),
                          ),
                          if (_kind == 'memory') ...[
                            TextButton(
                              onPressed: _loading
                                  ? null
                                  : () => _detailAction(
                                      sheetContext,
                                      () => _setExpiry(row),
                                    ),
                              child: const Text('到期时间'),
                            ),
                            TextButton(
                              onPressed: _loading
                                  ? null
                                  : () => _detailAction(
                                      sheetContext,
                                      () => _edit(
                                        row,
                                        changes: {
                                          'archivedAt':
                                              row['body']?['archivedAt'] == null
                                              ? DateTime.now()
                                                    .toUtc()
                                                    .toIso8601String()
                                              : null,
                                        },
                                      ),
                                    ),
                              child: Text(
                                row['body']?['archivedAt'] == null
                                    ? '归档'
                                    : '取消归档',
                              ),
                            ),
                            TextButton(
                              onPressed: _loading
                                  ? null
                                  : () => _detailAction(
                                      sheetContext,
                                      () => _edit(row),
                                    ),
                              child: Text(
                                row['body']?['allowContextUse'] == false
                                    ? '允许用于对话'
                                    : '停用',
                              ),
                            ),
                          ],
                          TextButton(
                            onPressed: _loading
                                ? null
                                : () => _detailAction(
                                    sheetContext,
                                    () => _edit(row, deleted: true),
                                  ),
                            child: const Text('删除'),
                          ),
                        ],
                      ),
                  ],
                ),
              ],
            ),
          );
        },
      ),
    );
  }
}

class _ToolEntry {
  final String title;
  final String subtitle;
  final IconData icon;
  final String route;

  _ToolEntry({
    required this.title,
    required this.subtitle,
    required this.icon,
    required this.route,
  });
}
