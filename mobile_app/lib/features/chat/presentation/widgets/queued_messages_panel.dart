import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../core/widgets/amitia_popup_menu.dart';

class QueuedMessagesPanel extends StatelessWidget {
  const QueuedMessagesPanel({
    super.key,
    required this.items,
    required this.busy,
    required this.canSteer,
    required this.error,
    this.editingId = '',
    required this.onReorder,
    required this.onSteer,
    required this.onDelete,
    required this.onEdit,
    required this.onShowError,
  });

  final List<Map<String, dynamic>> items;
  final bool busy;
  final bool canSteer;
  final String error;
  final String editingId;
  final ValueChanged<List<String>> onReorder;
  final ValueChanged<String> onSteer;
  final ValueChanged<String> onDelete;
  final ValueChanged<String> onEdit;
  final ValueChanged<String> onShowError;

  String _statusLabel(String status) {
    return switch (status) {
      'running' => '执行中',
      'failed' => '失败',
      'uncertain' => '状态不确定',
      'steering' => '引导确认中',
      'steered' => '已引导',
      _ => '等待',
    };
  }

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty && error.isEmpty) return const SizedBox.shrink();
    final theme = Theme.of(context);
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 0, 12, 6),
      padding: const EdgeInsets.fromLTRB(10, 4, 4, 6),
      decoration: BoxDecoration(
        color: context.surfacePrimary.withValues(alpha: 0.94),
        borderRadius: BorderRadius.circular(16),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withValues(alpha: 0.18),
            blurRadius: 20,
            offset: const Offset(0, 8),
          ),
        ],
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (items.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(left: 4, top: 4, bottom: 2),
              child: Row(
                children: [
                  Text('排队消息', style: theme.textTheme.labelSmall),
                  const SizedBox(width: 6),
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 6),
                    decoration: BoxDecoration(
                      color: theme.dividerColor.withValues(alpha: 0.4),
                      borderRadius: BorderRadius.circular(999),
                    ),
                    child: Text(
                      '${items.length}',
                      style: theme.textTheme.labelSmall,
                    ),
                  ),
                ],
              ),
            ),
          if (items.isNotEmpty)
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 220),
              child: ReorderableListView.builder(
                shrinkWrap: true,
                buildDefaultDragHandles: false,
                padding: EdgeInsets.zero,
                itemCount: items.length,
                onReorderItem: (oldIndex, newIndex) {
                  if (busy) return;
                  final ids = items
                      .map((item) => item['requestId'].toString())
                      .toList(growable: true);
                  final id = ids.removeAt(oldIndex);
                  ids.insert(newIndex, id);
                  onReorder(ids);
                },
                itemBuilder: (context, index) {
                  final item = items[index];
                  final id = item['requestId'].toString();
                  final status = item['status'].toString();
                  final attachments = item['attachments'];
                  final quote = item['quote'];
                  final isEditing = id == editingId;
                  return ReorderableDelayedDragStartListener(
                    key: ValueKey(id),
                    index: index,
                    enabled: !busy,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(vertical: 1),
                      child: DecoratedBox(
                        decoration: BoxDecoration(
                          color: isEditing
                              ? theme.colorScheme.primary.withValues(alpha: 0.12)
                              : Colors.transparent,
                          borderRadius: BorderRadius.circular(10),
                        ),
                        child: Row(
                          crossAxisAlignment: CrossAxisAlignment.center,
                          children: [
                            Expanded(
                              child: Padding(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 4,
                                  vertical: 4,
                                ),
                                child: Column(
                                  crossAxisAlignment: CrossAxisAlignment.start,
                                  children: [
                                    Text(
                                      _statusLabel(status),
                                      style: theme.textTheme.labelSmall,
                                    ),
                                    Text(
                                      item['message'].toString(),
                                      maxLines: 2,
                                      overflow: TextOverflow.ellipsis,
                                    ),
                                    if (quote is Map)
                                      Text(
                                        '引用 ${quote['messageId'] ?? ''}',
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: theme.textTheme.labelSmall,
                                      ),
                                    if (attachments is List && attachments.isNotEmpty)
                                      Text(
                                        attachments
                                            .whereType<Map>()
                                            .map(
                                              (attachment) =>
                                                  (attachment['name'] ?? '')
                                                      .toString(),
                                            )
                                            .where((name) => name.isNotEmpty)
                                            .join(' · '),
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: theme.textTheme.labelSmall,
                                      ),
                                    if ((item['error'] ?? '')
                                        .toString()
                                        .isNotEmpty)
                                      TextButton(
                                        onPressed: () => onShowError(
                                          item['error'].toString(),
                                        ),
                                        child: const Text('查看原始错误'),
                                      ),
                                  ],
                                ),
                              ),
                            ),
                            TextButton(
                              onPressed: busy || !canSteer || status != 'queued'
                                  ? null
                                  : () => onSteer(id),
                              child: const Text('引导'),
                            ),
                            AmitiaPopupMenuButton<String>(
                              tooltip: '排队消息操作',
                              enabled: !busy,
                              icon: const Icon(Icons.more_horiz, size: 18),
                              padding: const EdgeInsets.all(6),
                              menuWidth: 168,
                              onSelected: (value) {
                                if (value == 'edit') onEdit(id);
                                if (value == 'delete') onDelete(id);
                              },
                              itemBuilder: (context) => [
                                PopupMenuItem<String>(
                                  value: 'edit',
                                  enabled: status == 'queued',
                                  child: const Text('编辑消息'),
                                ),
                                const PopupMenuItem<String>(
                                  value: 'delete',
                                  child: Text('关闭排队'),
                                ),
                              ],
                            ),
                            IconButton(
                              onPressed: busy ? null : () => onDelete(id),
                              tooltip: '删除排队消息',
                              icon: const Icon(Icons.close, size: 18),
                            ),
                          ],
                        ),
                      ),
                    ),
                  );
                },
              ),
            ),
          if (error.isNotEmpty)
            TextButton(
              onPressed: () => onShowError(error),
              child: const Text('排队消息同步失败，查看原始错误'),
            ),
        ],
      ),
    );
  }
}
