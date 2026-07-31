import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class DevConsolePage extends ConsumerStatefulWidget {
  const DevConsolePage({super.key});

  @override
  ConsumerState<DevConsolePage> createState() => _DevConsolePageState();
}

class _DevConsolePageState extends ConsumerState<DevConsolePage> {
  final _levels = ['全部', 'INFO', 'WARN', 'ERROR'];
  final _modules = ['全部', 'backend', 'chat', 'mcp', 'qq', 'memory'];
  final _fields = ['时间', '级别', '模块', '消息'];

  int _selectedLevel = 0;
  int _selectedModule = 0;
  bool _isPaused = false;
  List<DevConsoleLog> _logs = [];

  @override
  void initState() {
    super.initState();
    _logs = List.from(MockKernel.devConsoleLogs);
  }

  List<DevConsoleLog> get _filteredLogs {
    return _logs.where((log) {
      if (_selectedLevel > 0 && log.level != _levels[_selectedLevel]) return false;
      if (_selectedModule > 0 && log.module != _modules[_selectedModule]) return false;
      return true;
    }).toList();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '诊断控制台',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: _isPaused ? Icons.play_arrow : Icons.pause,
            onPressed: _togglePause,
            color: _isPaused ? context.success : context.textSecondary,
            tooltip: _isPaused ? '继续' : '暂停',
          ),
          AmitiaIconButton(
            icon: Icons.delete_sweep_outlined,
            onPressed: _clearLogs,
            color: context.error,
            tooltip: '清空',
          ),
          AmitiaIconButton(
            icon: Icons.download_outlined,
            onPressed: _exportLogs,
            color: context.accentPrimary,
            tooltip: '导出',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildFilterBar(context),
            if (_isPaused) _buildPausedBanner(context),
            Expanded(
              child: _filteredLogs.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.terminal,
                      title: '暂无日志',
                      subtitle: '没有符合筛选条件的日志',
                    )
                  : ListView.builder(
                      reverse: true,
                      padding: const EdgeInsets.only(bottom: AppSpacing.lg),
                      itemCount: _filteredLogs.length,
                      itemBuilder: (context, index) {
                        final log = _filteredLogs[_filteredLogs.length - 1 - index];
                        return _buildLogItem(context, log);
                      },
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFilterBar(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(bottom: BorderSide(color: context.borderPrimary, width: 0.5)),
      ),
      child: Column(
        children: [
          _buildFilterRow(context, '级别', _levels, _selectedLevel, (i) => setState(() => _selectedLevel = i)),
          const SizedBox(height: AppSpacing.sm),
          _buildFilterRow(context, '模块', _modules, _selectedModule, (i) => setState(() => _selectedModule = i)),
          const SizedBox(height: AppSpacing.sm),
          _buildFieldSelector(context),
        ],
      ),
    );
  }

  Widget _buildFilterRow(BuildContext context, String label, List<String> items, int selected, ValueChanged<int> onChanged) {
    return Row(
      children: [
        SizedBox(
          width: 40,
          child: Text(label, style: AppTypography.label(context).copyWith(color: context.textSecondary)),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: SizedBox(
            height: 30,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: items.length,
              separatorBuilder: (_, __) => const SizedBox(width: 6),
              itemBuilder: (context, index) {
                final isSelected = index == selected;
                return GestureDetector(
                  onTap: () => onChanged(index),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                    decoration: BoxDecoration(
                      color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Center(
                      child: Text(
                        items[index],
                        style: TextStyle(
                          fontSize: 12,
                          fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
                          color: isSelected ? Colors.white : context.textSecondary,
                        ),
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildFieldSelector(BuildContext context) {
    return Row(
      children: [
        SizedBox(
          width: 40,
          child: Text('字段', style: AppTypography.label(context).copyWith(color: context.textSecondary)),
        ),
        const SizedBox(width: 8),
        Expanded(
          child: Wrap(
            spacing: 6,
            runSpacing: 4,
            children: _fields.map((field) {
              return Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brTag,
                ),
                child: Text(
                  field,
                  style: AppTypography.label(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w500),
                ),
              );
            }).toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildPausedBanner(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 8),
      color: context.warning.withValues(alpha: 0.1),
      child: Row(
        children: [
          Icon(Icons.pause_circle_outline, size: 16, color: context.warning),
          const SizedBox(width: 8),
          Text('日志流已暂停', style: AppTypography.label(context).copyWith(color: context.warning)),
          const Spacer(),
          GestureDetector(
            onTap: _togglePause,
            child: Text('继续', style: AppTypography.label(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w500)),
          ),
        ],
      ),
    );
  }

  Widget _buildLogItem(BuildContext context, DevConsoleLog log) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: 1),
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(bottom: BorderSide(color: context.borderSecondary, width: 0.5)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 50,
            child: Text(
              _formatTime(log.time),
              style: AppTypography.label(context).copyWith(fontFamily: 'monospace', fontSize: 11),
            ),
          ),
          const SizedBox(width: 8),
          _buildLevelTag(context, log.level),
          const SizedBox(width: 8),
          SizedBox(
            width: 60,
            child: Text(
              log.module,
              style: AppTypography.label(context).copyWith(fontFamily: 'monospace', fontSize: 11, color: context.accentPrimary),
            ),
          ),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              log.message,
              style: AppTypography.bodySmall(context).copyWith(fontSize: 13),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildLevelTag(BuildContext context, String level) {
    Color color;
    switch (level) {
      case 'INFO':
        color = context.info;
      case 'WARN':
        color = context.warning;
      case 'ERROR':
        color = context.error;
      default:
        color = context.textSecondary;
    }
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: AppRadius.brTag,
      ),
      child: Text(
        level,
        style: AppTypography.statusLabel(context).copyWith(color: color, fontSize: 10),
      ),
    );
  }

  String _formatTime(DateTime time) {
    return '${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}:${time.second.toString().padLeft(2, '0')}';
  }

  void _togglePause() {
    setState(() {
      _isPaused = !_isPaused;
    });
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(_isPaused ? '日志流已暂停' : '日志流已继续')),
    );
  }

  void _clearLogs() {
    showDialog(
      context: context,
      builder: (context) {
        return AlertDialog(
          backgroundColor: context.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('清空日志', style: AppTypography.cardTitle(context)),
          content: Text('确定要清空所有日志吗？此操作不可恢复。', style: AppTypography.body(context)),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                setState(() {
                  _logs.clear();
                });
                Navigator.pop(context);
                ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('日志已清空')));
              },
              child: Text('清空', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _exportLogs() {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('已导出 ${_filteredLogs.length} 条日志到剪贴板')),
    );
  }
}
