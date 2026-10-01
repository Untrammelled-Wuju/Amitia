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
