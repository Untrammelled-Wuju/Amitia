import 'dart:convert';

const _runtimeErrorFields = <String>[
  'provider',
  'errorType',
  'errorCode',
  'stage',
  'toolName',
  'userMessage',
  'internalMessage',
  'message',
  'error',
  'detail',
  'cause',
  'rawError',
  'stackTrace',
  'stack',
];

String runtimeErrorValueText(dynamic value) {
  if (value == null) return '';
  if (value is String) return value;
  try {
    return const JsonEncoder.withIndent('  ').convert(value);
  } catch (error, stackTrace) {
    return '$error\n$stackTrace\n${value.toString()}';
  }
}

String formatRuntimeErrorDetail(
  Map<String, dynamic>? payload, {
  String fallback = '',
}) {
  final values = <String>[];
  if (payload != null) {
    for (final field in _runtimeErrorFields) {
      final text = runtimeErrorValueText(payload[field]);
      if (text.trim().isEmpty) continue;
      values.add('$field: $text');
    }
  }
  if (values.isEmpty && fallback.trim().isNotEmpty) {
    return fallback;
  }
  if (payload != null && payload.isNotEmpty) {
    try {
      values.add(
        'originalPayload:\n${const JsonEncoder.withIndent('  ').convert(payload)}',
      );
    } catch (error, stackTrace) {
      values.add('originalPayload:\n$error\n$stackTrace\n$payload');
    }
  }
  return values.join('\n\n');
}
