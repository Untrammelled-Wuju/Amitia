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
import '../../../../core/widgets/amitia_misc.dart';
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

Future<GroupChatSelection?> showGroupChatInviteSheet(
  BuildContext context, {
  required List<CharacterDto> characters,
  required String preferredRoleId,
  List<ProjectDto> projects = const <ProjectDto>[],
  bool showProjectSelector = false,
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
  var selectedProjectId = availableProjects.firstOrNull?.id ?? '';
  final selected = <String>{};
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
                if (showProjectSelector) ...[
                  SizedBox(height: AppSpacing.lg),
                  Text('选择工作区', style: AppTypography.sectionTitle(context)),
                  SizedBox(height: AppSpacing.sm),
                  if (availableProjects.isEmpty)
                    Text(
                      '暂无可用项目，请先在项目模式中创建项目',
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
                          (showProjectSelector && selectedProjectId.isEmpty)
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
  bool showProjectSelector = false,
}) async {
  if (characters.length < 2) {
    amitiaSnackBar(context, '至少需要两个可用角色才能创建群聊，请先创建角色');
    return null;
  }
  final preferredRoleId = ref.read(currentCharacterIdProvider);
  final selection = await showGroupChatInviteSheet(
    context,
    characters: characters,
    preferredRoleId: preferredRoleId,
    projects: projects,
    showProjectSelector: showProjectSelector,
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
