import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

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

class ToolboxPromptTracePage extends ConsumerStatefulWidget {
  const ToolboxPromptTracePage({super.key});

  @override
  ConsumerState<ToolboxPromptTracePage> createState() => _ToolboxPromptTracePageState();
}

class _ToolboxPromptTracePageState extends ConsumerState<ToolboxPromptTracePage> {
  List<_TraceEntry> _traces = [];
  bool _loading = true;
  String? _error;
  String? _expandedId;

  @override
  void initState() {
    super.initState();
    _load();
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
      final resp = await api.get<List<dynamic>>('/api/system/prompt-traces');
      final items = resp ?? [];
      final traces = items.map((e) {
        final m = e as Map<String, dynamic>? ?? {};
        return _TraceEntry(
          id: (m['id'] ?? m['traceId'] ?? '').toString(),
          role: (m['role'] ?? 'assistant').toString(),
          model: (m['model'] ?? m['modelName'] ?? 'Unknown').toString(),
          blocks: (m['blocks'] ?? m['injectedBlocks'] ?? m['sections'] ?? '').toString(),
          promptTokens: (m['promptTokens'] ?? m['inputTokens'] ?? 0) as int,
          completionTokens: (m['completionTokens'] ?? m['outputTokens'] ?? 0) as int,
          snippet: (m['snippet'] ?? m['promptSnippet'] ?? m['content'] ?? '').toString(),
        );
      }).toList();
      if (mounted) {
        setState(() { _traces = traces; _loading = false; });
      }
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载 Prompt Trace...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);
    if (_traces.isEmpty) {
      return const AmitiaEmptyState(icon: Icons.psychology_outlined, title: '暂无 Prompt Trace', subtitle: '暂无已记录的 prompt 追踪数据');
    }

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
