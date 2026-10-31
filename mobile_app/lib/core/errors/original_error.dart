import 'package:dio/dio.dart';

import '../backend_transport/errors/backend_transport_error.dart';

String originalErrorMapText(Map<dynamic, dynamic> value) {
  final messages = <String>[];

  void append(Object? candidate) {
    if (candidate is Map) {
      final nested = originalErrorMapText(candidate);
      if (nested.isNotEmpty && !messages.contains(nested)) messages.add(nested);
      return;
    }
    if (candidate is String) {
      final text = candidate.trim();
      if (text.isNotEmpty && !messages.contains(text)) messages.add(text);
    }
  }

  for (final key in const [
    'message',
    'msg',
    'error',
    'detail',
    'internalMessage',
    'title',
    'reason',
    'cause',
  ]) {
    append(value[key]);
  }
  if (messages.isNotEmpty) return messages.join('\n');
  return value.toString();
}

String originalErrorText(Object error) {
  if (error is BackendTransportError) {
    final cause = error.cause;
    if (cause != null) return originalErrorText(cause);
  }
  if (error is DioException) {
    final data = error.response?.data;
    if (data is Map) {
      return originalErrorMapText(data);
    }
    final message = error.message;
    if (message != null && message.trim().isNotEmpty) return message;
  }
  if (error is Map) {
    return originalErrorMapText(error);
  }
  return error.toString();
}

void reportOriginalError(
  Object error,
  StackTrace stackTrace, {
  String source = 'client',
}) {
  print('[original-error][$source] ${originalErrorText(error)}\n$stackTrace');
}
