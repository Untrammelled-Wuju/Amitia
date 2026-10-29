import '../backend_transport/backend_service_api.dart';

class ChannelSandboxPolicy {
  final String channelId;
  final bool enabled;
  final bool allowNetwork;
  final String workspaceId;
  final String sandboxRoot;
  final int revision;

  const ChannelSandboxPolicy({
    required this.channelId,
    required this.enabled,
    required this.allowNetwork,
    required this.workspaceId,
    required this.sandboxRoot,
    required this.revision,
  });

  factory ChannelSandboxPolicy.fromJson(Map<String, dynamic> json) {
    return ChannelSandboxPolicy(
      channelId: (json['channelId'] ?? '').toString(),
      enabled: json['enabled'] == true,
      allowNetwork: json['allowNetwork'] != false,
      workspaceId: (json['workspaceId'] ?? '').toString(),
      sandboxRoot: (json['sandboxRoot'] ?? '').toString(),
      revision: (json['revision'] as num?)?.toInt() ?? 0,
    );
  }
}

class ChannelSandboxService {
  final BackendServiceApi _api;

  const ChannelSandboxService(this._api);

  Future<ChannelSandboxPolicy> getPolicy(String channelId) async {
    final result =
        await _api.get<Map<String, dynamic>>(
          '/api/channels/sandbox-policies',
          queryParameters: <String, dynamic>{'channelId': channelId.trim()},
        ) ??
        <String, dynamic>{};
    return ChannelSandboxPolicy.fromJson(result);
  }

  Future<ChannelSandboxPolicy> savePolicy({
    required String channelId,
    required bool enabled,
    required bool allowNetwork,
  }) async {
    final result =
        await _api.put<Map<String, dynamic>>(
          '/api/channels/sandbox-policies',
          data: <String, dynamic>{
            'channelId': channelId.trim(),
            'enabled': enabled,
            'allowNetwork': allowNetwork,
          },
        ) ??
        <String, dynamic>{};
    return ChannelSandboxPolicy.fromJson(result);
  }
}
