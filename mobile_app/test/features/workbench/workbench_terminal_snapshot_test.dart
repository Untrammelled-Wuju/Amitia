import 'dart:convert';
import 'package:flutter_test/flutter_test.dart';
import '../../../lib/features/workbench/runtime/workbench_terminal_client.dart';

void main() {
  final id = List.filled(40, 'a').join();
  Map<String, dynamic> frame(List<int> payload, int start, {String taskId = ''}) => {
    'id': id,
    'mode': 'pty',
    'taskId': taskId,
    'running': true,
    'exitCode': -1,
    'cursor': start + payload.length,
    'fromCursor': start,
    'data': base64Encode(payload),
    'truncated': false,
  };

  test('keeps task identity and exact byte positions', () {
    final snapshot = WorkbenchTerminalSnapshot.parse(frame(utf8.encode('你好'), 13, taskId: 'task-1'));
    expect(snapshot.taskId, 'task-1');
    expect(snapshot.fromCursor, 13);
    expect(snapshot.cursor, 19);
    expect(snapshot.data, utf8.encode('你好'));
  });

  test('rejects dishonest byte cursors and task identities', () {
    final corrupt = frame([65, 66], 0)..['cursor'] = 9;
    expect(() => WorkbenchTerminalSnapshot.parse(corrupt), throwsFormatException);
    final invalidTask = frame([65], 0)..['taskId'] = 50;
    expect(() => WorkbenchTerminalSnapshot.parse(invalidTask), throwsFormatException);
  });

  test('accepts legacy responses without fromCursor', () {
    final old = frame([65], 0)..remove('fromCursor');
    expect(WorkbenchTerminalSnapshot.parse(old).fromCursor, isNull);
  });
}
