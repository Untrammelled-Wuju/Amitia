import 'package:flutter/material.dart';
import '../../../../core/services/owned_memory_scope.dart';

Future<({String scope, String role})?> showMemoryScopeTransferDialog(
  BuildContext context,
  List<Map<String, dynamic>> roles,
  String partition,
) {
  var target = partition == globalMemoryRole ? 'character' : 'global';
  var role = roles.isEmpty ? '' : roles.first['id'].toString();
  return showDialog<({String scope, String role})>(
    context: context,
    builder: (context) => StatefulBuilder(
      builder: (context, setState) => AlertDialog(
        title: const Text('调整所属分区'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Text('移动记录并保留来源和历史。持续事项保留原执行角色。'),
            const SizedBox(height: 16),
            DropdownButtonFormField<String>(
              initialValue: target,
              decoration: const InputDecoration(labelText: '目标分区'),
              items: const [
                DropdownMenuItem(value: 'global', child: Text('全局')),
                DropdownMenuItem(value: 'character', child: Text('角色')),
              ],
              onChanged: (value) => setState(() => target = value ?? 'global'),
            ),
            if (target == 'character')
              DropdownButtonFormField<String>(
                initialValue: role.isEmpty ? null : role,
                decoration: const InputDecoration(labelText: '目标角色'),
                items: roles
                    .map(
                      (item) => DropdownMenuItem(
                        value: item['id'].toString(),
                        child: Text((item['name'] ?? item['id']).toString()),
                      ),
                    )
                    .toList(),
                onChanged: (value) => setState(() => role = value ?? ''),
              ),
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('取消'),
          ),
          FilledButton(
            onPressed: target == 'character' && role.isEmpty
                ? null
                : () => Navigator.pop(context, (scope: target, role: role)),
            child: const Text('确认迁移'),
          ),
        ],
      ),
    ),
  );
}
