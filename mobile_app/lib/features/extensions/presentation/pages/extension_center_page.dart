import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class ExtensionCenterPage extends ConsumerStatefulWidget {
  const ExtensionCenterPage({super.key});

  @override
  ConsumerState<ExtensionCenterPage> createState() => _ExtensionCenterPageState();
}

class _ExtensionCenterPageState extends ConsumerState<ExtensionCenterPage> {
  int _selectedCategory = 0;
  final Map<String, bool> _installState = {};
  final Map<String, bool> _enableState = {};

  final _categories = const ['全部', 'MCP', 'Skill', '插件', '主题'];

  bool _isInstalled(Extension e) => _installState[e.id] ?? e.isInstalled;
  bool _isEnabled(Extension e) => _enableState[e.id] ?? e.isEnabled;

  bool _matchesCategory(Extension e) {
    if (_selectedCategory == 0) return true;
    return e.type.index == _selectedCategory - 1;
  }

  List<Extension> get _filteredInstalled {
    final installed = <Extension>[];
    for (final e in MockData.installedExtensions) {
      if (_isInstalled(e) && _matchesCategory(e)) installed.add(e);
    }
    for (final e in MockData.recommendedExtensions) {
      if (_isInstalled(e) && _matchesCategory(e)) installed.add(e);
    }
    return installed;
  }

  List<Extension> get _filteredRecommended {
    return MockData.recommendedExtensions
        .where((e) => !_isInstalled(e) && _matchesCategory(e))
        .toList();
  }

  @override
  Widget build(BuildContext context) {
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
                  if (_filteredInstalled.isNotEmpty) ...[
                    const AmitiaSectionHeader(title: '已安装'),
                    const SizedBox(height: AppSpacing.sm),
                    ..._filteredInstalled.map((e) => _buildExtensionCard(context, e, isFromRecommended: false)),
                  ],
                  if (_filteredInstalled.isNotEmpty && _filteredRecommended.isNotEmpty)
                    const SizedBox(height: AppSpacing.sectionGap),
                  if (_filteredRecommended.isNotEmpty) ...[
                    const AmitiaSectionHeader(title: '推荐扩展'),
                    const SizedBox(height: AppSpacing.sm),
                    ..._filteredRecommended.map((e) => _buildExtensionCard(context, e, isFromRecommended: true)),
                  ],
                  if (_filteredInstalled.isEmpty && _filteredRecommended.isEmpty)
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

  Widget _buildExtensionCard(BuildContext context, Extension e, {required bool isFromRecommended}) {
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.xs,
      ),
      child: AmitiaExtensionCard(
        name: e.name,
        description: e.description,
        icon: e.icon,
        typeLabel: _getTypeLabel(e.type),
        isInstalled: _isInstalled(e),
        isEnabled: _isEnabled(e),
        isRecommended: isFromRecommended && e.isRecommended,
        onAction: _isInstalled(e)
            ? null
            : () {
                setState(() {
                  _installState[e.id] = true;
                  _enableState[e.id] = true;
                });
              },
        onToggle: _isInstalled(e)
            ? (value) {
                setState(() {
                  _enableState[e.id] = value;
                });
              }
            : null,
      ),
    );
  }

  String _getTypeLabel(ExtensionType type) {
    switch (type) {
      case ExtensionType.mcp:
        return 'MCP';
      case ExtensionType.skill:
        return 'Skill';
      case ExtensionType.plugin:
        return '插件';
      case ExtensionType.theme:
        return '主题';
    }
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
