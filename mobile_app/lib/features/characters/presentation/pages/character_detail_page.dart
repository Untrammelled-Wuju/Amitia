import 'dart:convert';

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
import '../../../../core/backend_connection/backend_uri_builder.dart';
import '../../../../core/backend_connection/providers/backend_connection_providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_drawer.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/memory.dart';
import '../../../../core/ui_runtime/mobile_extension_slot.dart';
import '../../../../core/ui_runtime/ui_runtime_controller.dart';

class CharacterDetailPage extends ConsumerStatefulWidget {
  final String characterId;

  const CharacterDetailPage({super.key, required this.characterId});

  @override
  ConsumerState<CharacterDetailPage> createState() => _CharacterDetailPageState();
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
  bool _savingSettings = false;
  bool _uploadingAvatar = false;

  final _tabs = const ['概览', '设定', '记忆', '关系', '能力', '生活'];

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
    _personalityConfigController.text = encoder.convert(character.personalityConfig);
    _chatStyleConfigController.text = encoder.convert(character.chatStyleConfig);
    _sceneRulesController.text = encoder.convert(character.sceneRules);
  }

  @override
  Widget build(BuildContext context) {
    final characterAsync = ref.watch(characterListProvider);
    final memoriesAsync = ref.watch(memoryListProvider);
    final isDevMode = ref.watch(isDeveloperModeProvider);
    final backendAvailability = ref.watch(backendConnectionProvider).valueOrNull;
    final uiSnapshot = ref.watch(uiRuntimeProvider).valueOrNull;
    final hasExtensionTab =
        uiSnapshot?.contributionsForSlot('character.detail.tab').isNotEmpty ?? false;

    return characterAsync.when(
      loading: () => const AmitiaScaffold(
        body: Center(child: CircularProgressIndicator()),
      ),
      error: (err, _) => AmitiaScaffold(
        body: Center(child: Text('加载失败: $err')),
      ),
      data: (characters) {
        final character = characters.cast<CharacterDto?>().firstWhere(
          (c) => c?.id == widget.characterId,
          orElse: () => null,
        );
        if (character == null) {
          return const AmitiaScaffold(
            body: Center(child: Text('角色不存在')),
          );
        }
        _syncControllers(character);

        return memoriesAsync.when(
          loading: () => const AmitiaScaffold(
            body: Center(child: CircularProgressIndicator()),
          ),
          error: (_, __) => _buildScaffold(context, character, [], isDevMode, hasExtensionTab, backendAvailability),
          data: (memories) => _buildScaffold(
            context,
            character,
            memories.take(5).toList(),
            isDevMode,
            hasExtensionTab,
            backendAvailability,
          ),
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
    List<MemoryDto> memories,
    bool isDevMode,
    bool hasExtensionTab,
    BackendConnectionAvailability? backendAvailability,
  ) {
    final tabs = <String>[..._tabs, if (hasExtensionTab) '扩展'];
    final selectedTab = _selectedTab < tabs.length ? _selectedTab : 0;
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
            _buildInfoSection(context, character, backendAvailability),
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
                  setState(() {
                    _selectedTab = index;
                  });
                },
              ),
            ),
            Expanded(
              child: _buildTabContent(
                context,
                character,
                memories,
                isDevMode,
                hasExtensionTab,
              ),
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
  ) {
    final normalizedStatus = character.status.toLowerCase();
    final isOnline = character.isActive == 1 || normalizedStatus == '在线' || normalizedStatus == 'enabled';
    final initial = character.name.isNotEmpty ? character.name[0] : '?';
    final avatarUrl = _resolveAvatarUrl(character.avatar, backendAvailability);

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
            onTap: _uploadingAvatar ? null : () => _uploadAvatar(character),
            customBorder: const CircleBorder(),
            child: Stack(
              children: [
                Container(
                  width: 72,
                  height: 72,
                  clipBehavior: Clip.antiAlias,
                  decoration: BoxDecoration(
                    color: context.accentPrimary,
                    shape: BoxShape.circle,
                  ),
                  child: avatarUrl.isNotEmpty
                      ? Image.network(
                          avatarUrl,
                          fit: BoxFit.cover,
                          errorBuilder: (_, __, ___) => Center(
                            child: Text(
                              initial,
                              style: const TextStyle(
                                color: Colors.white,
                                fontSize: 28,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        )
                      : Center(
                          child: Text(
                            initial,
                            style: const TextStyle(
                              color: Colors.white,
                              fontSize: 28,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
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
                        : Icon(Icons.edit, size: 13, color: context.textSecondary),
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
                      border: Border.all(color: context.surfacePrimary, width: 2.5),
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
                Text(character.name, style: AppTypography.sectionTitle(context)),
                const SizedBox(height: 4),
                Row(
                  children: [
                    AmitiaStatusBadge(
                      label: character.status,
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

  String _resolveAvatarUrl(
    String raw,
    BackendConnectionAvailability? availability,
  ) {
    final avatar = raw.trim();
    if (avatar.isEmpty) return '';
    final uri = Uri.tryParse(avatar);
    if (uri != null && uri.hasScheme) return avatar;
    if (!avatar.startsWith('/') || availability is! BackendConnectionAvailable) {
      return avatar;
    }
    return BackendUriBuilder().http(availability.config, avatar).toString();
  }

  Future<void> _saveCharacterSettings(CharacterDto character) async {
    if (_savingSettings) return;
    final name = _nameController.text.trim();
    if (name.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('角色名称不能为空')),
      );
      return;
    }
    Map<String, dynamic> personalityConfig;
    Map<String, dynamic> chatStyleConfig;
    Map<String, dynamic> sceneRules;
    try {
      personalityConfig = _decodeJsonObject(_personalityConfigController.text, '人格参数');
      chatStyleConfig = _decodeJsonObject(_chatStyleConfigController.text, '聊天风格配置');
      sceneRules = _decodeJsonObject(_sceneRulesController.text, '场景规则');
    } catch (e) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(e.toString().replaceFirst('FormatException: ', ''))));
      return;
    }
    setState(() => _savingSettings = true);
    try {
      await ref.read(characterServiceProvider).update(character.id, <String, dynamic>{
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
      });
      _loadedCharacterId = '';
      ref.invalidate(characterListProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('角色设定已保存')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('保存失败：$e')),
        );
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
    final path = picked.files.single.path;
    if (path == null || path.isEmpty) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('无法读取所选头像文件')),
        );
      }
      return;
    }
    setState(() => _uploadingAvatar = true);
    try {
      final result = await ref.read(characterDetailServiceProvider).uploadAvatar(character.id, path);
      final avatarUrl = (result?['avatarUrl'] ?? '').toString();
      if (avatarUrl.isEmpty) {
        throw StateError('后端未返回头像地址');
      }
      _loadedCharacterId = '';
      ref.invalidate(characterListProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('头像已更新')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('头像上传失败：$e')),
        );
      }
    } finally {
      if (mounted) setState(() => _uploadingAvatar = false);
    }
  }

  Widget _buildTabContent(
    BuildContext context,
    CharacterDto character,
    List<MemoryDto> memories,
    bool isDevMode,
    bool hasExtensionTab,
  ) {
    switch (_selectedTab) {
      case 0:
        return _buildOverviewTab(context, character, memories, isDevMode);
      case 1:
        return _buildSettingsTab(context, character);
      case 2:
        return _buildMemoryTab(context, memories);
      case 3:
        return _buildRelationTab(context, character);
      case 4:
        return _buildAbilityTab(context, character);
      case 5:
        return _buildLifeTab(context, character);
      case 6:
        if (hasExtensionTab) {
          return ListView(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            children: [
              MobileExtensionSlot(
                slotId: 'character.detail.tab',
                context: _characterSlotContext(character),
              ),
            ],
          );
        }
        return const SizedBox.shrink();
      default:
        return const SizedBox.shrink();
    }
  }

  Widget _buildOverviewTab(BuildContext context, CharacterDto character, List<MemoryDto> memories, bool isDevMode) {
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
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('最近记忆', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              ...memories.take(3).map((m) => _buildMemoryItem(context, m)),
              if (memories.isEmpty)
                Text('暂无记忆', style: AppTypography.caption(context)),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        _buildManagementSection(context, character, isDevMode),
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
              _buildLabeledField(context, '性格', _personalityController, '角色性格', maxLines: 3),
              _buildLabeledField(context, '说话方式', _speakingStyleController, '说话风格', maxLines: 3),
              _buildLabeledField(context, '关系风格', _relationshipStyleController, '与用户相处和建立关系的方式', maxLines: 3),
              _buildLabeledField(context, '角色基础设定', _characterBaseController, '不可轻易变化的角色基础信息', maxLines: 4),
              _buildLabeledField(context, '边界规则', _boundaryRulesController, '角色需要长期遵守的行为边界', maxLines: 4),
              _buildLabeledField(context, '简介', _descriptionController, '角色简介', maxLines: 3),
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
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('高级人格与场景配置', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.sm),
              _buildLabeledField(context, '人格参数 JSON', _personalityConfigController, '{}', maxLines: 8),
              _buildLabeledField(context, '聊天风格 JSON', _chatStyleConfigController, '{}', maxLines: 6),
              _buildLabeledField(context, '场景规则 JSON', _sceneRulesController, '{}', maxLines: 6),
            ],
          ),
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaButton(
          label: _savingSettings ? '保存中...' : '保存角色设定',
          icon: Icons.save_outlined,
          isFullWidth: true,
          onPressed: _savingSettings ? null : () => _saveCharacterSettings(character),
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

  Widget _buildMemoryTab(BuildContext context, List<MemoryDto> memories) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('最近记忆', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              ...memories.map((m) => _buildMemoryItem(context, m)),
              if (memories.isEmpty)
                Text('暂无记忆', style: AppTypography.caption(context)),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildRelationTab(BuildContext context, CharacterDto character) {
    return FutureBuilder<Map<String, dynamic>?>(
      future: ref.read(temporalServiceProvider).relationshipTimeState(character.id),
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
                    _buildStatItem(context, character.isActive == 1 ? '是' : '否', '活跃状态'),
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
                      Expanded(child: Text('关系时间状态', style: AppTypography.cardTitle(context))),
                      if (snapshot.connectionState == ConnectionState.waiting)
                        const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  if (snapshot.hasError)
                    Text('加载失败：${snapshot.error}', style: AppTypography.caption(context).copyWith(color: context.error))
                  else if (state.isEmpty && snapshot.connectionState != ConnectionState.waiting)
                    Text('尚未产生可展示的关系时间状态', style: AppTypography.caption(context))
                  else ...[
                    _buildInfoRow(context, '首次互动', _displayTime(state['firstInteractionAt'])),
                    _buildInfoRow(context, '关系持续', '${_intValue(state['relationshipAgeDays'])} 天'),
                    _buildInfoRow(context, '互动次数', '${_intValue(state['interactionCount'])}'),
                    _buildInfoRow(context, '会话次数', '${_intValue(state['sessionCount'])}'),
                    _buildInfoRow(context, '最近互动', _displayTime(state['lastSuccessfulExchangeAt'])),
                    _buildInfoRow(context, '期望间隔', _formatDuration(state['expectedGapSeconds'])),
                    _buildInfoRow(context, '连续性', _formatScore(state['continuityScore'])),
                    _buildInfoRow(context, '重新适应', '${_intValue(state['reacclimationTurnsLeft'])} 回合'),
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
    final activeModels = configs.where((item) => item.isActive == 1).toList(growable: false);
    final activeModel = activeModels.isEmpty ? null : activeModels.first;
    final modelLabel = activeModel == null
        ? (configsAsync.isLoading ? '加载中...' : '未配置激活模型')
        : '${activeModel.name.isEmpty ? activeModel.provider : activeModel.name}${activeModel.model.isEmpty ? '' : ' · ${activeModel.model}'}';
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        _buildAbilitySection(context, '模型', modelLabel, Icons.psychology_outlined),
        SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(context, '语音类型', character.voiceType ?? '默认', Icons.record_voice_over_outlined),
        SizedBox(height: AppSpacing.sm),
        _buildAbilitySection(
          context,
          '语音参数',
          '速度 ${character.voiceSpeed?.toStringAsFixed(2) ?? '-'} · 音调 ${character.voicePitch?.toStringAsFixed(2) ?? '-'} · 音量 ${character.voiceVolume?.toStringAsFixed(2) ?? '-'}',
          Icons.tune_outlined,
        ),
        SizedBox(height: AppSpacing.sm),
        AmitiaCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text('Emote AI 策略', style: AppTypography.cardTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text('控制 AI 回复时自动发送角色表情的概率、频率与冷却规则。', style: AppTypography.caption(context)),
              SizedBox(height: AppSpacing.md),
              AmitiaButton(
                label: '配置表情策略',
                icon: Icons.emoji_emotions_outlined,
                isSecondary: true,
                isFullWidth: true,
                onPressed: () => _showEmoteSettings(character),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Future<void> _showEmoteSettings(CharacterDto character) async {
    try {
      final service = ref.read(emoteServiceProvider);
      final raw = await service.getSettings(character.id) ?? const <String, dynamic>{};
      if (!mounted) return;
      var enabled = _boolValue(raw['enabled'], fallback: true);
      var allowEmoteOnly = _boolValue(raw['allowEmoteOnly']);
      final baseController = TextEditingController(text: ((_doubleValue(raw['baseProbability']) == 0 && !raw.containsKey('baseProbability')) ? 0.10 : _doubleValue(raw['baseProbability'])).toString());
      final maxController = TextEditingController(text: ((_doubleValue(raw['maxProbability']) == 0 && !raw.containsKey('maxProbability')) ? 0.30 : _doubleValue(raw['maxProbability'])).toString());
      final perHourController = TextEditingController(text: (_intValue(raw['maxPerHour']) == 0 && !raw.containsKey('maxPerHour') ? 5 : _intValue(raw['maxPerHour'])).toString());
      final gapController = TextEditingController(text: (_intValue(raw['minReplyGap']) == 0 && !raw.containsKey('minReplyGap') ? 3 : _intValue(raw['minReplyGap'])).toString());
      final cooldownController = TextEditingController(text: (_intValue(raw['sameEmoteCooldownMinutes']) == 0 && !raw.containsKey('sameEmoteCooldownMinutes') ? 30 : _intValue(raw['sameEmoteCooldownMinutes'])).toString());
      var saving = false;
      try {
        await showDialog<void>(
          context: context,
          builder: (dialogContext) => StatefulBuilder(
            builder: (dialogContext, setDialogState) {
              Future<void> save() async {
                if (saving) return;
                var base = double.tryParse(baseController.text.trim()) ?? 0.10;
                var max = double.tryParse(maxController.text.trim()) ?? 0.30;
                base = base.clamp(0, 1).toDouble();
                max = max.clamp(0, 1).toDouble();
                if (max < base) max = base;
                final maxPerHour = (int.tryParse(perHourController.text.trim()) ?? 5).clamp(1, 100);
                final minReplyGap = (int.tryParse(gapController.text.trim()) ?? 3).clamp(0, 1000);
                final cooldown = (int.tryParse(cooldownController.text.trim()) ?? 30).clamp(0, 1440);
                setDialogState(() => saving = true);
                try {
                  await service.saveSettings(character.id, <String, dynamic>{
                    'enabled': enabled,
                    'baseProbability': base,
                    'maxProbability': max,
                    'maxPerHour': maxPerHour,
                    'minReplyGap': minReplyGap,
                    'sameEmoteCooldownMinutes': cooldown,
                    'allowEmoteOnly': allowEmoteOnly,
                  });
                  if (dialogContext.mounted) Navigator.pop(dialogContext);
                  if (mounted) ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('表情策略已保存')));
                } catch (e) {
                  if (dialogContext.mounted) {
                    ScaffoldMessenger.of(dialogContext).showSnackBar(SnackBar(content: Text('保存失败：$e')));
                    setDialogState(() => saving = false);
                  }
                }
              }

              return AlertDialog(
                title: const Text('Emote AI 策略'),
                content: SizedBox(
                  width: 560,
                  child: SingleChildScrollView(
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        SwitchListTile(
                          contentPadding: EdgeInsets.zero,
                          title: const Text('启用 AI 自动表情'),
                          value: enabled,
                          onChanged: saving ? null : (value) => setDialogState(() => enabled = value),
                        ),
                        _buildDialogNumberField(baseController, '基础发送概率', '0.00 - 1.00', enabled && !saving),
                        _buildDialogNumberField(maxController, '最大发送概率', '0.00 - 1.00', enabled && !saving),
                        _buildDialogNumberField(perHourController, '每小时最多发送', '次数', enabled && !saving, decimal: false),
                        _buildDialogNumberField(gapController, '最小回复间隔', '回复轮数', enabled && !saving, decimal: false),
                        _buildDialogNumberField(cooldownController, '同一表情冷却', '分钟', enabled && !saving, decimal: false),
                        SwitchListTile(
                          contentPadding: EdgeInsets.zero,
                          title: const Text('允许仅发送表情'),
                          subtitle: const Text('关闭时优先在文字回复之后发送表情'),
                          value: allowEmoteOnly,
                          onChanged: !enabled || saving ? null : (value) => setDialogState(() => allowEmoteOnly = value),
                        ),
                      ],
                    ),
                  ),
                ),
                actions: [
                  TextButton(onPressed: saving ? null : () => Navigator.pop(dialogContext), child: const Text('取消')),
                  TextButton(onPressed: saving ? null : save, child: Text(saving ? '保存中...' : '保存')),
                ],
              );
            },
          ),
        );
      } finally {
        baseController.dispose();
        maxController.dispose();
        perHourController.dispose();
        gapController.dispose();
        cooldownController.dispose();
      }
    } catch (e) {
      if (mounted) ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('表情策略加载失败：$e')));
    }
  }

  Widget _buildDialogNumberField(
    TextEditingController controller,
    String label,
    String hint,
    bool enabled, {
    bool decimal = true,
  }) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.sm),
      child: TextField(
        controller: controller,
        enabled: enabled,
        keyboardType: TextInputType.numberWithOptions(decimal: decimal),
        decoration: InputDecoration(labelText: label, helperText: hint),
      ),
    );
  }

  Widget _buildLifeTab(BuildContext context, CharacterDto character) {
    final companion = ref.read(companionServiceProvider);
    return FutureBuilder<List<dynamic>>(
      future: Future.wait<dynamic>([
        companion.state(characterId: character.id),
        companion.schedule(characterId: character.id),
        companion.workProfile(characterId: character.id),
        companion.sleepSetting(characterId: character.id),
      ]),
      builder: (context, snapshot) {
        final values = snapshot.data ?? const <dynamic>[];
        final state = values.isNotEmpty ? values[0] : null;
        final schedule = values.length > 1 && values[1] is Map ? Map<String, dynamic>.from(values[1] as Map) : const <String, dynamic>{};
        final work = values.length > 2 && values[2] is Map ? Map<String, dynamic>.from(values[2] as Map) : const <String, dynamic>{};
        final sleep = values.length > 3 && values[3] is Map ? Map<String, dynamic>.from(values[3] as Map) : const <String, dynamic>{};
        return ListView(
          padding: EdgeInsets.all(AppSpacing.pagePadding),
          children: [
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(child: Text('当前状态', style: AppTypography.cardTitle(context))),
                      if (snapshot.connectionState == ConnectionState.waiting)
                        const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2)),
                    ],
                  ),
                  SizedBox(height: AppSpacing.sm),
                  _buildInfoRow(context, '状态', state?.state ?? character.status),
                  _buildInfoRow(context, '当前活动', state?.currentActivity ?? '-'),
                  _buildInfoRow(context, '下一活动', state?.nextActivity ?? '-'),
                  _buildInfoRow(context, '睡眠中', state?.isSleeping == true ? '是' : '否'),
                  _buildInfoRow(context, '生活场景', character.lifeIdentity),
                ],
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('今日作息', style: AppTypography.cardTitle(context)),
                  SizedBox(height: AppSpacing.sm),
                  if (schedule.isEmpty && snapshot.connectionState != ConnectionState.waiting)
                    Text('暂无作息数据', style: AppTypography.caption(context))
                  else ...[
                    _buildScheduleItem(context, _clock(schedule['wakeTime']), '起床'),
                    _buildScheduleItem(context, _clock(schedule['lunchTime']), '午饭'),
                    if (schedule['hasNap'] == true)
                      _buildScheduleItem(context, '${_clock(schedule['napStartTime'])} - ${_clock(schedule['napEndTime'])}', '午睡'),
                    _buildScheduleItem(context, _clock(schedule['dinnerTime']), '晚饭'),
                    _buildScheduleItem(context, _clock(schedule['sleepTime']), '睡觉'),
                  ],
                ],
              ),
            ),
            SizedBox(height: AppSpacing.sm),
            AmitiaCard(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text('工作与睡眠配置', style: AppTypography.cardTitle(context)),
                  SizedBox(height: AppSpacing.sm),
                  _buildInfoRow(context, '工作状态', work['enabled'] == true ? '启用' : '未启用'),
                  _buildInfoRow(context, '工作时间', '${work['workStartTime'] ?? '-'} - ${work['workEndTime'] ?? '-'}'),
                  _buildInfoRow(context, '睡眠规则', sleep['enabled'] == false ? '关闭' : '启用'),
                  _buildInfoRow(context, '睡眠时间', '${sleep['bedTime'] ?? '-'} - ${sleep['wakeTime'] ?? '-'}'),
                ],
              ),
            ),
          ],
        );
      },
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
            child: Text(value.isEmpty ? '-' : value, style: AppTypography.bodySmall(context)),
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
            style: AppTypography.sectionTitle(context).copyWith(color: context.accentPrimary),
          ),
          const SizedBox(height: 2),
          Text(label, style: AppTypography.label(context)),
        ],
      ),
    );
  }

  Widget _buildMemoryItem(BuildContext context, MemoryDto memory) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 6),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Container(
            width: 6,
            height: 6,
            margin: const EdgeInsets.only(top: 7),
            decoration: BoxDecoration(
              color: context.accentPrimary,
              shape: BoxShape.circle,
            ),
          ),
          SizedBox(width: AppSpacing.sm),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  memory.content,
                  style: AppTypography.bodySmall(context),
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                ),
                const SizedBox(height: 2),
                Text(
                  '${memory.type} · ${memory.importance}',
                  style: AppTypography.label(context),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAbilitySection(
    BuildContext context,
    String title,
    String content,
    IconData icon,
  ) {
    return AmitiaCard(
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
        ],
      ),
    );
  }

  Widget _buildScheduleItem(BuildContext context, String time, String activity) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 5),
      child: Row(
        children: [
          SizedBox(
            width: 120,
            child: Text(time, style: AppTypography.caption(context)),
          ),
          Expanded(
            child: Text(activity, style: AppTypography.bodySmall(context)),
          ),
        ],
      ),
    );
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

  bool _boolValue(dynamic value, {bool fallback = false}) {
    if (value is bool) return value;
    if (value is num) return value != 0;
    final normalized = value?.toString().trim().toLowerCase();
    if (normalized == 'true' || normalized == '1' || normalized == 'yes') return true;
    if (normalized == 'false' || normalized == '0' || normalized == 'no') return false;
    return fallback;
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

  String _clock(dynamic value) {
    final text = value?.toString().trim() ?? '';
    if (text.isEmpty) return '-';
    final parsed = DateTime.tryParse(text);
    if (parsed == null) {
      final match = RegExp(r'\b([01]?\d|2[0-3]):[0-5]\d\b').firstMatch(text);
      return match?.group(0) ?? text;
    }
    final local = parsed.toLocal();
    return '${local.hour.toString().padLeft(2, '0')}:${local.minute.toString().padLeft(2, '0')}';
  }

  Widget _buildManagementSection(BuildContext context, CharacterDto character, bool isDevMode) {
    final entries = <_ManageEntry>[
      _ManageEntry(
        title: '生活规则',
        icon: Icons.rule_outlined,
        route: AppRoutes.characterLifeRules(widget.characterId),
      ),
      _ManageEntry(
        title: '语音与声音复刻',
        icon: Icons.record_voice_over_outlined,
        route: AppRoutes.characterVoice(widget.characterId),
      ),
      _ManageEntry(
        title: '角色记忆',
        icon: Icons.memory_outlined,
        route: AppRoutes.characterMemory(widget.characterId),
      ),
      _ManageEntry(
        title: '关系时间线',
        icon: Icons.timeline_outlined,
        route: AppRoutes.characterTimeline(widget.characterId),
      ),
      _ManageEntry(
        title: '主动消息',
        icon: Icons.send_outlined,
        route: AppRoutes.characterProactive(widget.characterId),
      ),
      _ManageEntry(
        title: '心理状态',
        icon: Icons.psychology_outlined,
        route: AppRoutes.characterPsyche(widget.characterId),
      ),
      if (isDevMode)
        _ManageEntry(
          title: '调试与诊断',
          icon: Icons.bug_report_outlined,
          route: AppRoutes.characterDebug(widget.characterId),
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
          ...entries.map((e) => AmitiaListTile(
                leading: _buildManagementIcon(context, e.icon),
                title: e.title,
                trailing: Icon(Icons.chevron_right, color: context.textTertiary, size: 20),
                onTap: () => context.push(e.route),
              )),
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

  void _showCharacterActionsSheet(BuildContext context, CharacterDto character) {
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
                Text('角色操作', style: AppTypography.pageTitle(context)),
                const SizedBox(height: 16),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.star_outline, context.accentPrimary),
                  title: '设为当前角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _setAsCurrent(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.copy_outlined, context.accentPrimary),
                  title: '复制角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _copyCharacter(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.file_download_outlined, context.accentPrimary),
                  title: '导出角色卡',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _exportCharacter(character);
                  },
                ),
                AmitiaListTile(
                  leading: _buildActionIcon(context, Icons.delete_outline, context.error),
                  title: '删除角色',
                  onTap: () {
                    Navigator.pop(sheetContext);
                    _showDeleteConfirm(context, character);
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

  Future<void> _setAsCurrent(CharacterDto character) async {
    ref.read(currentCharacterIdProvider.notifier).state = widget.characterId;
    final svc = ref.read(characterServiceProvider);
    await svc.setActive(widget.characterId);
    ref.invalidate(characterListProvider);
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text('已将「${character.name}」设为当前角色')),
      );
    }
  }

  Future<void> _copyCharacter(CharacterDto character) async {
    try {
      final created = await ref.read(characterServiceProvider).duplicate(character.id);
      if (created == null) throw StateError('后端未返回复制后的角色');
      ref.invalidate(characterListProvider);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('已复制为「${created.name}」')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('复制角色失败：$e')),
        );
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
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('已导出角色卡：${character.name}')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('导出失败：$e')));
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
              child: Text('取消', style: TextStyle(color: dialogContext.textSecondary)),
            ),
            TextButton(
              onPressed: () async {
                Navigator.pop(dialogContext);
                final svc = ref.read(characterServiceProvider);
                final ok = await svc.delete(widget.characterId);
                if (ok) {
                  ref.invalidate(characterListProvider);
                }
                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(ok ? '已删除角色：${character.name}' : '删除失败'),
                      backgroundColor: ok ? null : context.error,
                    ),
                  );
                  if (ok) Navigator.of(context).pop();
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

  _ManageEntry({
    required this.title,
    required this.icon,
    required this.route,
  });
}
