import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class _LogEntry {
  final String time;
  final String level;
  final String module;
  final String content;
  const _LogEntry({required this.time, required this.level, required this.module, required this.content});
}

class ToolboxLogPage extends ConsumerStatefulWidget {
  const ToolboxLogPage({super.key});

  @override
  ConsumerState<ToolboxLogPage> createState() => _ToolboxLogPageState();
}

class _ToolboxLogPageState extends ConsumerState<ToolboxLogPage> {
  final _searchCtrl = TextEditingController();
  String _levelFilter = '全部';
  List<_LogEntry> _logs = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.watch(backendServiceProvider);
      final resp = await api.get<List<dynamic>>('/api/system/logs');
      final items = resp ?? [];
      final logs = items.map((e) {
        final m = e as Map<String, dynamic>? ?? {};
        return _LogEntry(
          time: (m['time'] ?? m['timestamp'] ?? '').toString(),
          level: (m['level'] ?? 'INFO').toString(),
          module: (m['module'] ?? m['source'] ?? m['service'] ?? 'System').toString(),
          content: (m['message'] ?? m['content'] ?? m['msg'] ?? '').toString(),
        );
      }).toList();
      if (mounted) {
        setState(() { _logs = logs; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
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
    switch (level.toUpperCase()) {
      case 'ERROR':
        return Colors.red;
      case 'WARN':
      case 'WARNING':
        return Colors.orange;
      case 'DEBUG':
        return Colors.blueGrey;
      default:
        return Colors.green;
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载日志...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

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
                ? const AmitiaEmptyState(icon: Icons.inbox_outlined, title: '暂无日志', subtitle: '尝试调整筛选或清空搜索')
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
