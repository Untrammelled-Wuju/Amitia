import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';

class QueuedMessagesPanel extends StatelessWidget {
  const QueuedMessagesPanel({
    super.key,
    required this.items,
    required this.busy,
    required this.canSteer,
    required this.error,
    required this.onReorder,
    required this.onSteer,
    required this.onDelete,
    required this.onShowError,
  });

  final List<Map<String, dynamic>> items;
  final bool busy;
  final bool canSteer;
  final String error;
  final ValueChanged<List<String>> onReorder;
  final ValueChanged<String> onSteer;
  final ValueChanged<String> onDelete;
  final ValueChanged<String> onShowError;

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty && error.isEmpty) return const SizedBox.shrink();
    return Container(
      margin: const EdgeInsets.fromLTRB(12, 0, 12, 8),
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(
        color: context.surfacePrimary,
        border: Border.all(color: Theme.of(context).dividerColor),
        borderRadius: BorderRadius.circular(12),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (items.isNotEmpty)
            Text(
              '消息队列 ${items.length}',
              style: Theme.of(context).textTheme.labelSmall,
            ),
          if (items.isNotEmpty)
            ConstrainedBox(
              constraints: const BoxConstraints(maxHeight: 220),
              child: ReorderableListView.builder(
                shrinkWrap: true,
                buildDefaultDragHandles: false,
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
                  return Row(
                    key: ValueKey(id),
                    children: [
                      ReorderableDelayedDragStartListener(
                        index: index,
                        enabled: !busy,
                        child: const Padding(
                          padding: EdgeInsets.all(8),
                          child: Icon(Icons.drag_indicator, size: 18),
                        ),
                      ),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(switch (status) {
                              'failed' => '失败',
                              'uncertain' => '状态不确定',
                              'steering' => '引导确认中',
                              _ => '等待',
                            }, style: Theme.of(context).textTheme.labelSmall),
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
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            if (attachments is List && attachments.isNotEmpty)
                              Text(
                                attachments
                                    .whereType<Map>()
                                    .map(
                                      (attachment) =>
                                          (attachment['name'] ?? '').toString(),
                                    )
                                    .where((name) => name.isNotEmpty)
                                    .join(' · '),
                                maxLines: 1,
                                overflow: TextOverflow.ellipsis,
                                style: Theme.of(context).textTheme.labelSmall,
                              ),
                            if ((item['error'] ?? '').toString().isNotEmpty)
                              TextButton(
                                onPressed: () =>
                                    onShowError(item['error'].toString()),
                                child: const Text('查看原始错误'),
                              ),
                          ],
                        ),
                      ),
                      TextButton(
                        onPressed: busy || !canSteer || status != 'queued'
                            ? null
                            : () => onSteer(id),
                        child: const Text('引导'),
                      ),
                      IconButton(
                        onPressed: busy ? null : () => onDelete(id),
                        tooltip: '删除排队消息',
                        icon: const Icon(Icons.close, size: 18),
                      ),
                    ],
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
