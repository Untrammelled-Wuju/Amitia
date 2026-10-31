import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';

import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/backend_transport/errors/backend_transport_error.dart';
import '../../../../core/backend_transport/backend_service_api.dart';
import '../../runtime/remote_browser_failure_policy.dart';

class WorkbenchRemoteBrowserView extends ConsumerStatefulWidget {
  final String conversationId;
  final String tabId;
  final String initialUrl;
  final String browserRef;
  final String targetDeviceId;
  final bool active;
  final ValueChanged<String>? onUrlChanged;

  const WorkbenchRemoteBrowserView({
    super.key,
    required this.conversationId,
    required this.tabId,
    required this.initialUrl,
    this.browserRef = '',
    this.targetDeviceId = '',
    required this.active,
    this.onUrlChanged,
  });

  @override
  ConsumerState<WorkbenchRemoteBrowserView> createState() => _WorkbenchRemoteBrowserViewState();
}

class _WorkbenchRemoteBrowserViewState extends ConsumerState<WorkbenchRemoteBrowserView> with WidgetsBindingObserver {
  late final TextEditingController _address;
  final TextEditingController _input = TextEditingController();
  Timer? _timer;
  Uint8List? _frame;
  int _frameWidth = 0;
  int _frameHeight = 0;
  int _generation = 0;
  int _refreshCounter = 0;
  bool _running = false;
  bool _foreground = true;
  bool _explicitlyClosed = false;
  bool _restoring = false;
  bool _controlPending = false;
  int _domRequestGeneration = 0;
  int _controlRequestGeneration = 0;
  int _reconnectTick = 0;
  int _retryFailures = 0;
  DateTime _nextRetryAt = DateTime.fromMillisecondsSinceEpoch(0);
  void _recordRemoteFailure(Object error) {
    final terminal = _isTerminalRemoteFailure(error);
    _running = false;
    _frame = null;
    _dom = '';
    _domVisible = false;
    _canControl = false;
    _leaseId = '';
    if (terminal) {
      _explicitlyClosed = true;
      _timer?.cancel();
      _timer = null;
      _error = '远程浏览器观察授权无效，请重新获取授权：$error';
      return;
    }
    _retryFailures = (_retryFailures + 1).clamp(1, 7).toInt();
    final delay = (2 << (_retryFailures - 1)).clamp(2, 30).toInt();
    _nextRetryAt = DateTime.now().add(Duration(seconds: delay));
    _error = '远程浏览器暂时不可用：$error';
  }

  bool _isTerminalRemoteFailure(Object error) {
    final code = error is BackendTransportError ? error.statusCode : error is ServiceApiException ? error.code : null;
    final cause = error is BackendTransportError ? error.cause : null;
    final body = cause is DioException ? cause.response?.data : null;
    final payload = body is Map ? body : null;
    return remoteBrowserFailureIsTerminal(code,
      errorCode: payload?['errorCode'] is String ? payload!['errorCode'] as String : null,
      retryable: payload?['retryable'] is bool ? payload!['retryable'] as bool : null);

  }
  bool _observed = false;
  bool _canControl = false;
  String _leaseId = '';
  int _controlTick = 0;
  bool _busy = false;
  bool _fetching = false;
  bool _domVisible = false;
  String _dom = '';
  String? _error;

  bool get _isRemote => widget.targetDeviceId.isNotEmpty;

  Future<Map<String, dynamic>> _remoteRead(String operation) async {
    if (!_isRemote || widget.browserRef.isEmpty) throw StateError('远程浏览器缺少目标设备或观察引用');
    final data = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
      '/api/device-mesh/v1/workbench/browser/read',
      data: {'targetDeviceId': widget.targetDeviceId, 'conversationId': widget.conversationId, 'browserRef': widget.browserRef, 'operation': operation},
      fromJson: (value) => Map<String, dynamic>.from(value as Map),
    );
    if (data == null || data['ready'] != true || data['observed'] != true) throw StateError('远程浏览器观察已失效');
    return data;
  }

  String get _endpoint => '/api/workbench/browser/${Uri.encodeComponent(widget.conversationId)}/${Uri.encodeComponent(widget.tabId)}';

  @override
  void initState() {
    super.initState();
    _address = TextEditingController(text: widget.initialUrl);
    WidgetsBinding.instance.addObserver(this);
    _foreground = WidgetsBinding.instance.lifecycleState == null || WidgetsBinding.instance.lifecycleState == AppLifecycleState.resumed;
    if (widget.active && _foreground) _resume();
  }

  @override
  void didUpdateWidget(covariant WorkbenchRemoteBrowserView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.initialUrl != widget.initialUrl && !_busy && widget.initialUrl.isNotEmpty) {
      _address.text = widget.initialUrl;
    }
    if (oldWidget.active != widget.active || oldWidget.tabId != widget.tabId || oldWidget.conversationId != widget.conversationId || oldWidget.browserRef != widget.browserRef || oldWidget.targetDeviceId != widget.targetDeviceId) {
      final changedResource = oldWidget.tabId != widget.tabId || oldWidget.conversationId != widget.conversationId || oldWidget.browserRef != widget.browserRef || oldWidget.targetDeviceId != widget.targetDeviceId;
      if (oldWidget.active && _canControl && (!widget.active || changedResource)) {
        _releaseControlFor(oldWidget.conversationId, oldWidget.tabId, oldWidget.targetDeviceId, leaseId: _leaseId);
      }
      _timer?.cancel();
      _generation++;
      _domRequestGeneration++;
      _controlRequestGeneration++;
      _controlPending = false;
      _fetching = false;
      _restoring = false;
      _busy = false;
      _canControl = false;
      _leaseId = '';
      _observed = false;
      _domVisible = false;
      _dom = '';
      _frame = null;
      _frameWidth = 0;
      _frameHeight = 0;
      if (changedResource) {
        _explicitlyClosed = false;
        _retryFailures = 0;
        _nextRetryAt = DateTime.fromMillisecondsSinceEpoch(0);
        _error = null;
      }
      _running = false;
      if (widget.active && _foreground) _resume();
    }
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    final resumed = state == AppLifecycleState.resumed;
    if (_foreground == resumed) return;
    _foreground = resumed;
    if (!resumed) {
      _generation++;
      _domRequestGeneration++;
      _controlRequestGeneration++;
      _controlPending = false;
      _timer?.cancel();
      _timer = null;
      _fetching = false;
      _restoring = false;
      _busy = false;
      if (_canControl) _releaseControl();
    } else if (widget.active && !_explicitlyClosed) {
      _resume();
    }
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _generation++;
    _domRequestGeneration++;
    _controlRequestGeneration++;
    if (_canControl) _releaseControl();
    _timer?.cancel();
    _address.dispose();
    _input.dispose();
    super.dispose();
  }

  void _resume() {
    if (!mounted || !widget.active || !_foreground || _explicitlyClosed) return;
    _timer?.cancel();
    _timer = Timer.periodic(const Duration(seconds: 3), (_) {
      if (!widget.active || !_foreground || _explicitlyClosed) return;
      if (_running) {
        _refresh();
        if (_canControl && ++_controlTick % 7 == 0) _control('renew');
      } else if (++_reconnectTick % 2 == 0 && !_busy && DateTime.now().isAfter(_nextRetryAt)) {
        _restore();
      }
    });
    _restore();
  }

  Future<void> _restore() async {
    if (_restoring || !widget.active || !_foreground || _explicitlyClosed) return;
    _restoring = true;
    final sequence = _generation;
    try {
      final result = _isRemote ? await _remoteRead('state') : await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        _endpoint,
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted || sequence != _generation || !widget.active) return;
      if (!_isRemote && widget.browserRef.isNotEmpty && result?['observed'] != true) {
        await _attach();
        return;
      }
      setState(() {
        _retryFailures = 0;
        _nextRetryAt = DateTime.fromMillisecondsSinceEpoch(0);
        _error = null;
        _running = result?['ready'] == true;
        _observed = result?['observed'] == true || widget.browserRef.isNotEmpty;
        _canControl = result?['canControl'] == true;
        _leaseId = _canControl ? (result?['leaseId'] as String? ?? '') : '';
        final url = result?['url'];
        if (url is String && (url.startsWith('https://') || url.startsWith('http://'))) {
          _address.text = url;
        }
      });
      if (_running) await _refresh();
    } catch (error) {
      if (!_isRemote && widget.browserRef.isNotEmpty && widget.active && _foreground && sequence == _generation) {
        await _attach();
      } else if (mounted && sequence == _generation) {
        setState(() {
          if (_isRemote) {
            _recordRemoteFailure(error);
          } else {
            _running = false;
            _error = '本地 Chromium 会话不可用，请重新连接';
          }
        });
      }
    } finally {
      if (sequence == _generation) _restoring = false;
    }
  }

  Future<void> _attach() async {
    if (_isRemote || widget.browserRef.isEmpty || _busy) return;
    final sequence = _generation;
    try {
      final response = await ref.read(backendServiceProvider).post<Map<String, dynamic>>(
        '/api/workbench/browser/observe',
        data: {'conversationId': widget.conversationId, 'viewId': widget.tabId, 'browserRef': widget.browserRef},
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted || sequence != _generation || !widget.active) return;
      setState(() {
        _running = response?['ready'] == true;
        _observed = response?['observed'] == true;
        _canControl = false;
        _leaseId = '';
        _error = null;
      });
      if (_running) await _refresh();
    } catch (error) {
      if (mounted && sequence == _generation) setState(() => _error = 'Agent 浏览器观察不可用：$error');
    }
  }

  Future<void> _refresh() async {
    if (!mounted || !widget.active || !_foreground || !_running || _fetching || _domVisible) return;
    _fetching = true;
    final sequence = _generation;
    try {
      Uint8List bytes;
      if (_isRemote) {
        final response = await _remoteRead('screenshot');
        final encoded = response['imageBase64'];
        if (encoded is! String || encoded.length > 3 * 1024 * 1024) throw StateError('远程截图无效或超出上限');
        bytes = base64Decode(encoded);
        if (bytes.length > 2 * 1024 * 1024) throw StateError('远程截图超过大小上限');
      } else {
        final stream = await ref.read(backendServiceProvider).getStream('$_endpoint/screenshot');
        final builder = BytesBuilder(copy: false);
        await for (final chunk in stream) {
          if (!mounted || sequence != _generation || !widget.active) return;
          if (builder.length + chunk.length > 6 * 1024 * 1024) throw StateError('浏览器截图超过大小上限');
          builder.add(chunk);
        }
        bytes = builder.takeBytes();
      }
      if (bytes.isEmpty || !mounted || sequence != _generation) return;
      final codec = await ui.instantiateImageCodec(bytes);
      final image = await codec.getNextFrame();
      final width = image.image.width;
      final height = image.image.height;
      image.image.dispose();
      codec.dispose();
      if (!mounted || sequence != _generation || !widget.active) return;
      setState(() {
        _frame = bytes;
        _frameWidth = width;
        _frameHeight = height;
        _error = null;
      });
      if (_observed && ++_refreshCounter % 4 == 0) await _syncObservedState(sequence);
    } catch (error) {
      if (mounted && sequence == _generation) setState(() {
        if (_isRemote) _recordRemoteFailure(error);
        else _error = error.toString();
      });
    } finally {
      if (sequence == _generation) _fetching = false;
    }
  }

  Future<void> _syncObservedState(int generation) async {
    try {
      final result = _isRemote ? await _remoteRead('state') : await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        _endpoint, fromJson: (value) => Map<String, dynamic>.from(value as Map));
      if (!mounted || generation != _generation || !widget.active || result?['observed'] != true) return;
      final updatedControl = result?['canControl'] == true;
      final updatedLeaseId = updatedControl ? (result?['leaseId'] as String? ?? '') : '';
      if (_canControl != updatedControl || _leaseId != updatedLeaseId) {
        setState(() { _canControl = updatedControl; _leaseId = updatedLeaseId; });
      }
      final url = result?['url'];
      if (url is String && (url.startsWith('http://') || url.startsWith('https://')) && url != _address.text) {
        _address.text = url;
        widget.onUrlChanged?.call(url);
      }
    } catch (error) {
      if (mounted && generation == _generation) setState(() {
        if (_isRemote) _recordRemoteFailure(error);
        else { _running = false; _error = error.toString(); }
      });
    }
  }

  Future<void> _execute(Future<dynamic> Function() command, {bool updateUrl = false, String? expectedLeaseId}) async {
    if (_busy || !widget.active || !_foreground) return;
    final sequence = _generation;
    setState(() { _busy = true; _error = null; });
    try {
      final response = await command();
      if (!mounted || sequence != _generation || !widget.active || !_foreground || (expectedLeaseId != null && _leaseId != expectedLeaseId)) return;
      if (updateUrl && response is Map && response['url'] is String) {
        final url = response['url'] as String;
        if (url.startsWith('https://') || url.startsWith('http://')) {
          _address.text = url;
          widget.onUrlChanged?.call(url);
        }
      }
      setState(() => _running = true);
      await _refresh();
    } catch (error) {
      if (mounted && sequence == _generation && _foreground && (expectedLeaseId == null || _leaseId == expectedLeaseId)) setState(() => _error = error.toString());
    } finally {
      if (mounted && sequence == _generation) setState(() => _busy = false);
    }
  }

  void _navigate() {
    if (_isRemote) return;
    if ((widget.browserRef.isNotEmpty || _observed) && !_canControl) {
      _attach();
      return;
    }
    final url = Uri.tryParse(_address.text.trim());
    if (url == null || !{'http', 'https'}.contains(url.scheme) || url.host.isEmpty || url.userInfo.isNotEmpty) {
      setState(() => _error = '请输入有效的 HTTP(S) 地址');
      return;
    }
    final api = ref.read(backendServiceProvider);
    if (!_running) {
      _execute(() => api.post<Map<String, dynamic>>('/api/workbench/browser/open',
        data: {'conversationId': widget.conversationId, 'viewId': widget.tabId, 'url': url.toString()},
        fromJson: (value) => Map<String, dynamic>.from(value as Map)), updateUrl: true);
    } else {
      final token = _observed ? _leaseId : null;
      _execute(() => api.post<Map<String, dynamic>>('$_endpoint/navigate',
        data: {'url': url.toString(), if (token != null) 'leaseId': token}, fromJson: (value) => Map<String, dynamic>.from(value as Map)), updateUrl: true, expectedLeaseId: token);
    }
  }

  void _action(String action) {
    if (_isRemote) return;
    if (_observed && !_canControl) return;
    final token = _observed ? _leaseId : null;
    _execute(() => ref.read(backendServiceProvider).post<Map<String, dynamic>>('$_endpoint/action',
      data: {'action': action, if (token != null) 'leaseId': token}, fromJson: (value) => Map<String, dynamic>.from(value as Map)), updateUrl: true, expectedLeaseId: token);
  }

  void _inputAction(Map<String, dynamic> payload) {
    if (_isRemote) return;
    if (_observed && !_canControl) return;
    final token = _observed ? _leaseId : null;
    _execute(() => ref.read(backendServiceProvider).post<Map<String, dynamic>>('$_endpoint/input',
      data: {...payload, if (token != null) 'leaseId': token}, fromJson: (value) => Map<String, dynamic>.from(value as Map)), expectedLeaseId: token);
  }

  Future<void> _readDom() async {
    final request = ++_domRequestGeneration;
    final sequence = _generation;
    if (_domVisible) {
      setState(() => _domVisible = false);
      await _refresh();
      return;
    }
    try {
      final response = _isRemote ? await _remoteRead('dom') : await ref.read(backendServiceProvider).get<Map<String, dynamic>>('$_endpoint/dom',
        fromJson: (value) => Map<String, dynamic>.from(value as Map));
      if (!mounted || !widget.active || !_foreground || _explicitlyClosed || sequence != _generation || request != _domRequestGeneration) return;
      final content = response?['content'];
      if (content is! String || content.length > 1024 * 1024) throw StateError('浏览器 DOM 数据无效或超出大小上限');
      setState(() { _dom = content; _domVisible = true; });
    } catch (error) {
      if (mounted && widget.active && sequence == _generation && request == _domRequestGeneration) setState(() {
        if (_isRemote) _recordRemoteFailure(error);
        else _error = error.toString();
      });
    }
  }

  void _click(TapUpDetails details, BoxConstraints constraints) {
    if (_isRemote || (_observed && !_canControl) || _frame == null || _frameWidth <= 0 || _frameHeight <= 0 || constraints.maxWidth <= 0 || constraints.maxHeight <= 0 || _busy) return;
    final scale = (constraints.maxWidth / _frameWidth) < (constraints.maxHeight / _frameHeight)
        ? constraints.maxWidth / _frameWidth : constraints.maxHeight / _frameHeight;
    final displayWidth = _frameWidth * scale;
    final displayHeight = _frameHeight * scale;
    final left = (constraints.maxWidth - displayWidth) / 2;
    final top = (constraints.maxHeight - displayHeight) / 2;
    final x = details.localPosition.dx - left;
    final y = details.localPosition.dy - top;
    if (x < 0 || y < 0 || x >= displayWidth || y >= displayHeight) return;
    _inputAction({'action': 'click', 'x': x / scale, 'y': y / scale});
  }

  Future<void> _control(String action) async {
    if (_isRemote || !_observed || !_running || !_foreground || !widget.active || _controlPending || (_busy && action != 'renew')) return;
    final requestedLeaseId = _leaseId;
    if (action != 'acquire' && requestedLeaseId.isEmpty) { _canControl = false; return; }
    _controlPending = true;
    final sequence = _generation;
    final request = ++_controlRequestGeneration;
    final api = ref.read(backendServiceProvider);
    final conversationId = widget.conversationId;
    final tabId = widget.tabId;
    final targetDeviceId = widget.targetDeviceId;
    final endpoint = _endpoint;
    try {
      final result = await api.post<Map<String, dynamic>>(
        '$endpoint/control', data: {'action': action, if (action != 'acquire') 'leaseId': requestedLeaseId},
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      final responseLeaseId = result?['leaseId'] is String ? result!['leaseId'] as String : '';
      if (!mounted || !widget.active || !_foreground || sequence != _generation || request != _controlRequestGeneration ||
          (action != 'acquire' && _leaseId != requestedLeaseId)) {
        if (action == 'acquire' && responseLeaseId.isNotEmpty) {
          _releaseControlFor(conversationId, tabId, targetDeviceId, leaseId: responseLeaseId, api: api);
        }
        return;
      }
      setState(() {
        _canControl = result?['canControl'] == true;
        _leaseId = _canControl ? responseLeaseId : '';
        _error = null;
      });
    } catch (error) {
      if (mounted && widget.active && _foreground && sequence == _generation && request == _controlRequestGeneration &&
          (action == 'acquire' || _leaseId == requestedLeaseId)) setState(() {
        _canControl = false;
        _leaseId = '';
        _error = '浏览器控制权不可用：$error';
      });
    } finally {
      if (request == _controlRequestGeneration) _controlPending = false;
    }
  }

  void _releaseControl() {
    final token = _leaseId;
    _canControl = false;
    _leaseId = '';
    _releaseControlFor(widget.conversationId, widget.tabId, widget.targetDeviceId, leaseId: token);
  }

  void _releaseControlFor(String conversationId, String tabId, String targetDeviceId, {required String leaseId, BackendServiceApi? api}) {
    if (targetDeviceId.isNotEmpty || leaseId.isEmpty) return;
    final endpoint = '/api/workbench/browser/${Uri.encodeComponent(conversationId)}/${Uri.encodeComponent(tabId)}';
    final client = api ?? ref.read<BackendServiceApi>(backendServiceProvider);
    client.post<Map<String, dynamic>>(
      '$endpoint/control', data: {'action': 'release', 'leaseId': leaseId},
      fromJson: (value) => Map<String, dynamic>.from(value as Map),
    ).then((_) {}, onError: (Object error) {
      print("[original-error] $error");
    });
  }

  void _reconnect() {
    if (!widget.active || !_foreground) return;
    setState(() {
      _explicitlyClosed = false;
      _retryFailures = 0;
      _nextRetryAt = DateTime.fromMillisecondsSinceEpoch(0);
      _error = null;
    });
    _resume();
  }

  Future<void> _close() async {
    _explicitlyClosed = true;
    _generation++;
    _domRequestGeneration++;
    _controlRequestGeneration++;
    _controlPending = false;
    _fetching = false;
    _timer?.cancel();
    setState(() { _running = false; _canControl = false; _leaseId = ''; _frame = null; _domVisible = false; });
    if (_isRemote) return;
    try {
      await ref.read(backendServiceProvider).delete(_endpoint);
    } catch (error) {
      if (mounted) setState(() => _error = error.toString());
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Padding(padding: const EdgeInsets.all(6), child: Row(children: [
        IconButton(tooltip: '后退', onPressed: !_isRemote && _running && !_busy && (!_observed || _canControl) ? () => _action('back') : null, icon: const Icon(Icons.arrow_back)),
        IconButton(tooltip: '前进', onPressed: !_isRemote && _running && !_busy && (!_observed || _canControl) ? () => _action('forward') : null, icon: const Icon(Icons.arrow_forward)),
        Expanded(child: TextField(controller: _address, readOnly: _isRemote || ((_observed || widget.browserRef.isNotEmpty) && !_canControl), keyboardType: TextInputType.url, onSubmitted: (_) => _navigate(), decoration: const InputDecoration(hintText: 'https://example.com', isDense: true, border: OutlineInputBorder()))),
        IconButton(tooltip: _running ? '访问' : '启动 Chromium', onPressed: _isRemote || _busy || (_running && _observed && !_canControl) ? null : _navigate, icon: Icon(_running ? Icons.open_in_browser : Icons.play_arrow)),
        IconButton(tooltip: '刷新', onPressed: !_isRemote && _running && !_busy && (!_observed || _canControl) ? () => _action('reload') : null, icon: const Icon(Icons.refresh)),
      ])),
      Padding(padding: const EdgeInsets.symmetric(horizontal: 8), child: Row(children: [
        Expanded(child: Text(_isRemote ? '远程设备 Chromium · 只读观察' : _observed ? (_canControl ? 'Agent Chromium · 临时控制中' : 'Agent Chromium · 同页只读观察') : _running ? '设备 Chromium · 受控截图' : '浏览器未连接', style: Theme.of(context).textTheme.labelSmall)),
        if (_running && _observed && !_isRemote) TextButton(
          onPressed: _busy ? null : () => _control(_canControl ? 'release' : 'acquire'),
          child: Text(_canControl ? '释放控制' : '申请控制'),
        ),
        if (_explicitlyClosed) TextButton(onPressed: _reconnect, child: const Text('重新连接')),
        TextButton(onPressed: _running ? _readDom : null, child: Text(_domVisible ? '画面' : 'DOM')),
        TextButton(onPressed: _running ? _close : null, child: Text(_observed ? '退出观察' : '结束')),
      ])),
      if (_error != null) Padding(padding: const EdgeInsets.all(8), child: SelectableText(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error))),
      if (_busy) const LinearProgressIndicator(minHeight: 2),
      if (_running && !_isRemote && (!_observed || _canControl)) Padding(padding: const EdgeInsets.symmetric(horizontal: 8), child: Row(children: [
        Expanded(child: TextField(controller: _input, decoration: const InputDecoration(isDense: true, hintText: '向当前焦点输入文字'))),
        TextButton(onPressed: _busy ? null : () { if (_input.text.isNotEmpty) { _inputAction({'action': 'text', 'text': _input.text}); _input.clear(); } }, child: const Text('输入')),
        IconButton(tooltip: '向上滚动', onPressed: _busy ? null : () => _inputAction({'action': 'scroll', 'amount': -420}), icon: const Icon(Icons.arrow_upward)),
        IconButton(tooltip: '向下滚动', onPressed: _busy ? null : () => _inputAction({'action': 'scroll', 'amount': 420}), icon: const Icon(Icons.arrow_downward)),
      ])),
      Expanded(child: _domVisible
          ? SingleChildScrollView(padding: const EdgeInsets.all(10), child: SelectableText(_dom))
          : LayoutBuilder(builder: (context, constraints) => GestureDetector(
              behavior: HitTestBehavior.opaque,
              onTapUp: _running && !_isRemote && (!_observed || _canControl) ? (details) => _click(details, constraints) : null,
              child: Center(child: _frame == null
                  ? const Text('启动浏览器后显示 Chromium 实时截图')
                  : Image.memory(_frame!, fit: BoxFit.contain, gaplessPlayback: true)),
            ))),
    ]);
  }
}
