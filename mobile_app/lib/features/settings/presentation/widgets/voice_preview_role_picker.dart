import 'package:flutter/material.dart';

Future<String?> pickVoicePreviewRole(
  BuildContext context,
  List<Map<String, dynamic>> roles, {
  String preferredRoleId = '',
}) async {
  if (roles.isEmpty) {
    throw StateError('请先创建或选择一个角色，再试听复刻音色');
  }
  if (roles.length == 1) {
    final roleId = (roles.single['id'] ?? '').toString().trim();
    if (roleId.isEmpty) throw StateError('角色数据缺少有效标识');
    return roleId;
  }
  var selected = roles.any((role) => role['id'] == preferredRoleId)
      ? preferredRoleId
      : '';
  return showDialog<String>(
    context: context,
    builder: (dialogContext) => StatefulBuilder(
      builder: (dialogContext, setDialogState) => AlertDialog(
        title: const Text('选择试听角色'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            const Text('复刻音色试听需要由具体角色的数据所有者授权。'),
            const SizedBox(height: 12),
            DropdownButtonFormField<String>(
              key: const Key('voice-preview-role'),
              initialValue: selected.isEmpty ? null : selected,
              isExpanded: true,
              decoration: const InputDecoration(
                labelText: '角色',
                border: OutlineInputBorder(),
                isDense: true,
              ),
              items: roles
                  .map(
                    (role) => DropdownMenuItem<String>(
                      value: (role['id'] ?? '').toString(),
                      child: Text(
                        ((role['name'] ?? '').toString().trim().isEmpty
                                ? role['id']
                                : role['name'])
                            .toString(),
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                  )
                  .toList(),
              onChanged: (value) =>
                  setDialogState(() => selected = value ?? ''),
            ),
            const SizedBox(height: 8),
            const Text('该选择只用于本次试听，不改变默认聊天角色。'),
          ],
        ),
        actions: <Widget>[
          TextButton(
            onPressed: () => Navigator.pop(dialogContext),
            child: const Text('取消'),
          ),
          FilledButton(
            key: const Key('voice-preview-role-confirm'),
            onPressed: selected.isEmpty
                ? null
                : () => Navigator.pop(dialogContext, selected),
            child: const Text('试听'),
          ),
        ],
      ),
    ),
  );
}
