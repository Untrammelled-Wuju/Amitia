import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../toolbox/presentation/pages/toolbox_file_browser_page.dart';
import '../../../toolbox/presentation/pages/toolbox_workspace_page.dart';
import 'workbench_document_view.dart';
import 'workbench_tasks_view.dart';
import 'workbench_agent_runs_view.dart';
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

  _WorkbenchTab update({String? title, String? uri, bool? pinned}) =>
      _WorkbenchTab(
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
    if (id.isEmpty ||
        !{
          'files',
          'file',
          'git',
          'diff',
          'browser',
          'terminal',
          'tasks',
          'agents',
        }.contains(type))
      return null;
    return _WorkbenchTab(
      id: id,
      type: type,
      title: (data['title']?.toString() ?? type),
      uri: data['uri']?.toString() ?? '',
      workspaceUri: data['workspaceUri']?.toString() ?? '',
      targetDeviceId: data['targetDeviceId']?.toString() ?? '',
      browserRef: data['browserRef']?.toString() ?? '',
      taskId:
          data['taskId'] is String &&
              RegExp(
                r'^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$',
              ).hasMatch(data['taskId'] as String)
          ? data['taskId'] as String
          : '',
      pinned: data['pinned'] == true,
    );
  }
}

class WorkbenchPage extends ConsumerStatefulWidget {
  final String conversationId;
  final String characterId;
  final VoidCallback? onExpand;
  final bool fullscreen;

  const WorkbenchPage({
    super.key,
    required this.conversationId,
    this.characterId = '',
    this.onExpand,
    this.fullscreen = false,
  });

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

  TextEditingController _addressFor(_WorkbenchTab tab) => _addresses
      .putIfAbsent(tab.id, () => TextEditingController(text: tab.uri));

  void _handleRemoteChange() {
    if (WorkbenchMobileEvents.lastChangedConversation ==
        widget.conversationId) {
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
          if (index is int && index >= 0 && index < restored.length)
            requestedIndex = index;
        }
      }
    } catch (error, stackTrace) {
      print("[original-error] $error\n$stackTrace");
      restored.clear();
    }
    if (!mounted ||
        generation != _loadGeneration ||
        epoch != _conversationEpoch ||
        conversation != widget.conversationId)
      return;
    if (restored.isEmpty)
      restored.add(
        const _WorkbenchTab(id: 'initial-files', type: 'files', title: '文件'),
      );
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
    WorkbenchMobileEvents.save(
      widget.conversationId,
      _tabs.map((tab) => tab.toJson()).toList(),
      _index,
    );
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

  void _add(
    String type, {
    String uri = '',
    String workspaceUri = '',
    String title = '',
    String targetDeviceId = '',
  }) {
    if (_tabs.length >= 25) {
      setState(() => _error = '最多同时打开 25 个工作区标签');
      return;
    }
    if (type == 'browser' &&
        _tabs.where((tab) => tab.type == 'browser').length >= 8) {
      setState(() => _error = '最多同时打开 8 个浏览器标签');
      return;
    }
    final existing = _tabs.indexWhere(
      (tab) =>
          tab.type == type &&
          tab.uri == uri &&
          tab.workspaceUri == workspaceUri &&
          tab.targetDeviceId == targetDeviceId,
    );
    if (uri.isNotEmpty && existing >= 0) {
      _select(existing);
      return;
    }
    setState(() {
      final tab = _WorkbenchTab(
        id: 'tab-${_next++}',
        type: type,
        title: title.isNotEmpty
            ? title
            : {
                    'files': '文件',
                    'file': '文件预览',
                    'git': 'Git',
                    'diff': '代码变更',
                    'browser': '浏览器',
                    'terminal': '交互终端',
                    'tasks': '任务状态',
                    'agents': '子智能体',
                  }[type] ??
                  type,
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
        final tab = _WorkbenchTab(
          id: 'tab-${_next++}',
          type: 'files',
          title: '文件',
        );
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
    setState(
      () => _tabs[index] = _tabs[index].update(pinned: !_tabs[index].pinned),
    );
    _save();
  }

  void _closeOtherTabs(String keepId) {
    final keepIndex = _tabs.indexWhere((tab) => tab.id == keepId);
    if (keepIndex < 0) return;
    final keep = _tabs[keepIndex];
    setState(() {
      final retained = _tabs
          .where((tab) => tab.id == keepId || tab.pinned)
          .toList();
      _tabs
        ..clear()
        ..addAll(retained);
      _index = _tabs.indexOf(keep);
    });
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
        ..setNavigationDelegate(
          NavigationDelegate(
            onNavigationRequest: (request) {
              final uri = Uri.tryParse(request.url);
              return (uri != null &&
                      (uri.scheme == 'http' || uri.scheme == 'https'))
                  ? NavigationDecision.navigate
                  : NavigationDecision.prevent;
            },
            onPageFinished: (url) {
              if (!mounted ||
                  epoch != _conversationEpoch ||
                  conversation != widget.conversationId ||
                  resourceEpoch != (_resourceEpochs[tab.id] ?? 0) ||
                  !identical(_controllers[tab.id], controller))
                return;
              final index = _tabs.indexWhere((item) => item.id == tab.id);
              if (index < 0) return;
              final nextUri = Uri.tryParse(url);
              if (nextUri == null ||
                  (nextUri.scheme != 'http' && nextUri.scheme != 'https'))
                return;
              setState(() {
                _tabs[index] = _tabs[index].update(
                  title: nextUri.host,
                  uri: url,
                );
                _addressFor(_tabs[index]).text = url;
              });
              _save();
            },
          ),
        );
      final start = Uri.tryParse(tab.uri);
      if (start != null &&
          (start.scheme == 'http' || start.scheme == 'https') &&
          start.host.isNotEmpty)
        controller.loadRequest(start);
      return controller;
    });
  }

  void _navigate(_WorkbenchTab tab) {
    final address = _addressFor(tab).text.trim();
    final uri = Uri.tryParse(address);
    if (uri == null ||
        (uri.scheme != 'https' && uri.scheme != 'http') ||
        uri.host.isEmpty) {
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
    final scopedConversation = conversation.isEmpty
        ? '__draft__'
        : conversation;
    final resourceEpoch = _resourceEpochs[tab.id] ?? 0;
    if (tab.targetDeviceId.isNotEmpty &&
        (tab.type != 'browser' || tab.browserRef.isEmpty))
      return const Center(child: Text('当前资源暂不支持跨设备访问'));
    if (tab.type == 'files')
      return ToolboxFileBrowserPage(
        embedded: true,
        onOpenFile: (uri, workspace, title) =>
            _add('file', uri: uri, workspaceUri: workspace, title: title),
      );
    if (tab.type == 'git')
      return ToolboxWorkspacePage(
        embedded: true,
        onOpenDiff: (uri, workspace, title) =>
            _add('diff', uri: uri, workspaceUri: workspace, title: title),
      );
    if (tab.type == 'tasks')
      return WorkbenchTasksView(
        conversationId: widget.conversationId,
        characterId: widget.characterId,
        active: _tabs[_index].id == tab.id,
      );
    if (tab.type == 'agents')
      return WorkbenchAgentRunsView(
        conversationId: widget.conversationId,
        characterId: widget.characterId,
        active: _tabs[_index].id == tab.id,
      );
    if (tab.type == 'terminal')
      return WorkbenchTerminalView(
        conversationId: scopedConversation,
        initialWorkspaceUri: tab.workspaceUri,
        taskId: tab.taskId,
        active: _tabs[_index].id == tab.id,
      );
    if (tab.type == 'file' || tab.type == 'diff')
      return WorkbenchDocumentView(
        kind: tab.type,
        title: tab.title,
        resourceUri: tab.uri,
        workspaceUri: tab.workspaceUri,
        targetDeviceId: tab.targetDeviceId,
      );
    if (!_nativePreviewIds.contains(tab.id)) {
      return WorkbenchRemoteBrowserView(
        key: ValueKey('chromium-$conversation-$epoch-${tab.id}-$resourceEpoch'),
        conversationId: scopedConversation,
        tabId: tab.id,
        initialUrl: tab.uri,
        browserRef: tab.browserRef,
        targetDeviceId: tab.targetDeviceId,
        active: _tabs[_index].id == tab.id,
        onUrlChanged: (value) {
          final index = _tabs.indexWhere((item) => item.id == tab.id);
          if (!mounted ||
              index < 0 ||
              epoch != _conversationEpoch ||
              conversation != widget.conversationId ||
              resourceEpoch != (_resourceEpochs[tab.id] ?? 0))
            return;
          final uri = Uri.tryParse(value);
          setState(
            () => _tabs[index] = _tabs[index].update(
              uri: value,
              title: uri?.host ?? tab.title,
            ),
          );
          _save();
        },
      );
    }
    final controller = _browser(tab);
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(8),
          child: Row(
            children: [
              IconButton(
                tooltip: '后退',
                onPressed: () async {
                  if (await controller.canGoBack()) controller.goBack();
                },
                icon: const Icon(Icons.arrow_back),
              ),
              IconButton(
                tooltip: '前进',
                onPressed: () async {
                  if (await controller.canGoForward()) controller.goForward();
                },
                icon: const Icon(Icons.arrow_forward),
              ),
              Expanded(
                child: TextField(
                  controller: _addressFor(tab),
                  keyboardType: TextInputType.url,
                  onSubmitted: (_) => _navigate(tab),
                  decoration: const InputDecoration(
                    isDense: true,
                    hintText: 'https://example.com',
                    border: OutlineInputBorder(),
                  ),
                ),
              ),
              IconButton(
                tooltip: '访问',
                onPressed: () => _navigate(tab),
                icon: const Icon(Icons.open_in_browser),
              ),
              IconButton(
                tooltip: '刷新',
                onPressed: controller.reload,
                icon: const Icon(Icons.refresh),
              ),
            ],
          ),
        ),
        const Padding(
          padding: EdgeInsets.symmetric(horizontal: 12, vertical: 4),
          child: Text(
            '本地 WebView 预览，不与 Chromium Agent 会话共享。',
            style: TextStyle(fontSize: 11),
          ),
        ),
        Expanded(child: WebViewWidget(controller: controller)),
      ],
    );
  }

  IconData _iconForType(String type) {
    switch (type) {
      case 'files':
      case 'file':
        return Icons.folder_outlined;
      case 'git':
      case 'diff':
        return Icons.account_tree_outlined;
      case 'browser':
        return Icons.public;
      case 'terminal':
        return Icons.terminal;
      case 'tasks':
        return Icons.checklist_rtl;
      case 'agents':
        return Icons.hub_outlined;
      default:
        return Icons.tab_outlined;
    }
  }

  Future<void> _showNewTabMenu() async {
    final kind = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      useSafeArea: true,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        top: false,
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            0,
            AppSpacing.lg,
            AppSpacing.xl,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('打开工作区', style: AppTypography.sectionTitle(sheetContext)),
              SizedBox(height: AppSpacing.md),
              _newTabTile(
                sheetContext,
                'files',
                Icons.folder_outlined,
                '文件',
                '浏览工作区文件和文档',
              ),
              _newTabTile(
                sheetContext,
                'git',
                Icons.account_tree_outlined,
                'Git',
                '查看变更、Diff、分支与 Worktree',
              ),
              _newTabTile(
                sheetContext,
                'browser',
                Icons.public,
                '浏览器',
                '启动真实 Chromium 会话',
              ),
              _newTabTile(
                sheetContext,
                'terminal',
                Icons.terminal,
                '终端',
                '在已授权工作区执行命令',
              ),
              _newTabTile(
                sheetContext,
                'tasks',
                Icons.checklist_rtl,
                '任务',
                '查看多 Agent 任务与检查点',
              ),
              _newTabTile(
                sheetContext,
                'agents',
                Icons.hub_outlined,
                '子智能体',
                '查看父子运行、消息与人工接管',
              ),
            ],
          ),
        ),
      ),
    );
    if (kind != null && mounted) _add(kind);
  }

  Widget _newTabTile(
    BuildContext context,
    String kind,
    IconData icon,
    String title,
    String subtitle,
  ) => ListTile(
    contentPadding: EdgeInsets.zero,
    minTileHeight: 52,
    leading: Icon(icon, size: 20, color: context.textSecondary),
    title: Text(title, style: AppTypography.body(context)),
    subtitle: Text(subtitle, style: AppTypography.caption(context)),
    onTap: () => Navigator.of(context).pop(kind),
  );

  Future<void> _showTabMenu(int index) async {
    if (index < 0 || index >= _tabs.length) return;
    final tab = _tabs[index];
    final action = await showModalBottomSheet<String>(
      context: context,
      showDragHandle: true,
      useSafeArea: true,
      isScrollControlled: true,
      builder: (sheetContext) => SafeArea(
        top: false,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ListTile(
                leading: Icon(_iconForType(tab.type), size: 20),
                title: Text(
                  tab.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                subtitle: Text(
                  tab.uri.isEmpty ? tab.type : tab.uri,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              const Divider(height: 1),
              ListTile(
                leading: Icon(
                  tab.pinned ? Icons.push_pin_outlined : Icons.push_pin,
                  size: 20,
                ),
                title: Text(tab.pinned ? '取消固定' : '固定标签'),
                onTap: () => Navigator.of(sheetContext).pop('pin'),
              ),
              if (!tab.pinned)
                ListTile(
                  leading: const Icon(Icons.close, size: 20),
                  title: const Text('关闭标签'),
                  onTap: () => Navigator.of(sheetContext).pop('close'),
                ),
              if (_tabs.length > 1)
                ListTile(
                  leading: const Icon(
                    Icons.close_fullscreen_outlined,
                    size: 20,
                  ),
                  title: const Text('关闭其他标签'),
                  onTap: () => Navigator.of(sheetContext).pop('others'),
                ),
            ],
          ),
        ),
      ),
    );
    if (!mounted) return;
    switch (action) {
      case 'pin':
        _togglePinned(tab.id);
      case 'close':
        _remove(index);
      case 'others':
        _closeOtherTabs(tab.id);
    }
  }

  Widget _workbenchHeader(BuildContext context) {
    final tab = _tabs[_index];
    return SizedBox(
      height: 52,
      child: Row(
        children: [
          if (widget.fullscreen || Navigator.of(context).canPop())
            IconButton(
              tooltip: '关闭工作区',
              visualDensity: VisualDensity.compact,
              onPressed: () => Navigator.of(context).maybePop(),
              icon: Icon(
                widget.fullscreen ? Icons.arrow_back : Icons.close,
                size: 20,
              ),
            )
          else
            const SizedBox(width: 14),
          Expanded(
            child: Text(
              '工作区',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: AppTypography.cardTitle(
                context,
              ).copyWith(fontWeight: FontWeight.w600),
            ),
          ),
          if (tab.type == 'browser' && tab.targetDeviceId.isEmpty)
            IconButton(
              tooltip: _nativePreviewIds.contains(tab.id)
                  ? '切换到 Chromium'
                  : '切换到本地 WebView',
              visualDensity: VisualDensity.compact,
              onPressed: () => setState(() {
                if (!_nativePreviewIds.add(tab.id))
                  _nativePreviewIds.remove(tab.id);
              }),
              icon: Icon(
                _nativePreviewIds.contains(tab.id)
                    ? Icons.desktop_windows_outlined
                    : Icons.phone_android_outlined,
                size: 19,
              ),
            ),
          IconButton(
            tooltip: '新建标签',
            visualDensity: VisualDensity.compact,
            onPressed: _showNewTabMenu,
            icon: const Icon(Icons.add, size: 20),
          ),
          if (widget.onExpand != null)
            IconButton(
              tooltip: '全屏工作区',
              visualDensity: VisualDensity.compact,
              onPressed: widget.onExpand,
              icon: const Icon(Icons.open_in_full, size: 19),
            ),
          PopupMenuButton<String>(
            tooltip: '更多',
            icon: const Icon(Icons.more_horiz, size: 20),
            onSelected: (value) {
              switch (value) {
                case 'pin':
                  _togglePinned(tab.id);
                case 'close':
                  _remove(_index);
                case 'close-others':
                  _closeOtherTabs(tab.id);
                case 'close-unpinned':
                  setState(() {
                    final active = _tabs[_index];
                    _tabs.removeWhere((item) => !item.pinned);
                    if (_tabs.isEmpty)
                      _tabs.add(
                        _WorkbenchTab(
                          id: 'tab-${_next++}',
                          type: 'files',
                          title: '文件',
                        ),
                      );
                    _index = _tabs.contains(active) ? _tabs.indexOf(active) : 0;
                  });
                  _save();
              }
            },
            itemBuilder: (_) => [
              PopupMenuItem(
                value: 'pin',
                child: Text(tab.pinned ? '取消固定' : '固定标签'),
              ),
              if (!tab.pinned)
                const PopupMenuItem(value: 'close', child: Text('关闭当前标签')),
              if (_tabs.length > 1)
                const PopupMenuItem(
                  value: 'close-others',
                  child: Text('关闭其他标签'),
                ),
              const PopupMenuItem(
                value: 'close-unpinned',
                child: Text('关闭未固定标签'),
              ),
            ],
          ),
          const SizedBox(width: 4),
        ],
      ),
    );
  }

  Widget _tabStrip(BuildContext context) => SizedBox(
    height: 42,
    child: ListView.separated(
      scrollDirection: Axis.horizontal,
      padding: const EdgeInsets.symmetric(horizontal: 8),
      itemCount: _tabs.length,
      separatorBuilder: (_, _) => const SizedBox(width: 4),
      itemBuilder: (context, index) {
        final item = _tabs[index];
        final active = index == _index;
        return Material(
          color: active ? context.surfaceSecondary : Colors.transparent,
          borderRadius: BorderRadius.circular(7),
          child: InkWell(
            borderRadius: BorderRadius.circular(7),
            onTap: () => _select(index),
            onLongPress: () => _showTabMenu(index),
            child: Container(
              constraints: const BoxConstraints(minWidth: 88, maxWidth: 210),
              padding: const EdgeInsets.only(left: 10, right: 3),
              decoration: BoxDecoration(
                border: Border(
                  bottom: BorderSide(
                    color: active ? context.accentPrimary : Colors.transparent,
                    width: 1.5,
                  ),
                ),
              ),
              child: Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(
                    _iconForType(item.type),
                    size: 15,
                    color: active ? context.textPrimary : context.textSecondary,
                  ),
                  const SizedBox(width: 6),
                  Flexible(
                    child: Text(
                      item.title,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: AppTypography.label(context).copyWith(
                        color: active
                            ? context.textPrimary
                            : context.textSecondary,
                      ),
                    ),
                  ),
                  if (item.pinned)
                    Padding(
                      padding: const EdgeInsets.only(left: 4),
                      child: Icon(
                        Icons.push_pin,
                        size: 11,
                        color: context.textTertiary,
                      ),
                    ),
                  if (!item.pinned)
                    InkWell(
                      borderRadius: BorderRadius.circular(4),
                      onTap: () => _remove(index),
                      child: Padding(
                        padding: const EdgeInsets.all(5),
                        child: Icon(
                          Icons.close,
                          size: 14,
                          color: context.textTertiary,
                        ),
                      ),
                    )
                  else
                    const SizedBox(width: 5),
                ],
              ),
            ),
          ),
        );
      },
    ),
  );

  @override
  Widget build(BuildContext context) {
    if (!_loaded)
      return ColoredBox(
        color: context.backgroundPrimary,
        child: const Center(child: CircularProgressIndicator()),
      );
    return Material(
      color: context.backgroundPrimary,
      child: SafeArea(
        top: widget.fullscreen,
        bottom: false,
        child: Column(
          children: [
            Container(
              decoration: BoxDecoration(
                color: context.backgroundPrimary,
                border: Border(
                  bottom: BorderSide(color: context.borderSecondary, width: .5),
                ),
              ),
              child: _workbenchHeader(context),
            ),
            Container(
              decoration: BoxDecoration(
                color: context.backgroundPrimary,
                border: Border(
                  bottom: BorderSide(color: context.borderSecondary, width: .5),
                ),
              ),
              child: _tabStrip(context),
            ),
            if (_error != null)
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 7,
                ),
                child: SelectableText(
                  _error!,
                  style: AppTypography.caption(
                    context,
                  ).copyWith(color: context.error),
                ),
              ),
            Expanded(
              child: IndexedStack(
                index: _index,
                children: _tabs
                    .map(
                      (tab) => KeyedSubtree(
                        key: ValueKey(
                          '${widget.conversationId}-$_conversationEpoch-${tab.id}-${_resourceEpochs[tab.id] ?? 0}',
                        ),
                        child: _visited.contains(tab.id)
                            ? _view(tab)
                            : const SizedBox.shrink(),
                      ),
                    )
                    .toList(),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
