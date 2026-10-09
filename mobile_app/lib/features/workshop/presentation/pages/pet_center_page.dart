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
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../desktop_pet/infrastructure/desktop_pet_plugin_dto.dart';
import '../../../desktop_pet/presentation/controllers/desktop_pet_plugin_controller_provider.dart';

class PetCenterPage extends ConsumerStatefulWidget {
  const PetCenterPage({super.key});

  @override
  ConsumerState<PetCenterPage> createState() => _PetCenterPageState();
}

class _PetCenterPageState extends ConsumerState<PetCenterPage> {
  List<Map<String, dynamic>> _tasks = [];
  List<DesktopPetPluginSummary> _plugins = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final api = ref.read(backendServiceProvider);
      final desktopPetApi = ref.read(desktopPetPluginApiProvider);
      final results = await Future.wait([
        api.get<Map<String, dynamic>>(
          '/api/desktop-pets/generation-tasks',
          queryParameters: {'page': 1, 'pageSize': 100},
        ),
        desktopPetApi.list(),
      ]);
      final response = results[0];
      final items = response is Map ? response['items'] : null;
      if (items is! List) {
        throw StateError('桌宠生成任务列表返回格式无效');
      }
      if (mounted) {
        setState(() {
          _tasks = items
              .whereType<Map>()
              .map((item) => Map<String, dynamic>.from(item))
              .toList();
          _plugins = (results[1] as DesktopPetPluginList).plugins;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _error = e.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final appBar = AmitiaAppBar(
      title: '桌宠制作',
      showBackButton: true,
      fallbackRoute: AppRoutes.workshop,
    );
    if (_loading) {
      return AmitiaScaffold(
        appBar: appBar,
        body: const SafeArea(child: Center(child: CircularProgressIndicator())),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: appBar,
        body: SafeArea(
          child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _load),
        ),
      );
    }

    final running = _plugins.where((p) => p.enabled).toList();
    final runningPet = running.isNotEmpty ? running.first : null;

    final activeTasks = _tasks.where((s) {
      final status = s['status']?.toString() ?? '';
      return !{
        'succeeded',
        'completed',
        'failed',
        'cancelled',
      }.contains(status);
    }).toList();

    final recentTasks = _tasks.take(3).toList();

    return AmitiaScaffold(
      appBar: appBar,
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
            _buildTaskListCard(context, activeTasks),
            SizedBox(height: AppSpacing.sectionGap),
            const AmitiaSectionHeader(title: '最近记录'),
            SizedBox(height: AppSpacing.sm),
            _buildRecentRecords(context, recentTasks),
          ],
        ),
      ),
    );
  }

  Widget _buildRunningPetCard(
    BuildContext context,
    DesktopPetPluginSummary pet,
  ) {
    final name = pet.name;

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
                    '${pet.description} · v${pet.version}',
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
      (
        Icons.add_circle_outline,
        '创建桌宠',
        () => context.push(AppRoutes.workshopPetCreate),
      ),
      (Icons.list_alt, '任务列表', () => context.push(AppRoutes.workshopPetTasks)),
      (
        Icons.install_desktop,
        '安装管理',
        () => context.push(AppRoutes.workshopPetInstallations),
      ),
    ];
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: Column(
        children: actions.map((action) {
          return Material(
            color: Colors.transparent,
            child: InkWell(
              onTap: action.$3,
              child: Container(
                constraints: const BoxConstraints(minHeight: 56),
                padding: const EdgeInsets.symmetric(vertical: 10),
                decoration: BoxDecoration(
                  border: Border(
                    bottom: BorderSide(
                      color: context.borderSecondary,
                      width: 0.5,
                    ),
                  ),
                ),
                child: Row(
                  children: [
                    Icon(action.$1, size: 22, color: context.accentPrimary),
                    SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Text(
                        action.$2,
                        style: AppTypography.body(context),
                      ),
                    ),
                    Icon(
                      Icons.chevron_right,
                      size: 20,
                      color: context.textTertiary,
                    ),
                  ],
                ),
              ),
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildTaskListCard(
    BuildContext context,
    List<Map<String, dynamic>> tasks,
  ) {
    if (tasks.isEmpty) {
      return Padding(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        child: AmitiaCard(
          child: AmitiaEmptyState(
            icon: Icons.check_circle_outline,
            title: '没有进行中的任务',
            subtitle: '创建桌宠后可在这里查看生成进度',
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
              if (i < tasks.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildTaskItem(BuildContext context, Map<String, dynamic> task) {
    final name = task['name']?.toString() ?? '';
    final selectedActionCount = (task['selectedActionCount'] is num)
        ? (task['selectedActionCount'] as num).toInt()
        : 0;
    final progress = (task['progress'] is num)
        ? (task['progress'] as num).toInt().clamp(0, 100)
        : 0;
    final status = task['status']?.toString() ?? '';
    final taskId = task['id']?.toString() ?? '';

    return GestureDetector(
      onTap: () => context.push(AppRoutes.petProcessing(taskId)),
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: EdgeInsets.symmetric(
          horizontal: AppSpacing.cardPadding,
          vertical: 12,
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(name, style: AppTypography.body(context))),
                AmitiaStatusBadge(
                  label: _statusLabel(status),
                  type: _statusBadgeType(status),
                ),
              ],
            ),
            SizedBox(height: AppSpacing.xs),
            Row(
              children: [
                Text(
                  '$selectedActionCount 个动作',
                  style: AppTypography.caption(context),
                ),
                SizedBox(width: AppSpacing.md),
                Expanded(child: AmitiaProgressBar(progress: progress / 100.0)),
                SizedBox(width: AppSpacing.sm),
                Text('$progress%', style: AppTypography.caption(context)),
              ],
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildRecentRecords(
    BuildContext context,
    List<Map<String, dynamic>> records,
  ) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        padding: EdgeInsets.symmetric(vertical: AppSpacing.xs),
        child: Column(
          children: [
            for (int i = 0; i < records.length; i++) ...[
              _buildRecordItem(context, records[i]),
              if (i < records.length - 1)
                Divider(height: 1, color: context.borderSecondary),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildRecordItem(BuildContext context, Map<String, dynamic> task) {
    final name = task['name']?.toString() ?? '';
    final modelName = task['modelName']?.toString() ?? '';
    final createdAt = task['createdAt']?.toString() ?? '';
    final status = task['status']?.toString() ?? '';

    return Padding(
      padding: EdgeInsets.symmetric(
        horizontal: AppSpacing.cardPadding,
        vertical: 10,
      ),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brExtraSmall,
            ),
            child: Icon(
              Icons.pets_outlined,
              size: 18,
              color: context.accentPrimary,
            ),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(name, style: AppTypography.body(context)),
                const SizedBox(height: 2),
                Text(
                  [
                    modelName,
                    createdAt,
                  ].where((value) => value.isNotEmpty).join(' · '),
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
          AmitiaStatusBadge(
            label: _statusLabel(status),
            type: _statusBadgeType(status),
          ),
        ],
      ),
    );
  }

  String _statusLabel(String status) {
    switch (status) {
      case 'pending':
        return '待处理';
      case 'queued':
        return '排队中';
      case 'running':
        return '生成中';
      case 'processing':
        return '处理中';
      case 'completed':
      case 'succeeded':
        return '已完成';
      case 'failed':
        return '生成失败';
      case 'cancelling':
        return '取消中';
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
      case 'running':
      case 'cancelling':
        return BadgeType.accent;
      case 'completed':
      case 'succeeded':
        return BadgeType.success;
      case 'cancelled':
      case 'failed':
        return BadgeType.error;
      default:
        return BadgeType.neutral;
    }
  }
}
