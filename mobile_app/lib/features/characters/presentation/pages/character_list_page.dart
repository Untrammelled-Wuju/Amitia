import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class CharacterListPage extends ConsumerWidget {
  const CharacterListPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色',
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.sm,
                AppSpacing.pagePadding,
                AppSpacing.sm,
              ),
              child: const AmitiaSearchField(hintText: '搜索角色'),
            ),
            Expanded(
              child: ListView.separated(
                padding: const EdgeInsets.fromLTRB(
                  AppSpacing.pagePadding,
                  AppSpacing.xs,
                  AppSpacing.pagePadding,
                  AppSpacing.md,
                ),
                itemCount: MockData.characters.length,
                separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                itemBuilder: (context, index) {
                  final character = MockData.characters[index];
                  return AmitiaCharacterCard(
                    name: character.name,
                    status: character.status,
                    identity: character.identity,
                    avatarInitial: character.avatarInitial,
                    avatarColor: character.avatarColor,
                    mood: character.mood,
                    lastActive: _getLastActive(character),
                    onTap: () => context.push(AppRoutes.character(character.id)),
                  );
                },
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.xs,
                AppSpacing.pagePadding,
                AppSpacing.lg,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: AmitiaButton(
                      label: '创建新角色',
                      icon: Icons.person_add_alt_1,
                      isFullWidth: true,
                      onPressed: () => context.push(AppRoutes.charactersCreate),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: AmitiaButton(
                      label: '管理角色',
                      isSecondary: true,
                      isFullWidth: true,
                      onPressed: () => _showManageSheet(context),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _getLastActive(Character character) {
    if (character.status == '在线') {
      return '刚刚活跃';
    }
    return '2小时前活跃';
  }

  void _showManageSheet(BuildContext context) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text('管理角色', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.sort, context.accentPrimary),
                  title: '排序',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('已打开排序选项')),
                    );
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.star_outline, context.accentPrimary),
                  title: '设为默认',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('请选择要设为默认的角色')),
                    );
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.copy_outlined, context.accentPrimary),
                  title: '复制',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('请选择要复制的角色')),
                    );
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.file_download_outlined, context.accentPrimary),
                  title: '导出',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('请选择要导出的角色')),
                    );
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.archive_outlined, context.accentPrimary),
                  title: '归档',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('请选择要归档的角色')),
                    );
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.delete_outline, context.error),
                  title: '删除',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    ScaffoldMessenger.of(context).showSnackBar(
                      const SnackBar(content: Text('请选择要删除的角色')),
                    );
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildSheetIcon(BuildContext context, IconData icon, Color color) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: color),
    );
  }
}
