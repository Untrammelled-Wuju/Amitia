import 'dart:async';
import 'dart:math';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DeviceCoreCallPage extends ConsumerStatefulWidget {
  const DeviceCoreCallPage({super.key, required this.deviceId, required this.label});
  final String deviceId;
  final String label;
  @override
  ConsumerState<DeviceCoreCallPage> createState() => _DeviceCoreCallPageState();
}

class _DeviceCoreCallPageState extends ConsumerState<DeviceCoreCallPage> {
  final _message = TextEditingController();
  List<Map<String, dynamic>> _roles = [];
  Map<String, dynamic>? _expectedScope;
  String _role = '';
  String _core = '';
  String _owner = '';
  String _reply = '';
  String _error = '';
  String _request = '';
  bool _saved = false;
  bool _loading = false;
  bool _running = false;
  bool _checking = false;
  int _generation = 0;
  CancelToken? _cancel;
  Timer? _providerTimer;

  @override
  void initState() {
    super.initState();
    _initialize();
  }

  Future<void> _initialize() async {
    final ticket = ++_generation;
    setState(() { _loading = true; _error = ''; _roles = []; _role = ''; _expectedScope = null; });
    try {
      final result = await ref.read(deviceMeshServiceProvider).prepareCall(widget.deviceId);
      if (!mounted || ticket != _generation) return;
      setState(() {
        _core = result['coreId'].toString();
        _owner = result['roleOwnerId'].toString();
        _expectedScope = Map<String, dynamic>.from(result['executionScope'] as Map);
        _roles = (result['roles'] as List).whereType<Map>().map((row) => Map<String, dynamic>.from(row)).toList();
        if (_roles.length == 1) _role = _roles.first['id'].toString();
        if (_roles.isEmpty) _error = '目标数据来源没有可用角色，拒绝调用';
      });
    } catch (cause) {
      if (mounted && ticket == _generation) setState(() => _error = cause.toString());
    } finally {
      if (mounted && ticket == _generation) setState(() => _loading = false);
    }
  }

  Future<void> _checkProvider() async {
    if (_checking || !_running) return;
    _checking = true;
    final current = _cancel;
    try {
      final state = await ref.read(deviceMeshServiceProvider).coordination();
      if (!mounted || !identical(current, _cancel) || !_running) return;
      if (state['coreId'] != _core || state['coordinationAvailable'] != true) {
        setState(() => _error = state['coreId'] != _core ? '云端服务提供者已从「$_core」切换为「${state['coreId']}」，当前调用已中断。' : 'Core 服务已暂停，当前调用已中断。');
        current?.cancel(_error);
      }
    } catch (_) {
      if (mounted && identical(current, _cancel) && _running) {
        setState(() => _error = '无法确认当前 Core 服务，调用已中断；恢复服务后再发起调用。');
        current?.cancel(_error);
      }
    } finally { _checking = false; }
  }

  Future<void> _send() async {
    if (_running || _role.isEmpty || _message.text.trim().isEmpty || _expectedScope == null) return;
    final selectedRole = _roles.firstWhere((role) => role['id'] == _role);
    final service = ref.read(deviceMeshServiceProvider);
    final ticket = _generation;
    final current = CancelToken();
    _cancel = current;
    _request = 'call-${DateTime.now().microsecondsSinceEpoch}-${Random.secure().nextInt(1 << 32)}';
    setState(() { _running = true; _reply = ''; _saved = false; _error = ''; });
    _providerTimer = Timer.periodic(const Duration(seconds: 3), (_) => _checkProvider());
    try {
      await for (final event in service.call(target: widget.deviceId, role: _role, core: _core, owner: _owner, requestId: _request, message: _message.text.trim(), expectedScope: {..._expectedScope!, 'roleId': _role, 'roleRevision': selectedRole['revision']}, cancelToken: current)) {
        if (!mounted || ticket != _generation) return;
        setState(() {
          if (event['type'] == 'delta' && event['reasoning'] != true) _reply += (event['text'] ?? '').toString();
          if (event['type'] == 'completed') {
            _reply = (event['data']['reply'] ?? _reply).toString();
            _saved = event['data']['saved'] == true;
          }
        });
      }
    } catch (cause) {
      if (mounted && ticket == _generation) {
        setState(() => _error = _error.isNotEmpty
            ? _error
            : cause is StateError
                ? cause.message.toString()
                : current.isCancelled
                    ? '当前调用已中断，未确认的结果不会显示为已保存。'
                    : cause.toString());
      }
    } finally {
      _providerTimer?.cancel();
      if (mounted && identical(current, _cancel)) {
        setState(() { _running = false; _cancel = null; _request = ''; });
      }
    }
  }

  Future<void> _interrupt() async {
    final active = _request;
    _cancel?.cancel('当前调用已中断');
    if (active.isEmpty) return;
    try { await ref.read(deviceMeshServiceProvider).interruptCall(active); }
    catch (cause) { if (mounted) setState(() => _error = cause.toString()); }
  }

  @override
  void dispose() {
    _generation++;
    _providerTimer?.cancel();
    _cancel?.cancel('调用页面已关闭');
    _message.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '通过 Core 调用 · ${widget.label}', showBackButton: true),
      body: ListView(padding: const EdgeInsets.all(20), children: [
        Text('服务提供者：$_core\n目标设备：${widget.deviceId}\n角色与新数据所有者：$_owner'),
        const SizedBox(height: 12),
        const Text('AI 计算由 Core 负责，数据按目标设备的统筹状态保存；调用需要目标设备的能力授权。'),
        if (_error.isNotEmpty) Text(_error, style: TextStyle(color: Theme.of(context).colorScheme.error)),
        DropdownButtonFormField<String>(initialValue: _role.isEmpty ? null : _role, decoration: const InputDecoration(labelText: '调用角色'), items: _roles.map((role) => DropdownMenuItem(value: role['id'].toString(), child: Text(role['name'].toString()))).toList(), onChanged: _running || _loading ? null : (value) => setState(() => _role = value ?? '')),
        TextField(controller: _message, enabled: !_running, maxLines: 4, maxLength: 32000, decoration: const InputDecoration(labelText: '任务或问题'), onChanged: (_) => setState(() {})),
        if (_reply.isNotEmpty) SelectableText(_reply),
        if (_saved) Text('已由 $_owner 确认保存'),
        TextButton(onPressed: _running ? null : _initialize, child: Text(_loading ? '正在加载' : '刷新角色与服务')),
        FilledButton(onPressed: _running ? _interrupt : _loading || _role.isEmpty || _message.text.trim().isEmpty ? null : _send, child: Text(_running ? '中断调用' : '发起调用')),
      ]),
    );
  }
}
