import '../../../shared/models/models.dart';

bool shouldShowRoleSwitch(ChatMessage? previous, ChatMessage current) {
  final previousCharacterId = previous?.characterId.trim() ?? '';
  final currentCharacterId = current.characterId.trim();
  return previousCharacterId.isNotEmpty &&
      currentCharacterId.isNotEmpty &&
      previousCharacterId != currentCharacterId;
}

bool shouldShowAssistantIdentity(ChatMessage current, ChatMessage? previous) {
  if (current.role != MessageRole.assistant) return true;
  if (previous?.role != MessageRole.assistant) return true;
  final currentSenderId = current.characterId.trim();
  final previousSenderId = previous!.characterId.trim();
  if (currentSenderId.isNotEmpty && previousSenderId.isNotEmpty) {
    return currentSenderId != previousSenderId;
  }
  final currentGroupId = current.responseGroupId.trim();
  final previousGroupId = previous.responseGroupId.trim();
  if (currentGroupId.isNotEmpty && previousGroupId.isNotEmpty) {
    return currentGroupId != previousGroupId;
  }
  return true;
}

bool shouldCompactAfterAssistantMessage(
  ChatMessage current,
  ChatMessage? next,
) {
  if (current.role != MessageRole.assistant ||
      next?.role != MessageRole.assistant) {
    return false;
  }
  return !shouldShowAssistantIdentity(next!, current);
}
