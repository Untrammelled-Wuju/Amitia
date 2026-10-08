import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/models/character.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/services/role_authority.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../widgets/character_generation_chat.dart';
import '../widgets/character_personality_editor.dart';

class CharacterCardWorkshopPage extends ConsumerStatefulWidget {
  final bool creating;
  final CharacterDto? character;

  const CharacterCardWorkshopPage({
    super.key,
    this.creating = false,
    this.character,
  });

  @override
  ConsumerState<CharacterCardWorkshopPage> createState() =>
      _CharacterCardWorkshopPageState();
}

class _CharacterCardWorkshopPageState
    extends ConsumerState<CharacterCardWorkshopPage> {
  static const _profileLabels = {
    'name': '名称',
    'avatar': '头像 URL',
    'identity': '身份',
    'personality': '性格',
    'speakingStyle': '说话风格',
    'relationshipStyle': '关系氛围',
    'boundaryRules': '安全边界规则',
  };
  final _profile = {
    for (final key in _profileLabels.keys) key: TextEditingController(),
  };
  Map<String, dynamic> _personalityConfig = {...characterPersonalityDefaults};
  bool _creating = false;
  bool _editing = false;
  bool _isActive = false;
  int _generationSession = 0;
  String? _pendingAvatarPath;
  final _description = TextEditingController();
  final _scenario = TextEditingController();
  final _systemPrompt = TextEditingController();
  final _exampleMessages = TextEditingController();
  final _alternateGreetings = TextEditingController();
  final _postHistory = TextEditingController();
  final _creator = TextEditingController();
  final _characterVersion = TextEditingController();
  final _tags = TextEditingController();
  Map<String, dynamic> _cardData = <String, dynamic>{};
  String _selectedId = '';
  bool _loading = false;
  bool _cardLoadFailed = false;
  bool _saving = false;
  bool _importing = false;
  bool _exporting = false;
  String _roleAuthority = '';

  bool get _inEditor => widget.creating || widget.character != null;

  @override
  void initState() {
    super.initState();
    if (widget.creating) _newDraft();
    if (widget.character != null) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) _selectCharacter(widget.character!);
      });
    }
  }

  Future<void> _openEditor({CharacterDto? character}) async {
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => CharacterCardWorkshopPage(
          creating: character == null,
          character: character,
        ),
      ),
    );
    if (mounted) ref.invalidate(characterListProvider);
  }

  void _back() {
    if (_creating && _editing) {
      setState(() => _editing = false);
    } else {
      Navigator.of(context).pop();
    }
  }

  @override
  void dispose() {
    for (final controller in _profile.values) {
      controller.dispose();
    }
    _description.dispose();
    _scenario.dispose();
    _systemPrompt.dispose();
    _exampleMessages.dispose();
    _alternateGreetings.dispose();
    _postHistory.dispose();
    _creator.dispose();
    _characterVersion.dispose();
    _tags.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final charactersAsync = ref.watch(characterListProvider);
    return PopScope(
      canPop: !_saving && !(_creating && _editing),
      onPopInvokedWithResult: (didPop, result) {
        if (!didPop && !_saving && _creating && _editing) _back();
      },
      child: AmitiaScaffold(
        appBar: AmitiaAppBar(
          title: _inEditor ? (_creating ? '创建角色卡' : '编辑角色卡') : '角色卡工坊',
          leading: _inEditor
              ? IconButton(
                  icon: const Icon(Icons.arrow_back_ios_new, size: 18),
                  tooltip: _creating && _editing ? '上一步' : '返回工坊',
                  onPressed: _saving ? null : _back,
                )
              : null,
          showBackButton: true,
          fallbackRoute: AppRoutes.workshop,
          actions: [
            if (!_inEditor)
              AmitiaIconButton(
                icon: Icons.file_upload_outlined,
                tooltip: '导入角色卡',
                onPressed: _importing || _saving || _loading
                    ? null
                    : _importCard,
              ),
            AmitiaIconButton(
              icon: Icons.file_download_outlined,
              tooltip: '导出 CHARX',
              onPressed:
                  _exporting || _saving || _loading || _selectedId.isEmpty
                  ? null
                  : _exportCard,
            ),
          ],
        ),
        body: SafeArea(
          top: false,
          child: IgnorePointer(
            ignoring: _saving,
            child: charactersAsync.when(
              loading: () => const AmitiaLoadingState(message: '正在加载角色...'),
              error: (err, _) => AmitiaErrorState(
                message: '角色加载失败：$err',
                onRetry: () => ref.invalidate(characterListProvider),
              ),
              data: (characters) => _buildBody(context, characters),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildBody(BuildContext context, List<CharacterDto> characters) {
    if (!_inEditor && characters.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.badge_outlined,
        title: '还没有角色卡',
        subtitle: '创建角色卡，通过对话生成后手动调整',
        actionText: '创建角色卡',
        onAction: () => _openEditor(),
      );
    }
    final selected = characters
        .where((item) => item.id == _selectedId)
        .firstOrNull;
    if (_creating) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Padding(
            padding: EdgeInsets.all(AppSpacing.pagePadding),
            child: Text(
              _editing ? '第 2 步：编辑角色卡' : '第 1 步：对话生成',
              style: Theme.of(context).textTheme.titleMedium,
            ),
          ),
          Expanded(
            child: Stack(
              fit: StackFit.expand,
              children: [
                Offstage(
                  offstage: _editing,
                  child: CharacterGenerationChat(
                    key: ValueKey(_generationSession),
                    currentDraft: _draftSnapshot,
                    onApply: _applyGenerated,
                  ),
                ),
                Offstage(
                  offstage: !_editing,
                  child: ListView(
                    padding: EdgeInsets.all(AppSpacing.pagePadding),
                    children: [_buildEditor(context, selected)],
                  ),
                ),
              ],
            ),
          ),
        ],
      );
    }
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        if (!_inEditor) ...[
          OutlinedButton.icon(
            onPressed: () => _openEditor(),
            icon: const Icon(Icons.add),
            label: const Text('创建角色卡'),
          ),
          for (final character in characters)
            ListTile(
              title: Text(character.name),
              subtitle: Text(character.identity),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => _openEditor(character: character),
            ),
        ],
        SizedBox(height: AppSpacing.lg),
        if (_loading)
          const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: CircularProgressIndicator(),
            ),
          )
        else if (_inEditor && _cardLoadFailed && selected != null)
          AmitiaErrorState(
            message: '角色卡加载失败，请重试后再编辑',
            onRetry: () => _selectCharacter(selected),
          )
        else if (_inEditor && (selected != null || _creating)) ...[
          SizedBox(height: AppSpacing.lg),
          Offstage(offstage: !_editing, child: _buildEditor(context, selected)),
        ],
      ],
    );
  }

  Widget _buildEditor(BuildContext context, CharacterDto? character) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final field in _profileLabels.entries)
          _field(
            field.value,
            _profile[field.key]!,
            maxLines: field.key == 'boundaryRules' ? 4 : 1,
          ),
        OutlinedButton.icon(
          onPressed: _saving ? null : _pickAvatar,
          icon: const Icon(Icons.add_photo_alternate_outlined),
          label: Text(_pendingAvatarPath == null ? '上传角色头像' : '已选择头像，保存时上传'),
        ),
        SizedBox(height: AppSpacing.lg),
        _field('角色描述', _description, maxLines: 3),
        _field('场景设定', _scenario, maxLines: 3),
        _field('System Prompt', _systemPrompt, maxLines: 8),
        _field('示例对话', _exampleMessages, maxLines: 5),
        _field('备选问候', _alternateGreetings, maxLines: 5, hint: '每行一条备选问候'),
        _field('Post-history 指令', _postHistory, maxLines: 4),
        _field('创作者', _creator),
        _field('角色卡版本', _characterVersion),
        _field('标签', _tags, hint: '使用英文逗号分隔'),
        CharacterPersonalityEditor(
          value: _personalityConfig,
          onChanged: (value) => setState(() => _personalityConfig = value),
        ),
        CheckboxListTile(
          title: const Text('设为当前启用角色'),
          value: _isActive,
          onChanged: character?.isActive == 1 || _saving
              ? null
              : (value) => setState(() => _isActive = value ?? false),
        ),
        SizedBox(height: AppSpacing.lg),
        AmitiaButton(
          label: _saving ? '保存中...' : '保存角色卡',
          icon: Icons.save_outlined,
          isFullWidth: true,
          onPressed: _saving ? null : () => _saveCard(character),
        ),
      ],
    );
  }

  Widget _field(
    String label,
    TextEditingController controller, {
    int maxLines = 1,
    String? hint,
  }) {
    return Padding(
      padding: EdgeInsets.only(bottom: AppSpacing.md),
      child: TextField(
        controller: controller,
        maxLines: maxLines,
        decoration: InputDecoration(labelText: label, hintText: hint),
      ),
    );
  }

  Future<void> _selectCharacter(CharacterDto character) async {
    if (_saving) return;
    setState(() {
      _roleAuthority = character.roleAuthority;
      _selectedId = character.id;
      _loading = true;
      _cardLoadFailed = false;
      _cardData = {};
      _creating = false;
      _editing = true;
      _generationSession++;
      _pendingAvatarPath = null;
      _isActive = character.isActive == 1;
    });
    final profile = {
      'name': character.name,
      'avatar': character.avatar,
      'identity': character.identity,
      'personality': character.personality,
      'speakingStyle': character.speakingStyle,
      'relationshipStyle': character.relationshipStyle,
      'boundaryRules': character.boundaryRules,
    };
    for (final entry in profile.entries) {
      _profile[entry.key]!.text = entry.value;
    }
    _personalityConfig = {
      ...characterPersonalityDefaults,
      ...character.personalityConfig,
    };
    _description.text = character.description;
    final generation = _generationSession;
    try {
      final data = await ref
          .read(backendServiceProvider)
          .get<Map<String, dynamic>>(
            '/api/characters/${character.id}/card-data',
            headers: roleAuthorityHeaders(_roleAuthority),
            fromJson: (value) => Map<String, dynamic>.from(value as Map),
          );
      if (!mounted ||
          _selectedId != character.id ||
          generation != _generationSession)
        return;
      _cardData = data ?? <String, dynamic>{};
      _description.text = (data?['description'] ?? character.description)
          .toString();
      _scenario.text = (_cardData['scenario'] ?? '').toString();
      _systemPrompt.text =
          (_cardData['systemPrompt'] ?? character.characterBase).toString();
      _exampleMessages.text = (_cardData['exampleMessages'] ?? '').toString();
      _alternateGreetings.text =
          (_cardData['alternateGreetings'] as List?)
              ?.map((item) => item.toString())
              .join('\n') ??
          '';
      _postHistory.text = (_cardData['postHistoryInstructions'] ?? '')
          .toString();
      _creator.text = (_cardData['creator'] ?? '').toString();
      _characterVersion.text = (_cardData['characterVersion'] ?? '').toString();
      _tags.text =
          (_cardData['tags'] as List?)
              ?.map((item) => item.toString())
              .join(', ') ??
          '';
    } catch (error) {
      if (mounted &&
          _selectedId == character.id &&
          generation == _generationSession) {
        _cardLoadFailed = true;
        amitiaSnackBar(context, '角色卡加载失败：$error');
      }
    } finally {
      if (mounted &&
          _selectedId == character.id &&
          generation == _generationSession) {
        setState(() => _loading = false);
      }
    }
  }

  Future<void> _saveCard(CharacterDto? character) async {
    if (_saving || _loading || _cardLoadFailed) return;
    if (_profile['name']!.text.trim().isEmpty) {
      amitiaSnackBar(context, '请输入角色名称');
      return;
    }
    setState(() => _saving = true);
    try {
      final api = ref.read(backendServiceProvider);
      final authority = _roleAuthority;
      final intent = roleAuthorityHeaders(authority);
      final cardPayload = {
        ..._cardData,
        ..._draftSnapshot(),
        'systemPrompt': _systemPrompt.text.trim(),
      };
      final avatarPath = _pendingAvatarPath;
      final activate = _isActive;
      final payload = {
        for (final entry in _profile.entries)
          entry.key: entry.value.text.trim(),
        'description': _description.text.trim(),
        'characterBase': _systemPrompt.text.trim(),
        'personalityConfig': _personalityConfig,
      };
      if (_selectedId.isEmpty) {
        final created = await ref
            .read(characterServiceProvider)
            .create(payload, roleAuthority: authority);
        if (created == null || created.id.isEmpty) throw StateError('角色创建失败');
        if (!mounted) return;
        setState(() => _selectedId = created.id);
      } else {
        await ref
            .read(characterServiceProvider)
            .update(_selectedId, payload, roleAuthority: authority);
      }
      if (avatarPath != null) {
        final uploaded = await ref
            .read(characterDetailServiceProvider)
            .uploadAvatar(_selectedId, avatarPath, roleAuthority: authority);
        final avatarUrl = uploaded?['avatarUrl'];
        if (avatarUrl is! String || avatarUrl.isEmpty) {
          throw StateError('头像上传未返回有效地址');
        }
        if (!mounted) return;
        _profile['avatar']!.text = avatarUrl;
        _pendingAvatarPath = null;
      }
      await api.put<Map<String, dynamic>>(
        '/api/characters/$_selectedId/card-data',
        data: cardPayload,
        headers: intent,
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (activate) {
        await ref
            .read(characterServiceProvider)
            .setActive(_selectedId, roleAuthority: authority);
      }
      ref.invalidate(characterListProvider);
      await ref.read(characterListProvider.future);
      if (mounted) {
        amitiaSnackBar(context, '角色卡已保存');
        setState(() {
          _creating = false;
          _saving = false;
        });
        await WidgetsBinding.instance.endOfFrame;
        if (!mounted) return;
        Navigator.of(context).pop();
      }
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '角色卡保存失败：$error');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  void _newDraft() {
    if (_saving) return;
    setState(() {
      _creating = true;
      _editing = false;
      _selectedId = '';
      _roleAuthority = '';
      _generationSession++;
      _isActive = true;
      _pendingAvatarPath = null;
      _cardData = {};
      _cardLoadFailed = false;
      for (final controller in [
        ..._profile.values,
        _description,
        _scenario,
        _systemPrompt,
        _exampleMessages,
        _alternateGreetings,
        _postHistory,
        _creator,
        _characterVersion,
        _tags,
      ]) {
        controller.clear();
      }
      _personalityConfig = {...characterPersonalityDefaults};
    });
    final generation = _generationSession;
    ref
        .read(characterServiceProvider)
        .authority()
        .then((authority) {
          if (mounted &&
              generation == _generationSession &&
              _selectedId.isEmpty)
            setState(() => _roleAuthority = authority);
        })
        .catchError((Object error) {
          if (mounted && generation == _generationSession)
            amitiaSnackBar(context, '无法确认角色归属：$error');
        });
  }

  Map<String, dynamic> _draftSnapshot() => {
    for (final entry in _profile.entries) entry.key: entry.value.text,
    'description': _description.text,
    'scenario': _scenario.text,
    'characterBase': _systemPrompt.text,
    'exampleMessages': _exampleMessages.text,
    'alternateGreetings': _alternateGreetings.text
        .split('\n')
        .where((value) => value.trim().isNotEmpty)
        .toList(),
    'postHistoryInstructions': _postHistory.text,
    'creator': _creator.text,
    'characterVersion': _characterVersion.text,
    'tags': _tags.text
        .split(',')
        .map((value) => value.trim())
        .where((value) => value.isNotEmpty)
        .toList(),
    'personalityConfig': _personalityConfig,
  };

  void _applyGenerated(Map<String, dynamic> draft) {
    setState(() {
      final controllers = {
        ..._profile,
        'description': _description,
        'scenario': _scenario,
        'characterBase': _systemPrompt,
        'exampleMessages': _exampleMessages,
        'postHistoryInstructions': _postHistory,
        'creator': _creator,
        'characterVersion': _characterVersion,
      };
      for (final entry in controllers.entries) {
        if (draft[entry.key] is String) {
          entry.value.text = draft[entry.key] as String;
        }
      }
      if (draft['tags'] is List) {
        _tags.text = (draft['tags'] as List).join(', ');
      }
      if (draft['alternateGreetings'] is List) {
        _alternateGreetings.text = (draft['alternateGreetings'] as List).join(
          '\n',
        );
      }
      if (draft['personalityConfig'] is Map) {
        _personalityConfig = {
          ..._personalityConfig,
          ...Map<String, dynamic>.from(draft['personalityConfig'] as Map),
        };
      }
      _editing = true;
    });
  }

  Future<void> _pickAvatar() async {
    final picked = await FilePicker.platform.pickFiles(type: FileType.image);
    if (!mounted) return;
    final path = picked?.files.single.path;
    if (path != null) setState(() => _pendingAvatarPath = path);
  }

  Future<void> _importCard() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['json', 'png', 'charx'],
    );
    final path = picked?.files.single.path;
    if (path == null || path.isEmpty) return;
    setState(() => _importing = true);
    try {
      final api = ref.read(backendServiceProvider);
      final previewResult = await api.postMultipart<Map<String, dynamic>>(
        '/api/characters/import-card/preview',
        files: {
          'card': [path],
        },
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      final preview = previewResult?['preview'] is Map
          ? Map<String, dynamic>.from(previewResult!['preview'] as Map)
          : <String, dynamic>{};
      if (!mounted) return;
      final confirmed = await showDialog<bool>(
        context: context,
        builder: (dialogContext) => AlertDialog(
          title: const Text('导入角色卡'),
          content: Text(
            '名称：${preview['name'] ?? picked?.files.single.name ?? ''}\n'
            '格式：${previewResult?['format'] ?? preview['format'] ?? '-'}\n'
            '世界书条目：${preview['lorebookEntryCount'] ?? 0}\n\n确认导入吗？',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, false),
              child: const Text('取消'),
            ),
            TextButton(
              onPressed: () => Navigator.pop(dialogContext, true),
              child: const Text('确认导入'),
            ),
          ],
        ),
      );
      if (confirmed != true) return;
      final authority = (previewResult?['roleAuthority'] ?? '').toString();
      roleAuthorityHeaders(authority);
      final result = await api.postMultipart<Map<String, dynamic>>(
        '/api/characters/import-card/confirm',
        fields: {'roleAuthority': authority},
        files: {
          'card': [path],
        },
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      final characterId = (result?['characterId'] ?? '').toString();
      if (result == null || characterId.isEmpty) {
        throw StateError('角色卡导入未返回有效的角色，请刷新后确认');
      }
      ref.invalidate(characterListProvider);
      if (characterId.isNotEmpty && mounted) {
        setState(() => _selectedId = characterId);
        final chars = await ref.read(characterListProvider.future);
        final imported = chars
            .where(
              (item) =>
                  item.id == characterId && item.roleAuthority == authority,
            )
            .firstOrNull;
        if (imported == null) {
          throw StateError('角色已导入，但当前角色数据归属已变化，请重新加载');
        }
        await _openEditor(character: imported);
      }
      if (mounted) amitiaSnackBar(context, '角色卡导入成功');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '角色卡导入失败：$error');
    } finally {
      if (mounted) setState(() => _importing = false);
    }
  }

  Future<void> _exportCard() async {
    if (_selectedId.isEmpty) return;
    setState(() => _exporting = true);
    try {
      final stream = await ref
          .read(backendServiceProvider)
          .getStream(
            '/api/characters/$_selectedId/export-card',
            queryParameters: const {'format': 'v3_charx', 'download': 'true'},
          );
      final chunks = await stream.toList();
      final bytes = chunks.expand((chunk) => chunk).toList();
      final target = await FilePicker.platform.saveFile(
        dialogTitle: '保存 CHARX 角色包',
        fileName: 'character.charx',
        type: FileType.custom,
        allowedExtensions: const ['charx'],
      );
      if (target == null || target.isEmpty) return;
      await File(target).writeAsBytes(bytes, flush: true);
      if (mounted) amitiaSnackBar(context, 'CHARX 角色包已导出');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '角色卡导出失败：$error');
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }
}
