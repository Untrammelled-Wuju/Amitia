import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:file_picker/file_picker.dart';
import 'package:dio/dio.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class PetCreatePage extends ConsumerStatefulWidget {
  const PetCreatePage({super.key});

  @override
  ConsumerState<PetCreatePage> createState() => _PetCreatePageState();
}

class _PetCreatePageState extends ConsumerState<PetCreatePage> {
  int _currentStep = 0;
  final _totalSteps = 5;
  final _nameController = TextEditingController();
  final _descController = TextEditingController();
  PlatformFile? _referenceImage;
  final Set<String> _selectedActions = {'idle', 'wave'};
  final Map<String, String> _actionDescs = {};
  double _modelScale = 1.0;
  String? _selectedModelConfigId;
  String? _selectedCharacterId;
  bool _submitting = false;

  final _availableActions = [
    ('idle', '待机', Icons.access_time),
    ('wave', '招手', Icons.waving_hand_outlined),
    ('happy', '开心', Icons.sentiment_satisfied),
    ('speaking', '说话', Icons.chat_bubble_outline),
    ('thinking', '思考', Icons.psychology_outlined),
    ('sleeping', '睡觉', Icons.bedtime_outlined),
  ];


  @override
  void dispose() {
    _nameController.dispose();
    _descController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '创建桌宠',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildStepIndicator(context),
            Expanded(
              child: SingleChildScrollView(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                child: _buildStepContent(context),
              ),
            ),
            _buildBottomNav(context),
          ],
        ),
      ),
    );
  }

  Widget _buildStepIndicator(BuildContext context) {
    return Container(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.md),
      child: Row(
        children: List.generate(_totalSteps, (i) {
          final isCompleted = i < _currentStep;
          final isCurrent = i == _currentStep;
          return Expanded(
            child: Row(
              children: [
                Container(
                  width: 28,
                  height: 28,
                  decoration: BoxDecoration(
                    color: isCompleted || isCurrent ? context.accentPrimary : context.surfaceSecondary,
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: isCompleted
                        ? Icon(Icons.check, size: 16, color: Colors.white)
                        : Text(
                            '${i + 1}',
                            style: TextStyle(
                              fontSize: 13,
                              fontWeight: FontWeight.w600,
                              color: isCurrent ? Colors.white : context.textTertiary,
                            ),
                          ),
                  ),
                ),
                if (i < _totalSteps - 1)
                  Expanded(
                    child: Container(
                      height: 2,
                      color: isCompleted ? context.accentPrimary : context.borderPrimary,
                    ),
                  ),
              ],
            ),
          );
        }),
      ),
    );
  }

  Widget _buildStepContent(BuildContext context) {
    switch (_currentStep) {
      case 0:
        return _buildUploadStep(context);
      case 1:
        return _buildActionSelectStep(context);
      case 2:
        return _buildActionDescStep(context);
      case 3:
        return _buildModelConfigStep(context);
      case 4:
        return _buildConfirmStep(context);
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildUploadStep(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('上传角色图', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('上传一张角色立绘图片，系统将基于此生成桌宠动作', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        GestureDetector(
          onTap: _pickReferenceImage,
          child: Container(
            width: double.infinity,
            height: 200,
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brLarge,
              border: Border.all(
                color: _referenceImage != null ? context.accentPrimary : context.borderPrimary,
                width: _referenceImage != null ? 2 : 1,
              ),
            ),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  _referenceImage != null ? Icons.check_circle : Icons.cloud_upload_outlined,
                  size: 48,
                  color: _referenceImage != null ? context.success : context.textTertiary,
                ),
                SizedBox(height: AppSpacing.sm),
                Text(
                  _referenceImage?.name ?? '点击上传角色图',
                  style: AppTypography.body(context),
                ),
                SizedBox(height: AppSpacing.xs),
                Text(
                  _referenceImage != null ? '点击重新选择' : '支持 PNG、JPG、WEBP 格式',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
        ),
        SizedBox(height: AppSpacing.lg),
        Text('桌宠名称', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.xs),
        AmitiaTextField(
          hintText: '输入桌宠名称',
          controller: _nameController,
        ),
      ],
    );
  }

  Widget _buildActionSelectStep(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('选择生成动作', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('选择需要生成的桌宠动作，已选 ${_selectedActions.length} 个', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 2,
            mainAxisSpacing: AppSpacing.sm,
            crossAxisSpacing: AppSpacing.sm,
            childAspectRatio: 2.2,
          ),
          itemCount: _availableActions.length,
          itemBuilder: (context, index) {
            final action = _availableActions[index];
            final isSelected = _selectedActions.contains(action.$1);
            return GestureDetector(
              onTap: () {
                setState(() {
                  if (isSelected) {
                    _selectedActions.remove(action.$1);
                  } else {
                    _selectedActions.add(action.$1);
                  }
                });
              },
              child: Container(
                padding: EdgeInsets.all(AppSpacing.md),
                decoration: BoxDecoration(
                  color: isSelected ? context.accentSoft : context.surfacePrimary,
                  borderRadius: AppRadius.brMedium,
                  border: Border.all(
                    color: isSelected ? context.accentPrimary : context.borderPrimary,
                    width: isSelected ? 1.5 : 0.5,
                  ),
                ),
                child: Row(
                  children: [
                    Icon(
                      action.$3,
                      size: 22,
                      color: isSelected ? context.accentPrimary : context.textSecondary,
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Text(
                        action.$2,
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                          color: isSelected ? context.accentPrimary : context.textPrimary,
                        ),
                      ),
                    ),
                    if (isSelected)
                      Icon(Icons.check_circle, size: 18, color: context.accentPrimary),
                  ],
                ),
              ),
            );
          },
        ),
      ],
    );
  }

  Widget _buildActionDescStep(BuildContext context) {
    final selectedList = _availableActions.where((a) => _selectedActions.contains(a.$1)).toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('动作说明', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('为每个动作添加描述说明（可选）', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        ...selectedList.map((action) {
          return Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.md),
            child: AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(action.$3, size: 20, color: context.accentPrimary),
                      SizedBox(width: AppSpacing.sm),
                      Text(action.$2, style: AppTypography.cardTitle(context)),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  AmitiaTextField(
                    hintText: '描述${action.$2}动作的表现...',
                    maxLines: 2,
                    onChanged: (value) {
                      _actionDescs[action.$1] = value;
                    },
                  ),
                ],
              ),
            ),
          );
        }),
        if (selectedList.isEmpty)
          AmitiaEmptyState(
            icon: Icons.info_outline,
            title: '请先选择动作',
            subtitle: '返回上一步选择需要生成的动作',
          ),
      ],
    );
  }

  Widget _buildModelConfigStep(BuildContext context) {
    final configs = ref.watch(modelConfigListProvider);
    final characters = ref.watch(characterListProvider);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('模型配置', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('桌宠生成直接使用后端现有模型配置，不再使用本地伪模型选项。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        Text('绑定角色', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.sm),
        characters.when(
          data: (items) {
            if (items.isEmpty) return const Text('暂无角色，可继续创建未绑定角色的桌宠');
            _selectedCharacterId ??= (items.where((e) => e.isActive == 1).isNotEmpty ? items.firstWhere((e) => e.isActive == 1).id : items.first.id);
            return DropdownButtonFormField<String>(
              value: items.any((e) => e.id == _selectedCharacterId) ? _selectedCharacterId : null,
              items: items.map((c) => DropdownMenuItem(value: c.id, child: Text(c.name))).toList(),
              onChanged: (value) => setState(() => _selectedCharacterId = value),
            );
          },
          loading: () => const LinearProgressIndicator(),
          error: (e, _) => Text('角色加载失败：$e'),
        ),
        SizedBox(height: AppSpacing.lg),
        Text('生成模型', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.sm),
        configs.when(
          data: (items) {
            if (items.isEmpty) return Text('暂无模型配置，请先在设置中配置模型。', style: TextStyle(color: context.error));
            _selectedModelConfigId ??= (items.where((e) => e.isActive == 1).isNotEmpty ? items.firstWhere((e) => e.isActive == 1).id : items.first.id);
            return Column(
              children: items.map((model) {
                final selected = model.id == _selectedModelConfigId;
                return Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.xs),
                  child: InkWell(
                    borderRadius: AppRadius.brSmall,
                    onTap: () => setState(() => _selectedModelConfigId = model.id),
                    child: Container(
                      padding: EdgeInsets.all(AppSpacing.md),
                      decoration: BoxDecoration(
                        color: selected ? context.accentSoft : context.surfacePrimary,
                        borderRadius: AppRadius.brSmall,
                        border: Border.all(color: selected ? context.accentPrimary : context.borderPrimary, width: selected ? 1.5 : .5),
                      ),
                      child: Row(children: [
                        Icon(selected ? Icons.radio_button_checked : Icons.radio_button_off, size: 20, color: selected ? context.accentPrimary : context.textTertiary),
                        SizedBox(width: AppSpacing.sm),
                        Expanded(child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                          Text(model.name.isEmpty ? model.model : model.name, style: AppTypography.body(context)),
                          const SizedBox(height: 2),
                          Text('${model.provider} · ${model.model}', style: AppTypography.label(context)),
                        ])),
                      ]),
                    ),
                  ),
                );
              }).toList(),
            );
          },
          loading: () => const LinearProgressIndicator(),
          error: (e, _) => Text('模型配置加载失败：$e'),
        ),
        SizedBox(height: AppSpacing.lg),
        Text('输出尺寸倍率', style: AppTypography.label(context)),
        Slider(
          value: _modelScale,
          min: 0.5,
          max: 2.0,
          divisions: 6,
          activeColor: context.accentPrimary,
          onChanged: (value) => setState(() => _modelScale = value),
        ),
        Align(alignment: Alignment.centerRight, child: Text('${_modelScale.toStringAsFixed(1)}x', style: AppTypography.bodySmall(context))),
      ],
    );
  }

  Widget _buildConfirmStep(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('确认创建', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('请确认以下信息无误后创建生成任务', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildConfirmRow(context, '桌宠名称', _nameController.text.isEmpty ? '未填写' : _nameController.text),
              SizedBox(height: AppSpacing.sm),
              _buildConfirmRow(context, '角色图', _referenceImage?.name ?? '未上传'),
              SizedBox(height: AppSpacing.sm),
              _buildConfirmRow(context, '动作数量', '${_selectedActions.length} 个'),
              SizedBox(height: AppSpacing.sm),
              _buildConfirmRow(
                context,
                '动作列表',
                _selectedActions.map((k) {
                  for (final a in _availableActions) {
                    if (a.$1 == k) return a.$2;
                  }
                  return k;
                }).join('、'),
              ),
              SizedBox(height: AppSpacing.sm),
              _buildConfirmRow(context, '模型配置 ID', _selectedModelConfigId ?? '未选择'),
              SizedBox(height: AppSpacing.sm),
              _buildConfirmRow(context, '缩放倍数', '${_modelScale.toStringAsFixed(1)}x'),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildConfirmRow(BuildContext context, String label, String value) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(width: 80, child: Text(label, style: AppTypography.caption(context))),
        Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
      ],
    );
  }

  Widget _buildBottomNav(BuildContext context) {
    final canProceed = _canProceed() && !_submitting;
    return Container(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(top: BorderSide(color: context.borderPrimary, width: 0.5)),
      ),
      child: Row(
        children: [
          if (_currentStep > 0)
            Expanded(
              child: AmitiaButton(
                label: '上一步',
                isSecondary: true,
                onPressed: () {
                  setState(() { _currentStep--; });
                },
              ),
            ),
          if (_currentStep > 0) SizedBox(width: AppSpacing.sm),
          Expanded(
            child: AmitiaButton(
              label: _currentStep == _totalSteps - 1
                  ? (_submitting ? '创建中...' : '创建任务')
                  : '下一步',
              icon: _currentStep == _totalSteps - 1 ? Icons.check : Icons.arrow_forward,
              onPressed: canProceed ? _nextStep : null,
            ),
          ),
        ],
      ),
    );
  }

  bool _canProceed() {
    switch (_currentStep) {
      case 0:
        return _referenceImage != null && _nameController.text.trim().isNotEmpty;
      case 1:
        return _selectedActions.isNotEmpty;
      default:
        return true;
    }
  }

  void _nextStep() {
    if (_currentStep < _totalSteps - 1) {
      setState(() { _currentStep++; });
    } else {
      _createTask();
    }
  }

  Future<void> _pickReferenceImage() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['png', 'jpg', 'jpeg', 'webp'],
      withData: false,
    );
    final file = result?.files.single;
    if (file == null || file.path == null) return;
    setState(() => _referenceImage = file);
  }

  Future<void> _createTask() async {
    final image = _referenceImage;
    final modelConfigId = int.tryParse(_selectedModelConfigId ?? '');
    if (image?.path == null || modelConfigId == null || modelConfigId <= 0) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请选择参考图和有效模型配置')));
      return;
    }
    setState(() => _submitting = true);
    try {
      final size = (512 * _modelScale).round().clamp(256, 2048);
      final form = FormData.fromMap({
        'characterId': _selectedCharacterId ?? '',
        'modelConfigId': modelConfigId.toString(),
        'name': _nameController.text.trim(),
        'prompt': _descController.text.trim(),
        'negativePrompt': '',
        'outputWidth': size.toString(),
        'outputHeight': size.toString(),
        'selectedActionKeys': jsonEncode(_selectedActions.toList()),
        'referenceImage': await MultipartFile.fromFile(image!.path!, filename: image.name),
      });
      final api = ref.read(backendServiceProvider);
      final created = await api.post<dynamic>('/api/desktop-pets/generation-tasks', data: form);
      final task = created is Map ? Map<String, dynamic>.from(created) : const <String, dynamic>{};
      final id = (task['id'] ?? '').toString();
      if (id.isNotEmpty) await api.post<dynamic>('/api/desktop-pets/generation-tasks/$id/start');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已创建桌宠任务：${_nameController.text}')));
        context.go(AppRoutes.workshopPetTasks);
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('创建失败: $e')));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

}
