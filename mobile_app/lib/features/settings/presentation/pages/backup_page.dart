import 'dart:convert';
import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
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
      final resp = await ref
          .read(backendServiceProvider)
          .get<Map<String, dynamic>>('/api/data-portability/backups');
      final rows = resp?['backups'];
      final items = rows is List
          ? rows
                .whereType<Map>()
                .map((e) => Map<String, dynamic>.from(e))
                .toList(growable: false)
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

  Future<Map<String, dynamic>?> _exportOptions() async {
    final characters = ref.read(characterListProvider).valueOrNull ?? const [];
    var scope = 'all';
    var profile = 'portable';
    String? characterId;
    final passphrase = TextEditingController();
    var encrypted = false;
    return showDialog<Map<String, dynamic>>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('数据导出预检'),
          content: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                DropdownButtonFormField<String>(
                  initialValue: scope,
                  decoration: const InputDecoration(labelText: '导出范围'),
                  items: const [
                    DropdownMenuItem(value: 'all', child: Text('全部数据')),
                    DropdownMenuItem(
                      value: 'character',
                      child: Text('角色及关联数据'),
                    ),
                    DropdownMenuItem(value: 'settings', child: Text('设置与集成')),
                    DropdownMenuItem(value: 'memory', child: Text('记忆')),
                  ],
                  onChanged: (value) =>
                      setDialogState(() => scope = value ?? 'all'),
                ),
                DropdownButtonFormField<String>(
                  initialValue: profile,
                  decoration: const InputDecoration(labelText: '导出格式'),
                  items: const [
                    DropdownMenuItem(value: 'portable', child: Text('可迁移格式')),
                    DropdownMenuItem(value: 'full', child: Text('完整格式')),
                  ],
                  onChanged: (value) =>
                      setDialogState(() => profile = value ?? 'portable'),
                ),
                if (scope == 'character')
                  DropdownButtonFormField<String>(
                    initialValue: characterId,
                    decoration: const InputDecoration(labelText: '角色'),
                    items: characters
                        .map(
                          (character) => DropdownMenuItem(
                            value: character.id,
                            child: Text(character.name),
                          ),
                        )
                        .toList(growable: false),
                    onChanged: (value) =>
                        setDialogState(() => characterId = value),
                  ),
                CheckboxListTile(
                  contentPadding: EdgeInsets.zero,
                  title: const Text('使用口令加密'),
                  value: encrypted,
                  onChanged: (value) =>
                      setDialogState(() => encrypted = value ?? false),
                ),
                if (encrypted)
                  TextField(
                    controller: passphrase,
                    obscureText: true,
                    decoration: const InputDecoration(
                      labelText: '导出口令（至少 8 位）',
                    ),
                  ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () {
                if (scope == 'character' &&
                    (characterId == null || characterId!.isEmpty)) {
                  return;
                }
                if (encrypted && passphrase.text.length < 8) return;
                Navigator.pop(dialogContext, {
                  'scope': scope,
                  'profile': profile,
                  'characterId': scope == 'character' ? characterId : '',
                  'passphrase': encrypted ? passphrase.text : '',
                });
              },
              child: const Text('生成导出包'),
            ),
          ],
        ),
      ),
    ).whenComplete(passphrase.dispose);
  }

  Future<void> _exportData() async {
    final options = await _exportOptions();
    if (options == null || !mounted) return;
    await _run('导出', () async {
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/data-portability/exports',
            data: options,
          );
      final filename = (result?['file'] ?? '').toString();
      if (result?['operationId'] == null || filename.isEmpty) {
        throw StateError('导出未返回有效制品');
      }
      final output = await FilePicker.platform.saveFile(
        dialogTitle: '保存 Amitia 可迁移数据',
        fileName: filename,
      );
      if (output == null || output.isEmpty) {
        _show('导出文件已生成：$filename');
        return;
      }
      final stream = await ref
          .read(backendServiceProvider)
          .getStream(
            '/api/data-portability/exports/${Uri.encodeComponent(filename)}',
          );
      final sink = File(output).openWrite();
      try {
        await for (final chunk in stream) {
          sink.add(chunk);
        }
      } finally {
        await sink.close();
      }
      _show('数据已导出');
    });
  }

  Future<String?> _askPassphrase(String title) {
    final controller = TextEditingController();
    return showDialog<String>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: TextField(
          controller: controller,
          obscureText: true,
          autofocus: true,
          decoration: const InputDecoration(labelText: '口令，未加密可留空'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(dialogContext, controller.text),
            child: const Text('继续'),
          ),
        ],
      ),
    ).whenComplete(controller.dispose);
  }

  Future<void> _importData() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['amitia-backup', 'enc'],
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) throw StateError('无法读取所选文件');
    final passphrase = await _askPassphrase('解密导入归档');
    if (passphrase == null) return;
    await _run('导入预检', () async {
      final preview = await ref
          .read(backendServiceProvider)
          .postMultipart<Map<String, dynamic>>(
            '/api/data-portability/imports/inspect',
            fields: passphrase.isEmpty ? const {} : {'passphrase': passphrase},
            files: {
              'file': [file.path!],
            },
          );
      final ticketId = (preview?['ticketId'] ?? '').toString();
      if (ticketId.isEmpty) throw StateError('导入预检未返回票据');
      if (!mounted) return;
      var policy = 'duplicate';
      var activate = false;
      final confirmed = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) => StatefulBuilder(
          builder: (context, setDialogState) {
            final manifest =
                preview?['preview'] is Map &&
                    (preview!['preview'] as Map)['manifest'] is Map
                ? Map<String, dynamic>.from(
                    (preview['preview'] as Map)['manifest'] as Map,
                  )
                : <String, dynamic>{};
            final components =
                preview?['preview'] is Map &&
                    (preview!['preview'] as Map)['components'] is List
                ? ((preview['preview'] as Map)['components'] as List)
                      .whereType<Map>()
                : const <Map>[];
            return AlertDialog(
              title: const Text('确认导入'),
              content: SingleChildScrollView(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Text('应用版本：${manifest['appVersion'] ?? '—'}'),
                    Text('来源平台：${manifest['sourcePlatform'] ?? '—'}'),
                    Text(
                      '导出范围：${_scopeLabel((manifest['scope'] ?? '').toString())}',
                    ),
                    const SizedBox(height: 12),
                    ...components.map((component) {
                      final collisions = component['collisions'] is List
                          ? (component['collisions'] as List).length
                          : 0;
                      return Padding(
                        padding: const EdgeInsets.only(bottom: 6),
                        child: Text(
                          '${component['logicalName'] ?? component['componentId']} · '
                          '${component['itemCount'] ?? 0} 条 · 冲突 $collisions',
                        ),
                      );
                    }),
                    RadioGroup<String>(
                      groupValue: policy,
                      onChanged: (value) =>
                          setDialogState(() => policy = value ?? 'duplicate'),
                      child: const Column(
                        children: [
                          RadioListTile(
                            value: 'duplicate',
                            title: Text('保留双方并生成新标识'),
                          ),
                          RadioListTile(value: 'skip', title: Text('跳过已有数据')),
                          RadioListTile(
                            value: 'replace',
                            title: Text('覆盖同一标识数据'),
                          ),
                        ],
                      ),
                    ),
                    CheckboxListTile(
                      contentPadding: EdgeInsets.zero,
                      title: const Text('导入后启用角色和配置'),
                      value: activate,
                      onChanged: (value) =>
                          setDialogState(() => activate = value ?? false),
                    ),
                  ],
                ),
              ),
              actions: [
                TextButton(
                  onPressed: () => Navigator.pop(dialogContext, false),
                  child: const Text('取消'),
                ),
                FilledButton(
                  onPressed: () => Navigator.pop(dialogContext, true),
                  child: const Text('确认导入'),
                ),
              ],
            );
          },
        ),
      );
      if (confirmed != true) {
        await ref
            .read(backendServiceProvider)
            .delete(
              '/api/data-portability/imports/${Uri.encodeComponent(ticketId)}',
            );
        return;
      }
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/data-portability/imports/${Uri.encodeComponent(ticketId)}/commit',
            data: {'policy': policy, 'activate': activate},
          );
      _show('已导入 ${result?['totalImported'] ?? 0} 条记录');
      ref.invalidate(characterListProvider);
      ref.invalidate(conversationListProvider);
      ref.invalidate(ownedMemoryCountProvider);
    });
  }

  Future<void> _exportConfig() async {
    await _run('配置导出', () async {
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>('/api/config/export');
      if (result?['exported'] != true) {
        throw StateError((result?['error'] ?? '配置导出失败').toString());
      }
      final output = await FilePicker.platform.saveFile(
        dialogTitle: '保存 Amitia 配置',
        fileName: 'amitia-config.json',
        type: FileType.custom,
        allowedExtensions: const ['json'],
      );
      if (output == null || output.isEmpty) return;
      await File(output).writeAsString(
        const JsonEncoder.withIndent('  ').convert(result),
        flush: true,
      );
      _show('配置已导出');
    });
  }

  Future<void> _importConfig() async {
    await _run('配置导入', () async {
      final picked = await FilePicker.platform.pickFiles(
        type: FileType.custom,
        allowedExtensions: const ['json'],
      );
      if (picked == null || picked.files.isEmpty) return;
      final path = picked.files.first.path;
      if (path == null || path.isEmpty) throw StateError('无法读取所选配置文件');
      final raw = await File(path).readAsString();
      final api = ref.read(backendServiceProvider);
      final preview = await api.post<Map<String, dynamic>>(
        '/api/config/import/preview',
        data: {'raw': raw},
      );
      if (preview?['valid'] != true) {
        throw StateError((preview?['error'] ?? '配置文件无效').toString());
      }
      if (!mounted) return;
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('确认导入配置'),
          content: Text(
            '共 ${preview?['itemCount'] ?? 0} 项配置；新增 ${preview?['newCount'] ?? 0} 项，修改 ${preview?['changed'] ?? 0} 项，未变化 ${preview?['unchanged'] ?? 0} 项。继续后会覆盖同名设置。',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('导入'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
      final result = await api.post<Map<String, dynamic>>(
        '/api/config/import/confirm',
        data: {'raw': raw},
      );
      if (result?['imported'] != true) {
        throw StateError((result?['error'] ?? '配置导入失败').toString());
      }
      _show('已导入 ${result?['importedCount'] ?? 0} 项配置');
    });
  }

  Future<void> _createBackup() async {
    final passphrase = await _askPassphrase('创建完整备份');
    if (passphrase == null) return;
    await _run('备份', () async {
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/data-portability/backups',
            data: {'passphrase': passphrase},
          );
      if ((result?['backupId'] ?? '').toString().isEmpty) {
        throw StateError('备份未创建');
      }
      _show('当前设备完整灾难恢复备份已创建');
      await _loadBackups();
    });
  }

  Future<void> _checkMigrations() async {
    await _run('迁移检查', () async {
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>('/api/storage/migrations/check');
      final needs = result?['needsMigration'] == true;
      _show(needs ? '检测到待处理的数据迁移' : '数据结构已是最新状态');
    });
  }

  Future<void> _cleanupTemp() async {
    await _run('缓存清理', () async {
      await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>('/api/runtime/cleanup-temp');
      _show('临时缓存已清理');
    });
  }

  Future<void> _restoreBackup(Map<String, dynamic> backup) async {
    final name = (backup['name'] ?? '').toString();
    if (name.isEmpty) return;
    final passphrase = backup['encrypted'] == true
        ? await _askPassphrase('解密备份')
        : '';
    if (passphrase == null) return;
    await _run('备份预检', () async {
      final preview = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/data-portability/backups/${Uri.encodeComponent(name)}/restore/preview',
            data: {'passphrase': passphrase},
          );
      final operationId = (preview?['operationId'] ?? '').toString();
      if (operationId.isEmpty) throw StateError('恢复预检未返回操作票据');
      final manifest = preview?['manifest'] is Map
          ? Map<String, dynamic>.from(preview!['manifest'] as Map)
          : <String, dynamic>{};
      final components = manifest['components'] is List
          ? (manifest['components'] as List).whereType<Map>().toList()
          : const <Map>[];
      if (!mounted) return;
      final confirmed = await showDialog<bool>(
        context: context,
        barrierDismissible: false,
        builder: (dialogContext) => AlertDialog(
          title: const Text('确认恢复完整备份'),
          content: SingleChildScrollView(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('备份：$name'),
                Text('应用版本：${manifest['appVersion'] ?? '—'}'),
                Text('范围：${_scopeLabel((manifest['scope'] ?? '').toString())}'),
                const SizedBox(height: 12),
                ...components.map(
                  (component) => Text(
                    '${component['logicalName'] ?? component['id']} · ${component['itemCount'] ?? 0} 条',
                  ),
                ),
                const SizedBox(height: 12),
                const Text('执行前会自动创建恢复前备份。恢复成功后需要重新启动应用。'),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('取消'),
            ),
            FilledButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('确认恢复'),
            ),
          ],
        ),
      );
      if (confirmed != true) {
        await ref
            .read(backendServiceProvider)
            .delete('/api/data-portability/restores/pending');
        return;
      }
      final result = await ref
          .read(backendServiceProvider)
          .post<Map<String, dynamic>>(
            '/api/data-portability/restores/${Uri.encodeComponent(operationId)}',
            data: const {},
          );
      if (result?['status'] != 'completed') throw StateError('恢复未完成');
      _show('备份已恢复，请重新启动应用');
    });
  }

  Future<void> _deleteBackup(Map<String, dynamic> backup) async {
    final name = (backup['name'] ?? '').toString();
    if (name.isEmpty) return;
    await _run('删除备份', () async {
      await ref
          .read(backendServiceProvider)
          .delete('/api/data-portability/backups/${Uri.encodeComponent(name)}');
      _show('备份已删除');
      await _loadBackups();
    });
  }

  String _formatBytes(dynamic raw) {
    final bytes = raw is num
        ? raw.toInt()
        : int.tryParse(raw?.toString() ?? '') ?? 0;
    if (bytes >= 1024 * 1024 * 1024) {
      return '${(bytes / (1024 * 1024 * 1024)).toStringAsFixed(2)} GB';
    }
    if (bytes >= 1024 * 1024) {
      return '${(bytes / (1024 * 1024)).toStringAsFixed(2)} MB';
    }
    if (bytes >= 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '$bytes B';
  }

  String _scopeLabel(String scope) {
    return const {
          'all': '全部数据',
          'character': '角色及关联数据',
          'settings': '设置与集成',
          'memory': '记忆',
        }[scope] ??
        scope;
  }

  @override
  Widget build(BuildContext context) {
    final charactersAsync = ref.watch(characterListProvider);
    final conversationsAsync = ref.watch(conversationListProvider);
    final memoriesAsync = ref.watch(ownedMemoryCountProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '数据与备份',
        showBackButton: true,
        fallbackRoute: AppRoutes.settings,
      ),
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
                memoriesCount: memoriesAsync.valueOrNull ?? 0,
                isLoading:
                    charactersAsync.isLoading ||
                    conversationsAsync.isLoading ||
                    memoriesAsync.isLoading,
              ),
              SizedBox(height: AppSpacing.sectionGap),
              Text('操作', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.md),
              _ActionGrid(
                actions: [
                  ('迁移导出', Icons.file_upload_outlined, _exportData),
                  ('迁移导入', Icons.file_download_outlined, _importData),
                  ('完整备份', Icons.save_outlined, _createBackup),
                  ('迁移检查', Icons.sync_alt_outlined, _checkMigrations),
                  ('清理缓存', Icons.cleaning_services_outlined, _cleanupTemp),
                  (
                    '导出配置',
                    Icons.settings_backup_restore_outlined,
                    _exportConfig,
                  ),
                  ('导入配置', Icons.settings_suggest_outlined, _importConfig),
                ],
              ),
              SizedBox(height: AppSpacing.sectionGap),
              Text(
                '迁移导出与导入先经过范围预检、归档校验和冲突确认。完整备份只用于当前设备 Runtime 灾难恢复；Cloud 模式下不会直接覆盖共享 Cloud Core 数据库。',
                style: AppTypography.body(
                  context,
                ).copyWith(color: context.textTertiary),
              ),
              SizedBox(height: AppSpacing.md),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('完整灾难恢复备份', style: AppTypography.sectionTitle(context)),
                  IconButton(
                    icon: Icon(
                      Icons.refresh,
                      size: 20,
                      color: context.textTertiary,
                    ),
                    onPressed: _loadingBackups ? null : _loadBackups,
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              if (_loadingBackups)
                const Padding(
                  padding: EdgeInsets.all(24),
                  child: Center(child: CircularProgressIndicator()),
                )
              else if (_backupsError != null)
                Padding(
                  padding: EdgeInsets.all(AppSpacing.md),
                  child: Text(
                    '加载失败: $_backupsError',
                    style: TextStyle(color: context.error),
                  ),
                )
              else if (_backups.isEmpty)
                Padding(
                  padding: EdgeInsets.all(AppSpacing.lg),
                  child: Center(
                    child: Text(
                      '暂无备份记录',
                      style: AppTypography.body(
                        context,
                      ).copyWith(color: context.textTertiary),
                    ),
                  ),
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
      (
        '对话数据',
        isLoading ? '...' : '$conversationsCount 条',
        Icons.chat_outlined,
      ),
      ('角色数据', isLoading ? '...' : '$charactersCount 个', Icons.people_outline),
      ('记忆数据', isLoading ? '...' : '$memoriesCount 条', Icons.memory),
    ];
    return Container(
      padding: EdgeInsets.symmetric(
        vertical: AppSpacing.lg,
        horizontal: AppSpacing.sm,
      ),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          for (final item in items)
            _DataItem(label: item.$1, value: item.$2, icon: item.$3),
        ],
      ),
    );
  }
}

class _DataItem extends StatelessWidget {
  final String label;
  final String value;
  final IconData icon;

  const _DataItem({
    required this.label,
    required this.value,
    required this.icon,
  });

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: Column(
        children: [
          Container(
            width: 32,
            height: 32,
            decoration: BoxDecoration(
              color: context.accentSoft,
              shape: BoxShape.circle,
            ),
            child: Icon(icon, size: 16, color: context.accentPrimary),
          ),
          SizedBox(height: AppSpacing.sm),
          Text(value, style: AppTypography.cardTitle(context)),
          const SizedBox(height: 2),
          Text(
            label,
            style: AppTypography.label(context),
            textAlign: TextAlign.center,
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
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
      children: actions
          .map((action) {
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
                    Text(
                      action.$1,
                      style: AppTypography.bodySmall(context),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
            );
          })
          .toList(growable: false),
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
                Text(
                  name,
                  style: AppTypography.bodySmall(context),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text('$time · $size', style: AppTypography.label(context)),
              ],
            ),
          ),
          IconButton(
            onPressed: onRestore,
            tooltip: '恢复',
            icon: Icon(Icons.restore, size: 20, color: context.accentPrimary),
          ),
          IconButton(
            onPressed: onDelete,
            tooltip: '删除',
            icon: Icon(Icons.delete_outline, size: 20, color: context.error),
          ),
        ],
      ),
    );
  }
}
