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

class PetCenterPage extends ConsumerStatefulWidget {
  const PetCenterPage({super.key});

  @override
  ConsumerState<PetCenterPage> createState() => _PetCenterPageState();
}

class _PetCenterPageState extends ConsumerState<PetCenterPage> {
  List<Map<String, dynamic>> _sessions = [];
  List<Map<String, dynamic>> _plugins = [];
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
      final results = await Future.wait([
        svc.workshopSessions(),
        svc.plugins(),
      ]);
      if (mounted) {
        setState(() {
          _sessions = results[0] as List<Map<String, dynamic>>;
          _plugins = results[1] as List<Map<String, dynamic>>;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return const AmitiaScaffold(
        body: SafeArea(child: Center(child: CircularProgressIndicator())),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        body: SafeArea(child: Center(child: Text('加载失败: $_error'))),
      );
    }

    final running = _plugins.where((p) => p['isRunning'] == true || p['enabled'] == true).toList();
    final runningPet = running.isNotEmpty ? running.first : null;

    final activeSessions = _sessions.where((s) {
      final status = s['status']?.toString() ?? '';
      return status != 'completed' && status != 'cancelled';
    }).toList();

    final recentSessions = _sessions.take(3).toList();

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '桌宠制作',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
      ),
      body: SafeArea(
        top: false,
        child: ListView(
          padding: EdgeInsets.only(bottom: AppSpacing.xxl),
          children: [
            SizedBox(height: AppSpacing.sm),
            if (runningPet != null) _buildRunningPetCard(context, runningPet),
            if (runningPet != null) SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '快速操作'),
            SizedBox(height: AppSpacing.sm),
            _buildQuickActions(context),
            SizedBox(height: AppSpacing.sectionGap),
            AmitiaSectionHeader(
              title: '生成任务',
              actionText: '查看全部',
              onAction: () => context.push(AppRoutes.workshopPetTasks),
            ),
            SizedBox(height: AppSpacing.sm),
            _buildTaskListCard(context, activeSessions),
            SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '最近记录'),
            SizedBox(height: AppSpacing.sm),
            _buildRecentRecords(context, recentSessions),
          ],
        ),
      ),
    );
  }

  Widget _buildRunningPetCard(BuildContext context, Map<String, dynamic> pet) {
    final name = pet['name']?.toString() ?? '';
    final characterName = pet['characterName']?.toString() ?? pet['character']?.toString() ??'';
    final petActions = pet['actions'];
    final actionsList = petActions is List ? petActions : <dynamic>[];
    final scale = (pet['scale'] is num) ? (pet['scale'] as num).toDouble() : 1.0;

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Container(
              width: 56,
              height: 56,
              decoration: BoxDecoration(
                color: context.accentPrimary,
                shape: BoxShape.circle,
              ),
              child: Center(
                child: Text(
                  name.isNotEmpty ? name.substring(0, 1) : '?',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 22,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ),
            ),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(name, style: AppTypography.cardTitle(context)),
                  const SizedBox(height: 2),
                  Text(
                    '$characterName · ${actionsList.length} 个动作 · 缩放 ${(scale * 100).round()}%',
                    style: AppTypography.caption(context),
                  ),
                ],
              ),
            ),
            AmitiaStatusBadge(label: '运行中', type: BadgeType.success),
          ],
        ),
      ),
    );
  }

  Widget _buildQuickActions(BuildContext context) {
    final actions = [
      (Icons.add_circle_outline, '创建桌宠', () => context.push(AppRoutes.workshopPetCreate)),
      (Icons.list_alt, '任务列表', () => context.push(AppRoutes.workshopPetTasks)),
      (Icons.install_desktop, '安装管理', () => context.push(AppRoutes.workshopPetInstallations)),
    ];
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Row(
        children: actions.map((action) {
          return Expanded(
            child: Padding(
              padding: EdgeInsets.only(right: AppSpacing.sm),
              child: GestureDetector(
                onTap: action.$3,
                child: AmitiaCard(
                  padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
                  child: Column(
                    children: [
                      Icon(action.$1, size: 26, color: context.accentPrimary),
                      SizedBox(height: AppSpacing.xs),
                      Text(action.$2, style: AppTypography.bodySmall(context)),
                    ],
                  ),
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildTaskListCard(BuildContext context, List<Map<String, dynamic>> tasks) {
    if (tasks.isEmpty) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaCard(
          child: AmitiaEmptyState(
            icon: Icons.check_circle_outline,
            title: '没有进行中的任务',
            subtitle: '所有生成任务已完成',
          ),
        ),
      );
    }
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < tasks.length; i++) ...[
              _buildTaskItem(context, tasks[i]),
              if (i < tasks.length - 1) Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTaskItem(BuildContext context, Map<String, dynamic> task) {
    final name = task['name']?.toString() ?? '';
    final completedActions = (task['completedActions'] is num) ? (task['completedActions'] as num).toInt() : 0;
    final totalActions = (task['totalActions'] is num) ? (task['totalActions'] as num).toInt() : 0;
    final progress = (task['progress'] is num) ? (task['progress'] as num).toInt() : 0;
    final status = task['status']?.toString() ?? '';
    final sessionId = task['id']?.toString() ?? '';

    return GestureDetector(
      onTap: () => context.push(AppRoutes.petProcessing(sessionId)),
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(name, style: AppTypography.body(context)),
                ),
                AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
              ],
            ),
            SizedBox(height: AppSpacing.xs),
            Row(
              children: [
                Text(
                  '$completedActions/$totalActions 动作',
                  style: AppTypography.caption(context),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(
                  child: AmitiaProgressBar(progress: progress / 100.0),
                ),
                SizedBox(width: AppSpacing.sm),
                Text('$progress%', style: AppTypography.caption(context)),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildRecentRecords(BuildContext context, List<Map<String, dynamic>> records) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < records.length; i++) ...[
              _buildRecordItem(context, records[i]),
              if (i < records.length - 1) Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildRecordItem(BuildContext context, Map<String, dynamic> task) {
    final name = task['name']?.toString() ?? '';
    final characterName = task['characterName']?.toString() ?? task['character']?.toString() ?? '';
    final createdAt = task['createdAt']?.toString() ?? '';
    final status = task['status']?.toString() ?? '';
    final progress = (task['progress'] is num) ? (task['progress'] as num).toInt() : 0;

    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.cardPadding, vertical: 10),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brExtraSmall,
            ),
            child: Icon(Icons.pets_outlined, size: 18, color: context.accentPrimary),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(
                  '$characterName · $createdAt',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
          AmitiaStatusBadge(label: _statusLabel(status), type: _statusBadgeType(status)),
        ],
      ),
    );
  }

  String _statusLabel(String status) {
    switch (status) {
      case 'pending':
        return '待处理';
      case 'processing':
        return '处理中';
      case 'completed':
        return '已完成';
      case 'cancelled':
        return '已取消';
      default:
        return status;
    }
  }

  BadgeType _statusBadgeType(String status) {
    switch (status) {
      case 'pending':
        return BadgeType.neutral;
      case 'processing':
        return BadgeType.accent;
      case 'completed':
        return BadgeType.success;
      case 'cancelled':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }
}
