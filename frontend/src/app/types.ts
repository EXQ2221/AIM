export type ToastState = {
  tone: "success" | "error" | "info";
  message: string;
} | null;

export type ToastTone = "success" | "error" | "info";
export type AuthMode = "login" | "register";
export type WsStatus = "connecting" | "open" | "closed";
export type DetailTab = "friends" | "members" | "bots" | "logs" | "account";
export type BrowserNotificationStatus = NotificationPermission | "unsupported";

export type PendingMessageEntry = {
  tempId: number;
  conversationId: string;
};

export type CropOffset = {
  x: number;
  y: number;
};

export type ImageSize = {
  width: number;
  height: number;
};

export const joinPolicies = [
  { value: "FREE", label: "自由加入" },
  { value: "INVITE_ONLY", label: "仅邀请" },
  { value: "APPROVAL", label: "需审核" }
];

export const wsReconnectDelays = [1000, 3000, 5000, 10000, 30000];

export const cropViewportSize = 220;
export const avatarOutputSize = 512;
