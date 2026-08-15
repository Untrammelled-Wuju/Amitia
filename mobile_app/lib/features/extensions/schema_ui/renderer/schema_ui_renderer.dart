import 'package:flutter/material.dart' hide ActionDispatcher;
import '../../../../../app/theme/app_colors.dart';
import '../../../../../app/theme/app_spacing.dart';
import '../../../../../app/theme/app_radius.dart';
import '../../../../../app/theme/app_typography.dart';
import '../../../../../core/widgets/amitia_misc.dart';
import '../../../../../core/widgets/amitia_button.dart';
import '../../../../../core/widgets/amitia_scaffold.dart';
import '../models/schema_ui_types.dart';
import '../engine/binding_engine.dart';
import '../engine/action_dispatcher.dart';
import '../engine/data_source_loader.dart';

class SchemaUIRenderer extends StatefulWidget {
  final SchemaUIDocument document;
  final String extensionId;
  final String contributionId;
  final String? moduleId;
  final List<String>? permissions;
  final Map<String, dynamic>? initialContext;
  final DataSourceLoader? dataSourceLoader;

  const SchemaUIRenderer({
    super.key,
    required this.document,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.initialContext,
    this.dataSourceLoader,
  });

  @override
  State<SchemaUIRenderer> createState() => _SchemaUIRendererState();
}

class _SchemaUIRendererState extends State<SchemaUIRenderer> {
  final BindingEngine _bindingEngine = const BindingEngine();
  late Map<String, dynamic> _formState;
  late Map<String, dynamic> _localState;
  final Map<String, DataSourceResult> _dataSources = {};

  @override
  void initState() {
    super.initState();
    _formState = Map<String, dynamic>.from(widget.initialContext?['formState'] ?? {});
    _localState = Map<String, dynamic>.from(widget.initialContext?['localState'] ?? {});
  }

  BindingContext _buildContext() {
    return BindingContext(
      formState: _formState,
      localState: _localState,
      runtime: widget.initialContext?['runtime'] ?? {},
      host: {
        'extensionId': widget.extensionId,
        'contributionId': widget.contributionId,
        if (widget.moduleId != null) 'moduleId': widget.moduleId,
        if (widget.permissions != null) 'permissions': widget.permissions,
      },
    );
  }

  void _handleAction(SchemaUIActionBinding action) {
    if (!mounted) return;
    ActionDispatcher(
      onDispatch: (invocation) {
        debugPrint('SchemaUI action: ${invocation.toJson()}');
      },
    ).dispatch(
      action: action,
      extensionId: widget.extensionId,
      contributionId: widget.contributionId,
      moduleId: widget.moduleId,
      permissions: widget.permissions,
    );
  }

  void _updateFormState(String path, dynamic value) {
    setState(() {
      _setPath(_formState, path, value);
    });
  }

  void _updateLocalState(String path, dynamic value) {
    setState(() {
      _setPath(_localState, path, value);
    });
  }

  void _setPath(Map<String, dynamic> map, String path, dynamic value) {
    final parts = path.split('.');
    var current = map;
    for (var i = 0; i < parts.length - 1; i++) {
      final p = parts[i];
      if (current[p] is! Map<String, dynamic>) {
        current[p] = <String, dynamic>{};
      }
      current = current[p] as Map<String, dynamic>;
    }
    current[parts.last] = value;
  }

  @override
  Widget build(BuildContext context) {
    final doc = widget.document;
    return SchemaUIThemeResolver(
      theme: doc.theme,
      child: Builder(
        builder: (context) {
          if (doc.children.isEmpty) {
            return _buildEmptyState(context);
          }
          return ListView(
            padding: const EdgeInsets.all(AppSpacing.pagePadding),
            children: doc.children.map((node) => _buildNode(context, node, 0)).toList(),
          );
        },
      ),
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    return AmitiaEmptyState(
      icon: Icons.dashboard_outlined,
      title: 'Empty Page',
      subtitle: 'No content to display',
    );
  }

  Widget _buildNode(BuildContext context, SchemaUINode node, int depth) {
    if (depth > 12) {
      return _buildErrorWidget(context, 'Node depth exceeded maximum');
    }
    if (!SchemaUI.isAllowed(node.type)) {
      return _buildErrorWidget(context, 'Unsupported node type: ${node.type}');
    }
    if (!evaluateVisibility(node.visibility, _buildContext())) {
      return const SizedBox.shrink();
    }

    try {
      switch (node.type) {
        case SchemaUI.nodePage:
        case SchemaUI.nodeSection:
          return _buildSection(context, node);
        case SchemaUI.nodeStack:
          return _buildStack(context, node);
        case SchemaUI.nodeRow:
          return _buildRow(context, node);
        case SchemaUI.nodeGrid:
          return _buildGrid(context, node);
        case SchemaUI.nodeTabs:
          return _buildTabs(context, node);
        case SchemaUI.nodeCard:
          return _buildCard(context, node);
        case SchemaUI.nodeText:
          return _buildText(context, node);
        case SchemaUI.nodeMarkdown:
          return _buildMarkdown(context, node);
        case SchemaUI.nodeBadge:
          return _buildBadge(context, node);
        case SchemaUI.nodeDivider:
          return _buildDivider(context, node);
        case SchemaUI.nodeIcon:
          return _buildIcon(context, node);
        case SchemaUI.nodeImage:
          return _buildImage(context, node);
        case SchemaUI.nodeField:
          return _buildField(context, node);
        case SchemaUI.nodeSelect:
          return _buildSelect(context, node);
        case SchemaUI.nodeSwitch:
          return _buildSwitch(context, node);
        case SchemaUI.nodeSlider:
          return _buildSlider(context, node);
        case SchemaUI.nodeButton:
          return _buildButton(context, node);
        case SchemaUI.nodeButtonGroup:
          return _buildButtonGroup(context, node);
        case SchemaUI.nodeList:
          return _buildList(context, node);
        case SchemaUI.nodeTable:
          return _buildTable(context, node);
        case SchemaUI.nodeEmptyState:
          return _buildNodeEmptyState(context, node);
        case SchemaUI.nodeAlert:
          return _buildAlert(context, node);
        case SchemaUI.nodeProgress:
          return _buildProgress(context, node);
        case SchemaUI.nodeKeyValue:
          return _buildKeyValue(context, node);
        default:
          return _buildErrorWidget(context, 'Unknown node type: ${node.type}');
      }
    } catch (e) {
      return _buildErrorWidget(context, 'Render error: $e');
    }
  }

  Widget _buildSection(BuildContext context, SchemaUINode node) {
    final title = node.props?['title'] as String?;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (title != null) ...[
          Text(title, style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
        ],
        ...node.children.map((child) => Padding(
          padding: const EdgeInsets.only(bottom: AppSpacing.componentGap),
          child: _buildNode(context, child, 0),
        )),
      ],
    );
  }

  Widget _buildStack(BuildContext context, SchemaUINode node) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: node.children.map((child) => _buildNode(context, child, 0)).toList(),
    );
  }

  Widget _buildRow(BuildContext context, SchemaUINode node) {
    return Wrap(
      spacing: AppSpacing.sm,
      runSpacing: AppSpacing.sm,
      children: node.children.map((child) => _buildNode(context, child, 0)).toList(),
    );
  }

  Widget _buildGrid(BuildContext context, SchemaUINode node) {
    final columns = (node.props?['columns'] as int?) ?? 2;
    return GridView.count(
      crossAxisCount: columns,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisSpacing: AppSpacing.sm,
      mainAxisSpacing: AppSpacing.sm,
      children: node.children.map((child) => _buildNode(context, child, 0)).toList(),
    );
  }

  Widget _buildTabs(BuildContext context, SchemaUINode node) {
    final tabs = node.children.where((c) => c.type == SchemaUI.nodeTabItem).toList();
    if (tabs.isEmpty) return const SizedBox.shrink();
    return DefaultTabController(
      length: tabs.length,
      child: Column(
        children: [
          TabBar(
            isScrollable: true,
            tabs: tabs.map((t) {
              final label = t.props?['label'] as String? ?? t.props?['title'] as String? ?? '';
              return Tab(text: label);
            }).toList(),
          ),
          SizedBox(
            height: 200,
            child: TabBarView(
              children: tabs.map((t) {
                if (t.children.isEmpty) return const SizedBox.shrink();
                return SingleChildScrollView(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  child: _buildNode(context, t.children.first, 0),
                );
              }).toList(),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCard(BuildContext context, SchemaUINode node) {
    return AmitiaCard(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: node.children.map((child) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.sm),
            child: _buildNode(context, child, 0),
          )).toList(),
        ),
      ),
    );
  }

  Widget _buildText(BuildContext context, SchemaUINode node) {
    final text = _resolveStringProp(node, 'text', '');
    final style = node.props?['variant'] as String? ?? 'body';
    final styleResolved = _textStyle(context, style);
    return Text(text, style: styleResolved);
  }

  TextStyle _textStyle(BuildContext context, String variant) {
    switch (variant) {
      case 'title':
        return AppTypography.cardTitle(context);
      case 'subtitle':
        return AppTypography.sectionTitle(context);
      case 'caption':
        return AppTypography.caption(context);
      case 'label':
        return AppTypography.label(context);
      default:
        return AppTypography.bodySmall(context);
    }
  }

  Widget _buildMarkdown(BuildContext context, SchemaUINode node) {
    final source = _resolveStringProp(node, 'source', '');
    return Text(source, style: AppTypography.bodySmall(context));
  }

  Widget _buildBadge(BuildContext context, SchemaUINode node) {
    final text = _resolveStringProp(node, 'text', '');
    final badgeType = _badgeType(node.props?['variant'] as String?);
    return AmitiaStatusBadge(label: text, type: badgeType);
  }

  BadgeType _badgeType(String? variant) {
    switch (variant) {
      case 'success': return BadgeType.success;
      case 'warning': return BadgeType.warning;
      case 'error': return BadgeType.error;
      case 'info': return BadgeType.info;
      case 'accent': return BadgeType.accent;
      default: return BadgeType.neutral;
    }
  }

  Widget _buildDivider(BuildContext context, SchemaUINode node) {
    return const Divider(height: 1);
  }

  Widget _buildIcon(BuildContext context, SchemaUINode node) {
    final iconName = node.props?['name'] as String? ?? 'help_outline';
    final size = ((node.props?['size'] as num?) ?? 24).toDouble();
    return Icon(_mapIconData(iconName), size: size, color: context.accentPrimary);
  }

  IconData _mapIconData(String name) {
    switch (name) {
      case 'home': return Icons.home_outlined;
      case 'settings': return Icons.settings_outlined;
      case 'person': return Icons.person_outlined;
      case 'star': return Icons.star_outline;
      case 'check': return Icons.check_circle_outline;
      case 'close': return Icons.close;
      case 'info': return Icons.info_outline;
      case 'warning': return Icons.warning_amber_outlined;
      default: return Icons.help_outline;
    }
  }

  Widget _buildImage(BuildContext context, SchemaUINode node) {
    final src = node.props?['src'] as String?;
    final alt = node.props?['alt'] as String? ?? '';
    if (src == null || src.isEmpty) {
      return Container(
        height: 120,
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brSmall,
        ),
        child: Center(child: Icon(Icons.image_outlined, color: context.textTertiary)),
      );
    }
    return ClipRRect(
      borderRadius: AppRadius.brSmall,
      child: Image.network(
        src,
        height: 120,
        fit: BoxFit.cover,
        errorBuilder: (_, __, ___) => Container(
          height: 120,
          color: context.surfaceSecondary,
          child: Center(child: Icon(Icons.broken_image_outlined, color: context.textTertiary)),
        ),
      ),
    );
  }

  Widget _buildField(BuildContext context, SchemaUINode node) {
    final label = node.props?['label'] as String? ?? '';
    final placeholder = node.props?['placeholder'] as String? ?? '';
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final value = _bindingEngine.resolveBinding(binding, _buildContext());
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty) ...[
          Text(label, style: AppTypography.label(context)),
          const SizedBox(height: 4),
        ],
        AmitiaTextField(
          hintText: placeholder,
          controller: TextEditingController(text: value?.toString() ?? ''),
          onChanged: (v) {
            if (binding != null) {
              _updateFormState(binding.path, v);
            }
          },
        ),
      ],
    );
  }

  Widget _buildSelect(BuildContext context, SchemaUINode node) {
    final label = node.props?['label'] as String? ?? '';
    final options = (node.props?['options'] as List?)?.map((e) {
      if (e is Map) {
        return DropdownMenuItem<String>(
          value: e['value']?.toString() ?? '',
          child: Text(e['label']?.toString() ?? e['value']?.toString() ?? ''),
        );
      }
      return DropdownMenuItem<String>(value: e.toString(), child: Text(e.toString()));
    }).toList() ?? [];
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty) ...[
          Text(label, style: AppTypography.label(context)),
          const SizedBox(height: 4),
        ],
        DropdownButtonFormField<String>(
          decoration: InputDecoration(
            filled: true,
            fillColor: context.surfaceSecondary,
            border: OutlineInputBorder(borderRadius: AppRadius.brMedium, borderSide: BorderSide.none),
            contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
          ),
          items: options,
          onChanged: (v) {
            if (v != null && node.bindings.isNotEmpty) {
              _updateFormState(node.bindings.first.path, v);
            }
          },
        ),
      ],
    );
  }

  Widget _buildSwitch(BuildContext context, SchemaUINode node) {
    final label = node.props?['label'] as String? ?? '';
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final value = _bindingEngine.resolveBinding(binding, _buildContext()) == true;
    return AmitiaSwitchTile(
      title: label,
      value: value,
      onChanged: (v) {
        if (binding != null) {
          _updateFormState(binding.path, v);
        }
      },
    );
  }

  Widget _buildSlider(BuildContext context, SchemaUINode node) {
    final label = node.props?['label'] as String? ?? '';
    final min = ((node.props?['min'] as num?) ?? 0).toDouble();
    final max = ((node.props?['max'] as num?) ?? 100).toDouble();
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final value = ((_bindingEngine.resolveBinding(binding, _buildContext()) as num?) ?? min).toDouble().clamp(min, max);
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty)
          Text(label, style: AppTypography.label(context)),
        Slider(
          value: value,
          min: min,
          max: max,
          onChanged: (v) {
            if (binding != null) {
              _updateFormState(binding.path, v);
            }
          },
        ),
      ],
    );
  }

  Widget _buildButton(BuildContext context, SchemaUINode node) {
    final label = _resolveStringProp(node, 'label', '');
    final isSecondary = node.props?['variant'] == 'secondary';
    final action = node.actions.isNotEmpty ? node.actions.first : null;
    return AmitiaButton(
      label: label,
      isSecondary: isSecondary,
      onPressed: action != null ? () => _handleAction(action) : () {},
    );
  }

  Widget _buildButtonGroup(BuildContext context, SchemaUINode node) {
    return Wrap(
      spacing: AppSpacing.sm,
      children: node.children.map((child) {
        if (child.type == SchemaUI.nodeButton) return _buildButton(context, child);
        return _buildNode(context, child, 0);
      }).toList(),
    );
  }

  Widget _buildList(BuildContext context, SchemaUINode node) {
    final items = node.props?['items'] as List? ?? [];
    return Column(
      children: items.map((item) {
        return ListTile(
          title: Text(item.toString(), style: AppTypography.bodySmall(context)),
          contentPadding: EdgeInsets.zero,
        );
      }).toList(),
    );
  }

  Widget _buildTable(BuildContext context, SchemaUINode node) {
    final headers = (node.props?['headers'] as List?)?.map((e) => e.toString()).toList() ?? [];
    final rowsRaw = node.props?['rows'] as List?;
    final rows = rowsRaw?.map((r) {
      if (r is List) return r.map((c) => c.toString()).toList();
      return [r.toString()];
    }).toList() ?? [];
    if (headers.isEmpty) return const SizedBox.shrink();
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: DataTable(
        columns: headers.map((h) => DataColumn(label: Text(h, style: AppTypography.label(context)))).toList(),
        rows: rows.map((row) => DataRow(
          cells: headers.map((h) {
            final idx = headers.indexOf(h);
            final cellValue = idx < row.length ? row[idx] : '';
            return DataCell(Text(cellValue, style: AppTypography.bodySmall(context)));
          }).toList(),
        )).toList(),
      ),
    );
  }

  Widget _buildNodeEmptyState(BuildContext context, SchemaUINode node) {
    final iconName = node.props?['icon'] as String? ?? 'inbox_outlined';
    final title = node.props?['title'] as String? ?? 'No data';
    final subtitle = node.props?['subtitle'] as String?;
    return AmitiaEmptyState(
      icon: _mapIconData(iconName),
      title: title,
      subtitle: subtitle,
    );
  }

  Widget _buildAlert(BuildContext context, SchemaUINode node) {
    final text = _resolveStringProp(node, 'text', '');
    final variant = node.props?['variant'] as String? ?? 'info';
    final color = _alertColor(context, variant);
    return Container(
      padding: const EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Row(
        children: [Icon(Icons.info_outline, color: color, size: 18), const SizedBox(width: 8), Expanded(child: Text(text, style: AppTypography.bodySmall(context)))],
      ),
    );
  }

  Color _alertColor(BuildContext context, String variant) {
    switch (variant) {
      case 'success': return context.success;
      case 'warning': return context.warning;
      case 'error': return context.error;
      default: return context.info;
    }
  }

  Widget _buildProgress(BuildContext context, SchemaUINode node) {
    final progress = ((_bindingEngine.resolveBinding(
      node.bindings.isNotEmpty ? node.bindings.first : null,
      _buildContext(),
    ) as num?) ?? 0).toDouble();
    return AmitiaProgressBar(progress: progress.clamp(0.0, 1.0));
  }

  Widget _buildKeyValue(BuildContext context, SchemaUINode node) {
    final items = node.props?['items'] as List? ?? [];
    return Column(
      children: items.map((item) {
        final key = item is Map ? item['key']?.toString() ?? '' : '';
        final value = item is Map ? item['value']?.toString() ?? '' : '';
        return Padding(
          padding: const EdgeInsets.only(bottom: AppSpacing.tightGap),
          child: Row(
            children: [
              Text(key, style: AppTypography.label(context)),
              const Spacer(),
              Text(value, style: AppTypography.bodySmall(context)),
            ],
          ),
        );
      }).toList(),
    );
  }

  String _resolveStringProp(SchemaUINode node, String key, String fallback) {
    final raw = node.props?[key];
    if (raw == null) return fallback;
    return raw.toString();
  }

  Widget _buildErrorWidget(BuildContext context, String message) {
    return Container(
      padding: const EdgeInsets.all(AppSpacing.sm),
      margin: const EdgeInsets.symmetric(vertical: 4),
      decoration: BoxDecoration(
        color: context.error.withValues(alpha: 0.08),
        borderRadius: AppRadius.brSmall,
      ),
      child: Text(message, style: AppTypography.caption(context).copyWith(color: context.error)),
    );
  }
}

class SchemaUIThemeResolver extends StatelessWidget {
  final ThemeConfig? theme;
  final Widget child;

  const SchemaUIThemeResolver({super.key, this.theme, required this.child});

  @override
  Widget build(BuildContext context) {
    return child;
  }
}
