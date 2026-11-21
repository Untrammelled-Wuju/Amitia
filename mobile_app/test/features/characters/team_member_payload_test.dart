import 'package:flutter_test/flutter_test.dart';
import 'package:amitia_app/features/characters/presentation/team_member_payload.dart';

void main() {
  test('统筹中心角色不带设备归属', () {
    final payload = buildTeamMemberPayload(
      source: 'center',
      deviceId: '',
      roleId: 'role-a',
    );
    expect(payload['memberType'], 'agent');
    expect(payload['memberId'], 'role-a');
    expect(payload['responderMode'], 'mention_or_lead');
    expect(payload.containsKey('ownerType'), isFalse);
    expect(payload.containsKey('ownerId'), isFalse);
    expect(payload.containsKey('roleRevision'), isFalse);
  });

  test('设备角色携带归属、调用模式与角色版本', () {
    final payload = buildTeamMemberPayload(
      source: 'device',
      deviceId: 'device-b',
      roleId: 'role-b',
      role: 'lead',
      roleRevision: 5,
    );
    expect(payload['ownerType'], 'device');
    expect(payload['ownerId'], 'device-b');
    expect(payload['invocationMode'], 'role');
    expect(payload['roleRevision'], 5);
    expect(payload['capabilities'], ['*']);
    expect(payload['responderMode'], 'lead');
  });

  test('缺少角色版本时不下发版本字段', () {
    final payload = buildTeamMemberPayload(
      source: 'device',
      deviceId: 'device-b',
      roleId: 'role-b',
    );
    expect(payload.containsKey('roleRevision'), isFalse);
    expect(payload['ownerId'], 'device-b');
  });
}
