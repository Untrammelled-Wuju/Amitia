import 'dart:convert';
import 'dart:developer' as developer;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class CharacterTeamsPage extends ConsumerStatefulWidget {
  const CharacterTeamsPage({super.key});

  @override
  ConsumerState<CharacterTeamsPage> createState() => _CharacterTeamsPageState();
}

class _CharacterTeamsPageState extends ConsumerState<CharacterTeamsPage> {
  List<Map<String, dynamic>> _teams = const [];
  List<Map<String, dynamic>> _roles = const [];
  List<Map<String, dynamic>> _templates = const [];
  Map<String, dynamic>? _detail;
  String _selectedId = '';
  bool _loading = true;
  bool _acting = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<dynamic> _get(String path, {Map<String, dynamic>? query}) async {
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) throw StateError('执行 Core 尚未就绪');
    return api.get<dynamic>(path, queryParameters: query);
  }

  Future<dynamic> _post(String path, {Object? data}) async {
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) throw StateError('执行 Core 尚未就绪');
    return api.post<dynamic>(path, data: data);
  }

  Future<dynamic> _put(String path, {Object? data}) async {
    final api = ref.read(rawBackendServiceApiProvider);
    if (api == null) throw StateError('执行 Core 尚未就绪');
    return api.put<dynamic>(path, data: data);
  }

  List<Map<String, dynamic>> _list(dynamic value) {
    if (value is! List) return const [];
    return value
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final values = await Future.wait<dynamic>([
        _get('/api/roles/teams'),
        _get('/api/roles', query: {'includeDisabled': false}),
        _get('/api/roles/teams/templates'),
      ]);
      final teams = _list(values[0]);
      if (!mounted) return;
      setState(() {
        _teams = teams;
        _roles = _list(values[1]);
        _templates = _list(values[2]);
      });
      final nextID = teams.any((team) => team['id']?.toString() == _selectedId)
          ? _selectedId
          : teams.firstOrNull?['id']?.toString() ?? '';
      if (nextID.isNotEmpty) {
        await _selectTeam(nextID);
      } else {
        if (mounted) {
          setState(() {
            _selectedId = '';
            _detail = null;
          });
        }
      }
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.load',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _selectTeam(String id) async {
    try {
      final detail = await _get('/api/roles/teams/$id');
      if (!mounted) return;
      setState(() {
        _selectedId = id;
        _detail = detail is Map ? Map<String, dynamic>.from(detail) : null;
        _error = null;
      });
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.select',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) setState(() => _error = error.toString());
    }
  }

  Map<String, dynamic> get _team => _detail?['team'] is Map
      ? Map<String, dynamic>.from(_detail!['team'] as Map)
      : <String, dynamic>{};

  List<Map<String, dynamic>> get _members =>
      _list(_detail?['members']).toList(growable: false);

  List<Map<String, dynamic>> get _contexts =>
      _list(_detail?['contexts']).toList(growable: false);

  List<Map<String, dynamic>> get _handoffs =>
      _list(_detail?['handoffs']).toList(growable: false);

  String _roleName(dynamic id) =>
      _roles
          .where((role) => role['id']?.toString() == id?.toString())
          .map((role) => role['name']?.toString() ?? '')
          .firstOrNull ??
      id?.toString() ??
      '系统';

  Future<void> _createTeam() async {
    final value = await _showTeamEditor(title: '新建团队');
    if (value == null) return;
    setState(() => _acting = true);
    try {
      final created = value['templateId']?.toString().isNotEmpty == true
          ? await _post('/api/roles/teams/from-template', data: value)
          : await _post('/api/roles/teams', data: value);
      final id =
          (created is Map ? created['id'] : null)?.toString() ??
          (created is Map && created['team'] is Map
              ? (created['team'] as Map)['id']?.toString()
              : null) ??
          '';
      await _load();
      if (id.isNotEmpty) await _selectTeam(id);
      if (mounted) amitiaSnackBar(context, '团队已创建');
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.create',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  Future<void> _editTeam() async {
    if (_detail == null) return;
    final value = await _showTeamEditor(title: '编辑团队', initial: _team);
    if (value == null) return;
    setState(() => _acting = true);
    try {
      await _put(
        '/api/roles/teams/${_team['id']}',
        data: {...value, 'expectedRevision': _team['revision']},
      );
      await _load();
      if (mounted) amitiaSnackBar(context, '团队已更新');
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.update',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  Future<void> _addMember() async {
    if (_detail == null) return;
    final memberIds = _members
        .map((member) => member['memberId']?.toString())
        .whereType<String>()
        .toSet();
    final available = _roles
        .where((role) => !memberIds.contains(role['id']?.toString()))
        .toList(growable: false);
    if (available.isEmpty) {
      amitiaSnackBar(context, '没有可加入团队的角色');
      return;
    }
    String roleId = available.first['id'].toString();
    String role = 'member';
    final value = await showDialog<Map<String, String>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('添加团队成员'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                initialValue: roleId,
                decoration: const InputDecoration(labelText: '角色'),
                items: available
                    .map(
                      (item) => DropdownMenuItem(
                        value: item['id'].toString(),
                        child: Text(item['name']?.toString() ?? ''),
                      ),
                    )
                    .toList(growable: false),
                onChanged: (next) {
                  if (next != null) setDialogState(() => roleId = next);
                },
              ),
              SizedBox(height: AppSpacing.sm),
              DropdownButtonFormField<String>(
                initialValue: role,
                decoration: const InputDecoration(labelText: '团队职责'),
                items: const [
                  DropdownMenuItem(value: 'member', child: Text('成员')),
                  DropdownMenuItem(value: 'lead', child: Text('负责人')),
                ],
                onChanged: (next) {
                  if (next != null) setDialogState(() => role = next);
                },
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, {
                'roleId': roleId,
                'role': role,
              }),
              child: const Text('加入'),
            ),
          ],
        ),
      ),
    );
    if (value == null) return;
    setState(() => _acting = true);
    try {
      await _post(
        '/api/roles/teams/${_team['id']}/members',
        data: {
          'memberType': 'agent',
          'memberId': value['roleId'],
          'role': value['role'],
          'responderMode': value['role'] == 'lead' ? 'lead' : 'mention_or_lead',
          'capabilities': value['role'] == 'lead'
              ? const ['*']
              : const <String>[],
        },
      );
      await _selectTeam(_selectedId);
      if (mounted) amitiaSnackBar(context, '角色已加入团队');
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.member',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  Future<void> _addContext() async {
    if (_detail == null) return;
    final content = await _askText('添加共享上下文', '规则、决策或共享知识', maxLines: 4);
    if (content == null || content.trim().isEmpty) return;
    setState(() => _acting = true);
    try {
      await _post(
        '/api/roles/teams/${_team['id']}/contexts',
        data: {
          'kind': 'instruction',
          'key': '',
          'content': content.trim(),
          'visibility': 'team',
        },
      );
      await _selectTeam(_selectedId);
      if (mounted) amitiaSnackBar(context, '共享上下文已添加');
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.context',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  Future<void> _testRoute() async {
    if (_detail == null) return;
    final message = await _askText('团队消息路由', '测试消息');
    if (message == null || message.trim().isEmpty) return;
    setState(() => _acting = true);
    try {
      final result = await _post(
        '/api/roles/teams/${_team['id']}/route',
        data: {
          'message': message.trim(),
          'conversationId':
              'team-route-${DateTime.now().microsecondsSinceEpoch}',
        },
      );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('路由结果'),
          content: SelectableText(
            const JsonEncoder.withIndent('  ').convert(result),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.route',
        error: error,
        stackTrace: stackTrace,
      );
      if (mounted) amitiaSnackBar(context, error.toString());
    } finally {
      if (mounted) setState(() => _acting = false);
    }
  }

  Future<Map<String, dynamic>?> _showTeamEditor({
    required String title,
    Map<String, dynamic> initial = const {},
  }) async {
    final name = TextEditingController(text: initial['name']?.toString() ?? '');
    final description = TextEditingController(
      text: initial['description']?.toString() ?? '',
    );
    String templateId = '';
    String leadAgentId = initial['leadAgentId']?.toString() ?? '';
    final selectedMemberIds = <String>{};
    bool sharedMemory = initial['sharedMemory'] as bool? ?? true;
    bool sharedWorkspace = initial['sharedWorkspace'] as bool? ?? true;
    final value = await showModalBottomSheet<Map<String, dynamic>>(
      context: context,
      isScrollControlled: true,
      useSafeArea: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (context, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            MediaQuery.viewInsetsOf(context).bottom + AppSpacing.lg,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.pageTitle(context)),
                SizedBox(height: AppSpacing.md),
                if (initial.isEmpty) ...[
                  DropdownButtonFormField<String>(
                    initialValue: templateId.isEmpty ? null : templateId,
                    decoration: const InputDecoration(labelText: '团队模板'),
                    items: _templates
                        .map(
                          (item) => DropdownMenuItem(
                            value: item['id'].toString(),
                            child: Text(item['name']?.toString() ?? ''),
                          ),
                        )
                        .toList(growable: false),
                    onChanged: (value) {
                      if (value == null) return;
                      final template = _templates.firstWhere(
                        (item) => item['id']?.toString() == value,
                      );
                      setSheetState(() {
                        templateId = value;
                        name.text = template['name']?.toString() ?? '';
                        description.text =
                            template['description']?.toString() ?? '';
                        sharedMemory = template['sharedMemory'] == true;
                        sharedWorkspace = template['sharedWorkspace'] == true;
                      });
                    },
                  ),
                  SizedBox(height: AppSpacing.sm),
                ],
                TextField(
                  controller: name,
                  onChanged: (_) => setSheetState(() {}),
                  decoration: const InputDecoration(labelText: '团队名称'),
                ),
                SizedBox(height: AppSpacing.sm),
                TextField(
                  controller: description,
                  minLines: 2,
                  maxLines: 4,
                  decoration: const InputDecoration(labelText: '说明'),
                ),
                SizedBox(height: AppSpacing.sm),
                DropdownButtonFormField<String>(
                  initialValue: leadAgentId.isEmpty ? null : leadAgentId,
                  decoration: const InputDecoration(labelText: '负责人'),
                  items: _roles
                      .map(
                        (role) => DropdownMenuItem(
                          value: role['id'].toString(),
                          child: Text(role['name']?.toString() ?? ''),
                        ),
                      )
                      .toList(growable: false),
                  onChanged: (value) => setSheetState(() {
                    leadAgentId = value ?? '';
                    selectedMemberIds.remove(leadAgentId);
                  }),
                ),
                if (initial.isEmpty) ...[
                  SizedBox(height: AppSpacing.sm),
                  Text('选择团队成员', style: AppTypography.label(context)),
                  SizedBox(height: AppSpacing.xs),
                  Wrap(
                    spacing: AppSpacing.xs,
                    runSpacing: AppSpacing.xs,
                    children: _roles
                        .where((role) => role['id']?.toString() != leadAgentId)
                        .map((role) {
                          final id = role['id'].toString();
                          return FilterChip(
                            label: Text(role['name']?.toString() ?? ''),
                            selected: selectedMemberIds.contains(id),
                            onSelected: (selected) => setSheetState(() {
                              if (selected) {
                                selectedMemberIds.add(id);
                              } else {
                                selectedMemberIds.remove(id);
                              }
                            }),
                          );
                        })
                        .toList(growable: false),
                  ),
                ],
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('共享团队记忆'),
                  value: sharedMemory,
                  onChanged: (value) =>
                      setSheetState(() => sharedMemory = value),
                ),
                SwitchListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('共享团队文件'),
                  value: sharedWorkspace,
                  onChanged: (value) =>
                      setSheetState(() => sharedWorkspace = value),
                ),
                SizedBox(height: AppSpacing.md),
                SizedBox(
                  width: double.infinity,
                  child: FilledButton(
                    onPressed: name.text.trim().isEmpty
                        ? null
                        : () => Navigator.pop(sheetContext, {
                            'templateId': templateId,
                            'name': name.text.trim(),
                            'description': description.text.trim(),
                            'leadAgentId': leadAgentId,
                            'memberIds': selectedMemberIds.toList(
                              growable: false,
                            ),
                            'sharedMemory': sharedMemory,
                            'sharedWorkspace': sharedWorkspace,
                          }),
                    child: Text(initial.isEmpty ? '创建团队' : '保存团队'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    name.dispose();
    description.dispose();
    return value;
  }

  Future<String?> _askText(
    String title,
    String label, {
    String initial = '',
    int maxLines = 1,
  }) async {
    final controller = TextEditingController(text: initial);
    final value = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          autofocus: true,
          minLines: maxLines,
          maxLines: maxLines,
          decoration: InputDecoration(labelText: label),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, controller.text),
            child: const Text('确定'),
          ),
        ],
      ),
    );
    controller.dispose();
    return value;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '团队',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            tooltip: '新建团队',
            onPressed: _acting ? null : _createTeam,
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                if (_error != null)
                  Container(
                    padding: EdgeInsets.all(AppSpacing.sm),
                    decoration: BoxDecoration(
                      color: context.error.withValues(alpha: 0.1),
                      border: Border.all(color: context.error),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: SelectableText(
                      _error!,
                      style: TextStyle(color: context.error),
                    ),
                  ),
                if (_error != null) SizedBox(height: AppSpacing.md),
                if (_teams.isEmpty)
                  AmitiaEmptyState(
                    icon: Icons.groups_outlined,
                    title: '暂无团队',
                    subtitle: '创建团队后，可统一管理负责人、成员和共享上下文。',
                    actionText: '新建团队',
                    onAction: _createTeam,
                  )
                else ...[
                  SizedBox(
                    height: 92,
                    child: ListView.separated(
                      scrollDirection: Axis.horizontal,
                      itemCount: _teams.length,
                      separatorBuilder: (_, _) => SizedBox(width: 8),
                      itemBuilder: (context, index) {
                        final team = _teams[index];
                        final active = team['id']?.toString() == _selectedId;
                        return SizedBox(
                          width: 210,
                          child: Material(
                            color: active
                                ? context.accentSoft
                                : context.surfacePrimary,
                            borderRadius: BorderRadius.circular(10),
                            child: InkWell(
                              onTap: () => _selectTeam(team['id'].toString()),
                              borderRadius: BorderRadius.circular(10),
                              child: Padding(
                                padding: const EdgeInsets.all(11),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      team['name']?.toString() ?? '',
                                      maxLines: 1,
                                      overflow: TextOverflow.ellipsis,
                                      style: AppTypography.cardTitle(context),
                                    ),
                                    SizedBox(height: 5),
                                    Expanded(
                                      child: Text(
                                        team['description']?.toString() ??
                                            '暂无团队说明',
                                        maxLines: 2,
                                        overflow: TextOverflow.ellipsis,
                                        style: AppTypography.caption(context),
                                      ),
                                    ),
                                  ],
                                ),
                              ),
                            ),
                          ),
                        );
                      },
                    ),
                  ),
                  SizedBox(height: AppSpacing.md),
                  if (_detail != null) ..._buildDetail(context),
                ],
              ],
            ),
    );
  }

  List<Widget> _buildDetail(BuildContext context) {
    final members = _members;
    final contexts = _contexts;
    final handoffs = _handoffs;
    return [
      Container(
        padding: EdgeInsets.all(AppSpacing.md),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          border: Border.all(color: context.borderPrimary),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    _team['name']?.toString() ?? '',
                    style: AppTypography.sectionTitle(context),
                  ),
                ),
                TextButton(
                  onPressed: _acting ? null : _editTeam,
                  child: const Text('编辑'),
                ),
              ],
            ),
            Text(
              _team['description']?.toString() ?? '暂无团队说明',
              style: AppTypography.bodySmall(context),
            ),
            SizedBox(height: AppSpacing.sm),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.xs,
              children: [
                _chip(context, '${members.length} 名成员'),
                _chip(context, '${contexts.length} 条上下文'),
                _chip(context, 'revision ${_team['revision'] ?? 1}'),
                _chip(context, _team['sharedMemory'] == true ? '共享记忆' : '独立记忆'),
                _chip(
                  context,
                  _team['sharedWorkspace'] == true ? '共享文件' : '独立文件',
                ),
              ],
            ),
            SizedBox(height: AppSpacing.md),
            Wrap(
              spacing: AppSpacing.sm,
              runSpacing: AppSpacing.sm,
              children: [
                OutlinedButton(
                  onPressed: _acting ? null : _addMember,
                  child: const Text('添加成员'),
                ),
                OutlinedButton(
                  onPressed: _acting ? null : _addContext,
                  child: const Text('添加共享上下文'),
                ),
                OutlinedButton(
                  onPressed: _acting ? null : _testRoute,
                  child: const Text('测试路由'),
                ),
              ],
            ),
          ],
        ),
      ),
      SizedBox(height: AppSpacing.md),
      _section(
        context,
        '角色成员',
        members.isEmpty
            ? Text('暂无成员', style: AppTypography.caption(context))
            : Column(
                children: members
                    .map(
                      (member) => ListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(_roleName(member['memberId'])),
                        subtitle: Text(
                          '${member['memberType']} · ${member['responderMode']}',
                        ),
                        trailing: Text(
                          member['role']?.toString() ?? 'member',
                          style: AppTypography.caption(context),
                        ),
                      ),
                    )
                    .toList(growable: false),
              ),
      ),
      SizedBox(height: AppSpacing.md),
      _section(
        context,
        '共享上下文',
        contexts.isEmpty
            ? Text('暂无共享上下文', style: AppTypography.caption(context))
            : Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: contexts
                    .map(
                      (item) => Padding(
                        padding: const EdgeInsets.only(bottom: 10),
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              item['kind']?.toString() ?? 'instruction',
                              style: AppTypography.caption(context),
                            ),
                            SizedBox(height: 3),
                            Text(
                              item['content']?.toString() ?? '',
                              style: AppTypography.bodySmall(context),
                            ),
                          ],
                        ),
                      ),
                    )
                    .toList(growable: false),
              ),
      ),
      SizedBox(height: AppSpacing.md),
      _section(
        context,
        '任务交接',
        handoffs.isEmpty
            ? Text('暂无任务交接', style: AppTypography.caption(context))
            : Column(
                children: handoffs
                    .map(
                      (item) => ListTile(
                        contentPadding: EdgeInsets.zero,
                        title: Text(
                          item['summary']?.toString() ??
                              item['taskId']?.toString() ??
                              '',
                        ),
                        subtitle: Text(
                          '${_roleName(item['fromAgentId'])} → ${_roleName(item['toAgentId'])}',
                        ),
                        trailing: Text(
                          item['status']?.toString() ?? '',
                          style: AppTypography.caption(context),
                        ),
                      ),
                    )
                    .toList(growable: false),
              ),
      ),
    ];
  }

  Widget _chip(BuildContext context, String label) => Container(
    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
    decoration: BoxDecoration(
      color: context.surfaceSecondary,
      borderRadius: BorderRadius.circular(7),
    ),
    child: Text(label, style: AppTypography.caption(context)),
  );

  Widget _section(BuildContext context, String title, Widget child) =>
      Container(
        padding: EdgeInsets.all(AppSpacing.md),
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          border: Border.all(color: context.borderPrimary),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: AppTypography.cardTitle(context)),
            SizedBox(height: AppSpacing.sm),
            Material(type: MaterialType.transparency, child: child),
          ],
        ),
      );
}
