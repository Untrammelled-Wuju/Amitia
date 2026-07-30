import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_button.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../shared/models/models.dart';
import '../../../../shared/mock_data/mock_data.dart';

class ChatImportPage extends ConsumerStatefulWidget {
  const ChatImportPage({super.key});

  @override
  ConsumerState<ChatImportPage> createState() => _ChatImportPageState();
}

class _ChatImportPageState extends ConsumerState<ChatImportPage> {
  int _currentStep = 0;
  late List<ImportBatch> _batches;
  String _selectedSource = '';
  final _contentController = TextEditingController();
  String _selectedCharacter = '';
  bool _isProcessing = false;

  final _sources = [
    {'name': '微信聊天记录', 'icon': Icons.chat, 'color': '#52B788'},
    {'name': 'QQ 聊天记录', 'icon': Icons.message, 'color': '#6C8FEA'},
    {'name': 'Telegram', 'icon': Icons.send, 'color': '#E9A23B'},
    {'name': '手动输入', 'icon': Icons.edit_note, 'color': '#7668EE'},
  ];

  final _characters = ['阿米娅', '小雨', 'Epsilon', 'Karin'];
  final _steps = ['选择来源', '输入内容', '解析预览', '编辑消息', '选择角色', '生成摘要', '提取记忆', '确认导入', '完成'];

  final _parsedMessages = [
    {'role': 'user', 'content': '你好，今天天气怎么样？', 'time': '2026-07-30 09:00'},
    {'role': 'assistant', 'content': '今天天气不错，阳光明媚。', 'time': '2026-07-30 09:01'},
    {'role': 'user', 'content': '帮我整理一下文件', 'time': '2026-07-30 09:15'},
    {'role': 'assistant', 'content': '好的，我来帮你扫描目录。', 'time': '2026-07-30 09:16'},
  ];

  @override
  void initState() {
    super.initState();
    _batches = List.from(MockMemory.importBatches);
  }

  @override
  void dispose() {
    _contentController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '导入聊天记录',
        showBackButton: true,
        actions: [
          AmitiaIconButton(
            icon: Icons.history,
            tooltip: '导入批次',
            onPressed: () => _showBatchList(context),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Column(
          children: [
            _buildStepIndicator(context),
            Expanded(child: _buildStepContent(context)),
            _buildNavigationButtons(context),
          ],
        ),
      ),
    );
  }

  Widget _buildStepIndicator(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding, vertical: AppSpacing.md),
      child: SingleChildScrollView(
        scrollDirection: Axis.horizontal,
        child: Row(
          children: List.generate(_steps.length, (index) {
            final isCurrent = index == _currentStep;
            final isCompleted = index < _currentStep;
            return Row(
              children: [
                Container(
                  width: 28,
                  height: 28,
                  decoration: BoxDecoration(
                    color: isCurrent ? context.accentPrimary : (isCompleted ? context.success : context.surfaceSecondary),
                    shape: BoxShape.circle,
                  ),
                  child: Center(
                    child: isCompleted
                        ? Icon(Icons.check, size: 16, color: Colors.white)
                        : Text('${index + 1}', style: TextStyle(fontSize: 12, color: isCurrent ? Colors.white : context.textTertiary, fontWeight: FontWeight.w600)),
                  ),
                ),
                if (index < _steps.length - 1)
                  Container(
                    width: 20,
                    height: 2,
                    color: isCompleted ? context.success : context.borderPrimary,
                  ),
              ],
            );
          }),
        ),
      ),
    );
  }

  Widget _buildStepContent(BuildContext context) {
    switch (_currentStep) {
      case 0: return _buildSourceStep(context);
      case 1: return _buildInputStep(context);
      case 2: return _buildParseStep(context);
      case 3: return _buildEditStep(context);
      case 4: return _buildCharacterStep(context);
      case 5: return _buildSummaryStep(context);
      case 6: return _buildMemoryStep(context);
      case 7: return _buildConfirmStep(context);
      case 8: return _buildCompleteStep(context);
      default: return const SizedBox.shrink();
    }
  }

  Widget _buildSourceStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('选择导入来源', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
          Text('选择聊天记录的来源渠道', style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.lg),
          ..._sources.map((s) {
            final isSelected = _selectedSource == s['name'];
            final color = Color(int.parse('FF${(s['color'] as String).replaceAll('#', '')}', radix: 16));
            return Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                border: Border.all(color: isSelected ? color : context.borderPrimary, width: isSelected ? 1.5 : 0.5),
                backgroundColor: isSelected ? color.withValues(alpha: 0.05) : null,
                onTap: () => setState(() => _selectedSource = s['name'] as String),
                child: Row(
                  children: [
                    Container(
                      width: 44,
                      height: 44,
                      decoration: BoxDecoration(color: color.withValues(alpha: 0.12), borderRadius: AppRadius.brSmall),
                      child: Icon(s['icon'] as IconData, size: 22, color: color),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(child: Text(s['name'] as String, style: AppTypography.cardTitle(context))),
                    Icon(isSelected ? Icons.check_circle : Icons.radio_button_unchecked, size: 22, color: isSelected ? color : context.textTertiary),
                  ],
                ),
              ),
            );
          }),
        ],
      ),
    );
  }

  Widget _buildInputStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('输入聊天内容', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
          Text('来源：$_selectedSource', style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.lg),
          AmitiaTextField(
            controller: _contentController,
            maxLines: 12,
            hintText: '粘贴或输入聊天记录内容...\n\n格式示例：\n[2026-07-30 09:00] 用户: 你好\n[2026-07-30 09:01] AI: 你好！',
          ),
          const SizedBox(height: AppSpacing.md),
          Row(
            children: [
              AmitiaButtonOutline(
                label: '从文件导入',
                onPressed: () {
                  ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请选择文件（Mock）'), duration: Duration(seconds: 1)));
                },
              ),
              const SizedBox(width: AppSpacing.sm),
              AmitiaButtonOutline(
                label: '使用示例',
                onPressed: () {
                  _contentController.text = '[2026-07-30 09:00] 用户: 你好，今天天气怎么样？\n[2026-07-30 09:01] AI: 今天天气不错，阳光明媚。\n[2026-07-30 09:15] 用户: 帮我整理一下文件\n[2026-07-30 09:16] AI: 好的，我来帮你扫描目录。';
                },
              ),
            ],
          ),
        ],
      ),
    );
  }

  Widget _buildParseStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('解析预览', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(AppSpacing.md),
            decoration: BoxDecoration(color: context.success.withValues(alpha: 0.08), borderRadius: AppRadius.brMedium),
            child: Row(
              children: [
                Icon(Icons.check_circle, size: 20, color: context.success),
                const SizedBox(width: AppSpacing.sm),
                Expanded(child: Text('成功解析 ${_parsedMessages.length} 条消息', style: AppTypography.bodySmall(context).copyWith(color: context.success))),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.lg),
          ..._parsedMessages.map((m) => Padding(
            padding: const EdgeInsets.only(bottom: AppSpacing.sm),
            child: AmitiaCard(
              child: Row(
                children: [
                  Container(
                    width: 32,
                    height: 32,
                    decoration: BoxDecoration(
                      color: m['role'] == 'user' ? context.accentPrimary : context.info,
                      shape: BoxShape.circle,
                    ),
                    child: Center(child: Text(m['role'] == 'user' ? '我' : 'AI', style: const TextStyle(color: Colors.white, fontSize: 11))),
                  ),
                  const SizedBox(width: AppSpacing.sm),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(m['content'] as String, style: AppTypography.bodySmall(context), maxLines: 1, overflow: TextOverflow.ellipsis),
                        Text(m['time'] as String, style: AppTypography.label(context)),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          )),
        ],
      ),
    );
  }

  Widget _buildEditStep(BuildContext context) {
    return Column(
      children: [
        Padding(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          child: Row(
            children: [
              Text('编辑消息', style: AppTypography.sectionTitle(context)),
              const Spacer(),
              AmitiaIconButton(icon: Icons.add, color: context.accentPrimary, onPressed: () {
                ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('添加新消息（Mock）'), duration: Duration(seconds: 1)));
              }),
            ],
          ),
        ),
        Expanded(
          child: ListView.separated(
            padding: const EdgeInsets.symmetric(horizontal: AppSpacing.pagePadding),
            itemCount: _parsedMessages.length,
            separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
            itemBuilder: (context, index) {
              final m = _parsedMessages[index];
              return AmitiaCard(
                child: Row(
                  children: [
                    Container(
                      width: 32,
                      height: 32,
                      decoration: BoxDecoration(
                        color: m['role'] == 'user' ? context.accentPrimary : context.info,
                        shape: BoxShape.circle,
                      ),
                      child: Center(child: Text(m['role'] == 'user' ? '我' : 'AI', style: const TextStyle(color: Colors.white, fontSize: 11))),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(m['content'] as String, style: AppTypography.bodySmall(context)),
                          Text(m['time'] as String, style: AppTypography.label(context)),
                        ],
                      ),
                    ),
                    AmitiaIconButton(icon: Icons.edit_outlined, size: 16, onPressed: () {
                      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('编辑消息 ${index + 1}'), duration: const Duration(seconds: 1)));
                    }),
                    AmitiaIconButton(icon: Icons.delete_outline, size: 16, color: context.error, onPressed: () {
                      setState(() => _parsedMessages.removeAt(index));
                    }),
                  ],
                ),
              );
            },
          ),
        ),
      ],
    );
  }

  Widget _buildCharacterStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('选择关联角色', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
          Text('选择将这批聊天记录关联到哪个角色', style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.lg),
          ..._characters.map((c) {
            final isSelected = _selectedCharacter == c;
            return Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                border: Border.all(color: isSelected ? context.accentPrimary : context.borderPrimary, width: isSelected ? 1.5 : 0.5),
                onTap: () => setState(() => _selectedCharacter = c),
                child: Row(
                  children: [
                    AmitiaAvatar(initial: c[0], colorHex: '#7668EE', size: 40),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(child: Text(c, style: AppTypography.cardTitle(context))),
                    Icon(isSelected ? Icons.check_circle : Icons.radio_button_unchecked, size: 22, color: isSelected ? context.accentPrimary : context.textTertiary),
                  ],
                ),
              ),
            );
          }),
        ],
      ),
    );
  }

  Widget _buildSummaryStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('生成会话摘要', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
          Text('AI 正在为这批聊天记录生成摘要（Mock）', style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.lg),
          AmitiaCard(
            backgroundColor: context.accentSoft,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Icon(Icons.auto_awesome, size: 18, color: context.accentPrimary),
                    const SizedBox(width: AppSpacing.sm),
                    Text('会话摘要', style: AppTypography.cardTitle(context)),
                  ],
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  '本次会话主要包含日常问候和文件整理请求。用户询问了天气情况，随后请求整理文件。AI 响应迅速，提供了天气信息和文件扫描服务。',
                  style: AppTypography.bodySmall(context).copyWith(height: 1.6),
                ),
                const SizedBox(height: AppSpacing.sm),
                Wrap(
                  spacing: AppSpacing.sm,
                  children: [
                    AmitiaStatusBadge(label: '日常对话', type: BadgeType.accent),
                    AmitiaStatusBadge(label: '文件处理', type: BadgeType.info),
                    AmitiaStatusBadge(label: '4条消息', type: BadgeType.neutral),
                  ],
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMemoryStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('提取记忆候选', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.sm),
          Text('AI 从聊天记录中提取了以下记忆候选（Mock）', style: AppTypography.caption(context)),
          const SizedBox(height: AppSpacing.lg),
          ...[
            {'content': '用户习惯在早上9点左右开始活动', 'type': '习惯', 'confidence': 0.85},
            {'content': '用户有整理文件的需求', 'type': '偏好', 'confidence': 0.75},
            {'content': '用户关心天气情况', 'type': '事实', 'confidence': 0.9},
          ].map((m) {
            return Padding(
              padding: const EdgeInsets.only(bottom: AppSpacing.sm),
              child: AmitiaCard(
                child: Row(
                  children: [
                    Icon(Icons.memory, size: 20, color: context.accentPrimary),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(m['content'] as String, style: AppTypography.bodySmall(context)),
                          const SizedBox(height: 2),
                          Row(
                            children: [
                              AmitiaStatusBadge(label: m['type'] as String, type: BadgeType.accent),
                              const SizedBox(width: AppSpacing.sm),
                              Text('置信度：${((m['confidence'] as double) * 100).round()}%', style: AppTypography.label(context)),
                            ],
                          ),
                        ],
                      ),
                    ),
                    Icon(Icons.check_circle, size: 20, color: context.success),
                  ],
                ),
              ),
            );
          }),
        ],
      ),
    );
  }

  Widget _buildConfirmStep(BuildContext context) {
    return SingleChildScrollView(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('确认导入', style: AppTypography.sectionTitle(context)),
          const SizedBox(height: AppSpacing.lg),
          AmitiaCard(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                _buildSummaryRow(context, '导入来源', _selectedSource),
                const Divider(height: AppSpacing.lg),
                _buildSummaryRow(context, '消息数量', '${_parsedMessages.length} 条'),
                const Divider(height: AppSpacing.lg),
                _buildSummaryRow(context, '关联角色', _selectedCharacter.isEmpty ? '未选择' : _selectedCharacter),
                const Divider(height: AppSpacing.lg),
                _buildSummaryRow(context, '已生成摘要', '是'),
                const Divider(height: AppSpacing.lg),
                _buildSummaryRow(context, '记忆候选', '3 条'),
              ],
            ),
          ),
          const SizedBox(height: AppSpacing.lg),
          AmitiaButton(
            label: '确认导入',
            icon: Icons.download,
            isFullWidth: true,
            onPressed: () => _showImportConfirmDialog(context),
          ),
        ],
      ),
    );
  }

  Widget _buildCompleteStep(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppSpacing.xxl),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Container(
              width: 80,
              height: 80,
              decoration: BoxDecoration(color: context.success.withValues(alpha: 0.12), shape: BoxShape.circle),
              child: Icon(Icons.check_circle, size: 48, color: context.success),
            ),
            const SizedBox(height: AppSpacing.lg),
            Text('导入完成', style: AppTypography.pageTitle(context)),
            const SizedBox(height: AppSpacing.sm),
            Text('已成功导入 ${_parsedMessages.length} 条消息', style: AppTypography.caption(context)),
            const SizedBox(height: AppSpacing.xl),
            AmitiaCard(
              child: Column(
                children: [
                  _buildSummaryRow(context, '来源', _selectedSource),
                  const Divider(height: AppSpacing.lg),
                  _buildSummaryRow(context, '消息数', '${_parsedMessages.length} 条'),
                  const Divider(height: AppSpacing.lg),
                  _buildSummaryRow(context, '关联角色', _selectedCharacter),
                  const Divider(height: AppSpacing.lg),
                  _buildSummaryRow(context, '记忆候选', '3 条已提取'),
                  const Divider(height: AppSpacing.lg),
                  _buildSummaryRow(context, '摘要', '已生成'),
                ],
              ),
            ),
            const SizedBox(height: AppSpacing.xl),
            AmitiaButton(
              label: '查看导入批次',
              isFullWidth: true,
              isSecondary: true,
              onPressed: () => _showBatchList(context),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildSummaryRow(BuildContext context, String label, String value) {
    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(label, style: AppTypography.caption(context)),
        Text(value, style: AppTypography.bodySmall(context).copyWith(fontWeight: FontWeight.w600)),
      ],
    );
  }

  Widget _buildNavigationButtons(BuildContext context) {
    final isLastStep = _currentStep == _steps.length - 1;
    final isFirstStep = _currentStep == 0;
    return Container(
      padding: const EdgeInsets.all(AppSpacing.pagePadding),
      decoration: BoxDecoration(color: context.surfacePrimary, border: Border(top: BorderSide(color: context.borderPrimary, width: 0.5))),
      child: Row(
        children: [
          if (!isFirstStep && !isLastStep)
            Expanded(
              child: AmitiaButton(
                label: '上一步',
                isSecondary: true,
                onPressed: () => setState(() => _currentStep--),
              ),
            ),
          if (!isFirstStep && !isLastStep) const SizedBox(width: AppSpacing.sm),
          if (!isLastStep)
            Expanded(
              child: AmitiaButton(
                label: _currentStep == 7 ? '确认导入' : '下一步',
                onPressed: () {
                  if (_currentStep == 0 && _selectedSource.isEmpty) {
                    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请选择导入来源'), duration: Duration(seconds: 1)));
                    return;
                  }
                  if (_currentStep == 4 && _selectedCharacter.isEmpty) {
                    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('请选择关联角色'), duration: Duration(seconds: 1)));
                    return;
                  }
                  setState(() => _currentStep++);
                },
              ),
            ),
          if (isLastStep)
            Expanded(
              child: AmitiaButton(
                label: '完成',
                onPressed: () => Navigator.pop(context),
              ),
            ),
        ],
      ),
    );
  }

  void _showImportConfirmDialog(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('确认导入', style: AppTypography.cardTitle(context)),
        content: Text('确定要导入这 ${_parsedMessages.length} 条消息吗？导入后将关联到角色「$_selectedCharacter」。', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              setState(() {
                _batches.add(ImportBatch(
                  id: 'ib${DateTime.now().millisecondsSinceEpoch}',
                  source: _selectedSource,
                  messageCount: _parsedMessages.length,
                  importTime: DateTime.now(),
                ));
                _currentStep++;
              });
            },
            child: Text('确认导入', style: TextStyle(color: context.accentPrimary)),
          ),
        ],
      ),
    );
  }

  void _showBatchList(BuildContext context) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('导入批次', style: AppTypography.cardTitle(context)),
        content: SizedBox(
          width: double.maxFinite,
          child: _batches.isEmpty
              ? Text('暂无导入批次', style: AppTypography.caption(context))
              : ListView.separated(
                  shrinkWrap: true,
                  itemCount: _batches.length,
                  separatorBuilder: (_, _) => const Divider(height: 1),
                  itemBuilder: (context, index) {
                    final batch = _batches[index];
                    return ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(Icons.history, size: 20, color: context.accentPrimary),
                      title: Text(batch.source, style: AppTypography.bodySmall(context)),
                      subtitle: Text('${batch.messageCount}条 · ${_formatTime(batch.importTime)}', style: AppTypography.label(context)),
                      trailing: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          AmitiaStatusBadge(label: batch.status, type: BadgeType.success),
                          const SizedBox(width: AppSpacing.sm),
                          GestureDetector(
                            onTap: () => _showDeleteBatchConfirm(context, batch),
                            child: Icon(Icons.delete_outline, size: 18, color: context.error),
                          ),
                        ],
                      ),
                      onTap: () => _showBatchDetail(context, batch),
                    );
                  },
                ),
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  void _showBatchDetail(BuildContext context, ImportBatch batch) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('批次详情', style: AppTypography.cardTitle(context)),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildSummaryRow(context, '来源', batch.source),
            const SizedBox(height: AppSpacing.sm),
            _buildSummaryRow(context, '消息数', '${batch.messageCount} 条'),
            const SizedBox(height: AppSpacing.sm),
            _buildSummaryRow(context, '导入时间', _formatTime(batch.importTime)),
            const SizedBox(height: AppSpacing.sm),
            _buildSummaryRow(context, '状态', batch.status),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('关闭')),
        ],
      ),
    );
  }

  void _showDeleteBatchConfirm(BuildContext context, ImportBatch batch) {
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: Text('删除批次', style: AppTypography.cardTitle(context)),
        content: Text('确定要删除来自「${batch.source}」的导入批次吗？', style: AppTypography.bodySmall(context)),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('取消')),
          TextButton(
            onPressed: () {
              Navigator.pop(ctx);
              Navigator.pop(context);
              setState(() => _batches.removeWhere((b) => b.id == batch.id));
              ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('批次已删除'), duration: Duration(seconds: 1)));
            },
            child: Text('删除', style: TextStyle(color: context.error)),
          ),
        ],
      ),
    );
  }

  String _formatTime(DateTime time) {
    return '${time.month}月${time.day}日 ${time.hour.toString().padLeft(2, '0')}:${time.minute.toString().padLeft(2, '0')}';
  }
}
