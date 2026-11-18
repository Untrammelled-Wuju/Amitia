import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'mobile_extension_slot.dart';
import 'mobile_ui_host_actions.dart';
import 'ui_provider.dart';
import 'ui_runtime_controller.dart';

class MobileUIPortalHost extends ConsumerWidget {
  const MobileUIPortalHost({
    super.key,
    this.context = const {},
    this.actions = const {},
  });

  final Map<String, dynamic> context;
  final Map<String, FutureOr<dynamic> Function(dynamic input)> actions;

  bool _hasPortal(UIProviderSnapshot? snapshot, String slotId) {
    if (snapshot == null) return false;
    return snapshot.contributionsForSlot(slotId).isNotEmpty;
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final snapshot = ref.watch(effectiveUIRuntimeSnapshotProvider);
    final hasOverlay = _hasPortal(snapshot, 'ui.portal.overlay');
    final hasModal = _hasPortal(snapshot, 'ui.portal.modal');
    final hasDrawer = _hasPortal(snapshot, 'ui.portal.drawer');
    final hasToast = _hasPortal(snapshot, 'ui.portal.toast');
    if (!hasOverlay && !hasModal && !hasDrawer && !hasToast) {
      return const SizedBox.shrink();
    }
    final portalContext = <String, dynamic>{
      ...this.context,
      'surface': 'portal',
      'surfaceRole': 'overlay',
    };
    final portalActions = buildMobileUIHostActions(
      context: context,
      ref: ref,
      actions: actions,
    );
    return Stack(
      children: [
        if (hasOverlay)
          Positioned.fill(
            child: MobileExtensionSlot(
              slotId: 'ui.portal.overlay',
              context: {...portalContext, 'slotLayout': 'stack'},
              actions: portalActions,
              fallback: const SizedBox.shrink(),
            ),
          ),
        if (hasModal)
          Positioned.fill(
            child: MobileExtensionSlot(
              slotId: 'ui.portal.modal',
              context: {...portalContext, 'slotLayout': 'stack'},
              actions: portalActions,
              fallback: const SizedBox.shrink(),
            ),
          ),
        if (hasDrawer)
          Positioned.fill(
            child: MobileExtensionSlot(
              slotId: 'ui.portal.drawer',
              context: {...portalContext, 'slotLayout': 'stack'},
              actions: portalActions,
              fallback: const SizedBox.shrink(),
            ),
          ),
        if (hasToast)
          Positioned(
            left: 16,
            right: 16,
            bottom: MediaQuery.paddingOf(context).bottom + 24,
            child: IgnorePointer(
              child: MobileExtensionSlot(
                slotId: 'ui.portal.toast',
                context: {...portalContext, 'slotLayout': 'stack'},
                actions: portalActions,
                fallback: const SizedBox.shrink(),
              ),
            ),
          ),
      ],
    );
  }
}
