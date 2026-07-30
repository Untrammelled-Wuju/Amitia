import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
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
        showBackButton: true,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
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
}
