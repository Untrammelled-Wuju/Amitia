import 'dart:convert';
import 'dart:developer' as developer;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/backend_transport/role_management_target.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/shared_core_data_access.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/role_management_target_field.dart';
import '../../../../core/widgets/shared_core_read_only_banner.dart';

class CharacterTeamsPage extends ConsumerStatefulWidget {
  const CharacterTeamsPage({super.key});

  @override
  ConsumerState<CharacterTeamsPage> createState() => _CharacterTeamsPageState();
}

class _CharacterTeamsPageState extends ConsumerState<CharacterTeamsPage> {
  List<Map<String, dynamic>> _teams = const [];
  List<Map<String, dynamic>> _roles = const [];
  List<Map<String, dynamic>> _localRoles = const [];
  List<Map<String, dynamic>> _centerRoles = const [];
  List<Map<String, dynamic>> _devices = const [];
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

  Map<String, String>? _targetHeaders(RoleManagementTarget? target) =>
      withRoleManagementTarget(
        null,
        target ?? ref.read(roleManagementTargetProvider),
      );

  Future<dynamic> _get(
    String path, {
    Map<String, dynamic>? query,
    RoleManagementTarget? target,
  }) async {
    final api = ref.read(backendServiceProvider);
    return api.get<dynamic>(
      path,
      queryParameters: query,
      headers: _targetHeaders(target),
    );
  }

  Future<dynamic> _post(
    String path, {
    Object? data,
    RoleManagementTarget? target,
  }) async {
    final api = ref.read(backendServiceProvider);
    return api.post<dynamic>(
      path,
      data: data,
      headers: _targetHeaders(target),
    );
  }

  Future<dynamic> _put(
    String path, {
    Object? data,
    RoleManagementTarget? target,
  }) async {
    final api = ref.read(backendServiceProvider);
    return api.put<dynamic>(
      path,
      data: data,
      headers: _targetHeaders(target),
    );
  }

  Future<RoleManagementTarget?> _chooseCreateTarget() async {
    final isCloud =
        ref.read(mobileDeploymentConfigProvider).mode ==
        MobileDeploymentMode.cloud;
    if (!isCloud) return RoleManagementTarget.local;
    var selected = resolveRoleManagementTarget(
      ref.read(roleManagementTargetProvider),
      cloud: true,
    );
    final confirmed = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.md,
            AppSpacing.pagePadding,
            MediaQuery.viewInsetsOf(sheetContext).bottom + AppSpacing.lg,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('新建团队', style: AppTypography.cardTitle(sheetContext)),
              SizedBox(height: AppSpacing.xs),
              RoleManagementTargetField(
                noun: '团队',
                value: selected,
                onChanged: (value) => setSheetState(() => selected = value),
              ),
              SizedBox(height: AppSpacing.md),
              FilledButton(
                onPressed: () => Navigator.pop(sheetContext, true),
                child: const Text('继续'),
              ),
            ],
          ),
        ),
      ),
    );
    if (confirmed != true) return null;
    ref.read(roleManagementTargetProvider.notifier).state = selected;
    return selected;
  }

  List<Map<String, dynamic>> _list(dynamic value) {
    if (value is! List) return const [];
    return value
        .whereType<Map>()
        .map((item) => Map<String, dynamic>.from(item))
        .toList(growable: false);
  }

  Future<List<Map<String, dynamic>>> _safeRoles(
    RoleManagementTarget target,
  ) async {
    try {
      return _list(
        await _get(
          '/api/roles',
          query: {'includeDisabled': false},
          target: target,
        ),
      );
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.roles',
        error: error,
        stackTrace: stackTrace,
      );
      return const [];
    }
  }

  Future<List<Map<String, dynamic>>> _peerDevices() async {
    try {
      final devices = await ref.read(deviceMeshServiceProvider).devices();
      final selfId = await _localDeviceId();
      return devices
          .where((device) {
            final id = device['deviceId']?.toString() ?? '';
            return id.isNotEmpty && id != selfId;
          })
          .toList(growable: false);
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.devices',
        error: error,
        stackTrace: stackTrace,
      );
      return const [];
    }
  }

  Future<String> _localDeviceId() async {
    final service = ref.read(deviceMeshLocalServiceProvider);
    if (service == null) return '';
    try {
      final identity = await service.identity();
      return identity['deviceId']?.toString() ?? '';
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.identity',
        error: error,
        stackTrace: stackTrace,
      );
      return '';
    }
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final isCloud =
          ref.read(mobileDeploymentConfigProvider).mode ==
          MobileDeploymentMode.cloud;
      final values = await Future.wait<dynamic>([
        _get('/api/roles/teams'),
        _get('/api/roles', query: {'includeDisabled': false}),
        _get('/api/roles/teams/templates'),
        if (isCloud) _peerDevices(),
        if (isCloud) _safeRoles(RoleManagementTarget.local),
        if (isCloud) _safeRoles(RoleManagementTarget.center),
      ]);
      final teams = _list(values[0]);
      final roles = _list(values[1]);
      if (!mounted) return;
      setState(() {
        _teams = teams;
        _roles = roles;
        _templates = _list(values[2]);
        _devices = values.length > 3
            ? _list(values[3])
            : const <Map<String, dynamic>>[];
        _localRoles = values.length > 4 ? _list(values[4]) : roles;
        _centerRoles = values.length > 5 ? _list(values[5]) : roles;
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
    final target = await _chooseCreateTarget();
    if (target == null || !mounted) return;
    final canWrite =
        target == RoleManagementTarget.local || _canWrite;
    if (!canWrite) {
      amitiaSnackBar(context, '当前设备没有统筹中心写入权限');
      return;
    }
    await _load();
    if (!mounted) return;
    final value = await _showTeamEditor(title: '新建团队');
    if (value == null) return;
    setState(() => _acting = true);
    try {
      final created = value['templateId']?.toString().isNotEmpty == true
          ? await _post(
              '/api/roles/teams/from-template',
              data: value,
              target: target,
            )
          : await _post('/api/roles/teams', data: value, target: target);
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
    if (!_canWrite) return;
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
    if (!_canWrite) return;
    if (_detail == null) return;
    final existingKeys = _members
        .map(
          (member) =>
              '${member['memberType']}|${member['ownerType'] ?? 'center'}|${member['ownerId'] ?? ''}|${member['memberId']}',
        )
        .toSet();
    final selection = await showDialog<_TeamMemberSelection>(
      context: context,
      builder: (dialogContext) => _AddTeamMemberDialog(
        localRoles: _localRoles,
        centerRoles: _centerRoles,
        devices: _devices,
        roleDevices: _localModeDevices(),
        existingKeys: existingKeys,
        loadDeviceRoles: _loadDeviceRoles,
      ),
    );
    if (selection == null || !mounted) return;
    setState(() => _acting = true);
    try {
      await _post(
        '/api/roles/teams/${_team['id']}/members',
        data: {
          'memberType': selection.memberType,
          'memberId': selection.memberId,
          'ownerType': selection.ownerType,
          'ownerId': selection.ownerId,
          'invocationMode': selection.invocationMode,
          'roleRevision': selection.roleRevision,
          'role': selection.role,
          'responderMode': selection.role == 'lead' ? 'lead' : 'mention_or_lead',
          'capabilities': selection.role == 'lead'
              ? const ['*']
              : const <String>[],
        },
      );
      await _selectTeam(_selectedId);
      if (mounted) amitiaSnackBar(context, '成员已加入团队');
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

  Future<List<Map<String, dynamic>>> _loadDeviceRoles(String deviceId) {
    return ref.read(deviceMeshServiceProvider).deviceRoles(deviceId);
  }

  List<Map<String, dynamic>> _localModeDevices() {
    return _devices
        .where((device) {
          final coordination = device['coordination'];
          if (coordination is! Map) return true;
          return coordination['coordinated'] != true;
        })
        .toList(growable: false);
  }

  String _deviceName(String? deviceId) {
    if (deviceId == null || deviceId.isEmpty) return '';
    return _devices
            .where((device) => device['deviceId']?.toString() == deviceId)
            .map(
              (device) =>
                  (device['reportedName'] ??
                          device['label'] ??
                          device['deviceId'] ??
                          '')
                      .toString(),
            )
            .firstOrNull ??
        deviceId;
  }

  String _memberLabel(Map<String, dynamic> member) {
    final ownerType = member['ownerType']?.toString() ?? 'center';
    final mode = member['invocationMode']?.toString() ?? 'role';
    final ownerId = member['ownerId']?.toString() ?? '';
    final memberId = member['memberId']?.toString() ?? '';
    if (member['memberType']?.toString() == 'device' || mode == 'capability') {
      return '设备${_deviceName(ownerId)}-能力';
    }
    final roleName = _roleName(memberId);
    if (ownerType == 'device') {
      return '设备${_deviceName(ownerId)}-角色-$roleName';
    }
    return '统筹中心-角色-$roleName';
  }

  String _memberSubtitle(Map<String, dynamic> member) {
    final mode = member['invocationMode']?.toString() ?? 'role';
    final parts = <String>[
      mode == 'capability' ? '能力调用' : '角色调用',
      member['responderMode']?.toString() ?? '',
      if ((member['roleRevision'] as num?)?.toInt() != null &&
          (member['roleRevision'] as num).toInt() > 0)
        '版本 ${member['roleRevision']}',
    ];
    return parts.where((part) => part.isNotEmpty).join(' · ');
  }

  Future<void> _addContext() async {
    if (!_canWrite) return;
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
    if (!_canWrite) return;
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

  Future<void> _dispatchTeam() async {
    if (!_canWrite) return;
    if (_detail == null) return;
    final message = await _askText('调度设备角色', '要交给设备角色的消息');
    if (message == null || message.trim().isEmpty) return;
    setState(() => _acting = true);
    try {
      final result = await _post(
        '/api/device-mesh/v1/business/teams/${_team['id']}/dispatch',
        data: {'message': message.trim()},
      );
      if (!mounted) return;
      final targets = result is Map && result['targets'] is List
          ? (result['targets'] as List)
          : const <dynamic>[];
      final lines = targets
          .whereType<Map>()
          .map((target) {
            final owner = (target['ownerId'] ?? '统筹中心').toString();
            final status = (target['status'] ?? '').toString();
            final text = (target['text'] ?? '').toString();
            final failure = (target['message'] ?? '').toString();
            return [
              '$owner · ${target['roleId'] ?? ''} · $status',
              if (failure.isNotEmpty) failure,
              if (text.isNotEmpty) text,
            ].join('\n');
          })
          .toList(growable: false);
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('调度结果'),
          content: SelectableText(
            lines.isEmpty ? '没有可调度的目标' : lines.join('\n\n'),
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
        name: 'characters.teams.dispatch',
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
    final accessAsync = ref.watch(sharedCoreDataAccessProvider);
    final access =
        accessAsync.valueOrNull ??
        SharedCoreDataAccess(
          read: true,
          write: false,
          reason: accessAsync.hasError
              ? '无法确认统筹中心数据权限：${accessAsync.error}'
              : '正在确认统筹中心数据权限',
          coreId: '',
        );
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '团队',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            tooltip: '新建团队',
            onPressed: _startCreateTeam,
          ),
        ],
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                if (access.readOnly) SharedCoreReadOnlyBanner(access: access),
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
                    actionText: access.write ? '新建团队' : null,
                    onAction: access.write ? _createTeam : null,
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
                  onPressed: () => _runIfAvailable(_editTeam),
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
                  onPressed: () => _runIfAvailable(_addMember),
                  child: const Text('添加成员'),
                ),
                OutlinedButton(
                  onPressed: () => _runIfAvailable(_addContext),
                  child: const Text('添加共享上下文'),
                ),
                OutlinedButton(
                  onPressed: () => _runIfAvailable(_testRoute),
                  child: const Text('测试路由'),
                ),
                OutlinedButton(
                  onPressed: () => _runIfAvailable(_dispatchTeam),
                  child: const Text('调度设备角色'),
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
                        title: Text(_memberLabel(member)),
                        subtitle: Text(_memberSubtitle(member)),
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

  bool get _canWrite {
    final state = ref.read(sharedCoreDataAccessProvider);
    return !state.isLoading &&
        !state.hasError &&
        state.valueOrNull?.write == true;
  }

  void _startCreateTeam() {
    if (_acting) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('正在处理，请稍后重试。')));
      return;
    }
    _createTeam();
  }

  void _runIfAvailable(Future<void> Function() action) {
    if (_acting) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('正在处理，请稍后重试。')));
    } else if (!_canWrite) {
      showSharedCoreAccessReason(
        context,
        ref.read(sharedCoreDataAccessProvider),
      );
    } else {
      action();
    }
  }
}

class _TeamMemberSelection {
  const _TeamMemberSelection({
    required this.memberType,
    required this.memberId,
    required this.ownerType,
    required this.ownerId,
    required this.invocationMode,
    required this.roleRevision,
    required this.role,
  });

  final String memberType;
  final String memberId;
  final String ownerType;
  final String ownerId;
  final String invocationMode;
  final int roleRevision;
  final String role;
}

enum _TeamMemberSource { local, center, device }

class _AddTeamMemberDialog extends ConsumerStatefulWidget {
  const _AddTeamMemberDialog({
    required this.localRoles,
    required this.centerRoles,
    required this.devices,
    required this.roleDevices,
    required this.existingKeys,
    required this.loadDeviceRoles,
  });

  final List<Map<String, dynamic>> localRoles;
  final List<Map<String, dynamic>> centerRoles;
  final List<Map<String, dynamic>> devices;
  final List<Map<String, dynamic>> roleDevices;
  final Set<String> existingKeys;
  final Future<List<Map<String, dynamic>>> Function(String deviceId)
  loadDeviceRoles;

  @override
  ConsumerState<_AddTeamMemberDialog> createState() =>
      _AddTeamMemberDialogState();
}

class _AddTeamMemberDialogState extends ConsumerState<_AddTeamMemberDialog> {
  _TeamMemberSource _source = _TeamMemberSource.local;
  String _mode = 'role';
  String _deviceId = '';
  String _roleId = '';
  String _role = 'member';
  List<Map<String, dynamic>> _deviceRoles = const [];
  bool _loadingRoles = false;
  String? _rolesError;

  @override
  void initState() {
    super.initState();
    final candidates = widget.roleDevices.isNotEmpty
        ? widget.roleDevices
        : widget.devices;
    if (candidates.isNotEmpty) {
      _deviceId = candidates.first['deviceId']?.toString() ?? '';
    }
    _roleId = _firstAvailableRoleId();
  }

  List<Map<String, dynamic>> get _selectableDevices =>
      _mode == 'capability' ? widget.devices : widget.roleDevices;

  bool get _deviceSupportsRole => widget.roleDevices.any(
    (device) => device['deviceId']?.toString() == _deviceId,
  );

  bool get _centerAvailable => roleManagementCenterAvailable(
    ref.watch(deviceCoordinationProvider).valueOrNull,
  );

  List<Map<String, dynamic>> get _visibleRoles {
    switch (_source) {
      case _TeamMemberSource.local:
        return widget.localRoles;
      case _TeamMemberSource.center:
        return widget.centerRoles;
      case _TeamMemberSource.device:
        return _deviceRoles;
    }
  }

  String _roleKey(String memberId) {
    if (_source == _TeamMemberSource.device) {
      return 'agent|device|$_deviceId|$memberId';
    }
    return 'agent|center||$memberId';
  }

  bool _taken(String memberId) => widget.existingKeys.contains(_roleKey(memberId));

  String _firstAvailableRoleId() {
    final roles = _source == _TeamMemberSource.device ? _deviceRoles : _visibleRoles;
    return roles
            .map((role) => role['id']?.toString() ?? '')
            .where((id) => id.isNotEmpty && !_taken(id))
            .firstOrNull ??
        '';
  }

  bool get _needsRole => _mode == 'role';

  bool get _canSubmit {
    if (_source == _TeamMemberSource.device && _mode == 'capability') {
      return _deviceId.isNotEmpty &&
          !widget.existingKeys.contains('device|device|$_deviceId|$_deviceId');
    }
    return _roleId.isNotEmpty;
  }

  Future<void> _selectSource(_TeamMemberSource next) async {
    if (next == _source) return;
    setState(() {
      _source = next;
      if (next != _TeamMemberSource.device) _mode = 'role';
      _roleId = '';
      _rolesError = null;
    });
    if (next == _TeamMemberSource.device) {
      await _loadDeviceRoles();
      return;
    }
    setState(() => _roleId = _firstAvailableRoleId());
  }

  Future<void> _selectDevice(String deviceId) async {
    setState(() {
      _deviceId = deviceId;
      _roleId = '';
      _deviceRoles = const [];
      _rolesError = null;
    });
    await _loadDeviceRoles();
  }

  Future<void> _loadDeviceRoles() async {
    if (_deviceId.isEmpty) return;
    setState(() {
      _loadingRoles = true;
      _rolesError = null;
    });
    try {
      final roles = await widget.loadDeviceRoles(_deviceId);
      if (!mounted) return;
      setState(() {
        _deviceRoles = roles;
        if (_mode == 'role') _roleId = _firstAvailableRoleId();
      });
    } catch (error, stackTrace) {
      developer.log(
        '[original-error] $error',
        name: 'characters.teams.deviceRoles',
        error: error,
        stackTrace: stackTrace,
      );
      if (!mounted) return;
      setState(() => _rolesError = error.toString());
    } finally {
      if (mounted) setState(() => _loadingRoles = false);
    }
  }

  _TeamMemberSelection _buildSelection() {
    if (_source == _TeamMemberSource.device) {
      if (_mode == 'capability') {
        return _TeamMemberSelection(
          memberType: 'device',
          memberId: _deviceId,
          ownerType: 'device',
          ownerId: _deviceId,
          invocationMode: 'capability',
          roleRevision: 0,
          role: _role,
        );
      }
      final revision = _deviceRoles
          .where((role) => role['id']?.toString() == _roleId)
          .map((role) => (role['revision'] as num?)?.toInt() ?? 0)
          .firstOrNull;
      return _TeamMemberSelection(
        memberType: 'agent',
        memberId: _roleId,
        ownerType: 'device',
        ownerId: _deviceId,
        invocationMode: 'role',
        roleRevision: revision ?? 0,
        role: _role,
      );
    }
    return _TeamMemberSelection(
      memberType: 'agent',
      memberId: _roleId,
      ownerType: 'center',
      ownerId: '',
      invocationMode: 'role',
      roleRevision: 0,
      role: _role,
    );
  }

  @override
  Widget build(BuildContext context) {
    final sources = <_TeamMemberSource>[
      _TeamMemberSource.local,
      if (_centerAvailable) _TeamMemberSource.center,
      if (widget.devices.isNotEmpty) _TeamMemberSource.device,
    ];
    final roles = _visibleRoles
        .where((role) => !_taken(role['id']?.toString() ?? ''))
        .toList(growable: false);
    return AlertDialog(
      title: const Text('添加团队成员'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('成员来源', style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.xs),
            SegmentedButton<_TeamMemberSource>(
              segments: [
                for (final source in sources)
                  ButtonSegment<_TeamMemberSource>(
                    value: source,
                    label: Text(
                      source == _TeamMemberSource.local
                          ? '本机'
                          : source == _TeamMemberSource.center
                          ? '统筹中心'
                          : '其他设备',
                    ),
                  ),
              ],
              selected: {_source},
              onSelectionChanged: (selection) {
                if (selection.isNotEmpty) _selectSource(selection.first);
              },
            ),
            if (!_centerAvailable)
              Padding(
                padding: const EdgeInsets.only(top: 6),
                child: Text(
                  '未获得统筹中心管理员授权，只能邀请本机角色',
                  style: AppTypography.caption(
                    context,
                  ).copyWith(color: context.textTertiary),
                ),
              ),
            if (_source == _TeamMemberSource.device) ...[
              SizedBox(height: AppSpacing.sm),
              DropdownButtonFormField<String>(
                initialValue: _deviceId.isEmpty ? null : _deviceId,
                decoration: const InputDecoration(labelText: '设备'),
                items: _selectableDevices
                    .map(
                      (device) => DropdownMenuItem(
                        value: device['deviceId'].toString(),
                        child: Text(
                          (device['reportedName'] ??
                                  device['label'] ??
                                  device['deviceId'])
                              .toString(),
                        ),
                      ),
                    )
                    .toList(growable: false),
                onChanged: (next) {
                  if (next != null) _selectDevice(next);
                },
              ),
              SizedBox(height: AppSpacing.sm),
              Text('调用方式', style: AppTypography.caption(context)),
              SizedBox(height: AppSpacing.xs),
              SegmentedButton<String>(
                segments: const [
                  ButtonSegment(value: 'role', label: Text('指定角色')),
                  ButtonSegment(value: 'capability', label: Text('设备能力')),
                ],
                selected: {_mode},
                onSelectionChanged: (selection) async {
                  if (selection.isEmpty) return;
                  final next = selection.first;
                  if (next == _mode) return;
                  setState(() {
                    _mode = next;
                    if (next == 'role' && !_deviceSupportsRole) {
                      _deviceId = widget.roleDevices.isEmpty
                          ? ''
                          : widget.roleDevices.first['deviceId'].toString();
                      _deviceRoles = const [];
                      _roleId = '';
                    }
                  });
                  if (next != 'role') return;
                  if (_deviceRoles.isNotEmpty) {
                    setState(() => _roleId = _firstAvailableRoleId());
                    return;
                  }
                  await _loadDeviceRoles();
                },
              ),
              if (_mode == 'capability')
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(
                    '不绑定角色，直接调用该设备的 Agent 能力',
                    style: AppTypography.caption(context),
                  ),
                ),
              if (_mode == 'role' && widget.roleDevices.isEmpty)
                Padding(
                  padding: const EdgeInsets.only(top: 6),
                  child: Text(
                    '其他设备都处于统筹模式，没有独立角色目录；请改用设备能力，或在统筹中心选择角色',
                    style: AppTypography.caption(
                      context,
                    ).copyWith(color: context.textTertiary),
                  ),
                ),
            ],
            if (_needsRole) ...[
              SizedBox(height: AppSpacing.sm),
              if (_loadingRoles)
                const Padding(
                  padding: EdgeInsets.symmetric(vertical: 8),
                  child: LinearProgressIndicator(),
                )
              else
                DropdownButtonFormField<String>(
                  initialValue: _roleId.isEmpty ? null : _roleId,
                  decoration: InputDecoration(
                    labelText: '角色',
                    errorText: _rolesError,
                  ),
                  items: roles
                      .map(
                        (role) => DropdownMenuItem(
                          value: role['id'].toString(),
                          child: Text(role['name']?.toString() ?? ''),
                        ),
                      )
                      .toList(growable: false),
                  onChanged: (next) {
                    if (next != null) setState(() => _roleId = next);
                  },
                ),
            ],
            SizedBox(height: AppSpacing.sm),
            DropdownButtonFormField<String>(
              initialValue: _role,
              decoration: const InputDecoration(labelText: '团队职责'),
              items: const [
                DropdownMenuItem(value: 'member', child: Text('成员')),
                DropdownMenuItem(value: 'lead', child: Text('负责人')),
              ],
              onChanged: (next) {
                if (next != null) setState(() => _role = next);
              },
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('取消'),
        ),
        FilledButton(
          onPressed: _canSubmit
              ? () => Navigator.pop(context, _buildSelection())
              : null,
          child: const Text('加入'),
        ),
      ],
    );
  }
}
