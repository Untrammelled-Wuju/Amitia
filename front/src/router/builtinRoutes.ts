import type { RouteRecordRaw } from "vue-router";

/** Built-in business pages. Core router owns only bootstrap/recovery routes. */
export const builtinBusinessRoutes: RouteRecordRaw[] = [
  { path: "/chat", name: "chat", component: () => import("@/views/web-chat/WebChatView.vue"), meta: { requiresAuth: true } },
  { path: "/attention", name: "attentionCenter", component: () => import("@/views/attention/AttentionCenterView.vue"), meta: { requiresAuth: true } },
  { path: "/call/:conversationId", name: "ownedIncomingCall", component: () => import("@/views/realtime/OwnedIncomingCallView.vue"), meta: { requiresAuth: true } },
  { path: "/extension/page/:pageId", name: "extensionPage", component: () => import("@/components/extension/ExtensionPageHost.vue"), meta: { requiresAuth: true, fullHeight: true } },
  { path: "/settings/ui-providers", name: "settingsUIProviders", component: () => import("@/views/settings/UIProviderSettingsView.vue"), meta: { requiresAuth: true, allowsExtensionChildren: true } },
];
