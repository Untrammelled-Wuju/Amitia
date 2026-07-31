import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ToolboxLogPage extends StatefulWidget {
  const ToolboxLogPage({super.key});

  @override
  State<ToolboxLogPage> createState() => _ToolboxLogPageState();
}

class _LogEntry {
  final String time;
  final String level;
  final String module;
  final String content;
  const _LogEntry({required this.time, required this.level, required this.module, required this.content});
}

class _ToolboxLogPageState extends State<ToolboxLogPage> {
  final _searchCtrl = TextEditingController();
  String _levelFilter = '全部';
  List<_LogEntry> _logs = const [
    _LogEntry(time: '09:28:15', level: 'INFO', module: 'Backend', content: 'Go 后端服务已启动，监听 :18899'),
    _LogEntry(time: '09:28:18', level: 'INFO', module: 'SurrealDB', content: '数据库连接成功，schema 已就绪'),
    _LogEntry(time: '09:28:20', level: 'INFO', module: 'Qdrant', content: '向量集合 amitia_memory 加载完成，共 1284 条'),
    _LogEntry(time: '09:29:02', level: 'WARN', module: 'MCP', content: 'MCP Runtime 未配置任何服务，相关能力不可用'),
    _LogEntry(time: '09:30:11', level: 'DEBUG', module: 'Router', content: '路由跳转 /chat -> /settings'),
    _LogEntry(time: '09:31:45', level: 'ERROR', module: 'LLM', content: '调用 GPT-4 超时，已切换至备选模型 Claude'),
    _LogEntry(time: '09:32:03', level: 'INFO', module: 'Agent', content: '任务「整理下载目录」开始执行'),
    _LogEntry(time: '09:32:50', level: 'DEBUG', module: 'FileSystem', content: '扫描完成，发现 1247 个文件'),
    _LogEntry(time: '09:33:21', level: 'WARN', module: 'Memory', content: '情景记忆容量已达 80%，建议清理'),
    _LogEntry(time: '09:34:00', level: 'INFO', module: 'Backend', content: '心跳检测正常，延迟 12ms'),
  ];

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  List<_LogEntry> get _filtered {
    final kw = _searchCtrl.text.trim().toLowerCase();
    return _logs.where((l) {
      if (_levelFilter != '全部' && l.level != _levelFilter) return false;
      if (kw.isEmpty) return true;
      return l.content.toLowerCase().contains(kw) || l.module.toLowerCase().contains(kw);
    }).toList();
  }

  Color _levelColor(String level) {
    switch (level) {
      case 'ERROR':
        return Colors.red;
      case 'WARN':
        return Colors.orange;
      case 'DEBUG':
        return Colors.blueGrey;
      default:
        return Colors.green;
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '运行日志', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(AppSpacing.pagePadding),
            child: Row(
              children: [
                Expanded(
                  child: AmitiaSearchField(
                    hintText: '搜索日志',
                    controller: _searchCtrl,
                    onChanged: (_) => setState(() {}),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10),
                  decoration: BoxDecoration(
                    color: context.surfaceSecondary,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: DropdownButtonHideUnderline(
                    child: DropdownButton<String>(
                      value: _levelFilter,
                      items: const ['全部', 'INFO', 'WARN', 'ERROR', 'DEBUG']
                          .map((l) => DropdownMenuItem(value: l, child: Text(l, style: AppTypography.label(context))))
                          .toList(),
                      onChanged: (v) => setState(() => _levelFilter = v ?? '全部'),
                    ),
                  ),
                ),
              ],
            ),
          ),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Row(
              children: [
                Text('共 ${_filtered.length} 条', style: AppTypography.caption(context)),
                const Spacer(),
                GestureDetector(
                  onTap: _logs.isEmpty ? null : () => setState(() => _logs = const []),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: _logs.isEmpty ? context.borderSecondary : context.error.withValues(alpha: 0.1),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.delete_outline, size: 16, color: _logs.isEmpty ? context.textTertiary : context.error),
                        const SizedBox(width: 4),
                        Text('清空', style: AppTypography.label(context).copyWith(color: _logs.isEmpty ? context.textTertiary : context.error)),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.sm),
          Expanded(
            child: _filtered.isEmpty
                ? AmitiaEmptyState(icon: Icons.inbox_outlined, title: '暂无日志', subtitle: '尝试调整筛选或清空搜索')
                : ListView.separated(
                    padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, 0, AppSpacing.pagePadding, AppSpacing.xl),
                    itemCount: _filtered.length,
                    separatorBuilder: (_, _) => Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
                    itemBuilder: (context, i) {
                      final l = _filtered[i];
                      return Padding(
                        padding: const EdgeInsets.symmetric(vertical: 10),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            SizedBox(width: 64, child: Text(l.time, style: AppTypography.label(context))),
                            const SizedBox(width: 8),
                            Container(
                              width: 52,
                              padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 2),
                              decoration: BoxDecoration(
                                color: _levelColor(l.level).withValues(alpha: 0.12),
                                borderRadius: AppRadius.brTag,
                              ),
                              child: Text(l.level,
                                  textAlign: TextAlign.center,
                                  style: AppTypography.label(context).copyWith(color: _levelColor(l.level), fontSize: 10)),
                            ),
                            const SizedBox(width: 8),
                            Expanded(
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Text(l.module, style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
                                  const SizedBox(height: 2),
                                  Text(l.content, style: AppTypography.bodySmall(context)),
                                ],
                              ),
                            ),
                          ],
                        ),
                      );
                    },
                  ),
          ),
        ],
      ),
    );
  }
}
