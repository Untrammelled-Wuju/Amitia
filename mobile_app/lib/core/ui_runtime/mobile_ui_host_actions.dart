import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'mobile_right_pane_runtime.dart';
import 'mobile_workspace_sidebar_runtime.dart';

Map<String, FutureOr<dynamic> Function(dynamic)> buildMobileUIHostActions({
  required BuildContext context,
  required WidgetRef ref,
  Map<String, FutureOr<dynamic> Function(dynamic)> actions = const {},
}) {
  final controller = ref.read(mobileRightPaneProvider.notifier);
  final workspaceSidebar = ref.read(mobileWorkspaceSidebarProvider.notifier);
  return <String, FutureOr<dynamic> Function(dynamic)>{
    'openRightPane': (input) {
      final data = input is Map
          ? input.cast<String, dynamic>()
          : <String, dynamic>{'address': input};
      final address = (data['address'] ?? data['uri'] ?? '').toString().trim();
      controller.open(
        address,
        title: data['title']?.toString(),
        params:
            (data['params'] as Map?)?.cast<String, dynamic>() ??
            const <String, dynamic>{},
        sessionId: data['sessionId']?.toString(),
      );
      return <String, dynamic>{'open': true, 'address': address};
    },
    'closeRightPane': (_) {
      controller.close();
      return <String, dynamic>{'open': false};
    },
    'openWorkspaceSidebar': (_) {
      workspaceSidebar.open();
      return <String, dynamic>{'open': true};
    },
    'closeWorkspaceSidebar': (_) {
      workspaceSidebar.close();
      return <String, dynamic>{'open': false};
    },
    'navigate': (input) {
      final data = input is Map
          ? input.cast<String, dynamic>()
          : <String, dynamic>{'target': input};
      final target = (data['target'] ?? data['route'] ?? '').toString().trim();
      if (target.isEmpty ||
          !target.startsWith('/') ||
          target.startsWith('//') ||
          target.contains('\\') ||
          target.contains('\u0000')) {
        throw StateError('mobile navigation target is invalid: $target');
      }
      GoRouter.of(context).go(target);
      return <String, dynamic>{'ok': true, 'target': target};
    },
    'showToast': (input) {
      final data = input is Map
          ? input.cast<String, dynamic>()
          : <String, dynamic>{'message': input};
      final message = (data['message'] ?? data['text'] ?? '').toString().trim();
      if (message.isEmpty) {
        throw StateError('mobile toast message is required');
      }
      final messenger = ScaffoldMessenger.maybeOf(context);
      messenger
        ?..hideCurrentSnackBar()
        ..showSnackBar(SnackBar(content: Text(message)));
      return <String, dynamic>{'ok': true};
    },
    ...actions,
  };
}
