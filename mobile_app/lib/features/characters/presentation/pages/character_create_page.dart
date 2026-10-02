import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_motion.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/profile_avatar.dart';

class CharacterCreatePage extends ConsumerStatefulWidget {
  const CharacterCreatePage({super.key});

  @override
  ConsumerState<CharacterCreatePage> createState() =>
      _CharacterCreatePageState();
}

class _CharacterCreatePageState extends ConsumerState<CharacterCreatePage> {
  final _pageController = PageController();
  int _currentStep = 0;
  final _nameController = TextEditingController();
  final _identityController = TextEditingController();
  final _personalityController = TextEditingController();
  final _speakingStyleController = TextEditingController();
  final _promptController = TextEditingController();
  XFile? _avatarFile;
  String _avatarPreview = '';
  bool _isPickingAvatar = false;
  bool _isCreating = false;

  final _steps = ['基础形象', '名字', '身份', '性格', '说话方式', '提示词', '完成预览'];

  @override
  void dispose() {
    _pageController.dispose();
    _nameController.dispose();
    _identityController.dispose();
    _personalityController.dispose();
    _speakingStyleController.dispose();
    _promptController.dispose();
    super.dispose();
  }

  void _nextStep() {
    if (_currentStep < _steps.length - 1) {
      _pageController.nextPage(
        duration: AppMotion.extended,
        curve: AppMotion.panelCurve,
      );
      setState(() => _currentStep++);
    }
  }

  void _prevStep() {
    if (_currentStep > 0) {
      _pageController.previousPage(
        duration: AppMotion.extended,
        curve: AppMotion.panelCurve,
      );
      setState(() => _currentStep--);
    }
  }

  Future<void> _finish() async {
    if (_isCreating) return;
    setState(() => _isCreating = true);
    try {
      final svc = ref.read(characterServiceProvider);
      final avatarService = ref.read(characterDetailServiceProvider);
      final data = {
        'name': _nameController.text,
        'identity': _identityController.text,
        'personality': _personalityController.text,
        'speakingStyle': _speakingStyleController.text,
        'description': _personalityController.text,
        'status': '在线',
        'voiceSpeed': 1.0,
      };
      final character = await svc.create(data);
      if (character == null || character.id.isEmpty) {
        throw StateError('角色创建结果为空，请确认服务连接后重试');
      }
      String? avatarError;
      final avatarFile = _avatarFile;
      if (avatarFile != null) {
        try {
          final result = await avatarService.uploadAvatar(
            character.id,
            avatarFile.path,
          );
          if ((result?['avatarUrl'] ?? '').toString().isEmpty) {
            throw StateError('服务未返回头像地址');
          }
        } catch (error) {
          avatarError = '头像上传失败，可在角色详情中重新上传';
        }
      }
      if (mounted) {
        ref.invalidate(characterListProvider);
        final messenger = ScaffoldMessenger.of(context);
        Navigator.of(context).pop();
        messenger.showSnackBar(
          SnackBar(
            content: Text(
              '角色「${character.name.isEmpty ? '未命名' : character.name}」已创建'
              '${avatarError == null ? '' : '，$avatarError'}',
            ),
          ),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('创建失败: $e')));
      }
    } finally {
      if (mounted) setState(() => _isCreating = false);
    }
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
              padding: EdgeInsets.symmetric(
                horizontal: AppSpacing.pagePadding,
                vertical: AppSpacing.sm,
              ),
              child: Row(
                children: List.generate(_steps.length, (i) {
                  return Expanded(
                    child: Container(
                      height: 3,
                      margin: EdgeInsets.only(
                        right: i < _steps.length - 1 ? 4 : 0,
                      ),
                      decoration: BoxDecoration(
                        color: i <= _currentStep
                            ? context.accentPrimary
                            : context.borderSecondary,
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
                  _buildPromptStep(),
                  _buildPreviewStep(),
                ],
              ),
            ),
            Padding(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: Row(
                children: [
                  if (_currentStep > 0)
                    Expanded(
                      child: AmitiaButton(
                        label: '上一步',
                        isSecondary: true,
                        isFullWidth: true,
                        onPressed: _isCreating || _isPickingAvatar
                            ? null
                            : _prevStep,
                      ),
                    ),
                  if (_currentStep > 0) SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: AmitiaButton(
                      label: _currentStep == _steps.length - 1
                          ? (_isCreating ? '创建中...' : '完成创建')
                          : '下一步',
                      isFullWidth: true,
                      onPressed: _currentStep == _steps.length - 1
                          ? (_isCreating ? null : _finish)
                          : (_isPickingAvatar ? null : _nextStep),
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

  Widget _buildStepContent({
    required String label,
    required String hint,
    required TextEditingController controller,
    int maxLines = 1,
  }) {
    return Padding(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.md),
          AmitiaTextField(
            hintText: hint,
            controller: controller,
            maxLines: maxLines,
          ),
        ],
      ),
    );
  }

  Future<void> _pickAvatar() async {
    if (_isPickingAvatar || _isCreating) return;
    setState(() => _isPickingAvatar = true);
    try {
      final file = await ImagePicker().pickImage(
        source: ImageSource.gallery,
        maxWidth: 1024,
        maxHeight: 1024,
        imageQuality: 85,
      );
      if (file == null) return;
      final bytes = await file.readAsBytes();
      if (bytes.isEmpty) throw StateError('无法读取所选头像');
      if (bytes.length > 3 * 1024 * 1024) {
        throw StateError('头像图片不能超过 3 MB');
      }
      if (!mounted) return;
      setState(() {
        _avatarFile = file;
        _avatarPreview = 'data:image/jpeg;base64,${base64Encode(bytes)}';
      });
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('选择头像失败：$error')));
      }
    } finally {
      if (mounted) setState(() => _isPickingAvatar = false);
    }
  }

  Widget _buildAvatar(double size) => ProfileAvatar(
    avatar: _avatarPreview,
    initial: _nameController.text.trim(),
    size: size,
  );

  Widget _buildAppearanceStep() {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text('上传角色头像', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text(
          '可选。不上传时使用角色名字的首字作为文字头像，之后也可以在角色详情中修改。',
          style: AppTypography.caption(context),
        ),
        SizedBox(height: AppSpacing.xl),
        Center(child: _buildAvatar(80)),
        SizedBox(height: AppSpacing.xl),
        AmitiaButton(
          label: _isPickingAvatar
              ? '读取中...'
              : (_avatarFile == null ? '选择图片' : '更换头像'),
          icon: Icons.photo_library_outlined,
          isSecondary: true,
          isFullWidth: true,
          onPressed: _isPickingAvatar ? null : _pickAvatar,
        ),
        if (_avatarFile != null) ...[
          SizedBox(height: AppSpacing.sm),
          TextButton(
            onPressed: _isPickingAvatar
                ? null
                : () => setState(() {
                    _avatarFile = null;
                    _avatarPreview = '';
                  }),
            child: const Text('使用文字头像'),
          ),
        ],
      ],
    );
  }

  Widget _buildNameStep() => _buildStepContent(
    label: '给角色起个名字',
    hint: '例如：Amitia',
    controller: _nameController,
  );
  Widget _buildIdentityStep() => _buildStepContent(
    label: '角色身份',
    hint: '例如：你的专属 AI 伙伴',
    controller: _identityController,
  );
  Widget _buildPersonalityStep() => _buildStepContent(
    label: '性格描述',
    hint: '例如：温柔、细心、有耐心，善于倾听',
    controller: _personalityController,
    maxLines: 3,
  );
  Widget _buildSpeakingStyleStep() => _buildStepContent(
    label: '说话方式',
    hint: '例如：语气温和，偶尔带着俏皮',
    controller: _speakingStyleController,
    maxLines: 3,
  );
  Widget _buildPromptStep() => _buildStepContent(
    label: '初始提示词',
    hint: '输入角色的系统提示词...',
    controller: _promptController,
    maxLines: 6,
  );

  Widget _buildPreviewStep() {
    return Padding(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('完成预览', style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.lg),
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
                  _buildAvatar(64),
                  const SizedBox(height: 12),
                  Text(
                    _nameController.text.isEmpty ? '未命名' : _nameController.text,
                    style: AppTypography.cardTitle(
                      context,
                    ).copyWith(fontSize: 18),
                  ),
                  if (_identityController.text.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      _identityController.text,
                      style: AppTypography.caption(context),
                    ),
                  ],
                  if (_personalityController.text.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(
                      _personalityController.text,
                      style: AppTypography.body(context),
                      textAlign: TextAlign.center,
                    ),
                  ],
                  if (_speakingStyleController.text.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      '说话方式：${_speakingStyleController.text}',
                      style: AppTypography.label(context),
                    ),
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
