bool isDevicePresenceOnline(Object? raw) {
  final state = raw?.toString().trim().toLowerCase() ?? '';
  return state == 'online' ||
      state == 'connected' ||
      state == 'ready' ||
      state == 'degraded';
}

bool isDeviceMeshConnectedState(String raw) => isDevicePresenceOnline(raw);

bool isDeviceMeshTrusted(Object? raw) =>
    raw?.toString().trim().toLowerCase() == 'trusted';

String deviceSyncBlockedLabel(Object? raw) {
  final state = raw?.toString().trim().toLowerCase() ?? '';
  if (state == 'trusted') return '';
  if (state == 'revoked') return '已撤销，不再同步';
  return '未受信任，暂不可同步';
}
