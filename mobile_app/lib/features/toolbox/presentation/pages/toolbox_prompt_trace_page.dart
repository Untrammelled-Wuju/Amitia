import 'package:flutter/material.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';

class ToolboxPromptTracePage extends StatefulWidget {
  const ToolboxPromptTracePage({super.key});

  @override
  State<ToolboxPromptTracePage> createState() => _ToolboxPromptTracePageState();
}

class _TraceEntry {
  final String id;
  final String role;
  final String model;
  final String blocks;
  final int promptTokens;
  final int completionTokens;
  final String snippet;
  const _TraceEntry({
    required this.id,
    required this.role,
    required this.model,
    required this.blocks,
    required this.promptTokens,
    required this.completionTokens,
    required this.snippet,
  });
}

class _ToolboxPromptTracePageState extends State<ToolboxPromptTracePage> {
  final _traces = const <_TraceEntry>[
    _TraceEntry(
      id: '#1024',
      role: 'assistant',
      model: 'GPT-4',
      blocks: 'system / character / memory / tools',
      promptTokens: 1842,
      completionTokens: 326,
      snippet: '[system] 你是阿米娅，一个温柔细心的 AI 伙伴…\n[character] 语气温柔，偶尔俏皮…\n[memory] 用户喜欢早上喝咖啡…\n[user] 帮我整理下载目录',
    ),
    _TraceEntry(
      id: '#1023',
      role: 'assistant',
      model: 'Claude 3.5',
      blocks: 'system / character / tools',
      promptTokens: 968,
      completionTokens: 512,
      snippet: '[system] 你是高效理性的助手…\n[tools] 文件系统、Web 搜索…\n[user] 生成一份周报模板',
    ),
    _TraceEntry(
      id: '#1022',
      role: 'assistant',
      model: 'GPT-4o',
      blocks: 'system / character / memory / vision',
      promptTokens: 2410,
      completionTokens: 188,
      snippet: '[vision] 图像内容描述：产品需求文档截图\n[user] 提取关键信息',
    ),
    _TraceEntry(
      id: '#1021',
      role: 'assistant',
      model: 'DeepSeek-Voice',
      blocks: 'system / character',
      promptTokens: 412,
      completionTokens: 96,
      snippet: '[user 语音] 今天天气怎么样\n[转录] 今天天气怎么样',
    ),
  ];
  String? _expandedId;

  @override
  Widget build(BuildContext context) {
    return AmitiaScaffold(
      appBar: AmitiaAppBar(title: 'Prompt Trace', showBackButton: true, fallbackRoute: AppRoutes.settingsToolbox),
      body: ListView.separated(
        padding: const EdgeInsets.all(AppSpacing.pagePadding),
        itemCount: _traces.length,
        separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.md),
        itemBuilder: (context, i) {
          final t = _traces[i];
          final expanded = _expandedId == t.id;
          return Container(
            decoration: BoxDecoration(
              color: context.surfacePrimary,
              borderRadius: AppRadius.brMedium,
              border: Border.all(color: context.borderPrimary, width: 0.5),
            ),
            child: Column(
              children: [
                GestureDetector(
                  behavior: HitTestBehavior.opaque,
                  onTap: () => setState(() => _expandedId = expanded ? null : t.id),
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.cardPadding),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Row(
                          children: [
                            Text(t.id, style: AppTypography.cardTitle(context)),
                            const SizedBox(width: 8),
                            AmitiaStatusBadge(label: t.role, type: BadgeType.accent),
                            const Spacer(),
                            Icon(expanded ? Icons.expand_less : Icons.expand_more, color: context.textTertiary),
                          ],
                        ),
                        const SizedBox(height: 6),
                        Row(
                          children: [
                            Icon(Icons.psychology_outlined, size: 14, color: context.textSecondary),
                            const SizedBox(width: 4),
                            Text(t.model, style: AppTypography.label(context)),
                            const SizedBox(width: 12),
                            Icon(Icons.show_chart, size: 14, color: context.textSecondary),
                            const SizedBox(width: 4),
                            Text('${t.promptTokens}/${t.completionTokens}', style: AppTypography.label(context)),
                          ],
                        ),
                        const SizedBox(height: 4),
                        Text('注入区块：${t.blocks}', style: AppTypography.caption(context)),
                      ],
                    ),
                  ),
                ),
                if (expanded)
                  Container(
                    width: double.infinity,
                    padding: const EdgeInsets.all(AppSpacing.cardPadding),
                    decoration: BoxDecoration(
                      color: context.surfaceSecondary,
                      borderRadius: const BorderRadius.vertical(bottom: Radius.circular(12)),
                    ),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('Prompt 片段', style: AppTypography.label(context).copyWith(fontWeight: FontWeight.w600)),
                        const SizedBox(height: 6),
                        Text(t.snippet, style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace')),
                      ],
                    ),
                  ),
              ],
            ),
          );
        },
      ),
    );
  }
}
