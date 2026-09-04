import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class _TaskEntry {
  final String name;
  final String status;
  final String time;
  final String description;
  const _TaskEntry({required this.name, required this.status, required this.time, required this.description});
}

class ToolboxTaskLogPage extends ConsumerStatefulWidget {
  const ToolboxTaskLogPage({super.key});

  @override
  ConsumerState<ToolboxTaskLogPage> createState() => _ToolboxTaskLogPageState();
}

class _ToolboxTaskLogPageState extends ConsumerState<ToolboxTaskLogPage> {
  List<_TaskEntry> _logs = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final items = await ref.read(extensionTaskServiceProvider).listRuns(limit: 200);
      final logs = items.map((m) {
        return _TaskEntry(
          name: (m['taskDefinitionId'] ?? m['taskRunId'] ?? '').toString(),
          status: (m['status'] ?? m['state'] ?? '未知').toString(),
          time: (m['finishedAt'] ?? m['startedAt'] ?? m['createdAt'] ?? '').toString(),
          description: (m['errorMessage'] ?? m['extensionId'] ?? m['moduleId'] ?? '').toString(),
        );
      }).toList();
      if (mounted) {
        setState(() { _logs = logs; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  BadgeType _badge(String s) {
    final lower = s.toLowerCase();
    if (lower == '成功' || lower == 'success' || lower == 'completed' || lower == 'done') return BadgeType.success;
    if (lower == '失败' || lower == 'error' || lower == 'failed') return BadgeType.error;
    if (lower == '执行中' || lower == 'running' || lower == 'in_progress' || lower == 'processing') return BadgeType.info;
    return BadgeType.neutral;
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载任务日志...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_logs.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.task_outlined, title: '暂无任务记录', subtitle: 'Agent 执行任务后将在此显示');
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '任务日志', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView.separated(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _logs.length,
        separatorBuilder: (_, _) => SizedBox(height: AppSpacing.md),
        itemBuilder: (context, i) {
          final l = _logs[i];
          return Container(
            padding: EdgeInsets.all(AppSpacing.cardPadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Expanded(child: Text(l.name, style: AppTypography.cardTitle(context))),
                    AmitiaStatusBadge(label: l.status, type: _badge(l.status)),
                  ],
                ),
                const SizedBox(height: 4),
                Text(l.description, style: AppTypography.caption(context)),
                const SizedBox(height: 4),
                Text('完成于 ${l.time}', style: AppTypography.label(context)),
              ],
            ),
          );
        },
      ),
    );
  }
}
