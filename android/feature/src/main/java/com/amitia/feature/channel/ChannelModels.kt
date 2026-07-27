package com.amitia.feature.channel

import com.amitia.core.designsystem.component.AmitiaStatusType

enum class ChannelType(val label: String, val key: String) {
    Web("Web", "web"),
    WeChat("微信", "wechat"),
    QQ("QQ", "qq"),
    Api("API", "api"),
    ThirdParty("第三方", "third_party")
}

data class ChannelSummary(
    val id: String,
    val name: String,
    val type: ChannelType,
    val bound: Boolean,
    val online: Boolean,
    val lastActivity: String?,
    val error: String?,
    val accountCount: Int,
    val isSystemPlugin: Boolean,
    val installedThirdParty: Boolean = false
) {
    val statusType: AmitiaStatusType
        get() = when {
            !bound -> AmitiaStatusType.Idle
            error != null -> AmitiaStatusType.Failed
            online -> AmitiaStatusType.Connected
            else -> AmitiaStatusType.Pending
        }
}

data class ChannelHomeData(
    val systemChannels: List<ChannelSummary> = emptyList(),
    val publicChannels: List<ChannelSummary> = emptyList(),
    val totalBound: Int = 0,
    val totalActive: Int = 0
)

data class WebChannelConfig(
    val enabled: Boolean = true,
    val currentSession: String,
    val messageSync: Boolean,
    val mergeRule: String,
    val notifications: Boolean
)

data class WeChatChannelDetail(
    val id: String,
    val bound: Boolean,
    val qrCode: String?,
    val online: Boolean,
    val lastHeartbeat: String?,
    val lastSend: String?,
    val lastReceive: String?,
    val assignedRole: String?,
    val messageLinkAbnormal: Boolean,
    val abnormalReason: String?
)

data class QQChannelDetail(
    val id: String,
    val bound: Boolean,
    val qrCode: String?,
    val online: Boolean,
    val protocol: String,
    val lastHeartbeat: String?,
    val lastSend: String?,
    val lastReceive: String?,
    val assignedRole: String?,
    val messageLinkAbnormal: Boolean,
    val abnormalReason: String?
)

data class ApiChannelConfig(
    val id: String,
    val name: String,
    val baseUrl: String,
    val apiKey: String,
    val enabled: Boolean,
    val rateLimit: Int,
    val webhookUrl: String
)

data class ChannelCreateStep(
    val index: Int,
    val title: String,
    val description: String
)

data class ChannelEditForm(
    val name: String = "",
    val type: ChannelType = ChannelType.Api,
    val baseUrl: String = "",
    val apiKey: String = "",
    val enabled: Boolean = true,
    val retryPolicy: String = "指数退避"
)

data class ChannelBindState(
    val scanning: Boolean,
    val countdownSeconds: Int,
    val scanned: Boolean,
    val success: Boolean?,
    val failReason: String?
)

data class ChannelDiagnosticItem(
    val key: String,
    val name: String,
    val passed: Boolean,
    val detail: String
)

data class ChannelDiagnosticData(
    val channelName: String,
    val items: List<ChannelDiagnosticItem>,
    val rawText: String
)

data class ChannelNotificationSettings(
    val newMessage: Boolean,
    val failureReminder: Boolean,
    val offlineReminder: Boolean,
    val duplicateProtection: Boolean,
    val roleProactive: Boolean
)

object ChannelMockData {
    val home = ChannelHomeData(
        systemChannels = listOf(
            ChannelSummary("c1", "Web 聊天", ChannelType.Web, true, true, "刚刚", null, 3, true),
            ChannelSummary("c2", "微信", ChannelType.WeChat, true, true, "2 分钟前", null, 1, true),
            ChannelSummary("c3", "QQ", ChannelType.QQ, true, false, "1 小时前", "心跳超时", 1, true),
            ChannelSummary("c4", "API 接入", ChannelType.Api, true, true, "5 分钟前", null, 2, true)
        ),
        publicChannels = listOf(
            ChannelSummary("c5", "Telegram", ChannelType.ThirdParty, false, false, null, null, 0, false, installedThirdParty = true),
            ChannelSummary("c6", "Discord", ChannelType.ThirdParty, true, true, "10 分钟前", null, 1, false, installedThirdParty = true)
        ),
        totalBound = 4,
        totalActive = 3
    )

    val webConfig = WebChannelConfig(
        enabled = true,
        currentSession = "amitia-session-7f3a",
        messageSync = true,
        mergeRule = "60 秒内连续消息合并",
        notifications = true
    )

    val weChatDetail = WeChatChannelDetail(
        id = "c2",
        bound = true,
        qrCode = "https://amitia.example/bind/wechat/qr",
        online = true,
        lastHeartbeat = "刚刚",
        lastSend = "2 分钟前",
        lastReceive = "1 分钟前",
        assignedRole = "艾米",
        messageLinkAbnormal = false,
        abnormalReason = null
    )

    val qqDetail = QQChannelDetail(
        id = "c3",
        bound = true,
        qrCode = "https://amitia.example/bind/qq/qr",
        online = false,
        protocol = "NTQQ",
        lastHeartbeat = "1 小时前",
        lastSend = "1 小时前",
        lastReceive = "1 小时前",
        assignedRole = "艾米",
        messageLinkAbnormal = true,
        abnormalReason = "心跳超时，消息链路异常"
    )

    val apiConfig = ApiChannelConfig(
        id = "c4",
        name = "API 接入",
        baseUrl = "https://api.amitia.example/v1",
        apiKey = "amitia_sk_****a2f9",
        enabled = true,
        rateLimit = 60,
        webhookUrl = "https://amitia.example/webhook/api"
    )

    val createSteps = listOf(
        ChannelCreateStep(0, "选择渠道类型", "从 Web、微信、QQ、API 或第三方中选择"),
        ChannelCreateStep(1, "填写渠道信息", "名称、端点、凭据等基础信息"),
        ChannelCreateStep(2, "绑定与授权", "扫码或填写授权凭据完成绑定"),
        ChannelCreateStep(3, "配置与确认", "设置默认角色与通知策略")
    )

    val diagnostics = ChannelDiagnosticData(
        channelName = "微信",
        items = listOf(
            ChannelDiagnosticItem("bind", "绑定凭据", true, "凭据有效，绑定于 2026-07-20"),
            ChannelDiagnosticItem("heartbeat", "心跳", true, "最近心跳 2 秒前"),
            ChannelDiagnosticItem("recv", "收消息", true, "最近接收 1 分钟前"),
            ChannelDiagnosticItem("send", "发消息", true, "最近发送 2 分钟前"),
            ChannelDiagnosticItem("dedup", "去重", true, "幂等键校验正常"),
            ChannelDiagnosticItem("websync", "Web 同步", false, "Web 端最近同步失败 1 次")
        ),
        rawText = "渠道：微信\n绑定凭据：通过\n心跳：正常\n收发：正常\n去重：正常\nWeb 同步：失败"
    )

    val notificationSettings = ChannelNotificationSettings(
        newMessage = true,
        failureReminder = true,
        offlineReminder = true,
        duplicateProtection = true,
        roleProactive = false
    )
}
