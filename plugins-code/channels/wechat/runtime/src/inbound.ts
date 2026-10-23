export interface InboundConfig {
    coreUrl: string;
    extensionId: string;
    moduleId: string;
    serviceAuthToken: string;
}

export interface BufferedMessage {
    fromUserId: string;
    messageId: string;
    text: string;
    audioBase64?: string;
    imageBase64?: string;
    imageUrl?: string;
    isVoice?: boolean;
}

export function buildInboundRequest(messages: BufferedMessage[], imageUrl = "") {
    if (!messages.length) throw new Error("入站消息不能为空");
    const last = messages[messages.length - 1];
    const audio = messages.find(message => message.audioBase64);
    const image = messages.find(message => message.imageBase64 || message.imageUrl);
    const audioUrl = audio?.audioBase64 ? `data:audio/wav;base64,${audio.audioBase64}` : "";
    const resolvedImage = imageUrl || (image?.imageBase64 ? `data:image/jpeg;base64,${image.imageBase64}` : image?.imageUrl || "");
    return {
        channelId: "wechat",
        accountId: "openclaw-wechat",
        conversationId: `conv-${last.fromUserId}`,
        peerId: last.fromUserId,
        messageId: last.messageId,
        contentType: audioUrl ? "audio" : resolvedImage ? "image" : "text",
        text: messages.map(message => message.text).join("\n"),
        ...(audioUrl ? { audioUrl } : {}),
        ...(resolvedImage ? { imageUrl: resolvedImage } : {}),
    };
}

export function createInboundClient(config: InboundConfig, fetcher: typeof fetch = fetch) {
    if (!config.coreUrl || !config.extensionId || !config.moduleId || !config.serviceAuthToken) {
        throw new Error("微信入站服务缺少宿主注入的身份或服务凭据");
    }
    return async (messages: BufferedMessage[], imageUrl = "") => {
        const response = await fetcher(`${config.coreUrl.replace(/\/$/, "")}/api/channels/inbound`, {
            method: "POST",
            headers: {
                "Content-Type": "application/json",
                "Authorization": `Bearer ${config.serviceAuthToken}`,
                "X-Amitia-Extension-ID": config.extensionId,
                "X-Amitia-Module-ID": config.moduleId,
            },
            body: JSON.stringify(buildInboundRequest(messages, imageUrl)),
            signal: AbortSignal.timeout(180000),
        });
        const body = await response.json() as { code?: number; msg?: string; data?: { requestId?: string; interactionId?: string; outcome?: string } };
        if (!response.ok || body.code !== 200) {
            throw new Error(`微信入站转发失败 HTTP ${response.status}: ${body.msg || "宿主拒绝请求"}`);
        }
        return body.data;
    };
}
