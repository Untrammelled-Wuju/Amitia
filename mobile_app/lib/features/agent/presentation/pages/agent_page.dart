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
        return tasks.where((t) => t.status == AgentTaskStatus.running || t.status == AgentTaskStatus.paused).toList();
      case 1:
        return tasks.where((t) => t.status == AgentTaskStatus.waitingApproval || t.status == AgentTaskStatus.pending).toList();
      case 2:
        return tasks.where((t) =>
            t.status == AgentTaskStatus.completed ||
            t.status == AgentTaskStatus.failed ||
            t.status == AgentTaskStatus.cancelled).toList();
      default:
        return const [];
    }
  }

  void _togglePause(AgentTaskItem task) {
    ref.read(agentTasksProvider.notifier).changeStatus(
      task.id,
      task.status == AgentTaskStatus.running ? AgentTaskStatus.paused : AgentTaskStatus.running,
    );
    amitiaSnackBar(context, task.status == AgentTaskStatus.running ? '任务已暂停' : '任务已继续');
  }

  void _startTask(AgentTaskItem task) {
    ref.read(agentTasksProvider.notifier).changeStatus(task.id, AgentTaskStatus.running);
    amitiaSnackBar(context, '任务已开始');
  }

  void _showCreateTaskSheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetCtx) {
        return _CreateTaskSheet(
          onCreate: (title, desc, abilities, stepCount) {
            Navigator.pop(sheetCtx);
            ref.read(agentTasksProvider.notifier).createTask(
              title: title,
              description: desc,
              abilities: abilities,
              stepCount: stepCount,
            );
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
              segments: const ['进行中', '等待审批', '已完成'],
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
                                ? '暂无等待审批的任务'
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
      case AgentTaskStatus.pending: return '待开始';
      case AgentTaskStatus.waitingApproval: return '待审批';
      case AgentTaskStatus.running: return '运行中';
      case AgentTaskStatus.paused: return '已暂停';
      case AgentTaskStatus.completed: return '已完成';
      case AgentTaskStatus.failed: return '已失败';
      case AgentTaskStatus.cancelled: return '已取消';
    }
  }

  BadgeType _badgeType(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.pending: return BadgeType.neutral;
      case AgentTaskStatus.waitingApproval: return BadgeType.warning;
      case AgentTaskStatus.running: return BadgeType.accent;
      case AgentTaskStatus.paused: return BadgeType.neutral;
      case AgentTaskStatus.completed: return BadgeType.success;
      case AgentTaskStatus.failed: return BadgeType.error;
      case AgentTaskStatus.cancelled: return BadgeType.neutral;
    }
  }

  IconData _icon(AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.running: return Icons.auto_awesome;
      case AgentTaskStatus.waitingApproval: return Icons.hourglass_top;
      case AgentTaskStatus.pending: return Icons.schedule;
      case AgentTaskStatus.completed: return Icons.check_circle_outline;
      case AgentTaskStatus.failed: return Icons.error_outline;
      case AgentTaskStatus.cancelled: return Icons.cancel_outlined;
      case AgentTaskStatus.paused: return Icons.pause_circle_outline;
    }
  }

  Color _iconColor(BuildContext context, AgentTaskStatus status) {
    switch (status) {
      case AgentTaskStatus.running: return context.accentPrimary;
      case AgentTaskStatus.waitingApproval:
      case AgentTaskStatus.pending: return context.warning;
      case AgentTaskStatus.completed: return context.success;
      case AgentTaskStatus.failed: return context.error;
      case AgentTaskStatus.cancelled:
      case AgentTaskStatus.paused: return context.textTertiary;
    }
  }

  bool get _showProgress =>
      task.status == AgentTaskStatus.running ||
      task.status == AgentTaskStatus.paused ||
      task.status == AgentTaskStatus.completed ||
      task.status == AgentTaskStatus.failed;

  String get _subtitle {
    switch (task.status) {
      case AgentTaskStatus.running:
        return task.currentStepIndex < task.steps.length ? task.steps[task.currentStepIndex] : task.steps.isNotEmpty ? task.steps.last : '';
      case AgentTaskStatus.paused:
        return '已暂停 · ${task.currentStepIndex < task.steps.length ? task.steps[task.currentStepIndex] : ''}';
      case AgentTaskStatus.waitingApproval:
        return '等待权限审批';
      case AgentTaskStatus.pending:
        return '等待开始';
      case AgentTaskStatus.completed:
        return '已完成';
      case AgentTaskStatus.failed:
        return task.error ?? '执行失败';
      case AgentTaskStatus.cancelled:
        return '已取消';
    }
  }

  @override
  Widget build(BuildContext context) {
    final statusLabel = _statusLabel(task.status);
    final badgeType = _badgeType(task.status);
    final iconColor = _iconColor(context, task.status);
    final iconData = _icon(task.status);

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
                  child: Icon(iconData, size: 18, color: iconColor),
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
              SizedBox(height: AppSpacing.md),
              AmitiaProgressBar(progress: task.progress / 100),
              SizedBox(height: AppSpacing.sm),
              Row(
                children: [
                  Text('${task.progress}%', style: AppTypography.label(context)),
                  const SizedBox(width: 12),
                  if (task.status == AgentTaskStatus.running || task.status == AgentTaskStatus.paused)
                    Text('已运行 ${task.elapsed}', style: AppTypography.label(context))
                  else if (task.status == AgentTaskStatus.completed)
                    Text('耗时 ${task.elapsed}', style: AppTypography.label(context))
                  else if (task.status == AgentTaskStatus.failed)
                    Text('失败 ${task.elapsed}', style: AppTypography.label(context)),
                  const Spacer(),
                  _buildAction(context),
                ],
              ),
            ] else ...[
              SizedBox(height: AppSpacing.md),
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
                            color: task.status == AgentTaskStatus.waitingApproval
                                ? context.warning.withValues(alpha: 0.08)
                                : context.accentSoft,
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Text(a, style: TextStyle(fontSize: 11, color: task.status == AgentTaskStatus.waitingApproval ? context.warning : context.accentPrimary)),
                        )).toList(),
                      ),
                    )
                  else
                    const Spacer(),
                  _buildAction(context),
                ],
              ),
            ],
            if (task.status == AgentTaskStatus.completed && task.result != null) ...[
              SizedBox(height: AppSpacing.sm),
              Text(task.result!, style: AppTypography.caption(context)),
            ],
            if (task.status == AgentTaskStatus.failed && task.error != null) ...[
              SizedBox(height: AppSpacing.sm),
              Text(task.error!, style: AppTypography.caption(context).copyWith(color: context.error)),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildAction(BuildContext context) {
    switch (task.status) {
      case AgentTaskStatus.running:
        return _miniButton(context, '暂停', Icons.pause, onTogglePause);
      case AgentTaskStatus.paused:
        return _miniButton(context, '继续', Icons.play_arrow, onTogglePause);
      case AgentTaskStatus.waitingApproval:
        return _miniButton(context, '审批', Icons.shield_outlined, onTap, accent: true);
      case AgentTaskStatus.pending:
        return _miniButton(context, '开始', Icons.play_arrow, onStart, accent: true);
      case AgentTaskStatus.completed:
        return _miniButton(context, '查看结果', Icons.visibility_outlined, onTap);
      case AgentTaskStatus.failed:
        return _miniButton(context, '查看错误', Icons.error_outline, onTap);
      case AgentTaskStatus.cancelled:
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

class _QuickTask {
  final String title;
  final IconData icon;
  final String category;
  const _QuickTask({required this.title, required this.icon, required this.category});
}
