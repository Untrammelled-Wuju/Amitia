import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/backend_transport/backend_service_api.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/services/providers.dart';

class BackupPage extends ConsumerStatefulWidget {
  const BackupPage({super.key});

  @override
  ConsumerState<BackupPage> createState() => _BackupPageState();
}

class _BackupPageState extends ConsumerState<BackupPage> {
  List<Map<String, dynamic>> _backups = [];
  bool _loadingBackups = true;
  String? _backupsError;

  @override
  void initState() {
    super.initState();
    _loadBackups();
  }

  Future<void> _loadBackups() async {
    setState(() { _loadingBackups = true; _backupsError = null; });
    try {
      final apiClient = ref.watch(backendServiceProvider);
      if (apiClient == null) {
        if (mounted) {
          setState(() { _backupsError = '后端服务未连接'; _loadingBackups = false; });
        }
        return;
      }
      final resp = await apiClient.get<List<dynamic>>('/api/maintenance/backups');
      if (mounted) {
        final list = resp ?? [];
        setState(() {
          _backups = list.map((e) => Map<String, dynamic>.from(e as Map)).toList();
          _loadingBackups = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _backupsError = e.toString(); _loadingBackups = false; });
    }
  }

  Future<void> _handleAction(String label) async {
    final svc = ref.read(systemServiceProvider);
    final apiClient = ref.watch(backendServiceProvider);
    if (apiClient == null) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$label · 失败: 后端服务未连接'), duration: const Duration(seconds: 2)),
        );
      }
      return;
    }
    try {
      switch (label) {
        case '导出数据':
          await svc.export({});
          if (mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('导出完成'), duration: Duration(seconds: 1)),
            );
          }
          break;
        case '本地备份':
          await apiClient.post('/api/maintenance/backups');
          if (mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              const SnackBar(content: Text('备份创建成功'), duration: Duration(seconds: 1)),
            );
            _loadBackups();
          }
          break;
        default:
          if (mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text('$label · 处理中'), duration: const Duration(seconds: 1)),
            );
          }
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('$label · 失败: $e'), duration: const Duration(seconds: 2)),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final charactersAsync = ref.watch(characterListProvider);
    final conversationsAsync = ref.watch(conversationListProvider);
    final memoriesAsync = ref.watch(memoryListProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '数据与备份', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text('数据概览', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          _DataOverview(
            charactersCount: charactersAsync.valueOrNull?.length ?? 0,
            conversationsCount: conversationsAsync.valueOrNull?.length ?? 0,
            memoriesCount: memoriesAsync.valueOrNull?.length ?? 0,
            isLoading: charactersAsync.isLoading || conversationsAsync.isLoading || memoriesAsync.isLoading,
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          Text('操作', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          _ActionGrid(onAction: _handleAction),
          const SizedBox(height: AppSpacing.sectionGap),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('本地备份', style: AppTypography.sectionTitle(context)),
              IconButton(
                icon: Icon(Icons.refresh, size: 20, color: context.textTertiary),
                onPressed: _loadBackups,
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.md),
          if (_loadingBackups)
            const Padding(
              padding: EdgeInsets.all(AppSpacing.lg),
              child: Center(child: CircularProgressIndicator()),
            )
          else if (_backupsError != null)
            Padding(
              padding: const EdgeInsets.all(AppSpacing.md),
              child: Text('加载失败: $_backupsError', style: TextStyle(color: context.error)),
            )
          else if (_backups.isEmpty)
            Padding(
              padding: const EdgeInsets.all(AppSpacing.lg),
              child: Center(
                child: Text('暂无备份记录', style: AppTypography.body(context).copyWith(color: context.textTertiary)),
              ),
            )
          else
            ..._backups.map(
              (b) => Padding(
                padding: const EdgeInsets.only(bottom: AppSpacing.sm),
                child._BackupRecord(
                  time: (b['time'] ?? b['created_at'] ?? '').toString(),
                  size: (b['size'] ?? '').toString(),
                  source: (b['source'] ?? '本地').toString(),
                ),
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
      padding: const EdgeInsets.symmetric(
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
          for (final item in items) _DataItem(label: item.$1, value: item.$2, icon: item.$3),
        ],
      ),
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
            decoration: BoxDecoration(
              color: context.accentSoft,
              shape: BoxShape.circle,
            ),
            child: Icon(icon, size: 16, color: context.accentPrimary),
          ),
          const SizedBox(height: AppSpacing.sm),
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
  final void Function(String) onAction;

  const _ActionGrid({required this.onAction});

  @override
  Widget build(BuildContext context) {
    final actions = <(String, IconData)>[
      ('导出数据', Icons.file_upload_outlined),
      ('导入数据', Icons.file_download_outlined),
      ('本地备份', Icons.save_outlined),
      ('云端备份', Icons.cloud_upload_outlined),
      ('清理缓存', Icons.cleaning_services_outlined),
    ];
    return GridView.count(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 3,
      mainAxisSpacing: AppSpacing.md,
      crossAxisSpacing: AppSpacing.md,
      childAspectRatio: 1.5,
      children: actions
          .map((a) => _ActionButton(label: a.$1, icon: a.$2, onAction: onAction))
          .toList(),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final String label;
  final IconData icon;
  final void Function(String) onAction;

  const _ActionButton({required this.label, required this.icon, required this.onAction});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () => onAction(label),
      child: Container(
        decoration: BoxDecoration(
          color: context.surfacePrimary,
          borderRadius: AppRadius.brMedium,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 22, color: context.accentPrimary),
            const SizedBox(height: 6),
            Text(
              label,
              style: AppTypography.bodySmall(context),
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      ),
    );
  }
}

class _BackupRecord extends StatelessWidget {
  final String time;
  final String size;
  final String source;

  const _BackupRecord({
    required this.time,
    required this.size,
    required this.source,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.lg,
        vertical: 14,
      ),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Row(
        children: [
          Icon(Icons.history, size: 20, color: context.textTertiary),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(time, style: AppTypography.bodySmall(context)),
                const SizedBox(height: 2),
                Text('$source备份 · $size', style: AppTypography.label(context)),
              ],
            ),
          ),
          Icon(Icons.restore, size: 20, color: context.textTertiary),
        ],
      ),
    );
  }
}
