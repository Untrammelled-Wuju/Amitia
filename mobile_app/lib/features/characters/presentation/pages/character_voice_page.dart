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
import '../../../../core/models/voice.dart';

class CharacterVoicePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterVoicePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterVoicePage> createState() => _CharacterVoicePageState();
}

class _CharacterVoicePageState extends ConsumerState<CharacterVoicePage> {
  double _speed = 1.0;
  double _pitch = 1.0;
  String? _activeConfigId;
  List<VoiceConfigDto> _configs = [];

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '拟态语音',
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          AmitiaIconButton(
            icon: Icons.add,
            tooltip: '新增音色',
            onPressed: () => _showVoiceEditor(context, null),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildVoiceCloneEntry(context),
            SizedBox(height: AppSpacing.sectionGap),
            _buildVoiceListSection(context),
            SizedBox(height: AppSpacing.sectionGap),
            _buildParamsSection(context),
            SizedBox(height: AppSpacing.xxl),
          ],
        ),
      ),
    );
  }

  Widget _buildVoiceCloneEntry(BuildContext context) {
    return AmitiaCard(
      backgroundColor: context.accentSoft,
      child: Row(
        children: [
          Container(
            width: 48,
            height: 48,
            decoration: BoxDecoration(
              color: context.accentPrimary,
              borderRadius: AppRadius.brSmall,
            ),
            child: const Icon(Icons.graphic_eq, color: Colors.white, size: 24),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('声音复刻', style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text('上传3-10秒语音样本，克隆专属音色', style: AppTypography.caption(context)),
              ],
            ),
          ),
          AmitiaButton(
            label: '开始',
            isSecondary: true,
            onPressed: () => _startVoiceClone(context),
          ),
        ],
      ),
    );
  }

  Widget _buildVoiceListSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '预设音色'),
        SizedBox(height: AppSpacing.sm),
        FutureBuilder<List<VoiceConfigDto>>(
          future: ref.read(ttsServiceProvider).listConfigs(),
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              return Text('加载失败: ${snapshot.error}', style: AppTypography.caption(context));
            }
            _configs = snapshot.data ?? [];
            if (_configs.isEmpty) {
              return Text('暂无音色配置', style: AppTypography.caption(context));
            }
            return Column(
              children: _configs.map((v) => _buildVoiceItem(context, v)).toList(),
            );
          },
        ),
      ],
    );
  }

  Widget _buildVoiceItem(BuildContext context, VoiceConfigDto voice) {
    final isCurrent = voice.id == _activeConfigId || voice.isActive == 1;
    if (_activeConfigId == null && voice.isActive == 1) {
      _activeConfigId = voice.id;
    }

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        border: Border.all(
          color: isCurrent ? context.accentPrimary : context.borderPrimary,
          width: isCurrent ? 1.5 : 0.5,
        ),
        child: Row(
          children: [
            Container(
              width: 44,
              height: 44,
              decoration: BoxDecoration(
                color: isCurrent ? context.accentPrimary : context.accentSoft,
                borderRadius: AppRadius.brSmall,
              ),
              child: Icon(
                isCurrent ? Icons.check_circle : Icons.record_voice_over,
                size: 22,
                color: isCurrent ? Colors.white : context.accentPrimary,
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(voice.name, style: AppTypography.cardTitle(context)),
                      if (isCurrent) ...[
                        SizedBox(width: AppSpacing.sm),
                        AmitiaStatusBadge(label: '当前', type: BadgeType.accent),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text('提供商：${voice.provider} · 音色：${voice.voiceId}', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaIconButton(
              icon: Icons.play_circle_outline,
              size: 26,
              color: context.accentPrimary,
              onPressed: () => _testVoice(voice),
            ),
            AmitiaIconButton(
              icon: Icons.edit_outlined,
              size: 18,
              onPressed: () => _showVoiceEditor(context, voice),
            ),
            if (!isCurrent)
              AmitiaIconButton(
                icon: Icons.delete_outline,
                size: 18,
                color: context.error,
                onPressed: () => _deleteVoice(voice),
              ),
          ],
        ),
        onTap: () async {
          final svc = ref.read(ttsServiceProvider);
          await svc.activate(voice.id);
          setState(() {
            _activeConfigId = voice.id;
            _speed = voice.speed.toDouble();
            _pitch = voice.pitch.toDouble();
          });
          if (mounted) {
            ScaffoldMessenger.of(context).showSnackBar(
              SnackBar(content: Text('已切换到「${voice.name}」'), duration: const Duration(seconds: 1)),
            );
          }
        },
      ),
    );
  }

  Widget _buildParamsSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '语音参数'),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            children: [
              _buildSliderRow(context, '语速', _speed, 0.5, 2.0, 'x', (v) => setState(() => _speed = v)),
              Divider(height: AppSpacing.lg),
              _buildSliderRow(context, '音调', _pitch, 0.5, 2.0, 'x', (v) => setState(() => _pitch = v)),
              SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '试听当前参数',
                icon: Icons.volume_up,
                isFullWidth: true,
                isSecondary: true,
                onPressed: () => _testCurrentVoice(context),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildSliderRow(
    BuildContext context,
    String label,
    double value,
    double min,
    double max,
    String suffix,
    ValueChanged<double> onChanged,
  ) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label, style: AppTypography.body(context)),
            Text(
              '${value.toStringAsFixed(1)}$suffix',
              style: AppTypography.bodySmall(context).copyWith(color: context.accentPrimary, fontWeight: FontWeight.w600),
            ),
          ],
        ),
        Slider(
          value: value,
          min: min,
          max: max,
          divisions: 50,
          activeColor: context.accentPrimary,
          onChanged: onChanged,
        ),
      ],
    );
  }

  Future<void> _testVoice(VoiceConfigDto voice) async {
    final svc = ref.read(ttsServiceProvider);
    await svc.test(voice.id);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('正在测试「${voice.name}」...'), duration: const Duration(seconds: 2)),
      );
    }
  }

  Future<void> _testCurrentVoice(BuildContext context) async {
    if (_activeConfigId == null) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请先选择一个音色')),
      );
      return;
    }
    final svc = ref.read(ttsServiceProvider);
    await svc.synthesize('你好，这是一段语音测试。', voiceId: _activeConfigId);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('正在试听...'), duration: Duration(seconds: 2)),
      );
    }
  }

  Future<void> _startVoiceClone(BuildContext context) async {
    final result = await ref.read(characterDetailServiceProvider).uploadAvatar(widget.characterId, 'voice_sample.wav');
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(result != null ? '声音复刻已启动' : '请先上传语音样本')),
      );
    }
  }

  Future<void> _deleteVoice(VoiceConfigDto voice) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除音色', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${voice.name}」吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
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
      final svc = ref.read(ttsServiceProvider);
      await svc.deleteConfig(voice.id);
      setState(() {});
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('音色已删除'), duration: Duration(seconds: 1)),
        );
      }
    }
  }

  void _showVoiceEditor(BuildContext context, VoiceConfigDto? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final providerCtrl = TextEditingController(text: existing?.provider ?? '');
    final voiceIdCtrl = TextEditingController(text: existing?.voiceId ?? '');
    int speed = existing?.speed ?? 1;
    int pitch = existing?.pitch ?? 1;

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
              Text(isEdit ? '编辑音色' : '新增音色', style: AppTypography.sectionTitle(context)),
              SizedBox(height: AppSpacing.lg),
              Text('音色名称', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: nameCtrl, hintText: '输入音色名称'),
              SizedBox(height: AppSpacing.md),
              Text('提供商', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: providerCtrl, hintText: '如：OpenAI, Azure'),
              SizedBox(height: AppSpacing.md),
              Text('音色ID', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: voiceIdCtrl, hintText: '如：alloy, nova'),
              SizedBox(height: AppSpacing.md),
              Text('语速: $speed', style: AppTypography.label(context)),
              Slider(
                value: speed.toDouble(),
                min: 1,
                max: 3,
                divisions: 4,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => speed = v.round()),
              ),
              Text('音调: $pitch', style: AppTypography.label(context)),
              Slider(
                value: pitch.toDouble(),
                min: 1,
                max: 3,
                divisions: 4,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => pitch = v.round()),
              ),
              SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '添加',
                isFullWidth: true,
                onPressed: () async {
                  if (nameCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  final svc = ref.read(ttsServiceProvider);
                  final data = {
                    'name': nameCtrl.text.trim(),
                    'provider': providerCtrl.text.trim(),
                    'voiceId': voiceIdCtrl.text.trim(),
                    'speed': speed,
                    'pitch': pitch,
                  };
                  if (isEdit) {
                    await svc.updateConfig(existing.id, data);
                  } else {
                    await svc.createConfig(data);
                  }
                  setState(() {});
                  if (mounted) {
                    ScaffoldMessenger.of(context).showSnackBar(
                      SnackBar(content: Text(isEdit ? '音色已更新' : '音色已添加'), duration: const Duration(seconds: 1)),
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
}
