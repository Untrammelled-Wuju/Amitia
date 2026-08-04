import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../shared/mock_data/mock_data.dart';
import '../../../../shared/models/models.dart';
import '../../presentation/providers/mock_agent_tasks.dart';

class AgentPage extends ConsumerStatefulWidget {
  const AgentPage({super.key});

  @override
  ConsumerState<AgentPage> createState() => _AgentPageState();
}

class _AgentPageState extends ConsumerState<AgentPage> {
  int _selectedSegment = 0;
  final Map<String, Timer> _timers = {};
  final Map<String, int> _elapsedSeconds = {};

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      _syncTimers(ref.read(agentTasksProvider));
    });
  }

  @override
  void dispose() {
    for (final t in _timers.values) {
      t.cancel();
    }
    _timers.clear();
    super.dispose();
  }

  int _parseElapsed(String e) {
    final parts = e.split(':');
    if (parts.length == 2) {
      return (int.tryParse(parts[0]) ?? 0) * 60 + (int.tryParse(parts[1]) ?? 0);
    }
    return 0;
  }

  String _formatElapsed(int s) {
    final m = (s ~/ 60).toString().padLeft(2, '0');
    final ss = (s % 60).toString().padLeft(2, '0');
    return '$m:$ss';
  }

  void _syncTimers(List<MockAgentTask> tasks) {
    final runningIds = tasks.where((t) => t.status == MockAgentTaskStatus.running).map((t) => t.id).toSet();
    for (final id in _timers.keys.toList()) {
      if (!runningIds.contains(id)) {
        _timers[id]?.cancel();
        _timers.remove(id);
      }
    }
    for (final task in tasks) {
      if (task.status == MockAgentTaskStatus.running && !_timers.containsKey(task.id)) {
        _elapsedSeconds.putIfAbsent(task.id, () => _parseElapsed(task.elapsed));
        _timers[task.id] = Timer.periodic(const Duration(seconds: 1), (_) => _tick(task.id));
      }
    }
  }

  void _tick(String taskId) {
    if (!mounted) return;
    final tasks = ref.read(agentTasksProvider);
    final idx = tasks.indexWhere((t) => t.id == taskId);
    if (idx == -1) {
      _timers[taskId]?.cancel();
      _timers.remove(taskId);
      return;
    }
    final task = tasks[idx];
    if (task.status != MockAgentTaskStatus.running) {
      _timers[taskId]?.cancel();
      _timers.remove(taskId);
      return;
    }
    final secs = (_elapsedSeconds[taskId] ?? 0) + 1;
    _elapsedSeconds[taskId] = secs;
    var progress = task.progress + 4;
    var stepIndex = task.currentStepIndex;
    final stepThreshold = ((stepIndex + 1) / task.steps.length * 100).round();
    if (progress >= stepThreshold && stepIndex < task.steps.length - 1) stepIndex++;
    MockAgentTask updated;
    if (progress >= 100) {
      updated = task.copyWith(
        progress: 100,
        currentStepIndex: task.steps.length - 1,
        elapsed: _formatElapsed(secs),
        status: MockAgentTaskStatus.completed,
        result: '任务已完成，共执行 ${task.steps.length} 个步骤',
      );
    } else {
      updated = task.copyWith(
        progress: progress,
        currentStepIndex: stepIndex,
        elapsed: _formatElapsed(secs),
      );
    }
    final next = List<MockAgentTask>.from(tasks);
    next[idx] = updated;
    ref.read(agentTasksProvider.notifier).state = next;
  }

  void _togglePause(MockAgentTask task) {
    final tasks = ref.read(agentTasksProvider);
    final idx = tasks.indexWhere((t) => t.id == task.id);
    if (idx == -1) return;
    final next = List<MockAgentTask>.from(tasks);
    next[idx] = task.copyWith(
      status: task.status == MockAgentTaskStatus.running ? MockAgentTaskStatus.paused : MockAgentTaskStatus.running,
    );
    ref.read(agentTasksProvider.notifier).state = next;
    _syncTimers(next);
    amitiaSnackBar(context, task.status == MockAgentTaskStatus.running ? '任务已暂停' : '任务已继续');
  }

  void _startTask(MockAgentTask task) {
    final tasks = ref.read(agentTasksProvider);
    final idx = tasks.indexWhere((t) => t.id == task.id);
    if (idx == -1) return;
    final next = List<MockAgentTask>.from(tasks);
    next[idx] = task.copyWith(
      status: MockAgentTaskStatus.running,
      progress: task.status == MockAgentTaskStatus.cancelled || task.status == MockAgentTaskStatus.failed ? 0 : task.progress,
    );
    ref.read(agentTasksProvider.notifier).state = next;
    _syncTimers(next);
    amitiaSnackBar(context, '任务已开始');
  }

  void _createTask(String title, String description, List<String> abilities, int stepCount) {
    final id = 't${DateTime.now().millisecondsSinceEpoch}';
    final steps = List.generate(stepCount, (i) => '步骤 ${i + 1}：执行子任务');
    final newTask = MockAgentTask(
      id: id,
      title: title,
      description: description,
      requiredAbilities: abilities,
      steps: steps,
      status: MockAgentTaskStatus.running,
      progress: 0,
      currentStepIndex: 0,
      elapsed: '00:00',
      createdAt: DateTime.now(),
    );
    final tasks = ref.read(agentTasksProvider);
    final next = [newTask, ...tasks];
    ref.read(agentTasksProvider.notifier).state = next;
    _elapsedSeconds[id] = 0;
    _syncTimers(next);
    context.push(AppRoutes.agentTask(id));
  }

  void _showCreateTaskSheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return _CreateTaskSheet(onCreate: (title, desc, abilities, stepCount) {
          Navigator.pop(sheetCtx);
          _createTask(title, desc, abilities, stepCount);
        });
      },
    );
  }

  List<MockAgentTask> _filterTasks(List<MockAgentTask> tasks) {
    switch (_selectedSegment) {
      case 0:
        return tasks.where((t) => t.status == MockAgentTaskStatus.running || t.status == MockAgentTaskStatus.paused).toList();
      case 1:
        return tasks.where((t) => t.status == MockAgentTaskStatus.waitingApproval || t.status == MockAgentTaskStatus.pending).toList();
      case 2:
        return tasks.where((t) =>
            t.status == MockAgentTaskStatus.completed ||
            t.status == MockAgentTaskStatus.failed ||
            t.status == MockAgentTaskStatus.cancelled).toList();
      default:
        return const [];
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(agentTasksProvider, (_, next) => _syncTimers(next));
    final tasks = ref.watch(agentTasksProvider);
    final filtered = _filterTasks(tasks);

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
            padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.md),
            child: AmitiaSegmentedControl(
              segments: const ['进行中', '等待审批', '已完成'],
              selectedIndex: _selectedSegment,
              onChanged: (i) => setState(() => _selectedSegment = i),
            ),
          ),
          Expanded(
            child: filtered.isEmpty
                ? AmitiaEmptyState(
                    icon: _selectedSegment == 0
                        ? Icons.auto_awesome
                        : _selectedSegment == 1
                            ? Icons.pending_actions
                            : Icons.task_alt,
                    title: _selectedSegment == 0
                        ? '暂无进行中的任务'
                        : _selectedSegment == 1
                            ? '暂无等待审批的任务'
                            : '暂无已完成的任务',
                  )
                : ListView.builder(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                    itemCount: filtered.length,
                    itemBuilder: (context, index) {
                      final task = filtered[index];
                      return _TaskCard(
                        task: task,
                        onTap: () => context.push(AppRoutes.agentTask(task.id)),
                        onTogglePause: () => _togglePause(task),
                        onStart: () => _startTask(task),
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
  final MockAgentTask task;
  final VoidCallback onTap;
  final VoidCallback onTogglePause;
  final VoidCallback onStart;

  const _TaskCard({
    required this.task,
    required this.onTap,
    required this.onTogglePause,
    required this.onStart,
  });

  bool get _showProgress =>
      task.status == MockAgentTaskStatus.running ||
      task.status == MockAgentTaskStatus.paused ||
      task.status == MockAgentTaskStatus.completed ||
      task.status == MockAgentTaskStatus.failed;

  @override
  Widget build(BuildContext context) {
    final statusLabel = mockAgentTaskStatusLabel(task.status);
    final badgeType = mockAgentTaskBadgeType(task.status);
    return Container(
      margin: const EdgeInsets.only(bottom: AppSpacing.sm),
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
                    color: _iconBg(context),
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(_icon, size: 18, color: _iconColor(context)),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(task.title, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(_subtitle, style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: statusLabel, type: badgeType),
              ],
            ),
            if (_showProgress) ...[
              const SizedBox(height: AppSpacing.md),
              AmitiaProgressBar(progress: task.progress / 100),
              const SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  Text('${task.progress}%', style: AppTypography.label(context)),
                  const SizedBox(width: 12),
                  if (task.status == MockAgentTaskStatus.running || task.status == MockAgentTaskStatus.paused)
                    Text('已运行 ${task.elapsed}', style: AppTypography.label(context))
                  else if (task.status == MockAgentTaskStatus.completed)
                    Text('耗时 ${task.elapsed}', style: AppTypography.label(context))
                  else if (task.status == MockAgentTaskStatus.failed)
                    Text('失败 ${task.elapsed}', style: AppTypography.label(context)),
                  const Spacer(),
                  _buildAction(context),
                ],
              ),
            ] else ...[
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  if (task.requiredAbilities.isNotEmpty)
                    Expanded(
                      child: Wrap(
                        spacing: 6,
                        runSpacing: 6,
                        children: task.requiredAbilities.take(3).map((a) => Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                          decoration: BoxDecoration(
                            color: task.status == MockAgentTaskStatus.waitingApproval
                                ? context.warning.withValues(alpha: 0.08)
                                : context.accentSoft,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Text(a, style: TextStyle(fontSize: 11, color: task.status == MockAgentTaskStatus.waitingApproval ? context.warning : context.accentPrimary)),
                        )).toList(),
                      ),
                    )
                  else
                    const Spacer(),
                  _buildAction(context),
                ],
              ),
            ],
            if (task.status == MockAgentTaskStatus.completed && task.result != null) ...[
              const SizedBox(height: AppSpacing.sm),
              Text(task.result!, style: AppTypography.caption(context)),
            ],
            if (task.status == MockAgentTaskStatus.failed && task.error != null) ...[
              const SizedBox(height: AppSpacing.sm),
              Text(task.error!, style: AppTypography.caption(context).copyWith(color: context.error)),
            ],
          ],
        ),
      ),
    );
  }

  String get _subtitle {
    switch (task.status) {
      case MockAgentTaskStatus.running:
        return task.currentStepIndex < task.steps.length ? task.steps[task.currentStepIndex] : task.steps.last;
      case MockAgentTaskStatus.paused:
        return '已暂停 · ${task.currentStepIndex < task.steps.length ? task.steps[task.currentStepIndex] : ''}';
      case MockAgentTaskStatus.waitingApproval:
        return '等待权限审批';
      case MockAgentTaskStatus.pending:
        return '等待开始';
      case MockAgentTaskStatus.completed:
        return '已完成';
      case MockAgentTaskStatus.failed:
        return task.error ?? '执行失败';
      case MockAgentTaskStatus.cancelled:
        return '已取消';
    }
  }

  IconData get _icon {
    switch (task.status) {
      case MockAgentTaskStatus.running:
        return Icons.auto_awesome;
      case MockAgentTaskStatus.waitingApproval:
        return Icons.hourglass_top;
      case MockAgentTaskStatus.pending:
        return Icons.schedule;
      case MockAgentTaskStatus.completed:
        return Icons.check_circle_outline;
      case MockAgentTaskStatus.failed:
        return Icons.error_outline;
      case MockAgentTaskStatus.cancelled:
        return Icons.cancel_outlined;
      case MockAgentTaskStatus.paused:
        return Icons.pause_circle_outline;
    }
  }

  Color _iconColor(BuildContext context) {
    switch (task.status) {
      case MockAgentTaskStatus.running:
        return context.accentPrimary;
      case MockAgentTaskStatus.waitingApproval:
      case MockAgentTaskStatus.pending:
        return context.warning;
      case MockAgentTaskStatus.completed:
        return context.success;
      case MockAgentTaskStatus.failed:
        return context.error;
      case MockAgentTaskStatus.cancelled:
      case MockAgentTaskStatus.paused:
        return context.textTertiary;
    }
  }

  Color _iconBg(BuildContext context) {
    return _iconColor(context).withValues(alpha: 0.12);
  }

  Widget _buildAction(BuildContext context) {
    switch (task.status) {
      case MockAgentTaskStatus.running:
        return _miniButton(context, '暂停', Icons.pause, onTogglePause);
      case MockAgentTaskStatus.paused:
        return _miniButton(context, '继续', Icons.play_arrow, onTogglePause);
      case MockAgentTaskStatus.waitingApproval:
        return _miniButton(context, '审批', Icons.shield_outlined, onTap, accent: true);
      case MockAgentTaskStatus.pending:
        return _miniButton(context, '开始', Icons.play_arrow, onStart, accent: true);
      case MockAgentTaskStatus.completed:
        return _miniButton(context, '查看结果', Icons.visibility_outlined, onTap);
      case MockAgentTaskStatus.failed:
        return _miniButton(context, '查看错误', Icons.error_outline, onTap);
      case MockAgentTaskStatus.cancelled:
        return _miniButton(context, '再次执行', Icons.refresh, onStart, accent: true);
    }
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

class _CreateTaskSheet extends StatefulWidget {
  final void Function(String title, String description, List<String> abilities, int stepCount) onCreate;

  const _CreateTaskSheet({required this.onCreate});

  @override
  State<_CreateTaskSheet> createState() => _CreateTaskSheetState();
}

class _CreateTaskSheetState extends State<_CreateTaskSheet> {
  final _titleCtrl = TextEditingController();
  final _descCtrl = TextEditingController();
  final List<String> _abilities = [];
  int _stepCount = 3;
  QuickTask? _selectedQuickTask;

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

  void _selectQuickTask(QuickTask task) {
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

  void _submit() {
    final title = _titleCtrl.text.trim();
    if (title.isEmpty) {
      amitiaSnackBar(context, '请输入任务名称');
      return;
    }
    final desc = _descCtrl.text.trim();
    final abilities = _abilities.isEmpty ? ['文件系统'] : List<String>.from(_abilities);
    widget.onCreate(title, desc, abilities, _stepCount);
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
          itemCount: MockData.quickTasks.length,
          itemBuilder: (context, index) {
            final task = MockData.quickTasks[index];
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
    return SafeArea(
      child: Padding(
        padding: EdgeInsets.fromLTRB(20, 0, 20, 20).copyWith(bottom: MediaQuery.viewInsetsOf(context).bottom + 20),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const SizedBox(height: 8),
              Center(
                child: Container(
                  width: 40,
                  height: 4,
                  decoration: BoxDecoration(
                    color: context.borderPrimary,
                    borderRadius: BorderRadius.circular(2),
                  ),
                ),
              ),
              const SizedBox(height: 16),
              Text('新建任务', style: AppTypography.pageTitle(context)),
              const SizedBox(height: 16),
              _buildQuickTaskSelector(),
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
                label: '开始任务',
                icon: Icons.play_arrow,
                isFullWidth: true,
                onPressed: _submit,
              ),
            ],
          ),
        ),
      ),
    );
  }
}
