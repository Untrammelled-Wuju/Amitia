import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../desktop_pet/infrastructure/desktop_pet_plugin_dto.dart';
import '../../../desktop_pet/presentation/controllers/desktop_pet_plugin_controller_provider.dart';

class PetCenterPage extends ConsumerStatefulWidget {
  const PetCenterPage({super.key});

  @override
  ConsumerState<PetCenterPage> createState() => _PetCenterPageState();
}

class _PetCenterPageState extends ConsumerState<PetCenterPage> {
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
      final plugins = await ref.read(desktopPetPluginApiProvider).list();
      if (mounted) {
        setState(() {
          _plugins = plugins.plugins;
          _loading = false;
        });
      }
    } catch (error) {
      if (mounted) {
        setState(() {
          _error = error.toString();
          _loading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final appBar = AmitiaAppBar(
      title: '桌宠中心',
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

    final running = _plugins.where((plugin) => plugin.enabled).toList();
    final runningPet = running.isNotEmpty ? running.first : null;

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
            _buildOverview(context),
          ],
        ),
      ),
    );
  }

  Widget _buildRunningPetCard(
    BuildContext context,
    DesktopPetPluginSummary plugin,
  ) {
    final name = plugin.name;
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
                    '${plugin.description} · v${plugin.version}',
                    style: AppTypography.caption(context),
                  ),
                ],
              ),
            ),
            const AmitiaStatusBadge(label: '已启用', type: BadgeType.success),
          ],
        ),
      ),
    );
  }

  Widget _buildQuickActions(BuildContext context) {
    final actions = [
      (
        Icons.install_desktop,
        '安装管理',
        () => context.push(AppRoutes.workshopPetInstallations),
      ),
      (Icons.pets, '桌宠运行', () => context.push(AppRoutes.desktopPet)),
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

  Widget _buildOverview(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
      child: AmitiaCard(
        child: Row(
          children: [
            Icon(Icons.extension_outlined, color: context.accentPrimary),
            SizedBox(width: AppSpacing.md),
            Expanded(
              child: Text(
                _plugins.isEmpty
                    ? '暂无已安装的桌宠插件'
                    : '已安装 ${_plugins.length} 个桌宠插件，其中 ${_plugins.where((item) => item.enabled).length} 个已启用',
                style: AppTypography.body(context),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
