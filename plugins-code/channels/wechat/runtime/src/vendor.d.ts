declare module "@tencent-weixin/openclaw-weixin/dist/src/auth/login-qr.js" {
    export const DEFAULT_ILINK_BOT_TYPE: string;
    export function startWeixinLoginWithQr(opts: {
        accountId?: string;
        apiBaseUrl: string;
        botType?: string;
        force?: boolean;
        verbose?: boolean;
    }): Promise<{
        qrcode: string;
        qrcodeUrl: string;
        sessionKey: string;
        message: string;
    }>;
    export function waitForWeixinLogin(opts: {
        sessionKey: string;
        apiBaseUrl: string;
        timeoutMs?: number;
    }): Promise<{
        connected: boolean;
        alreadyConnected?: boolean;
        botToken?: string;
        accountId?: string;
        baseUrl?: string;
        userId?: string;
        message: string;
    }>;
    export function displayQRCode(qrcodeUrl: string): void;
}
declare module "@tencent-weixin/openclaw-weixin/dist/src/api/api.js" {
    export function apiPostFetch(params: { baseUrl: string; token?: string; endpoint: string; body: string; timeoutMs?: number; label?: string }): Promise<string>;
    export function buildBaseInfo(): Record<string, unknown>;
    export function getUploadUrl(params: Record<string, unknown>): Promise<Record<string, any>>;
    export function getUpdates(params: {
        baseUrl: string;
        token?: string;
        get_updates_buf?: string;
        timeoutMs?: number;
    }): Promise<{
        ret?: number;
        errcode?: number;
        errmsg?: string;
        msgs?: any[];
        get_updates_buf?: string;
        longpolling_timeout_ms?: number;
    }>;
    export function sendMessage(params: {
        baseUrl: string;
        token?: string;
        body: {
            msg?: any;
        };
        timeoutMs?: number;
    }): Promise<void>;
    export function notifyStart(params: {
        baseUrl: string;
        token?: string;
    }): Promise<any>;
    export function notifyStop(params: {
        baseUrl: string;
        token?: string;
    }): Promise<any>;
}
declare module "@tencent-weixin/openclaw-weixin/dist/src/auth/accounts.js" {
    export const DEFAULT_BASE_URL: string;
    export interface WeixinAccount {
        token?: string;
        baseUrl?: string;
        userId?: string;
        getUpdatesBuf?: string;
        messageCount?: number;
        replyCount?: number;
    }
    export function saveWeixinAccount(id: string, data: WeixinAccount): void;
    export function loadWeixinAccount(id: string): WeixinAccount | null;
    export function listIndexedWeixinAccountIds(): string[];
    export function registerWeixinAccountId(id: string): void;
    export function unregisterWeixinAccountId(id: string): void;
    export function clearWeixinAccount(id: string): void;
}
declare module "@tencent-weixin/openclaw-weixin/dist/src/api/types.js" {
    export interface WeixinMessage {
        seq?: number;
        message_id?: number;
        from_user_id?: string;
        to_user_id?: string;
        create_time_ms?: number;
        session_id?: string;
        message_type?: number;
        message_state?: number;
        item_list?: MessageItem[];
        context_token?: string;
    }
    export interface MessageItem {
        type?: number;
        text_item?: {
            text?: string;
        };
        image_item?: any;
        voice_item?: any;
        file_item?: any;
        video_item?: any;
    }
}
