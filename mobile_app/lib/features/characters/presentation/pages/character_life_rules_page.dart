import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class CharacterLifeRulesPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterLifeRulesPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterLifeRulesPage> createState() => _CharacterLifeRulesPageState();
}

class _CharacterLifeRulesPageState extends ConsumerState<CharacterLifeRulesPage> {
  late CharacterLifeRules _lifeRules;
  late TextEditingController _promptController;
  late List<FixedSchedule> _schedules;
  late List<SpecialState> _specialStates;
  late bool _timeAwareness;
  late int _personalityScore;

  @override
  void initState() {
    super.initState();
    _lifeRules = MockCharacters.lifeRules(widget.characterId);
    _promptController = TextEditingController(text: _lifeRules.prompt);
    _schedules = List.from(_lifeRules.fixedSchedules);
    _specialStates = List.from(_lifeRules.specialStates);
    _timeAwareness = _lifeRules.timeAwareness;
    _personalityScore = _lifeRules.personalityScore;
  }

  @override
  void dispose() {
    _promptController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '生活规则',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.visibility_outlined,
            tooltip: '预览完整Prompt',
            onPressed: () => _showFullPromptPreview(context),
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '恢复默认',
            onPressed: () => _showRestoreDefaultConfirm(context),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildPromptSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildPersonalitySection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildInfoSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildSchedulesSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildSpecialStatesSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildSettingsSection(context),
            const SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildPromptSection(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text('角色 Prompt', style: AppTypography.cardTitle(context)),
              AmitiaIconButton(
                icon: Icons.check,
                color: context.accentPrimary,
                backgroundColor: context.accentSoft,
                onPressed: () => _showPromptEditConfirm(context),
              ),
            ],
          ),
          const SizedBox(height: AppSpacing.sm),
          AmitiaTextField(
            controller: _promptController,
            maxLines: 6,
            hintText: '输入角色 Prompt...',
          ),
          const SizedBox(height: AppSpacing.sm),
          Row(
            children: [
              AmitiaButtonOutline(
                label: '预览完整Prompt',
                onPressed: () => _showFullPromptPreview(context),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: AmitiaButton(
                  label: '保存修改',
                  icon: Icons.save_outlined,
                  onPressed: () => _showPromptEditConfirm(context),
                ),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildPersonalitySection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '性格设置'),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('性格倾向', style: AppTypography.cardTitle(context)),
              const SizedBox(height: AppSpacing.xs),
              Text(_lifeRules.personality, style: AppTypography.caption(context)),
              const SizedBox(height: AppSpacing.md),
              Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: [
                  Text('温和', style: AppTypography.label(context)),
                  Text('$_personalityScore', style: AppTypography.label(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600)),
                  Text('强势', style: AppTypography.label(context)),
                ],
              ),
              Slider(
                value: _personalityScore.toDouble(),
                min: 0,
                max: 100,
                divisions: 100,
                activeColor: context.accentPrimary,
                onChanged: (value) {
                  setState(() {
                    _personalityScore = value.round();
                  });
                },
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildInfoSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '基本信息'),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            children: [
              _buildInfoRow(context, '关系时间', _lifeRules.relationshipTime, Icons.favorite_outline),
              const Divider(height: AppSpacing.lg),
              _buildInfoRow(context, '工作/上课状态', _lifeRules.workStatus, Icons.work_outline),
              const Divider(height: AppSpacing.lg),
              _buildInfoRow(context, '睡眠设置', _lifeRules.sleepSettings, Icons.bedtime_outlined),
              const Divider(height: AppSpacing.lg),
              _buildInfoRow(context, '日常倾向', _lifeRules.dailyTendency, Icons.trending_up),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value, IconData icon) {
    return Row(
      children: [
        Icon(icon, size: 20, color: context.accentPrimary),
        const SizedBox(width: AppSpacing.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label, style: AppTypography.label(context)),
              const SizedBox(height: 2),
              Text(value, style: AppTypography.bodySmall(context)),
            ],
          ),
        ),
        AmitiaIconButton(
          icon: Icons.edit_outlined,
          size: 18,
          onPressed: () {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text('编辑$label'), duration: const Duration(seconds: 1)),
            );
          },
        ),
      ],
    );
  }

  Widget _buildSchedulesSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(
          title: '固定日程',
          actionText: '新增',
          onAction: () => _showScheduleEditor(context, null),
        ),
        const SizedBox(height: AppSpacing.sm),
        ..._schedules.map((s) => _buildScheduleItem(context, s)),
      ],
    );
  }

  Widget _buildScheduleItem(BuildContext context, FixedSchedule schedule) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(Icons.schedule, size: 22, color: context.accentPrimary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(schedule.title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text('${schedule.startTime} - ${schedule.endTime}', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaStatusBadge(label: schedule.category, type: BadgeType.accent),
            const SizedBox(width: AppSpacing.sm),
            AmitiaIconButton(
              icon: Icons.edit_outlined,
              size: 18,
              onPressed: () => _showScheduleEditor(context, schedule),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSpecialStatesSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(
          title: '特殊状态',
          actionText: '新增',
          onAction: () => _showSpecialStateEditor(context, null),
        ),
        const SizedBox(height: AppSpacing.sm),
        ..._specialStates.map((s) => _buildSpecialStateItem(context, s)),
      ],
    );
  }

  Widget _buildSpecialStateItem(BuildContext context, SpecialState state) {
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: state.isActive ? context.warning.withValues(alpha: 0.12) : context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(
                state.isActive ? Icons.flash_on : Icons.flash_off,
                size: 22,
                color: state.isActive ? context.warning : context.textTertiary,
              ),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(state.name, style: AppTypography.cardTitle(context)),
                      const SizedBox(width: AppSpacing.sm),
                      AmitiaStatusBadge(
                        label: state.isActive ? '激活中' : '未激活',
                        type: state.isActive ? BadgeType.warning : BadgeType.neutral,
                      ),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(state.description, style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaIconButton(
              icon: Icons.edit_outlined,
              size: 18,
              onPressed: () => _showSpecialStateEditor(context, state),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSettingsSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '其他设置'),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            children: [
              AmitiaSwitchTile(
                title: '时间感知',
                subtitle: '角色能感知当前时间并据此调整行为',
                value: _timeAwareness,
                onChanged: (value) {
                  setState(() {
                    _timeAwareness = value;
                  });
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('时间感知已${value ? '开启' : '关闭'}'), duration: const Duration(seconds: 1)),
                  );
                },
              ),
              const Divider(height: AppSpacing.lg),
              GestureDetector(
                onTap: () {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('表情包设置'), duration: Duration(seconds: 1)),
                  );
                },
                child: Row(
                  children: [
                    Icon(Icons.emoji_emotions_outlined, size: 20, color: context.accentPrimary),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text('表情包设置', style: AppTypography.body(context)),
                          const SizedBox(height: 2),
                          Text(_lifeRules.emoteSettings, style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                    Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
                  ],
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  void _showPromptEditConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('确认修改 Prompt', style: AppTypography.cardTitle(context)),
        content: Text('确定要保存对角色 Prompt 的修改吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _lifeRules = CharacterLifeRules(
                  prompt: _promptController.text,
                  personality: _lifeRules.personality,
                  personalityScore: _personalityScore,
                  relationshipTime: _lifeRules.relationshipTime,
                  workStatus: _lifeRules.workStatus,
                  sleepSettings: _lifeRules.sleepSettings,
                  dailyTendency: _lifeRules.dailyTendency,
                  fixedSchedules: _schedules,
                  specialStates: _specialStates,
                  timeAwareness: _timeAwareness,
                  emoteSettings: _lifeRules.emoteSettings,
                );
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('Prompt 已保存'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('确定', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showFullPromptPreview(BuildContext context) {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (ctx) => DraggableScrollableSheet(
        initialChildSize: 0.7,
        maxChildSize: 0.9,
        minChildSize: 0.5,
        expand: false,
        builder: (ctx, controller) => Container(
          padding: const EdgeInsets.all(AppSpacing.xl),
          child: Column(
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
              Text('完整 Prompt 预览', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.md),
              Expanded(
                child: SingleChildScrollView(
                  controller: controller,
                  child: Container(
                    padding: const EdgeInsets.all(AppSpacing.lg),
                    decoration: BoxDecoration(
                      color: context.surfaceSecondary,
                      borderRadius: AppRadius.brMedium,
                    ),
                    child: SelectableText(
                      _promptController.text,
                      style: AppTypography.bodySmall(context).copyWith(height: 1.6),
                    ),
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '关闭',
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => Navigator.pop(ctx),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showRestoreDefaultConfirm(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('恢复默认设置', style: AppTypography.cardTitle(context)),
        content: Text('确定要恢复所有生活规则为默认值吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              final defaults = MockCharacters.lifeRules(widget.characterId);
              setState(() {
                _lifeRules = defaults;
                _promptController.text = defaults.prompt;
                _schedules = List.from(defaults.fixedSchedules);
                _specialStates = List.from(defaults.specialStates);
                _timeAwareness = defaults.timeAwareness;
                _personalityScore = defaults.personalityScore;
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('已恢复默认设置'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('恢复', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  void _showScheduleEditor(BuildContext context, FixedSchedule? existing) {
    final isEdit = existing != null;
    final titleCtrl = TextEditingController(text: existing?.title ?? '');
    final startCtrl = TextEditingController(text: existing?.startTime ?? '08:00');
    final endCtrl = TextEditingController(text: existing?.endTime ?? '10:00');
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
              Text(isEdit ? '编辑日程' : '新增日程', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('日程名称', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: titleCtrl, hintText: '输入日程名称'),
              const SizedBox(height: AppSpacing.md),
              Row(
                children: [
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('开始时间', style: AppTypography.label(context)),
                        const SizedBox(height: AppSpacing.xs),
                        AmitiaTextField(controller: startCtrl, hintText: '08:00'),
                      ],
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('结束时间', style: AppTypography.label(context)),
                        const SizedBox(height: AppSpacing.xs),
                        AmitiaTextField(controller: endCtrl, hintText: '10:00'),
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
                children: ['日常', '工作', '休息', '特殊'].map((c) {
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
              const SizedBox(height: AppSpacing.xl),
              Row(
                children: [
                  if (isEdit)
                    Expanded(
                      child: AmitiaButton(
                        label: '删除',
                        isDestructive: true,
                        onPressed: () {
                          Navigator.pop(ctx);
                          setState(() {
                            _schedules.removeWhere((s) => s.id == existing.id);
                          });
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(content: Text('日程已删除'), duration: Duration(seconds: 1)),
                          );
                        },
                      ),
                    ),
                  if (isEdit) const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: AmitiaButton(
                      label: isEdit ? '保存' : '添加',
                      onPressed: () {
                        if (titleCtrl.text.trim().isEmpty) return;
                        Navigator.pop(ctx);
                        setState(() {
                          if (isEdit) {
                            final idx = _schedules.indexWhere((s) => s.id == existing.id);
                            _schedules[idx] = FixedSchedule(
                              id: existing.id,
                              title: titleCtrl.text.trim(),
                              startTime: startCtrl.text.trim(),
                              endTime: endCtrl.text.trim(),
                              category: category,
                            );
                          } else {
                            _schedules.add(FixedSchedule(
                              id: 'fs${DateTime.now().millisecondsSinceEpoch}',
                              title: titleCtrl.text.trim(),
                              startTime: startCtrl.text.trim(),
                              endTime: endCtrl.text.trim(),
                              category: category,
                            ));
                          }
                        });
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(isEdit ? '日程已更新' : '日程已添加'), duration: const Duration(seconds: 1)),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showSpecialStateEditor(BuildContext context, SpecialState? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final descCtrl = TextEditingController(text: existing?.description ?? '');
    bool isActive = existing?.isActive ?? false;

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
              Text(isEdit ? '编辑特殊状态' : '新增特殊状态', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('状态名称', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: nameCtrl, hintText: '输入状态名称'),
              const SizedBox(height: AppSpacing.md),
              Text('状态描述', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: descCtrl, maxLines: 3, hintText: '输入状态描述'),
              const SizedBox(height: AppSpacing.md),
              AmitiaSwitchTile(
                title: '是否激活',
                subtitle: '激活后角色将进入此状态',
                value: isActive,
                onChanged: (v) => setSheetState(() => isActive = v),
              ),
              const SizedBox(height: AppSpacing.xl),
              Row(
                children: [
                  if (isEdit)
                    Expanded(
                      child: AmitiaButton(
                        label: '删除',
                        isDestructive: true,
                        onPressed: () {
                          Navigator.pop(ctx);
                          setState(() {
                            _specialStates.removeWhere((s) => s.id == existing.id);
                          });
                          ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(content: Text('状态已删除'), duration: Duration(seconds: 1)),
                          );
                        },
                      ),
                    ),
                  if (isEdit) const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: AmitiaButton(
                      label: isEdit ? '保存' : '添加',
                      onPressed: () {
                        if (nameCtrl.text.trim().isEmpty) return;
                        Navigator.pop(ctx);
                        setState(() {
                          if (isEdit) {
                            final idx = _specialStates.indexWhere((s) => s.id == existing.id);
                            _specialStates[idx] = SpecialState(
                              id: existing.id,
                              name: nameCtrl.text.trim(),
                              description: descCtrl.text.trim(),
                              isActive: isActive,
                            );
                          } else {
                            _specialStates.add(SpecialState(
                              id: 'ss${DateTime.now().millisecondsSinceEpoch}',
                              name: nameCtrl.text.trim(),
                              description: descCtrl.text.trim(),
                              isActive: isActive,
                            ));
                          }
                        });
                        ScaffoldMessenger.of(context).showSnackBar(
                          SnackBar(content: Text(isEdit ? '状态已更新' : '状态已添加'), duration: const Duration(seconds: 1)),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}
