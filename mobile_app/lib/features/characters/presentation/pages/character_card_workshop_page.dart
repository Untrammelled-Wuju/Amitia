import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/models/character.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class CharacterCardWorkshopPage extends ConsumerStatefulWidget {
  const CharacterCardWorkshopPage({super.key});

  @override
  ConsumerState<CharacterCardWorkshopPage> createState() => _CharacterCardWorkshopPageState();
}

class _CharacterCardWorkshopPageState extends ConsumerState<CharacterCardWorkshopPage> {
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
  bool _saving = false;
  bool _importing = false;
  bool _exporting = false;

  @override
  void dispose() {
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
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '角色卡工坊',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          AmitiaIconButton(icon: Icons.file_upload_outlined, tooltip: '导入角色卡', onPressed: _importing ? null : _importCard),
          AmitiaIconButton(icon: Icons.file_download_outlined, tooltip: '导出 CHARX', onPressed: _exporting || _selectedId.isEmpty ? null : _exportCard),
        ],
      ),
      body: SafeArea(
        top: false,
        child: charactersAsync.when(
          loading: () => const AmitiaLoadingState(message: '正在加载角色...'),
          error: (err, _) => AmitiaErrorState(message: '角色加载失败：$err', onRetry: () => ref.invalidate(characterListProvider)),
          data: (characters) => _buildBody(context, characters),
        ),
      ),
    );
  }

  Widget _buildBody(BuildContext context, List<CharacterDto> characters) {
    if (characters.isEmpty) {
      return AmitiaEmptyState(
        icon: Icons.badge_outlined,
        title: '还没有角色卡',
        subtitle: '先创建角色，再在这里编辑角色卡并导出 CHARX',
        actionText: '创建角色',
        onAction: () => context.push(AppRoutes.charactersCreate),
      );
    }
    final selected = characters.where((item) => item.id == _selectedId).firstOrNull;
    if (selected == null && _selectedId.isEmpty) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted && characters.isNotEmpty) _selectCharacter(characters.first);
      });
    }
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        DropdownButtonFormField<String>(
          key: ValueKey(_selectedId),
          initialValue: selected?.id,
          decoration: const InputDecoration(labelText: '选择角色'),
          items: characters
              .map((character) => DropdownMenuItem(value: character.id, child: Text(character.name)))
              .toList(),
          onChanged: _loading ? null : (id) {
            final character = characters.where((item) => item.id == id).firstOrNull;
            if (character != null) _selectCharacter(character);
          },
        ),
        SizedBox(height: AppSpacing.lg),
        if (_loading)
          const Center(child: Padding(padding: EdgeInsets.all(24), child: CircularProgressIndicator()))
        else if (selected != null)
          _buildEditor(context, selected),
      ],
    );
  }

  Widget _buildEditor(BuildContext context, CharacterDto character) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        _field('角色描述', _description, maxLines: 3),
        _field('场景设定', _scenario, maxLines: 3),
        _field('System Prompt', _systemPrompt, maxLines: 8),
        _field('示例对话', _exampleMessages, maxLines: 5),
        _field('备选问候', _alternateGreetings, maxLines: 5, hint: '每行一条备选问候'),
        _field('Post-history 指令', _postHistory, maxLines: 4),
        _field('创作者', _creator),
        _field('角色卡版本', _characterVersion),
        _field('标签', _tags, hint: '使用英文逗号分隔'),
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

  Widget _field(String label, TextEditingController controller, {int maxLines = 1, String? hint}) {
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
    setState(() {
      _selectedId = character.id;
      _loading = true;
    });
    _description.text = character.description;
    try {
      final data = await ref.read(backendServiceProvider).get<Map<String, dynamic>>(
        '/api/characters/${character.id}/card-data',
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      if (!mounted) return;
      _cardData = data ?? <String, dynamic>{};
      _description.text = (data?['description'] ?? character.description).toString();
      _scenario.text = (_cardData['scenario'] ?? '').toString();
      _systemPrompt.text = (_cardData['systemPrompt'] ?? character.characterBase).toString();
      _exampleMessages.text = (_cardData['exampleMessages'] ?? '').toString();
      _alternateGreetings.text = (_cardData['alternateGreetings'] as List?)?.map((item) => item.toString()).join('\n') ?? '';
      _postHistory.text = (_cardData['postHistoryInstructions'] ?? '').toString();
      _creator.text = (_cardData['creator'] ?? '').toString();
      _characterVersion.text = (_cardData['characterVersion'] ?? '').toString();
      _tags.text = (_cardData['tags'] as List?)?.map((item) => item.toString()).join(', ') ?? '';
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '角色卡加载失败：$error');
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _saveCard(CharacterDto character) async {
    setState(() => _saving = true);
    try {
      final api = ref.read(backendServiceProvider);
      await api.put<Map<String, dynamic>>(
        '/api/characters/${character.id}',
        data: {'description': _description.text.trim()},
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      await api.put<Map<String, dynamic>>(
        '/api/characters/${character.id}/card-data',
        data: {
          ..._cardData,
          'description': _description.text.trim(),
          'scenario': _scenario.text.trim(),
          'systemPrompt': _systemPrompt.text.trim(),
          'exampleMessages': _exampleMessages.text.trim(),
          'alternateGreetings': _alternateGreetings.text.split('\n').map((item) => item.trim()).where((item) => item.isNotEmpty).toList(),
          'postHistoryInstructions': _postHistory.text.trim(),
          'creator': _creator.text.trim(),
          'characterVersion': _characterVersion.text.trim(),
          'tags': _tags.text.split(',').map((item) => item.trim()).where((item) => item.isNotEmpty).toList(),
        },
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      ref.invalidate(characterListProvider);
      if (mounted) amitiaSnackBar(context, '角色卡已保存');
    } catch (error) {
      if (mounted) amitiaSnackBar(context, '角色卡保存失败：$error');
    } finally {
      if (mounted) setState(() => _saving = false);
    }
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
        files: {'card': [path]},
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      final preview = previewResult?['preview'] is Map ? Map<String, dynamic>.from(previewResult!['preview'] as Map) : <String, dynamic>{};
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
            TextButton(onPressed: () => Navigator.pop(dialogContext, false), child: const Text('取消')),
            TextButton(onPressed: () => Navigator.pop(dialogContext, true), child: const Text('确认导入')),
          ],
        ),
      );
      if (confirmed != true) return;
      final result = await api.postMultipart<Map<String, dynamic>>(
        '/api/characters/import-card/confirm',
        files: {'card': [path]},
        fromJson: (value) => Map<String, dynamic>.from(value as Map),
      );
      ref.invalidate(characterListProvider);
      final characterId = (result?['characterId'] ?? '').toString();
      if (characterId.isNotEmpty && mounted) {
        setState(() => _selectedId = characterId);
        final chars = await ref.read(characterListProvider.future);
        final imported = chars.where((item) => item.id == characterId).firstOrNull;
        if (imported != null) await _selectCharacter(imported);
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
      final stream = await ref.read(backendServiceProvider).getStream(
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
