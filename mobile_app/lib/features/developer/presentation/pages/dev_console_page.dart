import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DevConsolePage extends ConsumerStatefulWidget {
  const DevConsolePage({super.key});

  @override
  ConsumerState<DevConsolePage> createState() => _DevConsolePageState();
}

class _DevConsolePageState extends ConsumerState<DevConsolePage> {
  static const _levels = ['全部', 'debug', 'info', 'warn', 'error'];
  static const _datasets = <String, String>{
    '日志': '/api/dev-console/logs',
    '调用': '/api/dev-console/invocations',
    '事件': '/api/dev-console/events',
    'Hooks': '/api/dev-console/hooks',
    '任务': '/api/dev-console/tasks',
    'UI会话': '/api/dev-console/ui-sessions',
    '存储': '/api/dev-console/storage',
    '权限': '/api/dev-console/permissions',
    '作用域': '/api/dev-console/scopes',
    '资源': '/api/dev-console/resources',
    '生命周期': '/api/dev-console/lifecycle',
    '性能': '/api/dev-console/performance',
    '迁移': '/api/dev-console/migration',
    '兼容性': '/api/dev-console/compatibility',
    'Host API': '/api/dev-console/host-api-audits',
  };

  Timer? _refreshTimer;
  bool _loading = true;
  bool _isPaused = false;
  String? _error;
  String _dataset = '日志';
  int _selectedLevel = 0;
  Map<String, dynamic> _overview = const {};
  List<Map<String, dynamic>> _records = const [];

  @override
  void initState() {
    super.initState();
    _load();
    _refreshTimer = Timer.periodic(const Duration(seconds: 5), (_) {
      if (!_isPaused && mounted) _load(silent: true);
    });
  }

  @override
  void dispose() {
    _refreshTimer?.cancel();
    super.dispose();
  }

  Future<void> _load({bool silent = false}) async {
    if (!silent && mounted) {
      setState(() {
        _loading = true;
        _error = null;
      });
    }
    try {
      final api = ref.read(backendServiceProvider);
      final overview = await api.get<dynamic>('/api/dev-console/overview');
      final result = await api.get<dynamic>(_datasets[_dataset]!);
      final records = _extractRecords(result);
      if (!mounted) return;
      setState(() {
        _overview = overview is Map ? Map<String, dynamic>.from(overview) : const {};
        _records = records;
        _loading = false;
        _error = null;
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = e.toString();
      });
    }
  }

  List<Map<String, dynamic>> _extractRecords(dynamic value) {
    dynamic source = value;
    if (value is Map) {
      source = value['items'] ?? value['logs'] ?? value['records'] ?? value['entries'] ?? value['data'] ?? const [];
    }
    if (source is! List) return const [];
    return source.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList();
  }

  List<Map<String, dynamic>> get _visibleRecords {
    if (_dataset != '日志' || _selectedLevel == 0) return _records;
    final expected = _levels[_selectedLevel];
    return _records.where((row) {
      final level = (row['level'] ?? row['severity'] ?? '').toString().toLowerCase();
      return level == expected;
    }).toList();
  }

  Future<void> _exportDiagnostics() async {
    try {
      final api = ref.read(backendServiceProvider);
      final diagnostics = await api.get<dynamic>('/api/dev-console/export-diagnostics');
      await Clipboard.setData(
        ClipboardData(text: const JsonEncoder.withIndent('  ').convert(diagnostics ?? const {})),
      );
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('完整诊断数据已复制到剪贴板')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('导出失败：$e')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState();
    if (_error != null && _records.isEmpty) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '诊断控制台',
        showBackButton: true,
        fallbackRoute: AppRoutes.developer,
        actions: [
          AmitiaIconButton(
            icon: _isPaused ? Icons.play_arrow : Icons.pause,
            onPressed: () => setState(() => _isPaused = !_isPaused),
            color: _isPaused ? context.success : context.textSecondary,
            tooltip: _isPaused ? '继续自动刷新' : '暂停自动刷新',
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            onPressed: () => _load(),
            color: context.textSecondary,
            tooltip: '刷新',
          ),
          AmitiaIconButton(
            icon: Icons.copy_all_outlined,
            onPressed: _exportDiagnostics,
            color: context.accentPrimary,
            tooltip: '复制诊断数据',
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildOverview(context),
            _buildToolbar(context),
            if (_isPaused) _buildPausedBanner(context),
            if (_error != null)
              Padding(
                padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
                child: Text(_error!, style: AppTypography.caption(context).copyWith(color: context.error)),
              ),
            Expanded(
              child: _visibleRecords.isEmpty
                  ? const AmitiaEmptyState(icon: Icons.terminal, title: '暂无诊断记录')
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView.builder(
                        padding: EdgeInsets.only(bottom: AppSpacing.lg),
                        itemCount: _visibleRecords.length,
                        itemBuilder: (context, index) => _buildRecord(context, _visibleRecords[index]),
                      ),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildOverview(BuildContext context) {
    final stats = <MapEntry<String, dynamic>>[
      MapEntry('扩展', _overview['extensions'] ?? 0),
      MapEntry('运行调用', _overview['activeInvocations'] ?? 0),
      MapEntry('任务', _overview['activeTasks'] ?? 0),
      MapEntry('事件/5m', _overview['eventsLast5Min'] ?? 0),
      MapEntry('错误', _overview['errors'] ?? 0),
      MapEntry('警告', _overview['warnings'] ?? 0),
    ];
    return Container(
      width: double.infinity,
      padding: EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.md, AppSpacing.pagePadding, AppSpacing.sm),
      color: context.surfacePrimary,
      child: Wrap(
        spacing: AppSpacing.sm,
        runSpacing: AppSpacing.sm,
        children: stats.map((entry) {
          return Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: context.borderPrimary, width: .5),
            ),
            child: Text('${entry.key} ${entry.value}', style: AppTypography.label(context)),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildToolbar(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(bottom: BorderSide(color: context.borderPrimary, width: .5)),
      ),
      child: Column(
        children: [
          SizedBox(
            height: 32,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: _datasets.length,
              separatorBuilder: (_, __) => const SizedBox(width: 6),
              itemBuilder: (context, index) {
                final name = _datasets.keys.elementAt(index);
                final selected = name == _dataset;
                return ChoiceChip(
                  label: Text(name),
                  selected: selected,
                  onSelected: (_) {
                    setState(() => _dataset = name);
                    _load();
                  },
                );
              },
            ),
          ),
          if (_dataset == '日志') ...[
            SizedBox(height: AppSpacing.xs),
            SizedBox(
              height: 30,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: _levels.length,
                separatorBuilder: (_, __) => const SizedBox(width: 6),
                itemBuilder: (context, index) => ChoiceChip(
                  label: Text(_levels[index]),
                  selected: index == _selectedLevel,
                  onSelected: (_) => setState(() => _selectedLevel = index),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  Widget _buildPausedBanner(BuildContext context) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: 7),
      color: context.warning.withValues(alpha: .1),
      child: Text('自动刷新已暂停；当前数据保持不变。', style: AppTypography.label(context).copyWith(color: context.warning)),
    );
  }

  Widget _buildRecord(BuildContext context, Map<String, dynamic> row) {
    final level = (row['level'] ?? row['severity'] ?? '').toString();
    final title = (row['message'] ?? row['name'] ?? row['eventType'] ?? row['taskId'] ?? row['invocationId'] ?? row['id'] ?? '记录').toString();
    final source = (row['extension'] ?? row['extensionId'] ?? row['module'] ?? row['moduleId'] ?? row['source'] ?? '').toString();
    final time = (row['at'] ?? row['createdAt'] ?? row['startedAt'] ?? row['updatedAt'] ?? '').toString();
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: 2),
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(bottom: BorderSide(color: context.borderSecondary, width: .5)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              if (level.isNotEmpty) ...[
                _levelTag(context, level),
                const SizedBox(width: 8),
              ],
              if (source.isNotEmpty)
                Expanded(child: Text(source, style: AppTypography.label(context).copyWith(color: context.accentPrimary)))
              else
                const Spacer(),
              if (time.isNotEmpty) Text(_shortTime(time), style: AppTypography.label(context).copyWith(color: context.textTertiary)),
            ],
          ),
          const SizedBox(height: 6),
          Text(title, style: AppTypography.bodySmall(context)),
          if (row.length > 4) ...[
            const SizedBox(height: 6),
            SelectableText(
              const JsonEncoder.withIndent('  ').convert(row),
              maxLines: 8,
              style: AppTypography.caption(context).copyWith(fontFamily: 'monospace', color: context.textSecondary),
            ),
          ],
        ],
      ),
    );
  }

  Widget _levelTag(BuildContext context, String raw) {
    final level = raw.toLowerCase();
    final color = level.contains('error')
        ? context.error
        : level.contains('warn')
            ? context.warning
            : level.contains('debug')
                ? context.textTertiary
                : context.accentPrimary;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
      decoration: BoxDecoration(color: color.withValues(alpha: .1), borderRadius: BorderRadius.circular(4)),
      child: Text(raw.toUpperCase(), style: AppTypography.label(context).copyWith(color: color, fontSize: 10)),
    );
  }

  String _shortTime(String value) {
    final parsed = DateTime.tryParse(value)?.toLocal();
    if (parsed == null) return value.length > 19 ? value.substring(0, 19) : value;
    return '${parsed.hour.toString().padLeft(2, '0')}:${parsed.minute.toString().padLeft(2, '0')}:${parsed.second.toString().padLeft(2, '0')}';
  }
}
