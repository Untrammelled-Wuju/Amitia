import 'package:amitia_app/features/chat/runtime/runtime_error_detail.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('运行时错误格式化保留全部原始字段和载荷', () {
    final detail = formatRuntimeErrorDetail({
      'errorCode': 'task_execution_failed',
      'userMessage': 'tool execution cancelled: device_authority_expired',
      'internalMessage': 'tool execution cancelled: device_authority_expired',
      'error': 'tool execution cancelled: device_authority_expired',
      'detail': 'wrapped detail',
      'cause': 'device_authority_expired',
      'rawError': 'tool execution cancelled: device_authority_expired',
      'stack': 'stack-line',
    });

    expect(detail, contains('errorCode: task_execution_failed'));
    expect(detail, contains('cause: device_authority_expired'));
    expect(detail, contains('stack: stack-line'));
    expect(detail, contains('originalPayload:'));
    expect(detail, contains('"rawError"'));
  });

  test('缺少结构化错误时保留原始回退文本', () {
    expect(formatRuntimeErrorDetail(null, fallback: '原始连接异常'), '原始连接异常');
  });
}
