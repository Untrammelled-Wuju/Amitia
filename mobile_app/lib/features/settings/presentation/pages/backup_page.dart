import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class BackupPage extends ConsumerStatefulWidget {
  const BackupPage({super.key});

  @override
  ConsumerState<BackupPage> createState() => _BackupPageState();
}

class _BackupPageState extends ConsumerState<BackupPage> {
  final _backups = <(String, String, String)>[
    ('2026-07-29 20:15', '128.4 MB', '本地'),
    ('2026-07-25 09:30', '124.1 MB', '本地'),
    ('2026-07-20 14:00', '119.8 MB', '本地'),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '数据与备份', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text('数据概览', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          const _DataOverview(),
          const SizedBox(height: AppSpacing.sectionGap),
          Text('操作', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          const _ActionGrid(),
          const SizedBox(height: AppSpacing.sectionGap),
          Text('本地备份', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          ..._backups.map(
            (b) => Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: _BackupRecord(time: b.$1, size: b.$2, source: b.$3),
            ),
          ),
        ],
      ),
    );
  }
}

class _DataOverview extends StatelessWidget {
  const _DataOverview();

  @override
  Widget build(BuildContext context) {
    final items = <(String, String, IconData)>[
      ('对话数据', '342 条', Icons.chat_outlined),
      ('角色数据', '4 个', Icons.people_outline),
      ('记忆数据', '128 条', Icons.memory),
      ('插件数据', '9 个', Icons.extension_outlined),
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
  const _ActionGrid();

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
          .map((a) => _ActionButton(label: a.$1, icon: a.$2))
          .toList(),
    );
  }
}

class _ActionButton extends StatelessWidget {
  final String label;
  final IconData icon;

  const _ActionButton({required this.label, required this.icon});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: () {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('$label · 处理中'),
            duration: const Duration(seconds: 1),
          ),
        );
      },
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
