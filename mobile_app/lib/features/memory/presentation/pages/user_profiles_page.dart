import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/profile.dart';

class UserProfilesPage extends ConsumerStatefulWidget {
  const UserProfilesPage({super.key});

  @override
  ConsumerState<UserProfilesPage> createState() => _UserProfilesPageState();
}

class _UserProfilesPageState extends ConsumerState<UserProfilesPage> {
  String _selectedCategory = '全部';
  final _categories = ['全部', '事实', '偏好', '习惯', '关系'];

  List<ProfileDto> _filteredProfiles(List<ProfileDto> profiles) {
    if (_selectedCategory == '全部') return profiles;
    return profiles.where((p) => _getCategory(p) == _selectedCategory).toList();
  }

  String _getCategory(ProfileDto p) {
    if (p.occupation.isNotEmpty) return '事实';
    if (p.personality.isNotEmpty) return '偏好';
    if (p.background.isNotEmpty) return '习惯';
    return '事实';
  }

  String _getProfileFact(ProfileDto p) {
    if (p.background.isNotEmpty) return p.background;
    if (p.personality.isNotEmpty) return p.personality;
    if (p.occupation.isNotEmpty) return p.occupation;
    return p.name;
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
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showProfileEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: profilesAsync.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (err, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(32),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                  const SizedBox(height: 16),
                  Text('加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                    style: AppTypography.body(context).copyWith(color: context.error),
                    textAlign: TextAlign.center),
                  const SizedBox(height: 16),
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(profileListProvider)),
                ],
              ),
            ),
          ),
          data: (profiles) {
            final filtered = _filteredProfiles(profiles);
            return Column(
              children: [
                _buildCategoryTabs(context),
                Expanded(
                  child: filtered.isEmpty
                      ? AmitiaEmptyState(
                          icon: Icons.person_outline,
                          title: '暂无画像',
                          subtitle: '互动后将自动提取用户画像',
                          actionText: '新增画像',
                          onAction: () => _showProfileEditor(context, null),
                        )
                      : ListView.separated(
                          padding: EdgeInsets.all(AppSpacing.pagePadding),
                          itemCount: filtered.length,
                          separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                          itemBuilder: (context, index) => _buildProfileCard(context, filtered[index]),
                        ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildCategoryTabs(BuildContext context) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: _categories.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedCategory == _categories[index];
          return GestureDetector(
            onTap: () => setState(() => _selectedCategory = _categories[index]),
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(
                  _categories[index],
                  style: TextStyle(fontSize: 13, fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400, color: isSelected ? Colors.white : context.textSecondary),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildProfileCard(BuildContext context, ProfileDto profile) {
    final category = _getCategory(profile);
    final fact = _getProfileFact(profile);
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: _getCategoryColor(context, category).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(_getCategoryIcon(category), size: 20, color: _getCategoryColor(context, category)),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(fact, style: AppTypography.body(context)),
              ),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              AmitiaStatusBadge(label: category, type: _getCategoryBadge(category)),
              SizedBox(width: AppSpacing.sm),
              Icon(Icons.source, size: 12, color: context.textTertiary),
              const SizedBox(width: 2),
              Text(profile.gender.isNotEmpty ? profile.gender : '系统生成', style: AppTypography.label(context)),
              const Spacer(),
              Text(_formatDateString(profile.createdAt), style: AppTypography.label(context)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              GestureDetector(
                onTap: () => _showProfileEditor(context, profile),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: context.accentSoft, borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.edit_outlined, size: 14, color: context.accentPrimary),
                      const SizedBox(width: 4),
                      Text('编辑', style: TextStyle(fontSize: 12, color: context.accentPrimary)),
                    ],
                  ),
                ),
              ),
              SizedBox(width: AppSpacing.sm),
              GestureDetector(
                onTap: () => _showDeleteConfirm(context, profile),
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(color: context.error.withValues(alpha: 0.1), borderRadius: AppRadius.brTag),
                  child: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Icon(Icons.delete_outline, size: 14, color: context.error),
                      const SizedBox(width: 4),
                      Text('删除', style: TextStyle(fontSize: 12, color: context.error)),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  void _showProfileEditor(BuildContext context, ProfileDto? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final occupationCtrl = TextEditingController(text: existing?.occupation ?? '');
    final personalityCtrl = TextEditingController(text: existing?.personality ?? '');
    final backgroundCtrl = TextEditingController(text: existing?.background ?? '');
    String gender = existing?.gender ?? '未知';

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(AppSpacing.xl, AppSpacing.lg, AppSpacing.xl, MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Center(child: Container(width: 40, height: 4, decoration: BoxDecoration(color: context.borderPrimary, borderRadius: BorderRadius.circular(2)))),
              SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑画像' : '新增画像', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('名称', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: nameCtrl, hintText: '输入名称'),
              SizedBox(height: AppSpacing.md),
              Text('职业', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: occupationCtrl, hintText: '输入职业'),
              SizedBox(height: AppSpacing.md),
              Text('性格', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: personalityCtrl, maxLines: 2, hintText: '输入性格描述'),
              SizedBox(height: AppSpacing.md),
              Text('背景', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: backgroundCtrl, maxLines: 2, hintText: '输入背景信息'),
              SizedBox(height: AppSpacing.md),
              Text('性别', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['未知', '男', '女'].map((c) {
                  final isSelected = gender == c;
                  return GestureDetector(
                    onTap: () => setSheetState(() => gender = c),
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                      decoration: BoxDecoration(
                        color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                        borderRadius: AppRadius.brTag,
                      ),
                      child: Text(c, style: TextStyle(fontSize: 13, color: isSelected ? Colors.white : context.textSecondary)),
                    ),
                  );
                }).toList(),
              ),
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (nameCtrl.text.trim().isEmpty && occupationCtrl.text.trim().isEmpty && personalityCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(profileServiceProvider);
                  final data = {
                    'name': nameCtrl.text.trim(),
                    'occupation': occupationCtrl.text.trim(),
                    'personality': personalityCtrl.text.trim(),
                    'background': backgroundCtrl.text.trim(),
                    'gender': gender,
                  };
                  try {
                    if (isEdit) {
                      await svc.update(existing.id, data);
                    } else {
                      await svc.create(data);
                    }
                    ref.invalidate(profileListProvider);
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '画像已更新' : '画像已创建'), duration: const Duration(seconds: 1)));
                    }
                  } catch (e) {
                    if (mounted) {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('操作失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                    }
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, ProfileDto profile) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除画像', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除这条画像吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () async {
              Navigator.pop(ctx);
              try {
                final svc = ref.read(profileServiceProvider);
                await svc.delete(profile.id);
                ref.invalidate(profileListProvider);
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('画像已删除'), duration: Duration(seconds: 1)));
                }
              } catch (e) {
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('删除失败: ${e.toString().replaceFirst('Exception: ', '')}')));
                }
              }
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  Color _getCategoryColor(BuildContext context, String category) {
    switch (category) {
      case '事实': return context.accentPrimary;
      case '偏好': return context.info;
      case '习惯': return context.success;
      case '关系': return context.warning;
      default: return context.textSecondary;
    }
  }

  IconData _getCategoryIcon(String category) {
    switch (category) {
      case '事实': return Icons.fact_check_outlined;
      case '偏好': return Icons.thumb_up_alt_outlined;
      case '习惯': return Icons.repeat;
      case '关系': return Icons.people_outline;
      default: return Icons.person_outline;
    }
  }

  BadgeType _getCategoryBadge(String category) {
    switch (category) {
      case '事实': return BadgeType.accent;
      case '偏好': return BadgeType.info;
      case '习惯': return BadgeType.success;
      case '关系': return BadgeType.warning;
      default: return BadgeType.neutral;
    }
  }

  String _formatDateString(String timeStr) {
    if (timeStr.isEmpty) return '';
    try {
      final date = DateTime.parse(timeStr);
      return '${date.month}/${date.day}';
    } catch (_) {
      return timeStr;
    }
  }
}
