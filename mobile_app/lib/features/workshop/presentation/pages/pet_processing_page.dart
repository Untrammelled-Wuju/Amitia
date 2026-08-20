import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/services/providers.dart';

class PetProcessingPage extends ConsumerStatefulWidget {
  final String taskId;

  const PetProcessingPage({super.key, required this.taskId});

  @override
  ConsumerState<PetProcessingPage> createState() => _PetProcessingPageState();
}

class _PetProcessingPageState extends ConsumerState<PetProcessingPage> {
  List<Map<String, dynamic>> _processingTasks = [];
  Map<String, dynamic>? _petTask;
  int _selectedActionIndex = 0;
  int _selectedAttemptIndex = 0;
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
      final sessionData = await svc.getExtensionRun(widget.taskId);
      if (mounted) {
        if (sessionData != null) {
          await _parseSessionData(sessionData, svc);
        } else {
          final sessions = await svc.workshopSessions();
          Map<String, dynamic>? found;
          for (final s in sessions) {
            if (s['id']?.toString() == widget.taskId) {
              found = s;
              break;
            }
          }
          if (found != null) {
            await _parseSessionData(found, svc);
          } else {
            setState(() { _loading = false; _error = '未找到任务'; });
          }
        }
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<void> _parseSessionData(Map<String, dynamic> data, dynamic svc) async {
    _petTask = Map<String, dynamic>.from(data);
    final actions = data['actions'];
    if (actions is List) {
      for (final action in actions) {
        if (action is Map<String, dynamic>) {
          final runs = await _fetchActionRuns(svc, action['id']?.toString() ?? '');
          action['_runs'] = runs;
        }
      }
      _processingTasks = List<Map<String, dynamic>>.from(actions);
    } else {
      _processingTasks = [];
    }
    setState(() { _loading = false; });
  }

  Future<List<Map<String, dynamic>>> _fetchActionRuns(dynamic svc, String actionId) async {
    if (actionId.isEmpty) return [];
    try {
      final runs = await svc.extensionRuns();
      return runs.where((r) => r['actionId']?.toString() == actionId || r['skillId']?.toString() == actionId)
          .toList()
          .cast<Map<String, dynamic>>();
    } catch (_) {
      return [];
    }
  }

  String get _taskName => _petTask?['name']?.toString() ?? '未知任务';
  String get _characterName => _petTask?['characterName']?.toString() ?? _petTask?['character']?.toString() ?? '未知';

  BadgeType _statusBadgeType(String? status) {
    switch (status) {
      case 'pending':
        return BadgeType.neutral;
      case 'reviewing':
        return BadgeType.warning;
      case 'approved':
        return BadgeType.success;
      case 'rejected':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }

  String _statusLabel(String? status) {
    switch (status) {
      case 'pending':
        return '待处理';
      case 'reviewing':
        return '审核中';
      case 'approved':
        return '已通过';
      case 'rejected':
        return '已拒绝';
      default:
        return '未知';
    }
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '处理审核',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
      ),
      body: SafeArea(
        top: false,
        child: _buildBody(context),
      ),
    );
  }

  Widget _buildBody(BuildContext context) {
    if (_loading) {
      return const Center(child: CircularProgressIndicator());
    }
    if (_error != null) {
      return Center(child: Text('加载失败: $_error'));
    }
    if (_processingTasks.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.assignment_late_outlined,
        title: '暂无处理任务',
        subtitle: '该任务还没有创建处理子任务',
      );
    }
    return Column(
      children: [
        _buildTaskHeader(context),
        _buildActionBarTabs(context),
        Expanded(
          child: ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              _buildQualityBanner(context),
              SizedBox(height: AppSpacing.md),
              _buildAttemptSelector(context),
              SizedBox(height: AppSpacing.md),
              _buildFrameGrid(context),
              SizedBox(height: AppSpacing.md),
              _buildOriginalResult(context),
              SizedBox(height: AppSpacing.md),
              _buildActionButtons(context),
              SizedBox(height: AppSpacing.xxl),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildTaskHeader(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border(bottom: BorderSide(color: context.borderPrimary, width: 0.5)),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(Icons.pets_outlined, size: 20, color: context.accentPrimary),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(_taskName, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(
                  '$_characterName · ${_processingTasks.length} 个动作待处理',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildActionBarTabs(BuildContext context) {
    return SizedBox(
      height: 44,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.xs),
        itemCount: _processingTasks.length,
        separatorBuilder: (_, _) => SizedBox(width: AppSpacing.xs),
        itemBuilder: (context, index) {
          final task = _processingTasks[index];
          final actionName = task['name']?.toString() ?? task['actionName']?.toString() ?? '';
          final isSelected = index == _selectedActionIndex;
          return GestureDetector(
            onTap: () {
              setState(() {
                _selectedActionIndex = index;
                _selectedAttemptIndex = 0;
              });
            },
            child: Container(
              padding: EdgeInsets.symmetric(horizontal: AppSpacing.md),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
              ),
              child: Center(
                child: Text(
                  actionName,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected ? Colors.white : context.textSecondary,
                  ),
                ),
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildQualityBanner(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    final actionName = task['name']?.toString() ?? task['actionName']?.toString() ?? '';
    final status = task['status']?.toString();
    final qualityStatus = task['qualityStatus']?.toString() ?? '待审核';
    final completedFrames = (task['completedFrames'] is num) ? (task['completedFrames'] as num).toInt() : 0;
    final totalFrames = (task['totalFrames'] is num) ? (task['totalFrames'] as num).toInt() : 0;

    final qualityType = qualityStatus == '高质量'
        ? BadgeType.success
        : qualityStatus == '部分不合格'
            ? BadgeType.error
            : qualityStatus == '待审核'
                ? BadgeType.warning
                : BadgeType.neutral;

    return AmitiaCard(
      child: Row(
        children: [
          Icon(Icons.verified_outlined, size: 22, color: context.accentPrimary),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(actionName, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(
                  '$completedFrames/$totalFrames 帧已完成',
                  style: AppTypography.caption(context),
                ),
              ],
            ),
          ),
          Column(
            crossAxisAlignment: CrossAxisAlignment.end,
            children: [
              AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
              const SizedBox(height: 4),
              AmitiaStatusBadge(label: qualityStatus, type: qualityType),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildAttemptSelector(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    final attempts = task['attempts'] is List ? (task['attempts'] as List) : [<dynamic>[]];

    if (attempts.isEmpty) {
      return AmitiaCard(
        child: Row(
          children: [
            Icon(Icons.history, size: 20, color: context.textTertiary),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: Text('暂无 Attempt 记录', style: AppTypography.caption(context)),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('Attempt 切换', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.xs),
        AmitiaCard(
          padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
          child: Row(
            children: attempts.asMap().entries.map((entry) {
              final i = entry.key;
              final attempt = entry.value;
              final isSelected = i == _selectedAttemptIndex;
              final label = attempt is Map ? (attempt['label']?.toString() ?? 'Attempt ${i + 1}') : 'Attempt ${i + 1}';
              return Expanded(
                child: GestureDetector(
                  onTap: () => _switchAttempt(i),
                  child: Container(
                    padding: const EdgeInsets.symmetric(vertical: 10),
                    decoration: BoxDecoration(
                      color: isSelected ? context.accentSoft : Colors.transparent,
                      borderRadius: AppRadius.brSmall,
                    ),
                    child: Center(
                      child: Text(
                        label,
                        style: TextStyle(
                          fontSize: 13,
                          fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                          color: isSelected ? context.accentPrimary : context.textSecondary,
                        ),
                      ),
                    ),
                  ),
                ),
              );
            }).toList(),
          ),
        ),
      ],
    );
  }

  Widget _buildFrameGrid(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    final frames = task['frames'] is List ? (task['frames'] as List) : <dynamic>[];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('帧列表', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.xs),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          gridDelegate: SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 4,
            mainAxisSpacing: AppSpacing.sm,
            crossAxisSpacing: AppSpacing.sm,
            childAspectRatio: 0.8,
          ),
          itemCount: frames.length,
          itemBuilder: (context, index) {
            final frame = frames[index];
            return _buildFrameCell(context, frame, index);
          },
        ),
      ],
    );
  }

  Widget _buildFrameCell(BuildContext context, dynamic frame, int index) {
    final frameMap = frame is Map ? frame : <String, dynamic>{};
    final status = frameMap['status']?.toString() ?? '等待中';
    final qualityLabel = frameMap['qualityLabel']?.toString();

    return GestureDetector(
      onTap: () => _showFramePreview(frameMap.cast<String, dynamic>(), index),
      child: Container(
        decoration: BoxDecoration(
          color: _frameColor(context, status, qualityLabel),
          borderRadius: AppRadius.brSmall,
          border: Border.all(color: context.borderPrimary, width: 0.5),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              status == '等待中' ? Icons.hourglass_empty : Icons.image,
              size: 24,
              color: _frameIconColor(context, status, qualityLabel),
            ),
            SizedBox(height: AppSpacing.xs),
            Text(
              '帧 ${index + 1}',
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w500,
                color: status == '等待中' ? context.textTertiary : context.textSecondary,
              ),
            ),
            if (qualityLabel != null && qualityLabel.isNotEmpty)
              Padding(
                padding: const EdgeInsets.only(top: 2),
                child: Text(
                  qualityLabel,
                  style: TextStyle(
                    fontSize: 9,
                    color: qualityLabel == '不合格'
                        ? context.error
                        : qualityLabel == '高质量'
                            ? context.success
                            : context.textTertiary,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Color _frameColor(BuildContext context, String status, String? qualityLabel) {
    if (status == '等待中') return context.surfaceSecondary;
    if (qualityLabel == '不合格') return context.error.withValues(alpha: 0.15);
    if (qualityLabel == '高质量') return context.success.withValues(alpha: 0.15);
    return context.accentSoft;
  }

  Color _frameIconColor(BuildContext context, String status, String? qualityLabel) {
    if (status == '等待中') return context.textTertiary;
    if (qualityLabel == '不合格') return context.error;
    if (qualityLabel == '高质量') return context.success;
    return context.accentPrimary;
  }

  Widget _buildOriginalResult(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    final actionName = task['name']?.toString() ?? task['actionName']?.toString() ?? '';
    final totalFrames = (task['totalFrames'] is num) ? (task['totalFrames'] as num).toInt() : 0;
    final completedFrames = (task['completedFrames'] is num) ? (task['completedFrames'] as num).toInt() : 0;
    final qualityStatus = task['qualityStatus']?.toString() ?? '待审核';
    final attempts = task['attempts'] is List ? (task['attempts'] as List) : <dynamic>[];
    final attemptLabel = _selectedAttemptIndex < attempts.length
        ? (attempts[_selectedAttemptIndex] is Map
            ? (attempts[_selectedAttemptIndex]['label']?.toString() ?? 'Attempt ${_selectedAttemptIndex + 1}')
            : 'Attempt ${_selectedAttemptIndex + 1}')
        : '无';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('原始结果', style: AppTypography.label(context)),
        SizedBox(height: AppSpacing.xs),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              _buildResultRow(context, '动作', actionName),
              SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, '帧数', '$totalFrames 帧'),
              SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, '已完成', '$completedFrames 帧'),
              SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, 'Attempt', attemptLabel),
              SizedBox(height: AppSpacing.xs),
              _buildResultRow(context, '质量', qualityStatus),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildResultRow(BuildContext context, String label, String value) {
    return Row(
      children: [
        SizedBox(width: 64, child: Text(label, style: AppTypography.label(context))),
        Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
      ],
    );
  }

  Widget _buildActionButtons(BuildContext context) {
    final task = _processingTasks[_selectedActionIndex];
    final actionKey = task['id']?.toString() ?? task['actionKey']?.toString() ?? '';
    final actionName = task['name']?.toString() ?? task['actionName']?.toString() ?? '';

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '编辑动作',
                isSecondary: true,
                icon: Icons.edit_outlined,
                onPressed: () => context.push(AppRoutes.petActionEditor(widget.taskId, actionKey)),
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '重处理',
                isSecondary: true,
                icon: Icons.refresh,
                onPressed: () => _showReprocessConfirm(actionName),
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        Row(
          children: [
            Expanded(
              child: AmitiaButton(
                label: '排除动作',
                isSecondary: true,
                icon: Icons.block,
                onPressed: () => _showExcludeConfirm(actionName),
              ),
            ),
            SizedBox(width: AppSpacing.sm),
            Expanded(
              child: AmitiaButton(
                label: '打包',
                icon: Icons.archive_outlined,
                onPressed: () => _showPackageDialog(actionName),
              ),
            ),
          ],
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(
          label: '安装到桌宠',
          icon: Icons.install_desktop,
          isFullWidth: true,
          onPressed: () => _showInstallDialog(actionName),
        ),
      ],
    );
  }

  void _showFramePreview(Map<String, dynamic> frame, int index) {
    final status = frame['status']?.toString() ?? '等待中';
    final qualityLabel = frame['qualityLabel']?.toString() ?? '';

    showDialog(
      context: context,
      builder: (dialogContext) {
        return Dialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brLarge),
          child: Padding(
            padding: EdgeInsets.all(AppSpacing.lg),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text('帧 ${index + 1} 预览', style: AppTypography.cardTitle(context)),
                SizedBox(height: AppSpacing.md),
                Container(
                  width: 200,
                  height: 200,
                  decoration: BoxDecoration(
                    color: _frameColor(context, status, qualityLabel),
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary),
                  ),
                  child: Icon(Icons.image, size: 64, color: _frameIconColor(context, status, qualityLabel)),
                ),
                SizedBox(height: AppSpacing.md),
                Row(
                  mainAxisAlignment: MainAxisAlignment.center,
                  children: [
                    AmitiaStatusBadge(
                      label: status,
                      type: status == '已完成' ? BadgeType.success : BadgeType.neutral,
                    ),
                    if (qualityLabel.isNotEmpty) ...[
                      SizedBox(width: AppSpacing.sm),
                      AmitiaStatusBadge(
                        label: qualityLabel,
                        type: qualityLabel == '不合格'
                            ? BadgeType.error
                            : qualityLabel == '高质量'
                                ? BadgeType.success
                                : BadgeType.neutral,
                      ),
                    ],
                  ],
                ),
                SizedBox(height: AppSpacing.lg),
                AmitiaButton(
                  label: '关闭',
                  isSecondary: true,
                  isFullWidth: true,
                  onPressed: () => Navigator.pop(dialogContext),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _switchAttempt(int index) {
    setState(() { _selectedAttemptIndex = index; });
    final task = _processingTasks[_selectedActionIndex];
    final attempts = task['attempts'] is List ? (task['attempts'] as List) : <dynamic>[];
    final label = index < attempts.length && attempts[index] is Map
        ? (attempts[index]['label']?.toString() ?? 'Attempt ${index + 1}')
        : 'Attempt ${index + 1}';
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text('已切换到 $label')),
    );
  }

  void _showReprocessConfirm(String actionName) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('重处理动作', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认重新处理动作「$actionName」？将生成新的 Attempt。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('「$actionName」重处理已启动')),
                );
              },
              child: Text('确认', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showExcludeConfirm(String actionName) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('排除动作', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认排除动作「$actionName」？排除后该动作将不会安装到桌宠。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                setState(() {
                  _processingTasks.removeAt(_selectedActionIndex);
                  if (_selectedActionIndex >= _processingTasks.length && _processingTasks.isNotEmpty) {
                    _selectedActionIndex = _processingTasks.length - 1;
                  }
                });
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('动作「$actionName」已排除')),
                );
              },
              child: Text('排除', style: TextStyle(color: context.error)),
            ),
          ],
        );
      },
    );
  }

  void _showPackageDialog(String actionName) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('打包资源', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认打包动作「$actionName」的资源？打包后将生成可安装的资源包。',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('「$actionName」资源打包完成')),
                );
              },
              child: Text('打包', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showInstallDialog(String actionName) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('安装到桌宠', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认将动作「$actionName」安装到桌宠「$_taskName」？',
            style: AppTypography.body(context),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text('取消', style: TextStyle(color: context.textSecondary)),
            ),
            TextButton(
              onPressed: () {
                Navigator.pop(dialogContext);
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('动作「$actionName」安装成功')),
                );
              },
              child: Text('安装', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }
}
