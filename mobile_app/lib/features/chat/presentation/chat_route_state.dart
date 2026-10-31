import 'package:flutter/widgets.dart';
import 'package:go_router/go_router.dart';

import '../../../app/app_routes.dart';

void replaceChatRouteWithConversation(
  BuildContext context,
  String conversationId,
) {
  final id = conversationId.trim();
  if (id.isEmpty) return;
  context.replace(
    '${AppRoutes.chat}?conversationId=${Uri.encodeQueryComponent(id)}',
  );
}

bool shouldApplyConversationRoute({
  required String previousConversationId,
  required String nextConversationId,
  required String previousProjectId,
  required String nextProjectId,
  required String runtimeConversationId,
  required String runtimeProjectId,
  required String routeSyncedConversationId,
}) {
  if (nextConversationId.isNotEmpty &&
      nextConversationId == routeSyncedConversationId &&
      nextConversationId == runtimeConversationId &&
      nextProjectId == runtimeProjectId) {
    return false;
  }
  if (previousConversationId != nextConversationId ||
      previousProjectId != nextProjectId) {
    return true;
  }
  if (nextConversationId.isEmpty) {
    return runtimeConversationId.isNotEmpty ||
        runtimeProjectId != nextProjectId;
  }
  return runtimeConversationId != nextConversationId;
}
