import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../runtime/workbench_terminal_client.dart';
import '../../runtime/workbench_terminal_output.dart';

class WorkbenchTerminalView extends ConsumerStatefulWidget {
  final String conversationId;
  final String initialWorkspaceUri;
  final String taskId;
  final bool active;

  const WorkbenchTerminalView({
    super.key,
    required this.conversationId,
    this.initialWorkspaceUri = '',
    this.taskId = '',
    required this.active,
  });

  @override
  ConsumerState<WorkbenchTerminalView> createState() => _WorkbenchTerminalViewState();
}

class _WorkbenchTerminalViewState extends ConsumerState<WorkbenchTerminalView> with WidgetsBindingObserver {
  final _input = TextEditingController();
  final _scroll = ScrollController();
  final _output = WorkbenchTerminalOutput();
  final List<Map<String, dynamic>> _mounts = [];
  final List<Map<String, dynamic>> _sessions = [];
  String _resumeId = '';
  Timer? _pollTimer;
  String _workspaceUri = '';
  String _sessionId = '';
  String _mode = '';
  String _error = '';
  String _latestLifecycle = '';
  Future<void> _showLifecycle() async {
    if (_sessionId.isEmpty) return;
    final id = _sessionId;
    try {
      final records = await _client.lifecycle(id, 0);
      if (!mounted || _closed || id != _sessionId) return;
      final messages = records.map((row) => '${row['sequence']} · ${row['kind']} · ${row['at'] ?? ''}').join('\n');
      setState(() => _latestLifecycle = messages.isEmpty ? '暂无记录' : messages);
      if (!mounted) return;
      await showDialog<void>(context: context, builder: (context) => AlertDialog(
        title: const Text('终端生命周期'),
        content: SingleChildScrollView(child: SelectableText(_latestLifecycle)),
        actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('关闭'))],
      ));
    } catch (error) {
      if (mounted && !_closed && id == _sessionId) setState(() => _error = '事件读取失败：$error');
    }
  }
  Future<void> _showTaskReceipts() async {
    if (widget.taskId.isEmpty) return;
    try {
      final diagnostics = await _client.taskReceiptDiagnostics(widget.taskId);
      final receipts = (diagnostics['receipts'] as List).whereType<Map<String,dynamic>>().toList();
      final summary = diagnostics['summary'] as Map<String,dynamic>;
      if (!mounted || _closed) return;
      final lines = receipts.map((row) => '${row['startedAt']} · ${row['state'] == 'interrupted' ? '旧会话中断/未知' : row['state']} · ${row['exitCode'] is int ? '退出码 ${row['exitCode']}' : '无退出码'}').join('\n');
      await showDialog<void>(context: context, builder: (context) => AlertDialog(
        title: const Text('任务终端历史'),
        content: SingleChildScrollView(child: SelectableText('总记录：${summary['total']} · 正在运行：${summary['running']} · 中断：${summary['interrupted']} · 撤销：${summary['revoked']}\n\n${lines.isEmpty ? '暂无运行记录' : lines}\n\n仅包含生命周期元数据，不是任务或测试通过证明。')),
        actions: [TextButton(onPressed: () => Navigator.of(context).pop(), child: const Text('关闭'))],
      ));
    } catch (error) {
      if (mounted && !_closed) setState(() => _error = '读取任务运行记录失败：$error');
    }
  }

  int _exitCode = -1;
  bool _running = false;
  bool _everExited = false;
  bool _busy = false;
  bool _polling = false;
  bool _foreground = true;
  bool _closed = false;
  int _generation = 0;
  int _retryFailures = 0;

  late final WorkbenchTerminalClient _client;

  @override
  void initState() {
    super.initState();
    _client = WorkbenchTerminalClient(ref.read(backendServiceProvider), widget.conversationId);
    _workspaceUri = widget.initialWorkspaceUri;
    WidgetsBinding.instance.addObserver(this);
    unawaited(_loadMounts());
    unawaited(_loadSessions());
  }

  @override
  void didUpdateWidget(covariant WorkbenchTerminalView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.active != widget.active) _syncPoller();
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    _foreground = state == AppLifecycleState.resumed;
    _syncPoller();
  }

  @override
  void dispose() {
    _closed = true;
    _generation++;
    _pollTimer?.cancel();
    WidgetsBinding.instance.removeObserver(this);
    _input.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _loadMounts() async {
    try {
      final rows = await _client.listLocalMounts();
      if (!mounted || _closed) return;
      setState(() {
        _mounts.addAll(rows);
        if (_workspaceUri.isEmpty && _mounts.length == 1) _workspaceUri = _mounts.first['rootUri'] as String;
      });
    } catch (error) {
      if (mounted && !_closed) setState(() => _error = '无法加载本机工作区：$error');
    }
  }

  Future<void> _loadSessions() async {
    try {
      final rows = await _client.listSessions();
      if (!mounted || _closed) return;
      setState(() {
        _sessions..clear()..addAll(rows);
        if (!_sessions.any((row) => row['id'] == _resumeId && (row['taskId'] ?? '') == widget.taskId)) _resumeId = '';
      });
    } catch (_) {
      if (mounted && !_closed) setState(() => _sessions.clear());
    }
  }

  Future<void> _resume() async {
    if (_busy || _resumeId.isEmpty || _sessionId.isNotEmpty) return;
    setState(() { _busy = true; _error = ''; });
    try {
      final snapshot = await _client.read(_resumeId, 0);
      if (snapshot.taskId != widget.taskId) throw const FormatException('终端任务绑定与当前工作台不匹配');
      if (!mounted || _closed) return;
      final generation = ++_generation;
      _output.reset();
      _everExited = false;
      _exitCode = -1;
      _sessionId = snapshot.id;
      _consume(snapshot, generation);
      _syncPoller();
    } catch (error) {
      if (mounted && !_closed) setState(() => _error = '恢复失败：$error');
      await _loadSessions();
    } finally {
      if (mounted && !_closed) setState(() => _busy = false);
    }
  }

  void _syncPoller() {
    _pollTimer?.cancel();
    _pollTimer = null;
    if (_closed || !widget.active || !_foreground || !_running || _sessionId.isEmpty) return;
    _pollTimer = Timer.periodic(const Duration(milliseconds: 950), (_) => unawaited(_read()));
    unawaited(_read());
  }

  void _consume(WorkbenchTerminalSnapshot snapshot, int generation) {
    if (!mounted || _closed || generation != _generation || snapshot.id != _sessionId) return;
    _retryFailures = 0;
    setState(() {
      _output.append(snapshot.cursor, snapshot.data, truncated: snapshot.truncated, fromCursor: snapshot.fromCursor);
      final wasExited = _everExited;
      if (!snapshot.running) _everExited = true;
      _running = !_everExited;
      if (!wasExited) _exitCode = snapshot.exitCode;
      _mode = snapshot.mode;
      _error = '';
    });
    if (!_running) _syncPoller();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted && _scroll.hasClients) _scroll.jumpTo(_scroll.position.maxScrollExtent);
    });
  }

  Future<void> _read() async {
    if (_polling || _closed || !widget.active || !_foreground || _sessionId.isEmpty) return;
    final generation = _generation;
    final id = _sessionId;
    _polling = true;
    try {
      final snapshot = await _client.read(id, _output.cursor);
      _consume(snapshot, generation);
    } catch (error) {
      if (!mounted || _closed || generation != _generation) return;
      final message = error.toString();
      _retryFailures++;
      setState(() => _error = '终端暂时不可用：$message');
      if (_retryFailures >= 3) {
        _pollTimer?.cancel();
        _pollTimer = null;
        if (mounted) setState(() => _error = '终端连接中断，请手动重新读取；服务端重启后需要新建终端');
      }
    } finally {
      _polling = false;
    }
  }

  Future<void> _open() async {
    if (_busy || _sessionId.isNotEmpty || _workspaceUri.isEmpty) return;
    setState(() { _busy = true; _error = ''; });
    try {
      final snapshot = await _client.open(_workspaceUri, taskId: widget.taskId);
      if (!mounted || _closed) return;
      final generation = ++_generation;
      _output.reset();
      _everExited = false;
      _exitCode = -1;
      _sessionId = snapshot.id;
      _consume(snapshot, generation);
      _syncPoller();
    } catch (error) {
      if (mounted && !_closed) setState(() => _error = '启动失败：$error');
    } finally {
      if (mounted && !_closed) setState(() => _busy = false);
    }
  }

  Future<void> _send(String text) async {
    if (_busy || !_running || _sessionId.isEmpty) return;
    setState(() { _busy = true; _error = ''; });
    try {
      await _client.input(_sessionId, text);
      if (mounted) unawaited(_read());
    } catch (error) {
      if (mounted && !_closed) setState(() => _error = '输入失败：$error');
    } finally {
      if (mounted && !_closed) setState(() => _busy = false);
    }
  }

  void _sendLine() {
    final line = _input.text;
    if (line.trim().isEmpty) return;
    _input.clear();
    unawaited(_send('$line\n'));
  }

  Future<void> _stop() async {
    final id = _sessionId;
    if (id.isEmpty) return;
    _generation++;
    _pollTimer?.cancel();
    _pollTimer = null;
    setState(() { _running = false; _everExited = true; _sessionId = ''; });
    try {
      await _client.close(id);
    } catch (error) {
      if (mounted && !_closed) setState(() => _error = '关闭会话失败：$error');
    }
    await _loadSessions();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return ColoredBox(
      color: const Color(0xff14171b),
      child: Column(children: [
        Padding(
          padding: const EdgeInsets.all(8),
          child: Wrap(spacing: 8, runSpacing: 6, crossAxisAlignment: WrapCrossAlignment.center, children: [
            SizedBox(width: 210, child: DropdownButtonFormField<String>(
              value: _mounts.any((row) => row['rootUri'] == _workspaceUri) ? _workspaceUri : null,
              isExpanded: true,
              decoration: const InputDecoration(labelText: '本机工作区', isDense: true, border: OutlineInputBorder()),
              items: _mounts.map((row) => DropdownMenuItem<String>(value: row['rootUri'] as String, child: Text((row['name'] ?? row['rootUri']).toString(), overflow: TextOverflow.ellipsis))).toList(),
              onChanged: _sessionId.isNotEmpty ? null : (value) => setState(() => _workspaceUri = value ?? ''),
            )),
            if (_sessionId.isEmpty) FilledButton(onPressed: _busy || _workspaceUri.isEmpty ? null : _open, child: const Text('启动终端')),
            if (_sessionId.isEmpty && _sessions.isNotEmpty) SizedBox(width: 160, child: DropdownButtonFormField<String>(
              value: _sessions.any((row) => row['id'] == _resumeId && (row['taskId'] ?? '') == widget.taskId) ? _resumeId : null,
              isExpanded: true,
              decoration: const InputDecoration(labelText: '已有会话', isDense: true, border: OutlineInputBorder()),
              items: _sessions.where((row) => (row['taskId'] ?? '') == widget.taskId).map((row) => DropdownMenuItem<String>(value: row['id'] as String, child: Text('${(row['id'] as String).substring(32)} · ${row['running'] == true ? '运行中' : '已结束'}', overflow: TextOverflow.ellipsis))).toList(),
              onChanged: _busy ? null : (value) => setState(() => _resumeId = value ?? ''),
            )),
            if (_sessionId.isEmpty && _resumeId.isNotEmpty) TextButton(onPressed: _busy ? null : _resume, child: const Text('恢复')),
            if (_sessionId.isNotEmpty) OutlinedButton(onPressed: _stop, child: const Text('结束')),
            if (_sessionId.isNotEmpty) TextButton(onPressed: _showLifecycle, child: const Text('运行事件')),
            if (widget.taskId.isNotEmpty) TextButton(onPressed: _showTaskReceipts, child: const Text('任务终端历史')),
            if (_sessionId.isNotEmpty) Text(_running ? (_mode == 'pipe' ? '管道兼容' : 'PTY 会话') : '已结束 ($_exitCode)', style: const TextStyle(color: Colors.white70)),
            if (_sessionId.isNotEmpty && _running) IconButton(tooltip: '手动刷新输出', onPressed: () { _retryFailures = 0; _syncPoller(); }, icon: const Icon(Icons.refresh, color: Colors.white70)),
          ]),
        ),
        if (_error.isNotEmpty) Padding(padding: const EdgeInsets.all(8), child: Text(_error, style: TextStyle(color: colors.error))),
        Expanded(child: Scrollbar(controller: _scroll, child: SingleChildScrollView(
          controller: _scroll,
          padding: const EdgeInsets.all(12),
          child: Align(alignment: Alignment.topLeft, child: SelectableText(
            _output.text.isEmpty ? '仅受信本机用户可启动。云端和其他设备不能远程执行此终端。\n输出为简化文本模式，不支持完整 ANSI/TUI。' : _output.text,
            style: const TextStyle(color: Color(0xffe1e6ee), fontSize: 12, fontFamily: 'monospace', height: 1.4),
          )),
        ))),
        SafeArea(top: false, child: Padding(padding: const EdgeInsets.all(8), child: Row(children: [
          Expanded(child: TextField(
            controller: _input,
            enabled: _running && !_busy,
            style: const TextStyle(color: Colors.white),
            decoration: const InputDecoration(hintText: '输入命令', isDense: true, border: OutlineInputBorder()),
            onSubmitted: (_) => _sendLine(),
          )),
          IconButton(tooltip: '发送命令', onPressed: _running && !_busy ? _sendLine : null, icon: const Icon(Icons.send, color: Colors.white70)),
          IconButton(tooltip: 'Ctrl+C', onPressed: _running && !_busy && _mode != 'pipe' ? () => unawaited(_send('\x03')) : null, icon: const Icon(Icons.stop_circle_outlined, color: Colors.white70)),
        ]))),
      ]),
    );
  }
}
