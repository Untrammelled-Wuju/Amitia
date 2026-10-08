import 'dart:convert';
import 'dart:math';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/runtime/backend/mobile_backend_providers.dart';
import '../../../../core/runtime/backend/mobile_deployment_mode.dart';
import '../../../../core/models/task_pause_controls.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class KernelTasksPage extends ConsumerStatefulWidget {
  const KernelTasksPage({super.key});

  @override
  ConsumerState<KernelTasksPage> createState() => _KernelTasksPageState();
}

class _KernelTasksPageState extends ConsumerState<KernelTasksPage> {
  bool _loading = true;
  bool _working = false;
  String? _error;
  List<Map<String, dynamic>> _definitions = const [];
  List<Map<String, dynamic>> _runs = const [];
  static const _approvalStatusLabels = {
    'pending': '待审批',
    'approved': '已批准',
    'denied': '已拒绝',
    'claimed': '执行中',
    'revoked': '已撤销',
  };

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
      final service = ref.read(extensionTaskServiceProvider);
      final values = await Future.wait<List<Map<String, dynamic>>>([
        service.listDefinitions(),
        service.listRuns(limit: 200),
      ]);
      if (!mounted) return;
      setState(() {
        _definitions = values[0];
        _runs = values[1];
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

  Future<void> _runAction(
    String successMessage,
    Future<void> Function() action,
  ) async {
    if (_working) return;
    setState(() => _working = true);
    try {
      await action();
      await _load();
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text(successMessage)));
    } catch (e) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('操作失败：$e')));
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  Future<void> _showSourceTaskApprovals() async {
    if (_working) return;
    setState(() => _working = true);
    try {
      final service = ref.read(sourceTaskApprovalServiceProvider);
      final approvals = await service.listSourceTaskApprovals();
      if (!mounted) return;
      final selected = await showDialog<Map<String, dynamic>>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('本机资源审批'),
          content: SizedBox(
            width: 480,
            child: SingleChildScrollView(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const Text('批准仅用于指定请求的一次执行，最长 30 分钟。拒绝后需重新发起请求。'),
                  if (approvals.isEmpty)
                    const Padding(
                      padding: EdgeInsets.all(16),
                      child: Text('没有当前绑定下有效的设备任务审批'),
                    ),
                  for (final approval in approvals) ...[
                    const Divider(),
                    Text(
                      '任务：${approval['binding']['target']['taskId']} · 安装代次 ${approval['binding']['target']['installedGeneration']}',
                    ),
                    Text(
                      '插件：${approval['binding']['target']['extensionId']} / ${approval['binding']['target']['moduleId']}',
                    ),
                    Text(
                      'Core：${approval['binding']['executionScope']['coreId']}',
                    ),
                    Text(
                      '发起设备：${approval['binding']['executionScope']['initiatorDeviceId']} · 角色 ${approval['binding']['executionScope']['roleId']}',
                    ),
                    Text(
                      '执行代次：${approval['binding']['taskGeneration']} · 数据保存至 ${approval['binding']['executionScope']['resourceOwnerId']}',
                    ),
                    Text(
                      '资源权限：${(approval['permissions'] as List).map((permission) => permission['permissionId']).join('、')}',
                    ),
                    Text('请求内容指纹：${approval['binding']['inputHash']}'),
                    Text(
                      '审批状态：${_approvalStatusLabels[approval['status']] ?? approval['status']} · 有效至 ${approval['expiresAt']}',
                    ),
                    if (approval['status'] == 'approved' ||
                        approval['status'] == 'claimed')
                      TextButton(
                        onPressed: () => Navigator.pop(dialogContext, {
                          'approval': approval,
                          'revoke': true,
                        }),
                        child: const Text('撤销单次授权'),
                      ),
                    if (approval['status'] == 'pending')
                      Wrap(
                        spacing: 12,
                        children: [
                          TextButton(
                            onPressed: () => Navigator.pop(dialogContext, {
                              'approval': approval,
                              'approved': true,
                            }),
                            child: const Text('批准本次执行'),
                          ),
                          TextButton(
                            onPressed: () => Navigator.pop(dialogContext, {
                              'approval': approval,
                              'approved': false,
                            }),
                            child: const Text('拒绝'),
                          ),
                        ],
                      ),
                  ],
                ],
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
      if (selected == null || !mounted) return;
      if (selected['revoke'] == true) {
        await service.revokeSourceTaskApproval(
          selected['approval'] as Map<String, dynamic>,
        );
      } else {
        await service.decideSourceTaskApproval(
          selected['approval'] as Map<String, dynamic>,
          selected['approved'] == true,
        );
      }
      if (mounted)
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              selected['revoke'] == true
                  ? '已撤销单次授权，正在执行的任务将被中断'
                  : selected['approved'] == true
                  ? '已批准，请在发起设备使用原请求重试'
                  : '已拒绝本次执行',
            ),
          ),
        );
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('审批失败：$error')));
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  String _definitionId(Map<String, dynamic> definition) =>
      (definition['taskId'] ?? definition['id'] ?? '').toString();

  String _runId(Map<String, dynamic> run) =>
      (run['taskRunId'] ?? run['id'] ?? '').toString();

  String _status(Map<String, dynamic> run) =>
      (run['status'] ?? 'unknown').toString();

  String _statusLabel(String status) {
    switch (status.toLowerCase()) {
      case 'queued':
        return '排队中';
      case 'leased':
      case 'running':
      case 'checkpointing':
      case 'resuming':
        return '运行中';
      case 'paused':
      case 'pausing':
      case 'pause_requested':
        return '已暂停';
      case 'succeeded':
        return '已完成';
      case 'failed':
      case 'timed_out':
        return '失败';
      case 'cancelled':
      case 'cancelling':
        return '已取消';
      case 'recovery_required':
        return '需要恢复';
      case 'manual_intervention':
        return '需要人工处理';
      default:
        return status.isEmpty ? '未知' : status;
    }
  }

  BadgeType _badgeType(String status) {
    switch (status.toLowerCase()) {
      case 'running':
      case 'leased':
      case 'queued':
      case 'checkpointing':
      case 'resuming':
        return BadgeType.accent;
      case 'succeeded':
        return BadgeType.success;
      case 'failed':
      case 'timed_out':
        return BadgeType.error;
      case 'paused':
      case 'pausing':
      case 'pause_requested':
      case 'recovery_required':
      case 'manual_intervention':
        return BadgeType.warning;
      default:
        return BadgeType.neutral;
    }
  }

  Future<Map<String, dynamic>?> _chooseDeviceTaskItem(
    String title,
    List<Map<String, dynamic>> items,
    String Function(Map<String, dynamic>) label,
  ) {
    return showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: SizedBox(
          width: 560,
          height: 360,
          child: ListView.builder(
            itemCount: items.length,
            itemBuilder: (_, index) => ListTile(
              title: Text(label(items[index])),
              onTap: () => Navigator.pop(dialogContext, items[index]),
            ),
          ),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
        ],
      ),
    );
  }

  Future<void> _enqueueDeviceCatalog() async {
    try {
      final devices =
          (await ref.read(extensionServiceProvider).workflowDevices())
              .where(
                (device) =>
                    device['online'] == true &&
                    (device['deviceId'] ?? '').toString().isNotEmpty,
              )
              .toList();
      if (!mounted) return;
      if (devices.isEmpty) throw StateError('当前没有可调用的在线设备');
      final device = await _chooseDeviceTaskItem(
        '选择执行设备',
        devices,
        (item) =>
            '${item['label'] ?? item['platform'] ?? '设备'} · ${item['deviceId']}',
      );
      if (device == null || !mounted) return;
      final service = ref.read(extensionTaskServiceProvider);
      final options = await service.prepareOwnedTask(
        device['deviceId'].toString(),
      );
      if (!mounted) return;
      final roles = (options['roles'] as List)
          .whereType<Map>()
          .map((role) => Map<String, dynamic>.from(role))
          .toList();
      if (roles.isEmpty) throw StateError('目标数据来源没有可用角色，拒绝调用');
      final role = roles.length == 1
          ? roles.single
          : await _chooseDeviceTaskItem(
              '选择执行角色',
              roles,
              (item) => (item['name'] ?? item['id']).toString(),
            );
      if (role == null || !mounted) return;
      final entries = <Map<String, dynamic>>[];
      String cursor = '';
      String? revision;
      final seen = <String>{};
      do {
        final page = await service.listOwnedDeviceTaskCatalog(
          options,
          role['id'].toString(),
          cursor: cursor,
        );
        if (!mounted) return;
        if (revision != null && revision != page['revision'])
          throw StateError('设备任务目录已更新，请重新选择');
        revision = page['revision'];
        for (final raw in page['entries'] as List) {
          final entry = Map<String, dynamic>.from(raw);
          if (!seen.add(entry['reference']['catalogId']))
            throw StateError('设备任务目录出现重复条目');
          entries.add(entry);
        }
        if (entries.length > 256) throw StateError('设备任务目录超过上限');
        final next = (page['nextCursor'] ?? '').toString();
        if (next.isNotEmpty &&
            (next == cursor || (page['entries'] as List).isEmpty))
          throw StateError('设备任务目录分页无效');
        cursor = next;
      } while (cursor.isNotEmpty);
      if (entries.isEmpty) throw StateError('目标设备没有可用的已安装任务');
      final selected = await _chooseDeviceTaskItem(
        '选择设备任务',
        entries,
        (entry) =>
            '${entry['reference']['sourceTaskId']} · ${entry['definition']['version'] ?? '当前版本'}',
      );
      if (selected == null || !mounted) return;
      await _enqueue({
        ...Map<String, dynamic>.from(selected['definition']),
        'taskId': selected['reference']['catalogId'],
        'executionPlacement': 'device',
        'taskCatalogReference': selected['reference'],
        'catalogRoleId': role['id'],
        'catalogExecutionScope': {
          ...Map<String, dynamic>.from(options['executionScope']),
          'roleId': role['id'],
          'roleRevision': role['revision'],
        },
      });
    } catch (error) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('设备任务选择失败：$error')));
    }
  }

  Future<void> _enqueue(Map<String, dynamic> definition) async {
    final definitionId = _definitionId(definition);
    if (definitionId.isEmpty) return;
    final controller = TextEditingController(text: '{}');
    final priorityController = TextEditingController(text: '0');
    final needsDevice =
        (definition['executionPlacement'] ?? '').toString() == 'device';
    final owned =
        definition['taskCatalogReference'] is Map ||
        ref.read(mobileDeploymentConfigProvider).mode ==
            MobileDeploymentMode.cloud;
    final declaredPermissions = definition['permissionRequirements'];
    final permissionNames = definition['permissionRequirementStrings'];
    final needsResourcePermission =
        declaredPermissions is List && declaredPermissions.isNotEmpty ||
        permissionNames is List && permissionNames.isNotEmpty;
    Map<String, dynamic>? roleOptions;
    String selectedRole = '';
    String? taskError;
    bool roleLoading = false;
    bool submitting = false;
    int roleGeneration = 0;
    String submissionKey = '';
    String requestId = '';
    Map<String, dynamic>? confirmedTask;
    List<Map<String, dynamic>> onlineDevices = const [];
    String selectedDeviceId = '';
    final catalogReference = definition['taskCatalogReference'];
    if (needsDevice) {
      try {
        final devices = await ref
            .read(extensionServiceProvider)
            .workflowDevices();
        onlineDevices = devices
            .where(
              (item) =>
                  item['online'] == true &&
                  (item['deviceId'] ?? '').toString().trim().isNotEmpty,
            )
            .toList(growable: false);
        if (onlineDevices.length == 1)
          selectedDeviceId = (onlineDevices.first['deviceId'] ?? '').toString();
        if (catalogReference is Map) {
          final target = catalogReference['deviceId'].toString();
          if (!onlineDevices.any((device) => device['deviceId'] == target))
            throw StateError('来源设备已离线，请重新选择设备任务');
          selectedDeviceId = target;
        }
      } catch (e) {
        if (mounted)
          ScaffoldMessenger.of(
            context,
          ).showSnackBar(SnackBar(content: Text('设备列表加载失败：$e')));
        if (catalogReference is Map) {
          controller.dispose();
          priorityController.dispose();
          return;
        }
      }
    }
    if (!mounted) {
      controller.dispose();
      priorityController.dispose();
      return;
    }
    if (owned && selectedDeviceId.isNotEmpty) {
      try {
        final prepared = await ref
            .read(extensionTaskServiceProvider)
            .prepareOwnedTask(selectedDeviceId);
        if (catalogReference is Map) {
          final expected = definition['catalogExecutionScope'] as Map;
          final actual = prepared['executionScope'] as Map;
          if (expected.entries.any(
                (entry) =>
                    ![
                      'requestId',
                      'turnId',
                      'executionId',
                      'roleId',
                      'roleRevision',
                    ].contains(entry.key) &&
                    actual[entry.key] != entry.value,
              ) ||
              !(prepared['roles'] as List).whereType<Map>().any(
                (role) =>
                    role['id'] == expected['roleId'] &&
                    role['revision'] == expected['roleRevision'],
              ))
            throw StateError('来源任务选择后服务、角色或统筹状态已变化，请重新选择');
        }
        roleOptions = prepared;
        final roles = (prepared['roles'] as List).whereType<Map>().toList();
        if (catalogReference is Map) {
          final roleId = definition['catalogRoleId'];
          if (!roles.any((role) => role['id'] == roleId))
            throw StateError('所选设备任务角色已失效，请重新选择');
          selectedRole = roleId.toString();
        } else if (roles.length == 1) {
          selectedRole = roles.single['id'].toString();
        } else if (roles.any(
          (role) => role['id'] == prepared['selectedRole'],
        )) {
          selectedRole = prepared['selectedRole'].toString();
        }
        if (roles.isEmpty) taskError = '目标数据来源没有可用角色，拒绝调用';
      } catch (error) {
        taskError = error.toString();
      }
      if (!mounted) {
        controller.dispose();
        priorityController.dispose();
        return;
      }
    }
    Future<void> refreshRoles(
      StateSetter update,
      BuildContext dialogContext,
    ) async {
      final ticket = ++roleGeneration;
      final target = selectedDeviceId;
      update(() {
        roleOptions = null;
        selectedRole = '';
        taskError = null;
        roleLoading = target.isNotEmpty;
      });
      if (target.isEmpty) return;
      try {
        final result = await ref
            .read(extensionTaskServiceProvider)
            .prepareOwnedTask(target);
        if (!dialogContext.mounted || ticket != roleGeneration) return;
        final roles = (result['roles'] as List).whereType<Map>().toList();
        update(() {
          roleOptions = result;
          if (roles.length == 1)
            selectedRole = roles.single['id'].toString();
          else if (roles.any((role) => role['id'] == result['selectedRole']))
            selectedRole = result['selectedRole'].toString();
          if (roles.isEmpty) taskError = '目标数据来源没有可用角色，拒绝调用';
        });
      } catch (error) {
        if (dialogContext.mounted && ticket == roleGeneration)
          update(() => taskError = error.toString());
      } finally {
        if (dialogContext.mounted && ticket == roleGeneration)
          update(() => roleLoading = false);
      }
    }

    final confirmed = await showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (dialogContext) => StatefulBuilder(
        builder: (dialogContext, setDialogState) => PopScope(
          canPop: !submitting,
          child: AlertDialog(
            title: Text('手动入队 · $definitionId'),
            content: SizedBox(
              width: 560,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  if (needsDevice) ...[
                    DropdownButtonFormField<String>(
                      value: selectedDeviceId.isEmpty ? null : selectedDeviceId,
                      decoration: const InputDecoration(
                        labelText: '执行设备',
                        border: OutlineInputBorder(),
                      ),
                      items: onlineDevices
                          .map((device) {
                            final id = (device['deviceId'] ?? '').toString();
                            final label =
                                (device['label'] ??
                                        device['platform'] ??
                                        'device')
                                    .toString();
                            return DropdownMenuItem(
                              value: id,
                              child: Text(
                                '$label · $id',
                                overflow: TextOverflow.ellipsis,
                              ),
                            );
                          })
                          .toList(growable: false),
                      onChanged: submitting || catalogReference is Map
                          ? null
                          : (value) {
                              setDialogState(
                                () => selectedDeviceId = value ?? '',
                              );
                              if (owned)
                                refreshRoles(setDialogState, dialogContext);
                            },
                    ),
                    if (onlineDevices.isEmpty) ...[
                      const SizedBox(height: 8),
                      const Align(
                        alignment: Alignment.centerLeft,
                        child: Text('当前没有在线设备，设备任务无法入队。'),
                      ),
                    ],
                    const SizedBox(height: 10),
                  ],
                  if (owned) ...[
                    DropdownButtonFormField<String>(
                      value: selectedRole.isEmpty ? null : selectedRole,
                      decoration: const InputDecoration(
                        labelText: '执行角色',
                        border: OutlineInputBorder(),
                      ),
                      items: ((roleOptions?['roles'] as List?) ?? const [])
                          .whereType<Map>()
                          .map(
                            (role) => DropdownMenuItem(
                              value: role['id'].toString(),
                              child: Text(
                                (role['name'] ?? role['id']).toString(),
                              ),
                            ),
                          )
                          .toList(),
                      onChanged: submitting || roleLoading
                          ? null
                          : (value) => setDialogState(
                              () => selectedRole = value ?? '',
                            ),
                    ),
                    if (roleOptions != null)
                      Text(
                        'Core：${roleOptions!['executionScope']['coreId']} · 新数据所有者：${roleOptions!['roleOwnerId']}',
                      ),
                    TextButton(
                      onPressed:
                          submitting || roleLoading || selectedDeviceId.isEmpty
                          ? null
                          : () => refreshRoles(setDialogState, dialogContext),
                      child: Text(roleLoading ? '正在加载角色' : '刷新角色与服务'),
                    ),
                    if (taskError != null)
                      Text(
                        taskError!,
                        style: const TextStyle(color: Colors.red),
                      ),
                    const SizedBox(height: 10),
                  ],
                  if (owned && needsResourcePermission) ...[
                    const Text('本机资源权限须由目标设备批准，提交时会再次确认。'),
                    const SizedBox(height: 10),
                  ],
                  TextField(
                    controller: controller,
                    enabled: !submitting,
                    minLines: 7,
                    maxLines: 14,
                    style: const TextStyle(
                      fontFamily: 'monospace',
                      fontSize: 12,
                    ),
                    decoration: const InputDecoration(
                      labelText: 'Input JSON',
                      border: OutlineInputBorder(),
                    ),
                  ),
                  const SizedBox(height: 10),
                  if (!owned)
                    TextField(
                      controller: priorityController,
                      keyboardType: TextInputType.number,
                      decoration: const InputDecoration(
                        labelText: 'Priority',
                        border: OutlineInputBorder(),
                      ),
                    ),
                ],
              ),
            ),
            actions: [
              TextButton(
                onPressed: submitting
                    ? null
                    : () {
                        roleGeneration++;
                        Navigator.pop(dialogContext, false);
                      },
                child: const Text('取消'),
              ),
              FilledButton(
                onPressed:
                    submitting ||
                        (needsDevice && selectedDeviceId.isEmpty) ||
                        (owned &&
                            (!needsDevice ||
                                roleLoading ||
                                roleOptions == null ||
                                selectedRole.isEmpty))
                    ? null
                    : () async {
                        if (!owned) {
                          Navigator.pop(dialogContext, true);
                          return;
                        }
                        setDialogState(() {
                          submitting = true;
                          taskError = null;
                        });
                        try {
                          final decoded = jsonDecode(controller.text);
                          if (decoded is! Map)
                            throw const FormatException(
                              'Input 必须是 JSON Object',
                            );
                          final options = roleOptions!;
                          final expected = options['executionScope'] as Map;
                          final role = (options['roles'] as List)
                              .whereType<Map>()
                              .firstWhere((item) => item['id'] == selectedRole);
                          final service = ref.read(
                            extensionTaskServiceProvider,
                          );
                          final current = await service.prepareOwnedTask(
                            selectedDeviceId,
                          );
                          final actual = current['executionScope'] as Map;
                          final fields = [
                            'coreId',
                            'modeRevision',
                            'providerEpoch',
                            'permissionRevision',
                            'targetPermissionRevision',
                            'targetProviderEpoch',
                            'resourceOwnerId',
                          ];
                          if (fields.any(
                                (field) => actual[field] != expected[field],
                              ) ||
                              !(current['roles'] as List).whereType<Map>().any(
                                (item) =>
                                    item['id'] == selectedRole &&
                                    item['revision'] == role['revision'],
                              )) {
                            throw StateError(
                              actual['coreId'] != expected['coreId']
                                  ? '云端服务提供者已从「${expected['coreId']}」切换为「${actual['coreId']}」，请刷新后重新提交。'
                                  : '目标角色、权限或统筹状态已变化，请刷新后重新提交',
                            );
                          }
                          final body = <String, dynamic>{
                            'taskDefinitionId': definitionId,
                            if (catalogReference is Map)
                              'taskCatalogReference': Map<String, dynamic>.from(
                                catalogReference,
                              ),
                            'targetDeviceId': selectedDeviceId,
                            'characterId': selectedRole,
                            'input': Map<String, dynamic>.from(decoded),
                            'expectedCoreId': expected['coreId'],
                            'expectedModeRevision': expected['modeRevision'],
                            'expectedRoleRevision': role['revision'],
                            'expectedExecutionScope': {
                              ...Map<String, dynamic>.from(expected),
                              'requestId': '',
                              'turnId': '',
                              'executionId': '',
                              'roleId': selectedRole,
                              'roleRevision': role['revision'],
                            },
                          };
                          final key = jsonEncode(body);
                          if (submissionKey != key) {
                            submissionKey = key;
                            final random = Random.secure();
                            requestId =
                                'task-${List.generate(16, (_) => random.nextInt(256).toRadixString(16).padLeft(2, '0')).join()}';
                          }
                          confirmedTask = await service.enqueueOwnedTask({
                            ...body,
                            'requestId': requestId,
                          });
                          if (dialogContext.mounted) {
                            roleGeneration++;
                            Navigator.pop(dialogContext, true);
                          }
                        } catch (error) {
                          if (dialogContext.mounted)
                            setDialogState(() => taskError = error.toString());
                        } finally {
                          if (dialogContext.mounted)
                            setDialogState(() => submitting = false);
                        }
                      },
                child: Text(submitting ? '正在提交' : '入队'),
              ),
            ],
          ),
        ),
      ),
    );
    roleGeneration++;
    if (owned) {
      controller.dispose();
      priorityController.dispose();
      if (confirmed == true && confirmedTask != null && mounted) {
        final task = confirmedTask!;
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(
              '${task['queued'] == true ? '任务已入队' : _statusLabel(task['status'].toString())}：${task['taskRunId']}',
            ),
          ),
        );
        await _load();
      }
      return;
    }
    if (confirmed != true) {
      controller.dispose();
      priorityController.dispose();
      return;
    }
    try {
      final decoded = jsonDecode(controller.text);
      if (decoded is! Map) throw const FormatException('Input 必须是 JSON Object');
      final priority = int.tryParse(priorityController.text.trim()) ?? 0;
      await _runAction('任务已真实入队', () async {
        await ref
            .read(extensionTaskServiceProvider)
            .enqueue(
              taskDefinitionId: definitionId,
              extensionId: (definition['extensionId'] ?? '').toString(),
              moduleId: (definition['moduleId'] ?? '').toString(),
              input: Map<String, dynamic>.from(decoded),
              priority: priority,
              source: 'mobile_kernel_tasks',
              deviceId: needsDevice ? selectedDeviceId : null,
            );
      });
    } catch (e) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('入队失败：$e')));
    } finally {
      controller.dispose();
      priorityController.dispose();
    }
  }

  Future<Map<String, dynamic>> _loadRuntimeDetail(Map<String, dynamic> run) {
    return ref
        .read(extensionTaskServiceProvider)
        .runtimeDetail(
          _runId(run),
          expectedRun: run['executionScope'] is Map ? run : null,
        );
  }

  Future<void> _showRunDetail(Map<String, dynamic> run) async {
    final id = _runId(run);
    if (id.isEmpty) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) => _KernelTaskRuntimeSheet(
        taskRunId: id,
        loader: () => _loadRuntimeDetail(run),
        onRetry: () async {
          Navigator.pop(sheetContext);
          await _runAction('任务已重新入队', () async {
            await ref
                .read(extensionTaskServiceProvider)
                .retry(
                  id,
                  expectedRun: run['executionScope'] is Map ? run : null,
                );
          });
        },
        onRecover: () async {
          Navigator.pop(sheetContext);
          await _runAction('任务恢复操作已提交', () async {
            await ref
                .read(extensionTaskServiceProvider)
                .recover(
                  id,
                  expectedRun: run['executionScope'] is Map ? run : null,
                );
          });
        },
      ),
    );
  }

  Future<void> _pause(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已暂停', () async {
      await ref
          .read(extensionTaskServiceProvider)
          .pause(
            id,
            generation: (run['generation'] as num?)?.toInt() ?? 0,
            reason: 'mobile_user',
            expectedRun: run['executionScope'] is Map ? run : null,
          );
    });
  }

  Future<void> _resume(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已提交继续执行，等待设备确认', () async {
      await ref
          .read(extensionTaskServiceProvider)
          .resume(
            id,
            generation: (run['generation'] as num?)?.toInt() ?? 0,
            expectedRun: run['executionScope'] is Map ? run : null,
          );
    });
  }

  Future<void> _cancel(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已请求取消', () async {
      await ref
          .read(extensionTaskServiceProvider)
          .cancel(
            id,
            reason: 'mobile_user',
            expectedRun: run['executionScope'] is Map ? run : null,
          );
    });
  }

  Future<void> _retry(Map<String, dynamic> run) async {
    final id = _runId(run);
    await _runAction('任务已重新入队', () async {
      await ref
          .read(extensionTaskServiceProvider)
          .retry(id, expectedRun: run['executionScope'] is Map ? run : null);
    });
  }

  Future<void> _recover(Map<String, dynamic> run) async {
    final id = _runId(run);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('恢复任务'),
        content: const Text('请先核对任务结果并确认旧执行已停止。是否从已确认的检查点恢复？'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('取消'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('确认恢复'),
          ),
        ],
      ),
    );
    if (!mounted || confirmed != true) return;
    await _runAction('任务恢复操作已提交', () async {
      await ref
          .read(extensionTaskServiceProvider)
          .recover(id, expectedRun: run['executionScope'] is Map ? run : null);
    });
  }

  @override
  Widget build(BuildContext context) {
    if (_loading)
      return const AmitiaLoadingState(message: '加载 Kernel Task Runtime...');
    if (_error != null)
      return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '任务运行时',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: Icons.verified_user_outlined,
            tooltip: '本机资源审批',
            onPressed: _working ? null : _showSourceTaskApprovals,
          ),
          AmitiaIconButton(
            icon: Icons.playlist_add,
            tooltip: '执行设备任务',
            onPressed: _working ? null : _enqueueDeviceCatalog,
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _working ? null : _load,
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: RefreshIndicator(
          onRefresh: _load,
          child: ListView(
            padding: EdgeInsets.fromLTRB(
              AppSpacing.pagePadding,
              AppSpacing.sm,
              AppSpacing.pagePadding,
              AppSpacing.xxxl,
            ),
            children: [
              AmitiaSectionHeader(
                title: 'Task Definitions',
                actionText: '${_definitions.length} 个定义',
              ),
              SizedBox(height: AppSpacing.sm),
              if (_definitions.isEmpty)
                const AmitiaEmptyState(
                  icon: Icons.schema_outlined,
                  title: '暂无任务定义',
                  subtitle: 'Kernel 扩展尚未注册 Task Definition',
                )
              else
                ..._definitions.map(_definitionCard),
              SizedBox(height: AppSpacing.sectionGap),
              AmitiaSectionHeader(
                title: '运行实例',
                actionText: '${_runs.length} 个运行实例',
              ),
              SizedBox(height: AppSpacing.sm),
              if (_runs.isEmpty)
                const AmitiaEmptyState(
                  icon: Icons.play_circle_outline,
                  title: '暂无运行实例',
                  subtitle: '可从上方任务定义手动入队',
                )
              else
                ..._runs.map(_runCard),
            ],
          ),
        ),
      ),
    );
  }

  Widget _definitionCard(Map<String, dynamic> definition) {
    final id = _definitionId(definition);
    final extensionId = (definition['extensionId'] ?? '').toString();
    final moduleId = (definition['moduleId'] ?? '').toString();
    final runtimeType = (definition['runtimeType'] ?? '').toString();
    final checkpoint =
        definition['checkpoint'] == true || definition['recoverable'] == true;
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    id.isEmpty ? 'Unnamed Task Definition' : id,
                    style: AppTypography.cardTitle(context),
                  ),
                ),
                if (checkpoint)
                  const AmitiaStatusBadge(
                    label: '可恢复',
                    type: BadgeType.success,
                  ),
              ],
            ),
            const SizedBox(height: 6),
            Text(
              [
                extensionId,
                moduleId,
                runtimeType,
              ].where((v) => v.isNotEmpty).join(' · '),
              style: AppTypography.caption(context),
            ),
            const SizedBox(height: 10),
            Row(
              children: [
                Expanded(
                  child: Text(
                    'Entry: ${(definition['entry'] ?? '').toString()}',
                    style: AppTypography.label(context),
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: 10),
                AmitiaButton(
                  label: '手动入队',
                  icon: Icons.add_task,
                  isSecondary: true,
                  onPressed: _working || id.isEmpty
                      ? null
                      : () => _enqueue(definition),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _runCard(Map<String, dynamic> run) {
    final id = _runId(run);
    final definitionId = (run['taskDefinitionId'] ?? '').toString();
    final status = _status(run);
    final error = (run['errorMessage'] ?? '').toString();
    final checkpointId = (run['checkpointId'] ?? '').toString();
    final resultArtifactId = (run['resultArtifactId'] ?? '').toString();
    final isRunning = {
      'running',
      'leased',
      'checkpointing',
    }.contains(status.toLowerCase());
    final isPaused = status.toLowerCase() == 'paused';
    Map<String, dynamic>? definition;
    for (final candidate in _definitions) {
      if (_definitionId(candidate) == definitionId) {
        definition = candidate;
        break;
      }
    }
    final controls = TaskPauseControls.fromRun(run, definition);
    final canRetry = {
      'failed',
      'timed_out',
      'cancelled',
      'succeeded',
    }.contains(status.toLowerCase());
    final canRecover = {
      'failed',
      'timed_out',
      'recovery_required',
      'manual_intervention',
    }.contains(status.toLowerCase());

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: id.isEmpty ? null : () => _showRunDetail(run),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 38,
                  height: 38,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(
                    Icons.task_alt,
                    color: context.accentPrimary,
                    size: 20,
                  ),
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        definitionId.isEmpty ? id : definitionId,
                        style: AppTypography.cardTitle(context),
                      ),
                      const SizedBox(height: 2),
                      Text(id, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(
                  label: _statusLabel(status),
                  type: _badgeType(status),
                ),
              ],
            ),
            if (error.isNotEmpty) ...[
              const SizedBox(height: 8),
              Text(
                error,
                style: AppTypography.caption(
                  context,
                ).copyWith(color: context.error),
                maxLines: 3,
                overflow: TextOverflow.ellipsis,
              ),
            ],
            if (checkpointId.isNotEmpty || resultArtifactId.isNotEmpty) ...[
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  if (checkpointId.isNotEmpty)
                    _RunMetaChip(
                      icon: Icons.bookmark_outline,
                      label: 'Checkpoint $checkpointId',
                    ),
                  if (resultArtifactId.isNotEmpty)
                    _RunMetaChip(
                      icon: Icons.inventory_2_outlined,
                      label: 'Result $resultArtifactId',
                    ),
                ],
              ),
            ],
            const SizedBox(height: 10),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                OutlinedButton.icon(
                  onPressed: id.isEmpty ? null : () => _showRunDetail(run),
                  icon: const Icon(Icons.info_outline),
                  label: const Text('真实详情'),
                ),
                if (controls.pause)
                  OutlinedButton.icon(
                    onPressed: _working || run['readOnly'] == true
                        ? null
                        : () => _pause(run),
                    icon: const Icon(Icons.pause),
                    label: const Text('暂停'),
                  ),
                if (controls.resume)
                  OutlinedButton.icon(
                    onPressed: _working || run['readOnly'] == true
                        ? null
                        : () => _resume(run),
                    icon: const Icon(Icons.play_arrow),
                    label: const Text('继续'),
                  ),
                if (isRunning || isPaused || status == 'queued')
                  OutlinedButton.icon(
                    onPressed: _working || run['readOnly'] == true
                        ? null
                        : () => _cancel(run),
                    icon: const Icon(Icons.cancel_outlined),
                    label: const Text('取消'),
                  ),
                if (canRetry)
                  OutlinedButton.icon(
                    onPressed: _working || run['readOnly'] == true
                        ? null
                        : () => _retry(run),
                    icon: const Icon(Icons.replay),
                    label: const Text('Retry'),
                  ),
                if (canRecover)
                  OutlinedButton.icon(
                    onPressed: _working || run['readOnly'] == true
                        ? null
                        : () => _recover(run),
                    icon: const Icon(Icons.settings_backup_restore),
                    label: const Text('Recover'),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _KernelTaskRuntimeSheet extends StatefulWidget {
  final String taskRunId;
  final Future<Map<String, dynamic>> Function() loader;
  final Future<void> Function() onRetry;
  final Future<void> Function() onRecover;

  const _KernelTaskRuntimeSheet({
    required this.taskRunId,
    required this.loader,
    required this.onRetry,
    required this.onRecover,
  });

  @override
  State<_KernelTaskRuntimeSheet> createState() =>
      _KernelTaskRuntimeSheetState();
}

class _KernelTaskRuntimeSheetState extends State<_KernelTaskRuntimeSheet> {
  late Future<Map<String, dynamic>> _future;

  @override
  void initState() {
    super.initState();
    _future = widget.loader();
  }

  String _pretty(dynamic value) {
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value?.toString() ?? '';
    }
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.86,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 0, 20, 22),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      'Runtime Detail',
                      style: AppTypography.pageTitle(context),
                    ),
                  ),
                  IconButton(
                    onPressed: () => setState(() => _future = widget.loader()),
                    icon: const Icon(Icons.refresh),
                  ),
                  IconButton(
                    onPressed: () => Navigator.pop(context),
                    icon: const Icon(Icons.close),
                  ),
                ],
              ),
              Text(widget.taskRunId, style: AppTypography.caption(context)),
              const SizedBox(height: 10),
              Expanded(
                child: FutureBuilder<Map<String, dynamic>>(
                  future: _future,
                  builder: (context, snapshot) {
                    if (snapshot.connectionState != ConnectionState.done)
                      return const Center(child: CircularProgressIndicator());
                    if (snapshot.hasError)
                      return Center(child: Text('加载失败：${snapshot.error}'));
                    final data = snapshot.data ?? const <String, dynamic>{};
                    final run = Map<String, dynamic>.from(
                      data['run'] as Map? ?? const {},
                    );
                    final progress = Map<String, dynamic>.from(
                      data['progress'] as Map? ?? const {},
                    );
                    final result = Map<String, dynamic>.from(
                      data['result'] as Map? ?? const {},
                    );
                    final checkpoint = Map<String, dynamic>.from(
                      data['checkpoint'] as Map? ?? const {},
                    );
                    final percentage = (progress['percentage'] as num?)
                        ?.toDouble();
                    final current = (progress['current'] as num?)?.toDouble();
                    final total = (progress['total'] as num?)?.toDouble();
                    final stage = (progress['stage'] ?? '').toString();
                    final message = (progress['message'] ?? '').toString();
                    final status = (run['status'] ?? '')
                        .toString()
                        .toLowerCase();
                    final canRetry = {
                      'failed',
                      'timed_out',
                      'cancelled',
                      'succeeded',
                    }.contains(status);
                    final canRecover = {
                      'failed',
                      'timed_out',
                      'recovery_required',
                      'manual_intervention',
                    }.contains(status);
                    return ListView(
                      children: [
                        if (percentage != null) ...[
                          Text(
                            '真实进度 ${percentage.clamp(0, 100).toStringAsFixed(1)}%',
                            style: AppTypography.cardTitle(context),
                          ),
                          const SizedBox(height: 6),
                          LinearProgressIndicator(
                            value: percentage.clamp(0, 100) / 100,
                          ),
                          const SizedBox(height: 6),
                        ] else if (current != null &&
                            total != null &&
                            total > 0) ...[
                          Text(
                            '真实进度 ${current.toStringAsFixed(0)} / ${total.toStringAsFixed(0)}',
                            style: AppTypography.cardTitle(context),
                          ),
                          const SizedBox(height: 6),
                          LinearProgressIndicator(
                            value: (current / total).clamp(0, 1),
                          ),
                          const SizedBox(height: 6),
                        ] else
                          Text(
                            '后端尚未上报 percentage/current/total，不显示虚构百分比',
                            style: AppTypography.caption(context),
                          ),
                        if (stage.isNotEmpty || message.isNotEmpty) ...[
                          const SizedBox(height: 8),
                          Text(
                            [
                              stage,
                              message,
                            ].where((v) => v.isNotEmpty).join(' · '),
                            style: AppTypography.bodySmall(context),
                          ),
                        ],
                        const SizedBox(height: 14),
                        _RuntimeBlock(title: 'Run', text: _pretty(run)),
                        _RuntimeBlock(
                          title: 'Progress',
                          text: _pretty(progress),
                        ),
                        _RuntimeBlock(title: 'Result', text: _pretty(result)),
                        _RuntimeBlock(
                          title: 'Checkpoint',
                          text: _pretty(checkpoint),
                        ),
                        Wrap(
                          spacing: 8,
                          runSpacing: 8,
                          children: [
                            if (canRetry && run['readOnly'] != true)
                              FilledButton.icon(
                                onPressed: widget.onRetry,
                                icon: const Icon(Icons.replay),
                                label: const Text('Retry'),
                              ),
                            if (canRecover && run['readOnly'] != true)
                              OutlinedButton.icon(
                                onPressed: widget.onRecover,
                                icon: const Icon(Icons.settings_backup_restore),
                                label: const Text('Recover'),
                              ),
                          ],
                        ),
                      ],
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
}

class _RuntimeBlock extends StatelessWidget {
  final String title;
  final String text;

  const _RuntimeBlock({required this.title, required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.cardTitle(context)),
          const SizedBox(height: 6),
          Container(
            width: double.infinity,
            constraints: const BoxConstraints(maxHeight: 230),
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
            ),
            child: SingleChildScrollView(
              child: SelectableText(
                text,
                style: const TextStyle(fontFamily: 'monospace', fontSize: 11),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _RunMetaChip extends StatelessWidget {
  final IconData icon;
  final String label;

  const _RunMetaChip({required this.icon, required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: context.textSecondary),
          const SizedBox(width: 4),
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 220),
            child: Text(
              label,
              style: AppTypography.caption(context),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
    );
  }
}
