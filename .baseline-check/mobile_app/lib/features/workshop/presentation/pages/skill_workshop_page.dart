import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _workshopSessionsProvider =
    FutureProvider.autoDispose<List<Map<String, dynamic>>>((ref) async {
  return ref.read(extensionServiceProvider).workshopSessions();
});

class SkillWorkshopPage extends ConsumerStatefulWidget {
  const SkillWorkshopPage({super.key});

  @override
  ConsumerState<SkillWorkshopPage> createState() => _SkillWorkshopPageState();
}

class _SkillWorkshopPageState extends ConsumerState<SkillWorkshopPage> {
  final _requirementController = TextEditingController();
  bool _creating = false;

  @override
  void dispose() {
    _requirementController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final sessions = ref.watch(_workshopSessionsProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '技能制作',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
      ),
      body: sessions.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => _ErrorState(
          error: error,
          onRetry: () => ref.invalidate(_workshopSessionsProvider),
        ),
        data: (items) => RefreshIndicator(
          onRefresh: () async => ref.refresh(_workshopSessionsProvider.future),
          child: items.isEmpty
              ? ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  children: [
                    SizedBox(height: AppSpacing.xxl),
                    AmitiaEmptyState(
                      icon: Icons.psychology_outlined,
                      title: '暂无制作会话',
                      subtitle: '从需求描述开始，生成、校验、测试并安装一个 Skill。',
                      actionText: '新建技能',
                      onAction: _showCreateSheet,
                    ),
                  ],
                )
              : ListView(
                  physics: const AlwaysScrollableScrollPhysics(),
                  padding: EdgeInsets.all(AppSpacing.pagePadding),
                  children: [
                    Row(
                      children: [
                        Expanded(
                          child: Text(
                            '制作会话',
                            style: AppTypography.sectionTitle(context),
                          ),
                        ),
                        AmitiaButton(
                          label: '新建技能',
                          icon: Icons.add,
                          height: 38,
                          onPressed: _showCreateSheet,
                        ),
                      ],
                    ),
                    SizedBox(height: AppSpacing.md),
                    ...items.map(_sessionCard),
                  ],
                ),
        ),
      ),
    );
  }

  Widget _sessionCard(Map<String, dynamic> session) {
    final id = (session['id'] ?? '').toString();
    final requirement = (session['requirement'] ?? '').toString().trim();
    final status = (session['status'] ?? 'draft').toString();
    final revision = (session['currentRevision'] as num?)?.toInt() ?? 0;
    final installedSkillId = (session['installedSkillId'] ?? '').toString();
    final updated = DateTime.tryParse((session['updatedAt'] ?? '').toString());

    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: AmitiaCard(
        onTap: id.isEmpty ? null : () => context.push(AppRoutes.skillDraftEditor(id)),
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
                  child: Icon(Icons.extension_outlined,
                      color: context.accentPrimary, size: 22),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        requirement.isEmpty ? '未命名 Skill 会话' : requirement,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: AppTypography.cardTitle(context),
                      ),
                      const SizedBox(height: 3),
                      Text(
                        [
                          if (revision > 0) 'Revision $revision',
                          if (updated != null)
                            '${updated.month}/${updated.day} ${updated.hour.toString().padLeft(2, '0')}:${updated.minute.toString().padLeft(2, '0')}',
                        ].join(' · '),
                        style: AppTypography.caption(context),
                      ),
                    ],
                  ),
                ),
                AmitiaStatusBadge(
                  label: _statusLabel(status),
                  type: _statusBadge(status),
                ),
              ],
            ),
            if (installedSkillId.isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              Text('已安装 Skill：$installedSkillId',
                  style: AppTypography.caption(context)),
            ],
            SizedBox(height: AppSpacing.md),
            Row(
              children: [
                Expanded(
                  child: AmitiaButton(
                    label: '打开',
                    isSecondary: true,
                    height: 38,
                    onPressed: id.isEmpty
                        ? null
                        : () => context.push(AppRoutes.skillDraftEditor(id)),
                  ),
                ),
                SizedBox(width: AppSpacing.sm),
                Expanded(
                  child: AmitiaButton(
                    label: '归档',
                    isSecondary: true,
                    height: 38,
                    onPressed: id.isEmpty ? null : () => _archive(session),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }

  void _showCreateSheet() {
    _requirementController.clear();
    showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(22)),
      ),
      builder: (sheetContext) => Padding(
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
            Text('新建 Skill 制作会话',
                style: AppTypography.sectionTitle(sheetContext)),
            SizedBox(height: AppSpacing.xs),
            Text(
              '这里提交的是 Workshop 的 requirement，不再伪造 title/type/status。',
              style: AppTypography.caption(sheetContext),
            ),
            SizedBox(height: AppSpacing.md),
            AmitiaTextField(
              controller: _requirementController,
              hintText: '例如：创建一个能按扩展名整理下载目录的 Skill',
              maxLines: 4,
            ),
            SizedBox(height: AppSpacing.lg),
            AmitiaButton(
              label: _creating ? '创建中…' : '创建并进入',
              icon: Icons.auto_awesome_outlined,
              isFullWidth: true,
              onPressed: _creating
                  ? null
                  : () async {
                      Navigator.pop(sheetContext);
                      await _createSession();
                    },
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _createSession() async {
    final requirement = _requirementController.text.trim();
    if (requirement.isEmpty) {
      if (mounted) amitiaSnackBar(context, '请输入 Skill 需求描述');
      return;
    }
    setState(() => _creating = true);
    try {
      final created = await ref
          .read(extensionServiceProvider)
          .createWorkshopSession(requirement: requirement);
      final id = (created?['id'] ?? '').toString();
      ref.invalidate(_workshopSessionsProvider);
      if (!mounted) return;
      if (id.isEmpty) {
        amitiaSnackBar(context, '制作会话已创建，但后端未返回会话 ID');
        return;
      }
      context.push(AppRoutes.skillDraftEditor(id));
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '创建失败：$error');
    } finally {
      if (mounted) setState(() => _creating = false);
    }
  }

  Future<void> _archive(Map<String, dynamic> session) async {
    final id = (session['id'] ?? '').toString();
    final requirement = (session['requirement'] ?? '').toString();
    final confirmed = await showAmitiaConfirmDialog(
      context,
      title: '归档制作会话',
      message: '确定归档“${requirement.isEmpty ? id : requirement}”吗？',
      confirmLabel: '归档',
    );
    if (confirmed != true) return;
    try {
      await ref.read(extensionServiceProvider).archiveWorkshopSession(id);
      ref.invalidate(_workshopSessionsProvider);
      if (mounted) amitiaSnackBar(context, '已归档');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '归档失败：$error');
    }
  }

  String _statusLabel(String status) {
    const labels = <String, String>{
      'draft': '草稿',
      'generating': '生成中',
      'generated': '已生成',
      'validating': '校验中',
      'validation_failed': '校验失败',
      'validated': '已校验',
      'awaiting_permission_confirmation': '待确认权限',
      'testing': '测试中',
      'test_failed': '测试失败',
      'test_passed': '测试通过',
      'installing': '安装中',
      'installed': '已安装',
      'enabled': '已启用',
      'disabled': '已停用',
      'archived': '已归档',
      'error': '错误',
    };
    return labels[status] ?? status;
  }

  BadgeType _statusBadge(String status) {
    if (status == 'installed' ||
        status == 'enabled' ||
        status == 'test_passed' ||
        status == 'validated') {
      return BadgeType.success;
    }
    if (status.contains('failed') || status == 'error') {
      return BadgeType.error;
    }
    if (status == 'generating' ||
        status == 'validating' ||
        status == 'testing' ||
        status == 'installing') {
      return BadgeType.accent;
    }
    return BadgeType.neutral;
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.error, required this.onRetry});

  final Object error;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: 16),
            Text(
              '加载失败：$error',
              textAlign: TextAlign.center,
              style: AppTypography.body(context),
            ),
            const SizedBox(height: 16),
            AmitiaButton(label: '重试', onPressed: onRetry),
          ],
        ),
      ),
    );
  }
}
