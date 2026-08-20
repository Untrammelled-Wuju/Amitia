import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class _WorkspaceMount {
  final String name;
  final String rootUri;
  final bool available;

  const _WorkspaceMount({
    required this.name,
    required this.rootUri,
    required this.available,
  });
}

class _FileItem {
  final String uri;
  final String name;
  final int? sizeBytes;
  final String modifiedAt;
  final bool isFolder;
  final bool readable;

  const _FileItem({
    required this.uri,
    required this.name,
    required this.sizeBytes,
    required this.modifiedAt,
    required this.isFolder,
    required this.readable,
  });
}

class ToolboxFileBrowserPage extends ConsumerStatefulWidget {
  const ToolboxFileBrowserPage({super.key});

  @override
  ConsumerState<ToolboxFileBrowserPage> createState() => _ToolboxFileBrowserPageState();
}

class _ToolboxFileBrowserPageState extends ConsumerState<ToolboxFileBrowserPage> {
  List<_WorkspaceMount> _mounts = const [];
  List<_FileItem> _files = const [];
  final List<String> _history = [];
  String? _currentUri;
  String? _currentMountName;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadMounts();
  }

  IconData _iconForName(String name, bool folder) {
    if (folder) return Icons.folder_outlined;
    final n = name.toLowerCase();
    if (n.endsWith('.pdf')) return Icons.picture_as_pdf_outlined;
    if (n.endsWith('.doc') || n.endsWith('.docx')) return Icons.description_outlined;
    if (n.endsWith('.png') || n.endsWith('.jpg') || n.endsWith('.jpeg') || n.endsWith('.gif') || n.endsWith('.webp')) {
      return Icons.image_outlined;
    }
    if (n.endsWith('.m4a') || n.endsWith('.mp3') || n.endsWith('.wav') || n.endsWith('.flac')) {
      return Icons.music_note_outlined;
    }
    if (n.endsWith('.json') || n.endsWith('.yaml') || n.endsWith('.xml') || n.endsWith('.toml')) {
      return Icons.code;
    }
    if (n.endsWith('.zip') || n.endsWith('.tar') || n.endsWith('.gz') || n.endsWith('.rar')) {
      return Icons.archive_outlined;
    }
    return Icons.insert_drive_file_outlined;
  }

  String _formatSize(int? value) {
    if (value == null) return '—';
    if (value < 1024) return '$value B';
    if (value < 1024 * 1024) return '${(value / 1024).toStringAsFixed(1)} KB';
    if (value < 1024 * 1024 * 1024) return '${(value / 1024 / 1024).toStringAsFixed(1)} MB';
    return '${(value / 1024 / 1024 / 1024).toStringAsFixed(1)} GB';
  }

  Future<void> _loadMounts() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final raw = await api.get<List<dynamic>>('/api/workspaces') ?? const [];
      final mounts = raw.whereType<Map>().map((entry) {
        final m = Map<String, dynamic>.from(entry);
        return _WorkspaceMount(
          name: (m['name'] ?? m['id'] ?? 'Workspace').toString(),
          rootUri: (m['rootUri'] ?? '').toString(),
          available: m['available'] == true,
        );
      }).where((m) => m.available && m.rootUri.isNotEmpty).toList();
      if (!mounted) return;
      setState(() { _mounts = mounts; });
      if (mounts.isEmpty) {
        setState(() { _files = const []; _loading = false; });
        return;
      }
      await _openMount(mounts.first);
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<void> _openMount(_WorkspaceMount mount) async {
    _history.clear();
    _currentMountName = mount.name;
    await _openUri(mount.rootUri, pushHistory: false);
  }

  Future<void> _openUri(String uri, {bool pushHistory = true}) async {
    if (uri.isEmpty) return;
    final previous = _currentUri;
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final resp = await api.get<Map<String, dynamic>>(
        '/api/workspaces/list',
        queryParameters: {'uri': uri, 'limit': 500},
        fromJson: (e) => Map<String, dynamic>.from(e as Map),
      );
      final entries = (resp?['Entries'] ?? resp?['entries']) as List<dynamic>? ?? const [];
      final files = entries.whereType<Map>().map((entry) {
        final m = Map<String, dynamic>.from(entry);
        final rawSize = m['sizeBytes'];
        return _FileItem(
          uri: (m['uri'] ?? '').toString(),
          name: (m['name'] ?? '').toString(),
          sizeBytes: rawSize is num ? rawSize.toInt() : null,
          modifiedAt: (m['modifiedAt'] ?? '').toString(),
          isFolder: (m['type'] ?? '').toString() == 'directory',
          readable: m['readable'] != false,
        );
      }).toList()
        ..sort((a, b) {
          if (a.isFolder != b.isFolder) return a.isFolder ? -1 : 1;
          return a.name.toLowerCase().compareTo(b.name.toLowerCase());
        });
      if (!mounted) return;
      setState(() {
        if (pushHistory && previous != null && previous != uri) _history.add(previous);
        _currentUri = uri;
        _files = files;
        _loading = false;
      });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  Future<bool> _backDirectory() async {
    if (_history.isEmpty) return false;
    final target = _history.removeLast();
    await _openUri(target, pushHistory: false);
    return true;
  }

  Future<void> _showFile(_FileItem item) async {
    if (!item.readable) return;
    try {
      final api = ref.read(backendServiceProvider);
      final resp = await api.get<Map<String, dynamic>>(
        '/api/workspaces/read',
        queryParameters: {'uri': item.uri, 'maxBytes': 1048576},
        fromJson: (e) => Map<String, dynamic>.from(e as Map),
      );
      if (!mounted) return;
      final isText = resp?['isText'] == true;
      final content = (resp?['content'] ?? '').toString();
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: Text(item.name),
          content: SizedBox(
            width: 640,
            child: isText
                ? SingleChildScrollView(
                    child: SelectableText(content, style: const TextStyle(fontFamily: 'monospace')),
                  )
                : const Text('当前文件不是可直接预览的文本文件。'),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context), child: const Text('关闭')),
          ],
        ),
      );
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('读取失败：$e')));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: _history.isEmpty,
      onPopInvokedWithResult: (didPop, _) {
        if (!didPop && _history.isNotEmpty) _backDirectory();
      },
      child: AmitiaScaffold(
        appBar: AmitiaAppBar(
          title: '文件浏览',
          showBackButton: true,
          fallbackRoute: AppRoutes.settingsToolbox,
        ),
        body: _loading
            ? const AmitiaLoadingState(message: '正在加载文件列表...')
            : _error != null
                ? AmitiaErrorState(message: _error!, onRetry: _loadMounts)
                : Column(
                    children: [
                      Padding(
                        padding: EdgeInsets.fromLTRB(
                          AppSpacing.pagePadding,
                          AppSpacing.md,
                          AppSpacing.pagePadding,
                          AppSpacing.sm,
                        ),
                        child: Row(
                          children: [
                            if (_history.isNotEmpty)
                              IconButton(
                                onPressed: _backDirectory,
                                icon: const Icon(Icons.arrow_upward),
                                tooltip: '上一级',
                              ),
                            Expanded(
                              child: Text(
                                _currentUri ?? '没有可用 Workspace',
                                maxLines: 2,
                                overflow: TextOverflow.ellipsis,
                                style: AppTypography.caption(context),
                              ),
                            ),
                            if (_mounts.length > 1)
                              PopupMenuButton<_WorkspaceMount>(
                                tooltip: '切换 Workspace',
                                onSelected: _openMount,
                                itemBuilder: (_) => _mounts
                                    .map((m) => PopupMenuItem(value: m, child: Text(m.name)))
                                    .toList(),
                                child: Padding(
                                  padding: const EdgeInsets.all(8),
                                  child: Row(
                                    mainAxisSize: MainAxisSize.min,
                                    children: [
                                      Text(_currentMountName ?? 'Workspace'),
                                      const Icon(Icons.arrow_drop_down),
                                    ],
                                  ),
                                ),
                              ),
                          ],
                        ),
                      ),
                      Expanded(
                        child: RefreshIndicator(
                          onRefresh: () => _currentUri == null
                              ? _loadMounts()
                              : _openUri(_currentUri!, pushHistory: false),
                          child: _files.isEmpty
                              ? ListView(
                                  children: [
                                    AmitiaEmptyState(
                                      icon: Icons.folder_open,
                                      title: _mounts.isEmpty ? '没有可用 Workspace' : '目录为空',
                                      subtitle: _mounts.isEmpty
                                          ? '请先创建或授权一个 Workspace'
                                          : '当前目录没有文件',
                                    ),
                                  ],
                                )
                              : ListView.separated(
                                  padding: EdgeInsets.fromLTRB(
                                    AppSpacing.pagePadding,
                                    0,
                                    AppSpacing.pagePadding,
                                    AppSpacing.xl,
                                  ),
                                  itemCount: _files.length,
                                  separatorBuilder: (_, _) => Divider(
                                    height: 1,
                                    thickness: 0.5,
                                    color: context.borderSecondary,
                                  ),
                                  itemBuilder: (context, index) {
                                    final item = _files[index];
                                    return ListTile(
                                      contentPadding: const EdgeInsets.symmetric(horizontal: 8),
                                      leading: Icon(
                                        _iconForName(item.name, item.isFolder),
                                        color: item.isFolder
                                            ? context.accentPrimary
                                            : context.textSecondary,
                                      ),
                                      title: Text(item.name, style: AppTypography.body(context)),
                                      subtitle: Text(
                                        '${_formatSize(item.sizeBytes)} · ${item.modifiedAt}',
                                        style: AppTypography.caption(context),
                                      ),
                                      trailing: Icon(
                                        item.isFolder ? Icons.chevron_right : Icons.open_in_new,
                                        size: 18,
                                        color: context.textTertiary,
                                      ),
                                      onTap: item.isFolder
                                          ? () => _openUri(item.uri)
                                          : () => _showFile(item),
                                    );
                                  },
                                ),
                        ),
                      ),
                    ],
                  ),
      ),
    );
  }
}
