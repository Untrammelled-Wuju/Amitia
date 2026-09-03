import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart' hide ActionDispatcher;
import 'package:flutter/services.dart';
import 'package:shared_preferences/shared_preferences.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
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
  final FutureOr<dynamic> Function(ActionInvocation invocation)? onActionDispatch;
  final FutureOr<void> Function()? onReloadSchema;
  final Widget Function(String slotId, String? contributionId, Map<String, dynamic> context)? slotBuilder;
  final bool embedded;

  const SchemaUIRenderer({
    super.key,
    required this.document,
    required this.extensionId,
    required this.contributionId,
    this.moduleId,
    this.permissions,
    this.initialContext,
    this.dataSourceLoader,
    this.onActionDispatch,
    this.onReloadSchema,
    this.slotBuilder,
    this.embedded = false,
  });

  @override
  State<SchemaUIRenderer> createState() => _SchemaUIRendererState();
}

class _SchemaUIRendererState extends State<SchemaUIRenderer> {
  final BindingEngine _bindingEngine = const BindingEngine();
  late Map<String, dynamic> _formState;
  late Map<String, dynamic> _localState;
  final Map<String, dynamic> _storageState = {};
  final Map<String, DataSourceResult> _dataSources = {};

  @override
  void initState() {
    super.initState();
    _formState = Map<String, dynamic>.from(widget.initialContext?['formState'] ?? {});
    _localState = Map<String, dynamic>.from(widget.initialContext?['localState'] ?? {});
    final initialStorage = widget.initialContext?['storage'];
    if (initialStorage is Map) {
      _storageState.addAll(initialStorage.cast<String, dynamic>());
    }
    unawaited(_loadStorageBindings());
  }

  @override
  void didUpdateWidget(covariant SchemaUIRenderer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!identical(oldWidget.document, widget.document)) {
      unawaited(_loadStorageBindings());
    }
  }

  Future<void> _loadStorageBindings() async {
    final keys = <String>{};
    void collect(SchemaUINode node) {
      for (final binding in node.bindings) {
        if (binding.source == 'storage' && binding.path.trim().isNotEmpty) {
          keys.add(binding.path.trim());
        }
      }
      for (final child in node.children) {
        collect(child);
      }
    }
    for (final node in widget.document.children) {
      collect(node);
    }
    if (keys.isEmpty) return;
    try {
      final prefs = await SharedPreferences.getInstance();
      final loaded = <String, dynamic>{};
      for (final key in keys) {
        final value = prefs.get(key);
        if (value is String) {
          try {
            loaded[key] = jsonDecode(value);
          } catch (_) {
            loaded[key] = value;
          }
        } else if (value != null) {
          loaded[key] = value;
        }
      }
      if (!mounted || loaded.isEmpty) return;
      setState(() => _storageState.addAll(loaded));
    } catch (error) {
      debugPrint('SchemaUI storage binding load failed: $error');
    }
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
      storage: _storageState,
    );
  }

  Future<void> _handleAction(
    SchemaUIActionBinding action, {
    String nodeId = '',
  }) async {
    if (!mounted || action.actionId.trim().isEmpty) return;
    final confirmation = action.confirmation?.trim() ?? '';
    if (confirmation.isNotEmpty) {
      final confirmed = await showDialog<bool>(
            context: context,
            builder: (dialogContext) => AlertDialog(
              title: const Text('确认操作'),
              content: Text(confirmation),
              actions: [
                TextButton(
                  onPressed: () => Navigator.of(dialogContext).pop(false),
                  child: const Text('取消'),
                ),
                FilledButton(
                  onPressed: () => Navigator.of(dialogContext).pop(true),
                  child: const Text('确定'),
                ),
              ],
            ),
          ) ??
          false;
      if (!confirmed || !mounted) return;
    }

    final invocation = ActionInvocation(
      actionId: action.actionId,
      target: action.target,
      input: <String, dynamic>{
        ...?action.input,
        if (nodeId.isNotEmpty) 'node_id': nodeId,
        'form_state': Map<String, dynamic>.from(_formState),
        'local_state': Map<String, dynamic>.from(_localState),
      },
      extensionId: widget.extensionId,
      contributionId: widget.contributionId,
      ownerIdentity: <String, dynamic>{
        'extensionId': widget.extensionId,
        'contributionId': widget.contributionId,
        if (widget.moduleId != null) 'moduleId': widget.moduleId,
        if (widget.permissions != null) 'permissions': widget.permissions,
      },
    );

    final dispatcher = widget.onActionDispatch;
    if (dispatcher == null) {
      debugPrint('SchemaUI action: ${invocation.toJson()}');
      return;
    }
    try {
      final result = await Future.sync(() => dispatcher(invocation));
      if (!mounted || result is! Map) return;
      final data = result.cast<dynamic, dynamic>();
      final formState = data['form_state'] ?? data['formState'];
      final localState = data['local_state'] ?? data['localState'];
      final contextUpdate = data['context_update'] ?? data['contextUpdate'];
      if (data['clientExecute'] == true && data['text'] is String) {
        await Clipboard.setData(ClipboardData(text: data['text'] as String));
      }
      if (!mounted) return;
      setState(() {
        if (formState is Map) {
          _formState.addAll(formState.cast<String, dynamic>());
        }
        if (localState is Map) {
          _localState.addAll(localState.cast<String, dynamic>());
        }
        if (contextUpdate is Map) {
          _localState.addAll(contextUpdate.cast<String, dynamic>());
        }
      });
      final message = data['message']?.toString().trim() ?? '';
      if (message.isNotEmpty && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
      }
      final reloadSchema = data['reload_schema'] == true || data['reloadSchema'] == true;
      if (reloadSchema) {
        await Future.sync(() => widget.onReloadSchema?.call());
      }
    } catch (error) {
      debugPrint('SchemaUI host action failed: $error');
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('操作失败：${error.toString().replaceFirst('Bad state: ', '')}')),
        );
      }
    }
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
          final children = doc.children.map((node) => _buildNode(context, node, 0)).toList();
          if (widget.embedded) {
            return Padding(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.stretch,
                children: children,
              ),
            );
          }
          return ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: children,
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
      final renderedNode = _withResolvedBindings(node);
      switch (renderedNode.type) {
        case SchemaUI.nodePage:
        case SchemaUI.nodeSection:
          return _buildSection(context, renderedNode, depth);
        case SchemaUI.nodeStack:
          return _buildStack(context, renderedNode, depth);
        case SchemaUI.nodeRow:
          return _buildRow(context, renderedNode, depth);
        case SchemaUI.nodeGrid:
          return _buildGrid(context, renderedNode, depth);
        case SchemaUI.nodeTabs:
          return _buildTabs(context, renderedNode);
        case SchemaUI.nodeTabItem:
        case SchemaUI.nodeColumn:
          return _buildColumn(context, renderedNode, depth);
        case SchemaUI.nodeCard:
          return _buildCard(context, renderedNode);
        case SchemaUI.nodeText:
          return _buildText(context, renderedNode);
        case SchemaUI.nodeMarkdown:
          return _buildMarkdown(context, renderedNode);
        case SchemaUI.nodeBadge:
          return _buildBadge(context, renderedNode);
        case SchemaUI.nodeDivider:
          return _buildDivider(context, renderedNode);
        case SchemaUI.nodeIcon:
          return _buildIcon(context, renderedNode);
        case SchemaUI.nodeImage:
          return _buildImage(context, renderedNode);
        case SchemaUI.nodeField:
          return _buildField(context, renderedNode);
        case SchemaUI.nodeSelect:
          return _buildSelect(context, renderedNode);
        case SchemaUI.nodeSwitch:
          return _buildSwitch(context, renderedNode);
        case SchemaUI.nodeSlider:
          return _buildSlider(context, renderedNode);
        case SchemaUI.nodeButton:
          return _buildButton(context, renderedNode);
        case SchemaUI.nodeButtonGroup:
          return _buildButtonGroup(context, renderedNode);
        case SchemaUI.nodeList:
          return _buildList(context, renderedNode);
        case SchemaUI.nodeTable:
          return _buildTable(context, renderedNode);
        case SchemaUI.nodeEmptyState:
          return _buildNodeEmptyState(context, renderedNode);
        case SchemaUI.nodeAlert:
          return _buildAlert(context, renderedNode);
        case SchemaUI.nodeProgress:
          return _buildProgress(context, renderedNode);
        case SchemaUI.nodeCode:
          return _buildCode(context, renderedNode);
        case SchemaUI.nodeKeyValue:
          return _buildKeyValue(context, renderedNode);
        case SchemaUI.nodeResourceLink:
          return _buildResourceLink(context, renderedNode);
        case SchemaUI.nodePermissionSummary:
          return _buildPermissionSummary(context, renderedNode);
        case SchemaUI.nodeRuntimeStatus:
          return _buildRuntimeStatus(context, renderedNode);
        case SchemaUI.nodeExtensionSlot:
          return _buildExtensionSlot(context, renderedNode);
        default:
          return _buildErrorWidget(context, 'Unknown node type: ${renderedNode.type}');
      }
    } catch (e) {
      return _buildErrorWidget(context, 'Render error: $e');
    }
  }

  SchemaUINode _withResolvedBindings(SchemaUINode node) {
    if (node.bindings.isEmpty) return node;
    final props = <String, dynamic>{...?node.props};
    for (final binding in node.bindings) {
      final value = _bindingEngine.resolveBinding(binding, _buildContext());
      if (value != null || binding.defaultValue != null) {
        props[binding.path] = value;
      }
    }
    return SchemaUINode(
      id: node.id,
      type: node.type,
      props: props,
      bindings: node.bindings,
      actions: node.actions,
      visibility: node.visibility,
      children: node.children,
    );
  }

  Widget _buildExtensionSlot(BuildContext context, SchemaUINode node) {
    final builder = widget.slotBuilder;
    if (builder == null) return const SizedBox.shrink();
    final slotId = (node.props?['slotId'] ?? node.props?['slot_id'] ?? '').toString().trim();
    if (slotId.isEmpty) return _buildErrorWidget(context, 'extension_slot requires slotId');
    final contributionId = (node.props?['contributionId'] ?? node.props?['contribution_id'])?.toString();
    final props = node.props ?? const <String, dynamic>{};
    final dispatchKey = props['dispatchKey'] ?? props['dispatch_key'] ?? props['entryKey'] ?? props['entry_key'];
    final dispatchOnly = props['dispatchOnly'] ?? props['dispatch_only'] ?? props['only'] ?? props['cellId'] ?? props['cell_id'];
    return builder(slotId, contributionId, {
      ...?widget.initialContext,
      'schemaNodeId': node.id,
      if (dispatchKey != null) 'dispatchKey': dispatchKey,
      if (dispatchOnly != null) 'dispatchOnly': dispatchOnly,
    });
  }

  Widget _buildSection(BuildContext context, SchemaUINode node, int depth) {
    final title = node.props?['title'] as String?;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (title != null) ...[
          Text(title, style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.sm),
        ],
        ...node.children.map((child) => Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.componentGap),
          child: _buildNode(context, child, depth + 1),
        )),
      ],
    );
  }

  Widget _buildStack(BuildContext context, SchemaUINode node, int depth) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: node.children.map((child) => _buildNode(context, child, depth + 1)).toList(),
    );
  }

  Widget _buildRow(BuildContext context, SchemaUINode node, int depth) {
    return Wrap(
      spacing: AppSpacing.sm,
      runSpacing: AppSpacing.sm,
      children: node.children.map((child) => _buildNode(context, child, depth + 1)).toList(),
    );
  }

  Widget _buildColumn(BuildContext context, SchemaUINode node, int depth) {
    final label = node.props?['label']?.toString().trim() ?? '';
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty) ...[
          Text(label, style: AppTypography.label(context)),
          SizedBox(height: AppSpacing.sm),
        ],
        ...node.children.map(
          (child) => Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.componentGap),
            child: _buildNode(context, child, depth + 1),
          ),
        ),
      ],
    );
  }

  Widget _buildGrid(BuildContext context, SchemaUINode node, int depth) {
    final columns = (node.props?['columns'] as int?) ?? 2;
    return GridView.count(
      crossAxisCount: columns,
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisSpacing: AppSpacing.sm,
      mainAxisSpacing: AppSpacing.sm,
      children: node.children.map((child) => _buildNode(context, child, depth + 1)).toList(),
    );
  }

  Widget _buildTabs(BuildContext context, SchemaUINode node) {
    final tabs = node.children.where((c) => c.type == SchemaUI.nodeTabItem).toList();
    if (tabs.isEmpty) return const SizedBox.shrink();
    final minHeight = (node.props?['minHeight'] as num?)?.toDouble();
    final tabsWidget = DefaultTabController(
      length: tabs.length,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          TabBar(
            isScrollable: true,
            tabs: tabs.map((t) {
              final label = t.props?['label'] as String? ?? t.props?['title'] as String? ?? '';
              return Tab(text: label);
            }).toList(),
          ),
          if (minHeight != null)
            SizedBox(
              height: minHeight,
              child: _buildTabViews(context, tabs),
            )
          else
            _buildTabViews(context, tabs),
        ],
      ),
    );
    return tabsWidget;
  }

  Widget _buildTabViews(BuildContext context, List<SchemaUINode> tabs) {
    return Flexible(
      child: TabBarView(
        children: tabs.map((t) {
          if (t.children.isEmpty) return const SizedBox.shrink();
          return SingleChildScrollView(
            padding: EdgeInsets.all(AppSpacing.md),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: t.children.map((child) => _buildNode(context, child, 0)).toList(),
            ),
          );
        }).toList(),
      ),
    );
  }

  Widget _buildCard(BuildContext context, SchemaUINode node) {
    return AmitiaCard(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: node.children.map((child) => Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.sm),
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
    final lines = source.split('\n');
    final spans = <InlineSpan>[];
    for (final line in lines) {
      if (line.startsWith('## ')) {
        spans.add(TextSpan(text: '${line.substring(3)}\n', style: AppTypography.sectionTitle(context).copyWith(fontSize: 18)));
      } else if (line.startsWith('# ')) {
        spans.add(TextSpan(text: '${line.substring(2)}\n', style: AppTypography.sectionTitle(context)));
      } else if (line.startsWith('- ') || line.startsWith('* ')) {
        spans.add(TextSpan(text: '• ${line.substring(2)}\n', style: AppTypography.bodySmall(context)));
      } else {
        spans.add(_parseInline(line, AppTypography.bodySmall(context)));
        spans.add(const TextSpan(text: '\n'));
      }
    }
    return RichText(text: TextSpan(children: spans, style: AppTypography.bodySmall(context)));
  }

  TextSpan _parseInline(String text, TextStyle baseStyle) {
    final spans = <InlineSpan>[];
    final regex = RegExp(r'\*\*(.+?)\*\*|\*(.+?)\*|`(.+?)`');
    int lastEnd = 0;
    for (final match in regex.allMatches(text)) {
      if (match.start > lastEnd) {
        spans.add(TextSpan(text: text.substring(lastEnd, match.start), style: baseStyle));
      }
      if (match.group(1) != null) {
        spans.add(TextSpan(text: match.group(1), style: baseStyle.copyWith(fontWeight: FontWeight.bold)));
      } else if (match.group(2) != null) {
        spans.add(TextSpan(text: match.group(2), style: baseStyle.copyWith(fontStyle: FontStyle.italic)));
      } else if (match.group(3) != null) {
        spans.add(TextSpan(text: match.group(3), style: baseStyle.copyWith(fontFamily: 'monospace', backgroundColor: Colors.grey.withOpacity(0.15))));
      }
      lastEnd = match.end;
    }
    if (lastEnd < text.length) {
      spans.add(TextSpan(text: text.substring(lastEnd), style: baseStyle));
    }
    return TextSpan(children: spans);
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
        cacheWidth: 240,
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
            contentPadding: EdgeInsets.symmetric(horizontal: 16, vertical: 12),
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
      onPressed: action != null ? () => _handleAction(action, nodeId: node.id) : () {},
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
      padding: EdgeInsets.all(AppSpacing.md),
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

  Widget _buildCode(BuildContext context, SchemaUINode node) {
    final content = (node.props?['content'] ?? node.props?['text'] ?? node.props?['value'] ?? '').toString();
    final language = node.props?['language']?.toString().trim() ?? '';
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brSmall,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (language.isNotEmpty) ...[
            Text(language, style: AppTypography.caption(context)),
            SizedBox(height: AppSpacing.sm),
          ],
          SelectableText(
            content,
            style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace'),
          ),
        ],
      ),
    );
  }

  Widget _buildResourceLink(BuildContext context, SchemaUINode node) {
    final href = node.props?['href']?.toString().trim() ?? '';
    final text = (node.props?['text'] ?? node.props?['label'] ?? href).toString();
    final action = node.actions.isNotEmpty ? node.actions.first : null;
    return InkWell(
      borderRadius: AppRadius.brSmall,
      onTap: action == null ? null : () => _handleAction(action, nodeId: node.id),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.open_in_new, size: 16, color: context.accentPrimary),
            const SizedBox(width: 6),
            Flexible(
              child: Text(
                text,
                style: AppTypography.bodySmall(context).copyWith(
                  color: context.accentPrimary,
                  decoration: TextDecoration.underline,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPermissionSummary(BuildContext context, SchemaUINode node) {
    final title = (node.props?['title'] ?? '权限概览').toString();
    final raw = node.props?['permissions'] ?? node.props?['items'];
    final permissions = raw is List ? raw.map((item) => item.toString()).where((item) => item.trim().isNotEmpty).toList() : <String>[];
    return AmitiaCard(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(title, style: AppTypography.sectionTitle(context)),
            SizedBox(height: AppSpacing.sm),
            if (permissions.isEmpty)
              Text('无权限声明', style: AppTypography.caption(context))
            else
              ...permissions.map(
                (permission) => Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.tightGap),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Icon(Icons.circle, size: 6, color: context.textTertiary),
                      const SizedBox(width: 8),
                      Expanded(child: Text(permission, style: AppTypography.bodySmall(context))),
                    ],
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  Widget _buildRuntimeStatus(BuildContext context, SchemaUINode node) {
    final status = (node.props?['status'] ?? node.props?['state'] ?? 'unknown').toString();
    final label = (node.props?['label'] ?? '运行时状态').toString();
    final message = (node.props?['message'] ?? node.props?['detail'] ?? '').toString();
    final normalized = status.toLowerCase();
    final badgeType = switch (normalized) {
      'ready' || 'running' || 'online' || 'healthy' => BadgeType.success,
      'failed' || 'error' || 'offline' => BadgeType.error,
      'starting' || 'loading' || 'degraded' || 'paused' => BadgeType.warning,
      _ => BadgeType.neutral,
    };
    return AmitiaCard(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Expanded(child: Text(label, style: AppTypography.label(context))),
                AmitiaStatusBadge(label: status, type: badgeType),
              ],
            ),
            if (message.trim().isNotEmpty) ...[
              SizedBox(height: AppSpacing.sm),
              Text(message, style: AppTypography.bodySmall(context)),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildKeyValue(BuildContext context, SchemaUINode node) {
    final items = node.props?['items'] as List? ?? [];
    return Column(
      children: items.map((item) {
        final key = item is Map ? item['key']?.toString() ?? '' : '';
        final value = item is Map ? item['value']?.toString() ?? '' : '';
        return Padding(
          padding: EdgeInsets.only(bottom: AppSpacing.tightGap),
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
      padding: EdgeInsets.all(AppSpacing.sm),
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
    if (theme == null) return child;
    final mode = theme!.mode;
    Brightness brightness;
    switch (mode) {
      case 'light':
        brightness = Brightness.light;
        break;
      case 'dark':
        brightness = Brightness.dark;
        break;
      default:
        brightness = Theme.of(context).brightness;
    }
    if (brightness == Theme.of(context).brightness && theme!.overrides == null) {
      return child;
    }
    return Theme(
      data: Theme.of(context).copyWith(
        brightness: brightness,
        colorScheme: Theme.of(context).colorScheme.copyWith(
          brightness: brightness,
        ),
      ),
      child: child,
    );
  }
}
