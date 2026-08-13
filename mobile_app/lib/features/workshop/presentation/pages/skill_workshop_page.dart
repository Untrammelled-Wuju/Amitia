import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

final _workshopSessionsProvider = FutureProvider<List<Map<String, dynamic>>?>((ref) async {
  final svc = ref.read(extensionServiceProvider);
  final sessions = await svc.workshopSessions();
  return sessions;
});

class SkillWorkshopPage extends ConsumerWidget {
  const SkillWorkshopPage({super.key});

  BadgeType _statusBadgeType(String status) {
    switch (status) {
      case '已完成':
        return BadgeType.success;
      case '进行中':
        return BadgeType.accent;
      case '草稿':
        return BadgeType.neutral;
      default:
        return BadgeType.neutral;
    }
  }

  String _formatDate(DateTime date) {
    return '${date.month}/${date.day}';
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessionsAsync = ref.watch(_workshopSessionsProvider);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '技能制作',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
      ),
      body: sessionsAsync.when(
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
                  onPressed: () => ref.invalidate(_workshopSessionsProvider),
                ),
              ],
            ),
          ),
        ),
        data: (sessions) {
          final sessionList = sessions ?? [];
          return _SkillWorkshopContent(
            sessions: sessionList,
            onRefresh: () => ref.invalidate(_workshopSessionsProvider),
          );
        },
      ),
    );
  }
}

class _SkillWorkshopContent extends ConsumerStatefulWidget {
  final List<Map<String, dynamic>> sessions;
  final VoidCallback onRefresh;

  const _SkillWorkshopContent({required this.sessions, required this.onRefresh});

  @override
  ConsumerState<_SkillWorkshopContent> createState() => _SkillWorkshopContentState();
}

class _SkillWorkshopContentState extends ConsumerState<_SkillWorkshopContent> {
  final _descController = TextEditingController();

  @override
  void dispose() {
    _descController.dispose();
    super.dispose();
  }

  BadgeType _statusBadgeType(String status) {
    switch (status) {
      case '已完成':
        return BadgeType.success;
      case '进行中':
        return BadgeType.accent;
      case '草稿':
        return BadgeType.neutral;
      default:
        return BadgeType.neutral;
    }
  }

  String _formatDate(DateTime date) {
    return '${date.month}/${date.day}';
  }

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: widget.sessions.isEmpty
          ? AmitiaEmptyState(
              icon: Icons.psychology_outlined,
              title: '暂无制作会话',
              subtitle: '创建新技能来开始制作',
              actionText: '新建技能',
              onAction: _showCreateBottomSheet,
            )
          : ListView.builder(
              padding: const EdgeInsets.all(AppSpacing.pagePadding),
              itemCount: widget.sessions.length,
              itemBuilder: (context, index) {
                return _buildSessionCard(context, widget.sessions[index]);
              },
            ),
    );
  }

  Widget _buildSessionCard(BuildContext context, Map<String, dynamic> session) {
    final title = (session['title'] ?? session['name'] ?? '').toString();
    final type = (session['type'] ?? 'Skill').toString();
    final status = (session['status'] ?? '草稿').toString();
    final updated = DateTime.tryParse((session['updatedAt'] ?? '').toString()) ?? DateTime.now();

    return Padding(
      padding: const EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: () => _openDraftEditor(session),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    color: context.accentSoft,
                    borderRadius: AppRadius.brSmall,
                  ),
                  child: Icon(Icons.psychology_outlined, size: 22, color: context.accentPrimary),
                ),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(title, style: AppTypography.cardTitle(context)),
                      const SizedBox(height: 2),
                      Text(
                        '$type · ${_formatDate(updated)}',
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                AmitiaStatusBadge(label: status, type: _statusBadgeType(status)),
              ],
            ),
            const SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: '编辑',
                    isSecondary: true,
                    height: 38,
                    onPressed: () => _openDraftEditor(session),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '归档',
                    isSecondary: true,
                    height: 38,
                    onPressed: () => _showArchiveConfirm(session),
                  ),
                ),
                const SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '安装',
                    height: 38,
                    onPressed: () => _showInstallConfirm(session),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showCreateBottomSheet() {
    _descController.clear();
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (sheetContext) {
        return Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.lg,
            AppSpacing.pagePadding,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
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
              Text('新建技能', style: AppTypography.sectionTitle(context)),
              const SizedBox(height: AppSpacing.md),
              Text('描述你要创建的技能，系统将自动生成结构化草稿', style: AppTypography.caption(context)),
              const SizedBox(height: AppSpacing.sm),
              AmitiaTextField(
                hintText: '例如：帮我自动分类下载目录中的文件',
                controller: _descController,
                maxLines: 4,
              ),
              const SizedBox(height: AppSpacing.lg),
              AmitiaButton(
                label: '创建草稿',
                icon: Icons.auto_awesome_outlined,
                isFullWidth: true,
                onPressed: () {
                  Navigator.pop(sheetContext);
                  _createDraft();
                },
              ),
            ],
          ),
        );
      },
    );
  }

  Future<void> _createDraft() async {
    final desc = _descController.text.trim();
    if (desc.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('请输入技能描述')),
      );
      return;
    }
    final svc = ref.read(extensionServiceProvider);
    await svc.createWorkshopSession({
      'title': desc.length > 12 ? '${desc.substring(0, 12)}…' : desc,
      'type': 'Skill',
      'status': '草稿',
      'description': desc,
    });
    if (mounted) {
      widget.onRefresh();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('已创建草稿：$desc')),
      );
    }
  }

  void _openDraftEditor(Map<String, dynamic> session) {
    final draftId = (session['id'] ?? 'sd_new').toString();
    context.push(AppRoutes.skillDraftEditor(draftId));
  }

  void _showInstallConfirm(Map<String, dynamic> session) {
    final title = (session['title'] ?? session['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('安装技能', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认安装技能「$title」？安装后可在扩展中心管理。',
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
                final svc = ref.read(extensionServiceProvider);
                await svc.enableSkill(session['id'].toString());
                if (mounted) {
                  widget.onRefresh();
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('「$title」安装成功')),
                  );
                }
              },
              child: Text('安装', style: TextStyle(color: context.accentPrimary)),
            ),
          ],
        );
      },
    );
  }

  void _showArchiveConfirm(Map<String, dynamic> session) {
    final title = (session['title'] ?? session['name'] ?? '').toString();
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('归档会话', style: AppTypography.cardTitle(context)),
          content: Text(
            '确认归档「$title」？归档后将不再显示在列表中。',
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
                final svc = ref.read(extensionServiceProvider);
                await svc.removeAgentSkill(session['id'].toString());
                if (mounted) {
                  widget.onRefresh();
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text('「$title」已归档')),
                  );
                }
              },
              child: Text('归档', style: TextStyle(color: context.warning)),
            ),
          ],
        );
      },
    );
  }
}
