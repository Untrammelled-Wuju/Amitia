import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../../app/theme/app_colors.dart';
import '../../../../app/theme/app_spacing.dart';
import '../../../../app/theme/app_typography.dart';
import '../../../../core/models/project.dart';
import '../../../../core/services/providers.dart';
import '../../../../core/widgets/amitia_misc.dart';
import 'project_workspace_picker.dart';

class ProjectPickResult {
  const ProjectPickResult(this.project);

  final ProjectDto? project;
}

Future<ProjectPickResult?> showProjectPickerSheet(
  BuildContext context, {
  required List<ProjectDto> projects,
  String title = '选择项目',
  bool allowNoProject = true,
}) {
  return showModalBottomSheet<ProjectPickResult>(
    context: context,
    useSafeArea: true,
    showDragHandle: true,
    backgroundColor: context.surfacePrimary,
    builder: (sheetContext) {
      final available = projects
          .where(
            (project) =>
                !project.readOnly &&
                project.available &&
                project.hiddenAt.isEmpty &&
                project.workspaces.isNotEmpty,
          )
          .toList(growable: false);
      return ConstrainedBox(
        constraints: BoxConstraints(
          maxHeight: MediaQuery.sizeOf(sheetContext).height * 0.72,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: EdgeInsets.fromLTRB(
                AppSpacing.lg,
                0,
                AppSpacing.lg,
                AppSpacing.sm,
              ),
              child: Text(title, style: AppTypography.pageTitle(sheetContext)),
            ),
            if (allowNoProject)
              ListTile(
                leading: const Icon(Icons.chat_bubble_outline),
                title: const Text('最近对话'),
                subtitle: const Text('不加入项目'),
                onTap: () => Navigator.of(
                  sheetContext,
                ).pop(const ProjectPickResult(null)),
              ),
            if (available.isEmpty)
              Padding(
                padding: EdgeInsets.fromLTRB(
                  AppSpacing.lg,
                  AppSpacing.md,
                  AppSpacing.lg,
                  AppSpacing.xl,
                ),
                child: Text('暂无可用项目'),
              )
            else
              Flexible(
                child: ListView.builder(
                  shrinkWrap: true,
                  itemCount: available.length,
                  itemBuilder: (context, index) {
                    final project = available[index];
                    final primary = project.workspaces
                        .where(
                          (workspace) =>
                              workspace.id == project.primaryWorkspaceId,
                        )
                        .firstOrNull;
                    return ListTile(
                      leading: const Icon(Icons.folder_outlined),
                      title: Text(
                        project.name,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      subtitle: Text(
                        primary == null
                            ? '${project.workspaces.length} 个工作区'
                            : '主工作区：${primary.name}',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                      onTap: () => Navigator.of(
                        sheetContext,
                      ).pop(ProjectPickResult(project)),
                    );
                  },
                ),
              ),
          ],
        ),
      );
    },
  );
}

Future<ProjectDto?> showCreateProjectSheet(
  BuildContext context,
  WidgetRef ref,
) async {
  final service = ref.read(chatServiceProvider);
  if (!await service.owned.refresh()) {
    throw StateError(
      service.owned.notice.isEmpty ? '设备数据服务尚未就绪' : service.owned.notice,
    );
  }
  final view = await service.owned.data(
    'memory',
    characterId: service.owned.selectRole(null),
  );
  final expectedScope = Map<String, dynamic>.from(
    view['executionScope'] as Map,
  );
  final deviceId = (expectedScope['targetDeviceId'] ?? '').toString().trim();
  if (deviceId.isEmpty) throw StateError('无法确认项目工作区所属设备');
  if (!context.mounted) return null;

  final nameController = TextEditingController();
  final workspaces = <ProjectWorkspaceDto>[];
  var primaryWorkspaceId = '';
  var busy = false;
  String? error;

  final created = await showModalBottomSheet<ProjectDto>(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    backgroundColor: context.surfacePrimary,
    builder: (sheetContext) => StatefulBuilder(
      builder: (context, setSheetState) {
        Future<void> addWorkspace() async {
          if (busy) return;
          setSheetState(() {
            busy = true;
            error = null;
          });
          try {
            final mount = await pickAndRegisterProjectWorkspace(ref);
            if (mount == null || !sheetContext.mounted) return;
            if (workspaces.any((item) => item.id == mount.id)) {
              throw StateError('该工作区已添加');
            }
            final workspace = ProjectWorkspaceDto(
              id: mount.id,
              deviceId: deviceId,
              name: mount.name.trim().isEmpty ? '工作目录' : mount.name.trim(),
              kind: mount.kind,
              rootUri: mount.rootUri,
              readOnly: mount.readOnly,
            );
            setSheetState(() {
              workspaces.add(workspace);
              if (primaryWorkspaceId.isEmpty) {
                primaryWorkspaceId = workspace.id;
              }
              if (nameController.text.trim().isEmpty) {
                nameController.text = workspace.name;
              }
            });
          } catch (value) {
            if (sheetContext.mounted) {
              setSheetState(() => error = value.toString());
            }
          } finally {
            if (sheetContext.mounted) {
              setSheetState(() => busy = false);
            }
          }
        }

        Future<void> createProject() async {
          if (busy) return;
          final name = nameController.text.trim();
          if (name.isEmpty) {
            setSheetState(() => error = '请输入项目名称');
            return;
          }
          if (workspaces.isEmpty || primaryWorkspaceId.isEmpty) {
            setSheetState(() => error = '请至少添加一个主工作区');
            return;
          }
          setSheetState(() {
            busy = true;
            error = null;
          });
          try {
            final project = await service.createProject(
              name: name,
              workspaces: List<ProjectWorkspaceDto>.unmodifiable(workspaces),
              primaryWorkspaceId: primaryWorkspaceId,
              expectedScope: expectedScope,
            );
            if (sheetContext.mounted) Navigator.of(sheetContext).pop(project);
          } catch (value) {
            if (sheetContext.mounted) {
              setSheetState(() => error = '创建项目失败：$value');
            }
          } finally {
            if (sheetContext.mounted) {
              setSheetState(() => busy = false);
            }
          }
        }

        return Padding(
          padding: EdgeInsets.fromLTRB(
            AppSpacing.lg,
            0,
            AppSpacing.lg,
            MediaQuery.viewInsetsOf(context).bottom + AppSpacing.lg,
          ),
          child: SingleChildScrollView(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Text('新建项目', style: AppTypography.pageTitle(context)),
                SizedBox(height: AppSpacing.lg),
                AmitiaTextField(
                  controller: nameController,
                  hintText: '项目名称',
                  maxLength: 120,
                ),
                SizedBox(height: AppSpacing.lg),
                Row(
                  children: [
                    Expanded(
                      child: Text(
                        '工作区',
                        style: AppTypography.sectionTitle(context),
                      ),
                    ),
                    TextButton.icon(
                      onPressed: busy ? null : addWorkspace,
                      icon: Icon(
                        workspaces.isEmpty
                            ? Icons.create_new_folder_outlined
                            : Icons.add,
                      ),
                      label: Text(workspaces.isEmpty ? '选择主工作区' : '添加副工作区'),
                    ),
                  ],
                ),
                if (workspaces.isEmpty)
                  Padding(
                    padding: EdgeInsets.symmetric(vertical: AppSpacing.md),
                    child: Text(
                      '主工作区用于相对路径和默认执行目录，副工作区可同时授权给项目使用。',
                      style: AppTypography.caption(context),
                    ),
                  )
                else
                  for (final workspace in workspaces)
                    Padding(
                      padding: const EdgeInsets.only(bottom: 8),
                      child: Material(
                        color: context.backgroundPrimary,
                        borderRadius: BorderRadius.circular(12),
                        child: InkWell(
                          borderRadius: BorderRadius.circular(12),
                          onTap: busy
                              ? null
                              : () => setSheetState(
                                  () => primaryWorkspaceId = workspace.id,
                                ),
                          child: Padding(
                            padding: const EdgeInsets.all(12),
                            child: Row(
                              children: [
                                Icon(
                                  projectWorkspaceIcon(workspace.kind),
                                  size: 20,
                                  color: context.textSecondary,
                                ),
                                const SizedBox(width: 10),
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment:
                                        CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        workspace.name,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: AppTypography.body(context),
                                      ),
                                      const SizedBox(height: 2),
                                      Text(
                                        workspace.id == primaryWorkspaceId
                                            ? '主工作区'
                                            : '副工作区',
                                        style: AppTypography.label(context)
                                            .copyWith(
                                              color:
                                                  workspace.id ==
                                                      primaryWorkspaceId
                                                  ? context.accentPrimary
                                                  : context.textTertiary,
                                            ),
                                      ),
                                    ],
                                  ),
                                ),
                                IconButton(
                                  tooltip: '切换为主工作区',
                                  onPressed:
                                      busy || workspace.id == primaryWorkspaceId
                                      ? null
                                      : () => setSheetState(
                                          () =>
                                              primaryWorkspaceId = workspace.id,
                                        ),
                                  icon: const Icon(
                                    Icons.radio_button_checked,
                                    size: 20,
                                  ),
                                ),
                                IconButton(
                                  tooltip: '移除工作区',
                                  onPressed: busy
                                      ? null
                                      : () => setSheetState(() {
                                          workspaces.remove(workspace);
                                          if (primaryWorkspaceId ==
                                              workspace.id) {
                                            primaryWorkspaceId =
                                                workspaces.firstOrNull?.id ??
                                                '';
                                          }
                                        }),
                                  icon: const Icon(
                                    Icons.remove_circle_outline,
                                    size: 20,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                if (error != null)
                  Padding(
                    padding: EdgeInsets.only(top: AppSpacing.sm),
                    child: Text(
                      error!,
                      style: AppTypography.bodySmall(
                        context,
                      ).copyWith(color: context.error),
                    ),
                  ),
                SizedBox(height: AppSpacing.lg),
                FilledButton(
                  onPressed: busy ? null : createProject,
                  child: busy
                      ? const SizedBox(
                          width: 18,
                          height: 18,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Text('创建项目'),
                ),
              ],
            ),
          ),
        );
      },
    ),
  );
  nameController.dispose();
  return created;
}

IconData projectWorkspaceIcon(String kind) {
  return kind == 'saf' ? Icons.folder_shared_outlined : Icons.folder_outlined;
}
