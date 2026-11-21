Map<String, dynamic> buildTeamMemberPayload({
  required String source,
  required String deviceId,
  required String roleId,
  String role = 'member',
  int roleRevision = 0,
}) {
  final payload = <String, dynamic>{
    'memberType': 'agent',
    'memberId': roleId,
    'role': role,
    'responderMode': role == 'lead' ? 'lead' : 'mention_or_lead',
    'capabilities': role == 'lead' ? <String>['*'] : <String>[],
  };
  if (source == 'device') {
    payload['ownerType'] = 'device';
    payload['ownerId'] = deviceId;
    payload['invocationMode'] = 'role';
    if (roleRevision > 0) payload['roleRevision'] = roleRevision;
  }
  return payload;
}
