import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/app_routes.dart';
import '../../../../core/widgets/amitia_scaffold.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../../core/services/providers.dart';

class MemoryGraphPage extends ConsumerStatefulWidget {
  const MemoryGraphPage({super.key});

  @override
  ConsumerState<MemoryGraphPage> createState() => _MemoryGraphPageState();
}

class _MemoryGraphPageState extends ConsumerState<MemoryGraphPage> {
  @override
  Widget build(BuildContext context) {
    final statusData = ref.watch(_vectorStatusProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '记忆图谱',
        showBackButton: true,
        fallbackRoute: AppRoutes.memory,
        actions: [
          AmitiaIconButton(
            icon: Icons.refresh,
            tooltip: '刷新状态',
            onPressed: () => ref.invalidate(_vectorStatusProvider),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.all(AppSpacing.pagePadding),
          child: statusData.when(
            loading: () => const Center(child: CircularProgressIndicator()),
            error: (err, _) => Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Icon(Icons.error_outline, size: 48, color: context.textSecondary),
                  const SizedBox(height: 16),
                  Text('加载失败: ${err.toString().replaceFirst('Exception: ', '')}',
                    style: AppTypography.body(context).copyWith(color: context.error),
                    textAlign: TextAlign.center),
                  const SizedBox(height: 16),
                  AmitiaButton(label: '重试', onPressed: () => ref.invalidate(_vectorStatusProvider)),
                ],
              ),
            ),
            data: (data) {
              if (data == null) {
                return Center(
                  child: AmitiaEmptyState(
                    icon: Icons.account_tree_outlined,
                    title: '暂无向量索引数据',
                    subtitle: '向量索引尚未建立',
                  ),
                );
              }
              return _buildStatusContent(context, data);
            },
          ),
        ),
      ),
    );
  }

  Widget _buildStatusContent(BuildContext context, Map<String, dynamic> data) {
    final entries = data.entries.toList();
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const AmitiaSectionHeader(title: '向量索引状态'),
        const SizedBox(height: AppSpacing.md),
        Expanded(
          child: ListView.separated(
            itemCount: entries.length,
            separatorBuilder: (_, _) => const SizedBox(height: AppSpacing.sm),
            itemBuilder: (context, index) {
              final entry = entries[index];
              final value = entry.value;
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
                      child: Icon(Icons.insights, size: 20, color: context.accentPrimary),
                    ),
                    const SizedBox(width: AppSpacing.md),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(entry.key, style: AppTypography.cardTitle(context)),
                          const SizedBox(height: 2),
                          Text(
                            value is Map || value is List
                                ? value.toString()
                                : value.toString(),
                            style: AppTypography.bodySmall(context),
                            maxLines: 3,
                            overflow: TextOverflow.ellipsis,
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              );
            },
          ),
        ),
      ],
    );
  }
}

final _vectorStatusProvider = FutureProvider.autoDispose<Map<String, dynamic>?>((ref) async {
  final svc = ref.read(memoryServiceProvider);
  return svc.vectorStatus();
});
