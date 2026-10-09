import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../app/app_routes.dart';
import '../backend_transport/providers/backend_transport_providers.dart';
import 'amitia_misc.dart';
import 'amitia_scaffold.dart';

void openLogFolder(BuildContext context) {
  Navigator.of(
    context,
  ).push<void>(MaterialPageRoute(builder: (_) => const LogFolderPage()));
}

class LogFolderButton extends StatelessWidget {
  const LogFolderButton({super.key});

  @override
  Widget build(BuildContext context) => IconButton(
    tooltip: '打开文件夹',
    icon: const Icon(Icons.folder_open_outlined),
    onPressed: () => openLogFolder(context),
  );
}

class LogFolderPage extends ConsumerStatefulWidget {
  const LogFolderPage({super.key});

  @override
  ConsumerState<LogFolderPage> createState() => _LogFolderPageState();
}

class _LogFolderPageState extends ConsumerState<LogFolderPage> {
  List<Map<String, dynamic>> _files = [];
  String _directory = '';
  String? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final data = await ref
          .read(backendServiceProvider)
          .get<Map<String, dynamic>>('/api/logs/files');
      final items = data?['files'];
      if (!mounted) return;
      setState(() {
        _directory = data?['directory']?.toString() ?? '';
        _files = items is List
            ? items
                  .whereType<Map>()
                  .map((item) => Map<String, dynamic>.from(item))
                  .toList()
            : [];
        _loading = false;
      });
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _error = error.toString();
        _loading = false;
      });
    }
  }

  Future<void> _openFile(String name) async {
    try {
      final content = await ref
          .read(backendServiceProvider)
          .get<String>(
            '/api/logs/files/${Uri.encodeComponent(name)}',
            fromJson: (value) => value?.toString() ?? '',
          );
      if (!mounted) return;
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: Text(name),
          content: SizedBox(
            width: 760,
            height: MediaQuery.sizeOf(context).height * .65,
            child: SingleChildScrollView(
              child: SelectableText(
                content ?? '',
                style: const TextStyle(fontFamily: 'monospace', fontSize: 12),
              ),
            ),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('关闭'),
            ),
          ],
        ),
      );
    } catch (error) {
      if (!mounted) return;
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('读取日志文件失败：$error')));
    }
  }

  @override
  Widget build(BuildContext context) => AmitiaScaffold(
    appBar: AmitiaAppBar(
      title: '日志文件夹',
      showBackButton: true,
      fallbackRoute: AppRoutes.settingsMaintenanceCategory,
      actions: [
        IconButton(
          tooltip: '刷新',
          icon: const Icon(Icons.refresh),
          onPressed: _load,
        ),
      ],
    ),
    body: SafeArea(
      top: false,
      child: _loading
          ? const AmitiaLoadingState()
          : _error != null
          ? AmitiaErrorState(message: _error!, onRetry: _load)
          : Column(
              children: [
                if (_directory.isNotEmpty)
                  Padding(
                    padding: const EdgeInsets.all(16),
                    child: SelectableText(_directory),
                  ),
                const Padding(
                  padding: EdgeInsets.all(8),
                  child: Text('当前连接后端的日志目录 · 文件预览显示末尾内容'),
                ),
                Expanded(
                  child: _files.isEmpty
                      ? const AmitiaEmptyState(
                          icon: Icons.folder_outlined,
                          title: '暂无日志文件',
                        )
                      : ListView.builder(
                          itemCount: _files.length,
                          itemBuilder: (_, index) {
                            final file = _files[index];
                            final name = file['name']?.toString() ?? '';
                            return ListTile(
                              leading: const Icon(Icons.description_outlined),
                              title: Text(name),
                              subtitle: Text(
                                '${file['size'] ?? 0} B · ${file['modTime'] ?? ''}',
                              ),
                              trailing: const Icon(Icons.chevron_right),
                              onTap: name.isEmpty
                                  ? null
                                  : () => _openFile(name),
                            );
                          },
                        ),
                ),
              ],
            ),
    ),
  );
}
