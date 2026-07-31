import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class StoragePage extends ConsumerStatefulWidget {
  const StoragePage({super.key});

  @override
  ConsumerState<StoragePage> createState() => _StoragePageState();
}

class _StoragePageState extends ConsumerState<StoragePage> {
  bool _dbHealthy = true;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '存储与备份', showBackButton: true, fallbackRoute: AppRoutes.settings),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          _SectionLabel(text: '数据库健康'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard([
            _buildInfoTile('SQLite 主库', '正常', BadgeType.success),
            _divider(),
            _buildInfoTile('SurrealDB', '正常', BadgeType.success),
            _divider(),
            _buildInfoTile('Qdrant 向量库', '正常', BadgeType.success),
            _divider(),
            _buildInfoTile('数据库完整性', '通过', BadgeType.success),
          ]),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '存储占用'),
          const SizedBox(height: AppSpacing.sm),
          _buildCard(
            MockSettings.storageInfo.map((s) => Column(children: [
              _buildStorageTile(s),
              if (s != MockSettings.storageInfo.last) _divider(),
            ])).expand((w) => [w]).toList(),
          ),
          const SizedBox(height: AppSpacing.sectionGap),
          _SectionLabel(text: '操作'),
          const SizedBox(height: AppSpacing.sm),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Column(
              children: [
                AmitiaButton(
                  label: '清理缓存',
                  icon: Icons.cleaning_services_outlined,
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _confirmCleanCache,
                ),
                const SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '数据迁移',
                  icon: Icons.swap_horiz,
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _confirmMigrate,
                ),
                const SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '加密备份',
                  icon: Icons.enhanced_encryption_outlined,
                  isFullWidth: true,
                  onPressed: _doEncryptBackup,
                ),
                const SizedBox(height: AppSpacing.sm),
                AmitiaButton(
                  label: '恢复备份',
                  icon: Icons.restore,
                  isFullWidth: true,
                  isSecondary: true,
                  onPressed: _showRestoreSheet,
                ),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }

  Widget _buildCard(List<Widget> children) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(children: children),
    );
  }

  Widget _divider() {
    return Padding(
      padding: const EdgeInsets.only(left: 56),
      child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
    );
  }

  Widget _buildInfoTile(String title, String value, BadgeType type) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Row(
        children: [
          Expanded(child: Text(title, style: AppTypography.body(context))),
          AmitiaStatusBadge(label: value, type: type),
        ],
      ),
    );
  }

  Widget _buildStorageTile(StorageInfo info) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 13),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 28,
                height: 28,
                decoration: BoxDecoration(color: info.color.withValues(alpha: 0.12), borderRadius: AppRadius.brExtraSmall),
                child: Icon(Icons.storage, size: 16, color: info.color),
              ),
              const SizedBox(width: 10),
              Expanded(child: Text(info.category, style: AppTypography.body(context))),
              Text(info.size, style: AppTypography.caption(context)),
            ],
          ),
          const SizedBox(height: 8),
          AmitiaProgressBar(progress: info.percentage / 100, color: info.color),
        ],
      ),
    );
  }

  void _confirmCleanCache() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('清理缓存', style: AppTypography.cardTitle(context)),
        content: Text('将清理 83 MB 缓存数据，此操作不可恢复。是否继续？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('缓存已清理 · 释放 83 MB'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('清理', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _confirmMigrate() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('数据迁移', style: AppTypography.cardTitle(context)),
        content: Text('将执行数据迁移操作，迁移期间服务可能短暂不可用。是否继续？', style: AppTypography.body(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('数据迁移完成'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('迁移', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _doEncryptBackup() {
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('加密备份已完成'), duration: Duration(seconds: 1)),
    );
  }

  void _showRestoreSheet() {
    final pwdCtrl = TextEditingController();
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.lg, AppSpacing.lg, AppSpacing.lg, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.lg),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('恢复加密备份', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('请输入备份文件密码', style: AppTypography.label(context)),
              const SizedBox(height: 4),
              AmitiaTextField(hintText: '输入密码', controller: pwdCtrl, obscureText: true),
              const SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '恢复',
                isFullWidth: true,
                onPressed: () {
                  Navigator.pop(ctx);
                  showDialog(
                    context: context,
                    builder: (dctx) => AlertDialog(
                      shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
                      title: Row(children: [
                        Icon(Icons.check_circle, color: context.success, size: 22),
                        const SizedBox(width: 8),
                        Text('恢复成功', style: AppTypography.cardTitle(context)),
                      ]),
                      content: Text('备份已成功恢复，请重启应用以应用变更。', style: AppTypography.body(context)),
                      actions: [
                        TextButton(onPressed: () => Navigator.pop(dctx), child: const Text('确定')),
                      ],
                    ),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _SectionLabel extends StatelessWidget {
  final String text;
  const _SectionLabel({required this.text});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(AppSpacing.pagePadding, AppSpacing.sm, AppSpacing.pagePadding, AppSpacing.sm),
      child: Text(text, style: AppTypography.caption(context)),
    );
  }
}
