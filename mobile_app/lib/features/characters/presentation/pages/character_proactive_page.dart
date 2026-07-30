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

class CharacterProactivePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterProactivePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterProactivePage> createState() => _CharacterProactivePageState();
}

class _CharacterProactivePageState extends ConsumerState<CharacterProactivePage> {
  late List<ProactiveRule> _rules;
  bool _masterEnabled = true;

  @override
  void initState() {
    super.initState();
    _rules = List.from(MockCharacters.proactiveRules);
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '主动消息',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '恢复默认',
            onPressed: () => _showRestoreDefaultConfirm(context),
          ),
          AmitiaIconButton(
            icon: Icons.add,
            onPressed: () => _showRuleEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildMasterSwitch(context),
            const SizedBox(height: AppSpacing.sectionGap),
            ..._rules.map((r) => _buildRuleCard(context, r)),
            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildMasterSwitch(BuildContext context) {
    return AmitiaCard(
      backgroundColor: _masterEnabled ? context.accentSoft : null,
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: _masterEnabled ? context.accentPrimary : context.borderPrimary,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(
              Icons.notifications_active,
              color: Colors.white,
              size: 24,
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('主动消息总开关', style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(
                  _masterEnabled ? '已开启 - 角色将按规则主动发送消息' : '已关闭 - 角色不会主动发送消息',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          Switch(
            value: _masterEnabled,
            onChanged: (v) {
              setState(() {
                _masterEnabled = v;
              });
              ScaffoldMessenger.of(context).showSnackBar(
                SnackBar(content: Text('主动消息已${v ? '开启' : '关闭'}'), duration: const Duration(seconds: 1)),
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildRuleCard(BuildContext context, ProactiveRule rule) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
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
                    color: rule.isEnabled ? context.accentSoft : context.surfaceSecondary,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(
                    _getCategoryIcon(rule.category),
                    size: 20,
                    color: rule.isEnabled ? context.accentPrimary : context.textTertiary,
                  ),
                ),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Row(
                        children: [
                          Text(rule.name, style: AppTypography.cardTitle(context)),
                          const SizedBox(width: AppSpacing.sm),
                          AmitiaStatusBadge(label: rule.category, type: BadgeType.accent),
                        ],
                      ),
                      const SizedBox(height: 2),
                      Text('触发：${rule.trigger} · 时间：${rule.time}', style: AppTypography.caption(context)),
                    ],
                  ),
                ),
                Switch(
                  value: rule.isEnabled,
                  onChanged: (v) {
                    setState(() {
                      final idx = _rules.indexWhere((r) => r.id == rule.id);
                      _rules[idx] = ProactiveRule(
                        id: rule.id,
                        name: rule.name,
                        trigger: rule.trigger,
                        time: rule.time,
                        probability: rule.probability,
                        cooldown: rule.cooldown,
                        isEnabled: v,
                        category: rule.category,
                      );
                    });
                  },
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.sm),
            Container(
              padding: const EdgeInsets.all(AppSpacing.md),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Column(
                children: [
                  _buildParamRow(context, '触发概率', '${rule.probability}%', rule.probability / 100, context.accentPrimary),
                  const SizedBox(height: AppSpacing.sm),
                  _buildParamRow(context, '冷却时间', '${rule.cooldown}分钟', (rule.cooldown / 180).clamp(0.0, 1.0), context.info),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.sm),
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
                const SizedBox(width: AppSpacing.sm),
                GestureDetector(
                  onTap: () => _showTestResult(context, rule),
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
                  onTap: () => _showDeleteConfirm(context, rule),
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
        const SizedBox(width: AppSpacing.sm),
        Expanded(
          child: AmitiaProgressBar(progress: progress, color: color),
        ),
        const SizedBox(width: AppSpacing.sm),
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

  void _showRuleEditor(BuildContext context, ProactiveRule? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final triggerCtrl = TextEditingController(text: existing?.trigger ?? '');
    final timeCtrl = TextEditingController(text: existing?.time ?? '08:00');
    int probability = existing?.probability ?? 80;
    int cooldown = existing?.cooldown ?? 60;
    String category = existing?.category ?? '日常';

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
              const SizedBox(height: AppSpacing.lg),
              Text(isEdit ? '编辑规则' : '新建规则', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('规则名称', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: nameCtrl, hintText: '输入规则名称'),
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('触发场景', style: AppTypography.label(context)),
                        const SizedBox(height: AppSpacing.xs),
                        AmitiaTextField(controller: triggerCtrl, hintText: '如：起床'),
                      ],
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('触发时间', style: AppTypography.label(context)),
                        const SizedBox(width: AppSpacing.xs),
                        AmitiaTextField(controller: timeCtrl, hintText: '08:00'),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.md),
              Text('分类', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
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
              const SizedBox(height: AppSpacing.md),
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
              const SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '创建',
                isFullWidth: true,
                onPressed: () {
                  if (nameCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  setState(() {
                    if (isEdit) {
                      final idx = _rules.indexWhere((r) => r.id == existing.id);
                      _rules[idx] = ProactiveRule(
                        id: existing.id,
                        name: nameCtrl.text.trim(),
                        trigger: triggerCtrl.text.trim(),
                        time: timeCtrl.text.trim(),
                        probability: probability,
                        cooldown: cooldown,
                        isEnabled: existing.isEnabled,
                        category: category,
                      );
                    } else {
                      _rules.add(ProactiveRule(
                        id: 'pr${DateTime.now().millisecondsSinceEpoch}',
                        name: nameCtrl.text.trim(),
                        trigger: triggerCtrl.text.trim(),
                        time: timeCtrl.text.trim(),
                        probability: probability,
                        cooldown: cooldown,
                        category: category,
                      ));
                    }
                  });
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(isEdit ? '规则已更新' : '规则已创建'), duration: const Duration(seconds: 1)),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showTestResult(BuildContext context, ProactiveRule rule) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('测试结果', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: context.success.withValues(alpha: 0.08),
                borderRadius: AppRadius.brMedium,
              ),
              child: Column(
                children: [
                  Icon(Icons.check_circle, size: 40, color: context.success),
                  const SizedBox(height: AppSpacing.sm),
                  Text('触发成功', style: AppTypography.cardTitle(context).copyWith(color: context.success)),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.md),
            _buildResultRow(context, '规则', rule.name),
            _buildResultRow(context, '触发时间', rule.time),
            _buildResultRow(context, '概率判定', '通过 (${rule.probability}%)'),
            _buildResultRow(context, '冷却检查', '已通过'),
            _buildResultRow(context, '模拟消息', '「${rule.trigger}时间到了，记得注意休息哦~」'),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  Widget _buildResultRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 80,
            child: Text(label, style: AppTypography.label(context)),
          ),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, ProactiveRule rule) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除规则', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${rule.name}」吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _rules.removeWhere((r) => r.id == rule.id);
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('规则已删除'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _showRestoreDefaultConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('恢复默认规则', style: AppTypography.cardTitle(context)),
        content: Text('确定要恢复所有主动消息规则为默认值吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _rules = List.from(MockCharacters.proactiveRules);
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('已恢复默认规则'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('恢复', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }
}
