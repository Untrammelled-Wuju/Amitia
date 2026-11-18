bool isCompactMobileWebRuntimeContext(Map<String, dynamic> context) {
  final role = (context['surfaceRole'] ?? '').toString().trim().toLowerCase();
  if (const <String>{'header', 'status', 'composer'}.contains(role)) {
    return true;
  }
  final layout = (context['slotLayout'] ?? '').toString().trim().toLowerCase();
  if (const <String>{'inline', 'row'}.contains(layout)) return true;
  final kind = (context['slotKind'] ?? '').toString().trim().toLowerCase();
  if (const <String>{'badge', 'status_item', 'toolbar_item'}.contains(kind)) {
    return true;
  }
  final slotId = (context['slotId'] ?? '').toString().trim().toLowerCase();
  return slotId.endsWith('.badge') ||
      slotId.endsWith('.title') ||
      slotId == 'chat.status.item' ||
      slotId == 'system.status.item';
}

double mobileWebRuntimeInitialHeight(Map<String, dynamic> context) {
  final role = (context['surfaceRole'] ?? '').toString().trim().toLowerCase();
  if (const <String>{'header', 'composer'}.contains(role)) return 56;
  if (role == 'status') return 40;
  if (isCompactMobileWebRuntimeContext(context)) return 1;
  return 320;
}
