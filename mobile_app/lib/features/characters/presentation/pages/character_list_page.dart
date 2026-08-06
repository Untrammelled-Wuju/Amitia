import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/models/character.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class CharacterListPage extends ConsumerStatefulWidget {
  const CharacterListPage({super.key});

  @override
  ConsumerState<CharacterListPage> createState() => _CharacterListPageState();
}

class _CharacterListPageState extends ConsumerState<CharacterListPage> {
  String? _defaultCharacterId;
  final Set<String> _archivedIds = {};
  List<CharacterDto> _characters = [];

  List<CharacterDto> get _activeCharacters =>
      _characters.where((c) => !_archivedIds.contains(c.id)).toList();

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
              padding: const EdgeInsets.fromLTRB(
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
                  _defaultCharacterId ??= characters.isNotEmpty ? characters.first.id : null;
                  final activeChars = characters.where((c) => !_archivedIds.contains(c.id)).toList();
                  if (activeChars.isEmpty) {
                    return Center(
                      child: Text(
                        '暂无角色，请先创建',
                        style: AppTypography.body(context).copyWith(color: context.textSecondary),
                      ),
                    );
                  }
                  return ListView.separated(
                    padding: const EdgeInsets.fromLTRB(
                      AppSpacing.pagePadding,
                      AppSpacing.xs,
                      AppSpacing.pagePadding,
                      AppSpacing.md,
                    ),
                    itemCount: activeChars.length,
                    separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
                    itemBuilder: (context, index) {
                      final character = activeChars[index];
                      final isDefault = character.id == _defaultCharacterId;
                      return AmitiaCharacterCard(
                        name: isDefault ? '${character.name} (默认)' : character.name,
                        status: character.status,
                        identity: character.identity,
                        avatarInitial: character.name.isNotEmpty ? character.name[0] : '?',
                        avatarColor: '#7668EE',
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
              padding: const EdgeInsets.fromLTRB(
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
                  const SizedBox(width: AppSpacing.md),
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
                      final copy = Character(
                        id: 'c${DateTime.now().millisecondsSinceEpoch}',
                        name: '${character.name} (副本)',
                        avatarColor: character.avatarColor,
                        avatarInitial: character.avatarInitial,
                        status: '离线',
                        mood: '新建',
                        identity: character.identity,
                        description: character.description,
                        relationshipDays: 0,
                        messageCount: 0,
                        personality: character.personality,
                        speakingStyle: character.speakingStyle,
                        userRelation: character.userRelation,
                        prompt: character.prompt,
                        currentActivity: '等待激活',
                        location: character.location,
                      );
                      setState(() {
                        MockData.characters.add(copy);
                      });
                      amitiaSnackBar(context, '已复制角色 ${character.name}');
                    });
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.file_download_outlined, context.accentPrimary),
                  title: '导出角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showCharacterSelection(context, '导出角色', (character) {
                      amitiaSnackBar(context, '角色 ${character.name} 已导出为 JSON 文件');
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
                      MockData.characters.sort((a, b) => a.name.compareTo(b.name));
                    });
                    amitiaSnackBar(context, '已按名称排序');
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.calendar_today_outlined, context.accentPrimary),
                  title: '按认识天数',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    setState(() {
                      MockData.characters.sort((a, b) => b.relationshipDays.compareTo(a.relationshipDays));
                    });
                    amitiaSnackBar(context, '已按认识天数排序');
                  },
                ),
                AmitiaListTile(
                  leading: _buildSheetIcon(context, Icons.chat_bubble_outline, context.accentPrimary),
                  title: '按消息数量',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    setState(() {
                      MockData.characters.sort((a, b) => b.messageCount.compareTo(a.messageCount));
                    });
                    amitiaSnackBar(context, '已按消息数量排序');
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
                                  color: _parseColor(character.avatarColor),
                                  shape: BoxShape.circle,
                                ),
                                child: Center(
                                  child: Text(
                                    character.avatarInitial,
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

  Color _parseColor(String hex) {
    final cleaned = hex.replaceAll('#', '');
    return Color(int.parse('FF$cleaned', radix: 16));
  }
}
