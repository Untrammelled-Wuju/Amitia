import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

class _FileItem {
  final IconData icon;
  final String name;
  final String size;
  final String date;
  final bool isFolder;
  const _FileItem({required this.icon, required this.name, required this.size, required this.date, this.isFolder = false});
}

class ToolboxFileBrowserPage extends ConsumerStatefulWidget {
  const ToolboxFileBrowserPage({super.key});

  @override
  ConsumerState<ToolboxFileBrowserPage> createState() => _ToolboxFileBrowserPageState();
}

class _ToolboxFileBrowserPageState extends ConsumerState<ToolboxFileBrowserPage> {
  List<_FileItem> _files = [];
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  IconData _iconForName(String name) {
    final n = name.toLowerCase();
    if (n.endsWith('.pdf')) return Icons.picture_as_pdf_outlined;
    if (n.endsWith('.doc') || n.endsWith('.docx')) return Icons.description_outlined;
    if (n.endsWith('.png') || n.endsWith('.jpg') || n.endsWith('.jpeg') || n.endsWith('.gif') || n.endsWith('.webp')) return Icons.image_outlined;
    if (n.endsWith('.m4a') || n.endsWith('.mp3') || n.endsWith('.wav') || n.endsWith('.flac')) return Icons.music_note_outlined;
    if (n.endsWith('.json') || n.endsWith('.yaml') || n.endsWith('.xml') || n.endsWith('.toml')) return Icons.code;
    if (n.endsWith('.zip') || n.endsWith('.tar') || n.endsWith('.gz') || n.endsWith('.rar')) return Icons.archive_outlined;
    return Icons.insert_drive_file_outlined;
  }

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.watch(backendServiceProvider);
      if (api == null) {
        if (mounted) {
          setState(() { _error = '后端服务未连接'; _loading = false; });
        }
        return;
      }
      List<_FileItem> files = [];
      try {
        final resp = await api.get<List<dynamic>>('/api/system/files');
        final items = resp ?? [];
        files = items.map((e) {
          final m = e as Map<String, dynamic>? ?? {};
          final name = (m['name'] ?? m['fileName'] ?? m['path'] ?? '').toString();
          final isFolder = (m['isDir'] ?? m['isDirectory'] ?? m['type'] == 'directory') as bool? ?? false;
          return _FileItem(
            icon: isFolder ? Icons.folder : _iconForName(name),
            name: name,
            size: (m['size'] ?? m['fileSize'] ?? '—').toString(),
            date: (m['modifiedAt'] ?? m['updatedAt'] ?? m['date'] ?? '').toString(),
            isFolder: isFolder,
          );
        }).toList();
      } catch (_) {
        final svc = ref.read(systemServiceProvider);
        final stats = await svc.chatStats();
        final data = stats as Map<String, dynamic>? ?? {};
        final convCount = data['totalConversations'] ?? data['conversationCount'] ?? 0;
        final charCount = data['totalCharacters'] ?? data['characterCount'] ?? 0;
        files = [
          _FileItem(icon: Icons.chat_bubble_outline, name: 'conversations', size: '$convCount 条', date: '存储', isFolder: true),
          _FileItem(icon: Icons.person_outline, name: 'characters', size: '$charCount 条', date: '存储', isFolder: true),
          _FileItem(icon: Icons.psychology_outlined, name: 'memories', size: '—', date: '存储', isFolder: true),
        ];
      }
      if (mounted) {
        setState(() { _files = files; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载文件列表...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: '文件浏览', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView(
        padding: const EdgeInsets.symmetric(vertical: AppSpacing.md),
        children: [
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            child: Text('当前路径：/storage/emulated/0', style: AppTypography.caption(context)),
          ),
          const SizedBox(height: AppSpacing.md),
          _files.isEmpty
              ? const AmitiaEmptyState(icon: Icons.folder_open, title: '暂无文件', subtitle: '目录为空或无法访问')
              : Container(
                  margin: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
                  decoration: BoxDecoration(
                    color: context.surfacePrimary,
                    borderRadius: AppRadius.brMedium,
                    border: Border.all(color: context.borderPrimary, width: 0.5),
                  ),
                  child: Column(
                    children: [
                      for (int i = 0; i < _files.length; i++) ...[
                        Padding(
                          padding: const EdgeInsets.symmetric(horizontal: AppSpacing.lg, vertical: 12),
                          child: Row(
                            children: [
                              Icon(_files[i].icon, size: 22,
                                  color: _files[i].isFolder ? context.accentPrimary : context.textSecondary),
                              const SizedBox(width: 14),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(_files[i].name, style: AppTypography.body(context)),
                                    Text('${_files[i].size} · ${_files[i].date}', style: AppTypography.caption(context)),
                                  ],
                                ),
                              ),
                              Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
                            ],
                          ),
                        ),
                        if (i < _files.length - 1)
                          Padding(
                            padding: const EdgeInsets.only(left: 52),
                            child: Divider(height: 1, thickness: 0.5, color: context.borderSecondary),
                          ),
                      ],
                    ],
                  ),
                ),
          const SizedBox(height: AppSpacing.xl),
        ],
      ),
    );
  }
}
