import '../backend_transport/backend_service_api.dart';
import 'device_owned_realtime_service.dart';

class OwnedRealtimeAcceptedInvitation {
  final DeviceOwnedRealtimeService service;
  final Map<String, dynamic> ticket;

  OwnedRealtimeAcceptedInvitation(this.service, this.ticket);
}

Future<OwnedRealtimeAcceptedInvitation> acceptOwnedRealtimeInvitation({
  required BackendServiceApi api,
  required bool Function() isCurrent,
  required String callId,
  required String characterId,
}) async {
  if (!RegExp(
        r'^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$',
      ).hasMatch(callId) ||
      characterId.isEmpty) {
    throw StateError('通话邀请编号或角色无效');
  }
  Map<String, dynamic> data(Map<String, dynamic>? response) {
    if (response == null) throw StateError('Core 未返回邀请数据');
    return response['data'] is Map
        ? Map<String, dynamic>.from(response['data'] as Map)
        : response;
  }

  void current() {
    if (!isCurrent()) throw StateError('Core 连接已切换，邀请已失效');
  }

  current();
  final path = '/api/device-mesh/v1/business/realtime/invitations/$callId';
  final original = data(
    await api.get<Map<String, dynamic>>(
      path,
      queryParameters: {'characterId': characterId},
    ),
  );
  current();
  if (original['invitation'] is! Map) throw StateError('邀请记录无效');
  final invitation = Map<String, dynamic>.from(original['invitation'] as Map);
  if (invitation['executionScope'] is! Map) throw StateError('邀请缺少原授权');
  final scope = Map<String, dynamic>.from(invitation['executionScope'] as Map);
  ownedRealtimeAuthority(scope);
  final expires = DateTime.tryParse('${invitation['expiresAt']}');
  if (invitation['id'] != callId ||
      invitation['characterId'] != characterId ||
      scope['roleId'] != characterId ||
      invitation['recipientDeviceId'] != scope['initiatorDeviceId'] ||
      invitation['status'] != 'pending' ||
      invitation['revision'] != 1 ||
      expires == null ||
      !expires.isAfter(DateTime.now()) ||
      invitation['nonce'] is! String ||
      !RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(invitation['nonce'] as String) ||
      invitation['conversationId'] is! String ||
      (invitation['conversationId'] as String).isEmpty) {
    throw StateError('邀请已过期、已处理或不属于当前设备角色');
  }
  final requestId = ownedRealtimeRequestId();
  final result = data(
    await api.post<Map<String, dynamic>>(
      '$path/accept',
      data: {
        'requestId': requestId,
        'characterId': characterId,
        'expectedExecutionScope': scope,
        'expectedRevision': 1,
        'nonce': invitation['nonce'],
      },
    ),
  );
  current();
  final ack = result['acknowledgement'];
  final accepted = result['invitation'];
  final ticket = result['ticket'];
  if (result['saved'] != true ||
      ack is! Map ||
      ack['ownerId'] != scope['resourceOwnerId'] ||
      ack['requestId'] != requestId ||
      ack['versions'] is! Map ||
      (ack['versions'] as Map).length != 1 ||
      (ack['versions'] as Map)['checkpoint/realtime-invitation/$callId'] != 2 ||
      accepted is! Map ||
      accepted['id'] != callId ||
      accepted['status'] != 'accepted' ||
      accepted['revision'] != 2 ||
      accepted['executionScope'] is! Map ||
      ownedRealtimeAuthority(accepted['executionScope'] as Map) !=
          ownedRealtimeAuthority(scope) ||
      ticket is! Map ||
      ticket['executionScope'] is! Map ||
      ownedRealtimeAuthority(ticket['executionScope'] as Map) !=
          ownedRealtimeAuthority(scope) ||
      ticket['wsPath'] != '/api/device-mesh/v1/business/realtime/session' ||
      ticket['ticket'] is! String ||
      !RegExp(r'^[A-Za-z0-9_-]{43}$').hasMatch(ticket['ticket'] as String)) {
    throw StateError('接听尚未获得原所有者确认或通话票据无效');
  }
  final service = DeviceOwnedRealtimeService(
    api: api,
    isCurrent: isCurrent,
    scope: scope,
    characterId: characterId,
    conversationId: invitation['conversationId'] as String,
    conversationOrigin: invitation['conversationOrigin'] is Map
        ? Map<String, String>.from(invitation['conversationOrigin'] as Map)
        : null,
    historicalRoleId: '${invitation['historicalRoleId'] ?? ''}',
  );
  return OwnedRealtimeAcceptedInvitation(
    service,
    Map<String, dynamic>.unmodifiable(ticket),
  );
}
