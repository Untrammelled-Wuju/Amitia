import '../models/schema_ui_types.dart';

class BindingContext {
  final Map<String, dynamic> input;
  final Map<String, dynamic> formState;
  final Map<String, dynamic> localState;
  final Map<String, dynamic> query;
  final Map<String, dynamic> runtime;
  final Map<String, dynamic> host;
  final Map<String, dynamic> storage;
  final Map<String, dynamic> runtimeStatus;
  final Map<String, dynamic> resourceList;
  final Map<String, dynamic> dataSources;
  final Map<String, dynamic> flat;

  const BindingContext({
    this.input = const {},
    this.formState = const {},
    this.localState = const {},
    this.query = const {},
    this.runtime = const {},
    this.host = const {},
    this.storage = const {},
    this.runtimeStatus = const {},
    this.resourceList = const {},
    this.dataSources = const {},
    this.flat = const {},
  });

  BindingContext copyWith({
    Map<String, dynamic>? input,
    Map<String, dynamic>? formState,
    Map<String, dynamic>? localState,
    Map<String, dynamic>? query,
    Map<String, dynamic>? runtime,
    Map<String, dynamic>? host,
    Map<String, dynamic>? storage,
    Map<String, dynamic>? runtimeStatus,
    Map<String, dynamic>? resourceList,
    Map<String, dynamic>? dataSources,
    Map<String, dynamic>? flat,
  }) {
    return BindingContext(
      input: input ?? this.input,
      formState: formState ?? this.formState,
      localState: localState ?? this.localState,
      query: query ?? this.query,
      runtime: runtime ?? this.runtime,
      host: host ?? this.host,
      storage: storage ?? this.storage,
      runtimeStatus: runtimeStatus ?? this.runtimeStatus,
      resourceList: resourceList ?? this.resourceList,
      dataSources: dataSources ?? this.dataSources,
      flat: flat ?? this.flat,
    );
  }
}

dynamic _lookupPath(dynamic data, String path) {
  if (path.isEmpty || data == null) return null;
  final parts = path.split('.');
  dynamic current = data;
  for (final p in parts) {
    if (current == null) return null;
    if (current is List) {
      final idx = int.tryParse(p);
      if (idx == null || idx < 0 || idx >= current.length) return null;
      current = current[idx];
    } else if (current is Map<String, dynamic>) {
      current = current[p];
    } else if (current is Map) {
      current = current[p];
    } else {
      return null;
    }
  }
  return current;
}

bool evaluateCondition(dynamic value, String op, dynamic expected) {
  switch (op) {
    case '==':
    case 'eq':
      return value == expected;
    case '!=':
    case 'ne':
      return value != expected;
    case '>':
    case 'gt':
      if (value is num && expected is num) return value > expected;
      if (value is String && expected is String) return value.compareTo(expected) > 0;
      return false;
    case '<':
    case 'lt':
      if (value is num && expected is num) return value < expected;
      if (value is String && expected is String) return value.compareTo(expected) < 0;
      return false;
    case '>=':
    case 'gte':
      if (value is num && expected is num) return value >= expected;
      if (value is String && expected is String) return value.compareTo(expected) >= 0;
      return false;
    case '<=':
    case 'lte':
      if (value is num && expected is num) return value <= expected;
      if (value is String && expected is String) return value.compareTo(expected) <= 0;
      return false;
    case 'in':
      if (expected is List) return expected.contains(value);
      if (expected is String && value is String) {
        return expected.split(',').map((s) => s.trim()).contains(value);
      }
      return false;
    case 'not_in':
      if (expected is List) return !expected.contains(value);
      if (expected is String && value is String) {
        return !expected.split(',').map((s) => s.trim()).contains(value);
      }
      return true;
    case 'contains':
      if (value is String && expected is String) return value.contains(expected);
      if (value is List) return value.contains(expected);
      return false;
    case 'regex':
      if (value is String && expected is String) {
        try {
          return RegExp(expected).hasMatch(value);
        } catch (_) {
          return false;
        }
      }
      return false;
    case 'not_null':
      return value != null;
    case 'is_null':
      return value == null;
    default:
      return false;
  }
}

bool evaluateVisibility(List<UICondition>? conditions, BindingContext context) {
  if (conditions == null || conditions.isEmpty) return true;
  for (final condition in conditions) {
    final value = _lookupPath(context.flat, condition.field) ??
        _lookupPath(context.localState, condition.field) ??
        _lookupPath(context.formState, condition.field) ??
        _lookupPath(context.input, condition.field) ??
        _lookupPath(context.query, condition.field) ??
        _lookupPath(context.runtime, condition.field) ??
        _lookupPath(context.host, condition.field) ??
        _lookupPath(context.dataSources, condition.field) ??
        _lookupPath(context.storage, condition.field);
    if (!evaluateCondition(value, condition.operator, condition.value)) return false;
  }
  return true;
}

class BindingEngine {
  const BindingEngine();

  dynamic resolveBinding(SchemaUIBinding? binding, BindingContext context) {
    if (binding == null) return null;
    final source = binding.source;
    final path = binding.path;
    dynamic resolved;

    switch (source) {
      case 'static':
        resolved = binding.defaultValue;
        break;
      case 'form_state':
      case 'form':
        resolved = _lookupPath(context.formState, path);
        break;
      case 'state':
        resolved = _lookupPath(context.localState, path);
        break;
      case 'input':
        resolved = _lookupPath(context.input, path);
        break;
      case 'query':
        resolved = _lookupPath(context.query, path);
        break;
      case 'runtime':
        resolved = _lookupPath(context.runtime, path);
        break;
      case 'host':
        resolved = _lookupPath(context.host, path);
        break;
      case 'storage':
        resolved = context.storage[path] ?? _lookupPath(context.storage, path);
        break;
      case 'runtime_status':
        resolved = _lookupPath(context.runtimeStatus, path);
        break;
      case 'resource_list':
        resolved = _lookupPath(context.resourceList, path);
        break;
      default:
        resolved = null;
    }

    if (resolved == null && {'input', 'form_state', 'form', 'state', 'query'}.contains(source)) {
      resolved = _lookupPath(context.flat, path);
    }
    if (resolved == null && binding.defaultValue != null) return binding.defaultValue;
    return resolved;
  }
}
