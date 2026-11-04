import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/character.dart';
import '../../../../core/models/project.dart';
import '../../../../core/settings/current_character_provider.dart';
import '../../../../core/services/device_owned_chat_service.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/backend_transport/providers/backend_transport_providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import '../../../characters/presentation/widgets/team_create_sheet.dart';
import '../../runtime/conversation_runtime_controller.dart';

class GroupChatSelection {
  const GroupChatSelection({
    required this.baseRoleId,
    required this.memberRoleIds,
    this.projectId = '',
  });

  final String baseRoleId;
  final List<String> memberRoleIds;
  final String projectId;
}

class CreatedGroupChat {
  const CreatedGroupChat({required this.group, required this.baseRoleId});

  final OwnedGroupState group;
  final String baseRoleId;
}

class GroupTeamOption {
  const GroupTeamOption({
    required this.id,
    required this.name,
    required this.leadCharacterId,
    required this.memberCharacterIds,
  });

  final String id;
  final String name;
  final String leadCharacterId;
  final List<String> memberCharacterIds;
}

void _applyTeam(
  GroupTeamOption team,
  List<CharacterDto> characters,
  String fallbackBaseRoleId,
  void Function(String baseRoleId, Set<String> members, Set<String> derived)
  apply,
) {
  final availableIds = characters.map((role) => role.id).toSet();
  final ids = team.memberCharacterIds
      .where(availableIds.contains)
      .toSet()
      .toList(growable: false);
  if (ids.isEmpty) {
    apply(fallbackBaseRoleId, <String>{}, <String>{});
    return;
  }
  final baseRoleId = availableIds.contains(team.leadCharacterId)
      ? team.leadCharacterId
      : ids.first;
  final members = ids.where((id) => id != baseRoleId).take(7).toSet();
  apply(baseRoleId, members, Set<String>.from(members));
}

Future<List<GroupTeamOption>> _loadGroupTeams(WidgetRef ref) async {
  final api = ref.read(rawBackendServiceApiProvider);
  if (api == null) throw StateError('执行 Core 尚未就绪');
  final values = await Future.wait<dynamic>([
    api.get<dynamic>('/api/roles/teams'),
    api.get<dynamic>('/api/roles', queryParameters: {'includeDisabled': false}),
  ]);
  final teams = _mapList(values[0]);
  final roles = _mapList(values[1]);
  final details = await Future.wait(
    teams.map(
      (team) => api.get<dynamic>(
        '/api/roles/teams/${Uri.encodeComponent(team['id'].toString())}',
      ),
    ),
  );
  return details
      .map((detail) => _teamOption(detail, roles))
      .whereType<GroupTeamOption>()
      .toList(growable: false);
}

Future<GroupTeamOption?> _loadGroupTeamOption(WidgetRef ref, String id) async {
  final api = ref.read(rawBackendServiceApiProvider);
  if (api == null) throw StateError('执行 Core 尚未就绪');
  final values = await Future.wait<dynamic>([
    api.get<dynamic>('/api/roles/teams/${Uri.encodeComponent(id)}'),
    api.get<dynamic>('/api/roles', queryParameters: {'includeDisabled': false}),
  ]);
  return _teamOption(values[0], _mapList(values[1]));
}

GroupTeamOption? _teamOption(dynamic value, List<Map<String, dynamic>> roles) {
  if (value is! Map) return null;
  final detail = Map<String, dynamic>.from(value);
  final team = detail['team'] is Map
      ? Map<String, dynamic>.from(detail['team'] as Map)
      : detail;
  final id = team['id']?.toString() ?? '';
  if (id.isEmpty) return null;
  final roleByAgentId = <String, Map<String, dynamic>>{
    for (final role in roles)
      if (role['id'] != null) role['id'].toString(): role,
  };
  final memberCharacterIds = <String>[];
  for (final member in _mapList(detail['members'])) {
    if (member['memberType']?.toString() != 'agent') continue;
    final role = roleByAgentId[member['memberId']?.toString() ?? ''];
    final characterId = role?['characterId']?.toString() ?? '';
    if (characterId.isNotEmpty && !memberCharacterIds.contains(characterId)) {
      memberCharacterIds.add(characterId);
    }
  }
  final leadRole = roleByAgentId[team['leadAgentId']?.toString() ?? ''];
  return GroupTeamOption(
    id: id,
    name: team['name']?.toString() ?? '未命名团队',
    leadCharacterId: leadRole?['characterId']?.toString() ?? '',
    memberCharacterIds: memberCharacterIds,
  );
}

List<Map<String, dynamic>> _mapList(dynamic value) {
  if (value is! List) return const [];
  return value
      .whereType<Map>()
      .map((item) => Map<String, dynamic>.from(item))
      .toList(growable: false);
}

Future<GroupChatSelection?> showGroupChatInviteSheet(
  BuildContext context, {
  required WidgetRef ref,
  required List<CharacterDto> characters,
  required String preferredRoleId,
  List<ProjectDto> projects = const <ProjectDto>[],
  List<GroupTeamOption> teams = const <GroupTeamOption>[],
  bool showProjectSelector = false,
  bool projectRequired = false,
}) {
  var baseRoleId = characters.any((role) => role.id == preferredRoleId)
      ? preferredRoleId
      : characters.first.id;
  final availableProjects = projects
      .where(
        (project) =>
            !project.readOnly &&
            project.available &&
            project.hiddenAt.isEmpty &&
            project.workspaces.isNotEmpty,
      )
      .toList(growable: false);
  var selectedProjectId = '';
  final selected = <String>{};
  var selectedTeamId = '';
  var teamDerivedMembers = <String>{};
  var creatingTeam = false;
  final availableTeams = [...teams];
  return showModalBottomSheet<GroupChatSelection>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    builder: (sheetContext) => StatefulBuilder(
      builder: (context, setSheetState) {
        final inviteRoles = characters
            .where((role) => role.id != baseRoleId)
            .toList(growable: false);
        return Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            AppSpacing.xs,
            AppSpacing.lg,
            AppSpacing.lg,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('新建群聊', style: AppTypography.pageTitle(context)),
                SizedBox(height: AppSpacing.xs),
                Text('选择默认发言角色和要邀请的群成员', style: AppTypography.caption(context)),
                SizedBox(height: AppSpacing.lg),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '从团队创建',
                        style: AppTypography.sectionTitle(context),
                      ),
                    ),
                    TextButton.icon(
                      onPressed: creatingTeam
                          ? null
                          : () async {
                              setSheetState(() => creatingTeam = true);
                              try {
                                final created = await showTeamCreateSheet(
                                  context,
                                  ref,
                                );
                                final id = created?['id']?.toString() ?? '';
                                if (id.isEmpty) return;
                                final option = await _loadGroupTeamOption(
                                  ref,
                                  id,
                                );
                                if (option == null) {
                                  throw StateError('新团队缺少可用角色成员');
                                }
                                if (!availableTeams.any(
                                  (team) => team.id == option.id,
                                )) {
                                  availableTeams.insert(0, option);
                                }
                                _applyTeam(option, characters, baseRoleId, (
                                  base,
                                  members,
                                  derived,
                                ) {
                                  setSheetState(() {
                                    baseRoleId = base;
                                    selected
                                      ..clear()
                                      ..addAll(members);
                                    teamDerivedMembers = derived;
                                    selectedTeamId = option.id;
                                    selectedProjectId = '';
                                  });
                                });
                              } catch (error) {
                                if (context.mounted) {
                                  amitiaSnackBar(context, '创建团队失败：$error');
                                }
                              } finally {
                                if (context.mounted) {
                                  setSheetState(() => creatingTeam = false);
                                }
                              }
                            },
                      icon: const Icon(Icons.add),
                      label: Text(creatingTeam ? '创建中' : '创建团队'),
                    ),
                  ],
                ),
                SizedBox(height: AppSpacing.sm),
                if (availableTeams.isEmpty)
                  Text(
                    '暂无持久团队，也可以直接在下方选择角色',
                    style: AppTypography.caption(context),
                  )
                else
                  DropdownButtonFormField<String>(
                    initialValue: selectedTeamId.isEmpty
                        ? null
                        : selectedTeamId,
                    decoration: const InputDecoration(labelText: '选择团队'),
                    items: availableTeams
                        .map(
                          (team) => DropdownMenuItem(
                            value: team.id,
                            child: Text(team.name),
                          ),
                        )
                        .toList(growable: false),
                    onChanged: (value) {
                      if (value == null) {
                        setSheetState(() {
                          selected.removeAll(teamDerivedMembers);
                          teamDerivedMembers = <String>{};
                          selectedTeamId = '';
                        });
                        return;
                      }
                      final team = availableTeams.firstWhere(
                        (item) => item.id == value,
                      );
                      _applyTeam(team, characters, baseRoleId, (
                        base,
                        members,
                        derived,
                      ) {
                        setSheetState(() {
                          baseRoleId = base;
                          selected
                            ..clear()
                            ..addAll(members);
                          teamDerivedMembers = derived;
                          selectedTeamId = team.id;
                          selectedProjectId = '';
                        });
                      });
                    },
                  ),
                Text(
                  '团队仅作为本次群聊的成员快照，不会建立持久关联。',
                  style: AppTypography.caption(context),
                ),
                if (showProjectSelector) ...[
                  SizedBox(height: AppSpacing.lg),
                  Text('创建到项目', style: AppTypography.sectionTitle(context)),
                  SizedBox(height: AppSpacing.sm),
                  if (availableProjects.isEmpty)
                    Text(
                      projectRequired
                          ? '项目模式下必须选择项目，请先创建可用项目'
                          : '暂无可用项目，可不加入项目',
                      style: AppTypography.bodySmall(
                        context,
                      ).copyWith(color: context.warning),
                    )
                  else
                    for (final project in availableProjects)
                      ListTile(
                        key: ValueKey('group-project-${project.id}'),
                        contentPadding: EdgeInsets.zero,
                        leading: Icon(
                          selectedProjectId == project.id
                              ? Icons.radio_button_checked
                              : Icons.radio_button_off,
                        ),
                        title: Text(
                          project.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        subtitle: Text(
                          _primaryWorkspaceLabel(project),
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                        ),
                        onTap: () =>
                            setSheetState(() => selectedProjectId = project.id),
                      ),
                ],
                SizedBox(height: AppSpacing.lg),
                Text('默认发言角色', style: AppTypography.sectionTitle(context)),
                SizedBox(height: AppSpacing.sm),
                RadioGroup<String>(
                  groupValue: baseRoleId,
                  onChanged: (value) {
                    if (value == null) return;
                    setSheetState(() {
                      baseRoleId = value;
                      selected.remove(value);
                    });
                  },
                  child: Column(
                    children: [
                      for (final role in characters)
                        RadioListTile<String>(
                          value: role.id,
                          contentPadding: EdgeInsets.zero,
                          title: Text(role.name),
                          secondary: _RoleAvatar(role: role),
                        ),
                    ],
                  ),
                ),
                SizedBox(height: AppSpacing.md),
                Text(
                  '邀请角色 (${selected.length})',
                  style: AppTypography.sectionTitle(context),
                ),
                SizedBox(height: AppSpacing.sm),
                for (final role in inviteRoles)
                  CheckboxListTile(
                    value: selected.contains(role.id),
                    contentPadding: EdgeInsets.zero,
                    onChanged: (value) {
                      setSheetState(() {
                        if (value == true) {
                          if (selected.length < 7) selected.add(role.id);
                        } else {
                          selected.remove(role.id);
                        }
                      });
                    },
                    title: Text(role.name),
                    secondary: _RoleAvatar(role: role),
                  ),
                SizedBox(height: AppSpacing.md),
                FilledButton(
                  onPressed:
                      selected.isEmpty ||
                          (projectRequired && selectedProjectId.isEmpty)
                      ? null
                      : () => Navigator.of(sheetContext).pop(
                          GroupChatSelection(
                            baseRoleId: baseRoleId,
                            memberRoleIds: selected.toList(growable: false),
                            projectId: showProjectSelector
                                ? selectedProjectId
                                : '',
                          ),
                        ),
                  child: const Text('创建群聊'),
                ),
              ],
            ),
          ),
        );
      },
    ),
  );
}

Future<CreatedGroupChat?> createGroupChatWithInvites(
  BuildContext context,
  WidgetRef ref, {
  required List<CharacterDto> characters,
  List<ProjectDto> projects = const <ProjectDto>[],
  bool showProjectSelector = true,
  bool projectRequired = false,
}) async {
  if (characters.length < 2) {
    amitiaSnackBar(context, '至少需要两个可用角色才能创建群聊，请先创建角色');
    return null;
  }
  final preferredRoleId = ref.read(currentCharacterIdProvider);
  List<GroupTeamOption> teams = const [];
  try {
    teams = await _loadGroupTeams(ref);
  } catch (error) {
    if (context.mounted) amitiaSnackBar(context, '团队加载失败：$error');
  }
  if (!context.mounted) return null;
  final selection = await showGroupChatInviteSheet(
    context,
    ref: ref,
    characters: characters,
    preferredRoleId: preferredRoleId,
    projects: projects,
    teams: teams,
    showProjectSelector: showProjectSelector,
    projectRequired: projectRequired,
  );
  if (selection == null || !context.mounted) return null;
  try {
    final group = await ref
        .read(conversationRuntimeControllerProvider)
        .createGroupConversation(
          selection.baseRoleId,
          selection.memberRoleIds,
          projectId: selection.projectId,
        );
    if (!context.mounted) return null;
    ref.read(currentCharacterIdProvider.notifier).state = selection.baseRoleId;
    ref.read(activeConversationIdProvider.notifier).state =
        group.conversationId;
    return CreatedGroupChat(group: group, baseRoleId: selection.baseRoleId);
  } catch (error) {
    if (context.mounted) {
      amitiaSnackBar(context, '创建群聊失败：$error');
    }
    return null;
  }
}

String _primaryWorkspaceLabel(ProjectDto project) {
  for (final workspace in project.workspaces) {
    if (workspace.id == project.primaryWorkspaceId) {
      return '主工作区：${workspace.name}';
    }
  }
  return project.workspaces.isEmpty
      ? '未设置工作区'
      : '${project.workspaces.length} 个工作区';
}

class _RoleAvatar extends StatelessWidget {
  const _RoleAvatar({required this.role});

  final CharacterDto role;

  @override
  Widget build(BuildContext context) {
    return CircleAvatar(
      backgroundImage: role.avatar.trim().isEmpty
          ? null
          : NetworkImage(role.avatar),
      child: role.avatar.trim().isEmpty
          ? Text(role.name.characters.firstOrNull ?? '?')
          : null,
    );
  }
}
