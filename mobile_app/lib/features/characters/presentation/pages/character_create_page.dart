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
import '../../../../core/widgets/amitia_misc.dart';

class CharacterCreatePage extends ConsumerStatefulWidget {
  const CharacterCreatePage({super.key});

  @override
  ConsumerState<CharacterCreatePage> createState() => _CharacterCreatePageState();
}

class _CharacterCreatePageState extends ConsumerState<CharacterCreatePage> {
  final _pageController = PageController();
  int _currentStep = 0;
  final _nameController = TextEditingController();
  final _identityController = TextEditingController();
  final _personalityController = TextEditingController();
  final _speakingStyleController = TextEditingController();
  final _relationController = TextEditingController();
  final _promptController = TextEditingController();
  String _selectedColor = '#7668EE';

  final _steps = [
    '基础形象',
    '名字',
    '身份',
    '性格',
    '说话方式',
    '关系设定',
    '初始提示词',
    '完成预览',
  ];

  final _colors = ['#7668EE', '#52B788', '#6C8FEA', '#E9A23B', '#E76F51', '#9B5DE5', '#F15BB5', '#00BBF9'];

  @override
  void dispose() {
    _pageController.dispose();
    _nameController.dispose();
    _identityController.dispose();
    _personalityController.dispose();
    _speakingStyleController.dispose();
    _relationController.dispose();
    _promptController.dispose();
    super.dispose();
  }

  void _nextStep() {
    if (_currentStep < _steps.length - 1) {
      _pageController.nextPage(duration: const Duration(milliseconds: 300), curve: Curves.easeInOut);
      setState(() => _currentStep++);
    }
  }

  void _prevStep() {
    if (_currentStep > 0) {
      _pageController.previousPage(duration: const Duration(milliseconds: 300), curve: Curves.easeInOut);
      setState(() => _currentStep--);
    }
  }

  void _finish() {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
        title: Text('创建成功', style: AppTypography.cardTitle(context)),
        content: Text('角色「${_nameController.text.isEmpty ? "未命名" : _nameController.text}」已创建（Mock）', style: AppTypography.body(context)),
        actions: [
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              context.go(AppRoutes.characters);
            },
            child: Text('查看角色列表', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '创建角色 (${_currentStep + 1}/${_steps.length})',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.sm),
              child: Row(
                children: List.generate(_steps.length, (i) {
                  return Expanded(
                    child: Container(
                      height: 3,
                      margin: EdgeInsets.only(right: i < _steps.length - 1 ? 4 : 0),
                      decoration: BoxDecoration(
                        color: i <= _currentStep ? context.accentPrimary : context.borderSecondary,
                        borderRadius: BorderRadius.circular(2),
                      ),
                    ),
                  );
                }),
              ),
            ),
            Expanded(
              child: PageView(
                controller: _pageController,
                physics: const NeverScrollableScrollPhysics(),
                children: [
                  _buildAppearanceStep(),
                  _buildNameStep(),
                  _buildIdentityStep(),
                  _buildPersonalityStep(),
                  _buildSpeakingStyleStep(),
                  _buildRelationStep(),
                  _buildPromptStep(),
                  _buildPreviewStep(),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(AppSpacing.pagePadding),
              child: Row(
                children: [
                  if (_currentStep > 0)
                    Expanded(
                      child: AmitiaButton(
                        label: '上一步',
                        isSecondary: true,
                        isFullWidth: true,
                        onPressed: _prevStep,
                      ),
                    ),
                  if (_currentStep > 0) const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: AmitiaButton(
                      label: _currentStep == _steps.length - 1 ? '完成创建' : '下一步',
                      isFullWidth: true,
                      onPressed: _currentStep == _steps.length - 1 ? _finish : _nextStep,
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

  Widget _buildStepContent({required String label, required String hint, required TextEditingController controller, int maxLines = 1}) {
    return Padding(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.md),
          AmitiaTextField(
            hintText: hint,
            controller: controller,
            maxLines: maxLines,
          ),
        ],
      ),
    );
  }

  Widget _buildAppearanceStep() {
    return Padding(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('选择角色主题色', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.lg),
          Center(
            child: Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: Color(int.parse('FF${_selectedColor.replaceAll('#', '')}', radix: 16)),
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  _nameController.text.isEmpty ? '?' : _nameController.text[0],
                  style: const TextStyle(color: Colors.white, fontSize: 32, fontWeight: FontWeight.w600),
                ),
              ),
            ),
          ),
          const SizedBox(height: AppSpacing.xl),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: _colors.map((color) {
              final isSelected = color == _selectedColor;
              return GestureDetector(
                onTap: () => setState(() => _selectedColor = color),
                child: Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    color: Color(int.parse('FF${color.replaceAll('#', '')}', radix: 16)),
                    shape: BoxShape.circle,
                    border: isSelected ? Border.all(color: context.accentPrimary, width: 3) : null,
                  ),
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }

  Widget _buildNameStep() => _buildStepContent(label: '给角色起个名字', hint: '例如：阿米娅', controller: _nameController);
  Widget _buildIdentityStep() => _buildStepContent(label: '角色身份', hint: '例如：你的专属 AI 伙伴', controller: _identityController);
  Widget _buildPersonalityStep() => _buildStepContent(label: '性格描述', hint: '例如：温柔、细心、有耐心，善于倾听', controller: _personalityController, maxLines: 3);
  Widget _buildSpeakingStyleStep() => _buildStepContent(label: '说话方式', hint: '例如：语气温和，偶尔带着俏皮', controller: _speakingStyleController, maxLines: 3);
  Widget _buildRelationStep() => _buildStepContent(label: '关系设定', hint: '例如：亲密伙伴', controller: _relationController);
  Widget _buildPromptStep() => _buildStepContent(label: '初始提示词', hint: '输入角色的系统提示词...', controller: _promptController, maxLines: 6);

  Widget _buildPreviewStep() {
    final color = Color(int.parse('FF${_selectedColor.replaceAll('#', '')}', radix: 16));
    return Padding(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('完成预览', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.lg),
          Center(
            child: Container(
              padding: const EdgeInsets.all(20),
              decoration: BoxDecoration(
                color: context.surfacePrimary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 0.5),
              ),
              child: Column(
                children: [
                  Container(
                    width: 64,
                    height: 64,
                    decoration: BoxDecoration(color: color, shape: BoxShape.circle),
                    child: Center(
                      child: Text(
                        _nameController.text.isEmpty ? '?' : _nameController.text[0],
                        style: const TextStyle(color: Colors.white, fontSize: 26, fontWeight: FontWeight.w600),
                      ),
                    ),
                  ),
                  const SizedBox(height: 12),
                  Text(_nameController.text.isEmpty ? '未命名' : _nameController.text, style: AppTypography.cardTitle(context).copyWith(fontSize: 18)),
                  if (_identityController.text.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(_identityController.text, style: AppTypography.caption(context)),
                  ],
                  if (_personalityController.text.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(_personalityController.text, style: AppTypography.body(context), textAlign: TextAlign.center),
                  ],
                  if (_speakingStyleController.text.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text('说话方式：${_speakingStyleController.text}', style: AppTypography.label(context)),
                  ],
                  if (_relationController.text.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text('关系：${_relationController.text}', style: AppTypography.label(context)),
                  ],
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
