import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../presentation/providers/agent_tasks_provider.dart';

class AgentPage extends ConsumerStatefulWidget {
  const AgentPage({super.key});

  @override
  ConsumerState<AgentPage> createState() => _AgentPageState();
}

class _AgentPageState extends ConsumerState<AgentPage> {
  int _selectedSegment = 0;

  List<AgentTaskItem> _filterTasks(List<AgentTaskItem> tasks) {
    switch (_selectedSegment) {
      case 0:
        return tasks.where((task) => task.isActive && !task.needsAttention).toList();
      case 1:
        return tasks.where((task) => task.needsAttention).toList();
      case 2:
        return tasks.where((task) => task.isTerminal && !task.needsAttention).toList();
      default:
        return const [];
    }
  }

  Future<void> _togglePause(AgentTaskItem task) async {
    try {
      if (task.canPause) {
        await ref.read(agentTasksProvider.notifier).pause(task.id);
        if (mounted) amitiaSnackBar(context, '任务已真实暂停');
      } else if (task.canResume) {
        await ref.read(agentTasksProvider.notifier).resume(task.id);
        if (mounted) amitiaSnackBar(context, '任务已真实继续');
      }
    } catch (e) {
      if (mounted) amitiaSnackBar(context, '操作失败：$e');
    }
  }

  Future<void> _startTask(AgentTaskItem task) async {
    if (!task.canRetry) {
      amitiaSnackBar(context, '当前 Kernel Task 状态不支持重试');
      return;
    }
    try {
      await ref.read(agentTasksProvider.notifier).retry(task.id);
      if (mounted) amitiaSnackBar(context, '任务已通过真实 Retry 接口重新入队');
    } catch (e) {
      if (mounted) amitiaSnackBar(context, '操作失败：$e');
    }
  }

  void _showCreateTaskSheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return _CreateTaskSheet(
          onCreate: (taskDefinitionId, title, desc, abilities, stepCount, deviceId) async {
            await ref.read(agentTasksProvider.notifier).createTask(
              taskDefinitionId: taskDefinitionId,
              title: title,
              description: desc,
              abilities: abilities,
              stepCount: stepCount,
              deviceId: deviceId,
            );
            if (sheetCtx.mounted) Navigator.pop(sheetCtx);
            if (mounted) amitiaSnackBar(context, '任务已提交到选定的 TaskDefinition');
          },
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final tasksAsync = ref.watch(agentTasksProvider);
    final filtered = tasksAsync.when(
      loading: () => <AgentTaskItem>[],
      error: (_, __) => <AgentTaskItem>[],
      data: (tasks) => _filterTasks(tasks),
    );

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Agent',
        centerTitle: true,
        navigation: AmitiaAppBarNavigation.back,
        actions: [
          AmitiaIconButton(
            icon: Icons.add_task_outlined,
            onPressed: _showCreateTaskSheet,
            tooltip: '新建任务',
          ),
        ],
      ),
      body: Column(
        children: [
          Padding(
            padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.md),
            child: AmitiaSegmentedControl(
              segments: const ['进行中', '需处理', '已结束'],
              selectedIndex: _selectedSegment,
              onChanged: (i) => setState(() => _selectedSegment = i),
            ),
          ),
          Expanded(
            child: tasksAsync.when(
              loading: () => const Center(child: CircularProgressIndicator()),
              error: (err, _) => Center(child: Text('加载失败: $err', style: AppTypography.bodySmall(context))),
              data: (tasks) {
                final items = _filterTasks(tasks);
                return items.isEmpty
                    ? AmitiaEmptyState(
                        icon: _selectedSegment == 0
                            ? Icons.auto_awesome
                            : _selectedSegment == 1
                                ? Icons.pending_actions
                                : Icons.task_alt,
                        title: _selectedSegment == 0
                            ? '暂无进行中的任务'
                            : _selectedSegment == 1
                                ? '暂无需要处理的任务'
                                : '暂无已完成的任务',
                      )
                    : ListView.builder(
                        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                        itemCount: items.length,
                        itemBuilder: (context, index) {
                          final task = items[index];
                          return _TaskCard(
                            task: task,
                            onTap: () => context.push(AppRoutes.agentTask(task.id)),
                            onTogglePause: () => _togglePause(task),
                            onStart: () => _startTask(task),
                          );
                        },
                      );
              },
            ),
          ),
        ],
      ),
    );
  }
}

class _TaskCard extends StatelessWidget {
  final AgentTaskItem task;
  final VoidCallback onTap;
  final VoidCallback onTogglePause;
  final VoidCallback onStart;

  const _TaskCard({
    required this.task,
    required this.onTap,
    required this.onTogglePause,
    required this.onStart,
  });

  String _statusLabel(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.created: return '已创建';
      case AgentTaskStatus.queued: return '排队中';
      case AgentTaskStatus.starting: return '启动中';
      case AgentTaskStatus.running: return '运行中';
      case AgentTaskStatus.checkpointing: return '检查点保存中';
      case AgentTaskStatus.pausing: return '暂停中';
      case AgentTaskStatus.paused: return '已暂停';
      case AgentTaskStatus.resuming: return '恢复中';
      case AgentTaskStatus.cancelling: return '取消中';
      case AgentTaskStatus.cancelled: return '已取消';
      case AgentTaskStatus.succeeded: return '已成功';
      case AgentTaskStatus.failed: return '已失败';
      case AgentTaskStatus.timedOut: return '已超时';
      case AgentTaskStatus.recoveryRequired: return '需恢复';
      case AgentTaskStatus.manualIntervention: return '需人工干预';
    }
  }

  BadgeType _badgeType(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.succeeded:
        return BadgeType.success;
      case AgentTaskStatus.failed:
      case AgentTaskStatus.timedOut:
      case AgentTaskStatus.manualIntervention:
        return BadgeType.error;
      case AgentTaskStatus.starting:
      case AgentTaskStatus.running:
      case AgentTaskStatus.checkpointing:
      case AgentTaskStatus.pausing:
      case AgentTaskStatus.resuming:
      case AgentTaskStatus.recoveryRequired:
        return BadgeType.warning;
      case AgentTaskStatus.created:
      case AgentTaskStatus.queued:
      case AgentTaskStatus.paused:
      case AgentTaskStatus.cancelling:
      case AgentTaskStatus.cancelled:
        return BadgeType.neutral;
    }
  }

  IconData _icon(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.succeeded:
        return Icons.check_circle_outline;
      case AgentTaskStatus.failed:
      case AgentTaskStatus.timedOut:
      case AgentTaskStatus.manualIntervention:
        return Icons.error_outline;
      case AgentTaskStatus.paused:
      case AgentTaskStatus.pausing:
        return Icons.pause_circle_outline;
      case AgentTaskStatus.cancelled:
      case AgentTaskStatus.cancelling:
        return Icons.cancel_outlined;
      case AgentTaskStatus.recoveryRequired:
        return Icons.settings_backup_restore;
      case AgentTaskStatus.running:
      case AgentTaskStatus.checkpointing:
      case AgentTaskStatus.resuming:
        return Icons.auto_awesome;
      case AgentTaskStatus.created:
      case AgentTaskStatus.queued:
      case AgentTaskStatus.starting:
        return Icons.schedule;
    }
  }

  Color _iconColor(BuildContext context, AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.succeeded:
        return context.success;
      case AgentTaskStatus.failed:
      case AgentTaskStatus.timedOut:
      case AgentTaskStatus.manualIntervention:
        return context.error;
      case AgentTaskStatus.running:
      case AgentTaskStatus.checkpointing:
      case AgentTaskStatus.resuming:
        return context.accentPrimary;
      case AgentTaskStatus.starting:
      case AgentTaskStatus.recoveryRequired:
        return context.warning;
      case AgentTaskStatus.created:
      case AgentTaskStatus.queued:
      case AgentTaskStatus.pausing:
      case AgentTaskStatus.paused:
      case AgentTaskStatus.cancelling:
      case AgentTaskStatus.cancelled:
        return context.textTertiary;
    }
  }

  String get _subtitle {
    if ((task.error ?? '').trim().isNotEmpty &&
        const <AgentTaskStatus>{
          AgentTaskStatus.failed,
          AgentTaskStatus.timedOut,
          AgentTaskStatus.manualIntervention,
        }.contains(task.status)) {
      return task.error!;
    }
    final attempt = task.maxAttempts > 0 ? ' · 尝试 ${task.attempt}/${task.maxAttempts}' : '';
    final placement = task.executionPlacement.trim().isNotEmpty ? ' · ${task.executionPlacement}' : '';
    return '${_statusLabel(task.status)}$attempt$placement';
  }

  @override
  Widget build(BuildContext context) {
    final progress = task.progress;
    final iconColor = _iconColor(context, task.status);
    return Container(
      margin: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: iconColor.withValues(alpha: 0.12),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(_icon(task.status), size: 18, color: iconColor),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(task.title, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(_subtitle, style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: _statusLabel(task.status), type: _badgeType(task.status)),
              ],
            ),
            if (progress != null) ...[
              SizedBox(height: AppSpacing.md),
              AmitiaProgressBar(progress: progress / 100),
              SizedBox(height: AppSpacing.sm),
              Text('${progress.toStringAsFixed(progress % 1 == 0 ? 0 : 1)}%', style: AppTypography.label(context)),
            ],
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                if (task.requiredAbilities.isNotEmpty)
                  Expanded(
                    child: Wrap(
                      spacing: 6,
                      runSpacing: 6,
                      children: task.requiredAbilities.take(3).map((ability) => Container(
                        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: task.needsAttention ? context.warning.withValues(alpha: 0.08) : context.accentSoft,
                          borderRadius: AppRadius.brTag,
                        ),
                        child: Text(
                          ability,
                          style: TextStyle(fontSize: 11, color: task.needsAttention ? context.warning : context.accentPrimary),
                        ),
                      )).toList(growable: false),
                    ),
                  )
                else
                  Text('已运行 ${task.elapsed}', style: AppTypography.label(context)),
                const SizedBox(width: 8),
                _buildAction(context),
              ],
            ),
            if (task.status == AgentTaskStatus.succeeded && task.result != null) ...[
              SizedBox(height: AppSpacing.sm),
              Text(task.result!, style: AppTypography.caption(context)),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildAction(BuildContext context) {
    if (task.canPause) return _miniButton(context, '暂停', Icons.pause, onTogglePause);
    if (task.canResume) return _miniButton(context, '继续', Icons.play_arrow, onTogglePause);
    if (task.canRecover) return _miniButton(context, '恢复', Icons.settings_backup_restore, onTap, accent: true);
    if (task.status == AgentTaskStatus.manualIntervention) {
      return _miniButton(context, '查看处理', Icons.build_outlined, onTap, accent: true);
    }
    if (task.canRetry) return _miniButton(context, '再次执行', Icons.refresh, onStart, accent: true);
    return _miniButton(context, '查看任务', Icons.visibility_outlined, onTap);
  }

  Widget _miniButton(BuildContext context, String label, IconData icon, VoidCallback onTap, {bool accent = false}) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 5),
        decoration: BoxDecoration(
          color: accent ? context.accentPrimary : context.surfaceSecondary,
          borderRadius: AppRadius.brTag,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(icon, size: 14, color: accent ? Colors.white : context.textSecondary),
            const SizedBox(width: 4),
            Text(label, style: TextStyle(fontSize: 12, color: accent ? Colors.white : context.textSecondary)),
          ],
        ),
      ),
    );
  }
}

const List<String> _abilityOptions = [
  '文件系统',
  'Web 搜索',
  '代码执行',
  '数据分析',
  '文本生成',
  '系统操作',
  '数据库',
  '通知读取',
];

const Map<String, List<String>> _quickTaskAbilities = {
  '控制手机': ['系统操作', '通知读取'],
  '处理文件': ['文件系统'],
  '打开工作区': ['文件系统', '系统操作'],
  '新建工作流': ['代码执行', '系统操作'],
  '数据分析': ['数据分析', '文本生成'],
  '信息搜索': ['Web 搜索'],
};

class _CreateTaskSheet extends ConsumerStatefulWidget {
  final Future<void> Function(
    String taskDefinitionId,
    String title,
    String description,
    List<String> abilities,
    int stepCount,
    String? deviceId,
  ) onCreate;

  const _CreateTaskSheet({required this.onCreate});

  @override
  ConsumerState<_CreateTaskSheet> createState() => _CreateTaskSheetState();
}

class _CreateTaskSheetState extends ConsumerState<_CreateTaskSheet> {
  final _titleCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final List<String> _abilities = [];
  int _stepCount = 3;
  String? _taskDefinitionId;
  String _taskExecutionPlacement = '';
  String? _deviceId;
  bool _submitting = false;
  _QuickTask? _selectedQuickTask;

  final _quickTasks = [
    _QuickTask(title: '控制手机', icon: Icons.phone_android, category: '设备'),
    _QuickTask(title: '处理文件', icon: Icons.folder_outlined, category: '文件'),
    _QuickTask(title: '打开工作区', icon: Icons.work_outline, category: '工作'),
    _QuickTask(title: '新建工作流', icon: Icons.account_tree_outlined, category: '自动化'),
    _QuickTask(title: '数据分析', icon: Icons.analytics_outlined, category: '分析'),
    _QuickTask(title: '信息搜索', icon: Icons.search, category: '搜索'),
  ];

  @override
  void dispose() {
    _titleCtrl.dispose();
    _descCtrl.dispose();
    super.dispose();
  }

  void _toggleAbility(String a) {
    setState(() {
      if (_abilities.contains(a)) {
        _abilities.remove(a);
      } else {
        _abilities.add(a);
      }
    });
  }

  void _selectQuickTask(_QuickTask task) {
    final abilities = _quickTaskAbilities[task.title] ?? ['文件系统'];
    setState(() {
      _selectedQuickTask = task;
      _titleCtrl.text = task.title;
      _descCtrl.text = '执行${task.title}任务';
      _abilities.clear();
      _abilities.addAll(abilities);
      _stepCount = 3;
    });
  }

  void _clearQuickTask() {
    setState(() {
      _selectedQuickTask = null;
      _titleCtrl.clear();
      _descCtrl.clear();
      _abilities.clear();
      _stepCount = 3;
    });
  }

  Future<void> _submit() async {
    if (_submitting) return;
    final title = _titleCtrl.text.trim();
    if (title.isEmpty) {
      amitiaSnackBar(context, '请输入任务名称');
      return;
    }
    if ((_taskDefinitionId ?? '').isEmpty) {
      amitiaSnackBar(context, '请选择实际执行的 Kernel Task 定义');
      return;
    }
    if (_taskExecutionPlacement == 'device' && (_deviceId ?? '').isEmpty) {
      amitiaSnackBar(context, '设备任务需要选择在线设备');
      return;
    }
    final desc = _descCtrl.text.trim();
    final abilities = _abilities.isEmpty ? ['文件系统'] : List<String>.from(_abilities);
    setState(() => _submitting = true);
    try {
      await widget.onCreate(
        _taskDefinitionId!,
        title,
        desc,
        abilities,
        _stepCount,
        _taskExecutionPlacement == 'device' ? _deviceId : null,
      );
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '任务创建失败：$error');
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  Widget _buildQuickTaskSelector() {
    if (_selectedQuickTask != null) {
      final task = _selectedQuickTask!;
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
        decoration: BoxDecoration(
          color: context.accentSoft,
          borderRadius: AppRadius.brMedium,
        ),
        child: Row(
          children: [
            Icon(task.icon, size: 20, color: context.accentPrimary),
            const SizedBox(width: 10),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(task.title, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600, color: context.accentPrimary)),
                  Text(task.category, style: AppTypography.label(context)),
                ],
              ),
            ),
            GestureDetector(
              onTap: _clearQuickTask,
              child: Icon(Icons.close, size: 18, color: context.textSecondary),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('快捷任务', style: AppTypography.sectionTitle(context)),
        const SizedBox(height: 8),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 2,
            mainAxisSpacing: 8,
            crossAxisSpacing: 8,
            childAspectRatio: 2.5,
          ),
          itemCount: _quickTasks.length,
          itemBuilder: (context, index) {
            final task = _quickTasks[index];
            return GestureDetector(
              onTap: () => _selectQuickTask(task),
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                decoration: BoxDecoration(
                  color: context.surfaceSecondary,
                  borderRadius: AppRadius.brMedium,
                ),
                child: Row(
                  children: [
                    Container(
                      width: 32,
                      height: 32,
                      decoration: BoxDecoration(
                        color: context.accentSoft,
                        borderRadius: AppRadius.brSmall,
                      ),
                      child: Icon(task.icon, size: 18, color: context.accentPrimary),
                    ),
                    const SizedBox(width: 10),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        mainAxisAlignment: MainAxisAlignment.center,
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Flexible(
                            child: Text(
                              task.title,
                              style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w500),
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                            ),
                          ),
                          Text(task.category, style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            );
          },
        ),
      ],
    );
  }

  @override
  Widget build(BuildContext context) {
    final definitionsAsync = ref.watch(agentTaskDefinitionsProvider);
    final deviceTaskSelected = _taskExecutionPlacement == 'device';
    final AsyncValue<List<AgentTaskDeviceOption>> devicesAsync = deviceTaskSelected
        ? ref.watch(agentTaskDevicesProvider)
        : const AsyncData<List<AgentTaskDeviceOption>>(<AgentTaskDeviceOption>[]);
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(20, 0, 20, 20).copyWith(bottom: MediaQuery.viewInsetsOf(context).bottom + 20),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SizedBox(height: 8),
              const SizedBox(height: 16),
              Text('新建任务', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 16),
              _buildQuickTaskSelector(),
              const SizedBox(height: 16),
              Text('执行定义', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 6),
              definitionsAsync.when(
                loading: () => const LinearProgressIndicator(),
                error: (error, _) => Text('任务定义加载失败：$error', style: AppTypography.caption(context).copyWith(color: context.error)),
                data: (definitions) {
                  if (definitions.isEmpty) {
                    return Text('当前没有可执行的 Kernel Task 定义', style: AppTypography.caption(context).copyWith(color: context.warning));
                  }
                  final selectedDefinitionID = _taskDefinitionId;
                  final selectedDefinitionIsValid = selectedDefinitionID == null ||
                      definitions.any((item) => item.taskId == selectedDefinitionID);
                  final dropdownDefinitionID = selectedDefinitionIsValid ? selectedDefinitionID : null;
                  if (!selectedDefinitionIsValid) {
                    WidgetsBinding.instance.addPostFrameCallback((_) {
                      if (!mounted) return;
                      final currentID = _taskDefinitionId;
                      if (currentID != null && !definitions.any((item) => item.taskId == currentID)) {
                        setState(() {
                          _taskDefinitionId = null;
                          _taskExecutionPlacement = '';
                          _deviceId = null;
                        });
                      }
                    });
                  }
                  return DropdownButtonFormField<String>(
                    value: dropdownDefinitionID,
                    isExpanded: true,
                    decoration: const InputDecoration(hintText: '选择真正要执行的 TaskDefinition'),
                    items: definitions
                        .map((item) => DropdownMenuItem<String>(
                              value: item.taskId,
                              child: Text(item.label, maxLines: 1, overflow: TextOverflow.ellipsis),
                            ))
                        .toList(growable: false),
                    onChanged: (value) {
                      AgentTaskDefinitionOption? selected;
                      for (final item in definitions) {
                        if (item.taskId == value) {
                          selected = item;
                          break;
                        }
                      }
                      setState(() {
                        _taskDefinitionId = value;
                        _taskExecutionPlacement = selected?.executionPlacement ?? '';
                        _deviceId = null;
                      });
                    },
                  );
                },
              ),
              if (deviceTaskSelected) ...[
                const SizedBox(height: 12),
                Text('执行设备', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
                const SizedBox(height: 6),
                devicesAsync.when(
                  loading: () => const LinearProgressIndicator(),
                  error: (error, _) => Text(
                    '在线设备加载失败：$error',
                    style: AppTypography.caption(context).copyWith(color: context.error),
                  ),
                  data: (devices) {
                    if (devices.isEmpty) {
                      return Text(
                        '当前没有在线设备，设备任务无法提交。',
                        style: AppTypography.caption(context).copyWith(color: context.warning),
                      );
                    }
                    final selectedDeviceID = _deviceId;
                    final selectedDeviceIsValid = selectedDeviceID == null ||
                        devices.any((item) => item.deviceId == selectedDeviceID);
                    String? dropdownDeviceID = selectedDeviceIsValid ? selectedDeviceID : null;
                    if (!selectedDeviceIsValid || (dropdownDeviceID == null && devices.length == 1)) {
                      final nextDeviceID = devices.length == 1 ? devices.first.deviceId : null;
                      dropdownDeviceID = nextDeviceID;
                      WidgetsBinding.instance.addPostFrameCallback((_) {
                        if (!mounted) return;
                        final currentID = _deviceId;
                        final currentIsValid = currentID == null || devices.any((item) => item.deviceId == currentID);
                        if (!currentIsValid || (currentID == null && nextDeviceID != null)) {
                          setState(() => _deviceId = nextDeviceID);
                        }
                      });
                    }
                    return DropdownButtonFormField<String>(
                      value: dropdownDeviceID,
                      isExpanded: true,
                      decoration: const InputDecoration(hintText: '选择在线设备'),
                      items: devices
                          .map((item) => DropdownMenuItem<String>(
                                value: item.deviceId,
                                child: Text(item.displayLabel, maxLines: 1, overflow: TextOverflow.ellipsis),
                              ))
                          .toList(growable: false),
                      onChanged: (value) => setState(() => _deviceId = value),
                    );
                  },
                ),
              ],
              const SizedBox(height: 16),
              Text('任务名称', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '输入任务名称', controller: _titleCtrl),
              const SizedBox(height: 14),
              Text('任务说明', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 6),
              AmitiaTextField(hintText: '描述任务目标', controller: _descCtrl, maxLines: 3),
              const SizedBox(height: 14),
              Text('所需能力', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: _abilityOptions.map((a) {
                  final selected = _abilities.contains(a);
                  return GestureDetector(
                    onTap: () => _toggleAbility(a),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 7),
                      decoration: BoxDecoration(
                        color: selected ? context.accentSoft : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                        border: selected ? null : Border.all(color: context.borderPrimary, width: 1),
                      ),
                      child: Text(
                        a,
                        style: TextStyle(
                          fontSize: 13,
                          color: selected ? context.accentPrimary : context.textSecondary,
                          fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                        ),
                      ),
                    ),
                  );
                }).toList(),
              ),
              const SizedBox(height: 14),
              Text('预计步骤数', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
              const SizedBox(height: 8),
              Row(
                children: List.generate(5, (i) {
                  final n = i + 2;
                  final selected = n == _stepCount;
                  return Expanded(
                    child: GestureDetector(
                      onTap: () => setState(() => _stepCount = n),
                      child: Container(
                        margin: const EdgeInsets.only(right: 8),
                        padding: const EdgeInsets.symmetric(vertical: 9),
                        decoration: BoxDecoration(
                          color: selected ? context.accentPrimary : context.surfaceSecondary,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: Center(
                          child: Text(
                            '$n',
                            style: TextStyle(
                              color: selected ? Colors.white : context.textSecondary,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                    ),
                  );
                }),
              ),
              const SizedBox(height: 24),
              AmitiaButton(
                label: _submitting ? '提交中…' : '开始任务',
                icon: Icons.play_arrow,
                isFullWidth: true,
                onPressed: _submitting ? null : _submit,
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _QuickTask {
  final String title;
  final IconData icon;
  final String category;
  const _QuickTask({required this.title, required this.icon, required this.category});
}
