import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class SystemPluginsPage extends ConsumerStatefulWidget {
  const SystemPluginsPage({super.key});

  @override
  ConsumerState<SystemPluginsPage> createState() => _SystemPluginsPageState();
}

class _SystemPluginsPageState extends ConsumerState<SystemPluginsPage> {
  List<Map<String, dynamic>> _plugins = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadPlugins();
  }

  Future<void> _loadPlugins() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final plugins = await svc.plugins();
      if (mounted) setState(() { _plugins = plugins; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(
          title: '系统插件',
          showBackButton: true,
          fallbackRoute: AppRoutes.extensions,
        ),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(
          title: '系统插件',
          showBackButton: true,
          fallbackRoute: AppRoutes.extensions,
        ),
        body: SafeArea(top: false, child: AmitiaErrorState(message: _error!, onRetry: _loadPlugins)),
      );
    }
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '系统插件',
        showBackButton: true,
        fallbackRoute: AppRoutes.extensions,
      ),
      body: SafeArea(
        top: false,
        child: ListView.separated(
          padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.xxxl),
          itemCount: _plugins.length,
          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
          itemBuilder: (context, index) => _buildPluginCard(context, _plugins[index]),
        ),
      ),
    );
  }

  List<String> _stringList(dynamic list) {
    if (list is List) {
      return list.map((e) => e?.toString() ?? '').where((s) => s.isNotEmpty).toList();
    }
    return [];
  }

  bool _isEnabled(Map<String, dynamic> plugin) {
    return (plugin['isEnabled'] as bool?) ?? ((plugin['enabled'] as int?) == 1);
  }

  Widget _buildPluginCard(BuildContext context, Map<String, dynamic> plugin) {
    final name = (plugin['name'] ?? '').toString();
    final description = (plugin['description'] ?? '').toString();
    final version = (plugin['version'] ?? '').toString();
    final runtimeStatus = (plugin['runtimeStatus'] ?? plugin['runtime_status'] ?? '运行中').toString();
    final isEnabled = _isEnabled(plugin);
    final hooks = _stringList(plugin['hooks']);
    final events = _stringList(plugin['events']);
    final schedules = _stringList(plugin['schedules']);
    final registeredSkills = _stringList(plugin['registeredSkills'] ?? plugin['registered_skills']);

    return AmitiaCard(
      onTap: () => _showDetailSheet(plugin),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 44,
                height: 44,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.extension, size: 22, color: context.accentPrimary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Row(
                      children: [
                        Expanded(child: Text(name, style: AppTypography.cardTitle(context))),
                        const SizedBox(width: 8),
                        Text('v$version', style: AppTypography.label(context)),
                      ],
                    ),
                    const SizedBox(height: 4),
                    Text(description, style: AppTypography.caption(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                  ],
                ),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              Container(
                width: 8,
                height: 8,
                decoration: BoxDecoration(
                  color: runtimeStatus == '运行中' ? context.success : context.warning,
                  shape: BoxShape.circle,
                ),
              ),
              const SizedBox(width: 6),
              Text(runtimeStatus, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w500)),
              const Spacer(),
              AmitiaStatusBadge(
                label: isEnabled ? '已启用' : '已禁用',
                type: isEnabled ? BadgeType.success : BadgeType.neutral,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.md),
          _buildInfoChips(context, hooks, events, schedules, registeredSkills),
          SizedBox(height: AppSpacing.md),
          Row(
            children: [
              GestureDetector(
                onTap: () => _showDetailSheet(plugin),
                child: _ActionButton(
                  label: '详情',
                  icon: Icons.info_outline,
                  color: context.accentPrimary,
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () => _showPermissionSettings(plugin),
                child: _ActionButton(
                  label: '权限',
                  icon: Icons.shield_outlined,
                  color: context.info,
                ),
              ),
              const SizedBox(width: 8),
              GestureDetector(
                onTap: () {
                  if (isEnabled) {
                    _showDisableImpactConfirm(plugin);
                  } else {
                    _enablePlugin(plugin);
                  }
                },
                child: _ActionButton(
                  label: isEnabled ? '禁用' : '启用',
                  icon: isEnabled ? Icons.block : Icons.check_circle_outline,
                  color: isEnabled ? context.error : context.success,
                ),
              ),
              const Spacer(),
              Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildInfoChips(BuildContext context, List<String> hooks, List<String> events, List<String> schedules, List<String> registeredSkills) {
    return Wrap(
      spacing: 6,
      runSpacing: 6,
      children: [
        if (hooks.isNotEmpty)
          _InfoChip(label: 'Hook ${hooks.length}', icon: Icons.link, color: context.accentPrimary),
        if (events.isNotEmpty)
          _InfoChip(label: '事件 ${events.length}', icon: Icons.event_outlined, color: context.info),
        if (schedules.isNotEmpty)
          _InfoChip(label: '调度 ${schedules.length}', icon: Icons.schedule, color: context.warning),
        if (registeredSkills.isNotEmpty)
          _InfoChip(label: '技能 ${registeredSkills.length}', icon: Icons.auto_awesome, color: context.success),
      ],
    );
  }

  void _showDetailSheet(Map<String, dynamic> plugin) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      isScrollControlled: true,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(22))),
      builder: (ctx) => _PluginDetailSheet(plugin: plugin, onPermissionTap: () {
        Navigator.pop(ctx);
        _showPermissionSettings(plugin);
      }),
    );
  }

  void _showDisableImpactConfirm(Map<String, dynamic> plugin) {
    final hooks = _stringList(plugin['hooks']);
    final events = _stringList(plugin['events']);
    final schedules = _stringList(plugin['schedules']);
    final registeredSkills = _stringList(plugin['registeredSkills'] ?? plugin['registered_skills']);
    showDialog(
      context: context,
      builder: (ctx) => _DisableImpactDialog(
        pluginName: (plugin['name'] ?? '').toString(),
        hooksCount: hooks.length,
        eventsCount: events.length,
        schedulesCount: schedules.length,
        registeredSkillsCount: registeredSkills.length,
        onConfirm: () async {
          Navigator.pop(ctx);
          await _disablePlugin(plugin);
        },
      ),
    );
  }

  Future<void> _enablePlugin(Map<String, dynamic> plugin) async {
    try {
      final svc = ref.read(extensionServiceProvider);
      final id = (plugin['id'] ?? '').toString();
      await svc.enablePlugin(id);
      _loadPlugins();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${plugin['name'] ?? ''} 已启用'), backgroundColor: context.success),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('启用失败: $e'), backgroundColor: context.error),
        );
      }
    }
  }

  Future<void> _disablePlugin(Map<String, dynamic> plugin) async {
    try {
      final svc = ref.read(extensionServiceProvider);
      final id = (plugin['id'] ?? '').toString();
      await svc.disablePlugin(id);
      _loadPlugins();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('${plugin['name'] ?? ''} 已禁用'), backgroundColor: context.error),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('禁用失败: $e'), backgroundColor: context.error),
        );
      }
    }
  }

  void _showPermissionSettings(Map<String, dynamic> plugin) {
    final characters = ref.read(characterListProvider).valueOrNull ?? const [];
    if (characters.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请先创建角色，再配置插件权限')),
      );
      return;
    }
    final active = characters.where((item) => item.isActive == 1).firstOrNull;
    final character = active ?? characters.first;
    showDialog(
      context: context,
      builder: (ctx) => _PermissionSettingsDialog(
        pluginId: (plugin['id'] ?? '').toString(),
        pluginName: (plugin['name'] ?? '').toString(),
        characterId: character.id,
        characterName: character.name,
      ),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;

  const _ActionButton({required this.label, required this.icon, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 15, color: color),
          const SizedBox(width: 5),
          Text(label, style: TextStyle(fontSize: 13, color: color, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}

class _InfoChip extends StatelessWidget {
  final String label;
  final IconData icon;
  final Color color;

  const _InfoChip({required this.label, required this.icon, required this.color});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.1),
        borderRadius: AppRadius.brTag,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 13, color: color),
          const SizedBox(width: 4),
          Text(label, style: TextStyle(fontSize: 11, color: color, fontWeight: FontWeight.w500)),
        ],
      ),
    );
  }
}

class _PluginDetailSheet extends ConsumerStatefulWidget {
  final Map<String, dynamic> plugin;
  final VoidCallback onPermissionTap;

  const _PluginDetailSheet({required this.plugin, required this.onPermissionTap});

  @override
  ConsumerState<_PluginDetailSheet> createState() => _PluginDetailSheetState();
}

class _PluginDetailSheetState extends ConsumerState<_PluginDetailSheet> {
  bool _loading = true;
  bool _working = false;
  String? _error;
  Map<String, dynamic> _config = const {};
  Map<String, dynamic> _health = const {};
  Map<String, dynamic> _surface = const {};
  List<Map<String, dynamic>> _state = const [];
  List<Map<String, dynamic>> _schedules = const [];
  List<Map<String, dynamic>> _events = const [];
  String _characterId = '';

  String get _pluginId => (widget.plugin['id'] ?? '').toString();

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    if (_pluginId.isEmpty) {
      setState(() {
        _loading = false;
        _error = '插件 ID 缺失';
      });
      return;
    }
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final characters = await ref.read(characterServiceProvider).list();
      final active = characters.where((item) => item.isActive == 1).firstOrNull;
      final characterId = (active ?? (characters.isNotEmpty ? characters.first : null))?.id ?? '';
      final svc = ref.read(extensionServiceProvider);
      final configFuture = svc.getPluginConfig(_pluginId);
      final healthFuture = svc.getPluginHealth(_pluginId);
      final surfaceFuture = svc.getPluginSurface(_pluginId);
      final stateFuture = characterId.isEmpty
          ? Future<List<Map<String, dynamic>>>.value(const [])
          : svc.getPluginState(_pluginId, characterId: characterId);
      final schedulesFuture = characterId.isEmpty
          ? Future<List<Map<String, dynamic>>>.value(const [])
          : svc.getPluginSchedules(_pluginId, characterId: characterId);
      final eventsFuture = characterId.isEmpty
          ? Future<Map<String, dynamic>>.value(const {'items': <dynamic>[]})
          : svc.getPluginEvents(_pluginId, characterId: characterId);
      final values = await Future.wait<dynamic>([
        configFuture,
        healthFuture,
        surfaceFuture,
        stateFuture,
        schedulesFuture,
        eventsFuture,
      ]);
      final eventPage = values[5] as Map<String, dynamic>;
      if (!mounted) return;
      setState(() {
        _characterId = characterId;
        _config = Map<String, dynamic>.from(values[0] as Map? ?? const {});
        _health = Map<String, dynamic>.from(values[1] as Map? ?? const {});
        _surface = Map<String, dynamic>.from(values[2] as Map? ?? const {});
        _state = (values[3] as List).cast<Map<String, dynamic>>();
        _schedules = (values[4] as List).cast<Map<String, dynamic>>();
        _events = ((eventPage['items'] as List?) ?? const [])
            .whereType<Map>()
            .map((item) => Map<String, dynamic>.from(item))
            .toList(growable: false);
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

  Future<void> _run(String successMessage, Future<void> Function() action) async {
    if (_working) return;
    setState(() => _working = true);
    try {
      await action();
      await _load();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(successMessage)));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: $e')));
      }
    } finally {
      if (mounted) setState(() => _working = false);
    }
  }

  Future<void> _editConfig() async {
    final controller = TextEditingController(
      text: const JsonEncoder.withIndent('  ').convert(_config),
    );
    final save = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('编辑真实插件配置'),
        content: SizedBox(
          width: 620,
          child: TextField(
            controller: controller,
            minLines: 12,
            maxLines: 22,
            decoration: const InputDecoration(
              hintText: '{\n  "key": "value"\n}',
              border: OutlineInputBorder(),
            ),
            style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
          ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('保存')),
        ],
      ),
    );
    if (save != true) return;
    try {
      final decoded = jsonDecode(controller.text);
      if (decoded is! Map) throw const FormatException('配置根节点必须是 JSON Object');
      await _run('插件配置已保存', () async {
        await ref.read(extensionServiceProvider).updatePluginConfig(
              _pluginId,
              Map<String, dynamic>.from(decoded),
            );
      });
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('配置无效: $e')));
      }
    } finally {
      controller.dispose();
    }
  }

  String _pretty(dynamic value) {
    try {
      return const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      return value.toString();
    }
  }

  String _scheduleId(Map<String, dynamic> item) =>
      (item['id'] ?? item['scheduleId'] ?? item['schedule_id'] ?? '').toString();

  bool _scheduleEnabled(Map<String, dynamic> item) {
    final raw = item['enabled'] ?? item['isEnabled'] ?? item['is_enabled'];
    if (raw is bool) return raw;
    if (raw is num) return raw != 0;
    final status = (item['status'] ?? '').toString().toLowerCase();
    return status != 'paused' && status != 'disabled';
  }

  String _eventId(Map<String, dynamic> item) =>
      (item['id'] ?? item['eventId'] ?? item['event_id'] ?? '').toString();

  List<Map<String, dynamic>> _surfaceActions() {
    final raw = _surface['actions'];
    if (raw is! List) return const [];
    return raw.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false);
  }

  Future<void> _executeAction(Map<String, dynamic> action) async {
    if (_characterId.isEmpty) return;
    final actionId = (action['id'] ?? action['actionId'] ?? '').toString();
    if (actionId.isEmpty) return;
    final controller = TextEditingController(text: '{}');
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text('执行 ${action['label'] ?? actionId}'),
        content: TextField(
          controller: controller,
          minLines: 4,
          maxLines: 10,
          decoration: const InputDecoration(labelText: 'Input JSON', border: OutlineInputBorder()),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('执行')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      final decoded = jsonDecode(controller.text);
      if (decoded is! Map) throw const FormatException('Input 必须是 JSON Object');
      final result = await ref.read(extensionServiceProvider).executePluginAction(
            _pluginId,
            actionId,
            characterId: _characterId,
            input: Map<String, dynamic>.from(decoded),
          );
      if (mounted) {
        await showDialog<void>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('Action Result'),
            content: SingleChildScrollView(child: SelectableText(_pretty(result))),
            actions: [TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭'))],
          ),
        );
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('执行失败: $e')));
    } finally {
      controller.dispose();
    }
  }

  @override
  Widget build(BuildContext context) {
    final plugin = widget.plugin;
    final name = (plugin['name'] ?? '').toString();
    final version = (plugin['version'] ?? '').toString();
    final runtimeStatus = (plugin['runtimeStatus'] ?? plugin['runtime_status'] ?? '未知').toString();
    final healthStatus = (_health['status'] ?? _health['state'] ?? '未知').toString();
    final circuit = (_health['circuit'] ?? _health['circuitState'] ?? _health['circuit_state'] ?? '未知').toString();
    final actions = _surfaceActions();

    return SafeArea(
      top: false,
      child: SizedBox(
        height: MediaQuery.sizeOf(context).height * 0.88,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(20, 12, 20, 20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              const SizedBox(height: 16),
              Row(
                children: [
                  Expanded(child: Text('$name  v$version', style: AppTypography.cardTitle(context))),
                  IconButton(onPressed: _working ? null : _load, icon: const Icon(Icons.refresh)),
                  IconButton(onPressed: () => Navigator.pop(context), icon: const Icon(Icons.close)),
                ],
              ),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  AmitiaStatusBadge(label: 'Runtime $runtimeStatus', type: BadgeType.neutral),
                  AmitiaStatusBadge(label: 'Health $healthStatus', type: healthStatus.toLowerCase().contains('healthy') ? BadgeType.success : BadgeType.warning),
                  AmitiaStatusBadge(label: 'Circuit $circuit', type: circuit.toLowerCase().contains('open') ? BadgeType.warning : BadgeType.neutral),
                ],
              ),
              const SizedBox(height: 12),
              if (_loading)
                const Expanded(child: Center(child: CircularProgressIndicator()))
              else if (_error != null)
                Expanded(child: AmitiaErrorState(message: _error!, onRetry: _load))
              else
                Expanded(
                  child: ListView(
                    children: [
                      Wrap(
                        spacing: 8,
                        runSpacing: 8,
                        children: [
                          OutlinedButton.icon(onPressed: _working ? null : () => _run('插件已重载', () async => ref.read(extensionServiceProvider).reloadPlugin(_pluginId)), icon: const Icon(Icons.restart_alt), label: const Text('重载插件')),
                          OutlinedButton.icon(onPressed: _working ? null : _editConfig, icon: const Icon(Icons.tune), label: const Text('编辑配置')),
                          OutlinedButton.icon(onPressed: _working ? null : () => _run('配置已恢复默认', () async => ref.read(extensionServiceProvider).resetPluginConfig(_pluginId)), icon: const Icon(Icons.settings_backup_restore), label: const Text('恢复配置')),
                          OutlinedButton.icon(onPressed: _working ? null : () => _run('Circuit 已重置', () async => ref.read(extensionServiceProvider).resetPluginCircuit(_pluginId)), icon: const Icon(Icons.electrical_services), label: const Text('重置 Circuit')),
                          OutlinedButton.icon(onPressed: widget.onPermissionTap, icon: const Icon(Icons.shield_outlined), label: const Text('权限设置')),
                        ],
                      ),
                      const SizedBox(height: 16),
                      _RuntimeJsonSection(title: 'Health / Circuit', value: _health),
                      _RuntimeJsonSection(title: '真实配置', value: _config),
                      _RuntimeJsonSection(title: 'UI Surface', value: _surface),
                      _RuntimeJsonSection(title: 'Runtime State', value: _state),
                      const SizedBox(height: 10),
                      Text('调度任务', style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 6),
                      if (_schedules.isEmpty)
                        Text(_characterId.isEmpty ? '未创建角色，无法读取角色作用域调度' : '暂无调度任务', style: AppTypography.bodySmall(context))
                      else
                        ..._schedules.map((item) {
                          final id = _scheduleId(item);
                          final enabled = _scheduleEnabled(item);
                          return SwitchListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text((item['name'] ?? item['label'] ?? id).toString()),
                            subtitle: Text(id.isEmpty ? _pretty(item) : id, maxLines: 2, overflow: TextOverflow.ellipsis),
                            value: enabled,
                            onChanged: id.isEmpty || _working || _characterId.isEmpty
                                ? null
                                : (value) => _run('调度状态已更新', () => ref.read(extensionServiceProvider).setPluginScheduleEnabled(_pluginId, id, value, characterId: _characterId)),
                          );
                        }),
                      const SizedBox(height: 10),
                      Text('事件状态', style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 6),
                      if (_events.isEmpty)
                        Text(_characterId.isEmpty ? '未创建角色，无法读取角色作用域事件' : '暂无事件', style: AppTypography.bodySmall(context))
                      else
                        ..._events.map((item) {
                          final id = _eventId(item);
                          final status = (item['status'] ?? 'unknown').toString();
                          final retryable = id.isNotEmpty && !{'success', 'completed', 'done'}.contains(status.toLowerCase());
                          return ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text((item['name'] ?? item['type'] ?? id).toString()),
                            subtitle: Text('状态: $status${id.isEmpty ? '' : ' · $id'}'),
                            trailing: retryable && _characterId.isNotEmpty
                                ? TextButton(onPressed: _working ? null : () => _run('事件已重新入队', () => ref.read(extensionServiceProvider).retryPluginEvent(_pluginId, id, characterId: _characterId)), child: const Text('重试'))
                                : null,
                          );
                        }),
                      if (actions.isNotEmpty) ...[
                        const SizedBox(height: 10),
                        Text('Surface Actions', style: AppTypography.cardTitle(context)),
                        const SizedBox(height: 6),
                        ...actions.map((action) {
                          final id = (action['id'] ?? action['actionId'] ?? '').toString();
                          return ListTile(
                            contentPadding: EdgeInsets.zero,
                            title: Text((action['label'] ?? action['name'] ?? id).toString()),
                            subtitle: Text(id),
                            trailing: TextButton(
                              onPressed: id.isEmpty || _characterId.isEmpty ? null : () => _executeAction(action),
                              child: const Text('执行'),
                            ),
                          );
                        }),
                      ],
                    ],
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}

class _RuntimeJsonSection extends StatelessWidget {
  final String title;
  final dynamic value;

  const _RuntimeJsonSection({required this.title, required this.value});

  @override
  Widget build(BuildContext context) {
    String text;
    try {
      text = const JsonEncoder.withIndent('  ').convert(value);
    } catch (_) {
      text = value.toString();
    }
    return Padding(
      padding: const EdgeInsets.only(bottom: 14),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.cardTitle(context)),
          const SizedBox(height: 6),
          Container(
            width: double.infinity,
            constraints: const BoxConstraints(maxHeight: 180),
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: AppRadius.brSmall),
            child: SingleChildScrollView(child: SelectableText(text, style: const TextStyle(fontFamily: 'monospace', fontSize: 11))),
          ),
        ],
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String label;
  final IconData icon;

  const _SectionLabel({required this.label, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(icon, size: 14, color: context.textTertiary),
        const SizedBox(width: 6),
        Text(label, style: AppTypography.caption(context).copyWith(fontWeight: FontWeight.w600)),
      ],
    );
  }
}

class _ListItem extends StatelessWidget {
  final String text;

  const _ListItem({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 4),
      child: Container(
        width: double.infinity,
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
        ),
        child: Text(text, style: AppTypography.label(context).copyWith(fontFamily: 'monospace')),
      ),
    );
  }
}

class _DisableImpactDialog extends StatelessWidget {
  final String pluginName;
  final int hooksCount;
  final int eventsCount;
  final int schedulesCount;
  final int registeredSkillsCount;
  final VoidCallback onConfirm;

  const _DisableImpactDialog({
    required this.pluginName,
    required this.hooksCount,
    required this.eventsCount,
    required this.schedulesCount,
    required this.registeredSkillsCount,
    required this.onConfirm,
  });

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Row(
        children: [
          Icon(Icons.warning_amber_rounded, color: context.warning, size: 24),
          const SizedBox(width: 8),
          Text('禁用影响确认', style: AppTypography.cardTitle(context)),
        ],
      ),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('禁用「$pluginName」将影响以下功能：', style: AppTypography.bodySmall(context)),
          const SizedBox(height: 12),
          if (hooksCount > 0)
            _ImpactItem(text: '$hooksCount 个 Hook 将停止响应', color: context.accentPrimary),
          if (eventsCount > 0)
            _ImpactItem(text: '$eventsCount 个事件将不再触发', color: context.info),
          if (schedulesCount > 0)
            _ImpactItem(text: '$schedulesCount 个调度任务将暂停', color: context.warning),
          if (registeredSkillsCount > 0)
            _ImpactItem(text: '$registeredSkillsCount 个注册技能将不可用', color: context.error),
          if (hooksCount == 0 && eventsCount == 0 && schedulesCount == 0 && registeredSkillsCount == 0)
            _ImpactItem(text: '无直接影响，可安全禁用', color: context.success),
          const SizedBox(height: 12),
          Container(
            padding: const EdgeInsets.all(10),
            decoration: BoxDecoration(
              color: context.warning.withValues(alpha: 0.08),
              borderRadius: AppRadius.brSmall,
            ),
            child: Row(
              children: [
                Icon(Icons.info_outline, size: 16, color: context.warning),
                const SizedBox(width: 8),
                Expanded(child: Text('禁用后相关功能将立即失效，请谨慎操作。', style: AppTypography.label(context).copyWith(color: context.warning))),
              ],
            ),
          ),
        ],
      ),
      actions: [
        TextButton(onPressed: () => Navigator.pop(context), child: Text('取消', style: TextStyle(color: context.textSecondary))),
        TextButton(onPressed: onConfirm, child: Text('确认禁用', style: TextStyle(color: context.error))),
      ],
    );
  }
}

class _ImpactItem extends StatelessWidget {
  final String text;
  final Color color;

  const _ImpactItem({required this.text, required this.color});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 6),
      child: Row(
        children: [
          Icon(Icons.arrow_right, size: 18, color: color),
          const SizedBox(width: 4),
          Expanded(child: Text(text, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}

class _PermissionSettingsDialog extends ConsumerStatefulWidget {
  final String pluginId;
  final String pluginName;
  final String characterId;
  final String characterName;

  const _PermissionSettingsDialog({
    required this.pluginId,
    required this.pluginName,
    required this.characterId,
    required this.characterName,
  });

  @override
  ConsumerState<_PermissionSettingsDialog> createState() => _PermissionSettingsDialogState();
}

class _PermissionSettingsDialogState extends ConsumerState<_PermissionSettingsDialog> {
  List<Map<String, dynamic>> _grants = const [];
  bool _loading = true;
  bool _saving = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    try {
      final items = await ref.read(extensionServiceProvider).getPluginPermissions(
            widget.pluginId,
            characterId: widget.characterId,
          );
      if (mounted) {
        setState(() {
          _grants = items
              .map((item) => Map<String, dynamic>.from(item))
              .toList(growable: true);
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  bool _allowed(Map<String, dynamic> grant) {
    final decision = (grant['decision'] ?? '').toString();
    return decision == 'allow_always' ||
        decision == 'allow_session' ||
        decision == 'allow_once' ||
        decision == 'allow';
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    try {
      final grants = _grants.map((grant) {
        return <String, dynamic>{
          'capability': (grant['capability'] ?? '').toString(),
          'decision': (grant['decision'] ?? 'deny').toString(),
          'scopeType': 'character',
          'scopeId': widget.characterId,
          if ((grant['expiresAt'] ?? '').toString().isNotEmpty)
            'expiresAt': grant['expiresAt'].toString(),
        };
      }).where((grant) => (grant['capability'] ?? '').toString().isNotEmpty).toList();

      await ref.read(extensionServiceProvider).updatePluginPermissions(
            widget.pluginId,
            characterId: widget.characterId,
            grants: grants,
          );
      if (!mounted) return;
      Navigator.pop(context);
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('${widget.pluginName} 权限已更新')),
      );
    } catch (e) {
      if (mounted) {
        setState(() => _saving = false);
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('保存权限失败: $e'), backgroundColor: context.error),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: context.surfacePrimary,
      shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
      title: Text('${widget.pluginName} - 权限', style: AppTypography.cardTitle(context)),
      content: SizedBox(
        width: double.maxFinite,
        child: _loading
            ? const Padding(
                padding: EdgeInsets.all(24),
                child: Center(child: CircularProgressIndicator()),
              )
            : _error != null
                ? Text(
                    '读取权限失败：$_error',
                    style: AppTypography.bodySmall(context).copyWith(color: context.error),
                  )
                : _grants.isEmpty
                    ? Text(
                        '当前角色「${widget.characterName}」没有已声明的插件权限。',
                        style: AppTypography.bodySmall(context),
                      )
                    : Column(
                        mainAxisSize: MainAxisSize.min,
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '作用角色：${widget.characterName}',
                            style: AppTypography.caption(context),
                          ),
                          const SizedBox(height: 8),
                          Flexible(
                            child: ListView.separated(
                              shrinkWrap: true,
                              itemCount: _grants.length,
                              separatorBuilder: (_, _) => const Divider(height: 1),
                              itemBuilder: (context, index) {
                                final grant = _grants[index];
                                final capability = (grant['capability'] ?? '').toString();
                                final description = (grant['description'] ?? '').toString();
                                final risk = (grant['risk'] ?? '').toString();
                                return SwitchListTile(
                                  contentPadding: EdgeInsets.zero,
                                  title: Text(
                                    capability,
                                    style: AppTypography.bodySmall(context).copyWith(
                                      fontWeight: FontWeight.w600,
                                    ),
                                  ),
                                  subtitle: Text(
                                    [
                                      if (description.isNotEmpty) description,
                                      if (risk.isNotEmpty) '风险：$risk',
                                    ].join('\n'),
                                    style: AppTypography.caption(context),
                                  ),
                                  value: _allowed(grant),
                                  onChanged: (value) => setState(() {
                                    _grants[index] = {
                                      ...grant,
                                      'decision': value ? 'allow_always' : 'deny',
                                      'scopeType': 'character',
                                      'scopeId': widget.characterId,
                                    };
                                  }),
                                );
                              },
                            ),
                          ),
                        ],
                      ),
      ),
      actions: [
        TextButton(
          onPressed: _saving ? null : () => Navigator.pop(context),
          child: Text('取消', style: TextStyle(color: context.textSecondary)),
        ),
        TextButton(
          onPressed: _loading || _error != null || _saving ? null : _save,
          child: Text(
            _saving ? '保存中...' : '保存',
            style: TextStyle(color: context.accentPrimary),
          ),
        ),
      ],
    );
  }
}
