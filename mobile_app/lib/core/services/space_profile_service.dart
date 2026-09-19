import 'package:shared_preferences/shared_preferences.dart';

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
      instanceId: (identity['instanceId'] ?? json['instanceId'] ?? '')
          .toString(),
      displayName: (profile['displayName'] ?? '').toString(),
      avatar: (profile['avatar'] ?? '').toString(),
      bio: (profile['bio'] ?? '').toString(),
      userLabel: (profile['userLabel'] ?? '').toString(),
      preferences: preferencesRaw is Map
          ? Map<String, dynamic>.from(preferencesRaw)
          : const <String, dynamic>{},
    );
  }

  SpaceProfile copyWith({
    String? spaceId,
    String? instanceId,
    String? displayName,
    String? avatar,
    String? bio,
    String? userLabel,
    Map<String, dynamic>? preferences,
  }) {
    return SpaceProfile(
      spaceId: spaceId ?? this.spaceId,
      instanceId: instanceId ?? this.instanceId,
      displayName: displayName ?? this.displayName,
      avatar: avatar ?? this.avatar,
      bio: bio ?? this.bio,
      userLabel: userLabel ?? this.userLabel,
      preferences: preferences ?? this.preferences,
    );
  }
}

class SpaceAvatarCache {
  static const String _key = 'space.profile.avatar.v1';

  Future<String> read() async {
    final preferences = await SharedPreferences.getInstance();
    return (preferences.getString(_key) ?? '').trim();
  }

  Future<void> write(String avatar) async {
    final preferences = await SharedPreferences.getInstance();
    final normalized = avatar.trim();
    if (normalized.isEmpty) {
      await preferences.remove(_key);
      return;
    }
    await preferences.setString(_key, normalized);
  }

  Future<void> clear() => write('');
}

class SpaceProfileService {
  final BackendServiceApi _api;
  final SpaceAvatarCache _avatarCache;

  SpaceProfileService(this._api, {SpaceAvatarCache? avatarCache})
    : _avatarCache = avatarCache ?? SpaceAvatarCache();

  Future<SpaceProfile> fetch() async {
    try {
      final response = await _api.get<Map<String, dynamic>>('/api/space');
      if (response == null) {
        throw ServiceApiException(code: 10000, message: 'Space 信息响应为空');
      }
      var profile = SpaceProfile.fromSpaceResponse(response);
      final avatar = profile.avatar.trim();
      if (avatar.isNotEmpty) {
        await _avatarCache.write(avatar);
      } else {
        final cachedAvatar = await _avatarCache.read();
        if (cachedAvatar.isNotEmpty) {
          profile = profile.copyWith(avatar: cachedAvatar);
        }
      }
      return profile;
    } catch (_) {
      final cachedAvatar = await _avatarCache.read();
      if (cachedAvatar.isNotEmpty) {
        return SpaceProfile(
          spaceId: '',
          instanceId: '',
          displayName: '',
          avatar: cachedAvatar,
        );
      }
      rethrow;
    }
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
    final updated = await fetch();
    await _avatarCache.write(avatar);
    return updated.copyWith(avatar: avatar.trim());
  }

  Future<SpaceProfile> updateAvatar(String avatar) async {
    final current = await fetch();
    return update(
      displayName: current.displayName,
      userLabel: current.userLabel,
      bio: current.bio,
      avatar: avatar,
      preferences: current.preferences,
    );
  }
}
