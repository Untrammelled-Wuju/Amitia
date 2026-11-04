import '../../../core/models/conversation.dart';

List<Map<String, dynamic>> projectCitationSnapshot(
  List<Map<String, dynamic>> persisted,
  AssistantTurnItemDto? finalText,
) {
  if (finalText != null && finalText.citationSnapshotProvided) {
    return finalText.citationSnapshot;
  }
  return persisted;
}
