import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../../app/app_routes.dart';
import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_radius.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_scaffold.dart';

final _workflowListProvider = FutureProvider<List<Map<String, dynamic>>>((ref) async {
  return ref.read(extensionServiceProvider).workflows(limit: 200);
});

class WorkflowListPage extends ConsumerWidget {
  const WorkflowListPage({super.key});

  Future<void> _create(BuildContext context, WidgetRef ref) async {
    final service = ref.read(extensionServiceProvider);
    final created = await service.createWorkflow(<String, dynamic>{
      'schemaVersion': 'workflow-v2',
      'name': '未命名工作流',
      'description': '',
      'inputSchema': <String, dynamic>{'type': 'object', 'properties': <String, dynamic>{}},
      'outputSchema': <String, dynamic>{'type': 'object', 'properties': <String, dynamic>{}},
      'nodes': <dynamic>[],
      'edges': <dynamic>[],
      'triggers': <dynamic>[
        <String, dynamic>{
          'id': 'manual',
          'type': 'manual',
          'config': <String, dynamic>{},
          'enabled': true,
        },
      ],
      'callableByAgent': false,
      'enabled': true,
      'source': 'user',
      'metadata': <String, dynamic>{},
    });
    final id = (created['id'] ?? '').toString();
    ref.invalidate(_workflowListProvider);
    if (context.mounted && id.isNotEmpty) {
      context.push(AppRoutes.workflowEditor(id));
    }
  }

  Future<void> _duplicate(BuildContext context, WidgetRef ref, Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final created = await ref.read(extensionServiceProvider).duplicateWorkflow(id);
    final createdId = (created['id'] ?? '').toString();
    ref.invalidate(_workflowListProvider);
    if (context.mounted && createdId.isNotEmpty) {
      context.push(AppRoutes.workflowEditor(createdId));
    }
  }

  Future<void> _delete(BuildContext context, WidgetRef ref, Map<String, dynamic> item) async {
    final id = (item['id'] ?? '').toString();
    if (id.isEmpty) return;
    final name = (item['name'] ?? id).toString();
    final confirmed = await showDialog<bool>(
          context: context,
          builder: (context) => AlertDialog(
            title: const Text('删除工作流'),
            content: Text('确定删除「$name」？运行历史会一并移除。'),
            actions: [
              TextButton(onPressed: () => Navigator.pop(context, false), child: const Text('取消')),
              FilledButton(onPressed: () => Navigator.pop(context, true), child: const Text('删除')),
            ],
          ),
        ) ??
        false;
    if (!confirmed) return;
    await ref.read(extensionServiceProvider).deleteWorkflow(id);
    ref.invalidate(_workflowListProvider);
    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('工作流已删除')));
    }
  }

  Future<void> _run(BuildContext context, WidgetRef ref, String id) async {
    if (id.isEmpty) return;
    final result = await ref.read(extensionServiceProvider).runWorkflow(id);
    if (context.mounted) {
      final runId = (result['executionId'] ?? '').toString();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(runId.isEmpty ? '已提交运行' : '已提交运行 · $runId')),
      );
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncItems = ref.watch(_workflowListProvider);
    return AmitiaScaffold(
      appBar: AmitiaAppBar(
        title: '工作流',
        showBackButton: true,
        fallbackRoute: AppRoutes.workshop,
        actions: [
          IconButton(
            tooltip: '刷新',
            onPressed: () => ref.invalidate(_workflowListProvider),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: asyncItems.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _ErrorState(
            message: error.toString(),
            onRetry: () => ref.invalidate(_workflowListProvider),
          ),
          data: (items) => RefreshIndicator(
            onRefresh: () async => ref.invalidate(_workflowListProvider),
            child: ListView(
              padding: EdgeInsets.all(AppSpacing.pagePadding),
              children: [
                _IntroCard(onCreate: () => _create(context, ref)),
                SizedBox(height: AppSpacing.sectionGap),
                Row(
                  children: [
                    Expanded(child: Text('我的工作流', style: AppTypography.sectionTitle(context))),
                    Text('${items.length}', style: AppTypography.caption(context)),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                if (items.isEmpty)
                  _EmptyState(onCreate: () => _create(context, ref))
                else
                  ...items.map(
                    (item) => Padding(
                      padding: EdgeInsets.only(bottom: AppSpacing.md),
                      child: _WorkflowCard(
                        item: item,
                        onOpen: () => context.push(AppRoutes.workflowEditor((item['id'] ?? '').toString())),
                        onRun: () => _run(context, ref, (item['id'] ?? '').toString()),
                        onToggle: (value) async {
                          await ref.read(extensionServiceProvider).setWorkflowEnabled((item['id'] ?? '').toString(), value);
                          ref.invalidate(_workflowListProvider);
                        },
                        onDuplicate: () => _duplicate(context, ref, item),
                        onDelete: () => _delete(context, ref, item),
                      ),
                    ),
                  ),
              ],
            ),
          ),
        ),
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => _create(context, ref),
        icon: const Icon(Icons.add),
        label: const Text('新建工作流'),
      ),
    );
  }
}

class _IntroCard extends StatelessWidget {
  final VoidCallback onCreate;
  const _IntroCard({required this.onCreate});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.all(AppSpacing.xl),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        borderRadius: AppRadius.brLarge,
        border: Border.all(color: context.borderPrimary, width: 0.5),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(Icons.account_tree_outlined, color: context.accentPrimary),
              const SizedBox(width: 10),
              Expanded(child: Text('Extension Kernel Workflow', style: AppTypography.cardTitle(context))),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            '拖拽节点创建 DAG，配置手动、事件与定时触发器；运行时可查看每个节点的输入、输出、错误与状态。',
            style: AppTypography.caption(context),
          ),
          const SizedBox(height: 14),
          FilledButton.icon(onPressed: onCreate, icon: const Icon(Icons.add), label: const Text('创建工作流')),
        ],
      ),
    );
  }
}

class _WorkflowCard extends StatelessWidget {
  final Map<String, dynamic> item;
  final VoidCallback onOpen;
  final VoidCallback onRun;
  final ValueChanged<bool> onToggle;
  final VoidCallback onDuplicate;
  final VoidCallback onDelete;

  const _WorkflowCard({
    required this.item,
    required this.onOpen,
    required this.onRun,
    required this.onToggle,
    required this.onDuplicate,
    required this.onDelete,
  });

  int _count(String key) => (item[key] as List?)?.length ?? 0;

  @override
  Widget build(BuildContext context) {
    final enabled = item['enabled'] == true;
    return Material(
      color: context.surfacePrimary,
      borderRadius: AppRadius.brLarge,
      child: InkWell(
        borderRadius: AppRadius.brLarge,
        onTap: onOpen,
        child: Container(
          padding: EdgeInsets.all(AppSpacing.cardPadding),
          decoration: BoxDecoration(
            borderRadius: AppRadius.brLarge,
            border: Border.all(color: context.borderPrimary, width: 0.5),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Expanded(
                    child: Text(
                      (item['name'] ?? '未命名工作流').toString(),
                      style: AppTypography.cardTitle(context),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  Switch(value: enabled, onChanged: onToggle),
                ],
              ),
              if ((item['description'] ?? '').toString().isNotEmpty) ...[
                const SizedBox(height: 4),
                Text((item['description'] ?? '').toString(), style: AppTypography.caption(context), maxLines: 2, overflow: TextOverflow.ellipsis),
              ],
              const SizedBox(height: 10),
              Wrap(
                spacing: 8,
                runSpacing: 6,
                children: [
                  _Pill(label: '${_count('nodes')} 节点'),
                  _Pill(label: '${_count('edges')} 连线'),
                  _Pill(label: '${_count('triggers')} Trigger'),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  TextButton.icon(onPressed: onRun, icon: const Icon(Icons.play_arrow, size: 18), label: const Text('运行')),
                  TextButton.icon(onPressed: onOpen, icon: const Icon(Icons.edit_outlined, size: 18), label: const Text('编辑')),
                  const Spacer(),
                  IconButton(tooltip: '复制', onPressed: onDuplicate, icon: const Icon(Icons.copy_outlined)),
                  IconButton(tooltip: '删除', onPressed: onDelete, icon: const Icon(Icons.delete_outline)),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _Pill extends StatelessWidget {
  final String label;
  const _Pill({required this.label});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(color: context.surfaceSecondary, borderRadius: BorderRadius.circular(999)),
      child: Text(label, style: AppTypography.caption(context)),
    );
  }
}

class _EmptyState extends StatelessWidget {
  final VoidCallback onCreate;
  const _EmptyState({required this.onCreate});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 48, horizontal: 24),
      alignment: Alignment.center,
      child: Column(
        children: [
          Icon(Icons.account_tree_outlined, size: 54, color: context.textTertiary),
          const SizedBox(height: 12),
          Text('还没有工作流', style: AppTypography.cardTitle(context)),
          const SizedBox(height: 6),
          Text('创建第一条可视化 DAG 工作流。', style: AppTypography.caption(context)),
          const SizedBox(height: 16),
          OutlinedButton(onPressed: onCreate, child: const Text('新建')),
        ],
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  final String message;
  final VoidCallback onRetry;
  const _ErrorState({required this.message, required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(28),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.error_outline, size: 48, color: context.error),
            const SizedBox(height: 12),
            Text(message, textAlign: TextAlign.center, style: AppTypography.caption(context)),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('重试')),
          ],
        ),
      ),
    );
  }
}
