import 'dart:async';
import 'dart:convert';
import 'package:flutter/material.dart' hide ActionDispatcher;
import 'package:flutter/services.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:webview_flutter/webview_flutter.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/design_tokens.dart';
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
  final Set<String> _dismissedNodeIds = <String>{};

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
    unawaited(_loadDataSources());
  }

  @override
  void didUpdateWidget(covariant SchemaUIRenderer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (!identical(oldWidget.document, widget.document) ||
        oldWidget.extensionId != widget.extensionId ||
        oldWidget.contributionId != widget.contributionId) {
      widget.dataSourceLoader?.invalidate(widget.extensionId, widget.contributionId);
      unawaited(_loadStorageBindings());
      unawaited(_loadDataSources());
    }
  }

  @override
  void dispose() {
    widget.dataSourceLoader?.cancel(widget.extensionId, widget.contributionId);
    super.dispose();
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

  Map<String, dynamic> _asMap(dynamic value) {
    if (value is Map<String, dynamic>) return Map<String, dynamic>.from(value);
    if (value is Map) return value.map((key, value) => MapEntry(key.toString(), value));
    return <String, dynamic>{};
  }

  Map<String, dynamic> _dataSourceGroup(String type) {
    final group = <String, dynamic>{};
    for (final source in widget.document.dataSources.where((item) => item.type == type)) {
      final result = _dataSources[source.id];
      if (result == null || !result.hasData) continue;
      final data = result.data;
      if (data is Map) group.addAll(_asMap(data));
      group[source.id] = data;
    }
    return group;
  }

  Map<String, dynamic> _allDataSourceValues() {
    final values = <String, dynamic>{};
    for (final entry in _dataSources.entries) {
      if (entry.value.hasData) values[entry.key] = entry.value.data;
    }
    return values;
  }

  Map<String, dynamic> _dataSourceInput(SchemaUIDataSource source) {
    final perSource = widget.initialContext?['dataSourceInput'];
    if (perSource is Map && perSource[source.id] is Map) {
      return _asMap(perSource[source.id]);
    }
    return _asMap(widget.initialContext?['input']);
  }

  Future<void> _loadDataSources() async {
    final loader = widget.dataSourceLoader;
    if (loader == null || widget.document.dataSources.isEmpty) {
      if (mounted && _dataSources.isNotEmpty) setState(_dataSources.clear);
      return;
    }
    final candidates = widget.document.dataSources.where(loader.requiresFetch).toList(growable: false);
    final fetchLimit = widget.document.performanceBudget?.maxDataFetchCount ?? 0;
    final sources = fetchLimit > 0
        ? candidates.take(fetchLimit).toList(growable: false)
        : candidates;
    if (sources.isEmpty) return;
    if (mounted) {
      setState(() {
        for (final source in sources) {
          _dataSources[source.id] = const DataSourceResult(isLoading: true);
        }
      });
    }
    final results = await Future.wait(
      sources.map((source) async => MapEntry(
            source.id,
            await loader.load(
              DataSourceRequest(
                dataSource: source,
                input: _dataSourceInput(source),
                extensionId: widget.extensionId,
                contributionId: widget.contributionId,
              ),
            ),
          )),
    );
    if (!mounted) return;
    setState(() {
      for (final entry in results) {
        _dataSources[entry.key] = entry.value;
      }
    });
  }

  BindingContext _buildContext() {
    final input = <String, dynamic>{
      ..._asMap(widget.initialContext?['input']),
      ..._dataSourceGroup('input'),
    };
    final query = <String, dynamic>{
      ..._asMap(widget.initialContext?['query']),
      ..._dataSourceGroup('query'),
    };
    final runtime = <String, dynamic>{
      ..._asMap(widget.initialContext?['runtime']),
      ..._dataSourceGroup('runtime'),
    };
    final runtimeStatus = <String, dynamic>{
      ..._asMap(widget.initialContext?['runtimeStatus'] ?? widget.initialContext?['runtime_status']),
      ..._dataSourceGroup('runtime_status'),
    };
    final resourceList = <String, dynamic>{
      ..._asMap(widget.initialContext?['resourceList'] ?? widget.initialContext?['resource_list']),
      ..._dataSourceGroup('resource_list'),
    };
    final allDataSources = _allDataSourceValues();
    final host = <String, dynamic>{
      ..._asMap(widget.initialContext?['host']),
      'extensionId': widget.extensionId,
      'contributionId': widget.contributionId,
      if (widget.moduleId != null) 'moduleId': widget.moduleId,
      if (widget.permissions != null) 'permissions': widget.permissions,
    };
    return BindingContext(
      input: input,
      formState: _formState,
      localState: _localState,
      query: query,
      runtime: runtime,
      host: host,
      storage: _storageState,
      runtimeStatus: runtimeStatus,
      resourceList: resourceList,
      dataSources: allDataSources,
      flat: <String, dynamic>{
        ...?widget.initialContext,
        ...allDataSources,
        ..._localState,
        ..._formState,
      },
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

  int _countNodes(SchemaUINode node) {
    var total = 1;
    for (final child in node.children) {
      total += _countNodes(child);
    }
    return total;
  }

  int _documentNodeCount(SchemaUIDocument document) {
    var total = 0;
    for (final node in document.children) {
      total += _countNodes(node);
    }
    return total;
  }

  @override
  Widget build(BuildContext context) {
    final doc = widget.document;
    if (doc.schemaVersion != 'schema-ui/1') {
      return _buildErrorWidget(
        context,
        'Schema UI 版本不兼容：期望 schema-ui/1，实际 ${doc.schemaVersion}',
      );
    }
    final nodeLimit = doc.performanceBudget?.maxNodeCount ?? 0;
    if (nodeLimit > 0) {
      final nodeCount = _documentNodeCount(doc);
      if (nodeCount > nodeLimit) {
        return _buildErrorWidget(
          context,
          'Schema UI 节点数量超出性能预算：$nodeCount > $nodeLimit',
        );
      }
    }
    Widget content = Builder(
      builder: (context) {
        if (doc.children.isEmpty) return _buildEmptyState(context);
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
    );
    final accessibility = doc.accessibility;
    if (accessibility != null && accessibility.enabled) {
      final accessibilityChild = content;
      content = Builder(
        builder: (context) {
          final media = MediaQuery.of(context);
          Widget accessible = MediaQuery(
            data: media.copyWith(
              highContrast: media.highContrast || accessibility.highContrast,
              disableAnimations: media.disableAnimations || accessibility.reducedMotion,
            ),
            child: Semantics(
              container: true,
              label: doc.title ?? 'Schema UI',
              liveRegion: accessibility.screenReader,
              child: accessibilityChild,
            ),
          );
          if (accessibility.keyboardNav) accessible = FocusTraversalGroup(child: accessible);
          return accessible;
        },
      );
    }
    final locale = _parseLocale(doc.locale?.current);
    if (locale != null) content = Localizations.override(context: context, locale: locale, child: content);
    return SchemaUIThemeResolver(theme: doc.theme, child: content);
  }

  Locale? _parseLocale(String? raw) {
    final normalized = raw?.trim().replaceAll('_', '-') ?? '';
    if (normalized.isEmpty) return null;
    final parts = normalized.split('-');
    if (parts.length == 1) return Locale(parts.first);
    return Locale(parts.first, parts[1]);
  }

  Future<void> _handleActions(
    List<SchemaUIActionBinding> actions, {
    String nodeId = '',
  }) async {
    for (final action in actions) {
      await _handleAction(action, nodeId: nodeId);
      if (!mounted) return;
    }
  }

  Future<void> _openResourceHref(String href) async {
    final uri = Uri.tryParse(href.trim());
    if (uri == null || !const {'http', 'https'}.contains(uri.scheme.toLowerCase())) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('仅支持打开 http/https 资源链接')),
        );
      }
      return;
    }
    final controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.disabled)
      ..loadRequest(uri);
    if (!mounted) return;
    await Navigator.of(context).push<void>(
      MaterialPageRoute(
        builder: (pageContext) => Scaffold(
          appBar: AppBar(
            title: Text(uri.host.isEmpty ? '资源链接' : uri.host),
          ),
          body: SafeArea(child: WebViewWidget(controller: controller)),
        ),
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
          return _buildList(context, renderedNode, depth);
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
    if (node.bindings.isEmpty && node.dataSource == null) return node;
    final props = <String, dynamic>{...?node.props};
    for (final binding in node.bindings) {
      final value = _bindingEngine.resolveBinding(binding, _buildContext());
      if (value != null || binding.defaultValue != null) {
        props[binding.path] = value;
      }
    }
    final dataSource = node.dataSource;
    if (dataSource != null) {
      final value = _bindingEngine.resolveBinding(dataSource, _buildContext());
      if (value != null || dataSource.defaultValue != null) {
        props['__boundValue'] = value;
      }
    }
    return SchemaUINode(
      id: node.id,
      type: node.type,
      props: props,
      bindings: node.bindings,
      actions: node.actions,
      visibility: node.visibility,
      disabledWhen: node.disabledWhen,
      dataSource: node.dataSource,
      children: node.children,
    );
  }

  dynamic _resolvedNodeValue(SchemaUINode node, [SchemaUIBinding? binding]) {
    final activeBinding = binding ?? (node.bindings.isNotEmpty ? node.bindings.first : null);
    return _bindingEngine.resolveBinding(activeBinding, _buildContext()) ??
        node.props?['__boundValue'] ??
        node.props?['value'];
  }

  bool _isNodeDisabled(SchemaUINode node) {
    if (node.props?['disabled'] == true) return true;
    return node.disabledWhen.isNotEmpty &&
        evaluateVisibility(node.disabledWhen, _buildContext());
  }

  double _dimension(dynamic value, double fallback) {
    if (value is num) return value.toDouble();
    final text = value?.toString().trim().replaceAll(RegExp(r'px$'), '') ?? '';
    return double.tryParse(text) ?? fallback;
  }

  Color? _schemaColor(dynamic raw) {
    var value = raw?.toString().trim() ?? '';
    if (!value.startsWith('#')) return null;
    value = value.substring(1);
    if (value.length == 3) value = value.split('').map((c) => '$c$c').join();
    if (value.length == 6) value = 'FF$value';
    if (value.length != 8) return null;
    final parsed = int.tryParse(value, radix: 16);
    return parsed == null ? null : Color(parsed);
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
    final fallback = props['fallback']?.toString().trim();
    final layout = props['layout']?.toString().trim();
    final surfaceRole = (props['surfaceRole'] ?? props['surface_role'])?.toString().trim();
    return builder(slotId, contributionId, {
      ...?widget.initialContext,
      'schemaNodeId': node.id,
      if (dispatchKey != null) 'dispatchKey': dispatchKey,
      if (dispatchOnly != null) 'dispatchOnly': dispatchOnly,
      if (fallback != null && fallback.isNotEmpty) 'slotFallback': fallback,
      if (layout != null && layout.isNotEmpty) 'slotLayout': layout,
      if (surfaceRole != null && surfaceRole.isNotEmpty) 'surfaceRole': surfaceRole,
    });
  }

  Widget _buildSection(BuildContext context, SchemaUINode node, int depth) {
    final title = node.props?['title']?.toString().trim() ?? '';
    final subtitle = node.props?['subtitle']?.toString().trim() ?? '';
    final bordered = node.props?['bordered'] != false;
    final content = Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (title.isNotEmpty) ...[
          Text(title, style: AppTypography.sectionTitle(context)),
          if (subtitle.isNotEmpty) ...[
            const SizedBox(height: 2),
            Text(subtitle, style: AppTypography.caption(context)),
          ],
          SizedBox(height: AppSpacing.sm),
        ],
        ...node.children.map((child) => Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.componentGap),
              child: _buildNode(context, child, depth + 1),
            )),
      ],
    );
    return AmitiaCard(
      border: bordered ? null : Border.all(color: Colors.transparent, width: 0),
      child: content,
    );
  }

  Widget _buildStack(BuildContext context, SchemaUINode node, int depth) {
    final gap = _dimension(node.props?['gap'], AppSpacing.sm);
    final alignment = switch (node.props?['align']?.toString().trim().toLowerCase()) {
      'center' => CrossAxisAlignment.center,
      'end' || 'flex-end' => CrossAxisAlignment.end,
      'stretch' => CrossAxisAlignment.stretch,
      _ => CrossAxisAlignment.start,
    };
    final children = <Widget>[];
    for (var index = 0; index < node.children.length; index++) {
      if (index > 0 && gap > 0) children.add(SizedBox(height: gap));
      children.add(_buildNode(context, node.children[index], depth + 1));
    }
    return Column(crossAxisAlignment: alignment, children: children);
  }

  Widget _buildRow(BuildContext context, SchemaUINode node, int depth) {
    final gap = _dimension(node.props?['gap'], AppSpacing.sm);
    final alignment = switch (node.props?['justify']?.toString().trim().toLowerCase()) {
      'center' => WrapAlignment.center,
      'end' || 'flex-end' => WrapAlignment.end,
      'space-between' || 'spacebetween' => WrapAlignment.spaceBetween,
      'space-around' || 'spacearound' => WrapAlignment.spaceAround,
      'space-evenly' || 'spaceevenly' => WrapAlignment.spaceEvenly,
      _ => WrapAlignment.start,
    };
    final crossAlignment = switch (node.props?['align']?.toString().trim().toLowerCase()) {
      'center' => WrapCrossAlignment.center,
      'end' || 'flex-end' => WrapCrossAlignment.end,
      _ => WrapCrossAlignment.start,
    };
    return Wrap(
      spacing: gap,
      runSpacing: gap,
      alignment: alignment,
      crossAxisAlignment: crossAlignment,
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

  int _gridColumnCount(SchemaUINode node, double availableWidth, double gap) {
    final rawColumns = node.props?['columns'];
    final explicit = rawColumns is num
        ? rawColumns.toInt()
        : int.tryParse(rawColumns?.toString() ?? '');
    if (explicit != null && explicit > 0) return explicit.clamp(1, 12).toInt();

    final template = node.props?['columnsTemplate']?.toString().trim() ?? '';
    if (template.isNotEmpty) {
      final repeat = RegExp(r'repeat\(\s*(\d+)\s*,', caseSensitive: false).firstMatch(template);
      final repeated = int.tryParse(repeat?.group(1) ?? '');
      if (repeated != null && repeated > 0) return repeated.clamp(1, 12).toInt();

      final minmax = RegExp(r'minmax\(\s*([0-9]+(?:\.[0-9]+)?)px', caseSensitive: false).firstMatch(template);
      final minWidth = double.tryParse(minmax?.group(1) ?? '');
      if (minWidth != null && minWidth > 0 && availableWidth.isFinite && availableWidth > 0) {
        return ((availableWidth + gap) / (minWidth + gap)).floor().clamp(1, 12).toInt();
      }

      final fractionalTracks = RegExp(r'(?:(?:^|\s))(?:[0-9]+(?:\.[0-9]+)?)fr(?=\s|$)', caseSensitive: false)
          .allMatches(template)
          .length;
      if (fractionalTracks > 0) return fractionalTracks.clamp(1, 12).toInt();
    }

    if (availableWidth.isFinite && availableWidth > 0) {
      return ((availableWidth + gap) / (220 + gap)).floor().clamp(1, 12).toInt();
    }
    return 2;
  }

  Widget _buildGrid(BuildContext context, SchemaUINode node, int depth) {
    final gap = _dimension(node.props?['gap'], AppSpacing.sm);
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = _gridColumnCount(node, constraints.maxWidth, gap);
        return GridView.count(
          crossAxisCount: columns,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          crossAxisSpacing: gap,
          mainAxisSpacing: gap,
          children: node.children.map((child) => _buildNode(context, child, depth + 1)).toList(),
        );
      },
    );
  }

  Widget _buildTabs(BuildContext context, SchemaUINode node) {
    final tabs = node.children.where((c) => c.type == SchemaUI.nodeTabItem).toList();
    if (tabs.isEmpty) return const SizedBox.shrink();
    final minHeight = (node.props?['minHeight'] as num?)?.toDouble();
    return _SchemaTabsView(
      tabs: tabs,
      position: node.props?['position']?.toString() ?? 'top',
      variant: node.props?['variant']?.toString() ?? 'line',
      minHeight: minHeight,
      isDisabled: _isNodeDisabled,
      contentBuilder: (tab) {
        if (tab.children.isEmpty) return const SizedBox.shrink();
        return Padding(
          padding: EdgeInsets.all(AppSpacing.md),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: tab.children.map((child) => _buildNode(context, child, 0)).toList(),
          ),
        );
      },
    );
  }

  Widget _buildCard(BuildContext context, SchemaUINode node) {
    final title = node.props?['title']?.toString().trim() ?? '';
    final bordered = node.props?['bordered'] != false;
    final shadow = node.props?['shadow']?.toString().trim().toLowerCase() ?? 'hover';
    Widget card = AmitiaCard(
      border: bordered ? null : Border.all(color: Colors.transparent, width: 0),
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.md),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (title.isNotEmpty) ...[
              Text(title, style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
            ],
            ...node.children.map((child) => Padding(
                  padding: EdgeInsets.only(bottom: AppSpacing.sm),
                  child: _buildNode(context, child, 0),
                )),
          ],
        ),
      ),
    );
    if (shadow != 'never' && shadow != 'none') {
      card = DecoratedBox(
        decoration: BoxDecoration(
          borderRadius: AppRadius.brMedium,
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: shadow == 'always' ? 0.12 : 0.07),
              blurRadius: shadow == 'always' ? 14 : 8,
              offset: const Offset(0, 3),
            ),
          ],
        ),
        child: card,
      );
    }
    return card;
  }

  Widget _buildText(BuildContext context, SchemaUINode node) {
    final text = (node.props?['text'] ?? node.props?['content'] ?? _resolvedNodeValue(node) ?? '').toString();
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
    final source = (node.props?['content'] ?? node.props?['text'] ?? node.props?['source'] ?? '').toString();
    final lines = source.split(RegExp(r'\r?\n'));
    final widgets = <Widget>[];
    final codeLines = <String>[];
    var inCode = false;

    void flushCode() {
      if (codeLines.isEmpty) return;
      widgets.add(_markdownCodeBlock(context, codeLines.join('\n')));
      codeLines.clear();
    }

    for (final rawLine in lines) {
      final line = rawLine;
      if (RegExp(r'^\s*```').hasMatch(line)) {
        if (inCode) flushCode();
        inCode = !inCode;
        continue;
      }
      if (inCode) {
        codeLines.add(line);
        continue;
      }
      final heading = RegExp(r'^(#{1,6})\s+(.*)$').firstMatch(line);
      if (heading != null) {
        final level = heading.group(1)!.length;
        final size = switch (level) { 1 => 22.0, 2 => 19.0, 3 => 17.0, _ => 15.0 };
        widgets.add(_markdownInlineText(
          context,
          heading.group(2) ?? '',
          AppTypography.sectionTitle(context).copyWith(fontSize: size),
        ));
        continue;
      }
      final unordered = RegExp(r'^[-*+]\s+(.*)$').firstMatch(line);
      if (unordered != null) {
        widgets.add(_markdownListRow(context, '•', unordered.group(1) ?? ''));
        continue;
      }
      final ordered = RegExp(r'^(\d+)\.\s+(.*)$').firstMatch(line);
      if (ordered != null) {
        widgets.add(_markdownListRow(context, '${ordered.group(1)}.', ordered.group(2) ?? ''));
        continue;
      }
      final quote = RegExp(r'^>\s?(.*)$').firstMatch(line);
      if (quote != null) {
        widgets.add(Container(
          width: double.infinity,
          padding: EdgeInsets.only(left: AppSpacing.sm, top: 4, bottom: 4),
          decoration: BoxDecoration(
            border: Border(left: BorderSide(color: context.borderPrimary, width: 3)),
          ),
          child: _markdownInlineText(
            context,
            quote.group(1) ?? '',
            AppTypography.bodySmall(context).copyWith(color: context.textSecondary),
          ),
        ));
        continue;
      }
      if (RegExp(r'^(-{3,}|\*{3,}|_{3,})$').hasMatch(line.trim())) {
        widgets.add(const Divider(height: 16));
        continue;
      }
      if (line.trim().isEmpty) {
        widgets.add(SizedBox(height: AppSpacing.sm));
        continue;
      }
      widgets.add(_markdownInlineText(context, line, AppTypography.bodySmall(context)));
    }
    if (inCode || codeLines.isNotEmpty) flushCode();

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: widgets
          .map((item) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.tightGap),
                child: item,
              ))
          .toList(growable: false),
    );
  }

  Widget _markdownListRow(BuildContext context, String marker, String text) {
    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(width: 28, child: Text(marker, style: AppTypography.bodySmall(context))),
        Expanded(child: _markdownInlineText(context, text, AppTypography.bodySmall(context))),
      ],
    );
  }

  Widget _markdownCodeBlock(BuildContext context, String code) {
    return Container(
      width: double.infinity,
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: context.surfaceSecondary,
        borderRadius: AppRadius.brSmall,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: SelectableText(
        code,
        style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace'),
      ),
    );
  }

  Widget _markdownInlineText(BuildContext context, String text, TextStyle baseStyle) {
    return Text.rich(
      TextSpan(children: _parseInlineSpans(context, text, baseStyle), style: baseStyle),
      softWrap: true,
    );
  }

  List<InlineSpan> _parseInlineSpans(BuildContext context, String text, TextStyle baseStyle) {
    final spans = <InlineSpan>[];
    final regex = RegExp(
      r'`([^`]+)`|\*\*([^*]+)\*\*|__([^_]+)__|\*([^*]+)\*|_([^_]+)_|\[([^\]]+)\]\((https?://[^)]+)\)|(https?://[^\s]+)',
      caseSensitive: false,
    );
    var lastEnd = 0;
    for (final match in regex.allMatches(text)) {
      if (match.start > lastEnd) {
        spans.add(TextSpan(text: text.substring(lastEnd, match.start), style: baseStyle));
      }
      if (match.group(1) != null) {
        spans.add(TextSpan(
          text: match.group(1),
          style: baseStyle.copyWith(fontFamily: 'monospace', backgroundColor: context.surfaceSecondary),
        ));
      } else if (match.group(2) != null || match.group(3) != null) {
        spans.add(TextSpan(text: match.group(2) ?? match.group(3), style: baseStyle.copyWith(fontWeight: FontWeight.bold)));
      } else if (match.group(4) != null || match.group(5) != null) {
        spans.add(TextSpan(text: match.group(4) ?? match.group(5), style: baseStyle.copyWith(fontStyle: FontStyle.italic)));
      } else {
        final label = match.group(6) ?? match.group(8) ?? '';
        final href = match.group(7) ?? match.group(8) ?? '';
        spans.add(WidgetSpan(
          alignment: PlaceholderAlignment.baseline,
          baseline: TextBaseline.alphabetic,
          child: InkWell(
            onTap: href.isEmpty ? null : () => _openResourceHref(href),
            child: Text(
              label,
              style: baseStyle.copyWith(
                color: context.accentPrimary,
                decoration: TextDecoration.underline,
              ),
            ),
          ),
        ));
      }
      lastEnd = match.end;
    }
    if (lastEnd < text.length) {
      spans.add(TextSpan(text: text.substring(lastEnd), style: baseStyle));
    }
    return spans;
  }

  Widget _buildBadge(BuildContext context, SchemaUINode node) {
    if (_dismissedNodeIds.contains(node.id)) return const SizedBox.shrink();
    final text = (node.props?['text'] ?? node.props?['label'] ?? _resolvedNodeValue(node) ?? '').toString();
    final badgeType = _badgeType((node.props?['variant'] ?? node.props?['type'])?.toString());
    final badge = AmitiaStatusBadge(label: text, type: badgeType);
    if (node.props?['closable'] != true) return badge;
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        badge,
        const SizedBox(width: 2),
        IconButton(
          visualDensity: VisualDensity.compact,
          tooltip: '关闭',
          onPressed: () => setState(() => _dismissedNodeIds.add(node.id)),
          icon: const Icon(Icons.close, size: 14),
        ),
      ],
    );
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
    final direction = node.props?['direction']?.toString().trim().toLowerCase() ?? 'horizontal';
    final text = node.props?['text']?.toString().trim() ?? '';
    if (direction == 'vertical') {
      return SizedBox(
        height: _dimension(node.props?['height'], 24),
        child: const VerticalDivider(width: 1),
      );
    }
    if (text.isEmpty) return const Divider(height: 1);
    final position = node.props?['position']?.toString().trim().toLowerCase() ?? 'center';
    final leadingFlex = position == 'left' || position == 'start' ? 0 : 1;
    final trailingFlex = position == 'right' || position == 'end' ? 0 : 1;
    return Row(
      children: [
        if (leadingFlex > 0) const Expanded(child: Divider(height: 1)),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 8),
          child: Text(text, style: AppTypography.caption(context)),
        ),
        if (trailingFlex > 0) const Expanded(child: Divider(height: 1)),
      ],
    );
  }

  Widget _buildIcon(BuildContext context, SchemaUINode node) {
    final iconName = (node.props?['name'] ?? node.props?['symbol'] ?? node.props?['label'] ?? 'help_outline').toString();
    final size = _dimension(node.props?['size'], 24);
    final color = _schemaColor(node.props?['color']) ?? context.accentPrimary;
    return Icon(_mapIconData(iconName), size: size, color: color);
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
    final src = node.props?['src']?.toString().trim() ?? '';
    final alt = node.props?['alt']?.toString() ?? '';
    final width = (node.props?['width'] as num?)?.toDouble();
    final height = (node.props?['height'] as num?)?.toDouble();
    final preview = node.props?['preview'] == true;
    final fit = _boxFit(node.props?['fit']?.toString());
    if (src.isEmpty) {
      return Container(
        width: width,
        height: height ?? 80,
        decoration: BoxDecoration(
          color: context.surfaceSecondary,
          borderRadius: AppRadius.brSmall,
        ),
        child: Center(child: Icon(Icons.image_outlined, color: context.textTertiary)),
      );
    }

    final image = ClipRRect(
      borderRadius: AppRadius.brSmall,
      child: _schemaImage(
        context,
        src,
        width: width,
        height: height,
        fit: fit,
        alt: alt,
      ),
    );
    if (!preview) return image;
    return InkWell(
      borderRadius: AppRadius.brSmall,
      onTap: () => showDialog<void>(
        context: context,
        builder: (dialogContext) => Dialog(
          insetPadding: const EdgeInsets.all(20),
          child: Stack(
            children: [
              InteractiveViewer(
                minScale: 0.5,
                maxScale: 5,
                child: _schemaImage(
                  dialogContext,
                  src,
                  fit: BoxFit.contain,
                  alt: alt,
                ),
              ),
              Positioned(
                top: 8,
                right: 8,
                child: IconButton.filledTonal(
                  onPressed: () => Navigator.of(dialogContext).pop(),
                  icon: const Icon(Icons.close),
                ),
              ),
            ],
          ),
        ),
      ),
      child: image,
    );
  }

  Widget _schemaImage(
    BuildContext context,
    String src, {
    double? width,
    double? height,
    required BoxFit fit,
    required String alt,
  }) {
    Widget error() => Container(
          width: width,
          height: height ?? 80,
          color: context.surfaceSecondary,
          alignment: Alignment.center,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(Icons.broken_image_outlined, color: context.textTertiary),
              if (alt.trim().isNotEmpty) ...[
                const SizedBox(height: 4),
                Text(alt, style: AppTypography.caption(context), textAlign: TextAlign.center),
              ],
            ],
          ),
        );

    if (src.startsWith('data:image/')) {
      final comma = src.indexOf(',');
      if (comma > 0) {
        try {
          final bytes = base64Decode(src.substring(comma + 1));
          return Image.memory(
            bytes,
            width: width,
            height: height,
            fit: fit,
            errorBuilder: (_, __, ___) => error(),
          );
        } catch (_) {
          return error();
        }
      }
    }
    return Image.network(
      src,
      width: width,
      height: height,
      fit: fit,
      errorBuilder: (_, __, ___) => error(),
      loadingBuilder: (context, child, progress) => progress == null
          ? child
          : Container(
              width: width,
              height: height ?? 80,
              color: context.surfaceSecondary,
              alignment: Alignment.center,
              child: const CircularProgressIndicator(strokeWidth: 2),
            ),
    );
  }

  BoxFit _boxFit(String? value) {
    switch (value?.trim().toLowerCase()) {
      case 'contain':
        return BoxFit.contain;
      case 'fill':
        return BoxFit.fill;
      case 'none':
        return BoxFit.none;
      case 'scale-down':
      case 'scaledown':
        return BoxFit.scaleDown;
      case 'fit-width':
      case 'fitwidth':
        return BoxFit.fitWidth;
      case 'fit-height':
      case 'fitheight':
        return BoxFit.fitHeight;
      case 'cover':
      default:
        return BoxFit.cover;
    }
  }

  bool _isEditableBinding(SchemaUIBinding? binding) {
    return binding != null && (binding.source == 'form' || binding.source == 'form_state');
  }

  void _updateBinding(SchemaUIBinding? binding, dynamic value) {
    if (!_isEditableBinding(binding)) return;
    _updateFormState(binding!.path, value);
  }

  Widget _buildField(BuildContext context, SchemaUINode node) {
    final label = (node.props?['label'] ?? node.props?['title'] ?? '').toString();
    final required = node.props?['required'] == true;
    final error = node.props?['error']?.toString().trim() ?? '';
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final disabled = _isNodeDisabled(node) || !_isEditableBinding(binding);
    if (node.children.isNotEmpty) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (label.isNotEmpty) ...[
            Text(required ? '$label *' : label, style: AppTypography.label(context)),
            const SizedBox(height: 4),
          ],
          ...node.children.map((child) => _buildNode(context, child, 0)),
          if (error.isNotEmpty) ...[
            const SizedBox(height: 4),
            Text(error, style: AppTypography.caption(context).copyWith(color: context.error)),
          ],
        ],
      );
    }
    final value = _resolvedNodeValue(node, binding);
    final text = value?.toString() ?? '';
    final rows = ((node.props?['rows'] as num?) ?? 1).toInt().clamp(1, 20);
    final variant = node.props?['variant']?.toString().trim().toLowerCase() ?? 'text';
    final multiline = variant == 'textarea' || rows > 1;
    final rawMaxLength = node.props?['maxlength'] ?? node.props?['maxLength'];
    final maxLength = rawMaxLength is num ? rawMaxLength.toInt() : int.tryParse(rawMaxLength?.toString() ?? '');
    final clearable = node.props?['clearable'] == true;
    final keyboardType = switch (variant) {
      'number' => TextInputType.number,
      'email' || 'emailaddress' => TextInputType.emailAddress,
      'url' => TextInputType.url,
      _ => multiline ? TextInputType.multiline : TextInputType.text,
    };
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty) ...[
          Text(required ? '$label *' : label, style: AppTypography.label(context)),
          const SizedBox(height: 4),
        ],
        AmitiaTextField(
          key: ValueKey('${node.id}:$text'),
          hintText: node.props?['placeholder']?.toString() ?? '',
          controller: TextEditingController(text: text),
          readOnly: disabled,
          obscureText: variant == 'password',
          keyboardType: keyboardType,
          maxLines: multiline ? rows : 1,
          maxLength: maxLength != null && maxLength > 0 ? maxLength : null,
          showCounter: node.props?['showWordLimit'] == true,
          errorText: error.isEmpty ? null : error,
          suffixIcon: clearable && text.isNotEmpty && !disabled
              ? IconButton(
                  onPressed: () => _updateBinding(binding, ''),
                  icon: const Icon(Icons.close, size: 18),
                )
              : null,
          onChanged: disabled ? null : (v) => _updateBinding(binding, v),
        ),
      ],
    );
  }

  Widget _buildSelect(BuildContext context, SchemaUINode node) {
    final label = (node.props?['label'] ?? node.props?['title'] ?? '').toString();
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final rawValue = _resolvedNodeValue(node, binding);
    final multiple = node.props?['multiple'] == true;
    final clearable = node.props?['clearable'] == true;
    final filterable = node.props?['filterable'] == true;
    final disabled = _isNodeDisabled(node) || !_isEditableBinding(binding);
    final rawOptions = node.props?['options'];
    final options = <_SchemaSelectOption>[];
    if (rawOptions is List) {
      for (final raw in rawOptions) {
        if (raw is Map) {
          final value = raw.containsKey('value') ? raw['value'] : (raw['id'] ?? raw['label'] ?? raw['text']);
          options.add(
            _SchemaSelectOption(
              value: value,
              label: (raw['label'] ?? raw['text'] ?? value ?? '').toString(),
            ),
          );
        } else {
          options.add(_SchemaSelectOption(value: raw, label: raw?.toString() ?? ''));
        }
      }
    }

    final selectedValues = multiple
        ? (rawValue is List ? List<dynamic>.from(rawValue) : <dynamic>[])
        : <dynamic>[if (rawValue != null) rawValue];
    final selectedLabels = options
        .where((option) => selectedValues.any((value) => value == option.value))
        .map((option) => option.label)
        .where((item) => item.isNotEmpty)
        .toList();
    final placeholder = node.props?['placeholder']?.toString() ?? '';
    final displayText = selectedLabels.isNotEmpty
        ? selectedLabels.join(multiple ? '、' : '')
        : (rawValue != null && !multiple ? rawValue.toString() : '');

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty) ...[
          Text(label, style: AppTypography.label(context)),
          const SizedBox(height: 4),
        ],
        InputDecorator(
          isEmpty: displayText.isEmpty,
          decoration: InputDecoration(
            hintText: placeholder,
            filled: true,
            fillColor: context.surfaceSecondary,
            border: OutlineInputBorder(
              borderRadius: AppRadius.brMedium,
              borderSide: BorderSide.none,
            ),
            contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
            enabled: !disabled,
          ),
          child: Row(
            children: [
              Expanded(
                child: InkWell(
                  onTap: disabled
                      ? null
                      : () => _openSelectPicker(
                            node: node,
                            binding: binding,
                            options: options,
                            selectedValues: selectedValues,
                            multiple: multiple,
                            filterable: filterable,
                          ),
                  child: Padding(
                    padding: const EdgeInsets.symmetric(vertical: 10),
                    child: Text(
                      displayText.isEmpty ? placeholder : displayText,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.body(context).copyWith(
                        color: displayText.isEmpty ? context.textTertiary : context.textPrimary,
                      ),
                    ),
                  ),
                ),
              ),
              if (clearable && selectedValues.isNotEmpty && !disabled)
                IconButton(
                  tooltip: '清除',
                  visualDensity: VisualDensity.compact,
                  onPressed: () => _updateBinding(binding, multiple ? <dynamic>[] : null),
                  icon: const Icon(Icons.close, size: 18),
                )
              else
                Icon(Icons.arrow_drop_down, color: disabled ? context.textTertiary : context.textSecondary),
            ],
          ),
        ),
      ],
    );
  }

  Future<void> _openSelectPicker({
    required SchemaUINode node,
    required SchemaUIBinding? binding,
    required List<_SchemaSelectOption> options,
    required List<dynamic> selectedValues,
    required bool multiple,
    required bool filterable,
  }) async {
    if (!_isEditableBinding(binding) || _isNodeDisabled(node)) return;
    final working = List<dynamic>.from(selectedValues);
    var query = '';
    await showModalBottomSheet<void>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      builder: (sheetContext) {
        return StatefulBuilder(
          builder: (sheetContext, setSheetState) {
            final normalizedQuery = query.trim().toLowerCase();
            final visibleOptions = normalizedQuery.isEmpty
                ? options
                : options
                    .where((option) => option.label.toLowerCase().contains(normalizedQuery))
                    .toList(growable: false);
            return SafeArea(
              child: Padding(
                padding: EdgeInsets.only(
                  left: 16,
                  right: 16,
                  bottom: 16 + MediaQuery.viewInsetsOf(sheetContext).bottom,
                ),
                child: ConstrainedBox(
                  constraints: BoxConstraints(
                    maxHeight: MediaQuery.sizeOf(sheetContext).height * 0.72,
                  ),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (filterable) ...[
                        TextField(
                          autofocus: true,
                          decoration: const InputDecoration(
                            prefixIcon: Icon(Icons.search),
                            hintText: '筛选选项',
                          ),
                          onChanged: (value) => setSheetState(() => query = value),
                        ),
                        const SizedBox(height: 8),
                      ],
                      Flexible(
                        child: visibleOptions.isEmpty
                            ? const Center(child: Padding(
                                padding: EdgeInsets.all(24),
                                child: Text('没有匹配的选项'),
                              ))
                            : ListView.builder(
                                shrinkWrap: true,
                                itemCount: visibleOptions.length,
                                itemBuilder: (context, index) {
                                  final option = visibleOptions[index];
                                  final selected = working.any((value) => value == option.value);
                                  if (multiple) {
                                    return CheckboxListTile(
                                      value: selected,
                                      title: Text(option.label),
                                      controlAffinity: ListTileControlAffinity.leading,
                                      onChanged: (checked) {
                                        setSheetState(() {
                                          working.removeWhere((value) => value == option.value);
                                          if (checked == true) working.add(option.value);
                                        });
                                      },
                                    );
                                  }
                                  return ListTile(
                                    leading: selected ? const Icon(Icons.check) : const SizedBox(width: 24),
                                    title: Text(option.label),
                                    onTap: () {
                                      _updateBinding(binding, option.value);
                                      Navigator.of(sheetContext).pop();
                                    },
                                  );
                                },
                              ),
                      ),
                      if (multiple) ...[
                        const SizedBox(height: 8),
                        Row(
                          children: [
                            TextButton(
                              onPressed: working.isEmpty ? null : () => setSheetState(working.clear),
                              child: const Text('清空'),
                            ),
                            const Spacer(),
                            FilledButton(
                              onPressed: () {
                                _updateBinding(binding, List<dynamic>.from(working));
                                Navigator.of(sheetContext).pop();
                              },
                              child: const Text('确定'),
                            ),
                          ],
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            );
          },
        );
      },
    );
  }

  Widget _buildSwitch(BuildContext context, SchemaUINode node) {
    final label = (node.props?['label'] ?? node.props?['title'] ?? '').toString();
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final raw = _resolvedNodeValue(node, binding);
    final activeValue = node.props?.containsKey('activeValue') == true ? node.props!['activeValue'] : true;
    final inactiveValue = node.props?.containsKey('inactiveValue') == true ? node.props!['inactiveValue'] : false;
    final value = raw == activeValue || (activeValue == true && raw == true);
    final activeText = node.props?['activeText']?.toString().trim() ?? '';
    final inactiveText = node.props?['inactiveText']?.toString().trim() ?? '';
    final subtitle = value ? activeText : inactiveText;
    final disabled = _isNodeDisabled(node) || !_isEditableBinding(binding);
    return AmitiaSwitchTile(
      title: label,
      subtitle: subtitle.isEmpty ? null : subtitle,
      value: value,
      onChanged: disabled
          ? null
          : (enabled) => _updateBinding(binding, enabled ? activeValue : inactiveValue),
    );
  }

  Widget _buildSlider(BuildContext context, SchemaUINode node) {
    final label = (node.props?['label'] ?? node.props?['title'] ?? '').toString();
    final min = _dimension(node.props?['min'], 0);
    final rawMax = _dimension(node.props?['max'], 100);
    final max = rawMax > min ? rawMax : min + 1;
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final raw = _resolvedNodeValue(node, binding);
    final parsed = raw is num ? raw.toDouble() : double.tryParse(raw?.toString() ?? '') ?? min;
    final value = parsed.clamp(min, max).toDouble();
    final rawStep = _dimension(node.props?['step'], 0);
    final divisions = rawStep > 0 ? ((max - min) / rawStep).round().clamp(1, 10000) : null;
    final disabled = _isNodeDisabled(node) || !_isEditableBinding(binding);
    final showInput = node.props?['showInput'] == true;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (label.isNotEmpty) Text(label, style: AppTypography.label(context)),
        Row(
          children: [
            Expanded(
              child: Slider(
                value: value,
                min: min,
                max: max,
                divisions: divisions,
                onChanged: disabled ? null : (v) => _updateBinding(binding, v),
              ),
            ),
            if (showInput) ...[
              const SizedBox(width: 8),
              SizedBox(
                width: 88,
                child: AmitiaTextField(
                  key: ValueKey('${node.id}:slider:$value'),
                  controller: TextEditingController(text: value.toStringAsFixed(rawStep > 0 && rawStep < 1 ? 2 : 0)),
                  readOnly: disabled,
                  keyboardType: const TextInputType.numberWithOptions(decimal: true, signed: true),
                  onChanged: disabled
                      ? null
                      : (text) {
                          final next = double.tryParse(text);
                          if (next != null) _updateBinding(binding, next.clamp(min, max));
                        },
                ),
              ),
            ],
          ],
        ),
      ],
    );
  }

  Widget _buildButton(BuildContext context, SchemaUINode node) {
    final label = (node.props?['text'] ?? node.props?['label'] ?? '按钮').toString();
    final kind = (node.props?['variant'] ?? node.props?['type'] ?? '').toString().trim().toLowerCase();
    final size = node.props?['size']?.toString().trim().toLowerCase() ?? 'default';
    final loading = node.props?['loading'] == true;
    final disabled = _isNodeDisabled(node) || loading || node.actions.isEmpty;
    final height = switch (size) {
      'small' => 32.0,
      'large' => 48.0,
      _ => AppSpacing.buttonHeight,
    };
    return AmitiaButton(
      label: loading ? '加载中…' : label,
      height: height,
      isSecondary: kind == 'secondary' || kind == 'default' || kind == 'info',
      isDestructive: kind == 'danger' || kind == 'error',
      outlined: node.props?['plain'] == true,
      round: node.props?['round'] == true,
      onPressed: disabled ? null : () => _handleActions(node.actions, nodeId: node.id),
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

  Widget _buildList(BuildContext context, SchemaUINode node, int depth) {
    final items = node.props?['items'] as List? ?? const [];
    if (items.isNotEmpty) {
      return Column(
        children: items
            .map((item) => ListTile(
                  title: Text(item.toString(), style: AppTypography.bodySmall(context)),
                  contentPadding: EdgeInsets.zero,
                ))
            .toList(),
      );
    }
    return Column(
      children: node.children
          .map((child) => Padding(
                padding: EdgeInsets.only(bottom: AppSpacing.sm),
                child: _buildNode(context, child, depth + 1),
              ))
          .toList(),
    );
  }

  Widget _buildTable(BuildContext context, SchemaUINode node) {
    final headers = <String>[];
    final keys = <String>[];
    final columns = node.props?['columns'];
    if (columns is List) {
      for (final column in columns) {
        if (column is Map) {
          final key = (column['prop'] ?? column['key'] ?? column['field'] ?? '').toString();
          if (key.isEmpty) continue;
          keys.add(key);
          headers.add((column['label'] ?? column['title'] ?? key).toString());
        } else {
          final key = column.toString();
          keys.add(key);
          headers.add(key);
        }
      }
    } else {
      final legacyHeaders = node.props?['headers'];
      if (legacyHeaders is List) {
        for (final header in legacyHeaders) {
          keys.add(header.toString());
          headers.add(header.toString());
        }
      }
    }
    final rawRows = node.props?['data'] ?? node.props?['items'] ?? node.props?['rows'];
    final rows = <List<String>>[];
    if (rawRows is List) {
      for (final row in rawRows) {
        if (row is Map) {
          rows.add(keys.map((key) => row[key]?.toString() ?? '').toList());
        } else if (row is List) {
          rows.add(row.map((cell) => cell?.toString() ?? '').toList());
        } else {
          rows.add(<String>[row.toString()]);
        }
      }
    }
    if (headers.isEmpty && rawRows is List) {
      for (final row in rawRows) {
        if (row is Map) {
          for (final key in row.keys.map((item) => item.toString())) {
            if (!keys.contains(key)) {
              keys.add(key);
              headers.add(key);
            }
          }
        }
      }
      rows.clear();
      for (final row in rawRows) {
        if (row is Map) rows.add(keys.map((key) => row[key]?.toString() ?? '').toList());
      }
    }
    if (headers.isEmpty) return const SizedBox.shrink();

    final bordered = node.props?['bordered'] == true;
    final striped = node.props?['stripe'] != false;
    final maxHeight = _dimension(node.props?['maxHeight'], 0);
    final divider = Theme.of(context).dividerColor;
    final table = DataTable(
      border: bordered ? TableBorder.all(color: divider) : null,
      columns: headers.map((h) => DataColumn(label: Text(h, style: AppTypography.label(context)))).toList(),
      rows: rows
          .asMap()
          .entries
          .map((entry) => DataRow(
                color: striped && entry.key.isOdd
                    ? WidgetStatePropertyAll(context.surfaceSecondary.withValues(alpha: 0.55))
                    : null,
                cells: List.generate(
                  headers.length,
                  (index) => DataCell(
                    Text(
                      index < entry.value.length ? entry.value[index] : '',
                      style: AppTypography.bodySmall(context),
                    ),
                  ),
                ),
              ))
          .toList(),
    );
    final horizontal = SingleChildScrollView(scrollDirection: Axis.horizontal, child: table);
    if (maxHeight > 0) {
      return SizedBox(
        height: maxHeight,
        child: SingleChildScrollView(child: horizontal),
      );
    }
    return horizontal;
  }

  Widget _buildNodeEmptyState(BuildContext context, SchemaUINode node) {
    final iconName = node.props?['icon']?.toString() ?? 'inbox_outlined';
    final title = (node.props?['title'] ?? node.props?['description'] ?? node.props?['text'] ?? '暂无数据').toString();
    final subtitle = (node.props?['subtitle'])?.toString();
    final imageSize = _dimension(node.props?['imageSize'], 60).clamp(24.0, 160.0).toDouble();
    return AmitiaEmptyState(
      icon: _mapIconData(iconName),
      title: title,
      subtitle: subtitle,
      iconSize: imageSize,
    );
  }

  Widget _buildAlert(BuildContext context, SchemaUINode node) {
    if (_dismissedNodeIds.contains(node.id)) return const SizedBox.shrink();
    final title = node.props?['title']?.toString().trim() ?? '';
    final text = (node.props?['text'] ?? node.props?['message'] ?? node.props?['description'] ?? node.props?['detail'] ?? '').toString();
    final variant = (node.props?['variant'] ?? node.props?['type'] ?? 'info').toString();
    final color = _alertColor(context, variant);
    final showIcon = node.props?['showIcon'] != false;
    final closable = node.props?['closable'] != false;
    return Container(
      padding: EdgeInsets.all(AppSpacing.md),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.08),
        borderRadius: AppRadius.brMedium,
        border: Border.all(color: color.withValues(alpha: 0.3)),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (showIcon) ...[
            Icon(Icons.info_outline, color: color, size: 18),
            const SizedBox(width: 8),
          ],
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (title.isNotEmpty) Text(title, style: AppTypography.label(context)),
                if (title.isNotEmpty && text.isNotEmpty) const SizedBox(height: 2),
                if (text.isNotEmpty) Text(text, style: AppTypography.bodySmall(context)),
              ],
            ),
          ),
          if (closable)
            IconButton(
              visualDensity: VisualDensity.compact,
              tooltip: '关闭',
              onPressed: () => setState(() => _dismissedNodeIds.add(node.id)),
              icon: const Icon(Icons.close, size: 16),
            ),
        ],
      ),
    );
  }

  Color _alertColor(BuildContext context, String variant) {
    switch (variant) {
      case 'success':
        return context.success;
      case 'warning':
        return context.warning;
      case 'error':
      case 'danger':
        return context.error;
      default:
        return context.info;
    }
  }

  Widget _buildProgress(BuildContext context, SchemaUINode node) {
    final binding = node.bindings.isNotEmpty ? node.bindings.first : null;
    final raw = _resolvedNodeValue(node, binding) ?? node.props?['percentage'] ?? 0;
    var percentage = (raw is num ? raw.toDouble() : double.tryParse(raw.toString()) ?? 0).clamp(0.0, 100.0).toDouble();
    if (percentage <= 1 && (raw is num && raw.toDouble() <= 1)) percentage *= 100;
    final progress = (percentage / 100).clamp(0.0, 1.0).toDouble();
    final variant = node.props?['variant']?.toString().trim().toLowerCase() ?? 'line';
    final status = node.props?['status']?.toString().trim().toLowerCase() ?? '';
    final showText = node.props?['showText'] != false;
    final strokeWidth = _dimension(node.props?['strokeWidth'], 6).clamp(2.0, 24.0).toDouble();
    final color = switch (status) {
      'success' => context.success,
      'warning' => context.warning,
      'exception' || 'error' => context.error,
      _ => context.accentPrimary,
    };
    if (variant == 'circle' || variant == 'dashboard') {
      return SizedBox(
        width: 72,
        height: 72,
        child: Stack(
          alignment: Alignment.center,
          children: [
            SizedBox(
              width: 64,
              height: 64,
              child: CircularProgressIndicator(
                value: progress,
                strokeWidth: strokeWidth,
                color: color,
                backgroundColor: context.accentSoft,
              ),
            ),
            if (showText) Text('${percentage.round()}%', style: AppTypography.caption(context)),
          ],
        ),
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.end,
      children: [
        AmitiaProgressBar(progress: progress, height: strokeWidth, color: color),
        if (showText) ...[
          const SizedBox(height: 4),
          Text('${percentage.round()}%', style: AppTypography.caption(context)),
        ],
      ],
    );
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
    final disabled = node.props?['disabled'] == true;
    final underline = node.props?['underline'] != false;
    final canActivate = !disabled && (node.actions.isNotEmpty || href.isNotEmpty);
    return InkWell(
      borderRadius: AppRadius.brSmall,
      onTap: !canActivate
          ? null
          : () async {
              if (node.actions.isNotEmpty) {
                await _handleActions(node.actions, nodeId: node.id);
                return;
              }
              await _openResourceHref(href);
            },
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 4),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.open_in_new,
              size: 16,
              color: canActivate ? context.accentPrimary : context.textTertiary,
            ),
            const SizedBox(width: 6),
            Flexible(
              child: Text(
                text,
                style: AppTypography.bodySmall(context).copyWith(
                  color: canActivate ? context.accentPrimary : context.textTertiary,
                  decoration: canActivate && underline ? TextDecoration.underline : TextDecoration.none,
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
    final raw = node.props?['items'] ?? node.props?['data'];
    final entries = <MapEntry<String, String>>[];
    if (raw is Map) {
      for (final entry in raw.entries) {
        entries.add(MapEntry(entry.key.toString(), entry.value?.toString() ?? ''));
      }
    } else if (raw is List) {
      for (final item in raw) {
        if (item is Map) {
          final key = (item['key'] ?? item['label'] ?? item['name'] ?? '').toString();
          final value = (item['value'] ?? item['content'] ?? '').toString();
          entries.add(MapEntry(key, value));
        } else {
          entries.add(MapEntry(item.toString(), ''));
        }
      }
    }
    final title = node.props?['title']?.toString().trim() ?? '';
    final rawColumns = node.props?['columns'];
    final columns = (rawColumns is num ? rawColumns.toInt() : int.tryParse(rawColumns?.toString() ?? '') ?? 1)
        .clamp(1, 6)
        .toInt();
    final bordered = node.props?['bordered'] != false;

    Widget item(MapEntry<String, String> entry) {
      return Container(
        padding: EdgeInsets.symmetric(horizontal: AppSpacing.sm, vertical: AppSpacing.tightGap),
        decoration: bordered
            ? BoxDecoration(
                border: Border.all(color: context.borderPrimary),
                borderRadius: AppRadius.brSmall,
              )
            : null,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(child: Text(entry.key, style: AppTypography.label(context))),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                entry.value,
                style: AppTypography.bodySmall(context),
                textAlign: TextAlign.end,
              ),
            ),
          ],
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (title.isNotEmpty) ...[
          Text(title, style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.sm),
        ],
        LayoutBuilder(
          builder: (context, constraints) {
            final gap = AppSpacing.tightGap;
            final width = constraints.maxWidth.isFinite
                ? ((constraints.maxWidth - gap * (columns - 1)) / columns).clamp(0.0, double.infinity).toDouble()
                : null;
            return Wrap(
              spacing: gap,
              runSpacing: gap,
              children: entries
                  .map((entry) => width == null ? item(entry) : SizedBox(width: width, child: item(entry)))
                  .toList(),
            );
          },
        ),
      ],
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

class _SchemaSelectOption {
  final dynamic value;
  final String label;

  const _SchemaSelectOption({required this.value, required this.label});
}

class _SchemaTabsView extends StatefulWidget {
  final List<SchemaUINode> tabs;
  final bool Function(SchemaUINode tab) isDisabled;
  final Widget Function(SchemaUINode tab) contentBuilder;
  final String position;
  final String variant;
  final double? minHeight;

  const _SchemaTabsView({
    required this.tabs,
    required this.isDisabled,
    required this.contentBuilder,
    required this.position,
    required this.variant,
    this.minHeight,
  });

  @override
  State<_SchemaTabsView> createState() => _SchemaTabsViewState();
}

class _SchemaTabsViewState extends State<_SchemaTabsView> {
  int _activeIndex = 0;

  @override
  void initState() {
    super.initState();
    _activeIndex = _firstEnabledIndex();
  }

  @override
  void didUpdateWidget(covariant _SchemaTabsView oldWidget) {
    super.didUpdateWidget(oldWidget);
    final activeId = oldWidget.tabs.isNotEmpty && _activeIndex < oldWidget.tabs.length
        ? oldWidget.tabs[_activeIndex].id
        : '';
    final preserved = widget.tabs.indexWhere((tab) => tab.id == activeId && !widget.isDisabled(tab));
    if (preserved >= 0) {
      _activeIndex = preserved;
      return;
    }
    _activeIndex = _firstEnabledIndex();
  }

  int _firstEnabledIndex() {
    final index = widget.tabs.indexWhere((tab) => !widget.isDisabled(tab));
    return index >= 0 ? index : 0;
  }

  void _activate(int index) {
    if (index < 0 || index >= widget.tabs.length || widget.isDisabled(widget.tabs[index])) return;
    if (_activeIndex != index) setState(() => _activeIndex = index);
  }

  Widget _tabButton(BuildContext context, int index, {required bool vertical}) {
    final tab = widget.tabs[index];
    final disabled = widget.isDisabled(tab);
    final selected = index == _activeIndex;
    final label = (tab.props?['label'] ?? tab.props?['title'] ?? tab.id).toString();
    final variant = widget.variant.trim().toLowerCase();
    final scheme = Theme.of(context).colorScheme;
    final border = variant == 'card' || variant == 'border-card'
        ? Border.all(color: selected ? scheme.primary : Theme.of(context).dividerColor)
        : Border(
            bottom: BorderSide(
              color: selected ? scheme.primary : Colors.transparent,
              width: selected ? 2 : 0,
            ),
          );
    return Semantics(
      button: true,
      selected: selected,
      enabled: !disabled,
      child: InkWell(
        onTap: disabled ? null : () => _activate(index),
        borderRadius: BorderRadius.circular(8),
        child: Container(
          constraints: vertical ? const BoxConstraints(minWidth: 112) : null,
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
          decoration: BoxDecoration(
            border: border,
            borderRadius: variant == 'line' ? null : BorderRadius.circular(8),
            color: selected && variant != 'line' ? scheme.primaryContainer.withValues(alpha: 0.35) : null,
          ),
          child: Text(
            label,
            textAlign: vertical ? TextAlign.start : TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: disabled
                      ? Theme.of(context).disabledColor
                      : selected
                          ? scheme.primary
                          : null,
                  fontWeight: selected ? FontWeight.w600 : FontWeight.w400,
                ),
          ),
        ),
      ),
    );
  }

  Widget _horizontalHeader(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: List<Widget>.generate(
          widget.tabs.length,
          (index) => Padding(
            padding: const EdgeInsets.only(right: 4),
            child: _tabButton(context, index, vertical: false),
          ),
        ),
      ),
    );
  }

  Widget _verticalHeader(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: List<Widget>.generate(
        widget.tabs.length,
        (index) => Padding(
          padding: const EdgeInsets.only(bottom: 4),
          child: _tabButton(context, index, vertical: true),
        ),
      ),
    );
  }

  Widget _content() {
    if (widget.tabs.isEmpty) return const SizedBox.shrink();
    final active = _activeIndex.clamp(0, widget.tabs.length - 1).toInt();
    Widget child = widget.contentBuilder(widget.tabs[active]);
    if (widget.minHeight != null && widget.minHeight! > 0) {
      child = ConstrainedBox(
        constraints: BoxConstraints(minHeight: widget.minHeight!),
        child: child,
      );
    }
    return child;
  }

  @override
  Widget build(BuildContext context) {
    if (widget.tabs.isEmpty) return const SizedBox.shrink();
    final position = widget.position.trim().toLowerCase();
    final content = _content();
    if (position == 'left' || position == 'right') {
      final header = _verticalHeader(context);
      final children = position == 'right'
          ? <Widget>[
              Expanded(child: content),
              const SizedBox(width: 8),
              SizedBox(width: 128, child: header),
            ]
          : <Widget>[
              SizedBox(width: 128, child: header),
              const SizedBox(width: 8),
              Expanded(child: content),
            ];
      return Row(crossAxisAlignment: CrossAxisAlignment.start, children: children);
    }
    final header = _horizontalHeader(context);
    if (position == 'bottom') {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [content, const SizedBox(height: 8), header],
      );
    }
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [header, const SizedBox(height: 8), content],
    );
  }
}

class SchemaUIThemeResolver extends StatelessWidget {
  final ThemeConfig? theme;
  final Widget child;

  const SchemaUIThemeResolver({super.key, this.theme, required this.child});

  Color? _parseColor(String? raw) {
    var value = raw?.trim() ?? '';
    if (!value.startsWith('#')) return null;
    value = value.substring(1);
    if (value.length == 3) value = value.split('').map((c) => '$c$c').join();
    if (value.length == 6) value = 'FF$value';
    if (value.length != 8) return null;
    final parsed = int.tryParse(value, radix: 16);
    return parsed == null ? null : Color(parsed);
  }

  double? _parseDimension(String? raw) {
    final value = raw?.trim().replaceAll(RegExp(r'px$'), '') ?? '';
    return double.tryParse(value);
  }

  @override
  Widget build(BuildContext context) {
    if (theme == null) return child;
    final base = Theme.of(context);
    final brightness = switch (theme!.mode) {
      'light' => Brightness.light,
      'dark' => Brightness.dark,
      _ => base.brightness,
    };
    var colors = brightness == base.brightness
        ? (base.extension<AmitiaColorTokens>() ??
            (brightness == Brightness.dark ? defaultDarkColorTokens() : defaultLightColorTokens()))
        : (brightness == Brightness.dark ? defaultDarkColorTokens() : defaultLightColorTokens());
    var layout = base.extension<AmitiaLayoutTokens>() ?? const AmitiaLayoutTokens();
    final overrides = theme!.overrides ?? const <String, String>{};
    Color? color(String key) => _parseColor(overrides[key]);
    colors = colors.copyWith(
      backgroundPrimary: color('--amitia-bg-primary') ?? color('--amitia-color-background'),
      backgroundSecondary: color('--amitia-bg-secondary'),
      surfacePrimary: color('--amitia-bg-surface') ?? color('--amitia-color-surface'),
      surfaceSecondary: color('--amitia-bg-surface-secondary'),
      accentPrimary: color('--amitia-color-accent') ?? color('--amitia-color-primary'),
      textPrimary: color('--amitia-text-primary') ?? color('--amitia-color-text'),
      textSecondary: color('--amitia-text-secondary') ?? color('--amitia-color-text-secondary'),
      borderPrimary: color('--amitia-border') ?? color('--amitia-color-border'),
      success: color('--amitia-color-success'),
      warning: color('--amitia-color-warning'),
      error: color('--amitia-color-danger') ?? color('--amitia-color-error'),
      info: color('--amitia-color-info'),
    );
    layout = layout.copyWith(
      radiusSmall: _parseDimension(overrides['--amitia-radius-sm']),
      radiusMedium: _parseDimension(overrides['--amitia-radius-md']),
      radiusLarge: _parseDimension(overrides['--amitia-radius-lg']),
    );
    final extensions = <ThemeExtension<dynamic>>[
      ...base.extensions.values.where((value) => value is! AmitiaColorTokens && value is! AmitiaLayoutTokens),
      colors,
      layout,
    ];
    final scheme = base.colorScheme.copyWith(
      brightness: brightness,
      primary: colors.accentPrimary,
      surface: colors.surfacePrimary,
      error: colors.error,
      onSurface: colors.textPrimary,
    );
    return Theme(
      data: base.copyWith(brightness: brightness, colorScheme: scheme, extensions: extensions),
      child: child,
    );
  }
}

