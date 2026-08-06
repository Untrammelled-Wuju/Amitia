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
import '../../../../core/services/providers.dart';

class ExtensionCenterPage extends ConsumerStatefulWidget {
  const ExtensionCenterPage({super.key});

  @override
  ConsumerState<ExtensionCenterPage> createState() => _ExtensionCenterPageState();
}

class _ExtensionCenterPageState extends ConsumerState<ExtensionCenterPage> {
  int _selectedCategory = 0;
  bool _loading = true;
  String? _error;
  final Map<String, bool> _installState = {};
  final Map<String, bool> _enableState = {};
  List<Map<String, dynamic>> _allExtensions = [];

  final _categories = const ['全部', 'MCP', 'Skill', '插件', '主题'];

  @override
  void initState() {
    super.initState();
    _loadExtensions();
  }

  Future<void> _loadExtensions() async {
    setState(() { _loading = true; _error = null; });
    try {
      final svc = ref.read(extensionServiceProvider);
      final skills = await svc.skills();
      final plugins = await svc.plugins();
      final agentSkills = await svc.agentSkills();

      final all = <Map<String, dynamic>>[];
      for (final s in skills) {
        final m = Map<String, dynamic>.from(s);
        m['_category'] = 'Skill';
        all.add(m);
      }
      for (final p in plugins) {
        final m = Map<String, dynamic>.from(p);
        m['_category'] = '插件';
        all.add(m);
      }
      for (final a in agentSkills) {
        final m = Map<String, dynamic>.from(a);
        m['_category'] = 'Skill';
        all.add(m);
      }
      if (mounted) setState(() { _allExtensions = all; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  bool _isInstalled(Map<String, dynamic> e) => _installState[e['id'] ?? ''] ?? ((e['isEnabled'] as bool?) ?? ((e['enabled'] as int?) == 1));

  bool _isEnabled(Map<String, dynamic> e) => _enableState[e['id'] ?? ''] ?? ((e['isEnabled'] as bool?) ?? ((e['enabled'] as int?) == 1));

  bool _matchesCategory(Map<String, dynamic> e) {
    if (_selectedCategory == 0) return true;
    return (e['_category'] ?? '').toString() == _categories[_selectedCategory];
  }

  List<Map<String, dynamic>> get _filteredInstalled {
    return _allExtensions.where((e) => _isInstalled(e) && _matchesCategory(e)).toList();
  }

  List<Map<String, dynamic>> get _filteredRecommended {
    return _allExtensions.where((e) => !_isInstalled(e) && _matchesCategory(e)).toList();
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
        body: SafeArea(top: false, child: AmitiaErrorState(message: '加载失败: $_error', onRetry: _loadExtensions)),
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

  Widget _buildExtensionCard(BuildContext context, Map<String, dynamic> e, {required bool isFromRecommended}) {
    final name = (e['name'] ?? '').toString();
    final description = (e['description'] ?? '').toString();
    final isInstalled = _isInstalled(e);
    final isEnabled = _isEnabled(e);
    final id = (e['id'] ?? '').toString();

    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppSpacing.pagePadding,
        vertical: AppSpacing.xs,
      ),
      child: AmitiaExtensionCard(
        name: name,
        description: description,
        icon: Icons.extension_outlined,
        typeLabel: (e['_category'] ?? '').toString(),
        isInstalled: isInstalled,
        isEnabled: isEnabled,
        isRecommended: isFromRecommended && ((e['isRecommended'] as bool?) ?? false),
        onAction: isInstalled
            ? null
            : () {
                setState(() {
                  _installState[id] = true;
                  _enableState[id] = true;
                });
              },
        onToggle: isInstalled
            ? (value) {
                setState(() {
                  _enableState[id] = value;
                });
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
