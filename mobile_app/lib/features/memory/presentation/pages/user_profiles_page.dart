import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class UserProfilesPage extends ConsumerStatefulWidget {
  const UserProfilesPage({super.key});

  @override
  ConsumerState<UserProfilesPage> createState() => _UserProfilesPageState();
}

class _UserProfilesPageState extends ConsumerState<UserProfilesPage> {
  late List<UserProfile> _profiles;
  String _selectedCategory = '全部';
  final _categories = ['全部', '事实', '偏好', '习惯', '关系'];

  @override
  void initState() {
    super.initState();
    _profiles = List.from(MockMemory.userProfiles);
  }

  List<UserProfile> get _filteredProfiles {
    if (_selectedCategory == '全部') return _profiles;
    return _profiles.where((p) => p.category == _selectedCategory).toList();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '用户画像',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showProfileEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildCategoryTabs(context),
            Expanded(
              child: _filteredProfiles.isEmpty
                  ? AmitiaEmptyState(
                      icon: Icons.person_outline,
                      title: '暂无画像',
                      subtitle: '互动后将自动提取用户画像',
                      actionText: '新增画像',
                      onAction: () => _showProfileEditor(context, null),
                    )
                  : ListView.separated(
                      padding: const EdgeInsets.all(AppSpacing.pagePadding),
                      itemCount: _filteredProfiles.length,
                      separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                      itemBuilder: (context, index) => _buildProfileCard(context, _filteredProfiles[index]),
                    ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCategoryTabs(BuildContext context) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: _categories.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedCategory == _categories[index];
          return GestureDetector(
            onTap: () => setState(() => _selectedCategory = _categories[index]),
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
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

  Widget _buildProfileCard(BuildContext context, UserProfile profile) {
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
                  color: _getCategoryColor(context, profile.category).withValues(alpha: 0.12),
                  borderRadius: AppRadius.brSmall,
                ),
                child: Icon(_getCategoryIcon(profile.category), size: 20, color: _getCategoryColor(context, profile.category)),
              ),
              const SizedBox(width: AppSpacing.md),
              Expanded(
                child: Text(profile.fact, style: AppTypography.body(context)),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              AmitiaStatusBadge(label: profile.category, type: _getCategoryBadge(profile.category)),
              const SizedBox(width: AppSpacing.sm),
              Icon(Icons.source, size: 12, color: context.textTertiary),
              const SizedBox(width: 2),
              Text(profile.source, style: AppTypography.label(context)),
              const Spacer(),
              Text(_formatDate(profile.updated), style: AppTypography.label(context)),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              Text('置信度', style: AppTypography.label(context)),
              const SizedBox(width: AppSpacing.sm),
              Expanded(child: AmitiaProgressBar(progress: profile.confidence, color: _getConfidenceColor(context, profile.confidence))),
              const SizedBox(width: AppSpacing.sm),
              Text('${(profile.confidence * 100).round()}%', style: AppTypography.label(context).copyWith(color: _getConfidenceColor(context, profile.confidence), fontWeight: FontWeight.w600)),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
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
              const SizedBox(width: AppSpacing.sm),
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

  void _showProfileEditor(BuildContext context, UserProfile? existing) {
    final isEdit = existing != null;
    final factCtrl = TextEditingController(text: existing?.fact ?? '');
    final sourceCtrl = TextEditingController(text: existing?.source ?? '对话');
    String category = existing?.category ?? '事实';
    double confidence = existing?.confidence ?? 0.8;

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
              const SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑画像' : '新增画像', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('内容', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: factCtrl, maxLines: 3, hintText: '输入画像内容'),
              const SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['事实', '偏好', '习惯', '关系'].map((c) {
                  final isSelected = category == c;
                  return GestureDetector(
                    onTap: () => setSheetState(() => category = c),
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
              const SizedBox(height: AppSpacing.md),
              Text('来源', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: sourceCtrl, hintText: '如：对话、行为分析'),
              const SizedBox(height: AppSpacing.md),
              Text('置信度：${(confidence * 100).round()}%', style: AppTypography.label(context)),
              Slider(value: confidence, min: 0.0, max: 1.0, divisions: 20, activeColor: context.accentPrimary, onChanged: (v) => setSheetState(() => confidence = v)),
              const SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () {
                  if (factCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  setState(() {
                    if (isEdit) {
                      final idx = _profiles.indexWhere((p) => p.id == existing.id);
                      _profiles[idx] = UserProfile(
                        id: existing.id, category: category, fact: factCtrl.text.trim(),
                        confidence: confidence, source: sourceCtrl.text.trim(), updated: DateTime.now(),
                      );
                    } else {
                      _profiles.insert(0, UserProfile(
                        id: 'up${DateTime.now().millisecondsSinceEpoch}', category: category,
                        fact: factCtrl.text.trim(), confidence: confidence, source: sourceCtrl.text.trim(), updated: DateTime.now(),
                      ));
                    }
                  });
                  ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(isEdit ? '画像已更新' : '画像已创建'), duration: const Duration(seconds: 1)));
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, UserProfile profile) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除画像', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除这条画像吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() => _profiles.removeWhere((p) => p.id == profile.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('画像已删除'), duration: Duration(seconds: 1)));
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

  Color _getConfidenceColor(BuildContext context, double confidence) {
    if (confidence >= 0.85) return context.success;
    if (confidence >= 0.6) return context.warning;
    return context.error;
  }

  String _formatDate(DateTime date) {
    return '${date.month}/${date.day}';
  }
}
