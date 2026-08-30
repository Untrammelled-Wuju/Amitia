import 'dart:io';

import 'package:file_picker/file_picker.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/models/character.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class ChatImportPage extends ConsumerStatefulWidget {
  const ChatImportPage({super.key});

  @override
  ConsumerState<ChatImportPage> createState() => _ChatImportPageState();
}

class _ChatImportPageState extends ConsumerState<ChatImportPage> {
  final _contentController = TextEditingController();
  final _titleController = TextEditingController(text: '已导入的聊天');
  final List<Map<String, dynamic>> _parsedMessages = [];
  final List<Map<String, dynamic>> _memoryCandidates = [];
  int _currentStep = 0;
  String _selectedSource = '';
  String _selectedCharacterId = '';
  String _selectedCharacterName = '';
  String _detectedFormat = '';
  String _summary = '';
  String _conversationId = '';
  bool _busy = false;

  final _steps = const ['选择来源', '输入内容', '解析预览', '编辑消息', '选择角色', '确认导入', '生成摘要', '提取记忆', '完成'];
  final _sources = const [
    ('微信聊天记录', Icons.chat, '#52B788'),
    ('QQ 聊天记录', Icons.message, '#6C8FEA'),
    ('Telegram', Icons.send, '#E9A23B'),
    ('手动输入', Icons.edit_note, '#8A5728'),
  ];

  @override
  void dispose() {
    _contentController.dispose();
    _titleController.dispose();
    super.dispose();
  }

  void _show(String text, {bool error = false}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(text), backgroundColor: error ? context.error : null),
    );
  }

  Future<T?> _run<T>(Future<T> Function() action) async {
    if (_busy) return null;
    setState(() => _busy = true);
    try {
      return await action();
    } catch (e) {
      _show(e.toString(), error: true);
      return null;
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _pickFile() async {
    final picked = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: const ['txt', 'log', 'csv', 'md', 'json'],
    );
    if (picked == null || picked.files.isEmpty) return;
    final file = picked.files.first;
    if (file.path == null || file.path!.isEmpty) {
      _show('无法读取所选文件', error: true);
      return;
    }
    try {
      final text = await File(file.path!).readAsString();
      if (mounted) setState(() => _contentController.text = text);
    } catch (e) {
      _show('读取文件失败：$e', error: true);
    }
  }

  Future<bool> _parse() async {
    final raw = _contentController.text.trim();
    if (raw.isEmpty) {
      _show('请先输入或选择聊天记录', error: true);
      return false;
    }
    final result = await _run(() => ref.read(backendServiceProvider).post<Map<String, dynamic>>(
          '/api/imports/parse-text',
          data: {
            'rawText': raw,
            'format': 'auto',
            'defaultRole': 'user',
          },
        ));
    if (result == null) return false;
    final rows = result['items'];
    if (rows is! List || rows.isEmpty) {
      _show('没有解析出可导入的消息', error: true);
      return false;
    }
    setState(() {
      _detectedFormat = (result['detectedFormat'] ?? 'auto').toString();
      _parsedMessages
        ..clear()
        ..addAll(rows.whereType<Map>().map((e) => Map<String, dynamic>.from(e)));
    });
    return true;
  }

  Future<bool> _confirmImport() async {
    if (_selectedCharacterId.isEmpty) {
      _show('请选择关联角色', error: true);
      return false;
    }
    if (_parsedMessages.isEmpty) {
      _show('没有可导入的消息', error: true);
      return false;
    }
    final result = await _run(() => ref.read(backendServiceProvider).post<Map<String, dynamic>>(
          '/api/imports/confirm',
          data: {
            'characterId': _selectedCharacterId,
            'title': _titleController.text.trim().isEmpty ? '已导入的聊天' : _titleController.text.trim(),
            'items': _parsedMessages,
            'defaultRole': 'user',
          },
        ));
    if (result == null || result['confirmed'] != true) {
      _show((result?['message'] ?? '导入未完成').toString(), error: true);
      return false;
    }
    final conversationId = (result['conversationId'] ?? '').toString();
    if (conversationId.isEmpty) {
      _show('后端未返回导入会话 ID', error: true);
      return false;
    }
    setState(() => _conversationId = conversationId);
    ref.invalidate(conversationListProvider);
    return true;
  }

  Future<void> _generateSummary() async {
    if (_conversationId.isEmpty) return;
    final result = await _run(() => ref.read(backendServiceProvider).post<Map<String, dynamic>>(
          '/api/imports/batches/${Uri.encodeComponent(_conversationId)}/generate-summary',
        ));
    if (result == null) return;
    setState(() => _summary = (result['summary'] ?? '').toString());
  }

  Future<void> _extractMemories() async {
    if (_conversationId.isEmpty) return;
    final result = await _run(() => ref.read(backendServiceProvider).post<Map<String, dynamic>>(
          '/api/imports/batches/${Uri.encodeComponent(_conversationId)}/extract-memory-candidates',
        ));
    if (result == null) return;
    final rows = result['candidates'];
    setState(() {
      _memoryCandidates
        ..clear()
        ..addAll(rows is List ? rows.whereType<Map>().map((e) => Map<String, dynamic>.from(e)) : const []);
    });
  }

  Future<void> _next() async {
    if (_busy) return;
    if (_currentStep == 0 && _selectedSource.isEmpty) {
      _show('请选择导入来源', error: true);
      return;
    }
    if (_currentStep == 1 && !await _parse()) return;
    if (_currentStep == 4 && _selectedCharacterId.isEmpty) {
      _show('请选择关联角色', error: true);
      return;
    }
    if (_currentStep == 5 && !await _confirmImport()) return;
    if (_currentStep == 6) await _generateSummary();
    if (_currentStep == 7) await _extractMemories();
    if (mounted && _currentStep < _steps.length - 1) {
      setState(() => _currentStep++);
    }
  }

  Future<void> _editMessage(int index) async {
    final item = _parsedMessages[index];
    final controller = TextEditingController(text: (item['content'] ?? '').toString());
    String role = (item['role'] ?? 'user').toString();
    final saved = await showDialog<bool>(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('编辑消息'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              DropdownButtonFormField<String>(
                initialValue: role == 'assistant' ? 'assistant' : 'user',
                items: const [
                  DropdownMenuItem(value: 'user', child: Text('用户')),
                  DropdownMenuItem(value: 'assistant', child: Text('AI')),
                ],
                onChanged: (value) => setDialogState(() => role = value ?? 'user'),
              ),
              const SizedBox(height: 12),
              TextField(controller: controller, minLines: 2, maxLines: 8),
            ],
          ),
          actions: [
            TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('取消')),
            FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('保存')),
          ],
        ),
      ),
    );
    if (saved == true && controller.text.trim().isNotEmpty && mounted) {
      setState(() {
        item['role'] = role;
        item['content'] = controller.text.trim();
      });
    }
    controller.dispose();
  }

  Future<void> _showBatchList() async {
    final result = await _run(() => ref.read(backendServiceProvider).get<Map<String, dynamic>>('/api/imports/batches'));
    if (!mounted || result == null) return;
    final rows = result['items'];
    final batches = rows is List ? rows.whereType<Map>().map((e) => Map<String, dynamic>.from(e)).toList() : <Map<String, dynamic>>[];
    await showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('导入批次'),
        content: SizedBox(
          width: 520,
          child: batches.isEmpty
              ? const Padding(padding: EdgeInsets.all(24), child: Center(child: Text('暂无导入记录')))
              : ListView.separated(
                  shrinkWrap: true,
                  itemCount: batches.length,
                  separatorBuilder: (_, _) => const Divider(height: 1),
                  itemBuilder: (_, index) {
                    final batch = batches[index];
                    final id = (batch['id'] ?? '').toString();
                    final count = batch['message_count'] ?? batch['totalItems'] ?? 0;
                    final title = (batch['title'] ?? '已导入的聊天').toString();
                    final createdAt = (batch['created_at'] ?? batch['createdAt'] ?? '').toString();
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      title: Text(title),
                      subtitle: Text('$count 条消息 · $createdAt'),
                      trailing: IconButton(
                        icon: Icon(Icons.delete_outline, color: context.error),
                        onPressed: id.isEmpty
                            ? null
                            : () async {
                                await ref.read(backendServiceProvider).delete('/api/imports/batches/${Uri.encodeComponent(id)}');
                                if (ctx.mounted) Navigator.pop(ctx);
                                _show('导入批次已删除');
                                ref.invalidate(conversationListProvider);
                              },
                      ),
                    );
                  },
                ),
        ),
        actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭'))],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '导入聊天记录',
        navigation: AmitiaAppBarNavigation.back,
        actions: [AmitiaIconButton(icon: Icons.history, tooltip: '导入批次', onPressed: _busy ? null : _showBatchList)],
      ),
      body: Stack(
        children: [
          SafeArea(
            top: false,
            child: Column(
              children: [
                _stepIndicator(context),
                Expanded(child: _content(context)),
                _navigation(context),
              ],
            ),
          ),
          if (_busy)
            Positioned.fill(
              child: ColoredBox(color: Colors.black.withValues(alpha: 0.08), child: const Center(child: CircularProgressIndicator())),
            ),
        ],
      ),
    );
  }

  Widget _stepIndicator(BuildContext context) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.md),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: List.generate(_steps.length, (index) {
            final current = index == _currentStep;
            final completed = index < _currentStep;
            return Row(
              children: [
                Container(
                  width: 28,
                  height: 28,
                  decoration: BoxDecoration(
                    color: current ? context.accentPrimary : completed ? context.success : context.surfaceSecondary,
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: completed
                        ? const Icon(Icons.check, size: 16, color: Colors.white)
                        : Text('${index + 1}', style: TextStyle(fontSize: 12, color: current ? Colors.white : context.textTertiary)),
                  ),
                ),
                if (index < _steps.length - 1) Container(width: 20, height: 2, color: completed ? context.success : context.borderPrimary),
              ],
            );
          }),
        ),
      ),
    );
  }

  Widget _content(BuildContext context) {
    switch (_currentStep) {
      case 0:
        return _sourceStep(context);
      case 1:
        return _inputStep(context);
      case 2:
        return _messageList(context, title: '解析预览', editable: false);
      case 3:
        return _messageList(context, title: '编辑消息', editable: true);
      case 4:
        return _characterStep(context);
      case 5:
        return _confirmStep(context);
      case 6:
        return _summaryStep(context);
      case 7:
        return _memoryStep(context);
      default:
        return _completeStep(context);
    }
  }

  Widget _sourceStep(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text('选择导入来源', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.lg),
        for (final source in _sources)
          Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.sm),
            child: AmitiaCard(
              onTap: () => setState(() => _selectedSource = source.$1),
              border: Border.all(color: _selectedSource == source.$1 ? context.accentPrimary : context.borderPrimary),
              child: Row(
                children: [
                  Icon(source.$2, color: context.accentPrimary),
                  SizedBox(width: AppSpacing.md),
                  Expanded(child: Text(source.$1, style: AppTypography.cardTitle(context))),
                  Icon(_selectedSource == source.$1 ? Icons.check_circle : Icons.radio_button_unchecked, color: _selectedSource == source.$1 ? context.accentPrimary : context.textTertiary),
                ],
              ),
            ),
          ),
      ],
    );
  }

  Widget _inputStep(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text('输入聊天内容', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('来源：$_selectedSource', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        AmitiaTextField(
          controller: _contentController,
          maxLines: 14,
          hintText: '[2026-07-30 09:00] 用户: 你好\n[2026-07-30 09:01] AI: 你好！',
        ),
        SizedBox(height: AppSpacing.md),
        Row(
          children: [
            AmitiaButtonOutline(label: '从文件导入', onPressed: _pickFile),
            SizedBox(width: AppSpacing.sm),
            AmitiaButtonOutline(
              label: '使用示例',
              onPressed: () => setState(() {
                _contentController.text = '[2026-07-30 09:00] 用户: 你好，今天天气怎么样？\n[2026-07-30 09:01] AI: 今天天气不错。';
              }),
            ),
          ],
        ),
      ],
    );
  }

  Widget _messageList(BuildContext context, {required String title, required bool editable}) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Row(
          children: [
            Text(title, style: AppTypography.sectionTitle(context)),
            const Spacer(),
            Text('${_parsedMessages.length} 条 · $_detectedFormat', style: AppTypography.caption(context)),
          ],
        ),
        SizedBox(height: AppSpacing.md),
        for (var index = 0; index < _parsedMessages.length; index++)
          Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.sm),
            child: AmitiaCard(
              child: Row(
                children: [
                  CircleAvatar(
                    radius: 16,
                    backgroundColor: _parsedMessages[index]['role'] == 'assistant' ? context.info : context.accentPrimary,
                    child: Text(_parsedMessages[index]['role'] == 'assistant' ? 'AI' : '我', style: const TextStyle(color: Colors.white, fontSize: 11)),
                  ),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text((_parsedMessages[index]['content'] ?? '').toString(), style: AppTypography.bodySmall(context)),
                        if ((_parsedMessages[index]['timestamp'] ?? '').toString().isNotEmpty)
                          Text((_parsedMessages[index]['timestamp'] ?? '').toString(), style: AppTypography.label(context)),
                      ],
                    ),
                  ),
                  if (editable) ...[
                    AmitiaIconButton(icon: Icons.edit_outlined, size: 17, onPressed: () => _editMessage(index)),
                    AmitiaIconButton(icon: Icons.delete_outline, size: 17, color: context.error, onPressed: () => setState(() => _parsedMessages.removeAt(index))),
                  ],
                ],
              ),
            ),
          ),
      ],
    );
  }

  Widget _characterStep(BuildContext context) {
    final characters = ref.watch(characterListProvider);
    return characters.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(child: Text('角色加载失败：$error')),
      data: (items) => ListView(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        children: [
          Text('选择关联角色', style: AppTypography.sectionTitle(context)),
          SizedBox(height: AppSpacing.lg),
          if (items.isEmpty) const Text('暂无角色，请先创建角色'),
          for (final CharacterDto character in items)
            Padding(
              padding: EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                onTap: () => setState(() {
                  _selectedCharacterId = character.id;
                  _selectedCharacterName = character.name;
                }),
                border: Border.all(color: _selectedCharacterId == character.id ? context.accentPrimary : context.borderPrimary),
                child: Row(
                  children: [
                    CircleAvatar(child: Text(character.name.isEmpty ? '?' : character.name.substring(0, 1))),
                    SizedBox(width: AppSpacing.md),
                    Expanded(child: Text(character.name, style: AppTypography.cardTitle(context))),
                    Icon(_selectedCharacterId == character.id ? Icons.check_circle : Icons.radio_button_unchecked, color: _selectedCharacterId == character.id ? context.accentPrimary : context.textTertiary),
                  ],
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _confirmStep(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text('确认导入', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.lg),
        AmitiaTextField(controller: _titleController, hintText: '会话标题'),
        SizedBox(height: AppSpacing.md),
        AmitiaCard(
          child: Column(
            children: [
              _row(context, '来源', _selectedSource),
              _row(context, '关联角色', _selectedCharacterName),
              _row(context, '消息数量', '${_parsedMessages.length} 条'),
              _row(context, '识别格式', _detectedFormat),
            ],
          ),
        ),
      ],
    );
  }

  Widget _summaryStep(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text('会话摘要', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.lg),
        AmitiaCard(
          backgroundColor: context.accentSoft,
          child: Text(_summary.isEmpty ? '后端未生成摘要，可继续完成导入。' : _summary, style: AppTypography.bodySmall(context).copyWith(height: 1.6)),
        ),
      ],
    );
  }

  Widget _memoryStep(BuildContext context) {
    return ListView(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      children: [
        Text('记忆候选', style: AppTypography.sectionTitle(context)),
        SizedBox(height: AppSpacing.sm),
        Text('候选由当前后端模型从已导入会话中实时提取。', style: AppTypography.caption(context)),
        SizedBox(height: AppSpacing.lg),
        if (_memoryCandidates.isEmpty) const Text('没有提取到长期记忆候选'),
        for (final item in _memoryCandidates)
          Padding(
            padding: EdgeInsets.only(bottom: AppSpacing.sm),
            child: AmitiaCard(
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(Icons.memory, size: 20, color: context.accentPrimary),
                  SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text((item['key'] ?? '记忆').toString(), style: AppTypography.cardTitle(context)),
                        const SizedBox(height: 4),
                        Text((item['value'] ?? '').toString(), style: AppTypography.bodySmall(context)),
                        const SizedBox(height: 4),
                        Text('重要性：${item['importance'] ?? 0}/10', style: AppTypography.label(context)),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),
      ],
    );
  }

  Widget _completeStep(BuildContext context) {
    return Center(
      child: Padding(
        padding: EdgeInsets.all(AppSpacing.pagePadding),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.check_circle, size: 64, color: context.success),
            SizedBox(height: AppSpacing.lg),
            Text('导入完成', style: AppTypography.pageTitle(context)),
            SizedBox(height: AppSpacing.sm),
            Text('${_parsedMessages.length} 条消息已写入真实会话数据。', style: AppTypography.caption(context), textAlign: TextAlign.center),
          ],
        ),
      ),
    );
  }

  Widget _navigation(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(border: Border(top: BorderSide(color: context.borderPrimary))),
      child: Row(
        children: [
          if (_currentStep > 0 && _currentStep < _steps.length - 1)
            Expanded(child: AmitiaButtonOutline(label: '上一步', onPressed: _busy ? null : () => setState(() => _currentStep--))),
          if (_currentStep > 0 && _currentStep < _steps.length - 1) SizedBox(width: AppSpacing.md),
          Expanded(
            child: AmitiaButton(
              label: _currentStep == _steps.length - 1 ? '完成' : _currentStep == 5 ? '确认并导入' : '下一步',
              onPressed: _busy ? null : _currentStep == _steps.length - 1 ? () => Navigator.maybePop(context) : _next,
            ),
          ),
        ],
      ),
    );
  }

  Widget _row(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 7),
      child: Row(
        children: [
          SizedBox(width: 88, child: Text(label, style: AppTypography.label(context))),
          Expanded(child: Text(value, style: AppTypography.bodySmall(context))),
        ],
      ),
    );
  }
}
