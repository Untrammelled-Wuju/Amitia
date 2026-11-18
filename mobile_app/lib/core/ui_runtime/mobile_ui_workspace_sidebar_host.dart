import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'mobile_extension_slot.dart';
import 'mobile_ui_host_actions.dart';
import 'mobile_workspace_sidebar_runtime.dart';

class MobileUIWorkspaceSidebarHost extends ConsumerWidget {
  const MobileUIWorkspaceSidebarHost({
    super.key,
    this.context = const {},
    this.actions = const {},
  });

  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sidebar = ref.watch(mobileWorkspaceSidebarProvider);
    if (!sidebar.open) return const SizedBox.shrink();
    final controller = ref.read(mobileWorkspaceSidebarProvider.notifier);
    final sidebarActions = buildMobileUIHostActions(
      context: context,
      ref: ref,
      actions: actions,
    );
    final sidebarContext = <String, dynamic>{
      ...this.context,
      ...sidebar.toContext(),
      'surface': 'workspace-sidebar',
      'surfaceRole': 'sidebar',
    };
    final width = MediaQuery.sizeOf(context).width.clamp(280.0, 420.0) * 0.88;
    return Stack(
      children: [
        Positioned.fill(
          child: GestureDetector(
            behavior: HitTestBehavior.translucent,
            onTap: controller.close,
            child: ColoredBox(
              color: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.2),
            ),
          ),
        ),
        Positioned(
          top: MediaQuery.paddingOf(context).top + 8,
          left: 8,
          bottom: MediaQuery.paddingOf(context).bottom + 8,
          width: width,
          child: Material(
            elevation: 18,
            borderRadius: BorderRadius.circular(14),
            clipBehavior: Clip.antiAlias,
            color: Theme.of(context).colorScheme.surface,
            child: Column(
              children: [
                SizedBox(
                  height: 48,
                  child: Row(
                    children: [
                      const Expanded(
                        child: Padding(
                          padding: EdgeInsets.only(left: 14),
                          child: Text(
                            '工作区',
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: TextStyle(
                              fontSize: 14,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                      IconButton(
                        tooltip: '关闭工作区侧栏',
                        onPressed: controller.close,
                        icon: const Icon(Icons.close, size: 20),
                      ),
                    ],
                  ),
                ),
                const Divider(height: 1),
                Expanded(
                  child: ClipRect(
                    child: MobileExtensionSlot(
                      slotId: 'provider.app.workspace.sidebar',
                      context: sidebarContext,
                      actions: sidebarActions,
                      fallback: const SizedBox.shrink(),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}
