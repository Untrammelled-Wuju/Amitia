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
import '../../../../core/services/providers.dart';

class CharacterProactivePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterProactivePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterProactivePage> createState() => _CharacterProactivePageState();
}

class _CharacterProactivePageState extends ConsumerState<CharacterProactivePage> {
  bool _masterEnabled = true;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '主动消息',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showRuleEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildMasterSwitch(context),
            SizedBox(height: AppSpacing.sectionGap),
            FutureBuilder<List<Map<String, dynamic>>>(
              future: ref.read(proactiveServiceProvider).rules(),
              builder: (context, snapshot) {
                if (snapshot.connectionState == ConnectionState.waiting) {
                  return const Center(child: CircularProgressIndicator());
                }
                if (snapshot.hasError) {
                  return Center(child: Text('加载失败: ${snapshot.error}'));
                }
                final rules = snapshot.data ?? [];
                if (rules.isEmpty) {
                  return const Center(child: Text('暂无规则'));
                }
                return Column(
                  children: [
                    ...rules.map((r) => _buildRuleCard(context, r)),
                    SizedBox(height: AppSpacing.xxl),
                  ],
                );
              },
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildMasterSwitch(BuildContext context) {
    return FutureBuilder<Map<String, dynamic>?>(
      future: ref.read(proactiveServiceProvider).status(),
      builder: (context, snapshot) {
        final status = snapshot.data;
        final enabled = status?['enabled'] == true || _masterEnabled;
        return AmitiaCard(
          backgroundColor: enabled ? context.accentSoft : null,
          child: Row(
            children: [
              Container(
                width: 48,
                height: 48,
                decoration: BoxDecoration(
                  color: enabled ? context.accentPrimary : context.borderPrimary,
                  borderRadius: AppRadius.brSmall,
                ),
                child: const Icon(Icons.notifications_active, color: Colors.white, size: 24),
              ),
              SizedBox(width: AppSpacing.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('主动消息总开关', style: AppTypography.cardTitle(context)),
                    const SizedBox(height: 2),
                    Text(
                      enabled ? '已开启 - 角色将按规则主动发送消息' : '已关闭 - 角色不会主动发送消息',
                      style: AppTypography.caption(context),
                    ),
                  ],
                ),
              ),
              Switch(
                value: enabled,
                onChanged: (v) {
                  setState(() => _masterEnabled = v);
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('主动消息已${v ? '开启' : '关闭'}'), duration: const Duration(seconds: 1)),
                  );
                },
              ),
            ],
          ),
        );
      },
    );
  }

  Widget _buildRuleCard(BuildContext context, Map<String, dynamic> rule) {
    final name = rule['name']?.toString() ?? '';
    final trigger = rule['trigger']?.toString() ?? '';
    final time = rule['time']?.toString() ?? '';
    final probability = (rule['probability'] as num?)?.toInt() ?? 50;
    final cooldown = (rule['cooldown'] as num?)?.toInt() ?? 60;
    final enabled = rule['isEnabled'] == true || rule['enabled'] == true;
    final category = rule['category']?.toString() ?? '日常';
    final id = rule['id']?.toString() ?? '';

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 40,
                  height: 40,
                  decoration: BoxDecoration(
                    color: enabled ? context.accentSoft : context.surfaceSecondary,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(
                    _getCategoryIcon(category),
                    size: 20,
                    color: enabled ? context.accentPrimary : context.textTertiary,
                  ),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Text(name, style: AppTypography.cardTitle(context)),
                          SizedBox(width: AppSpacing.sm),
                          AmitiaStatusBadge(label: category, type: BadgeType.accent),
                        ],
                      ),
                      const SizedBox(height: 2),
                      Text('触发：$trigger · 时间：$time', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Switch(
                  value: enabled,
                  onChanged: (v) {
                    ref.read(proactiveServiceProvider).toggleRule(id, v);
                    setState(() {});
                  },
                ),
              ],
            ),
            SizedBox(height: AppSpacing.sm),
            Container(
              padding: EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Column(
                children: [
                  _buildParamRow(context, '触发概率', '$probability%', probability / 100, context.accentPrimary),
                  SizedBox(height: AppSpacing.sm),
                  _buildParamRow(context, '冷却时间', '$cooldown分钟', (cooldown / 180).clamp(0.0, 1.0), context.info),
                ],
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            Row(
              children: [
                GestureDetector(
                  onTap: () => _showRuleEditor(context, rule),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: context.accentSoft,
                      borderRadius: AppRadius.brTag,
                    ),
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
                  onTap: () => _testRule(id, name),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: context.success.withValues(alpha: 0.12),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.science_outlined, size: 14, color: context.success),
                        const SizedBox(width: 4),
                        Text('测试', style: TextStyle(fontSize: 12, color: context.success)),
                      ],
                    ),
                  ),
                ),
                const Spacer(),
                GestureDetector(
                  onTap: () => _deleteRule(id, name),
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                    decoration: BoxDecoration(
                      color: context.error.withValues(alpha: 0.1),
                      borderRadius: AppRadius.brTag,
                    ),
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
      ),
    );
  }

  Widget _buildParamRow(BuildContext context, String label, String value, double progress, Color color) {
    return Row(
      children: [
        SizedBox(
          width: 80,
          child: Text(label, style: AppTypography.label(context)),
        ),
        SizedBox(width: AppSpacing.sm),
        Expanded(
          child: AmitiaProgressBar(progress: progress, color: color),
        ),
        SizedBox(width: AppSpacing.sm),
        SizedBox(
          width: 60,
          child: Text(value, style: AppTypography.label(context).copyWith(color: color, fontWeight: FontWeight.w600), textAlign: TextAlign.right),
        ),
      ],
    );
  }

  IconData _getCategoryIcon(String category) {
    switch (category) {
      case '起床':
        return Icons.wb_sunny_outlined;
      case '吃饭':
        return Icons.restaurant_outlined;
      case '午睡':
        return Icons.bedtime_outlined;
      case '睡觉':
        return Icons.nights_stay_outlined;
      case '工作':
        return Icons.work_outline;
      default:
        return Icons.notifications_outlined;
    }
  }

  void _showRuleEditor(BuildContext context, Map<String, dynamic>? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?['name']?.toString() ?? '');
    final triggerCtrl = TextEditingController(text: existing?['trigger']?.toString() ?? '');
    final timeCtrl = TextEditingController(text: existing?['time']?.toString() ?? '08:00');
    int probability = (existing?['probability'] as num?)?.toInt() ?? 80;
    int cooldown = (existing?['cooldown'] as num?)?.toInt() ?? 60;
    String category = existing?['category']?.toString() ?? '日常';
    final id = existing?['id']?.toString() ?? '';

    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.xl,
            AppSpacing.lg,
            AppSpacing.xl,
            MediaQuery.of(ctx).viewInsets.bottom + AppSpacing.xl,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
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
              SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑规则' : '新建规则', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('规则名称', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: nameCtrl, hintText: '输入规则名称'),
              SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('触发场景', style: AppTypography.label(context)),
                        SizedBox(height: AppSpacing.xs),
                        AmitiaTextField(controller: triggerCtrl, hintText: '如：起床'),
                      ],
                    ),
                  ),
                  SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('触发时间', style: AppTypography.label(context)),
                        SizedBox(width: AppSpacing.xs),
                        AmitiaTextField(controller: timeCtrl, hintText: '08:00'),
                      ],
                    ),
                  ),
                ],
              ),
              SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.sm,
                children: ['起床', '吃饭', '午睡', '睡觉', '工作', '日常'].map((c) {
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
              SizedBox(height: AppSpacing.md),
              Text('触发概率：$probability%', style: AppTypography.label(context)),
              Slider(
                value: probability.toDouble(),
                min: 0,
                max: 100,
                divisions: 100,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => probability = v.round()),
              ),
              Text('冷却时间：$cooldown分钟', style: AppTypography.label(context)),
              Slider(
                value: cooldown.toDouble(),
                min: 10,
                max: 180,
                divisions: 17,
                activeColor: context.info,
                onChanged: (v) => setSheetState(() => cooldown = v.round()),
              ),
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () async {
                  if (nameCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(proactiveServiceProvider);
                  final data = {
                    'name': nameCtrl.text.trim(),
                    'trigger': triggerCtrl.text.trim(),
                    'time': timeCtrl.text.trim(),
                    'probability': probability,
                    'cooldown': cooldown,
                    'category': category,
                  };
                  if (isEdit) {
                    await svc.updateRule(id, data);
                  } else {
                    await svc.createRule(data);
                  }
                  setState(() {});
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(isEdit ? '规则已更新' : '规则已创建'), duration: const Duration(seconds: 1)),
                    );
                  }
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _testRule(String id, String name) async {
    final svc = ref.read(proactiveServiceProvider);
    await svc.triggerRule(id);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('已触发规则: $name'), duration: const Duration(seconds: 1)),
      );
    }
  }

  Future<void> _deleteRule(String id, String name) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除规则', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「$name」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      final svc = ref.read(proactiveServiceProvider);
      await svc.deleteRule(id);
      setState(() {});
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('规则已删除'), duration: Duration(seconds: 1)),
        );
      }
    }
  }
}
