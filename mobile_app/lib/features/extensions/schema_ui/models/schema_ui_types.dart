import 'dart:convert';

typedef SchemaUINodeType = String;

class SchemaUI {
  static const String nodePage = 'page';
  static const String nodeSection = 'section';
  static const String nodeStack = 'stack';
  static const String nodeRow = 'row';
  static const String nodeGrid = 'grid';
  static const String nodeTabs = 'tabs';
  static const String nodeCard = 'card';
  static const String nodeText = 'text';
  static const String nodeMarkdown = 'markdown';
  static const String nodeBadge = 'badge';
  static const String nodeDivider = 'divider';
  static const String nodeIcon = 'icon';
  static const String nodeImage = 'image';
  static const String nodeField = 'field';
  static const String nodeSelect = 'select';
  static const String nodeSwitch = 'switch';
  static const String nodeSlider = 'slider';
  static const String nodeButton = 'button';
  static const String nodeButtonGroup = 'button_group';
  static const String nodeList = 'list';
  static const String nodeTable = 'table';
  static const String nodeEmptyState = 'empty_state';
  static const String nodeAlert = 'alert';
  static const String nodeProgress = 'progress';
  static const String nodeCode = 'code';
  static const String nodeKeyValue = 'key_value';
  static const String nodeResourceLink = 'resource_link';
  static const String nodePermissionSummary = 'permission_summary';
  static const String nodeRuntimeStatus = 'runtime_status';
  static const String nodeTabItem = 'tab_item';
  static const String nodeColumn = 'column';

  static const Set<String> allowedNodeTypes = {
    nodePage, nodeSection, nodeStack, nodeRow, nodeGrid, nodeTabs, nodeCard,
    nodeText, nodeMarkdown, nodeBadge, nodeDivider, nodeIcon, nodeImage,
    nodeField, nodeSelect, nodeSwitch, nodeSlider, nodeButton, nodeButtonGroup,
    nodeList, nodeTable, nodeEmptyState, nodeAlert, nodeProgress, nodeCode,
    nodeKeyValue, nodeResourceLink, nodePermissionSummary, nodeRuntimeStatus,
    nodeTabItem, nodeColumn,
  };

  static const Set<String> forbiddenNodeTypes = {
    'html', 'script', 'style', 'iframe', 'webview', 'canvas', 'template',
  };

  static bool isAllowed(String? type) {
    if (type == null) return false;
    return allowedNodeTypes.contains(type) && !forbiddenNodeTypes.contains(type);
  }
}

class UICondition {
  final String field;
  final String operator;
  final dynamic value;

  const UICondition({required this.field, required this.operator, this.value});

  factory UICondition.fromJson(Map<String, dynamic> json) {
    return UICondition(
      field: json['field'] as String? ?? '',
      operator: json['operator'] as String? ?? '==',
      value: json['value'],
    );
  }
}

class SchemaUIBinding {
  final String path;
  final String source;
  final String? format;
  final dynamic defaultValue;

  const SchemaUIBinding({
    required this.path,
    required this.source,
    this.format,
    this.defaultValue,
  });

  factory SchemaUIBinding.fromJson(Map<String, dynamic> json) {
    return SchemaUIBinding(
      path: json['path'] as String? ?? '',
      source: json['source'] as String? ?? '',
      format: json['format'] as String?,
      defaultValue: json['default'],
    );
  }
}

class SchemaUIActionBinding {
  final String actionId;
  final String target;
  final Map<String, dynamic>? input;
  final String? confirmation;

  const SchemaUIActionBinding({
    required this.actionId,
    required this.target,
    this.input,
    this.confirmation,
  });

  factory SchemaUIActionBinding.fromJson(Map<String, dynamic> json) {
    return SchemaUIActionBinding(
      actionId: json['action_id'] as String? ?? '',
      target: json['target'] as String? ?? '',
      input: json['input'] as Map<String, dynamic>?,
      confirmation: json['confirmation'] as String?,
    );
  }
}

class SchemaUINode {
  final String id;
  final String type;
  final Map<String, dynamic>? props;
  final List<SchemaUIBinding> bindings;
  final List<SchemaUIActionBinding> actions;
  final List<UICondition> visibility;
  final List<SchemaUINode> children;

  const SchemaUINode({
    required this.id,
    required this.type,
    this.props,
    this.bindings = const [],
    this.actions = const [],
    this.visibility = const [],
    this.children = const [],
  });

  factory SchemaUINode.fromJson(Map<String, dynamic> json) {
    return SchemaUINode(
      id: json['id'] as String? ?? '',
      type: json['type'] as String? ?? '',
      props: _parseProps(json['props']),
      bindings: _parseList(json['bindings'], SchemaUIBinding.fromJson),
      actions: _parseList(json['actions'], SchemaUIActionBinding.fromJson),
      visibility: _parseList(json['visibility'], UICondition.fromJson),
      children: _parseList(json['children'], SchemaUINode.fromJson),
    );
  }

  static Map<String, dynamic>? _parseProps(dynamic raw) {
    if (raw == null) return null;
    if (raw is Map<String, dynamic>) return raw;
    if (raw is String && raw.isNotEmpty) {
      try {
        final decoded = jsonDecode(raw);
        if (decoded is Map<String, dynamic>) return decoded;
      } catch (_) {}
    }
    return null;
  }

  static List<T> _parseList<T>(dynamic raw, T Function(Map<String, dynamic>) fromJson) {
    if (raw is! List) return [];
    final result = <T>[];
    for (final item in raw) {
      if (item is Map<String, dynamic>) {
        result.add(fromJson(item));
      }
    }
    return result;
  }
}

class ThemeConfig {
  final String mode;
  final Map<String, String>? overrides;

  const ThemeConfig({this.mode = 'auto', this.overrides});

  factory ThemeConfig.fromJson(Map<String, dynamic> json) {
    return ThemeConfig(
      mode: json['mode'] as String? ?? 'auto',
      overrides: (json['overrides'] as Map<String, dynamic>?)?.map(
        (k, v) => MapEntry(k, v.toString()),
      ),
    );
  }
}

class SchemaUIDataSource {
  final String id;
  final String type;
  final Map<String, dynamic>? inputSchema;
  final Map<String, dynamic>? outputSchema;
  final String? refreshPolicy;
  final String? runtimeEntry;

  const SchemaUIDataSource({
    required this.id,
    required this.type,
    this.inputSchema,
    this.outputSchema,
    this.refreshPolicy,
    this.runtimeEntry,
  });

  factory SchemaUIDataSource.fromJson(Map<String, dynamic> json) {
    return SchemaUIDataSource(
      id: json['id'] as String? ?? '',
      type: json['type'] as String? ?? '',
      inputSchema: json['inputSchema'] as Map<String, dynamic>?,
      outputSchema: json['outputSchema'] as Map<String, dynamic>?,
      refreshPolicy: json['refreshPolicy'] as String?,
      runtimeEntry: json['runtimeEntry'] as String?,
    );
  }
}

class SchemaUIDeclaredAction {
  final String actionId;
  final String target;
  final Map<String, dynamic>? inputSchema;

  const SchemaUIDeclaredAction({
    required this.actionId,
    required this.target,
    this.inputSchema,
  });

  factory SchemaUIDeclaredAction.fromJson(Map<String, dynamic> json) {
    return SchemaUIDeclaredAction(
      actionId: json['actionId'] as String? ?? '',
      target: json['target'] as String? ?? '',
      inputSchema: json['inputSchema'] as Map<String, dynamic>?,
    );
  }
}

class SchemaUIDocument {
  final String schemaVersion;
  final String type;
  final String? title;
  final Map<String, dynamic>? layout;
  final List<SchemaUINode> children;
  final List<SchemaUIDataSource> dataSources;
  final List<SchemaUIDeclaredAction> actions;
  final ThemeConfig? theme;

  const SchemaUIDocument({
    this.schemaVersion = 'schema-ui/1',
    this.type = 'document',
    this.title,
    this.layout,
    this.children = const [],
    this.dataSources = const [],
    this.actions = const [],
    this.theme,
  });

  factory SchemaUIDocument.fromJson(Map<String, dynamic> json) {
    return SchemaUIDocument(
      schemaVersion: json['schemaVersion'] as String? ?? 'schema-ui/1',
      type: json['type'] as String? ?? 'document',
      title: json['title'] as String?,
      layout: json['layout'] as Map<String, dynamic>?,
      children: SchemaUINode._parseList(json['children'], SchemaUINode.fromJson),
      dataSources: SchemaUINode._parseList(json['dataSources'], SchemaUIDataSource.fromJson),
      actions: SchemaUINode._parseList(json['actions'], SchemaUIDeclaredAction.fromJson),
      theme: json['theme'] != null ? ThemeConfig.fromJson(json['theme'] as Map<String, dynamic>) : null,
    );
  }

  static SchemaUIDocument? fromJsonString(String? raw) {
    if (raw == null || raw.isEmpty) return null;
    try {
      final decoded = jsonDecode(raw);
      if (decoded is Map<String, dynamic>) {
        return SchemaUIDocument.fromJson(decoded);
      }
    } catch (_) {}
    return null;
  }
}
