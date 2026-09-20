import 'package:flutter/material.dart';

class AmitiaRoleSwitchDivider extends StatelessWidget {
  final String characterName;

  const AmitiaRoleSwitchDivider({super.key, required this.characterName});

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 2, 16, 20),
      child: Row(
        children: [
          Expanded(child: Divider(color: colorScheme.outlineVariant)),
          Padding(
            padding: const EdgeInsets.symmetric(horizontal: 12),
            child: Text(
              '当前对话角色已切换为 $characterName',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(child: Divider(color: colorScheme.outlineVariant)),
        ],
      ),
    );
  }
}
