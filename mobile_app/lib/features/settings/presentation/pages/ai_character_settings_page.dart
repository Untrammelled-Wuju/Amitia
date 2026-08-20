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

class AiCharacterSettingsPage extends ConsumerWidget {
  const AiCharacterSettingsPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final charactersAsync = ref.watch(characterListProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色性格设置',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
      ),
      body: charactersAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (err, _) => Center(
          child: Padding(
            padding: const EdgeInsets.all(32),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                const SizedBox(height: 16),
                Text(
                  '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                  style: AppTypography.body(context).copyWith(color: context.error),
                  textAlign: TextAlign.center,
                ),
                const SizedBox(height: 16),
                AmitiaButton(
                  label: '重试',
                  onPressed: () => ref.invalidate(characterListProvider),
                ),
              ],
            ),
          ),
        ),
        data: (characters) {
          final activeCharacter = characters.where((c) => c.isActive == 1).firstOrNull ?? characters.firstOrNull;
          return _CharacterContent(character: activeCharacter);
        },
      ),
    );
  }
}

class _CharacterContent extends ConsumerStatefulWidget {
  final dynamic character;

  const _CharacterContent({this.character});

  @override
  ConsumerState<_CharacterContent> createState() => _CharacterContentState();
}

class _CharacterContentState extends ConsumerState<_CharacterContent> {
  late TextEditingController _nameController;
  late TextEditingController _descController;
  bool _isDefault = false;
  bool _saving = false;

  final Map<String, double> _personality = {
    '熟悉度': 78,
    '正式度': 22,
    '直接度': 75,
    '详细程度': 32,
    '结构度': 40,
    '短句偏好': 85,
    '温暖度': 58,
    '安慰倾向': 55,
    '说教回避': 88,
    '陪伴感': 55,
    '边界感': 85,
    '依赖回避': 85,
    '执行性': 75,
    '判断力': 75,
    '理性度': 50,
    '幽默感': 40,
    '主动性': 50,
    '调侃度': 30,
    '耐心': 60,
    '亲密度表达': 25,
  };

  @override
  void initState() {
    super.initState();
    _nameController = TextEditingController(text: widget.character?.name ?? '轻熟朋友');
    _descController = TextEditingController(
      text: widget.character?.personality ?? '自然、简短、有反应，有一点熟悉感，但不过度装熟。',
    );
    _isDefault = widget.character?.isActive == 1;
  }

  @override
  void dispose() {
    _nameController.dispose();
    _descController.dispose();
    super.dispose();
  }

  Future<void> _saveConfig() async {
    if (!mounted) return;
    setState(() => _saving = true);

    final svc = ref.read(characterServiceProvider);
    final characterId = widget.character?.id;
    if (characterId != null) {
      await svc.update(characterId, {
        'name': _nameController.text,
        'personality': _descController.text,
      });
    }

    if (!mounted) return;
    setState(() => _saving = false);
    ref.invalidate(characterListProvider);
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('角色设置已保存'),
        backgroundColor: context.success,
        behavior: SnackBarBehavior.floating,
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brSmall),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        Expanded(
          child: ListView(
            padding: EdgeInsets.zero,
            children: [
              _buildInfoCard(context),
              SizedBox(height: AppSpacing.sm),
              _buildPersonalityCard(context),
              SizedBox(height: AppSpacing.sm),
              _buildPersonalityCard(context, title: '情感与氛围', sectionKey: 'emotion'),
              SizedBox(height: AppSpacing.sm),
              _buildGenderCard(context),
              SizedBox(height: AppSpacing.sm),
              _buildSleepCard(context),
              SizedBox(height: AppSpacing.lg),
            ],
          ),
        ),
        Padding(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          child: AmitiaButton(
            label: _saving ? '保存中...' : '保存',
            icon: Icons.check,
            isFullWidth: true,
            onPressed: _saving ? null : _saveConfig,
          ),
        ),
      ],
    );
  }

  Widget _buildInfoCard(BuildContext context) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('角色名称', style: AppTypography.caption(context)),
                    const SizedBox(height: 6),
                    TextField(
                      controller: _nameController,
                      style: AppTypography.body(context).copyWith(fontWeight: FontWeight.w500),
                      decoration: InputDecoration(
                        hintText: '输入角色名称',
                        filled: true,
                        fillColor: context.surfaceSecondary,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brSmall,
                          borderSide: BorderSide.none,
                        ),
                        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                      ),
                    ),
                  ],
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text('角色描述', style: AppTypography.caption(context)),
                    const SizedBox(height: 6),
                    TextField(
                      controller: _descController,
                      style: AppTypography.body(context),
                      maxLines: 2,
                      decoration: InputDecoration(
                        hintText: '简要描述角色特点',
                        filled: true,
                        fillColor: context.surfaceSecondary,
                        border: OutlineInputBorder(
                          borderRadius: AppRadius.brSmall,
                          borderSide: BorderSide.none,
                        ),
                        contentPadding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              Text('设为默认角色', style: AppTypography.body(context)),
              const Spacer(),
              Switch(
                value: _isDefault,
                onChanged: (v) => setState(() => _isDefault = v),
                activeColor: context.accentPrimary,
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildPersonalityCard(BuildContext context, {String title = '性格特征', String? sectionKey}) {
    final keys = sectionKey == 'emotion'
        ? ['温暖度', '安慰倾向', '陪伴感', '边界感', '依赖回避', '亲密度表达', '调侃度']
        : _personality.keys.where((k) => k != '温暖度' && k != '安慰倾向' && k != '陪伴感' && k != '边界感' && k != '依赖回避' && k != '亲密度表达' && k != '调侃度').toList();

    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: AppTypography.sectionTitle(context)),
          const SizedBox(height: 14),
          GridView.builder(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
              crossAxisCount: 2,
              childAspectRatio: 3.2,
              crossAxisSpacing: 8,
              mainAxisSpacing: 10,
            ),
            itemCount: keys.length,
            itemBuilder: (ctx, i) => _buildSlider(context, keys[i]),
          ),
        ],
      ),
    );
  }

  Widget _buildSlider(BuildContext context, String label) {
    final value = _personality[label] ?? 50;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label, style: TextStyle(fontSize: 12, color: context.textSecondary)),
            Text('$value', style: TextStyle(fontSize: 11, color: context.textTertiary)),
          ],
        ),
        const SizedBox(height: 4),
        Slider(
          value: value,
          min: 0,
          max: 100,
          divisions: 100,
          activeColor: context.accentPrimary,
          onChanged: (v) => setState(() => _personality[label] = v),
        ),
      ],
    );
  }

  Widget _buildGenderCard(BuildContext context) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('性别与称呼', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: 14),
          Row(
            children: [
              Expanded(
                child: _buildGenderRow(context, '代词', 'TA'),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildGenderRow(context, '自称', '我'),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: _buildGenderRow(context, '用户称呼', '自然称呼'),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildGenderRow(context, '性别表达', '适中'),
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildGenderRow(BuildContext context, String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: AppTypography.caption(context)),
        const SizedBox(height: 6),
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: AppRadius.brSmall,
          ),
          child: Text(value, style: AppTypography.body(context)),
        ),
      ],
    );
  }

  Widget _buildSleepCard(BuildContext context) {
    return Container(
      margin: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('作息设置', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: 14),
          Row(
            children: [
              Expanded(
                child: _buildTimeRow(context, '工作时间', '09:00 - 18:00'),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildTimeRow(context, '午休', '12:00 - 13:30'),
              ),
            ],
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: _buildTimeRow(context, '通勤', '15 - 45 分钟'),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: _buildTimeRow(context, '准备', '20 - 60 分钟'),
              ),
            ],
          ),
          const SizedBox(height: 14),
          Row(
            children: [
              Text('睡眠时回复', style: AppTypography.body(context)),
              const Spacer(),
              Switch(
                value: false,
                onChanged: (_) {},
                activeColor: context.accentPrimary,
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildTimeRow(BuildContext context, String label, String value) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(label, style: AppTypography.caption(context)),
        const SizedBox(height: 6),
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: AppRadius.brSmall,
          ),
          child: Text(value, style: AppTypography.body(context)),
        ),
      ],
    );
  }
}
