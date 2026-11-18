import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'mobile_extension_slot.dart';
import 'mobile_right_pane_runtime.dart';
import 'mobile_ui_host_actions.dart';

class MobileUIRightPaneHost extends ConsumerWidget {
  const MobileUIRightPaneHost({
    super.key,
    this.context = const {},
    this.actions = const {},
  });

  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final pane = ref.watch(mobileRightPaneProvider);
    if (!pane.open) return const SizedBox.shrink();
    final controller = ref.read(mobileRightPaneProvider.notifier);
    final paneActions = buildMobileUIHostActions(
      context: context,
      ref: ref,
      actions: actions,
    );
    final width = MediaQuery.sizeOf(context).width.clamp(280.0, 460.0) * 0.9;
    final paneContext = <String, dynamic>{
      ...this.context,
      ...pane.toContext(),
      'surface': 'right-pane',
      'surfaceRole': 'sidebar',
    };
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
          right: 8,
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
                      Expanded(
                        child: Padding(
                          padding: const EdgeInsets.only(left: 14),
                          child: MobileExtensionSlot(
                            slotId: 'ui.sidebar.right.pane.tab.title',
                            context: paneContext,
                            actions: paneActions,
                            fallback: Text(
                              pane.title ?? '资源',
                              maxLines: 1,
                              overflow: TextOverflow.ellipsis,
                              style: const TextStyle(
                                fontSize: 14,
                                fontWeight: FontWeight.w600,
                              ),
                            ),
                          ),
                        ),
                      ),
                      IconButton(
                        tooltip: '关闭右侧面板',
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
                      slotId: 'ui.sidebar.right.pane.tab',
                      context: paneContext,
                      actions: paneActions,
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
