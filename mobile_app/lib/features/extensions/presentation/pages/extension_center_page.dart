import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/center_navigation.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/extension_service.dart';

class ExtensionCenterPage extends ConsumerStatefulWidget {
  const ExtensionCenterPage({super.key});

  @override
  ConsumerState<ExtensionCenterPage> createState() => _ExtensionCenterPageState();
}

class _ExtensionCenterPageState extends ConsumerState<ExtensionCenterPage> {
  int _selectedCategory = 0;
  bool _loading = true;
  String? _error;
  List<ExtensionCenterCard> _installedCards = [];
  List<ExtensionCenterCard> _discoverCards = [];
  List<ExtensionCenterCard> _updateCards = [];
  List<ExtensionCenterCard> _needsActionCards = [];

  final _categories = const ['全部', 'MCP', 'Skill', 'Tools', 'UI'];

  @override
  void initState() {
    super.initState();
    _loadView();
  }

  Future<void> _loadView() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final view = await svc.getExtensionCenterView();
      if (mounted) {
        setState(() {
          _installedCards = view.installed;
          _discoverCards = view.discover;
          _updateCards = view.updates;
          _needsActionCards = view.needsAction;
          _loading = false;
        });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  bool _isInstalled(ExtensionCenterCard e) => e.status == 'installed_enabled' || e.status == 'installed_disabled';

  bool _isEnabled(ExtensionCenterCard e) => e.enabled;

  bool _matchesCategory(ExtensionCenterCard e) {
    if (_selectedCategory == 0) return true;
    final cat = _categories[_selectedCategory];
    return e.contributionTags.any((t) => t.toLowerCase() == cat.toLowerCase());
  }

  List<ExtensionCenterCard> get _filteredInstalled {
    return _installedCards.where(_matchesCategory).toList();
  }

  List<ExtensionCenterCard> get _filteredRecommended {
    return _discoverCards.where(_matchesCategory).toList();
  }

  List<ExtensionCenterCard> get _filteredUpdates {
    return _updateCards.where(_matchesCategory).toList();
  }

  List<ExtensionCenterCard> get _filteredNeedsAction {
    return _needsActionCards.where(_matchesCategory).toList();
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '扩展', navigation: AmitiaAppBarNavigation.back),
        body: SafeArea(top: false, child: const AmitiaLoadingState(message: '加载中...')),
      );
    }
    if (_error != null) {
      return AmitiaScaffold(
        appBar: AmitiaAppBar(title: '扩展', navigation: AmitiaAppBarNavigation.back),
        body: SafeArea(top: false, child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadView)),
      );
    }
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '扩展',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildManagementEntries(context),
            _buildCategoryTabs(context),
            const SizedBox(height: AppSpacing.sm),
            Expanded(
              child: ListView(
                padding: const EdgeInsets.only(bottom: AppSpacing.lg),
                children: [
                  if (_filteredNeedsAction.isNotEmpty) ...[
                    const AmitiaSectionHeader(title: '需要处理'),
                    const SizedBox(height: AppSpacing.sm),
                    ..._filteredNeedsAction.map((e) => _buildCard(context, e, isFromRecommended: false)),
                  ],
                  if (_filteredInstalled.isNotEmpty) ...[
                    const AmitiaSectionHeader(title: '已安装'),
                    const SizedBox(height: AppSpacing.sm),
                    ..._filteredInstalled.map((e) => _buildCard(context, e, isFromRecommended: false)),
                  ],
                  if (_filteredUpdates.isNotEmpty) ...[
                    const AmitiaSectionHeader(title: '可更新'),
                    const SizedBox(height: AppSpacing.sm),
                    ..._filteredUpdates.map((e) => _buildCard(context, e, isFromRecommended: false)),
                  ],
                  if (_filteredInstalled.isNotEmpty && _filteredRecommended.isNotEmpty)
                    const SizedBox(height: AppSpacing.sectionGap),
                  if (_filteredRecommended.isNotEmpty) ...[
                    const AmitiaSectionHeader(title: '推荐扩展'),
                    const SizedBox(height: AppSpacing.sm),
                    ..._filteredRecommended.map((e) => _buildCard(context, e, isFromRecommended: true)),
                  ],
                  if (_filteredInstalled.isEmpty && _filteredRecommended.isEmpty && _filteredUpdates.isEmpty && _filteredNeedsAction.isEmpty)
                    AmitiaEmptyState(
                      icon: Icons.extension_outlined,
                      title: '暂无扩展',
                      subtitle: '该分类下暂时没有可用的扩展',
                    ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildCategoryTabs(BuildContext context) {
    return SizedBox(
      height: 38,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
        itemCount: _categories.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
        itemBuilder: (context, index) {
          final isSelected = _selectedCategory == index;
          return GestureDetector(
            onTap: () {
              setState(() {
                _selectedCategory = index;
              });
            },
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: 8),
              decoration: BoxDecoration(
                color: isSelected ? context.accentPrimary : context.surfaceSecondary,
                borderRadius: AppRadius.brTag,
                border: Border.all(
                  color: isSelected ? context.accentPrimary : Colors.transparent,
                  width: 0.5,
                ),
              ),
              child: Center(
                child: Text(
                  _categories[index],
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: isSelected ? FontWeight.w500 : FontWeight.w400,
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

  Widget _buildCard(BuildContext context, ExtensionCenterCard e, {required bool isFromRecommended}) {
    final isInstalled = _isInstalled(e);
    final isEnabled = _isEnabled(e);

    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.xs,
      ),
      child: AmitiaExtensionCard(
        name: e.displayName,
        description: e.description,
        icon: Icons.extension_outlined,
        typeLabel: e.contributionTags.isNotEmpty ? e.contributionTags.first : '',
        isInstalled: isInstalled,
        isEnabled: isEnabled,
        isRecommended: isFromRecommended,
        onAction: isInstalled
            ? null
            : () {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('安装 ${e.displayName}')),
                );
              },
        onToggle: isInstalled
            ? (value) {
                ScaffoldMessenger.of(context).showSnackBar(
                  SnackBar(content: Text('${value ? "启用" : "禁用"} ${e.displayName}')),
                );
              }
            : null,
      ),
    );
  }

  Widget _buildManagementEntries(BuildContext context) {
    final entries = <_MgmtEntry>[
      _MgmtEntry(title: '扩展包', icon: Icons.inventory_2_outlined, route: AppRoutes.extensionsPackages),
      _MgmtEntry(title: 'MCP', icon: Icons.hub_outlined, route: AppRoutes.extensionsMcp),
      _MgmtEntry(title: 'Agent Skills', icon: Icons.smart_toy_outlined, route: AppRoutes.extensionsAgentSkills),
      _MgmtEntry(title: '兼容 Skills', icon: Icons.extension_outlined, route: AppRoutes.extensionsSkills),
      _MgmtEntry(title: '系统插件', icon: Icons.widgets_outlined, route: AppRoutes.extensionsPlugins),
      _MgmtEntry(title: '执行记录', icon: Icons.history_outlined, route: AppRoutes.extensionsRuns),
    ];

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const SizedBox(height: AppSpacing.sm),
        const AmitiaSectionHeader(title: '管理入口'),
        const SizedBox(height: AppSpacing.sm),
        SizedBox(
          height: 80,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            itemCount: entries.length,
            separatorBuilder: (_, _) => const SizedBox(width: AppSpacing.sm),
            itemBuilder: (context, index) {
              final e = entries[index];
              return GestureDetector(
                onTap: () => context.push(e.route),
                child: AmitiaCard(
                  padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
                  child: SizedBox(
                    width: 56,
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Container(
                          width: 36,
                          height: 36,
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brSmall,
                          ),
                          child: Icon(e.icon, size: 20, color: context.accentPrimary),
                        ),
                        const SizedBox(height: AppSpacing.xs),
                        Text(
                          e.title,
                          style: AppTypography.label(context),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          textAlign: TextAlign.center,
                        ),
                      ],
                    ),
                  ),
                ),
              );
            },
          ),
        ),
        const SizedBox(height: AppSpacing.md),
        _buildCrossCenterEntries(context),
      ],
    );
  }

  Widget _buildCrossCenterEntries(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const AmitiaSectionHeader(title: '其他中心'),
        const SizedBox(height: AppSpacing.sm),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
          child: Row(
            children: [
              Expanded(
                child: GestureDetector(
                  onTap: () => CenterNavigation.openGameCenter(context),
                  child: AmitiaCard(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
                    child: Row(
                      children: [
                        Container(
                          width: 32,
                          height: 32,
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brSmall,
                          ),
                          child: Icon(Icons.sports_esports_outlined, size: 18, color: context.accentPrimary),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('游戏中心', style: AppTypography.label(context)),
                              Text('管理游戏相关扩展', style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        Icon(Icons.chevron_right, size: 16, color: context.textSecondary),
                      ],
                    ),
                  ),
                ),
              ),
              const SizedBox(width: AppSpacing.sm),
              Expanded(
                child: GestureDetector(
                  onTap: () => CenterNavigation.openDesktopPetCenter(context),
                  child: AmitiaCard(
                    padding: const EdgeInsets.symmetric(horizontal: AppSpacing.md, vertical: AppSpacing.sm),
                    child: Row(
                      children: [
                        Container(
                          width: 32,
                          height: 32,
                          decoration: BoxDecoration(
                            color: context.accentSoft,
                            borderRadius: AppRadius.brSmall,
                          ),
                          child: Icon(Icons.pets_outlined, size: 18, color: context.accentPrimary),
                        ),
                        const SizedBox(width: AppSpacing.sm),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text('桌宠中心', style: AppTypography.label(context)),
                              Text('管理桌宠相关扩展', style: AppTypography.caption(context)),
                            ],
                          ),
                        ),
                        Icon(Icons.chevron_right, size: 16, color: context.textSecondary),
                      ],
                    ),
                  ),
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: AppSpacing.md),
      ],
    );
  }
}

class _MgmtEntry {
  final String title;
  final IconData icon;
  final String route;

  _MgmtEntry({
    required this.title,
    required this.icon,
    required this.route,
  });
}
