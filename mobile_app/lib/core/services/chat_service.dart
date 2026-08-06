import '../api/api_client.dart';
import '../models/conversation.dart';

class ChatService {
  final ApiClient _api = ApiClient();

  Future<List<ConversationDto>> listConversations() async {
    final resp = await _api.get<List<dynamic>>('/api/chats/conversations');
    if (resp.data == null) return [];
    return resp.data!
        .map((e) => ConversationDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<ConversationDto?> createConversation(String? characterId) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chats/conversations',
      data: characterId != null ? {'characterId': characterId} : null,
    );
    if (resp.data == null) return null;
    return ConversationDto.fromJson(resp.data!);
  }

  Future<List<MessageDto>> getMessages(String conversationId) async {
    final resp = await _api.get<List<dynamic>>(
      '/api/chats/conversations/$conversationId/messages',
    );
    if (resp.data == null) return [];
    return resp.data!
        .map((e) => MessageDto.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<bool> deleteConversation(String id) async {
    await _api.delete('/api/chats/conversations/$id');
    return true;
  }

  Future<Map<String, dynamic>?> chat(String message, {String? conversationId}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/chat',
      data: {
        'message': message,
        if (conversationId != null) 'conversationId': conversationId,
      },
    );
    return resp.data;
  }
}
