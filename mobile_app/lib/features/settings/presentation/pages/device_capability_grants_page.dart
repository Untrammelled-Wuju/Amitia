import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class DeviceCapabilityGrantsPage extends ConsumerStatefulWidget {
  const DeviceCapabilityGrantsPage({super.key, required this.deviceId, required this.label, required this.devices});
  final String deviceId;
  final String label;
  final List<Map<String, dynamic>> devices;
  @override
  ConsumerState<DeviceCapabilityGrantsPage> createState() => _DeviceCapabilityGrantsPageState();
}

class _DeviceCapabilityGrantsPageState extends ConsumerState<DeviceCapabilityGrantsPage> {
  final _capability = TextEditingController(text: 'ai.chat');
  String _caller = '';
  String _core = '';
  String _error = '';
  List<Map<String, dynamic>> _grants = [];
  bool _loading = false;
  bool _busy = false;
  bool _allowed = false;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    final ticket = ++_generation;
    setState(() { _loading = true; _allowed = false; _error = ''; });
    try {
      final service = ref.read(deviceMeshServiceProvider);
      final before = await service.coordination();
      if (before['canAdminister'] != true && before['policy']?['deviceId'] != widget.deviceId) {
        throw StateError('能力授权只能由目标设备或当前 Core 管理员修改');
      }
      final grants = await service.capabilityGrants(widget.deviceId);
      final after = await service.coordination();
      if (before['coreId'] != after['coreId']) throw StateError('服务提供者已变化，请刷新后重新授权');
      if (!mounted || ticket != _generation) return;
      setState(() { _grants = grants; _core = before['coreId'].toString(); _allowed = true; });
    } catch (cause) {
      if (mounted && ticket == _generation) setState(() => _error = cause.toString());
    } finally {
      if (mounted && ticket == _generation) setState(() => _loading = false);
    }
  }

  Future<void> _save(bool allowed, {Map<String, dynamic>? grant}) async {
    if (_busy || !_allowed) return;
    final caller = grant?['callerId'].toString() ?? _caller;
    final capability = grant?['capability'].toString() ?? _capability.text.trim();
    if (caller.isEmpty || capability.isEmpty) return;
    final previous = grant ?? _grants.where((row) => row['callerId'] == caller && row['capability'] == capability).firstOrNull;
    setState(() { _busy = true; _error = ''; });
    try {
      await ref.read(deviceMeshServiceProvider).setCapabilityGrant(target: widget.deviceId, caller: caller, capability: capability, allowed: allowed, expectedRevision: (previous?['revision'] as num? ?? 0).toInt(), expectedCoreId: _core);
      if (mounted) await _load();
    } catch (cause) {
      if (mounted) setState(() => _error = cause.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  void dispose() {
    _generation++;
    _capability.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final devices = widget.devices.where((row) => row['deviceId'] != widget.deviceId && row['trustState'] == 'trusted').toList();
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '${widget.label} · 能力授权', showBackButton: true),
      body: ListView(padding: const EdgeInsets.all(20), children: [
        Text('当前 Core：$_core'),
        const Text('只授予明确的设备和能力。撤销后会中断正在进行的相关调用。'),
        if (_error.isNotEmpty) Text(_error, style: TextStyle(color: Theme.of(context).colorScheme.error)),
        if (_loading) const LinearProgressIndicator(),
        DropdownButtonFormField<String>(initialValue: _caller.isEmpty ? null : _caller, decoration: const InputDecoration(labelText: '调用设备'), items: devices.map((row) => DropdownMenuItem(value: row['deviceId'].toString(), child: Text((row['label'] ?? row['deviceId']).toString()))).toList(), onChanged: !_allowed || _busy || _loading ? null : (value) => setState(() => _caller = value ?? '')),
        TextField(controller: _capability, maxLength: 128, enabled: _allowed && !_busy && !_loading, decoration: const InputDecoration(labelText: '能力标识', hintText: 'ai.chat 或准确的设备能力标识'), onChanged: (_) => setState(() {})),
        FilledButton(onPressed: !_allowed || _busy || _loading || _caller.isEmpty || _capability.text.trim().isEmpty ? null : () => _save(true), child: const Text('允许调用')),
        for (final grant in _grants) ListTile(
          title: Text('${grant['callerId']} · ${grant['capability']}'),
          subtitle: Text(grant['allowed'] == true ? '已授权' : '已撤销'),
          trailing: TextButton(onPressed: _busy || _loading || !_allowed ? null : () => _save(grant['allowed'] != true, grant: grant), child: Text(grant['allowed'] == true ? '撤销' : '重新授予')),
        ),
        TextButton(onPressed: _busy ? null : _load, child: const Text('刷新')),
      ]),
    );
  }
}
