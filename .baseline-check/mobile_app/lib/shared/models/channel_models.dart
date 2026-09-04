enum ChannelType { wechat, qq }
enum ChannelStatus { connected, disconnected, connecting, expired }

class ChannelConnection {
  final ChannelType type;
  final ChannelStatus status;
  final String? account;
  final String? lastHeartbeat;
  final bool receivingMessages;
  final bool sendingMessages;
  final String? qrCodePlaceholder;
  final String? botAppId;
  final String? botToken;
  final String? wsStatus;
  final List<String> logs;
  final String? errorMessage;

  ChannelConnection({
    required this.type,
    required this.status,
    this.account,
    this.lastHeartbeat,
    this.receivingMessages = false,
    this.sendingMessages = false,
    this.qrCodePlaceholder,
    this.botAppId,
    this.botToken,
    this.wsStatus,
    this.logs = const [],
    this.errorMessage,
  });
}
