import '../api/api_client.dart';
import '../models/voice.dart';

class TTSService {
  final ApiClient _api = ApiClient();

  Future<List<VoiceConfigDto>> listConfigs() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/configs');
    if (resp.data == null) return [];
    return resp.data!.map((e) => VoiceConfigDto.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<VoiceConfigDto?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tts/configs', data: data);
    if (resp.data == null) return null;
    return VoiceConfigDto.fromJson(resp.data!);
  }

  Future<VoiceConfigDto?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/tts/configs/$id', data: data);
    if (resp.data == null) return null;
    return VoiceConfigDto.fromJson(resp.data!);
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/tts/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/tts/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tts/configs/$id/test');
    return resp.data;
  }

  Future<Map<String, dynamic>?> testConnection(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/tts/test-connection', data: data);
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/providers');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> voices() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/voices');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<List<Map<String, dynamic>>> emotions() async {
    final resp = await _api.get<List<dynamic>>('/api/tts/emotions');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> synthesize(String text, {String? voiceId}) async {
    final resp = await _api.post<Map<String, dynamic>>(
      '/api/tts/synthesize',
      data: {
        'text': text,
        if (voiceId != null) 'voiceId': voiceId,
      },
    );
    return resp.data;
  }
}

class ASRService {
  final ApiClient _api = ApiClient();

  Future<List<Map<String, dynamic>>> configs() async {
    final resp = await _api.get<List<dynamic>>('/api/asr/configs');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> createConfig(Map<String, dynamic> data) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/configs', data: data);
    return resp.data;
  }

  Future<Map<String, dynamic>?> updateConfig(String id, Map<String, dynamic> data) async {
    final resp = await _api.put<Map<String, dynamic>>('/api/asr/configs/$id', data: data);
    return resp.data;
  }

  Future<bool> deleteConfig(String id) async {
    await _api.delete('/api/asr/configs/$id');
    return true;
  }

  Future<bool> activate(String id) async {
    await _api.post('/api/asr/configs/$id/activate');
    return true;
  }

  Future<Map<String, dynamic>?> test(String id) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/configs/$id/test');
    return resp.data;
  }

  Future<List<Map<String, dynamic>>> providers() async {
    final resp = await _api.get<List<dynamic>>('/api/asr/providers');
    if (resp.data == null) return [];
    return resp.data!.map((e) => e as Map<String, dynamic>).toList();
  }

  Future<Map<String, dynamic>?> uploadAudio(String filePath) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/upload', data: {'filePath': filePath});
    return resp.data;
  }

  Future<Map<String, dynamic>?> submitUrl(String url) async {
    final resp = await _api.post<Map<String, dynamic>>('/api/asr/submit', data: {'url': url});
    return resp.data;
  }

  Future<Map<String, dynamic>?> queryResult(String taskId) async {
    final resp = await _api.get<Map<String, dynamic>>('/api/asr/query', queryParameters: {'taskId': taskId});
    return resp.data;
  }
}
