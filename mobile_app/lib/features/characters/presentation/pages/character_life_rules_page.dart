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

class CharacterLifeRulesPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterLifeRulesPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterLifeRulesPage> createState() => _CharacterLifeRulesPageState();
}

class _CharacterLifeRulesPageState extends ConsumerState<CharacterLifeRulesPage> {
  final _promptController = TextEditingController();
  bool _timeAwareness = true;
  int _personalityScore = 65;

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
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.visibility_outlined,
            tooltip: '预览完整Prompt',
            onPressed: () => _showFullPromptPreview(context),
          ),
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '恢复默认',
            onPressed: () => _resetToDefaults(),
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
            _buildFixedEventsSection(context),
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
                onPressed: () => _savePrompt(),
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
                  onPressed: () => _savePrompt(),
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
              Text('角色性格从温和到倾向的设置', style: AppTypography.caption(context)),
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

  Widget _buildFixedEventsSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '固定日程'),
        const SizedBox(height: AppSpacing.sm),
        FutureBuilder<List<Map<String, dynamic>>>(
          future: ref.read(companionServiceProvider).fixedEvents(),
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Padding(
                padding: EdgeInsets.all(AppSpacing.lg),
                child: Center(child: CircularProgressIndicator()),
              );
            }
            if (snapshot.hasError) {
              return Padding(
                padding: const EdgeInsets.all(AppSpacing.md),
                child: Text('加载失败: ${snapshot.error}', style: AppTypography.caption(context)),
              );
            }
            final events = snapshot.data ?? [];
            if (events.isEmpty) {
              return Padding(
                padding: const EdgeInsets.all(AppSpacing.md),
                child: Text('暂无固定日程', style: AppTypography.caption(context)),
              );
            }
            return Column(
              children: events.map((e) => _buildScheduleItem(context, e)).toList(),
            );
          },
        ),
      ],
    );
  }

  Widget _buildScheduleItem(BuildContext context, Map<String, dynamic> event) {
    final title = event['title']?.toString() ?? '';
    final startTime = event['startTime']?.toString() ?? '';
    final endTime = event['endTime']?.toString() ?? '';
    final type = event['type']?.toString() ?? '';
    final enabled = event['enabled'] == 1 || event['enabled'] == true;

    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: enabled ? context.accentSoft : context.surfaceSecondary,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(Icons.schedule, size: 22, color: enabled ? context.accentPrimary : context.textTertiary),
            ),
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(title, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text('$startTime - $endTime', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaStatusBadge(
              label: type.isEmpty ? '日程' : type,
              type: enabled ? BadgeType.success : BadgeType.neutral,
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
          child: AmitiaSwitchTile(
            title: '时间感知',
            subtitle: '角色能感知当前时间并据此调整行为',
            value: _timeAwareness,
            onChanged: (value) async {
              setState(() => _timeAwareness = value);
              final svc = ref.read(companionServiceProvider);
              await svc.updateSleepSetting({'timeAwareness': value});
              if (mounted) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('时间感知已${value ? '开启' : '关闭'}'), duration: const Duration(seconds: 1)),
                );
              }
            },
          ),
        ),
      ],
    );
  }

  void _resetToDefaults() {
    setState(() {
      _promptController.text = '';
      _timeAwareness = true;
      _personalityScore = 65;
    });
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('已恢复默认设置'), duration: Duration(seconds: 1)),
    );
  }

  Future<void> _savePrompt() async {
    final svc = ref.read(characterDetailServiceProvider);
    await svc.updateRoleProfile({'prompt': _promptController.text, 'personalityScore': _personalityScore});
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Prompt 已保存'), duration: Duration(seconds: 1)),
      );
    }
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
                      _promptController.text.isEmpty ? '暂无Prompt' : _promptController.text,
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
}
