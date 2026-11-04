import 'dart:developer' as developer;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';

Future<Map<String, dynamic>?> showTeamCreateSheet(
  BuildContext context,
  WidgetRef ref,
) async {
  final api = ref.read(rawBackendServiceApiProvider);
  if (api == null) throw StateError('执行 Core 尚未就绪');
  final values = await Future.wait<dynamic>([
    api.get<dynamic>('/api/roles', queryParameters: {'includeDisabled': false}),
    api.get<dynamic>('/api/roles/teams/templates'),
  ]);
  final roles = _list(values[0]);
  final templates = _list(values[1]);
  if (!context.mounted) return null;
  final name = TextEditingController();
  final description = TextEditingController();
  String templateId = '';
  String leadAgentId = '';
  final memberIds = <String>{};
  bool sharedMemory = true;
  bool sharedWorkspace = true;
  bool creating = false;
  String? error;
  final result = await showModalBottomSheet<Map<String, dynamic>>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    builder: (sheetContext) => StatefulBuilder(
      builder: (context, setSheetState) => Padding(
        padding: EdgeInsets.fromLTRB(
          AppSpacing.pagePadding,
          AppSpacing.md,
          AppSpacing.pagePadding,
          MediaQuery.viewInsetsOf(context).bottom + AppSpacing.lg,
        ),
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text('新建团队', style: AppTypography.pageTitle(context)),
              SizedBox(height: AppSpacing.xs),
              Text(
                '创建持久团队，之后可复用负责人和成员配置',
                style: AppTypography.caption(context),
              ),
              SizedBox(height: AppSpacing.md),
              if (templates.isNotEmpty)
                DropdownButtonFormField<String>(
                  initialValue: templateId.isEmpty ? null : templateId,
                  decoration: const InputDecoration(labelText: '团队模板'),
                  items: templates
                      .map(
                        (item) => DropdownMenuItem(
                          value: item['id'].toString(),
                          child: Text(item['name']?.toString() ?? ''),
                        ),
                      )
                      .toList(growable: false),
                  onChanged: (value) {
                    if (value == null) return;
                    final template = templates.firstWhere(
                      (item) => item['id']?.toString() == value,
                    );
                    setSheetState(() {
                      templateId = value;
                      name.text = template['name']?.toString() ?? '';
                      description.text =
                          template['description']?.toString() ?? '';
                      sharedMemory = template['sharedMemory'] == true;
                      sharedWorkspace = template['sharedWorkspace'] == true;
                    });
                  },
                ),
              if (templates.isNotEmpty) SizedBox(height: AppSpacing.sm),
              TextField(
                controller: name,
                onChanged: (_) => setSheetState(() {}),
                decoration: const InputDecoration(labelText: '团队名称'),
              ),
              SizedBox(height: AppSpacing.sm),
              TextField(
                controller: description,
                minLines: 2,
                maxLines: 4,
                decoration: const InputDecoration(labelText: '说明'),
              ),
              SizedBox(height: AppSpacing.sm),
              DropdownButtonFormField<String>(
                initialValue: leadAgentId.isEmpty ? null : leadAgentId,
                decoration: const InputDecoration(labelText: '负责人'),
                items: roles
                    .map(
                      (role) => DropdownMenuItem(
                        value: role['id'].toString(),
                        child: Text(role['name']?.toString() ?? ''),
                      ),
                    )
                    .toList(growable: false),
                onChanged: (value) => setSheetState(() {
                  leadAgentId = value ?? '';
                  memberIds.remove(leadAgentId);
                }),
              ),
              SizedBox(height: AppSpacing.sm),
              Text('团队成员', style: AppTypography.label(context)),
              SizedBox(height: AppSpacing.xs),
              Wrap(
                spacing: AppSpacing.xs,
                runSpacing: AppSpacing.xs,
                children: roles
                    .where((role) => role['id']?.toString() != leadAgentId)
                    .map((role) {
                      final id = role['id'].toString();
                      return FilterChip(
                        label: Text(role['name']?.toString() ?? ''),
                        selected: memberIds.contains(id),
                        onSelected: (selected) => setSheetState(() {
                          if (selected) {
                            memberIds.add(id);
                          } else {
                            memberIds.remove(id);
                          }
                        }),
                      );
                    })
                    .toList(growable: false),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('共享团队记忆'),
                value: sharedMemory,
                onChanged: (value) => setSheetState(() => sharedMemory = value),
              ),
              SwitchListTile(
                contentPadding: EdgeInsets.zero,
                title: const Text('共享团队文件'),
                value: sharedWorkspace,
                onChanged: (value) =>
                    setSheetState(() => sharedWorkspace = value),
              ),
              if (error != null) ...[
                SizedBox(height: AppSpacing.sm),
                SelectableText(
                  error!,
                  style: TextStyle(color: context.error, fontSize: 12),
                ),
              ],
              SizedBox(height: AppSpacing.md),
              FilledButton(
                onPressed: creating || name.text.trim().isEmpty
                    ? null
                    : () async {
                        setSheetState(() {
                          creating = true;
                          error = null;
                        });
                        try {
                          final payload = <String, dynamic>{
                            'templateId': templateId,
                            'name': name.text.trim(),
                            'description': description.text.trim(),
                            'leadAgentId': leadAgentId,
                            'memberIds': memberIds.toList(growable: false),
                            'sharedMemory': sharedMemory,
                            'sharedWorkspace': sharedWorkspace,
                          };
                          final response = templateId.isNotEmpty
                              ? await api.post<dynamic>(
                                  '/api/roles/teams/from-template',
                                  data: payload,
                                )
                              : await api.post<dynamic>(
                                  '/api/roles/teams',
                                  data: payload,
                                );
                          if (response is Map) {
                            final team = response['team'] is Map
                                ? Map<String, dynamic>.from(
                                    response['team'] as Map,
                                  )
                                : Map<String, dynamic>.from(response);
                            if (sheetContext.mounted) {
                              Navigator.pop(sheetContext, team);
                            }
                          } else {
                            throw StateError('创建团队响应格式无效: $response');
                          }
                        } catch (cause, stackTrace) {
                          developer.log(
                            '[original-error] $cause',
                            name: 'characters.teams.create-sheet',
                            error: cause,
                            stackTrace: stackTrace,
                          );
                          setSheetState(() {
                            creating = false;
                            error = cause.toString();
                          });
                        }
                      },
                child: Text(creating ? '创建中' : '创建团队'),
              ),
            ],
          ),
        ),
      ),
    ),
  );
  name.dispose();
  description.dispose();
  return result;
}

List<Map<String, dynamic>> _list(dynamic value) {
  if (value is! List) return const [];
  return value
      .whereType<Map>()
      .map((item) => Map<String, dynamic>.from(item))
      .toList(growable: false);
}
