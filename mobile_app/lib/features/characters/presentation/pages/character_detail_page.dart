import 'dart:convert';
import 'dart:developer' as developer;

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
import '../../../../core/widgets/character_avatar.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/shared_core_data_access.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/settings/current_character_provider.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/model_config.dart';
import '../../../../core/ui_runtime/mobile_extension_slot.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';
import '../../../../core/widgets/shared_core_read_only_banner.dart';

class CharacterDetailPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterDetailPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterDetailPage> createState() =>
      _CharacterDetailPageState();
}

class _CharacterDetailPageState extends ConsumerState<CharacterDetailPage> {
  int _selectedTab = 0;
  final _nameController = TextEditingController();
  final _identityController = TextEditingController();
  final _personalityController = TextEditingController();
  final _speakingStyleController = TextEditingController();
  final _relationshipStyleController = TextEditingController();
  final _characterBaseController = TextEditingController();
  final _boundaryRulesController = TextEditingController();
  final _descriptionController = TextEditingController();
  final _promptController = TextEditingController();
  final _personalityConfigController = TextEditingController();
  final _chatStyleConfigController = TextEditingController();
  final _sceneRulesController = TextEditingController();
  String _loadedCharacterId = '';
  String _editorAuthority = '';
  bool _savingSettings = false;
  bool _uploadingAvatar = false;

  final _tabs = const ['概览', '设定', '关系', '能力'];

  @override
  void dispose() {
    _nameController.dispose();
    _identityController.dispose();
    _personalityController.dispose();
    _speakingStyleController.dispose();
    _relationshipStyleController.dispose();
    _characterBaseController.dispose();
    _boundaryRulesController.dispose();
    _descriptionController.dispose();
    _promptController.dispose();
    _personalityConfigController.dispose();
    _chatStyleConfigController.dispose();
    _sceneRulesController.dispose();
    super.dispose();
  }

  void _syncControllers(CharacterDto character) {
    if (_loadedCharacterId == character.id) return;
    _loadedCharacterId = character.id;
    _editorAuthority = character.roleAuthority;
    _nameController.text = character.name;
    _identityController.text = character.identity;
    _personalityController.text = character.personality;
    _speakingStyleController.text = character.speakingStyle;
    _relationshipStyleController.text = character.relationshipStyle;
    _characterBaseController.text = character.characterBase;
    _boundaryRulesController.text = character.boundaryRules;
    _descriptionController.text = character.description;
    _promptController.text = character.basePrompt;
    const encoder = JsonEncoder.withIndent('  ');
    _personalityConfigController.text = encoder.convert(
      character.personalityConfig,
    );
    _chatStyleConfigController.text = encoder.convert(
      character.chatStyleConfig,
    );
    _sceneRulesController.text = encoder.convert(character.sceneRules);
  }

  @override
  Widget build(BuildContext context) {
    final characterAsync = ref.watch(characterListProvider);
    final coreAccessAsync = ref.watch(sharedCoreDataAccessProvider);
    final coreAccess =
        coreAccessAsync.valueOrNull ??
        SharedCoreDataAccess(
          read: true,
          write: false,
          reason: coreAccessAsync.hasError
              ? '无法确认统筹中心数据权限：${coreAccessAsync.error}'
              : '正在确认统筹中心数据权限',
          coreId: '',
        );
    final backendAvailability = ref
        .watch(backendConnectionProvider)
        .valueOrNull;
    final uiSnapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final hasExtensionTab =
        uiSnapshot
            ?.contributionsForSlot('character.detail.tab')
            .where(
              (item) =>
                  !item.runtimeId.startsWith('host.character.') &&
                  item.contributionId !=
                      'com.amitia.builtin.active-task.character-tab',
            )
            .isNotEmpty ??
        false;

    return characterAsync.when(
      loading: () => const AmitiaScaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (err, _) =>
          AmitiaScaffold(body: Center(child: Text('加载失败: $err'))),
      data: (characters) {
        final character = characters.cast<CharacterDto?>().firstWhere(
          (c) => c?.id == widget.characterId,
          orElse: () => null,
        );
        if (character == null) {
          return const AmitiaScaffold(body: Center(child: Text('角色不存在')));
        }
        final currentCharacterId =
            characters.where((item) => item.isActive == 1).firstOrNull?.id ??
            characters.where((item) => item.isDefault).firstOrNull?.id ??
            '';
        _syncControllers(character);
        return _buildScaffold(
          context,
          character,
          hasExtensionTab,
          backendAvailability,
          coreAccess,
          character.id == currentCharacterId,
        );
      },
    );
  }

  Map<String, dynamic> _characterSlotContext(CharacterDto character) => {
    'characterId': character.id,
    'character': {
      'id': character.id,
      'name': character.name,
      'status': character.status,
      'identity': character.identity,
    },
  };

  Widget _buildScaffold(
    BuildContext context,
    CharacterDto character,
    bool hasExtensionTab,
    BackendConnectionAvailability? backendAvailability,
    SharedCoreDataAccess coreAccess,
    bool isCurrent,
  ) {
    final tabs = <String>[
      '概览',
      '设定',
      _selectedTab >= 2
          ? '${_selectedTab < _tabs.length ? _tabs[_selectedTab] : '扩展'} ▾'
          : '更多',
    ];
    final selectedTab = _selectedTab < 2 ? _selectedTab : 2;
    final slotContext = _characterSlotContext(character);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: character.name,
        showBackButton: true,
        fallbackRoute: AppRoutes.characters,
        actions: [
          ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 180),
            child: MobileExtensionSlot(
              slotId: 'character.detail.action',
              context: slotContext,
            ),
          ),
          AmitiaIconButton(
            icon: Icons.more_horiz,
            onPressed: () => _showCharacterActionsSheet(context, character),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            SharedCoreReadOnlyBanner(access: coreAccess),
            _buildInfoSection(
              context,
              character,
              backendAvailability,
              isCurrent,
            ),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
              child: MobileExtensionSlot(
                slotId: 'character.sidebar.card',
                context: slotContext,
              ),
            ),
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.pagePadding,
                AppSpacing.md,
                AppSpacing.pagePadding,
                AppSpacing.sm,
              ),
              child: AmitiaSegmentedControl(
                segments: tabs,
                selectedIndex: selectedTab,
                onChanged: (index) {
                  if (index == 2) {
                    _showSections(context, character, hasExtensionTab);
                  } else {
                    setState(() => _selectedTab = index);
                  }
                },
              ),
            ),
            Expanded(
              child: _buildTabContent(context, character, hasExtensionTab),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildInfoSection(
    BuildContext context,
    CharacterDto character,
    BackendConnectionAvailability? backendAvailability,
    bool isCurrent,
  ) {
    final normalizedStatus = character.status.toLowerCase();
    final isExplicitOnline = const {
      '在线',
      'online',
      'active',
    }.contains(normalizedStatus);
    final isOnline = isCurrent || isExplicitOnline;
    final statusLabel = isCurrent
        ? '使用中'
        : switch (normalizedStatus) {
            'enabled' => '已启用',
            'disabled' => '已停用',
            'archived' => '已归档',
            _ => character.status,
          };
    final initial = character.name.isNotEmpty ? character.name[0] : '?';

    return Padding(
      padding: EdgeInsets.fromLTRB(
        AppSpacing.pagePadding,
        AppSpacing.md,
        AppSpacing.pagePadding,
        AppSpacing.xs,
      ),
      child: Row(
        children: [
          InkWell(
            onTap: _uploadingAvatar || !_canWrite
                ? null
                : () => _uploadAvatar(character),
            customBorder: const CircleBorder(),
            child: Stack(
              children: [
                CharacterAvatar(
                  characterId: character.id,
                  avatar: character.avatar,
                  initial: initial,
                  size: 72,
                ),
                Positioned(
                  right: 0,
                  top: 0,
                  child: Container(
                    width: 22,
                    height: 22,
                    decoration: BoxDecoration(
                      color: context.surfacePrimary,
                      shape: BoxShape.circle,
                      border: Border.all(color: context.borderPrimary),
                    ),
                    child: _uploadingAvatar
                        ? const Padding(
                            padding: EdgeInsets.all(5),
                            child: CircularProgressIndicator(strokeWidth: 2),
                          )
                        : Icon(
                            Icons.edit,
                            size: 13,
                            color: context.textSecondary,
                          ),
                  ),
                ),
                if (isOnline)
                  Positioned(
                    right: 0,
                    bottom: 0,
                    child: Container(
                      width: 18,
                      height: 18,
                      decoration: BoxDecoration(
                        color: context.success,
                        shape: BoxShape.circle,
                        border: Border.all(
                          color: context.surfacePrimary,
                          width: 2.5,
                        ),
                      ),
                    ),
                  ),
              ],
            ),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  character.name,
                  style: AppTypography.sectionTitle(context),
                ),
                const SizedBox(height: 4),
                Row(
                  children: [
                    AmitiaStatusBadge(
                      label: statusLabel,
                      type: isOnline ? BadgeType.success : BadgeType.neutral,
                    ),
                    SizedBox(width: AppSpacing.sm),
                    Flexible(
                      child: Text(
                        '· ${character.speakingStyle}',
                        style: AppTypography.caption(context),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  character.description,
                  style: AppTypography.caption(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  void _showSections(
    BuildContext context,
    CharacterDto character,
    bool hasExtensionTab,
  ) {
    showModalBottomSheet<void>(
      context: context,
      useSafeArea: true,
      isScrollControlled: true,
      builder: (sheetContext) => SingleChildScrollView(
        child: Padding(
          padding: EdgeInsets.fromLTRB(
            16,
            0,
            16,
            24 + MediaQuery.paddingOf(sheetContext).bottom,
          ),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('角色资料', style: AppTypography.sectionTitle(sheetContext)),
              const SizedBox(height: 12),
              for (var i = 2; i < _tabs.length + (hasExtensionTab ? 1 : 0); i++)
                ListTile(
                  title: Text(i < _tabs.length ? _tabs[i] : '扩展'),
                  trailing: const Icon(Icons.chevron_right),
                  onTap: () {
                    Navigator.pop(sheetContext);
                    setState(() => _selectedTab = i);
                  },
                ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _saveCharacterSettings(CharacterDto character) async {
    if (_savingSettings) return;
    final name = _nameController.text.trim();
    if (name.isEmpty) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('角色名称不能为空')));
      return;
    }
    Map<String, dynamic> personalityConfig;
    Map<String, dynamic> chatStyleConfig;
    Map<String, dynamic> sceneRules;
    try {
      personalityConfig = _decodeJsonObject(
        _personalityConfigController.text,
        '人格参数',
      );
      chatStyleConfig = _decodeJsonObject(
        _chatStyleConfigController.text,
        '聊天风格配置',
      );
      sceneRules = _decodeJsonObject(_sceneRulesController.text, '场景规则');
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(e.toString().replaceFirst('FormatException: ', '')),
        ),
      );
      return;
    }
    setState(() => _savingSettings = true);
    try {
      await ref
          .read(characterServiceProvider)
          .update(character.id, <String, dynamic>{
            'name': name,
            'identity': _identityController.text.trim(),
            'personality': _personalityController.text.trim(),
            'speakingStyle': _speakingStyleController.text.trim(),
            'relationshipStyle': _relationshipStyleController.text.trim(),
            'characterBase': _characterBaseController.text.trim(),
            'boundaryRules': _boundaryRulesController.text.trim(),
            'description': _descriptionController.text.trim(),
            'basePrompt': _promptController.text.trim(),
            'personalityConfig': personalityConfig,
            'chatStyleConfig': jsonEncode(chatStyleConfig),
            'sceneRules': jsonEncode(sceneRules),
          }, roleAuthority: _editorAuthority);
      _loadedCharacterId = '';
      ref.invalidate(characterListProvider);
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('角色设定已保存')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('保存失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _savingSettings = false);
    }
  }

  Map<String, dynamic> _decodeJsonObject(String raw, String label) {
    final text = raw.trim();
    if (text.isEmpty) return const <String, dynamic>{};
    final decoded = jsonDecode(text);
    if (decoded is! Map) throw FormatException('$label 必须是 JSON 对象');
    return decoded.map((key, value) => MapEntry(key.toString(), value));
  }

  Future<void> _uploadAvatar(CharacterDto character) async {
    if (_uploadingAvatar) return;
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.image,
      allowMultiple: false,
    );
    if (picked == null || picked.files.isEmpty) return;
    if (!mounted) return;
    final path = picked.files.single.path;
    if (path == null || path.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('无法读取所选头像文件')));
      }
      return;
    }
    setState(() => _uploadingAvatar = true);
    try {
      final result = await ref
          .read(characterDetailServiceProvider)
          .uploadAvatar(
            character.id,
            path,
            roleAuthority: character.roleAuthority,
          );
      final avatarUrl = (result?['avatarUrl'] ?? '').toString();
      if (avatarUrl.isEmpty) {
        throw StateError('后端未返回头像地址');
      }
      final refreshed = await ref.refresh(characterListProvider.future);
      if (!refreshed.any(
        (item) => item.id == character.id && item.avatar == avatarUrl,
      )) {
        throw StateError('头像已上传，但角色资料尚未同步，请重试刷新');
      }
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(const SnackBar(content: Text('头像已更新')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('头像上传失败：$e')));
      }
    } finally {
      if (mounted) setState(() => _uploadingAvatar = false);
    }
  }

  Widget _buildTabContent(
    BuildContext context,
    CharacterDto character,
    bool hasExtensionTab,
  ) {
    switch (_selectedTab) {
      case 0:
        return _buildOverviewTab(context, character);
      case 1:
        return _buildSettingsTab(context, character);
      case 2:
        return _buildRelationTab(context, character);
      case 3:
        return _buildAbilityTab(context, character);
      case 4:
        if (hasExtensionTab) {
          return ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              MobileExtensionSlot(
                slotId: 'character.detail.tab',
                context: _characterSlotContext(character),
                excludedContributionIds: const {
                  'com.amitia.builtin.active-task.character-tab',
                },
              ),
            ],
          );
        }
        return const SizedBox.shrink();
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildOverviewTab(BuildContext context, CharacterDto character) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('基本资料', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              _buildInfoRow(context, '名字', character.name),
              _buildInfoRow(context, '身份', character.identity),
              _buildInfoRow(context, '性格', character.personality),
              _buildInfoRow(context, '说话方式', character.speakingStyle),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: IntrinsicHeight(
            child: Row(
              children: [
                _buildStatItem(context, character.createdAt, '创建时间'),
                VerticalDivider(width: 1, color: context.borderPrimary),
                _buildStatItem(context, character.voiceType ?? '默认', '语音类型'),
              ],
            ),
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        _buildManagementSection(context),
      ],
    );
  }

  Widget _buildSettingsTab(BuildContext context, CharacterDto character) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('基础设定', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.md),
              _buildLabeledField(context, '名字', _nameController, '角色名称'),
              _buildLabeledField(context, '身份', _identityController, '角色身份'),
              _buildLabeledField(
                context,
                '性格',
                _personalityController,
                '角色性格',
                maxLines: 3,
              ),
              _buildLabeledField(
                context,
                '说话方式',
                _speakingStyleController,
                '说话风格',
                maxLines: 3,
              ),
              _buildLabeledField(
                context,
                '关系风格',
                _relationshipStyleController,
                '与用户相处和建立关系的方式',
                maxLines: 3,
              ),
              _buildLabeledField(
                context,
                '角色基础设定',
                _characterBaseController,
                '不可轻易变化的角色基础信息',
                maxLines: 4,
              ),
              _buildLabeledField(
                context,
                '边界规则',
                _boundaryRulesController,
                '角色需要长期遵守的行为边界',
                maxLines: 4,
              ),
              _buildLabeledField(
                context,
                '简介',
                _descriptionController,
                '角色简介',
                maxLines: 3,
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('性格提示词', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              AmitiaTextField(
                controller: _promptController,
                maxLines: 8,
                hintText: '输入基础提示词...',
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: ExpansionTile(
            tilePadding: EdgeInsets.zero,
            childrenPadding: EdgeInsets.only(top: AppSpacing.sm),
            shape: const Border(),
            collapsedShape: const Border(),
            maintainState: true,
            title: Text('高级人格与场景配置', style: AppTypography.cardTitle(context)),
            subtitle: Text(
              '人格、聊天风格与场景规则 JSON',
              style: AppTypography.caption(context),
            ),
            children: [
              SizedBox(height: AppSpacing.sm),
              _buildLabeledField(
                context,
                '人格参数 JSON',
                _personalityConfigController,
                '{}',
                maxLines: 8,
              ),
              _buildLabeledField(
                context,
                '聊天风格 JSON',
                _chatStyleConfigController,
                '{}',
                maxLines: 6,
              ),
              _buildLabeledField(
                context,
                '场景规则 JSON',
                _sceneRulesController,
                '{}',
                maxLines: 6,
              ),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(
          label: _savingSettings ? '保存中...' : '保存角色设定',
          icon: Icons.save_outlined,
          isFullWidth: true,
          onPressed: _savingSettings || !_canWrite
              ? null
              : () => _saveCharacterSettings(character),
        ),
      ],
    );
  }

  Widget _buildLabeledField(
    BuildContext context,
    String label,
    TextEditingController controller,
    String hint, {
    int maxLines = 1,
  }) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.md),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(label, style: AppTypography.label(context)),
          SizedBox(height: AppSpacing.xs),
          AmitiaTextField(
            controller: controller,
            hintText: hint,
            maxLines: maxLines,
          ),
        ],
      ),
    );
  }

  Widget _buildRelationTab(BuildContext context, CharacterDto character) {
    return FutureBuilder<Map<String, dynamic>?>(
      future: ref
          .read(temporalServiceProvider)
          .relationshipTimeState(character.id),
      builder: (context, snapshot) {
        final state = snapshot.data ?? const <String, dynamic>{};
        return ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            AmitiaCard(
              child: IntrinsicHeight(
                child: Row(
                  children: [
                    _buildStatItem(context, character.createdAt, '创建时间'),
                    VerticalDivider(width: 1, color: context.borderPrimary),
                    _buildStatItem(
                      context,
                      character.isActive == 1 ? '是' : '否',
                      '活跃状态',
                    ),
                  ],
                ),
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          '关系时间状态',
                          style: AppTypography.cardTitle(context),
                        ),
                      ),
                      if (snapshot.connectionState == ConnectionState.waiting)
                        const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  if (snapshot.hasError)
                    Text(
                      '加载失败：${snapshot.error}',
                      style: AppTypography.caption(
                        context,
                      ).copyWith(color: context.error),
                    )
                  else if (state.isEmpty &&
                      snapshot.connectionState != ConnectionState.waiting)
                    Text(
                      '尚未产生可展示的关系时间状态',
                      style: AppTypography.caption(context),
                    )
                  else ...[
                    _buildInfoRow(
                      context,
                      '首次互动',
                      _displayTime(state['firstInteractionAt']),
                    ),
                    _buildInfoRow(
                      context,
                      '关系持续',
                      '${_intValue(state['relationshipAgeDays'])} 天',
                    ),
                    _buildInfoRow(
                      context,
                      '互动次数',
                      '${_intValue(state['interactionCount'])}',
                    ),
                    _buildInfoRow(
                      context,
                      '会话次数',
                      '${_intValue(state['sessionCount'])}',
                    ),
                    _buildInfoRow(
                      context,
                      '最近互动',
                      _displayTime(state['lastSuccessfulExchangeAt']),
                    ),
                    _buildInfoRow(
                      context,
                      '期望间隔',
                      _formatDuration(state['expectedGapSeconds']),
                    ),
                    _buildInfoRow(
                      context,
                      '连续性',
                      _formatScore(state['continuityScore']),
                    ),
                    _buildInfoRow(
                      context,
                      '重新适应',
                      '${_intValue(state['reacclimationTurnsLeft'])} 回合',
                    ),
                    if ((state['reunion'] as Map?)?.isNotEmpty == true)
                      _buildInfoRow(
                        context,
                        '重逢状态',
                        '${(state['reunion'] as Map)['level'] ?? '-'} · ${(state['reunion'] as Map)['state'] ?? '-'}',
                      ),
                  ],
                ],
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('关系设定', style: AppTypography.cardTitle(context)),
                  SizedBox(height: AppSpacing.sm),
                  _buildInfoRow(context, '关系风格', character.relationshipStyle),
                  _buildInfoRow(context, '边界规则', character.boundaryRules),
                ],
              ),
            ),
          ],
        );
      },
    );
  }

  Widget _buildAbilityTab(BuildContext context, CharacterDto character) {
    final configsAsync = ref.watch(modelConfigListProvider);
    final configs = configsAsync.valueOrNull ?? const [];
    final activeModels = configs
        .where((item) => item.isActive == 1)
        .toList(growable: false);
    final activeModel = activeModels.isEmpty ? null : activeModels.first;
    final roleModel = configs
        .where((item) => _intValue(item.id) == character.modelConfigId)
        .firstOrNull;
    final effectiveModel = roleModel ?? activeModel;
    final modelLabel = effectiveModel == null
        ? (configsAsync.isLoading ? '加载中...' : '未配置激活模型')
        : '${roleModel == null ? '默认模型 · ' : '角色模型 · '}${effectiveModel.name.isEmpty ? effectiveModel.provider : effectiveModel.name}${effectiveModel.model.isEmpty ? '' : ' · ${effectiveModel.model}'}'
              '${character.reasoningEffort.isEmpty ? '' : ' · 思考 ${character.reasoningEffort}'}';
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        _buildAbilitySection(
          context,
          '模型',
          modelLabel,
          Icons.psychology_outlined,
          onTap: _canWrite
              ? () => _showCharacterModelSettings(character, configs)
              : null,
        ),
        SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(
          context,
          '语音类型',
          character.voiceType ?? '默认',
          Icons.record_voice_over_outlined,
        ),
        SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(
          context,
          '语音参数',
          '速度 ${character.voiceSpeed?.toStringAsFixed(2) ?? '-'} · 音调 ${character.voicePitch?.toStringAsFixed(2) ?? '-'} · 音量 ${character.voiceVolume?.toStringAsFixed(2) ?? '-'}',
          Icons.tune_outlined,
        ),
      ],
    );
  }

  Widget _buildInfoRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 72,
            child: Text(label, style: AppTypography.caption(context)),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Text(
              value.isEmpty ? '-' : value,
              style: AppTypography.bodySmall(context),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildStatItem(BuildContext context, String value, String label) {
    return Expanded(
      child: Column(
        children: [
          Text(
            value.isEmpty ? '-' : value,
            style: AppTypography.sectionTitle(
              context,
            ).copyWith(color: context.accentPrimary),
          ),
          const SizedBox(height: 2),
          Text(label, style: AppTypography.label(context)),
        ],
      ),
    );
  }

  Widget _buildAbilitySection(
    BuildContext context,
    String title,
    String content,
    IconData icon, {
    VoidCallback? onTap,
  }) {
    final card = AmitiaCard(
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: context.accentSoft,
              borderRadius: AppRadius.brSmall,
            ),
            child: Icon(icon, size: 20, color: context.accentPrimary),
          ),
          SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(title, style: AppTypography.cardTitle(context)),
                const SizedBox(height: 2),
                Text(content, style: AppTypography.caption(context)),
              ],
            ),
          ),
          if (onTap != null) const Icon(Icons.chevron_right_rounded, size: 20),
        ],
      ),
    );
    if (onTap == null) return card;
    return InkWell(borderRadius: AppRadius.brMedium, onTap: onTap, child: card);
  }

  Future<void> _showCharacterModelSettings(
    CharacterDto character,
    List<ModelConfigDto> configs,
  ) async {
    var modelConfigId = character.modelConfigId;
    var reasoningEnabled = character.reasoningEnabled;
    var reasoningValue = switch (character.reasoningEffort) {
      'low' => 0,
      'medium' => 1,
      'high' => 2,
      'xhigh' => 3,
      _ => 2,
    };
    final saved = await showModalBottomSheet<bool>(
      context: context,
      isScrollControlled: true,
      backgroundColor: context.surfacePrimary,
      builder: (sheetContext) => StatefulBuilder(
        builder: (sheetContext, setSheetState) => Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.pagePadding,
            AppSpacing.lg,
            AppSpacing.pagePadding,
            MediaQuery.of(sheetContext).viewInsets.bottom + AppSpacing.lg,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text(
                  '${character.name} · 模型设置',
                  style: AppTypography.sectionTitle(sheetContext),
                ),
                SizedBox(height: AppSpacing.md),
                DropdownButtonFormField<int>(
                  initialValue: modelConfigId,
                  decoration: const InputDecoration(labelText: '角色模型'),
                  items: [
                    const DropdownMenuItem(value: 0, child: Text('默认模型（跟随全局）')),
                    ...configs.map(
                      (model) => DropdownMenuItem(
                        value: _intValue(model.id),
                        child: Text(
                          '${model.name.isEmpty ? model.model : model.name} · ${model.model}',
                        ),
                      ),
                    ),
                  ],
                  onChanged: (value) {
                    if (value != null) {
                      setSheetState(() => modelConfigId = value);
                    }
                  },
                ),
                SizedBox(height: AppSpacing.sm),
                DropdownButtonFormField<int>(
                  initialValue: reasoningEnabled,
                  decoration: const InputDecoration(labelText: '思考模式'),
                  items: const [
                    DropdownMenuItem(value: -1, child: Text('跟随默认')),
                    DropdownMenuItem(value: 1, child: Text('开启')),
                    DropdownMenuItem(value: 0, child: Text('关闭')),
                  ],
                  onChanged: (value) {
                    if (value != null) {
                      setSheetState(() => reasoningEnabled = value);
                    }
                  },
                ),
                SizedBox(height: AppSpacing.md),
                Text('思考强度', style: AppTypography.caption(sheetContext)),
                Slider(
                  value: reasoningValue.toDouble(),
                  min: 0,
                  max: 3,
                  divisions: 3,
                  label: const ['低', '中', '高', '极高'][reasoningValue],
                  onChanged: reasoningEnabled == 0
                      ? null
                      : (value) {
                          setSheetState(() {
                            reasoningValue = value.round();
                            if (reasoningEnabled < 0) reasoningEnabled = 1;
                          });
                        },
                ),
                Text(
                  '仅覆盖当前角色，不修改模型本身的全局配置。',
                  style: AppTypography.caption(sheetContext),
                ),
                SizedBox(height: AppSpacing.md),
                FilledButton(
                  onPressed: () async {
                    try {
                      await ref
                          .read(characterDetailServiceProvider)
                          .updateCharacter(character.id, {
                            'modelConfigId': modelConfigId,
                            'reasoningEffort': reasoningEnabled == -1
                                ? ''
                                : const [
                                    'low',
                                    'medium',
                                    'high',
                                    'xhigh',
                                  ][reasoningValue],
                            'reasoningEnabled': reasoningEnabled,
                          }, roleAuthority: character.roleAuthority);
                      ref.invalidate(characterListProvider);
                      if (sheetContext.mounted) {
                        Navigator.pop(sheetContext, true);
                      }
                    } catch (error, stackTrace) {
                      developer.log(
                        '[original-error] $error',
                        name: 'character.model-settings',
                        error: error,
                        stackTrace: stackTrace,
                      );
                      if (sheetContext.mounted) {
                        amitiaSnackBar(sheetContext, '保存失败：$error');
                      }
                    }
                  },
                  child: const Text('保存角色模型设置'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
    if (saved == true && mounted) {
      amitiaSnackBar(context, '角色模型设置已保存');
    }
  }

  String _displayTime(dynamic value) {
    final text = value?.toString().trim() ?? '';
    if (text.isEmpty || text.startsWith('0001-01-01')) return '-';
    final parsed = DateTime.tryParse(text);
    if (parsed == null) return text;
    final local = parsed.toLocal();
    String two(int number) => number.toString().padLeft(2, '0');
    return '${local.year}-${two(local.month)}-${two(local.day)} ${two(local.hour)}:${two(local.minute)}';
  }

  int _intValue(dynamic value) {
    if (value is num) return value.toInt();
    return int.tryParse(value?.toString() ?? '') ?? 0;
  }

  double _doubleValue(dynamic value) {
    if (value is num) return value.toDouble();
    return double.tryParse(value?.toString() ?? '') ?? 0;
  }

  String _formatScore(dynamic value) {
    final numeric = _doubleValue(value);
    final percent = numeric.abs() <= 1 ? numeric * 100 : numeric;
    return '${percent.clamp(0, 100).toStringAsFixed(0)}%';
  }

  String _formatDuration(dynamic seconds) {
    final total = _doubleValue(seconds).round();
    if (total <= 0) return '-';
    final days = total ~/ 86400;
    final hours = (total % 86400) ~/ 3600;
    final minutes = (total % 3600) ~/ 60;
    final parts = <String>[];
    if (days > 0) parts.add('$days 天');
    if (hours > 0) parts.add('$hours 小时');
    if (minutes > 0 && days == 0) parts.add('$minutes 分钟');
    return parts.isEmpty ? '$total 秒' : parts.join(' ');
  }

  Widget _buildManagementSection(BuildContext context) {
    final entries = <_ManageEntry>[
      _ManageEntry(
        title: '表情包',
        icon: Icons.emoji_emotions_outlined,
        route: AppRoutes.emotes,
      ),
      _ManageEntry(
        title: '语音与声音复刻',
        icon: Icons.record_voice_over_outlined,
        route: AppRoutes.characterVoice(widget.characterId),
      ),
      _ManageEntry(
        title: '心理状态',
        icon: Icons.psychology_outlined,
        route: AppRoutes.characterPsyche(widget.characterId),
      ),
    ];

    return AmitiaCard(
      padding: EdgeInsets.symmetric(vertical: AppSpacing.cardPadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: EdgeInsets.symmetric(horizontal: AppSpacing.lg),
            child: Text('角色管理', style: AppTypography.cardTitle(context)),
          ),
          SizedBox(height: AppSpacing.xs),
          ...entries.map(
            (e) => AmitiaListTile(
              leading: _buildManagementIcon(context, e.icon),
              title: e.title,
              trailing: Icon(
                Icons.chevron_right,
                color: context.textTertiary,
                size: 20,
              ),
              onTap: () => context.push(e.route),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildManagementIcon(BuildContext context, IconData icon) {
    return Container(
      width: 36,
      height: 36,
      decoration: BoxDecoration(
        color: context.accentSoft,
        borderRadius: AppRadius.brSmall,
      ),
      child: Icon(icon, size: 20, color: context.accentPrimary),
    );
  }

  void _showCharacterActionsSheet(
    BuildContext context,
    CharacterDto character,
  ) {
    final canWrite = _canWrite;
    showModalBottomSheet(
      context: context,
      backgroundColor: context.surfacePrimary,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetContext) {
        return SafeArea(
          child: Padding(
            padding: const EdgeInsets.fromLTRB(20, 0, 20, 34),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('角色操作', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: _buildActionIcon(
                    context,
                    Icons.star_outline,
                    context.accentPrimary,
                  ),
                  title: '设为当前角色',
                  onTap: canWrite
                      ? () {
                          Navigator.pop(sheetContext);
                          _setAsCurrent(character);
                        }
                      : null,
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(
                    context,
                    Icons.copy_outlined,
                    context.accentPrimary,
                  ),
                  title: '复制角色',
                  onTap: canWrite
                      ? () {
                          Navigator.pop(sheetContext);
                          _copyCharacter(character);
                        }
                      : null,
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(
                    context,
                    Icons.file_download_outlined,
                    context.accentPrimary,
                  ),
                  title: '导出角色卡',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _exportCharacter(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(
                    context,
                    Icons.delete_outline,
                    context.error,
                  ),
                  title: '删除角色',
                  onTap: canWrite
                      ? () {
                          Navigator.pop(sheetContext);
                          _showDeleteConfirm(context, character);
                        }
                      : null,
                ),
                const SizedBox(height: 8),
              ],
            ),
          ),
        );
      },
    );
  }

  Widget _buildActionIcon(BuildContext context, IconData icon, Color color) {
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

  bool get _canWrite =>
      ref.read(sharedCoreDataAccessProvider).valueOrNull?.write ?? false;

  Future<void> _setAsCurrent(CharacterDto character) async {
    try {
      final svc = ref.read(characterServiceProvider);
      final activated = await svc.setActive(
        widget.characterId,
        roleAuthority: character.roleAuthority,
      );
      if (activated == null) throw StateError('服务未确认角色激活');
      if (!mounted) return;
      ref.read(currentCharacterIdProvider.notifier).state = widget.characterId;
      ref.invalidate(characterListProvider);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('已将「${character.name}」设为当前角色')));
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('角色激活失败：$error')));
      }
    }
  }

  Future<void> _copyCharacter(CharacterDto character) async {
    try {
      final created = await ref
          .read(characterServiceProvider)
          .duplicate(character.id, roleAuthority: character.roleAuthority);
      if (created == null) throw StateError('后端未返回复制后的角色');
      ref.invalidate(characterListProvider);
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('已复制为「${created.name}」')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('复制角色失败：$e')));
      }
    }
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
        '/api/characters/${widget.characterId}/export-card',
        output,
        queryParameters: const {'format': 'v3_charx', 'download': 'true'},
      );
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('已导出角色卡：${character.name}')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(
          context,
        ).showSnackBar(SnackBar(content: Text('导出失败：$e')));
      }
    } finally {
      dio.close(force: true);
    }
  }

  void _showDeleteConfirm(BuildContext context, CharacterDto character) {
    showDialog(
      context: context,
      builder: (dialogContext) {
        return AlertDialog(
          backgroundColor: dialogContext.surfacePrimary,
          shape: RoundedRectangleBorder(borderRadius: AppRadius.brMedium),
          title: Text('删除角色', style: AppTypography.cardTitle(dialogContext)),
          content: Text(
            '确定要删除角色「${character.name}」吗？所有相关数据将被清除，此操作不可恢复。',
            style: AppTypography.body(dialogContext),
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext),
              child: Text(
                '取消',
                style: TextStyle(color: dialogContext.textSecondary),
              ),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                try {
                  final svc = ref.read(characterServiceProvider);
                  final ok = await svc.delete(
                    widget.characterId,
                    roleAuthority: character.roleAuthority,
                  );
                  if (!mounted) return;
                  if (ok) ref.invalidate(characterListProvider);
                  ScaffoldMessenger.of(this.context).showSnackBar(
                    SnackBar(
                      content: Text(ok ? '已删除角色：${character.name}' : '删除失败'),
                      backgroundColor: ok ? null : this.context.error,
                    ),
                  );
                  if (ok) Navigator.of(this.context).pop();
                } catch (error) {
                  if (mounted) {
                    ScaffoldMessenger.of(
                      this.context,
                    ).showSnackBar(SnackBar(content: Text('删除角色失败：$error')));
                  }
                }
              },
              child: Text('删除', style: TextStyle(color: dialogContext.error)),
            ),
          ],
        );
      },
    );
  }
}

class _ManageEntry {
  final String title;
  final IconData icon;
  final String route;

  _ManageEntry({required this.title, required this.icon, required this.route});
}
