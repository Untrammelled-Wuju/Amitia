import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../conversation/rendering/code/amitia_diff_block.dart';
import '../../runtime/workbench_document_chunks.dart';

class WorkbenchDocumentView extends ConsumerStatefulWidget {
  final String kind;
  final String title;
  final String resourceUri;
  final String workspaceUri;
  final String targetDeviceId;

  const WorkbenchDocumentView({
    super.key,
    required this.kind,
    required this.title,
    required this.resourceUri,
    required this.workspaceUri,
    this.targetDeviceId = '',
  });

  @override
  ConsumerState<WorkbenchDocumentView> createState() => _WorkbenchDocumentViewState();
}

class _WorkbenchDocumentViewState extends ConsumerState<WorkbenchDocumentView> {
  bool _loading = false;
  bool _isText = true;
  String? _error;
  String _content = '';
  int _offset = 0;
  WorkbenchDocumentContent? _fileContent;
  bool _hasMore = false;
  Map<String, dynamic>? _stats;
  int _generation = 0;

  @override
  void initState() {
    super.initState();
    Future<void>.microtask(() => _load(false));
  }

  @override
  void didUpdateWidget(covariant WorkbenchDocumentView oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.kind == widget.kind && oldWidget.resourceUri == widget.resourceUri &&
        oldWidget.workspaceUri == widget.workspaceUri && oldWidget.targetDeviceId == widget.targetDeviceId) return;
    _generation++;
    _loading = false;
    _isText = true;
    _error = null;
    _content = '';
    _fileContent = null;
    _offset = 0;
    _hasMore = false;
    _stats = null;
    Future<void>.microtask(() => _load(false));
  }

  @override
  void dispose() {
    _generation++;
    super.dispose();
  }

  Future<void> _load(bool append) async {
    if (widget.targetDeviceId.isNotEmpty) {
      if (mounted) setState(() {
        _loading = false;
        _error = '目标设备工作区尚未授权，已阻止向本机发出请求';
      });
      return;
    }
    final seq = ++_generation;
    if (mounted) setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      if (widget.kind == 'file') {
        final start = append ? _offset : 0;
        final response = await api.get<Map<String, dynamic>>(
          '/api/workspaces/read',
          queryParameters: {'uri': widget.resourceUri, 'offset': start, 'maxBytes': 262144, 'rawBytes': 1},
          fromJson: (input) => Map<String, dynamic>.from(input as Map),
        );
        if (!mounted || seq != _generation) return;
        final chunk = WorkbenchDocumentChunk.parse(response, start);
        final updated = WorkbenchDocumentContent.apply(append ? _fileContent : null, chunk, start);
        setState(() {
          _fileContent = updated;
          _isText = updated.isText;
          _content = updated.text;
          _offset = updated.offset;
          _hasMore = updated.hasMore;
          _stats = null;
        });
      } else {
        final response = await api.post<Map<String, dynamic>>(
          '/api/workspaces/git/diff',
          data: {'workspaceUri': widget.workspaceUri, 'mode': 'worktree', 'paths': [widget.resourceUri], 'maxBytes': 2097152},
          fromJson: (input) => Map<String, dynamic>.from(input as Map),
        );
        if (!mounted || seq != _generation) return;
        final files = (response?['files'] as List<dynamic>? ?? const []).whereType<Map>();
        setState(() {
          _stats = response?['stats'] is Map ? Map<String, dynamic>.from(response!['stats'] as Map) : null;
          _content = files.map((file) => (file['patch'] ?? '[无文本变更或二进制文件]').toString()).join('\n');
          _hasMore = false;
          _fileContent = null;
        });
      }
    } catch (error) {
      if (mounted && seq == _generation) setState(() => _error = error.toString());
    } finally {
      if (mounted && seq == _generation) setState(() => _loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Column(children: [
      Padding(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
        child: Row(children: [
          Expanded(child: Text(widget.title, maxLines: 1, overflow: TextOverflow.ellipsis)),
          IconButton(tooltip: '刷新', onPressed: _loading ? null : () => _load(false), icon: const Icon(Icons.refresh)),
        ]),
      ),
      if (_loading) const LinearProgressIndicator(minHeight: 2),
      if (_error != null) Padding(padding: const EdgeInsets.all(12), child: SelectableText(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error))),
      if (widget.kind == 'diff' && _stats != null)
        Padding(padding: const EdgeInsets.all(8), child: Text('${_stats!['filesChanged'] ?? 0} 个文件 · +${_stats!['insertions'] ?? 0} / -${_stats!['deletions'] ?? 0}')),
      if (!_isText) const Padding(padding: EdgeInsets.all(12), child: Text('当前文件不支持文本预览')),
      Expanded(child: widget.kind == 'diff'
          ? SingleChildScrollView(padding: const EdgeInsets.all(12), child: AmitiaDiffBlock(diff: _content, filename: widget.title))
          : SingleChildScrollView(padding: const EdgeInsets.all(12), child: SingleChildScrollView(scrollDirection: Axis.horizontal, child: SelectableText(_content, style: const TextStyle(fontFamily: 'monospace', fontSize: 12))))),
      if (_hasMore && !_loading) TextButton(onPressed: () => _load(true), child: const Text('加载下一部分')),
    ]);
  }
}
