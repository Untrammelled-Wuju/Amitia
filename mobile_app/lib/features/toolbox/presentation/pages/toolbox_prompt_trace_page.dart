import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

class _TraceEntry {
  final String id;
  final String time;
  final String source;
  final String promptHash;
  final List<String> sections;
  final int rawReplyLength;
  final int finalReplyLength;
  final Map<String, dynamic> raw;

  const _TraceEntry({
    required this.id,
    required this.time,
    required this.source,
    required this.promptHash,
    required this.sections,
    required this.rawReplyLength,
    required this.finalReplyLength,
    required this.raw,
  });
}

class ToolboxPromptTracePage extends ConsumerStatefulWidget {
  const ToolboxPromptTracePage({super.key});

  @override
  ConsumerState<ToolboxPromptTracePage> createState() => _ToolboxPromptTracePageState();
}

class _ToolboxPromptTracePageState extends ConsumerState<ToolboxPromptTracePage> {
  List<_TraceEntry> _traces = const [];
  bool _loading = true;
  String? _error;
  String? _expandedId;

  @override
  void initState() {
    super.initState();
    _load();
  }

  int _int(dynamic value) => value is num ? value.toInt() : int.tryParse('$value') ?? 0;

  Future<void> _load() async {
    setState(() { _loading = true; _error = null; });
    try {
      final api = ref.read(backendServiceProvider);
      final resp = await api.get<Map<String, dynamic>>(
        '/api/logs/prompt-traces',
        fromJson: (e) => Map<String, dynamic>.from(e as Map),
      );
      final items = resp?['traces'] as List<dynamic>? ?? const [];
      final traces = items.whereType<Map>().map((entry) {
        final m = Map<String, dynamic>.from(entry);
        final sectionList = (m['section_names'] as List<dynamic>? ?? const [])
            .map((e) => e.toString())
            .toList();
        final requestId = (m['request_id'] ?? '').toString();
        final promptHash = (m['prompt_hash'] ?? '').toString();
        return _TraceEntry(
          id: requestId.isNotEmpty ? requestId : promptHash,
          time: (m['@timestamp'] ?? '').toString(),
          source: (m['source'] ?? m['channel'] ?? 'chat').toString(),
          promptHash: promptHash,
          sections: sectionList,
          rawReplyLength: _int(m['raw_reply_length']),
          finalReplyLength: _int(m['final_reply_length']),
          raw: m,
        );
      }).toList();
      if (mounted) setState(() { _traces = traces; _loading = false; });
    } catch (e) {
      if (mounted) setState(() { _error = e.toString(); _loading = false; });
    }
  }

  String _details(_TraceEntry t) {
    final entries = t.raw.entries
        .where((e) => e.value != null && e.value.toString().isNotEmpty)
        .map((e) => '${e.key}: ${e.value}')
        .join('\n');
    return entries;
  }

  @override
  Widget build(BuildContext context) {
    if (_loading) return const AmitiaLoadingState(message: '正在加载 Prompt Trace...');
    if (_error != null) return AmitiaErrorState(message: _error!, onRetry: _load);

    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: 'Prompt Trace',
        showBackButton: true,
        fallbackRoute: AppRoutes.settingsToolbox,
      ),
      body: RefreshIndicator(
        onRefresh: _load,
        child: _traces.isEmpty
            ? ListView(
                children: const [
                  AmitiaEmptyState(
                    icon: Icons.psychology_outlined,
                    title: '暂无 Prompt Trace',
                    subtitle: '产生一次真实模型回复后会记录 Prompt Trace',
                  ),
                ],
              )
            : ListView.separated(
                padding: EdgeInsets.all(AppSpacing.pagePadding),
                itemCount: _traces.length,
                separatorBuilder: (_, _) => SizedBox(height: AppSpacing.md),
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
                        InkWell(
                          onTap: () => setState(() => _expandedId = expanded ? null : t.id),
                          child: Padding(
                            padding: EdgeInsets.all(AppSpacing.cardPadding),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Row(
                                  children: [
                                    Expanded(
                                      child: Text(
                                        t.id.isEmpty ? t.promptHash : t.id,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: AppTypography.cardTitle(context),
                                      ),
                                    ),
                                    AmitiaStatusBadge(label: t.source, type: BadgeType.accent),
                                    Icon(
                                      expanded ? Icons.expand_less : Icons.expand_more,
                                      color: context.textTertiary,
                                    ),
                                  ],
                                ),
                                const SizedBox(height: 6),
                                Text(t.time, style: AppTypography.caption(context)),
                                const SizedBox(height: 4),
                                Text(
                                  '注入区块：${t.sections.isEmpty ? '无' : t.sections.join(' / ')}',
                                  style: AppTypography.caption(context),
                                ),
                                Text(
                                  '回复长度：${t.rawReplyLength} → ${t.finalReplyLength}',
                                  style: AppTypography.caption(context),
                                ),
                              ],
                            ),
                          ),
                        ),
                        if (expanded)
                          Container(
                            width: double.infinity,
                            padding: EdgeInsets.all(AppSpacing.cardPadding),
                            decoration: BoxDecoration(
                              color: context.surfaceSecondary,
                              borderRadius: const BorderRadius.vertical(bottom: Radius.circular(12)),
                            ),
                            child: SelectableText(
                              _details(t),
                              style: AppTypography.bodySmall(context).copyWith(fontFamily: 'monospace'),
                            ),
                          ),
                      ],
                    ),
                  );
                },
              ),
      ),
    );
  }
}
