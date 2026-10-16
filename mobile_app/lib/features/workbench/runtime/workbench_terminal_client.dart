import 'dart:convert';

import '../../../core/backend_transport/backend_service_api.dart';

class WorkbenchTerminalSnapshot {
  final String id;
  final String mode;
  final String taskId;
  final bool running;
  final int exitCode;
  final int cursor;
  final int? fromCursor;
  final bool truncated;
  final List<int> data;

  const WorkbenchTerminalSnapshot({
    required this.id,
    required this.mode,
    required this.taskId,
    required this.running,
    required this.exitCode,
    required this.cursor,
    this.fromCursor,
    required this.truncated,
    required this.data,
  });

  factory WorkbenchTerminalSnapshot.parse(Map<String, dynamic> data) {
    final id = data['id'];
    final mode = data['mode'];
    final cursor = data['cursor'];
    final taskId = data['taskId'];
    final content = data['data'];
    if (id is! String || !RegExp(r'^[a-f0-9]{40}$').hasMatch(id) ||
        mode is! String || !{'pty', 'conpty', 'pipe'}.contains(mode) ||
        cursor is! int || cursor < 0 || (taskId != null && taskId is! String) ||
        content is! String || content.length > 100000) {
      throw const FormatException('终端快照格式无效');
    }
    final bytes = base64Decode(content);
    if (bytes.length > 65536) throw const FormatException('终端输出超过限制');
    final rawStart = data['fromCursor'];
    if (rawStart != null && (rawStart is! int || rawStart < 0 || rawStart > cursor ||
        cursor - rawStart != bytes.length)) {
      throw const FormatException('终端输出分片游标无效');
    }
    return WorkbenchTerminalSnapshot(
      id: id,
      mode: mode,
      taskId: taskId is String ? taskId : '',
      running: data['running'] == true,
      exitCode: data['exitCode'] is int ? data['exitCode'] as int : -1,
      cursor: cursor,
      fromCursor: rawStart is int ? rawStart : null,
      truncated: data['truncated'] == true,
      data: bytes,
    );
  }
}

class WorkbenchTerminalClient {
  final BackendServiceApi api;
  final String conversationId;

  const WorkbenchTerminalClient(this.api, this.conversationId);

  Future<List<Map<String, dynamic>>> listLocalMounts() async {
    final rows = await api.get<List<dynamic>>('/api/workspaces');
    return (rows ?? const <dynamic>[])
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['kind'] == 'local' && row['readOnly'] != true && row['available'] == true && row['rootUri'] is String)
        .toList();
  }

  Future<Map<String, dynamic>> taskReceiptDiagnostics(String taskId) async {
    if (!RegExp(r'^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$').hasMatch(taskId)) throw const FormatException('任务标识格式不正确');
    final data = await api.get<Map<String,dynamic>>('/api/workbench/terminal/task-receipts', queryParameters: {'conversationId': conversationId, 'taskId': taskId});
    if (data == null || data['conversationId'] != conversationId || data['taskId'] != taskId || data['receipts'] is! List) throw const FormatException('任务终端凭据格式不正确');
    final rows = data['receipts'] as List;
    if (rows.length > 32) throw const FormatException('任务终端凭据超过限制');
    final receipts = rows.whereType<Map>().map((row) => Map<String,dynamic>.from(row))
        .where((row) => row['taskId'] == taskId && row['conversationId'] == conversationId &&
          row['sessionId'] is String && RegExp(r'^[a-f0-9]{40}$').hasMatch(row['sessionId'] as String) &&
          {'running','exited','closed','revoked','interrupted'}.contains(row['state']) &&
          row['evidenceKind'] == 'terminal-lifecycle-metadata-not-test-proof').toList();
    final rawSummary = data['summary'];
    if (rawSummary is! Map) throw const FormatException('任务终端诊断摘要缺失');
    final summary = Map<String,dynamic>.from(rawSummary);
    if (summary['verificationKind'] != 'lifecycle-only-not-test-proof' || summary['total'] != receipts.length ||
        !{'none','running','exited','closed','revoked','interrupted'}.contains(summary['latestState']) ||
        (receipts.isNotEmpty && (summary['latestSessionId'] != receipts.first['sessionId'] || summary['latestState'] != receipts.first['state']))) {
      throw const FormatException('任务终端诊断摘要与记录不一致');
    }
    return {'receipts': receipts, 'summary': summary};
  }

  Future<List<Map<String, dynamic>>> taskReceipts(String taskId) async {
    final result = await taskReceiptDiagnostics(taskId);
    return (result['receipts'] as List).whereType<Map<String,dynamic>>().toList();
  }

  Future<List<Map<String, dynamic>>> listSessions() async {
    final rows = await api.get<List<dynamic>>(
      '/api/workbench/terminal/sessions',
      queryParameters: {'conversationId': conversationId},
    );
    return (rows ?? const <dynamic>[])
        .whereType<Map>()
        .map((row) => Map<String, dynamic>.from(row))
        .where((row) => row['id'] is String && RegExp(r'^[a-f0-9]{40}$').hasMatch(row['id'] as String))
        .toList();
  }

  Future<WorkbenchTerminalSnapshot> open(String workspaceUri, {String taskId = ''}) async {
    final result = await api.post<Map<String, dynamic>>(
      '/api/workbench/terminal/open',
      data: {'conversationId': conversationId, 'workspaceUri': workspaceUri, 'taskId': taskId, 'rows': 30, 'cols': 100},
    );
    if (result == null) throw const FormatException('终端没有返回会话信息');
    return WorkbenchTerminalSnapshot.parse(result);
  }

  Future<WorkbenchTerminalSnapshot> read(String id, int cursor) async {
    final result = await api.get<Map<String, dynamic>>(
      '/api/workbench/terminal/${Uri.encodeComponent(id)}',
      queryParameters: {'conversationId': conversationId, 'cursor': cursor},
    );
    if (result == null) throw const FormatException('终端没有返回输出信息');
    final snapshot = WorkbenchTerminalSnapshot.parse(result);
    if (snapshot.id != id) throw const FormatException('终端会话身份不匹配');
    return snapshot;
  }

  Future<List<Map<String, dynamic>>> lifecycle(String id, int after) async {
    if (after < 0) throw const FormatException('事件游标无效');
    final result = await api.get<Map<String, dynamic>>(
      '/api/workbench/terminal/${Uri.encodeComponent(id)}/lifecycle',
      queryParameters: {'conversationId': conversationId, 'after': after},
    );
    if (result == null || result['sessionId'] != id || result['scope'] != 'live-session-memory-only' || result['events'] is! List) {
      throw const FormatException('终端生命周期记录格式不正确');
    }
    final events = result['events'] as List;
    if (events.length > 64) throw const FormatException('终端事件超过限制');
    return events.whereType<Map>().map((row) => Map<String, dynamic>.from(row)).where((row) => row['kind'] is String && row['sequence'] is int).toList();
  }

  Future<void> input(String id, String text) async {
    final bytes = utf8.encode(text);
    if (bytes.isEmpty || bytes.length > 4096) throw const FormatException('单次输入必须为 1～4096 字节');
    await api.post<void>(
      '/api/workbench/terminal/${Uri.encodeComponent(id)}/input',
      data: {'conversationId': conversationId, 'data': base64Encode(bytes)},
    );
  }

  Future<void> resize(String id, int rows, int cols) async {
    if (rows < 10 || rows > 120 || cols < 20 || cols > 300) throw const FormatException('终端尺寸超出限制');
    await api.post<void>(
      '/api/workbench/terminal/${Uri.encodeComponent(id)}/resize',
      data: {'conversationId': conversationId, 'rows': rows, 'cols': cols},
    );
  }

  Future<void> close(String id) async {
    await api.delete(
      '/api/workbench/terminal/${Uri.encodeComponent(id)}',
      queryParameters: {'conversationId': conversationId},
    );
  }
}
