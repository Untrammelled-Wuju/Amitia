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

class CharacterVoicePage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterVoicePage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterVoicePage> createState() => _CharacterVoicePageState();
}

class _CharacterVoicePageState extends ConsumerState<CharacterVoicePage> {
  late List<CharacterVoiceConfig> _voices;
  late String _currentVoiceId;
  double _speed = 1.0;
  double _pitch = 1.1;
  double _volume = 0.8;

  @override
  void initState() {
    super.initState();
    _voices = MockCharacters.voiceConfigs(widget.characterId);
    _currentVoiceId = _voices.firstWhere((v) => v.isCurrent, orElse: () => _voices.first).id;
    final current = _voices.firstWhere((v) => v.id == _currentVoiceId);
    _speed = current.speed;
    _pitch = current.pitch;
    _volume = current.volume;
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '拟态语音',
        showBackButton: true,
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
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            _buildVoiceCloneEntry(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildVoiceListSection(context),
            const SizedBox(height: AppSpacing.sectionGap),
            _buildParamsSection(context),
            const SizedBox(height: AppSpacing.xxl),
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
          const SizedBox(width: AppSpacing.md),
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
            onPressed: () => _showVoiceCloneDialog(context),
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
        const SizedBox(height: AppSpacing.sm),
        ..._voices.map((v) => _buildVoiceItem(context, v)),
      ],
    );
  }

  Widget _buildVoiceItem(BuildContext context, CharacterVoiceConfig voice) {
    final isCurrent = voice.id == _currentVoiceId;
    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
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
            const SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Text(voice.name, style: AppTypography.cardTitle(context)),
                      if (isCurrent) ...[
                        const SizedBox(width: AppSpacing.sm),
                        AmitiaStatusBadge(label: '当前', type: BadgeType.accent),
                      ],
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text('预设：${voice.preset}', style: AppTypography.caption(context)),
                ],
              ),
            ),
            AmitiaIconButton(
              icon: Icons.play_circle_outline,
              size: 26,
              color: context.accentPrimary,
              onPressed: () => _showPreviewSheet(context, voice),
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
                onPressed: () => _showDeleteConfirm(context, voice),
              ),
          ],
        ),
        onTap: () {
          setState(() {
            _currentVoiceId = voice.id;
            _speed = voice.speed;
            _pitch = voice.pitch;
            _volume = voice.volume;
          });
          ScaffoldMessenger.of(context).showSnackBar(
            SnackBar(content: Text('已切换到「${voice.name}」'), duration: const Duration(seconds: 1)),
          );
        },
      ),
    );
  }

  Widget _buildParamsSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        AmitiaSectionHeader(title: '语音参数'),
        const SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            children: [
              _buildSliderRow(context, '语速', _speed, 0.5, 2.0, 'x', (v) => setState(() => _speed = v)),
              const Divider(height: AppSpacing.lg),
              _buildSliderRow(context, '音调', _pitch, 0.5, 2.0, 'x', (v) => setState(() => _pitch = v)),
              const Divider(height: AppSpacing.lg),
              _buildSliderRow(context, '音量', _volume, 0.0, 1.0, '%', (v) => setState(() => _volume = v)),
              const SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '试听当前参数',
                icon: Icons.volume_up,
                isFullWidth: true,
                isSecondary: true,
                onPressed: () {
                  ScaffoldMessenger.of(context).showSnackBar(
                    const SnackBar(content: Text('正在试听...（Mock）'), duration: Duration(seconds: 2)),
                  );
                },
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
              suffix == '%' ? '${(value * 100).round()}%' : '${value.toStringAsFixed(1)}$suffix',
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

  void _showPreviewSheet(BuildContext context, CharacterVoiceConfig voice) {
    showModalBottomSheet(
      context: context,
      builder: (ctx) => Container(
        padding: const EdgeInsets.all(AppSpacing.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 40,
              height: 4,
              decoration: BoxDecoration(
                color: context.borderPrimary,
                borderRadius: BorderRadius.circular(2),
              ),
            ),
            const SizedBox(height: AppSpacing.lg),
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(
                color: context.accentSoft,
                shape: BoxShape.circle,
              ),
              child: Icon(Icons.graphic_eq, size: 40, color: context.accentPrimary),
            ),
            const SizedBox(height: AppSpacing.md),
            Text(voice.name, style: AppTypography.sectionTitle(context)),
            const SizedBox(height: AppSpacing.xs),
            Text('预设：${voice.preset}', style: AppTypography.caption(context)),
            const SizedBox(height: AppSpacing.xl),
            AmitiaButton(
              label: '播放',
              icon: Icons.play_arrow,
              isFullWidth: true,
              onPressed: () {
                Navigator.pop(ctx);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('正在播放「${voice.name}」试听...（Mock）'), duration: const Duration(seconds: 2)),
                );
              },
            ),
            const SizedBox(height: AppSpacing.sm),
            AmitiaButton(
              label: '关闭',
              isFullWidth: true,
              isSecondary: true,
              onPressed: () => Navigator.pop(ctx),
            ),
          ],
        ),
      ),
    );
  }

  void _showVoiceCloneDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('声音复刻', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('请上传3-10秒的清晰语音样本', style: AppTypography.bodySmall(context)),
            const SizedBox(height: AppSpacing.md),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.xl),
              decoration: BoxDecoration(
                color: context.surfaceSecondary,
                borderRadius: AppRadius.brMedium,
                border: Border.all(color: context.borderPrimary, width: 1, style: BorderStyle.solid),
              ),
              child: Column(
                children: [
                  Icon(Icons.cloud_upload_outlined, size: 40, color: context.textTertiary),
                  const SizedBox(height: AppSpacing.sm),
                  Text('点击或拖拽上传音频文件', style: AppTypography.caption(context)),
                  Text('支持 MP3、WAV、M4A', style: AppTypography.label(context)),
                ],
              ),
            ),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('请先上传语音样本（Mock）'), duration: Duration(seconds: 2)),
              );
            },
            child: Text('开始复刻', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showVoiceEditor(BuildContext context, CharacterVoiceConfig? existing) {
    final isEdit = existing != null;
    final nameCtrl = TextEditingController(text: existing?.name ?? '');
    final presetCtrl = TextEditingController(text: existing?.preset ?? '');
    double speed = existing?.speed ?? 1.0;
    double pitch = existing?.pitch ?? 1.0;
    double volume = existing?.volume ?? 0.8;

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
              Text(isEdit ? '编辑音色' : '新增音色', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.lg),
              Text('音色名称', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: nameCtrl, hintText: '输入音色名称'),
              const SizedBox(height: AppSpacing.md),
              Text('预设类型', style: AppTypography.label(context)),
              const SizedBox(height: AppSpacing.xs),
              AmitiaTextField(controller: presetCtrl, hintText: '如：温柔、活力、沉稳'),
              const SizedBox(height: AppSpacing.md),
              Text('语速 (${speed.toStringAsFixed(1)}x)', style: AppTypography.label(context)),
              Slider(
                value: speed,
                min: 0.5,
                max: 2.0,
                divisions: 15,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => speed = v),
              ),
              Text('音调 (${pitch.toStringAsFixed(1)}x)', style: AppTypography.label(context)),
              Slider(
                value: pitch,
                min: 0.5,
                max: 2.0,
                divisions: 15,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => pitch = v),
              ),
              Text('音量 (${(volume * 100).round()}%)', style: AppTypography.label(context)),
              Slider(
                value: volume,
                min: 0.0,
                max: 1.0,
                divisions: 20,
                activeColor: context.accentPrimary,
                onChanged: (v) => setSheetState(() => volume = v),
              ),
              const SizedBox(height: AppSpacing.xl),
              AmitiaButton(
                label: isEdit ? '保存' : '添加',
                isFullWidth: true,
                onPressed: () {
                  if (nameCtrl.text.trim().isEmpty) return;
                  Navigator.pop(ctx);
                  setState(() {
                    if (isEdit) {
                      final idx = _voices.indexWhere((v) => v.id == existing.id);
                      _voices[idx] = CharacterVoiceConfig(
                        id: existing.id,
                        name: nameCtrl.text.trim(),
                        preset: presetCtrl.text.trim(),
                        speed: speed,
                        pitch: pitch,
                        volume: volume,
                        isCurrent: existing.isCurrent,
                      );
                    } else {
                      _voices.add(CharacterVoiceConfig(
                        id: 'v${DateTime.now().millisecondsSinceEpoch}',
                        name: nameCtrl.text.trim(),
                        preset: presetCtrl.text.trim(),
                        speed: speed,
                        pitch: pitch,
                        volume: volume,
                      ));
                    }
                  });
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(isEdit ? '音色已更新' : '音色已添加'), duration: const Duration(seconds: 1)),
                  );
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _showDeleteConfirm(BuildContext context, CharacterVoiceConfig voice) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除音色', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除「${voice.name}」吗？此操作不可撤销。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _voices.removeWhere((v) => v.id == voice.id);
              });
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(content: Text('音色已删除'), duration: Duration(seconds: 1)),
              );
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }
}
