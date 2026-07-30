import '../models/models.dart';

class MockChannels {
  MockChannels._();

  static ChannelConnection wechat = ChannelConnection(
    type: ChannelType.wechat,
    status: ChannelStatus.connected,
    account: '微信用户_2026',
    lastHeartbeat: '2 秒前',
    receivingMessages: true,
    sendingMessages: true,
    logs: [
      '[2026-07-30 09:15] 登录成功',
      '[2026-07-30 09:16] 开始接收消息',
      '[2026-07-30 09:20] 收到 3 条消息',
      '[2026-07-30 09:25] 发送 1 条消息',
    ],
  );

  static ChannelConnection qq = ChannelConnection(
    type: ChannelType.qq,
    status: ChannelStatus.disconnected,
    botAppId: '1024xxxxx',
    botToken: '****',
    wsStatus: '未连接',
    logs: [
      '[2026-07-30 08:00] 尝试连接',
      '[2026-07-30 08:01] WebSocket 连接失败',
      '[2026-07-30 08:01] Token 验证失败',
    ],
    errorMessage: 'Token 验证失败，请检查 Bot 配置',
  );
}

class MockDashboard {
  MockDashboard._();

  static DashboardRunInfo runInfo = DashboardRunInfo(
    recentErrors: ['MCP Runtime 已停止', 'QQ 渠道连接失败'],
    recentTasks: ['整理下载目录 (进行中)', '生成周报摘要 (已完成)'],
  );

  static DashboardDataInfo dataInfo = DashboardDataInfo(
    recentImports: ['微信聊天记录 (156条)', 'QQ 群聊记录 (89条)'],
  );
}
