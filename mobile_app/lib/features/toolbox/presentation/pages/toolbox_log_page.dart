import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/log_folder_button.dart';

class _LogEntry {
  final String file;
  final String level;
  final String module;
  final String content;
  final String raw;

  const _LogEntry({
    required this.file,
    required this.level,
    required this.module,
    required this.content,
    required this.raw,
  });
}

class ToolboxLogPage extends ConsumerStatefulWidget {
  const ToolboxLogPage({super.key});

  @override
  ConsumerState<ToolboxLogPage> createState() => _ToolboxLogPageState();
}

class _ToolboxLogPageState extends ConsumerState<ToolboxLogPage> {
  final _searchCtrl = TextEditingController();
  String _levelFilter = '全部';
  List<_LogEntry> _logs = const [];
  List<Map<String, dynamic>> _files = const [];
  bool _loading = true;
  bool _clearing = false;
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

  _LogEntry _parseLine(Map<String, dynamic> raw) {
    final line = (raw['line'] ?? '').toString();
    Map<String, dynamic> structured = const {};
    try {
      final decoded = jsonDecode(line);
      if (decoded is Map<String, dynamic>) structured = decoded;
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
    }
    return _LogEntry(
      file: (raw['file'] ?? 'runtime.log').toString(),
      level: (structured['@level'] ?? structured['level'] ?? 'INFO')
          .toString()
          .toUpperCase(),
      module:
          (structured['stage'] ??
                  structured['path'] ??
                  structured['source'] ??
                  raw['file'] ??
                  'System')
              .toString(),
      content: (structured['@message'] ?? structured['message'] ?? line)
          .toString(),
      raw: line,
    );
  }

  Future<void> _load() async {
    if (mounted)
      setState(() {
        _loading = true;
        _error = null;
      });
    try {
      final api = ref.read(backendServiceProvider);
      final results = await Future.wait([
        api.get<Map<String, dynamic>>(
          '/api/logs/recent',
          queryParameters: const {'limit': 200},
          fromJson: (e) => Map<String, dynamic>.from(e as Map),
        ),
        api.get<Map<String, dynamic>>(
          '/api/logs/files',
          fromJson: (e) => Map<String, dynamic>.from(e as Map),
        ),
      ]);
      final recent = results[0];
      final filesResponse = results[1];
      final items = (recent?['logs'] as List<dynamic>? ?? const []);
      final logs = items
          .whereType<Map>()
          .map((e) => _parseLine(Map<String, dynamic>.from(e)))
          .toList();
      final files = (filesResponse?['files'] as List<dynamic>? ?? const [])
          .whereType<Map>()
          .map((e) => Map<String, dynamic>.from(e))
          .toList(growable: false);
      if (mounted)
        setState(() {
          _logs = logs;
          _files = files;
          _loading = false;
        });
    } catch (e) {
      if (mounted)
        setState(() {
          _error = e.toString();
          _loading = false;
        });
    }
  }

  Future<void> _clear() async {
    if (_clearing) return;
    setState(() => _clearing = true);
    try {
      await ref.read(backendServiceProvider).delete('/api/logs');
      if (mounted) setState(() => _logs = const []);
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('清空失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _clearing = false);
    }
  }

  Future<void> _openLogFile(Map<String, dynamic> file) async {
    final name = (file['name'] ?? '').toString();
    if (name.isEmpty) return;
    try {
      final content = await ref
          .read(backendServiceProvider)
          .get<String>(
            '/api/logs/files/${Uri.encodeComponent(name)}',
            fromJson: (value) => value?.toString() ?? '',
          );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: Text(name),
          content: SizedBox(
            width: 760,
            height: MediaQuery.of(dialogContext).size.height * 0.65,
            child: SingleChildScrollView(
              child: SelectableText(
                content ?? '',
                style: const TextStyle(
                  fontFamily: 'monospace',
                  fontSize: 12,
                  height: 1.45,
                ),
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (e) {
      if (mounted)
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('读取日志文件失败：$e')));
    }
  }

  Future<void> _showLogFiles() async {
    if (!mounted) return;
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        child: SizedBox(
          height: MediaQuery.of(sheetContext).size.height * 0.72,
          child: Column(
            children: [
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.lg,
                  0,
                  AppSpacing.lg,
                  AppSpacing.lg,
                ),
                child: Row(
                  children: [
                    Expanded(
                      child: Text(
                        '日志文件 (${_files.length})',
                        style: AppTypography.sectionTitle(context),
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.refresh),
                      onPressed: () async {
                        Navigator.pop(sheetContext);
                        await _load();
                        if (mounted) await _showLogFiles();
                      },
                    ),
                  ],
                ),
              ),
              Expanded(
                child: _files.isEmpty
                    ? const Center(child: Text('暂无日志文件'))
                    : ListView.separated(
                        itemCount: _files.length,
                        separatorBuilder: (_, _) => const Divider(height: 1),
                        itemBuilder: (_, index) {
                          final file = _files[index];
                          final name = (file['name'] ?? '').toString();
                          final size = (file['size'] as num?)?.toInt() ?? 0;
                          return ListTile(
                            leading: const Icon(Icons.description_outlined),
                            title: Text(name),
                            subtitle: Text(
                              '${_formatBytes(size)} · ${file['modTime'] ?? ''}',
                            ),
                            trailing: const Icon(Icons.chevron_right),
                            onTap: () {
                              Navigator.pop(sheetContext);
                              _openLogFile(file);
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

  String _formatBytes(int bytes) {
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }

  List<_LogEntry> get _filtered {
    final kw = _searchCtrl.text.trim().toLowerCase();
    return _logs.where((l) {
      if (_levelFilter != '全部' && l.level != _levelFilter) return false;
      if (kw.isEmpty) return true;
      return l.content.toLowerCase().contains(kw) ||
          l.module.toLowerCase().contains(kw);
    }).toList();
  }

  String _formatLogFile(List<_LogEntry> logs) {
    final groups = <String, List<_LogEntry>>{};
    for (final log in logs) {
      final file = log.file.trim().isEmpty ? 'runtime.log' : log.file.trim();
      groups.putIfAbsent(file, () => []).add(log);
    }
    final buffer = StringBuffer();
    for (final entry in groups.entries) {
      if (buffer.isNotEmpty) buffer.writeln();
      buffer.writeln('==> ${entry.key} <==');
      for (final log in entry.value) {
        buffer.writeln(log.raw);
      }
    }
    return buffer.toString().trimRight();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '运行日志',
        showBackButton: true,
        fallbackRoute: AppRoutes.settingsMaintenanceCategory,
        actions: [
          const LogFolderButton(),
          AmitiaIconButton(
            icon: Icons.folder_open_outlined,
            tooltip: '日志文件',
            onPressed: _showLogFiles,
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新',
            onPressed: _load,
          ),
        ],
      ),
      body: _loading
          ? const AmitiaLoadingState(message: '正在加载日志...')
          : _error != null
          ? AmitiaErrorState(message: _error!, onRetry: _load)
          : Column(
              children: [
                Padding(
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  child: Row(
                    children: [
                      Expanded(
                        child: AmitiaSearchField(
                          hintText: '搜索日志',
                          controller: _searchCtrl,
                          onChanged: (_) => setState(() {}),
                        ),
                      ),
                      SizedBox(width: AppSpacing.sm),
                      Container(
                        padding: const EdgeInsets.symmetric(horizontal: 10),
                        decoration: BoxDecoration(
                          color: context.surfaceSecondary,
                          borderRadius: AppRadius.brSmall,
                        ),
                        child: DropdownButtonHideUnderline(
                          child: DropdownButton<String>(
                            value: _levelFilter,
                            items:
                                const ['全部', 'INFO', 'WARN', 'ERROR', 'DEBUG']
                                    .map(
                                      (l) => DropdownMenuItem(
                                        value: l,
                                        child: Text(
                                          l,
                                          style: AppTypography.label(context),
                                        ),
                                      ),
                                    )
                                    .toList(),
                            onChanged: (v) =>
                                setState(() => _levelFilter = v ?? '全部'),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                Padding(
                  padding: EdgeInsets.symmetric(
                    horizontal: AppSpacing.pagePadding,
                  ),
                  child: Row(
                    children: [
                      Text(
                        '共 ${_filtered.length} 条',
                        style: AppTypography.caption(context),
                      ),
                      const Spacer(),
                      GestureDetector(
                        onTap: _logs.isEmpty || _clearing ? null : _clear,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 12,
                            vertical: 6,
                          ),
                          decoration: BoxDecoration(
                            color: _logs.isEmpty
                                ? context.borderSecondary
                                : context.error.withValues(alpha: 0.1),
                            borderRadius: AppRadius.brTag,
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(
                                Icons.delete_outline,
                                size: 16,
                                color: _logs.isEmpty
                                    ? context.textTertiary
                                    : context.error,
                              ),
                              const SizedBox(width: 4),
                              Text(
                                _clearing ? '清理中' : '清空',
                                style: AppTypography.label(context).copyWith(
                                  color: _logs.isEmpty
                                      ? context.textTertiary
                                      : context.error,
                                ),
                              ),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                SizedBox(height: AppSpacing.sm),
                Expanded(
                  child: RefreshIndicator(
                    onRefresh: _load,
                    child: _filtered.isEmpty
                        ? ListView(
                            children: const [
                              AmitiaEmptyState(
                                icon: Icons.inbox_outlined,
                                title: '暂无日志',
                                subtitle: '当前没有符合条件的运行日志',
                              ),
                            ],
                          )
                        : SingleChildScrollView(
                            physics: const AlwaysScrollableScrollPhysics(),
                            padding: EdgeInsets.fromLTRB(
                              AppSpacing.pagePadding,
                              0,
                              AppSpacing.pagePadding,
                              AppSpacing.xl,
                            ),
                            child: Container(
                              width: double.infinity,
                              clipBehavior: Clip.antiAlias,
                              decoration: BoxDecoration(
                                color: context.surfaceSecondary,
                                border: Border.all(
                                  color: context.borderSecondary,
                                ),
                                borderRadius: AppRadius.brMedium,
                              ),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.stretch,
                                children: [
                                  Padding(
                                    padding: const EdgeInsets.symmetric(
                                      horizontal: 12,
                                      vertical: 9,
                                    ),
                                    child: Row(
                                      children: [
                                        Text(
                                          '最近日志',
                                          style: AppTypography.label(context),
                                        ),
                                        const Spacer(),
                                        Text(
                                          '${_filtered.length} 行',
                                          style: AppTypography.caption(context),
                                        ),
                                      ],
                                    ),
                                  ),
                                  Divider(
                                    height: 1,
                                    color: context.borderSecondary,
                                  ),
                                  Padding(
                                    padding: const EdgeInsets.all(12),
                                    child: SelectableText(
                                      _formatLogFile(_filtered),
                                      style: TextStyle(
                                        color: context.textPrimary,
                                        fontFamily: 'monospace',
                                        fontSize: 12,
                                        height: 1.5,
                                      ),
                                    ),
                                  ),
                                ],
                              ),
                            ),
                          ),
                  ),
                ),
              ],
            ),
    );
  }
}
