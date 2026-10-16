import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../toolbox/presentation/pages/toolbox_file_browser_page.dart';
import '../../../toolbox/presentation/pages/toolbox_workspace_page.dart';
import 'workbench_document_view.dart';
import 'workbench_tasks_view.dart';
import 'workbench_terminal_view.dart';
import 'workbench_remote_browser_view.dart';
import '../../runtime/workbench_mobile_events.dart';

class _WorkbenchTab {
  final String id;
  final String type;
  final String title;
  final String uri;
  final String workspaceUri;
  final String targetDeviceId;
  final String browserRef;
  final String taskId;
  final bool pinned;

  const _WorkbenchTab({
    required this.id,
    required this.type,
    required this.title,
    this.uri = '',
    this.workspaceUri = '',
    this.targetDeviceId = '',
    this.browserRef = '',
    this.taskId = '',
    this.pinned = false,
  });

  bool hasSameResource(_WorkbenchTab other) =>
      id == other.id &&
      type == other.type &&
      uri == other.uri &&
      workspaceUri == other.workspaceUri &&
      targetDeviceId == other.targetDeviceId &&
      browserRef == other.browserRef &&
      taskId == other.taskId;

  _WorkbenchTab update({String? title, String? uri, bool? pinned}) => _WorkbenchTab(
    id: id,
    type: type,
    title: title ?? this.title,
    uri: uri ?? this.uri,
    workspaceUri: workspaceUri,
    targetDeviceId: targetDeviceId,
    browserRef: browserRef,
    taskId: taskId,
    pinned: pinned ?? this.pinned,
  );

  Map<String, dynamic> toJson() => {
    'id': id,
    'type': type,
    'title': title,
    'uri': uri,
    'workspaceUri': workspaceUri,
    'targetDeviceId': targetDeviceId,
    'browserRef': browserRef,
    'taskId': taskId,
    'pinned': pinned,
  };

  static _WorkbenchTab? fromJson(Object? data) {
    if (data is! Map) return null;
    final type = data['type']?.toString() ?? '';
    final id = data['id']?.toString() ?? '';
    if (id.isEmpty || !{'files', 'file', 'git', 'diff', 'browser', 'terminal', 'tasks'}.contains(type)) return null;
    return _WorkbenchTab(
      id: id,
      type: type,
      title: (data['title']?.toString() ?? type),
      uri: data['uri']?.toString() ?? '',
      workspaceUri: data['workspaceUri']?.toString() ?? '',
      targetDeviceId: data['targetDeviceId']?.toString() ?? '',
      browserRef: data['browserRef']?.toString() ?? '',
      taskId: data['taskId'] is String && RegExp(r'^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$').hasMatch(data['taskId'] as String) ? data['taskId'] as String : '',
      pinned: data['pinned'] == true,
    );
  }
}

class WorkbenchPage extends ConsumerStatefulWidget {
  final String conversationId;
  final String characterId;
  final VoidCallback? onExpand;

  const WorkbenchPage({super.key, required this.conversationId, this.characterId = '', this.onExpand});

  @override
  ConsumerState<WorkbenchPage> createState() => _WorkbenchPageState();
}

class _WorkbenchPageState extends ConsumerState<WorkbenchPage> {
  final List<_WorkbenchTab> _tabs = [];
  final Map<String, WebViewController> _controllers = {};
  final Map<String, TextEditingController> _addresses = {};
  final Set<String> _visited = {};
  final Set<String> _nativePreviewIds = {};
  final Map<String, int> _resourceEpochs = {};
  int _resourceSerial = 0;
  int _index = 0;
  int _next = 0;
  bool _loaded = false;
  int _loadGeneration = 0;
  int _conversationEpoch = 0;
  String? _error;


  @override
  void initState() {
    super.initState();
    WorkbenchMobileEvents.revision.addListener(_handleRemoteChange);
    _load();
  }

  @override
  void didUpdateWidget(covariant WorkbenchPage oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.conversationId == widget.conversationId) return;
    _conversationEpoch++;
    _loadGeneration++;
    for (final controller in _addresses.values) {
      controller.dispose();
    }
    _addresses.clear();
    _controllers.clear();
    _visited.clear();
    _nativePreviewIds.clear();
    _resourceEpochs.clear();
    _tabs.clear();
    _index = 0;
    _loaded = false;
    _error = null;
    _load();
  }

  @override
  void dispose() {
    _loadGeneration++;
    _conversationEpoch++;
    WorkbenchMobileEvents.revision.removeListener(_handleRemoteChange);
    for (final controller in _addresses.values) {
      controller.dispose();
    }
    super.dispose();
  }

  TextEditingController _addressFor(_WorkbenchTab tab) => _addresses.putIfAbsent(tab.id, () => TextEditingController(text: tab.uri));

  void _handleRemoteChange() {
    if (WorkbenchMobileEvents.lastChangedConversation == widget.conversationId) {
      _load();
    }
  }

  Future<void> _load() async {
    final generation = ++_loadGeneration;
    final conversation = widget.conversationId;
    final epoch = _conversationEpoch;
    final restored = <_WorkbenchTab>[];
    var requestedIndex = 0;
    try {
      final raw = await WorkbenchMobileEvents.read(conversation);
      if (raw != null) {
        final parsed = jsonDecode(raw);
        if (parsed is Map) {
          final rows = parsed['tabs'];
          if (rows is List) {
            final ids = <String>{};
            for (final row in rows.take(25)) {
              final tab = _WorkbenchTab.fromJson(row);
              if (tab != null && ids.add(tab.id)) restored.add(tab);
            }
          }
          final index = parsed['index'];
          if (index is int && index >= 0 && index < restored.length) requestedIndex = index;
        }
      }
    } catch (_) {
      restored.clear();
    }
    if (!mounted || generation != _loadGeneration || epoch != _conversationEpoch || conversation != widget.conversationId) return;
    if (restored.isEmpty) restored.add(const _WorkbenchTab(id: 'initial-files', type: 'files', title: '文件'));
    final ids = restored.map((tab) => tab.id).toSet();
    final oldTabs = {for (final tab in _tabs) tab.id: tab};
    for (final tab in restored) {
      final previous = oldTabs[tab.id];
      if (previous == null) {
        _resourceEpochs[tab.id] = ++_resourceSerial;
        continue;
      }
      if (previous.hasSameResource(tab)) continue;
      _resourceEpochs[tab.id] = ++_resourceSerial;
      _addresses.remove(tab.id)?.dispose();
      _controllers.remove(tab.id);
      _visited.remove(tab.id);
      _nativePreviewIds.remove(tab.id);
    }
    _resourceEpochs.removeWhere((id, _) => !ids.contains(id));
    for (final id in _addresses.keys.toList()) {
      if (!ids.contains(id)) _addresses.remove(id)?.dispose();
    }
    _controllers.removeWhere((id, _) => !ids.contains(id));
    _visited.removeWhere((id) => !ids.contains(id));
    _nativePreviewIds.removeWhere((id) => !ids.contains(id));
    setState(() {
      _tabs
        ..clear()
        ..addAll(restored);
      _index = requestedIndex;
      _next = DateTime.now().microsecondsSinceEpoch;
      _visited.add(_tabs[_index].id);
      _loaded = true;
    });
  }

  void _save() {
    WorkbenchMobileEvents.save(widget.conversationId, _tabs.map((tab) => tab.toJson()).toList(), _index);
  }

  void _select(int index) {
    if (index < 0 || index >= _tabs.length) return;
    setState(() {
      _index = index;
      _visited.add(_tabs[index].id);
      _error = null;
    });
    _save();
  }

  void _add(String type, {String uri = '', String workspaceUri = '', String title = '', String targetDeviceId = ''}) {
    if (_tabs.length >= 25) {
      setState(() => _error = '最多同时打开 25 个工作区标签');
      return;
    }
    if (type == 'browser' && _tabs.where((tab) => tab.type == 'browser').length >= 8) {
      setState(() => _error = '最多同时打开 8 个浏览器标签');
      return;
    }
    final existing = _tabs.indexWhere((tab) => tab.type == type && tab.uri == uri && tab.workspaceUri == workspaceUri && tab.targetDeviceId == targetDeviceId);
    if (uri.isNotEmpty && existing >= 0) {
      _select(existing);
      return;
    }
    setState(() {
      final tab = _WorkbenchTab(
        id: 'tab-${_next++}',
        type: type,
        title: title.isNotEmpty ? title : {'files': '文件', 'file': '文件预览', 'git': 'Git', 'diff': '代码变更', 'browser': '浏览器', 'terminal': '交互终端', 'tasks': '任务状态'}[type] ?? type,
        uri: uri,
        workspaceUri: workspaceUri,
        targetDeviceId: targetDeviceId,
      );
      _tabs.add(tab);
      _resourceEpochs[tab.id] = ++_resourceSerial;
      _index = _tabs.length - 1;
      _visited.add(tab.id);
      _error = null;
    });
    _save();
  }

  void _remove(int index) {
    if (index < 0 || index >= _tabs.length || _tabs[index].pinned) return;
    final removed = _tabs[index];
    setState(() {
      _tabs.removeAt(index);
      _controllers.remove(removed.id);
      _visited.remove(removed.id);
      _nativePreviewIds.remove(removed.id);
      _resourceEpochs.remove(removed.id);
      _addresses.remove(removed.id)?.dispose();
      if (index < _index) _index -= 1;
      if (_tabs.isEmpty) {
        final tab = _WorkbenchTab(id: 'tab-${_next++}', type: 'files', title: '文件');
        _tabs.add(tab);
        _resourceEpochs[tab.id] = ++_resourceSerial;
      }
      if (_index >= _tabs.length) _index = _tabs.length - 1;
      if (_index < 0) _index = 0;
      _visited.add(_tabs[_index].id);
    });
    _save();
  }

  void _togglePinned(String id) {
    final index = _tabs.indexWhere((tab) => tab.id == id);
    if (index < 0) return;
    setState(() => _tabs[index] = _tabs[index].update(pinned: !_tabs[index].pinned));
    _save();
  }

  WebViewController _browser(_WorkbenchTab tab) {
    final epoch = _conversationEpoch;
    final conversation = widget.conversationId;
    return _controllers.putIfAbsent(tab.id, () {
      final resourceEpoch = _resourceEpochs[tab.id] ?? 0;
      late final WebViewController controller;
      controller = WebViewController()
        ..setJavaScriptMode(JavaScriptMode.unrestricted)
        ..setNavigationDelegate(NavigationDelegate(
          onNavigationRequest: (request) {
            final uri = Uri.tryParse(request.url);
            return (uri != null && (uri.scheme == 'http' || uri.scheme == 'https'))
                ? NavigationDecision.navigate : NavigationDecision.prevent;
          },
          onPageFinished: (url) {
            if (!mounted || epoch != _conversationEpoch || conversation != widget.conversationId || resourceEpoch != (_resourceEpochs[tab.id] ?? 0) || !identical(_controllers[tab.id], controller)) return;
            final index = _tabs.indexWhere((item) => item.id == tab.id);
            if (index < 0) return;
            final nextUri = Uri.tryParse(url);
            if (nextUri == null || (nextUri.scheme != 'http' && nextUri.scheme != 'https')) return;
            setState(() {
              _tabs[index] = _tabs[index].update(title: nextUri.host, uri: url);
              _addressFor(_tabs[index]).text = url;
            });
            _save();
          },
        ));
      final start = Uri.tryParse(tab.uri);
      if (start != null && (start.scheme == 'http' || start.scheme == 'https') && start.host.isNotEmpty) controller.loadRequest(start);
      return controller;
    });
  }

  void _navigate(_WorkbenchTab tab) {
    final address = _addressFor(tab).text.trim();
    final uri = Uri.tryParse(address);
    if (uri == null || (uri.scheme != 'https' && uri.scheme != 'http') || uri.host.isEmpty) {
      setState(() => _error = '请输入有效的 HTTP(S) 网页地址');
      return;
    }
    final index = _tabs.indexWhere((item) => item.id == tab.id);
    if (index < 0) return;
    setState(() {
      _tabs[index] = _tabs[index].update(title: uri.host, uri: uri.toString());
      _error = null;
    });
    _browser(_tabs[index]).loadRequest(uri);
    _save();
  }

  Widget _view(_WorkbenchTab tab) {
    final epoch = _conversationEpoch;
    final conversation = widget.conversationId;
    final resourceEpoch = _resourceEpochs[tab.id] ?? 0;
    if (tab.targetDeviceId.isNotEmpty && (tab.type != 'browser' || tab.browserRef.isEmpty)) return const Center(child: Text('当前资源暂不支持跨设备访问'));
    if (tab.type == 'files') return ToolboxFileBrowserPage(
      embedded: true,
      onOpenFile: (uri, workspace, title) => _add('file', uri: uri, workspaceUri: workspace, title: title),
    );
    if (tab.type == 'git') return ToolboxWorkspacePage(
      embedded: true,
      onOpenDiff: (uri, workspace, title) => _add('diff', uri: uri, workspaceUri: workspace, title: title),
    );
    if (tab.type == 'tasks') return WorkbenchTasksView(conversationId: widget.conversationId, characterId: widget.characterId, active: _tabs[_index].id == tab.id);
    if (tab.type == 'terminal') return WorkbenchTerminalView(
      conversationId: widget.conversationId,
      initialWorkspaceUri: tab.workspaceUri,
      taskId: tab.taskId,
      active: _tabs[_index].id == tab.id,
    );
    if (tab.type == 'file' || tab.type == 'diff') return WorkbenchDocumentView(
      kind: tab.type,
      title: tab.title,
      resourceUri: tab.uri,
      workspaceUri: tab.workspaceUri,
      targetDeviceId: tab.targetDeviceId,
    );
    if (!_nativePreviewIds.contains(tab.id)) {
      return WorkbenchRemoteBrowserView(
        key: ValueKey('chromium-$conversation-$epoch-${tab.id}-$resourceEpoch'),
        conversationId: widget.conversationId,
        tabId: tab.id,
        initialUrl: tab.uri,
        browserRef: tab.browserRef,
        targetDeviceId: tab.targetDeviceId,
        active: _tabs[_index].id == tab.id,
        onUrlChanged: (value) {
          final index = _tabs.indexWhere((item) => item.id == tab.id);
          if (!mounted || index < 0 || epoch != _conversationEpoch || conversation != widget.conversationId || resourceEpoch != (_resourceEpochs[tab.id] ?? 0)) return;
          final uri = Uri.tryParse(value);
          setState(() => _tabs[index] = _tabs[index].update(uri: value, title: uri?.host ?? tab.title));
          _save();
        },
      );
    }
    final controller = _browser(tab);
    return Column(children: [
      Padding(
        padding: const EdgeInsets.all(8),
        child: Row(children: [
          IconButton(tooltip: '后退', onPressed: () async { if (await controller.canGoBack()) controller.goBack(); }, icon: const Icon(Icons.arrow_back)),
          IconButton(tooltip: '前进', onPressed: () async { if (await controller.canGoForward()) controller.goForward(); }, icon: const Icon(Icons.arrow_forward)),
          Expanded(child: TextField(controller: _addressFor(tab), keyboardType: TextInputType.url, onSubmitted: (_) => _navigate(tab), decoration: const InputDecoration(isDense: true, hintText: 'https://example.com', border: OutlineInputBorder()))),
          IconButton(tooltip: '访问', onPressed: () => _navigate(tab), icon: const Icon(Icons.open_in_browser)),
          IconButton(tooltip: '刷新', onPressed: controller.reload, icon: const Icon(Icons.refresh)),
        ]),
      ),
      const Padding(
        padding: EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        child: Text('本地 WebView 预览，不与 Chromium Agent 会话共享。', style: TextStyle(fontSize: 11)),
      ),
      Expanded(child: WebViewWidget(controller: controller)),
    ]);
  }

  @override
  Widget build(BuildContext context) {
    if (!_loaded) return const Scaffold(body: Center(child: CircularProgressIndicator()));
    return Scaffold(
      appBar: AppBar(
        title: const Text('工作区'),
        actions: [
          if (widget.onExpand != null) IconButton(tooltip: '全屏工作区', onPressed: widget.onExpand, icon: const Icon(Icons.open_in_full)),
          PopupMenuButton<String>(
            tooltip: '新建标签',
            icon: const Icon(Icons.add),
            onSelected: (kind) => _add(kind),
            itemBuilder: (_) => const [
              PopupMenuItem(value: 'files', child: Text('文件浏览')),
              PopupMenuItem(value: 'git', child: Text('Git 工作区')),
              PopupMenuItem(value: 'browser', child: Text('网页浏览器')),
              PopupMenuItem(value: 'terminal', child: Text('交互终端')),
              PopupMenuItem(value: 'tasks', child: Text('任务状态')),
            ],
          ),
          if (_tabs[_index].type == 'browser' && _tabs[_index].targetDeviceId.isEmpty) IconButton(
            tooltip: _nativePreviewIds.contains(_tabs[_index].id) ? '切换到 Chromium' : '切换到本地 WebView',
            onPressed: () => setState(() {
              final id = _tabs[_index].id;
              if (!_nativePreviewIds.add(id)) _nativePreviewIds.remove(id);
            }),
            icon: Icon(_nativePreviewIds.contains(_tabs[_index].id) ? Icons.desktop_windows : Icons.phone_android),
          ),
          IconButton(tooltip: '固定或取消固定标签', onPressed: () => _togglePinned(_tabs[_index].id), icon: Icon(_tabs[_index].pinned ? Icons.push_pin : Icons.push_pin_outlined)),
        ],
      ),
      body: Column(children: [
        SizedBox(height: 46, child: ListView.builder(
          scrollDirection: Axis.horizontal,
          itemCount: _tabs.length,
          itemBuilder: (context, index) {
            final item = _tabs[index];
            return Padding(
              padding: const EdgeInsets.symmetric(horizontal: 3, vertical: 4),
              child: Material(
                color: index == _index ? Theme.of(context).colorScheme.primaryContainer : Theme.of(context).colorScheme.surfaceContainer,
                borderRadius: BorderRadius.circular(9),
                child: Row(mainAxisSize: MainAxisSize.min, children: [
                  InkWell(onTap: () => _select(index), child: Padding(padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 7), child: Text(item.title, maxLines: 1))),
                  if (item.pinned) const Icon(Icons.push_pin, size: 12),
                  if (!item.pinned) InkWell(onTap: () => _remove(index), child: const Padding(padding: EdgeInsets.only(right: 8), child: Icon(Icons.close, size: 16))),
                ]),
              ),
            );
          },
        )),
        if (_error != null) Padding(padding: const EdgeInsets.all(8), child: Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error))),
        Expanded(child: IndexedStack(index: _index, children: _tabs.map((tab) => KeyedSubtree(
          key: ValueKey('${widget.conversationId}-$_conversationEpoch-${tab.id}-${_resourceEpochs[tab.id] ?? 0}'),
          child: _visited.contains(tab.id) ? _view(tab) : const SizedBox.shrink(),
        )).toList())),
      ]),
    );
  }
}
