import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/artifact/artifact_providers.dart';
import '../../../../core/backend_connection/backend_connection_availability.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/character.dart';

class CharacterListPage extends ConsumerStatefulWidget {
  const CharacterListPage({super.key});

  @override
  ConsumerState<CharacterListPage> createState() => _CharacterListPageState();
}

enum _SortOrder { none, name, createdAt }

class _CharacterListPageState extends ConsumerState<CharacterListPage> {
  String? _defaultCharacterId;
  final Set<String> _archivedIds = {};
  List<CharacterDto> _characters = [];
  _SortOrder _sortOrder = _SortOrder.none;

  List<CharacterDto> get _activeCharacters {
    var list = _characters.where((c) => !_archivedIds.contains(c.id)).toList();
    switch (_sortOrder) {
      case _SortOrder.name:
        list.sort((a, b) => a.name.compareTo(b.name));
        break;
      case _SortOrder.createdAt:
        list.sort((a, b) => b.createdAt.compareTo(a.createdAt));
        break;
      case _SortOrder.none:
        break;
    }
    return list;
  }

  @override
  Widget build(BuildContext context) {
    final charactersAsync = ref.watch(characterListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色',
        navigation: AmitiaAppBarNavigation.back,
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.sm,
                AppSpacing.pagePadding,
                AppSpacing.sm,
              ),
              child: const AmitiaSearchField(hintText: '搜索角色'),
            ),
            Expanded(
              child: charactersAsync.when(
                loading: () => const Center(child: CircularProgressIndicator()),
                error: (err, _) => Center(
                  child: Padding(
                    padding: const EdgeInsets.all(32),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                        const SizedBox(height: 16),
                        Text(
                          '加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                          style: AppTypography.body(context).copyWith(color: context.error),
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: 16),
                        AmitiaButton(
                          label: '重试',
                          onPressed: () => ref.invalidate(characterListProvider),
                        ),
                      ],
                    ),
                  ),
                ),
                data: (characters) {
                  _characters = characters;
                  _defaultCharacterId ??= characters.isNotEmpty ? characters.first.id : null;
                  final activeChars = _activeCharacters;
                  if (activeChars.isEmpty) {
                    return Center(
                      child: Text(
                        '暂无角色，请先创建',
                        style: AppTypography.body(context).copyWith(color: context.textSecondary),
                      ),
                    );
                  }
                  return ListView.separated(
                    padding: EdgeInsets.fromLTRB(
                      AppSpacing.pagePadding,
                      AppSpacing.xs,
                      AppSpacing.pagePadding,
                      AppSpacing.md,
                    ),
                    itemCount: activeChars.length,
                    separatorBuilder: (_, _) => SizedBox(height: AppSpacing.sm),
                    itemBuilder: (context, index) {
                      final character = activeChars[index];
                      final isDefault = character.id == _defaultCharacterId;
                      return AmitiaCharacterCard(
                        name: isDefault ? '${character.name} (默认)' : character.name,
                        status: character.status,
                        identity: character.identity,
                        avatarInitial: character.name.isNotEmpty ? character.name[0] : '?',
                        avatarColor: '#8A5728',
                        mood: '',
                        lastActive: _getLastActive(character.isActive == 1),
                        onTap: () => context.push(AppRoutes.character(character.id)),
                      );
                    },
                  );
                },
              ),
            ),
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.xs,
                AppSpacing.pagePadding,
                AppSpacing.lg,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: AmitiaButton(
                      label: '创建新角色',
                      icon: Icons.person_add_alt_1,
                      isFullWidth: true,
                      onPressed: () => context.push(AppRoutes.charactersCreate),
                    ),
                  ),
                  SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: AmitiaButton(
                      label: '管理角色',
                      isSecondary: true,
                      isFullWidth: true,
                      onPressed: () => _showManageSheet(context),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _getLastActive(bool isOnline) {
    if (isOnline) {
      return '刚刚活跃';
    }
    return '离线';
  }

  Future<Dio> _dio() async {
    final availability = await ref.read(backendConnectionProvider.future);
    if (availability is! BackendConnectionAvailable) {
      throw StateError('后端当前不可用');
    }
    return createAuthenticatedDio(availability.config);
  }

  Future<void> _exportCharacter(CharacterDto character) async {
    final output = await FilePicker.platform.saveFile(
      dialogTitle: '保存角色卡',
      fileName: '${character.name}.charx',
      type: FileType.custom,
      allowedExtensions: const ['charx'],
    );
    if (output == null || output.isEmpty) return;
    final dio = await _dio();
    try {
      await dio.download(
        '/api/characters/${character.id}/export-card',
        output,
        queryParameters: const {'format': 'v3_charx', 'download': 'true'},
      );
      if (!mounted) return;
      amitiaSnackBar(context, '角色 ${character.name} 已导出');
    } catch (e) {
      if (!mounted) return;
      amitiaSnackBar(context, '导出失败：$e');
    } finally {
      dio.close(force: true);
    }
  }

  Future<void> _importCharacterCard() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['charx', 'json', 'png'],
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    final path = file.path;
    if (path == null || path.isEmpty) {
      if (mounted) amitiaSnackBar(context, '无法读取所选角色卡文件');
      return;
    }

    final dio = await _dio();
    try {
      final previewForm = FormData.fromMap({
        'card': await MultipartFile.fromFile(path, filename: file.name),
      });
      final previewResponse = await dio.post('/api/characters/import-card/preview', data: previewForm);
      final body = previewResponse.data;
      final payload = body is Map ? body['data'] : null;
      if (payload is! Map) throw StateError('后端未返回有效的角色卡预览');
      final preview = payload['preview'] is Map ? Map<String, dynamic>.from(payload['preview'] as Map) : <String, dynamic>{};
      final risks = preview['risks'] is List ? preview['risks'] as List : const [];
      if (!mounted) return;
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('导入角色卡', style: AppTypography.cardTitle(dialogContext)),
          content: Text(
            '名称：${preview['name'] ?? file.name}\n'
            '格式：${preview['format'] ?? payload['format'] ?? '-'}\n'
            '问候语：${preview['greetingCount'] ?? 0}\n'
            '世界书条目：${preview['lorebookEntryCount'] ?? 0}\n'
            '风险提示：${risks.length} 项\n\n确认导入该角色卡吗？',
            style: AppTypography.body(dialogContext),
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('确认导入')),
          ],
        ),
      );
      if (confirmed != true) return;

      final confirmForm = FormData.fromMap({
        'card': await MultipartFile.fromFile(path, filename: file.name),
      });
      final confirmResponse = await dio.post('/api/characters/import-card/confirm', data: confirmForm);
      final confirmBody = confirmResponse.data;
      final result = confirmBody is Map ? confirmBody['data'] : null;
      if (result is! Map) throw StateError('角色卡导入未完成');
      ref.invalidate(characterListProvider);
      if (!mounted) return;
      amitiaSnackBar(context, '已导入角色 ${(result['name'] ?? preview['name'] ?? '').toString()}');
    } catch (e) {
      if (!mounted) return;
      amitiaSnackBar(context, '导入失败：$e');
    } finally {
      dio.close(force: true);
    }
  }

  void _showManageSheet(BuildContext context) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text('管理角色', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.sort, context.accentPrimary),
                  title: '排序',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showSortSheet(context);
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.star_outline, context.accentPrimary),
                  title: '设为默认',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showCharacterSelection(context, '设为默认角色', (character) {
                      setState(() {
                        _defaultCharacterId = character.id;
                      });
                      amitiaSnackBar(context, '已将 ${character.name} 设为默认角色');
                    });
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.copy_outlined, context.accentPrimary),
                  title: '复制角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showCharacterSelection(context, '复制角色', (character) {
                      amitiaSnackBar(context, '角色复制功能开发中');
                    });
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.file_upload_outlined, context.accentPrimary),
                  title: '导入角色卡',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _importCharacterCard();
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.file_download_outlined, context.accentPrimary),
                  title: '导出角色卡',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showCharacterSelection(context, '导出角色卡', (character) {
                      _exportCharacter(character);
                    });
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.archive_outlined, context.accentPrimary),
                  title: '归档角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showCharacterSelection(context, '归档角色', (character) {
                      setState(() {
                        _archivedIds.add(character.id);
                        if (_defaultCharacterId == character.id) {
                          _defaultCharacterId = _activeCharacters.isNotEmpty
                              ? _activeCharacters.first.id
                              : '';
                        }
                      });
                      amitiaSnackBar(context, '角色 ${character.name} 已归档');
                    });
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.delete_outline, context.error),
                  title: '删除角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showCharacterSelection(context, '删除角色', (character) {
                      _handleDelete(context, character);
                    });
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showSortSheet(BuildContext context) {
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text('排序方式', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.text_fields, context.accentPrimary),
                  title: '按名称',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    setState(() {
                      _sortOrder = _SortOrder.name;
                    });
                    amitiaSnackBar(context, '已按名称排序');
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.calendar_today_outlined, context.accentPrimary),
                  title: '按创建时间',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    setState(() {
                      _sortOrder = _SortOrder.createdAt;
                    });
                    amitiaSnackBar(context, '已按创建时间排序');
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.sort_by_alpha, context.accentPrimary),
                  title: '默认排序',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    setState(() {
                      _sortOrder = _SortOrder.none;
                    });
                    amitiaSnackBar(context, '已恢复默认排序');
                  },
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  void _showCharacterSelection(
    BuildContext context,
    String title,
    ValueChanged<CharacterDto> onSelected,
  ) {
    final characters = _activeCharacters;
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(borderRadius: BorderRadius.vertical(top: Radius.circular(20))),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const SizedBox(height: 8),
                Center(
                  child: Container(
                    width: 40,
                    height: 4,
                    decoration: BoxDecoration(
                      color: context.borderPrimary,
                      borderRadius: BorderRadius.circular(2),
                    ),
                  ),
                ),
                const SizedBox(height: 20),
                Text(title, style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                SizedBox(
                  height: 400,
                  child: ListView.separated(
                    shrinkWrap: true,
                    itemCount: characters.length,
                    separatorBuilder: (_, _) => Divider(height: 1, color: context.borderSecondary),
                    itemBuilder: (_, index) {
                      final character = characters[index];
                      final isDefault = character.id == _defaultCharacterId;
                      return GestureDetector(
                        behavior: HitTestBehavior.opaque,
                        onTap: () {
                          Navigator.pop(sheetContext);
                          onSelected(character);
                        },
                        child: Padding(
                          padding: const EdgeInsets.symmetric(vertical: 12),
                          child: Row(
                            children: [
                              Container(
                                width: 40,
                                height: 40,
                                decoration: BoxDecoration(
                                  color: context.accentPrimary,
                                  shape: BoxShape.circle,
                                ),
                                child: Center(
                                  child: Text(
                                    character.name.isNotEmpty ? character.name[0] : '?',
                                    style: const TextStyle(color: Colors.white, fontSize: 16, fontWeight: FontWeight.w600),
                                  ),
                                ),
                              ),
                              const SizedBox(width: 12),
                              Expanded(
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      isDefault ? '${character.name} (默认)' : character.name,
                                      style: AppTypography.body(context),
                                    ),
                                    const SizedBox(height: 2),
                                    Text(character.identity, style: AppTypography.label(context)),
                                  ],
                                ),
                              ),
                              Icon(Icons.chevron_right, size: 18, color: context.textTertiary),
                            ],
                          ),
                        ),
                      );
                    },
                  ),
                ),
              ],
            ),
          ),
        );
      },
    );
  }

  void _handleDelete(BuildContext context, CharacterDto character) {
    if (character.id == _defaultCharacterId) {
      amitiaSnackBar(context, '删除默认角色需要先选择替代角色');
    } else {
      showAmitiaConfirmDialog(
        context,
        title: '删除角色',
        message: '确定要删除 ${character.name} 吗？此操作不可撤销。',
        confirmLabel: '删除',
        isDestructive: true,
      ).then((confirmed) {
        if (confirmed == true) {
          setState(() {
            _archivedIds.add(character.id);
          });
          ref.invalidate(characterListProvider);
          amitiaSnackBar(context, '${character.name} 已删除');
        }
      });
    }
  }

  Widget _buildSheetIcon(BuildContext context, IconData icon, Color color) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: color),
    );
  }
}
