import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/extension_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

WorkflowApiTarget _workflowTargetFromKey(String key) {
  if (key == 'local') return const WorkflowApiTarget.local();
  if (key.startsWith('device:')) return WorkflowApiTarget.device(key.substring('device:'.length));
  return const WorkflowApiTarget.cloud();
}

final _workflowListProvider = FutureProvider.family<List<Map<String, dynamic>>, String>((ref, key) async {
  if (key == 'device:') return const <Map<String, dynamic>>[];
  return ref.read(extensionServiceProvider).workflows(limit: 200, target: _workflowTargetFromKey(key));
});

class WorkflowListPage extends ConsumerStatefulWidget {
  const WorkflowListPage({super.key});

  @override
  ConsumerState<WorkflowListPage> createState() => _WorkflowListPageState();
}

class _WorkflowListPageState extends ConsumerState<WorkflowListPage> {
  final Set<String> _selected = <String>{};
  String _search = '';
  String _filter = 'all';
  bool _busy = false;
  String _location = 'local';
  String _deviceId = '';
  List<Map<String, dynamic>> _devices = <Map<String, dynamic>>[];
  Timer? _reconcileTimer;
  int? _syncCursor;
  String _syncTargetKey = '';
  bool _syncPolling = false;
  int _deviceRefreshTicks = 0;

  String get _targetKey => _location == 'device' ? 'device:$_deviceId' : _location;
  WorkflowApiTarget get _target => _workflowTargetFromKey(_targetKey);
  bool get _hasUsableTarget => _location != 'device' || _deviceId.isNotEmpty;
  int? _revisionOf(Map<String, dynamic> item) {
    final installation = item['installation'];
    if (installation is! Map) return null;
    final value = installation['revision'];
    return value is int ? value : int.tryParse(value?.toString() ?? '');
  }
  String _editorRoute(String id) => AppRoutes.workflowEditor(id, location: _location, deviceId: _deviceId);
  void _refresh() => ref.invalidate(_workflowListProvider(_targetKey));

  @override
  void initState() {
    super.initState();
    Future<void>.microtask(() async {
      await _loadDevices();
      if (!mounted) return;
      await _primeSyncCursor();
    });
    _reconcileTimer = Timer.periodic(const Duration(seconds: 2), (_) {
      unawaited(_pollSyncEvents());
    });
  }

  @override
  void dispose() {
    _reconcileTimer?.cancel();
    super.dispose();
  }

  Future<void> _loadDevices() async {
    try {
      final items = await ref.read(extensionServiceProvider).workflowDevices();
      if (!mounted) return;
      setState(() {
        _devices = items;
        final exists = items.any((item) => (item['deviceId'] ?? '').toString() == _deviceId);
        if (!exists) _deviceId = items.isEmpty ? '' : (items.first['deviceId'] ?? '').toString();
      });
    } catch (_) {
      if (mounted) setState(() => _devices = <Map<String, dynamic>>[]);
    }
  }

  Future<void> _primeSyncCursor() async {
    if (!_hasUsableTarget) return;
    final key = _targetKey;
    try {
      final page = await ref.read(extensionServiceProvider).workflowSyncEvents(target: _target);
      if (!mounted || _targetKey != key) return;
      final raw = page['cursor'];
      _syncCursor = raw is int ? raw : int.tryParse(raw?.toString() ?? '') ?? 0;
      _syncTargetKey = key;
    } catch (_) {
      if (mounted && _targetKey == key) {
        _syncCursor = null;
        _syncTargetKey = '';
      }
    }
  }

  Future<void> _pollSyncEvents() async {
    if (!mounted || _busy || _syncPolling || !_hasUsableTarget) return;
    final key = _targetKey;
    _syncPolling = true;
    try {
      if (_syncCursor == null || _syncTargetKey != key) {
        await _primeSyncCursor();
        return;
      }
      final page = await ref.read(extensionServiceProvider).workflowSyncEvents(
            target: _target,
            afterCursor: _syncCursor,
          );
      if (!mounted || _targetKey != key) return;
      final rawCursor = page['cursor'];
      _syncCursor = rawCursor is int ? rawCursor : int.tryParse(rawCursor?.toString() ?? '') ?? _syncCursor;
      final items = page['items'];
      final changed = items is List && items.isNotEmpty;
      if (changed) _refresh();
      _deviceRefreshTicks++;
      if (_location != 'local' && _deviceRefreshTicks % 5 == 0) unawaited(_loadDevices());
      // Device-local edits made directly on another device do not yet push into
      // Cloud Core's outbox, so reconcile that catalog at a lower frequency.
      if (_location == 'device' && !changed && _deviceRefreshTicks % 8 == 0) _refresh();
    } catch (_) {
      // Durable cursor is intentionally retained and retried on the next tick.
    } finally {
      _syncPolling = false;
    }
  }

  Future<void> _resetSyncForTarget() async {
    _syncCursor = null;
    _syncTargetKey = '';
    _deviceRefreshTicks = 0;
    _refresh();
    await _primeSyncCursor();
  }

  void _show(String message) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
      ..clearSnackBars()
      ..showSnackBar(SnackBar(content: Text(message)));
  }

  Future<void> _create() async {
    if (!_hasUsableTarget) { _show('请先选择目标设备'); return; }
    try {
      final service = ref.read(extensionServiceProvider);
      final created = await service.createWorkflow(<String, dynamic>{
        'schemaVersion': 'workflow-v2',
        'name': '未命名工作流',
        'description': '',
        'inputSchema': <String, dynamic>{'type': 'object', 'properties': <String, dynamic>{}},
        'outputSchema': <String, dynamic>{'type': 'object', 'properties': <String, dynamic>{}},
        'nodes': <dynamic>[],
        'edges': <dynamic>[],
        'triggers': <dynamic>[
          <String, dynamic>{'id': 'manual', 'type': 'manual', 'config': <String, dynamic>{}, 'enabled': true},
        ],
        'callableByAgent': false,
        'enabled': true,
        'source': 'user',
        'metadata': <String, dynamic>{},
      }, target: _target);
      final id = (created['id'] ?? '').toString();
      _refresh();
      if (mounted && id.isNotEmpty) context.push(_editorRoute(id));
    } catch (error) {
      _show('创建失败：$error');
    }
  }

  Future<void> _createWithAI() async {
    final controller = TextEditingController();
    final instruction = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('AI 创建工作流'),
        content: TextField(
          controller: controller,
          autofocus: true,
          minLines: 4,
          maxLines: 8,
          decoration: const InputDecoration(hintText: '描述想自动化的流程，例如：每天早上获取天气，如果下雨就通知我带伞。'),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          FilledButton(
            onPressed: () {
              final value = controller.text.trim();
              if (value.isNotEmpty) Navigator.pop(dialogContext, value);
            },
            child: const Text('生成'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (instruction == null || instruction.trim().isEmpty || !mounted) return;
    setState(() => _busy = true);
    try {
      final service = ref.read(extensionServiceProvider);
      final proposal = await service.generateWorkflowWithAI(instruction.trim(), target: _target);
      final rawDefinition = proposal['definition'];
      if (rawDefinition is! Map) throw StateError('AI 未返回有效工作流定义');
      final definition = Map<String, dynamic>.from(rawDefinition)..remove('definitionHash');
      final created = await service.createWorkflow(definition, target: _target);
      final id = (created['id'] ?? '').toString();
      _refresh();
      final summary = (proposal['summary'] ?? '').toString().trim();
      if (summary.isNotEmpty) _show(summary);
      if (mounted && id.isNotEmpty) context.push(_editorRoute(id));
    } catch (error) {
      _show('AI 创建失败：$error');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _duplicate(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final created = await ref.read(extensionServiceProvider).duplicateWorkflow(id, target: _target);
    final createdId = (created['id'] ?? '').toString();
    _refresh();
    if (mounted && createdId.isNotEmpty) context.push(_editorRoute(createdId));
  }

  Future<void> _delete(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final name = (item['name'] ?? id).toString();
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('删除工作流'),
            content: Text('确定删除「$name」？运行历史和版本快照会一并移除。'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('删除')),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;
    await ref.read(extensionServiceProvider).deleteWorkflow(id, target: _target);
    _selected.remove(id);
    _refresh();
    if (mounted) setState(() {});
    _show('工作流已删除');
  }

  Future<void> _run(String id) async {
    if (id.isEmpty) return;
    final result = await ref.read(extensionServiceProvider).runWorkflow(id, target: _target);
    final runId = (result['executionId'] ?? '').toString();
    _show(runId.isEmpty ? '已提交运行' : '已提交运行 · $runId');
  }

  Future<void> _saveTemplate(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final controller = TextEditingController(text: (item['name'] ?? '').toString());
    final name = await showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('保存为我的模板'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('模板保存在当前 Amitia 数据库并按用户隔离，不会公开发布。'),
            const SizedBox(height: 12),
            TextField(controller: controller, autofocus: true, decoration: const InputDecoration(labelText: '模板名称')),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(dialogContext, controller.text.trim()), child: const Text('保存')),
        ],
      ),
    );
    controller.dispose();
    if (name == null || name.trim().isEmpty) return;
    await ref.read(extensionServiceProvider).saveWorkflowTemplate(
          id,
          name: name.trim(),
          description: (item['description'] ?? '').toString(),
          target: _target,
        );
    _show('已保存为我的模板');
  }

  Future<void> _showTemplates() async {
    final templates = await ref.read(extensionServiceProvider).workflowTemplates(target: _target);
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.sizeOf(sheetContext).height * 0.72,
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 0, 20, 12),
                child: Row(
                  children: [
                    Expanded(child: Text('我的工作流模板', style: AppTypography.sectionTitle(sheetContext))),
                    Text('${templates.length}', style: AppTypography.caption(sheetContext)),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 20),
                child: Text(
                  '由模板创建的新工作流默认停用自动触发和 Agent 调用，打开编辑器检查后再启用。',
                  style: AppTypography.caption(sheetContext),
                ),
              ),
              const SizedBox(height: 8),
              Expanded(
                child: templates.isEmpty
                    ? const Center(child: Text('还没有我的模板'))
                    : ListView.separated(
                        padding: const EdgeInsets.all(16),
                        itemCount: templates.length,
                        separatorBuilder: (_, __) => const Divider(height: 1),
                        itemBuilder: (context, index) {
                          final item = templates[index];
                          return ListTile(
                            contentPadding: const EdgeInsets.symmetric(horizontal: 4, vertical: 5),
                            title: Text((item['name'] ?? '模板').toString()),
                            subtitle: Text(
                              '${item['nodeCount'] ?? 0} 节点 · ${item['triggerCount'] ?? 0} 触发器\n${(item['description'] ?? '').toString()}',
                              maxLines: 3,
                              overflow: TextOverflow.ellipsis,
                            ),
                            isThreeLine: true,
                            trailing: PopupMenuButton<String>(
                              onSelected: (value) async {
                                if (value == 'use') {
                                  final created = await ref.read(extensionServiceProvider).instantiateWorkflowTemplate((item['templateId'] ?? '').toString(), target: _target);
                                  final id = (created['id'] ?? '').toString();
                                  _refresh();
                                  if (sheetContext.mounted) Navigator.pop(sheetContext);
                                  if (mounted && id.isNotEmpty) context.push(_editorRoute(id));
                                } else if (value == 'delete') {
                                  await ref.read(extensionServiceProvider).deleteWorkflowTemplate((item['templateId'] ?? '').toString(), target: _target);
                                  if (sheetContext.mounted) Navigator.pop(sheetContext);
                                  _show('我的模板已删除');
                                }
                              },
                              itemBuilder: (_) => const [
                                PopupMenuItem(value: 'use', child: Text('使用模板')),
                                PopupMenuItem(value: 'delete', child: Text('删除模板')),
                              ],
                            ),
                            onTap: () async {
                              final created = await ref.read(extensionServiceProvider).instantiateWorkflowTemplate((item['templateId'] ?? '').toString(), target: _target);
                              final id = (created['id'] ?? '').toString();
                              _refresh();
                              if (sheetContext.mounted) Navigator.pop(sheetContext);
                              if (mounted && id.isNotEmpty) context.push(_editorRoute(id));
                            },
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

  String _safeName(String value) {
    final sanitized = value.replaceAll(RegExp(r'[\\/:*?"<>|\r\n]+'), '-').trim();
    return sanitized.isEmpty ? 'workflow' : sanitized.substring(0, sanitized.length > 80 ? 80 : sanitized.length);
  }

  Future<void> _export(Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final envelope = await ref.read(extensionServiceProvider).exportWorkflow(id, target: _target);
    final output = await FilePicker.platform.saveFile(
      dialogTitle: '导出工作流',
      fileName: '${_safeName((item['name'] ?? 'workflow').toString())}.workflow.json',
      type: FileType.custom,
      allowedExtensions: const ['json'],
    );
    if (output == null || output.isEmpty) return;
    await File(output).writeAsString(const JsonEncoder.withIndent('  ').convert(envelope), flush: true);
    _show('工作流已导出');
  }

  Future<void> _import() async {
    final picked = await FilePicker.platform.pickFiles(
      dialogTitle: '导入工作流 JSON',
      type: FileType.custom,
      allowedExtensions: const ['json'],
      withData: true,
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    try {
      String text;
      if (file.bytes != null) {
        text = utf8.decode(file.bytes!);
      } else if (file.path != null && file.path!.isNotEmpty) {
        text = await File(file.path!).readAsString();
      } else {
        throw StateError('无法读取所选文件');
      }
      final decoded = jsonDecode(text);
      if (decoded is! Map) throw const FormatException('根节点必须是 JSON Object');
      final created = await ref.read(extensionServiceProvider).importWorkflow(decoded.cast<String, dynamic>(), target: _target);
      final id = (created['id'] ?? '').toString();
      _refresh();
      _show('已导入。自动触发和 Agent 调用默认停用。');
      if (mounted && id.isNotEmpty) context.push(_editorRoute(id));
    } catch (error) {
      _show('导入失败：$error');
    }
  }

  Future<void> _batchEnable(bool enabled) async {
    if (_selected.isEmpty) return;
    setState(() => _busy = true);
    try {
      for (final id in _selected) {
        await ref.read(extensionServiceProvider).setWorkflowEnabled(id, enabled, target: _target);
      }
      _selected.clear();
      _refresh();
      _show(enabled ? '已批量启用' : '已批量停用');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _batchDelete() async {
    if (_selected.isEmpty) return;
    final count = _selected.length;
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (dialogContext) => AlertDialog(
            title: const Text('批量删除'),
            content: Text('确定删除选中的 $count 个工作流？运行历史和版本快照会一起删除。'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('删除')),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;
    setState(() => _busy = true);
    try {
      for (final id in _selected.toList()) {
        await ref.read(extensionServiceProvider).deleteWorkflow(id, target: _target);
      }
      _selected.clear();
      _refresh();
      _show('已批量删除');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  List<Map<String, dynamic>> _filtered(List<Map<String, dynamic>> items) {
    final q = _search.trim().toLowerCase();
    return items.where((item) {
      final enabled = item['enabled'] == true;
      final callable = item['callableByAgent'] == true;
      if (_filter == 'enabled' && !enabled) return false;
      if (_filter == 'disabled' && enabled) return false;
      if (_filter == 'agent' && !callable) return false;
      if (q.isEmpty) return true;
      final name = (item['name'] ?? '').toString().toLowerCase();
      final description = (item['description'] ?? '').toString().toLowerCase();
      return name.contains(q) || description.contains(q);
    }).toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final asyncItems = ref.watch(_workflowListProvider(_targetKey));
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '工作流',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          IconButton(tooltip: '导入 JSON', onPressed: _busy || _location == 'device' ? null : _import, icon: const Icon(Icons.file_download_outlined)),
          IconButton(tooltip: '我的模板', onPressed: _busy || _location == 'device' ? null : _showTemplates, icon: const Icon(Icons.dashboard_customize_outlined)),
          IconButton(tooltip: 'AI 创建', onPressed: _busy || _location == 'device' ? null : _createWithAI, icon: const Icon(Icons.auto_awesome_outlined)),
          IconButton(tooltip: '刷新', onPressed: _refresh, icon: const Icon(Icons.refresh)),
        ],
      ),
      body: SafeArea(
        top: false,
        child: asyncItems.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _ErrorState(message: error.toString(), onRetry: _refresh),
          data: (items) {
            final filtered = _filtered(items);
            return RefreshIndicator(
              onRefresh: () async => _refresh(),
              child: ListView(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  SegmentedButton<String>(
                    segments: const <ButtonSegment<String>>[
                      ButtonSegment<String>(value: 'local', label: Text('当前设备'), icon: Icon(Icons.phone_android_outlined)),
                      ButtonSegment<String>(value: 'cloud', label: Text('云端'), icon: Icon(Icons.cloud_outlined)),
                      ButtonSegment<String>(value: 'device', label: Text('我的设备'), icon: Icon(Icons.devices_other_outlined)),
                    ],
                    selected: <String>{_location},
                    onSelectionChanged: (values) async {
                      if (values.isEmpty) return;
                      setState(() { _location = values.first; _selected.clear(); });
                      if (_location == 'device' && _devices.isEmpty) await _loadDevices();
                      await _resetSyncForTarget();
                    },
                  ),
                  if (_location == 'device') ...[
                    const SizedBox(height: 10),
                    DropdownButtonFormField<String>(
                      value: _deviceId.isEmpty ? null : _deviceId,
                      decoration: const InputDecoration(labelText: '目标设备'),
                      items: _devices.map((device) {
                        final id = (device['deviceId'] ?? '').toString();
                        final label = (device['label'] ?? id).toString();
                        final online = device['online'] == true;
                        return DropdownMenuItem<String>(value: id, child: Text('$label · ${online ? '在线' : '离线'}'));
                      }).toList(growable: false),
                      onChanged: (value) {
                        setState(() { _deviceId = value ?? ''; _selected.clear(); });
                        unawaited(_resetSyncForTarget());
                      },
                    ),
                  ],
                  const SizedBox(height: 12),
                  _IntroCard(
                    onCreate: _create,
                    onCreateAI: _location == 'device' ? () => _show('远程设备控制面不提供 AI 创建') : _createWithAI,
                    onTemplates: _location == 'device' ? () => _show('远程设备控制面不提供模板管理') : _showTemplates,
                  ),
                  SizedBox(height: AppSpacing.sectionGap),
                  TextField(
                    decoration: const InputDecoration(prefixIcon: Icon(Icons.search), hintText: '搜索工作流名称或描述'),
                    onChanged: (value) => setState(() => _search = value),
                  ),
                  const SizedBox(height: 10),
                  Row(
                    children: [
                      Expanded(
                        child: DropdownButtonFormField<String>(
                          initialValue: _filter,
                          decoration: const InputDecoration(labelText: '筛选'),
                          items: const [
                            DropdownMenuItem(value: 'all', child: Text('全部')),
                            DropdownMenuItem(value: 'enabled', child: Text('已启用')),
                            DropdownMenuItem(value: 'disabled', child: Text('已停用')),
                            DropdownMenuItem(value: 'agent', child: Text('允许 AI 调用')),
                          ],
                          onChanged: (value) => setState(() => _filter = value ?? 'all'),
                        ),
                      ),
                      const SizedBox(width: 12),
                      Text('${filtered.length} / ${items.length}', style: AppTypography.caption(context)),
                    ],
                  ),
                  if (_selected.isNotEmpty) ...[
                    const SizedBox(height: 10),
                    Container(
                      padding: const EdgeInsets.all(10),
                      decoration: BoxDecoration(
                        color: context.surfaceSecondary,
                        borderRadius: AppRadius.brMedium,
                        border: Border.all(color: context.borderPrimary, width: 0.5),
                      ),
                      child: Wrap(
                        spacing: 8,
                        runSpacing: 6,
                        crossAxisAlignment: WrapCrossAlignment.center,
                        children: [
                          Text('已选 ${_selected.length}', style: AppTypography.caption(context)),
                          OutlinedButton(onPressed: _busy ? null : () => _batchEnable(true), child: const Text('启用')),
                          OutlinedButton(onPressed: _busy ? null : () => _batchEnable(false), child: const Text('停用')),
                          OutlinedButton(onPressed: _busy ? null : _batchDelete, child: const Text('删除')),
                          TextButton(onPressed: () => setState(_selected.clear), child: const Text('取消选择')),
                        ],
                      ),
                    ),
                  ],
                  SizedBox(height: AppSpacing.sectionGap),
                  Row(
                    children: [
                      Expanded(child: Text('我的工作流', style: AppTypography.sectionTitle(context))),
                      Text('${filtered.length}', style: AppTypography.caption(context)),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  if (filtered.isEmpty)
                    _EmptyState(
                      hasAny: items.isNotEmpty,
                      onCreate: _create,
                      onCreateAI: _location == 'device' ? () => _show('远程设备控制面不提供 AI 创建') : _createWithAI,
                      onTemplates: _location == 'device' ? () => _show('远程设备控制面不提供模板管理') : _showTemplates,
                    )
                  else
                    ...filtered.map(
                      (item) {
                        final id = (item['id'] ?? '').toString();
                        return Padding(
                          padding: EdgeInsets.only(bottom: AppSpacing.md),
                          child: _WorkflowCard(
                            item: item,
                            selected: _selected.contains(id),
                            onSelected: (value) => setState(() => value ? _selected.add(id) : _selected.remove(id)),
                            onOpen: item['offline'] == true ? () => _show('设备离线：当前只能查看最后已知目录，完整 Definition 需要设备上线后读取') : () => context.push(_editorRoute(id)),
                            onRun: item['enabled'] == true && item['offline'] != true ? () => _run(id) : null,
                            onToggle: (value) async {
                              if (item['offline'] == true) { _show('设备离线：不能修改本地工作流'); return; }
                              await ref.read(extensionServiceProvider).setWorkflowEnabled(id, value, target: _target, expectedRevision: _revisionOf(item));
                              _refresh();
                            },
                            onDuplicate: _location == 'device' ? () => _show('远程设备控制面不提供复制') : () => _duplicate(item),
                            onTemplate: _location == 'device' ? () => _show('远程设备控制面不提供模板管理') : () => _saveTemplate(item),
                            onExport: _location == 'device' ? () => _show('远程设备离线目录不提供导出') : () => _export(item),
                            onDelete: item['offline'] == true ? () => _show('设备离线：不能删除本地工作流') : () => _delete(item),
                          ),
                        );
                      },
                    ),
                ],
              ),
            );
          },
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _busy || !_hasUsableTarget ? null : _create,
        icon: const Icon(Icons.add),
        label: const Text('新建工作流'),
      ),
    );
  }
}

class _IntroCard extends StatelessWidget {
  final VoidCallback onCreate;
  final VoidCallback onCreateAI;
  final VoidCallback onTemplates;
  const _IntroCard({required this.onCreate, required this.onCreateAI, required this.onTemplates});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.xl),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brLarge,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.account_tree_outlined, color: context.accentPrimary),
              const SizedBox(width: 10),
              Expanded(child: Text('Extension Kernel Workflow', style: AppTypography.cardTitle(context))),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            '创建和调试 DAG；模板、JSON 导入导出和版本回滚都保存在当前 Amitia 数据环境中。',
            style: AppTypography.caption(context),
          ),
          const SizedBox(height: 14),
          Wrap(
            spacing: 10,
            runSpacing: 8,
            children: [
              FilledButton.icon(onPressed: onCreateAI, icon: const Icon(Icons.auto_awesome_outlined), label: const Text('AI 创建')),
              OutlinedButton.icon(onPressed: onCreate, icon: const Icon(Icons.add), label: const Text('手动创建')),
              OutlinedButton.icon(onPressed: onTemplates, icon: const Icon(Icons.dashboard_customize_outlined), label: const Text('我的模板')),
            ],
          ),
        ],
      ),
    );
  }
}

class _WorkflowCard extends StatelessWidget {
  final Map<String, dynamic> item;
  final bool selected;
  final ValueChanged<bool> onSelected;
  final VoidCallback onOpen;
  final VoidCallback? onRun;
  final ValueChanged<bool> onToggle;
  final VoidCallback onDuplicate;
  final VoidCallback onTemplate;
  final VoidCallback onExport;
  final VoidCallback onDelete;

  const _WorkflowCard({
    required this.item,
    required this.selected,
    required this.onSelected,
    required this.onOpen,
    required this.onRun,
    required this.onToggle,
    required this.onDuplicate,
    required this.onTemplate,
    required this.onExport,
    required this.onDelete,
  });

  int _count(String key) => (item[key] as List?)?.length ?? 0;

  @override
  Widget build(BuildContext context) {
    final enabled = item['enabled'] == true;
    return Material(
      color: context.surfacePrimary,
      borderRadius: AppRadius.brLarge,
      child: InkWell(
        borderRadius: AppRadius.brLarge,
        onTap: onOpen,
        child: Container(
          padding: EdgeInsets.all(AppSpacing.cardPadding),
          decoration: BoxDecoration(
            borderRadius: AppRadius.brLarge,
            border: Border.all(color: selected ? context.accentPrimary : context.borderPrimary, width: selected ? 1.2 : 0.5),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Checkbox(value: selected, onChanged: (value) => onSelected(value == true)),
                  Expanded(
                    child: Text(
                      (item['name'] ?? '未命名工作流').toString(),
                      style: AppTypography.cardTitle(context),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  Switch(value: enabled, onChanged: onToggle),
                ],
              ),
              if ((item['description'] ?? '').toString().isNotEmpty) ...[
                const SizedBox(height: 4),
                Text((item['description'] ?? '').toString(), style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
              ],
              const SizedBox(height: 10),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  _Pill(label: '${_count('nodes')} 节点'),
                  _Pill(label: '${_count('edges')} 连线'),
                  _Pill(label: '${_count('triggers')} Trigger'),
                  if (item['callableByAgent'] == true) const _Pill(label: 'Agent Tool'),
                  if (item['cached'] == true) const _Pill(label: '离线目录缓存'),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  TextButton.icon(onPressed: onRun, icon: const Icon(Icons.play_arrow, size: 18), label: const Text('运行')),
                  TextButton.icon(onPressed: onOpen, icon: const Icon(Icons.edit_outlined, size: 18), label: const Text('编辑')),
                  const Spacer(),
                  PopupMenuButton<String>(
                    tooltip: '更多',
                    onSelected: (value) {
                      switch (value) {
                        case 'duplicate':
                          onDuplicate();
                          break;
                        case 'template':
                          onTemplate();
                          break;
                        case 'export':
                          onExport();
                          break;
                        case 'delete':
                          onDelete();
                          break;
                      }
                    },
                    itemBuilder: (_) => const [
                      PopupMenuItem(value: 'duplicate', child: Text('复制')),
                      PopupMenuItem(value: 'template', child: Text('保存为我的模板')),
                      PopupMenuItem(value: 'export', child: Text('导出 JSON')),
                      PopupMenuDivider(),
                      PopupMenuItem(value: 'delete', child: Text('删除')),
                    ],
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Pill extends StatelessWidget {
  final String label;
  const _Pill({required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(999)),
      child: Text(label, style: AppTypography.caption(context)),
    );
  }
}

class _EmptyState extends StatelessWidget {
  final bool hasAny;
  final VoidCallback onCreate;
  final VoidCallback onCreateAI;
  final VoidCallback onTemplates;
  const _EmptyState({required this.hasAny, required this.onCreate, required this.onCreateAI, required this.onTemplates});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      alignment: Alignment.center,
      child: Column(
        children: [
          Icon(Icons.account_tree_outlined, size: 54, color: context.textTertiary),
          const SizedBox(height: 12),
          Text(hasAny ? '没有匹配的工作流' : '还没有工作流', style: AppTypography.cardTitle(context)),
          const SizedBox(height: 6),
          Text(hasAny ? '调整搜索或筛选条件。' : '从空白、AI 或我的模板开始。', style: AppTypography.caption(context)),
          if (!hasAny) ...[
            const SizedBox(height: 16),
            Wrap(
              spacing: 10,
              runSpacing: 8,
              children: [
                FilledButton.icon(onPressed: onCreateAI, icon: const Icon(Icons.auto_awesome_outlined), label: const Text('AI 创建')),
                OutlinedButton(onPressed: onCreate, child: const Text('手动新建')),
                OutlinedButton(onPressed: onTemplates, child: const Text('我的模板')),
              ],
            ),
          ],
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorState({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center, style: AppTypography.caption(context)),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}
