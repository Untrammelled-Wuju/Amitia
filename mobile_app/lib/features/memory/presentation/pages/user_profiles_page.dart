import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/profile.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class UserProfilesPage extends ConsumerStatefulWidget {
  const UserProfilesPage({super.key});

  @override
  ConsumerState<UserProfilesPage> createState() => _UserProfilesPageState();
}

class _UserProfilesPageState extends ConsumerState<UserProfilesPage> {
  static const _categoryLabels = <String, String>{
    'personal_info': '个人信息',
    'preference': '偏好',
    'habit': '习惯',
    'fear': '恐惧',
    'relationship': '关系',
    'health': '健康',
    'plan': '计划',
  };

  String _selectedCategory = '';

  String _categoryLabel(String category) =>
      _categoryLabels[category] ?? (category.trim().isEmpty ? '未分类' : category);

  List<ProfileDto> _filtered(List<ProfileDto> profiles) {
    if (_selectedCategory.isEmpty) return profiles;
    return profiles.where((item) => item.category == _selectedCategory).toList(growable: false);
  }

  @override
  Widget build(BuildContext context) {
    final profilesAsync = ref.watch(profileListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '用户画像',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(icon: Icons.add, onPressed: () => _showEditor(null)),
        ],
      ),
      body: SafeArea(
        top: false,
        child: profilesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => AmitiaErrorState(
            message: error.toString().replaceFirst('Exception: ', ''),
            onRetry: () => ref.invalidate(profileListProvider),
          ),
          data: (profiles) {
            final filtered = _filtered(profiles);
            return Column(
              children: [
                _buildFilters(context),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.person_search_outlined,
                          title: '暂无画像',
                          subtitle: _selectedCategory.isEmpty
                              ? '互动后会自动提取，也可以手动新增'
                              : '当前分类暂无画像',
                          actionText: '新增画像',
                          onAction: () => _showEditor(null),
                        )
                      : RefreshIndicator(
                          onRefresh: () async => ref.invalidate(profileListProvider),
                          child: ListView.separated(
                            padding: EdgeInsets.all(AppSpacing.pagePadding),
                            itemCount: filtered.length,
                            separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                            itemBuilder: (context, index) => _buildCard(filtered[index]),
                          ),
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildFilters(BuildContext context) {
    final entries = <MapEntry<String, String>>[
      const MapEntry('', '全部'),
      ..._categoryLabels.entries,
    ];
    return SizedBox(
      height: 42,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: entries.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final entry = entries[index];
          final selected = entry.key == _selectedCategory;
          return GestureDetector(
            onTap: () => setState(() => _selectedCategory = entry.key),
            child: Container(
              alignment: Alignment.center,
              padding: const EdgeInsets.symmetric(horizontal: 12),
              decoration: BoxDecoration(
                color: selected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Text(
                entry.value,
                style: TextStyle(
                  fontSize: 13,
                  color: selected ? Colors.white : context.textSecondary,
                  fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildCard(ProfileDto profile) {
    final verified = profile.verifiedAt.trim().isNotEmpty;
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: context.accentSoft,
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(Icons.person_pin_outlined, color: context.accentPrimary, size: 20),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(profile.attributeName, style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 4),
                    Text(profile.attributeValue, style: AppTypography.bodySmall(context)),
                  ],
                ),
              ),
              AmitiaStatusBadge(
                label: '${profile.confidence.clamp(0, 100)}%',
                type: profile.confidence >= 80
                    ? BadgeType.success
                    : profile.confidence >= 50
                        ? BadgeType.warning
                        : BadgeType.error,
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Wrap(
            spacing: AppSpacing.sm,
            runSpacing: AppSpacing.xs,
            crossAxisAlignment: WrapCrossAlignment.center,
            children: [
              AmitiaStatusBadge(label: _categoryLabel(profile.category), type: BadgeType.neutral),
              if (verified) const AmitiaStatusBadge(label: '已确认', type: BadgeType.success),
              if (profile.source.trim().isNotEmpty)
                Text('来源：${profile.source}', style: AppTypography.label(context)),
              if (profile.createdAt.trim().isNotEmpty)
                Text(_formatDate(profile.createdAt), style: AppTypography.label(context)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              TextButton.icon(
                onPressed: () => _showEditor(profile),
                icon: const Icon(Icons.edit_outlined, size: 16),
                label: const Text('编辑'),
              ),
              TextButton.icon(
                onPressed: () => _delete(profile),
                icon: Icon(Icons.delete_outline, size: 16, color: context.error),
                label: Text('删除', style: TextStyle(color: context.error)),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _showEditor(ProfileDto? existing) async {
    final isEdit = existing != null;
    final nameController = TextEditingController(text: existing?.attributeName ?? '');
    final valueController = TextEditingController(text: existing?.attributeValue ?? '');
    var category = existing?.category ?? 'personal_info';
    var confidence = (existing?.confidence ?? 50).toDouble().clamp(0, 100);

    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.lg,
            AppSpacing.xl,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.xl,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)),
                  ),
                ),
                SizedBox(height: AppSpacing.lg),
                Text(isEdit ? '编辑画像' : '新增画像', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.lg),
                Text('分类', style: AppTypography.label(context)),
                SizedBox(height: AppSpacing.xs),
                DropdownButtonFormField<String>(
                  value: category,
                  decoration: const InputDecoration(border: OutlineInputBorder()),
                  items: _categoryLabels.entries
                      .map((entry) => DropdownMenuItem(value: entry.key, child: Text(entry.value)))
                      .toList(growable: false),
                  onChanged: isEdit
                      ? null
                      : (value) {
                          if (value != null) setSheetState(() => category = value);
                        },
                ),
                SizedBox(height: AppSpacing.md),
                Text('属性名称', style: AppTypography.label(context)),
                SizedBox(height: AppSpacing.xs),
                AmitiaTextField(
                  controller: nameController,
                  hintText: '例如：常用称呼、喜欢的饮料',
                  readOnly: isEdit,
                ),
                SizedBox(height: AppSpacing.md),
                Text('属性值', style: AppTypography.label(context)),
                SizedBox(height: AppSpacing.xs),
                AmitiaTextField(controller: valueController, hintText: '输入画像事实', maxLines: 3),
                SizedBox(height: AppSpacing.md),
                Row(
                  children: [
                    Text('置信度', style: AppTypography.label(context)),
                    const Spacer(),
                    Text('${confidence.round()}%', style: AppTypography.bodySmall(context)),
                  ],
                ),
                Slider(
                  value: confidence,
                  min: 0,
                  max: 100,
                  divisions: 20,
                  onChanged: (value) => setSheetState(() => confidence = value),
                ),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: isEdit ? '保存' : '创建',
                  isFullWidth: true,
                  onPressed: () async {
                    final attributeName = nameController.text.trim();
                    final attributeValue = valueController.text.trim();
                    if (attributeName.isEmpty || attributeValue.isEmpty) {
                      ScaffoldMessenger.of(sheetContext).showSnackBar(const SnackBar(content: Text('属性名称和值不能为空')));
                      return;
                    }
                    try {
                      final service = ref.read(profileServiceProvider);
                      if (isEdit) {
                        await service.update(existing.id, {
                          'attributeValue': attributeValue,
                          'confidence': confidence.round(),
                        });
                      } else {
                        await service.create({
                          'category': category,
                          'attributeName': attributeName,
                          'attributeValue': attributeValue,
                          'confidence': confidence.round(),
                          'source': 'manual',
                        });
                      }
                      ref.invalidate(profileListProvider);
                      if (sheetContext.mounted) Navigator.pop(sheetContext);
                    } catch (error) {
                      if (sheetContext.mounted) {
                        ScaffoldMessenger.of(sheetContext).showSnackBar(
                          SnackBar(content: Text('保存失败：${error.toString().replaceFirst('Exception: ', '')}')),
                        );
                      }
                    }
                  },
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Future<void> _delete(ProfileDto profile) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('删除画像'),
        content: Text('确定删除“${profile.attributeName}：${profile.attributeValue}”吗？'),
        actions: [
          TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
          TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: Text('删除', style: TextStyle(color: context.error))),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(profileServiceProvider).delete(profile.id);
      ref.invalidate(profileListProvider);
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败：$error')));
      }
    }
  }

  String _formatDate(String value) {
    try {
      final date = DateTime.parse(value);
      return '${date.year}-${date.month.toString().padLeft(2, '0')}-${date.day.toString().padLeft(2, '0')}';
    } catch (_) {
      return value;
    }
  }
}
