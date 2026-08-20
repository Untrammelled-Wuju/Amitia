import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/services/providers.dart';

class PetActionEditorPage extends ConsumerStatefulWidget {
  final String taskId;
  final String actionKey;

  const PetActionEditorPage({
    super.key,
    required this.taskId,
    required this.actionKey,
  });

  @override
  ConsumerState<PetActionEditorPage> createState() => _PetActionEditorPageState();
}

class _PetActionEditorPageState extends ConsumerState<PetActionEditorPage> {
  Map<String, dynamic>? _skill;
  int _currentFrame = 0;
  double _playbackSpeed = 1.0;
  double _cropStart = 0.0;
  double _cropEnd = 1.0;
  double _scale = 1.0;
  double _anchorX = 0.5;
  double _anchorY = 0.5;
  bool _loop = true;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final data = await svc.getSkill(widget.actionKey);
      if (mounted) {
        if (data != null) {
          final config = data['config'] as Map<String, dynamic>?;
          setState(() {
            _skill = data;
            if (config != null) {
              _playbackSpeed = (config['playbackSpeed'] is num) ? (config['playbackSpeed'] as num).toDouble() : 1.0;
              _cropStart = (config['cropStart'] is num) ? (config['cropStart'] as num).toDouble() : 0.0;
              _cropEnd = (config['cropEnd'] is num) ? (config['cropEnd'] as num).toDouble() : 1.0;
              _scale = (config['scale'] is num) ? (config['scale'] as num).toDouble() : 1.0;
              _anchorX = (config['anchorX'] is num) ? (config['anchorX'] as num).toDouble() : 0.5;
              _anchorY = (config['anchorY'] is num) ? (config['anchorY'] as num).toDouble() : 0.5;
              _loop = config['loop'] == true;
            }
            _loading = false;
          });
        } else {
          setState(() { _loading = false; _error = '未找到该技能'; });
        }
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String get _actionName => _skill?['name']?.toString() ?? widget.actionKey;

  int get _totalFrames {
    final frames = _skill?['frames'];
    if (frames is List) return frames.length;
    final config = _skill?['config'] as Map<String, dynamic>?;
    if (config != null && config['totalFrames'] is num) {
      return (config['totalFrames'] as num).toInt();
    }
    return 8;
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const AmitiaScaffold(body: Center(child: CircularProgressIndicator()));
    }
    if (_error != null) {
      return AmitiaScaffold(
        body: SafeArea(child: Center(child: Text('加载失败: $_error'))),
      );
    }

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '动作编辑器',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Expanded(
              child: ListView(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                children: [
                  _buildPreviewSection(context),
                  SizedBox(height: AppSpacing.md),
                  _buildFrameScrubber(context),
                  SizedBox(height: AppSpacing.sectionGap),
                  _buildPlaybackSpeedSection(context),
                  SizedBox(height: AppSpacing.md),
                  _buildCropRangeSection(context),
                  SizedBox(height: AppSpacing.md),
                  _buildAnchorSection(context),
                  SizedBox(height: AppSpacing.md),
                  _buildScaleSection(context),
                  SizedBox(height: AppSpacing.md),
                  _buildLoopSection(context),
                  SizedBox(height: AppSpacing.xxl),
                ],
              ),
            ),
            _buildBottomActions(context),
          ],
        ),
      ),
    );
  }

  Widget _buildPreviewSection(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Text('帧预览', style: AppTypography.sectionTitle(context)),
            const Spacer(),
            AmitiaStatusBadge(label: _actionName, type: BadgeType.accent),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        Container(
          width: double.infinity,
          height: 220,
          decoration: BoxDecoration(
            color: context.surfaceSecondary,
            borderRadius: AppRadius.brLarge,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Stack(
            children: [
              Center(
                child: Icon(Icons.image, size: 80, color: context.textTertiary),
              ),
              Positioned(
                top: AppSpacing.sm,
                left: AppSpacing.sm,
                child: Container(
                  padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                  decoration: BoxDecoration(
                    color: context.scrim,
                    borderRadius: AppRadius.brTag,
                  ),
                  child: Text(
                    '帧 ${_currentFrame + 1}/$_totalFrames',
                    style: const TextStyle(fontSize: 12, color: Colors.white),
                  ),
                ),
              ),
              if (_loop)
                Positioned(
                  top: AppSpacing.sm,
                  right: AppSpacing.sm,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: context.accentPrimary.withValues(alpha: 0.9),
                      borderRadius: AppRadius.brTag,
                    ),
                    child: const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.loop, size: 12, color: Colors.white),
                        SizedBox(width: 4),
                        Text('循环', style: TextStyle(fontSize: 11, color: Colors.white)),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildFrameScrubber(BuildContext context) {
    final totalFrames = _totalFrames;
    return Row(
      children: [
        AmitiaIconButton(
          icon: Icons.skip_previous,
          color: context.textSecondary,
          onPressed: _currentFrame > 0
              ? () { setState(() { _currentFrame--; }); }
              : null,
        ),
        Expanded(
          child: Slider(
            value: _currentFrame.toDouble().clamp(0, (totalFrames - 1).toDouble()),
            min: 0,
            max: (totalFrames - 1).toDouble().clamp(0, double.infinity),
            divisions: totalFrames > 1 ? totalFrames - 1 : 1,
            activeColor: context.accentPrimary,
            onChanged: (value) {
              setState(() { _currentFrame = value.round(); });
            },
          ),
        ),
        AmitiaIconButton(
          icon: Icons.skip_next,
          color: context.textSecondary,
          onPressed: _currentFrame < totalFrames - 1
              ? () { setState(() { _currentFrame++; }); }
              : null,
        ),
      ],
    );
  }

  Widget _buildPlaybackSpeedSection(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.speed, size: 20, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Text('播放速度', style: AppTypography.body(context)),
              const Spacer(),
              Text('${_playbackSpeed.toStringAsFixed(1)}x', style: AppTypography.bodySmall(context)),
            ],
          ),
          Slider(
            value: _playbackSpeed,
            min: 0.25,
            max: 3.0,
            divisions: 11,
            activeColor: context.accentPrimary,
            onChanged: (value) {
              setState(() { _playbackSpeed = value; });
            },
          ),
        ],
      ),
    );
  }

  Widget _buildCropRangeSection(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.content_cut, size: 20, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Text('裁切范围', style: AppTypography.body(context)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Text('起始帧', style: AppTypography.label(context)),
          Slider(
            value: _cropStart,
            min: 0.0,
            max: 1.0,
            divisions: 20,
            activeColor: context.accentPrimary,
            onChanged: (value) {
              setState(() { _cropStart = value.clamp(0.0, _cropEnd - 0.05); });
            },
          ),
          Text('结束帧', style: AppTypography.label(context)),
          Slider(
            value: _cropEnd,
            min: 0.0,
            max: 1.0,
            divisions: 20,
            activeColor: context.accentPrimary,
            onChanged: (value) {
              setState(() { _cropEnd = value.clamp(_cropStart + 0.05, 1.0); });
            },
          ),
          SizedBox(height: AppSpacing.xs),
          Text(
            '裁切范围：${(_cropStart * 100).round()}% - ${(_cropEnd * 100).round()}%',
            style: AppTypography.caption(context),
          ),
        ],
      ),
    );
  }

  Widget _buildAnchorSection(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.gps_fixed, size: 20, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Text('锚点设置', style: AppTypography.body(context)),
            ],
          ),
          SizedBox(height: AppSpacing.sm),
          Container(
            height: 120,
            decoration: BoxDecoration(
              color: context.surfaceSecondary,
              borderRadius: AppRadius.brSmall,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: LayoutBuilder(
              builder: (context, constraints) {
                return GestureDetector(
                  onTapDown: (details) {
                    setState(() {
                      _anchorX = details.localPosition.dx / constraints.maxWidth;
                      _anchorY = details.localPosition.dy / constraints.maxHeight;
                    });
                  },
                  onPanUpdate: (details) {
                    setState(() {
                      _anchorX = (details.localPosition.dx / constraints.maxWidth).clamp(0.0, 1.0);
                      _anchorY = (details.localPosition.dy / constraints.maxHeight).clamp(0.0, 1.0);
                    });
                  },
                  child: Stack(
                    children: [
                      Center(
                        child: Container(
                          width: 40,
                          height: 40,
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            shape: BoxShape.circle,
                            border: Border.all(color: context.borderPrimary),
                          ),
                        ),
                      ),
                      Positioned(
                        left: _anchorX * constraints.maxWidth - 12,
                        top: _anchorY * constraints.maxHeight - 12,
                        child: Container(
                          width: 24,
                          height: 24,
                          decoration: BoxDecoration(
                            color: context.accentPrimary,
                            shape: BoxShape.circle,
                            border: Border.all(color: Colors.white, width: 2),
                          ),
                          child: const Icon(Icons.center_focus_strong, size: 14, color: Colors.white),
                        ),
                      ),
                    ],
                  ),
                );
              },
            ),
          ),
          SizedBox(height: AppSpacing.xs),
          Text(
            '锚点位置：X ${(_anchorX * 100).round()}% · Y ${(_anchorY * 100).round()}%',
            style: AppTypography.caption(context),
          ),
        ],
      ),
    );
  }

  Widget _buildScaleSection(BuildContext context) {
    return AmitiaCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.zoom_in, size: 20, color: context.accentPrimary),
              SizedBox(width: AppSpacing.sm),
              Text('缩放', style: AppTypography.body(context)),
              const Spacer(),
              Text('${(_scale * 100).round()}%', style: AppTypography.bodySmall(context)),
            ],
          ),
          Slider(
            value: _scale,
            min: 0.5,
            max: 2.0,
            divisions: 15,
            activeColor: context.accentPrimary,
            onChanged: (value) {
              setState(() { _scale = value; });
            },
          ),
        ],
      ),
    );
  }

  Widget _buildLoopSection(BuildContext context) {
    return AmitiaCard(
      child: Row(
        children: [
          Icon(Icons.loop, size: 20, color: context.accentPrimary),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('循环播放', style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text('动作播放完毕后自动重新开始', style: AppTypography.label(context)),
              ],
            ),
          ),
          Switch(
            value: _loop,
            activeThumbColor: context.accentPrimary,
            onChanged: (value) {
              setState(() { _loop = value; });
            },
          ),
        ],
      ),
    );
  }

  Widget _buildBottomActions(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(top: BorderSide(color: context.borderPrimary, width: 0.5)),
      ),
      child: Row(
        children: [
          Expanded(
            child: AmitiaButton(
              label: '保存草稿',
              isSecondary: true,
              icon: Icons.save_outlined,
              onPressed: _saveDraft,
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: AmitiaButton(
              label: '放弃',
              isDestructive: true,
              onPressed: _showAbandonConfirm,
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: AmitiaButton(
              label: '提交',
              icon: Icons.check,
              onPressed: _showSubmitConfirm,
            ),
          ),
        ],
      ),
    );
  }

  void _saveDraft() {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('「$_actionName」草稿已保存')),
    );
  }

  Future<void> _updateConfig() async {
    try {
      final svc = ref.read(extensionServiceProvider);
      await svc.updateSkillConfig(widget.actionKey, {
        'playbackSpeed': _playbackSpeed,
        'cropStart': _cropStart,
        'cropEnd': _cropEnd,
        'scale': _scale,
        'anchorX': _anchorX,
        'anchorY': _anchorY,
        'loop': _loop,
      });
    } catch (_) {}
  }

  void _showSubmitConfirm() {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('提交会话', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认提交动作「$_actionName」的编辑会话？提交后编辑配置将生效。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                await _updateConfig();
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('「$_actionName」会话已提交')),
                  );
                  Navigator.pop(context);
                }
              },
              child: Text('提交', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showAbandonConfirm() {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('放弃会话', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认放弃动作「$_actionName」的编辑会话？所有未保存的修改将丢失。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('保留', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('「$_actionName」会话已放弃')),
                );
                Navigator.pop(context);
              },
              child: Text('放弃', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }
}
