import '../backend_transport/backend_service_api.dart';

class SpaceProfile {
  final String spaceId;
  final String instanceId;
  final String displayName;
  final String avatar;
  final String bio;
  final String userLabel;
  final Map<String, dynamic> preferences;

  const SpaceProfile({
    required this.spaceId,
    required this.instanceId,
    required this.displayName,
    this.avatar = '',
    this.bio = '',
    this.userLabel = '',
    this.preferences = const <String, dynamic>{},
  });

  factory SpaceProfile.fromSpaceResponse(Map<String, dynamic> json) {
    final identityRaw = json['identity'];
    final profileRaw = json['profile'];
    final identity = identityRaw is Map
        ? Map<String, dynamic>.from(identityRaw)
        : const <String, dynamic>{};
    final profile = profileRaw is Map
        ? Map<String, dynamic>.from(profileRaw)
        : const <String, dynamic>{};
    final preferencesRaw = profile['preferences'];
    return SpaceProfile(
      spaceId: (identity['spaceId'] ?? json['spaceId'] ?? '').toString(),
      instanceId: (identity['instanceId'] ?? json['instanceId'] ?? '').toString(),
      displayName: (profile['displayName'] ?? '').toString(),
      avatar: (profile['avatar'] ?? '').toString(),
      bio: (profile['bio'] ?? '').toString(),
      userLabel: (profile['userLabel'] ?? '').toString(),
      preferences: preferencesRaw is Map
          ? Map<String, dynamic>.from(preferencesRaw)
          : const <String, dynamic>{},
    );
  }
}

class SpaceProfileService {
  final BackendServiceApi _api;

  SpaceProfileService(this._api);

  Future<SpaceProfile> fetch() async {
    final response = await _api.get<Map<String, dynamic>>('/api/space');
    if (response == null) {
      throw ServiceApiException(code: 10000, message: 'Space 信息响应为空');
    }
    return SpaceProfile.fromSpaceResponse(response);
  }

  Future<SpaceProfile> update({
    required String displayName,
    required String userLabel,
    required String bio,
    String avatar = '',
    Map<String, dynamic>? preferences,
  }) async {
    await _api.put<Map<String, dynamic>>(
      '/api/space/profile',
      data: <String, dynamic>{
        'displayName': displayName.trim(),
        'avatar': avatar.trim(),
        'userLabel': userLabel.trim(),
        'bio': bio.trim(),
        'preferences': preferences ?? const <String, dynamic>{},
      },
    );
    return fetch();
  }
}
