import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/backend_uri_builder.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class BackupPage extends ConsumerStatefulWidget {
  const BackupPage({super.key});

  @override
  ConsumerState<BackupPage> createState() => _BackupPageState();
}

class _BackupPageState extends ConsumerState<BackupPage> {
  List<Map<String, dynamic>> _backups = const [];
  bool _loadingBackups = true;
  bool _busy = false;
  String? _backupsError;

  @override
  void initState() {
    super.initState();
    _loadBackups();
  }

  Future<void> _loadBackups() async {
    if (mounted) {
      setState(() {
        _loadingBackups = true;
        _backupsError = null;
      });
    }
    try {
      final resp = await ref.read(backendServiceProvider).get<Map<String, dynamic>>('/api/storage/backups');
      final rows = resp?['backups'];
      final items = rows is List
          ? rows.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList(growable: false)
          : <Map<String, dynamic>>[];
      if (mounted) {
        setState(() {
          _backups = items;
          _loadingBackups = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _backupsError = e.toString();
          _loadingBackups = false;
        });
      }
    }
  }

  Future<void> _run(String label, Future<void> Function() task) async {
    if (_busy) return;
    setState(() => _busy = true);
    try {
      await task();
    } catch (e) {
      if (mounted) _show('$label失败：$e', error: true);
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  void _show(String message, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: error ? context.error : null,
        duration: const Duration(seconds: 2),
      ),
    );
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) {
      throw StateError('后端当前不可用');
    }
    return createAuthenticatedDio(availability.config);
  }

  Future<void> _exportData() async {
    await _run('导出', () async {
      final api = ref.read(backendServiceProvider);
      final result = await api.post<Map<String, dynamic>>(
        '/api/storage/export-amitia',
        data: const {'scope': 'all'},
      );
      if (result?['exported'] != true) {
        throw StateError((result?['error'] ?? '导出未完成').toString());
      }
      final filename = (result?['file'] ?? '').toString();
      if (filename.isEmpty) throw StateError('后端未返回导出文件名');
      final output = await FilePicker.platform.saveFile(
        dialogTitle: '保存 Amitia 数据备份',
        fileName: filename,
      );
      if (output == null || output.isEmpty) {
        _show('导出文件已生成：$filename');
        return;
      }
      final dio = await _dio();
      try {
        await dio.download('/api/storage/export-download/${Uri.encodeComponent(filename)}', output);
      } finally {
        dio.close(force: true);
      }
      _show('数据已导出');
    });
  }

  Future<void> _importData() async {
    await _run('导入', () async {
      final picked = await FilePicker.platform.pickFiles(
        type: FileType.custom,
        allowedExtensions: const ['amitia', 'zip', 'tar', 'gz'],
      );
      if (picked == null || picked.files.isEmpty) return;
      final file = picked.files.first;
      if (file.path == null || file.path!.isEmpty) throw StateError('无法读取所选文件');
      final dio = await _dio();
      try {
        final data = FormData.fromMap({
          'file': await MultipartFile.fromFile(file.path!, filename: file.name),
        });
        final response = await dio.post('/api/storage/import-amitia', data: data);
        final body = response.data;
        final payload = body is Map ? body['data'] : null;
        if (payload is! Map || payload['imported'] != true) {
          throw StateError((payload is Map ? payload['error'] : null)?.toString() ?? '导入未完成');
        }
      } finally {
        dio.close(force: true);
      }
      _show('数据导入完成，建议重新启动应用');
      ref.invalidate(characterListProvider);
      ref.invalidate(conversationListProvider);
      ref.invalidate(memoryListProvider);
    });
  }

  Future<void> _exportConfig() async {
    await _run('配置导出', () async {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/config/export');
      if (result?['exported'] != true) throw StateError((result?['error'] ?? '配置导出失败').toString());
      final output = await FilePicker.platform.saveFile(
        dialogTitle: '保存 Amitia 配置',
        fileName: 'amitia-config.json',
        type: FileType.custom,
        allowedExtensions: const ['json'],
      );
      if (output == null || output.isEmpty) return;
      await File(output).writeAsString(const JsonEncoder.withIndent('  ').convert(result), flush: true);
      _show('配置已导出');
    });
  }

  Future<void> _importConfig() async {
    await _run('配置导入', () async {
      final picked = await FilePicker.platform.pickFiles(type: FileType.custom, allowedExtensions: const ['json']);
      if (picked == null || picked.files.isEmpty) return;
      final path = picked.files.first.path;
      if (path == null || path.isEmpty) throw StateError('无法读取所选配置文件');
      final raw = await File(path).readAsString();
      final api = ref.read(backendServiceProvider);
      final preview = await api.post<Map<String, dynamic>>('/api/config/import/preview', data: {'raw': raw});
      if (preview?['valid'] != true) throw StateError((preview?['error'] ?? '配置文件无效').toString());
      if (!mounted) return;
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('确认导入配置'),
          content: Text(
            '共 ${preview?['itemCount'] ?? 0} 项配置；新增 ${preview?['newCount'] ?? 0} 项，修改 ${preview?['changed'] ?? 0} 项，未变化 ${preview?['unchanged'] ?? 0} 项。继续后会覆盖同名设置。',
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            FilledButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('导入')),
          ],
        ),
      );
      if (confirmed != true) return;
      final result = await api.post<Map<String, dynamic>>('/api/config/import/confirm', data: {'raw': raw});
      if (result?['imported'] != true) throw StateError((result?['error'] ?? '配置导入失败').toString());
      _show('已导入 ${result?['importedCount'] ?? 0} 项配置');
    });
  }

  Future<void> _createBackup() async {
    await _run('备份', () async {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/storage/backups');
      if (result?['ok'] != true) throw StateError((result?['error'] ?? '备份未完成').toString());
      _show('当前业务 Core 备份已创建');
      await _loadBackups();
    });
  }

  Future<void> _checkMigrations() async {
    await _run('迁移检查', () async {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/storage/migrations/check');
      final needs = result?['needsMigration'] == true;
      _show(needs ? '检测到待处理的数据迁移' : '数据结构已是最新状态');
    });
  }

  Future<void> _cleanupTemp() async {
    await _run('缓存清理', () async {
      await ref.read(backendServiceProvider).post<Map<String, dynamic>>('/api/runtime/cleanup-temp');
      _show('临时缓存已清理');
    });
  }

  Future<void> _restoreBackup(Map<String, dynamic> backup) async {
    final name = (backup['name'] ?? '').toString();
    if (name.isEmpty) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('恢复 Core 备份'),
        content: Text('将用「$name」覆盖当前业务 Core 的数据库。Local 模式对应本机 Core，Cloud 模式对应 Cloud Core。是否继续？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('恢复')),
        ],
      ),
    );
    if (confirmed != true) return;
    await _run('恢复', () async {
      final result = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/storage/backups/${Uri.encodeComponent(name)}/restore',
      );
      if (result?['ok'] != true) throw StateError((result?['error'] ?? '恢复未完成').toString());
      _show('备份已恢复，建议重新启动应用');
    });
  }

  Future<void> _deleteBackup(Map<String, dynamic> backup) async {
    final name = (backup['name'] ?? '').toString();
    if (name.isEmpty) return;
    await _run('删除备份', () async {
      await ref.read(backendServiceProvider).delete('/api/storage/backups/${Uri.encodeComponent(name)}');
      _show('备份已删除');
      await _loadBackups();
    });
  }

  String _formatBytes(dynamic raw) {
    final bytes = raw is num ? raw.toInt() : int.tryParse(raw?.toString() ?? '') ?? 0;
    if (bytes >= 1024 * 1024 * 1024) return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB';
    if (bytes >= 1024 * 1024) return '${(bytes / (1024 * 1024)).toStringAsFixed(2)} MB';
    if (bytes >= 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '$bytes B';
  }

  @override
  Widget build(BuildContext context) {
    final charactersAsync = ref.watch(characterListProvider);
    final conversationsAsync = ref.watch(conversationListProvider);
    final memoriesAsync = ref.watch(memoryListProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '数据与备份', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: Stack(
        children: [
          ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              Text('数据概览', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              _DataOverview(
                charactersCount: charactersAsync.valueOrNull?.length ?? 0,
                conversationsCount: conversationsAsync.valueOrNull?.length ?? 0,
                memoriesCount: memoriesAsync.valueOrNull?.length ?? 0,
                isLoading: charactersAsync.isLoading || conversationsAsync.isLoading || memoriesAsync.isLoading,
              ),
              SizedBox(height: AppSpacing.sectionGap),
              Text('操作', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              _ActionGrid(
                actions: [
                  ('导出数据', Icons.file_upload_outlined, _exportData),
                  ('导入数据', Icons.file_download_outlined, _importData),
                  ('Core 备份', Icons.save_outlined, _createBackup),
                  ('迁移检查', Icons.sync_alt_outlined, _checkMigrations),
                  ('清理缓存', Icons.cleaning_services_outlined, _cleanupTemp),
                  ('导出配置', Icons.settings_backup_restore_outlined, _exportConfig),
                  ('导入配置', Icons.settings_suggest_outlined, _importConfig),
                ],
              ),
              SizedBox(height: AppSpacing.sectionGap),
              Text(
                '备份接口作用于当前业务 Core：Local 模式为本机 Core，Cloud 模式为 Cloud Core；不会把云端数据库误标为当前设备本地数据库。',
                style: AppTypography.body(context).copyWith(color: context.textTertiary),
              ),
              SizedBox(height: AppSpacing.md),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('Core 备份', style: AppTypography.sectionTitle(context)),
                  IconButton(icon: Icon(Icons.refresh, size: 20, color: context.textTertiary), onPressed: _loadingBackups ? null : _loadBackups),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              if (_loadingBackups)
                const Padding(padding: EdgeInsets.all(24), child: Center(child: CircularProgressIndicator()))
              else if (_backupsError != null)
                Padding(padding: EdgeInsets.all(AppSpacing.md), child: Text('加载失败: $_backupsError', style: TextStyle(color: context.error)))
              else if (_backups.isEmpty)
                Padding(
                  padding: EdgeInsets.all(AppSpacing.lg),
                  child: Center(child: Text('暂无备份记录', style: AppTypography.body(context).copyWith(color: context.textTertiary))),
                )
              else
                ..._backups.map(
                  (backup) => Padding(
                    padding: EdgeInsets.only(bottom: AppSpacing.sm),
                    child: _BackupRecord(
                      name: (backup['name'] ?? '').toString(),
                      time: (backup['createdAt'] ?? '').toString(),
                      size: _formatBytes(backup['size']),
                      onRestore: () => _restoreBackup(backup),
                      onDelete: () => _deleteBackup(backup),
                    ),
                  ),
                ),
            ],
          ),
          if (_busy)
            Positioned.fill(
              child: ColoredBox(
                color: Colors.black.withValues(alpha: 0.08),
                child: const Center(child: CircularProgressIndicator()),
              ),
            ),
        ],
      ),
    );
  }
}

class _DataOverview extends StatelessWidget {
  final int charactersCount;
  final int conversationsCount;
  final int memoriesCount;
  final bool isLoading;

  const _DataOverview({
    required this.charactersCount,
    required this.conversationsCount,
    required this.memoriesCount,
    required this.isLoading,
  });

  @override
  Widget build(BuildContext context) {
    final items = <(String, String, IconData)>[
      ('对话数据', isLoading ? '...' : '$conversationsCount 条', Icons.chat_outlined),
      ('角色数据', isLoading ? '...' : '$charactersCount 个', Icons.people_outline),
      ('记忆数据', isLoading ? '...' : '$memoriesCount 条', Icons.memory),
    ];
    return Container(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.lg, horizontal: AppSpacing.sm),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(children: [for (final item in items) _DataItem(label: item.$1, value: item.$2, icon: item.$3)]),
    );
  }
}

class _DataItem extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;

  const _DataItem({required this.label, required this.value, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Column(
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(color: context.accentSoft, shape: BoxShape.circle),
            child: Icon(icon, size: 16, color: context.accentPrimary),
          ),
          SizedBox(height: AppSpacing.sm),
          Text(value, style: AppTypography.cardTitle(context)),
          const SizedBox(height: 2),
          Text(label, style: AppTypography.label(context), textAlign: TextAlign.center, maxLines: 1, overflow: TextOverflow.ellipsis),
        ],
      ),
    );
  }
}

class _ActionGrid extends StatelessWidget {
  final List<(String, IconData, Future<void> Function())> actions;

  const _ActionGrid({required this.actions});

  @override
  Widget build(BuildContext context) {
    return GridView.count(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 3,
      mainAxisSpacing: AppSpacing.md,
      crossAxisSpacing: AppSpacing.md,
      childAspectRatio: 1.5,
      children: actions.map((action) {
        return GestureDetector(
          onTap: action.$3,
          child: Container(
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(action.$2, size: 22, color: context.accentPrimary),
                const SizedBox(height: 6),
                Text(action.$1, style: AppTypography.bodySmall(context), maxLines: 1, overflow: TextOverflow.ellipsis),
              ],
            ),
          ),
        );
      }).toList(growable: false),
    );
  }
}

class _BackupRecord extends StatelessWidget {
  final String name;
  final String time;
  final String size;
  final VoidCallback onRestore;
  final VoidCallback onDelete;

  const _BackupRecord({
    required this.name,
    required this.time,
    required this.size,
    required this.onRestore,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 14),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Icon(Icons.history, size: 20, color: context.textTertiary),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTypography.bodySmall(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                const SizedBox(height: 2),
                Text('$time · $size', style: AppTypography.label(context)),
              ],
            ),
          ),
          IconButton(onPressed: onRestore, tooltip: '恢复', icon: Icon(Icons.restore, size: 20, color: context.accentPrimary)),
          IconButton(onPressed: onDelete, tooltip: '删除', icon: Icon(Icons.delete_outline, size: 20, color: context.error)),
        ],
      ),
    );
  }
}
