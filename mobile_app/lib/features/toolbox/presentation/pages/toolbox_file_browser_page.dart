import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ToolboxFileBrowserPage extends StatelessWidget {
  const ToolboxFileBrowserPage({super.key});

  static const _files = <(IconData, String, String, String)>[
    (Icons.folder, 'Documents', '—', '2026-07-28'),
    (Icons.folder, 'Downloads', '—', '2026-07-30'),
    (Icons.folder, 'Pictures', '—', '2026-07-29'),
    (Icons.folder, 'Amitia', '—', '2026-07-31'),
    (Icons.picture_as_pdf_outlined, '产品需求文档.pdf', '2.0 MB', '2026-07-30'),
    (Icons.description_outlined, '周报模板.docx', '128 KB', '2026-07-29'),
    (Icons.image_outlined, '截图_01.png', '860 KB', '2026-07-29'),
    (Icons.music_note_outlined, '录音_001.m4a', '4.2 MB', '2026-07-28'),
    (Icons.code, 'config.json', '12 KB', '2026-07-31'),
    (Icons.archive_outlined, 'backup_0731.zip', '186 MB', '2026-07-31'),
  ];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '文件浏览', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Text('当前路径：/storage/emulated/0', style: AppTypography.caption(context)),
          ),
          const SizedBox(height: AppSpacing.md),
          Container(
            margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                for (int i = 0; i < _files.length; i++) ...[
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
                    child: Row(
                      children: [
                        Icon(_files[i].$1, size: 22, color: _files[i].$1 == Icons.folder ? context.accentPrimary : context.textSecondary),
                        const SizedBox(width: 14),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(_files[i].$2, style: AppTypography.body(context)),
                              Text('${_files[i].$3} · ${_files[i].$4}', style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
                      ],
                    ),
                  ),
                  if (i < _files.length - 1)
                    Padding(
                      padding: const EdgeInsets.only(left: 52),
                      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
                    ),
                ],
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}
