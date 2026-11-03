import 'package:flutter/material.dart';

import '../../../../app/theme/app_colors.dart';

class ChatErrorNotice extends StatelessWidget {
  final String detail;
  final VoidCallback? onRetry;

  const ChatErrorNotice({super.key, required this.detail, this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(12, 0, 12, 38),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        mainAxisSize: MainAxisSize.min,
        children: [
          Row(
            children: [
              Expanded(child: Divider(color: context.borderPrimary, height: 1)),
              if (onRetry != null) ...[
                const SizedBox(width: 8),
                TextButton(onPressed: onRetry, child: const Text('重试')),
              ],
              const SizedBox(width: 10),
              Expanded(child: Divider(color: context.borderPrimary, height: 1)),
            ],
          ),
          const SizedBox(height: 8),
          SelectionArea(
            child: Text(
              '-$detail-',
              style: TextStyle(color: context.error, fontSize: 12, height: 1.5),
            ),
          ),
        ],
      ),
    );
  }
}
