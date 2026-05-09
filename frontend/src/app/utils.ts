import type React from "react";
import { APIError } from "../api";
import type {
  ConversationInfo,
  FriendInfo,
  FriendRequestInfo,
  MemberInfo,
  MessageInfo
} from "../types";
import type { BrowserNotificationStatus } from "./types";

export function getNotificationStatus(): BrowserNotificationStatus {
  if (typeof Notification === "undefined") {
    return "unsupported";
  }
  return Notification.permission;
}

export function notificationStatusLabel(status: BrowserNotificationStatus) {
  switch (status) {
    case "granted":
      return "浏览器通知已开启";
    case "denied":
      return "浏览器通知已拒绝";
    case "unsupported":
      return "当前浏览器不支持通知";
    default:
      return "浏览器通知未授权";
  }
}

export function truncateNotificationBody(content: string) {
  const runes = Array.from(content.trim());
  if (runes.length <= 50) return content.trim();
  return `${runes.slice(0, 50).join("")}...`;
}

export function scrollMessagesToBottom(messageListRef?: React.RefObject<HTMLDivElement | null>) {
  const scroller = messageListRef?.current;
  if (!scroller) return;
  window.requestAnimationFrame(() => {
    window.requestAnimationFrame(() => {
      scroller.scrollTop = scroller.scrollHeight;
    });
  });
}

export function handleAvatarMention(
  event: React.MouseEvent<HTMLImageElement | HTMLSpanElement>,
  mentionTarget: string | undefined,
  onMention: (mentionTarget: string) => void
) {
  const normalized = normalizeMentionTarget(mentionTarget);
  if (!normalized) return;
  event.preventDefault();
  onMention(normalized);
}

export function normalizeMentionTarget(value: string | undefined) {
  return value?.trim().replace(/^@+/, "") || "";
}

export function appendMentionToDraft(currentDraft: string, mentionTarget: string) {
  const mention = `@${mentionTarget} `;
  const trimmedRight = currentDraft.replace(/\s+$/, "");
  if (!trimmedRight) return mention;
  if (trimmedRight.endsWith(`@${mentionTarget}`)) {
    return `${trimmedRight} `;
  }
  return `${trimmedRight} ${mention}`;
}

export function sortMessages(messages: MessageInfo[]) {
  return [...messages].sort((a, b) => a.id - b.id);
}

export function mergeMessagesById(oldMessages: MessageInfo[], incomingMessages: MessageInfo[]) {
  const byID = new Map<number, MessageInfo>();
  for (const message of oldMessages) {
    byID.set(message.id, message);
  }
  for (const message of incomingMessages) {
    byID.set(message.id, message);
  }
  return sortMessages(Array.from(byID.values()));
}

export function reconcilePendingMessage(messages: MessageInfo[], clientMsgId: string, patch: Partial<MessageInfo>) {
  return mergeMessagesById(
    [],
    messages.map((message) => (message.clientMsgId === clientMsgId ? { ...message, ...patch } : message))
  );
}

export function sortConversations(conversations: ConversationInfo[]) {
  return [...conversations].sort((a, b) => {
    const left = a.lastMessageAt ?? a.updatedAt ?? 0;
    const right = b.lastMessageAt ?? b.updatedAt ?? 0;
    return right - left;
  });
}

export function sortFriends(friends: FriendInfo[]) {
  return [...friends].sort((a, b) => {
    const left = a.updated_at ?? a.created_at ?? 0;
    const right = b.updated_at ?? b.created_at ?? 0;
    return right - left;
  });
}

export function sortFriendRequests(requests: FriendRequestInfo[]) {
  return [...requests].sort((a, b) => {
    const left = a.updated_at ?? a.created_at ?? 0;
    const right = b.updated_at ?? b.created_at ?? 0;
    return right - left;
  });
}

export function conversationPreview(conversation: ConversationInfo) {
  if (!conversation.lastMessageContent) {
    return "暂无消息";
  }
  const sender =
    conversation.lastMessageSenderName ||
    (conversation.lastMessageSenderId ? `用户${conversation.lastMessageSenderId}` : "");
  if (sender) {
    return `${sender}: ${conversation.lastMessageContent}`;
  }
  return conversation.lastMessageContent;
}

export function canSendToConversation(
  conversation: ConversationInfo | null,
  members: MemberInfo[],
  friends: FriendInfo[]
) {
  if (!conversation) return false;
  if (conversation.type !== "SINGLE") return true;

  const peer = members.find(
    (member) => member.status !== "REMOVED" && friends.some((friend) => friend.user_id === member.userId)
  );
  return Boolean(peer);
}

export function parseGroupValue(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return null;
  const parsed = Number(trimmed);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
}

export function initials(value: string) {
  const trimmed = value.trim();
  if (!trimmed) return "A";
  return trimmed.slice(0, 2).toUpperCase();
}

export function formatClock(value?: number | null) {
  if (!value) return "--:--";
  return new Intl.DateTimeFormat("zh-CN", {
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value * 1000));
}

export function formatRelative(value?: number | null) {
  if (!value) return "刚刚";
  const now = Date.now();
  const date = value > 10_000_000_000 ? new Date(value) : new Date(value * 1000);
  const delta = Math.max(0, now - date.getTime());
  const minutes = Math.floor(delta / 60_000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时前`;
  return new Intl.DateTimeFormat("zh-CN", { month: "2-digit", day: "2-digit" }).format(date);
}

export function roleLabel(role: string) {
  const map: Record<string, string> = {
    OWNER: "群主",
    ADMIN: "管理员",
    MEMBER: "成员",
    BOT: "Bot",
    user: "用户",
    admin: "管理员"
  };
  return map[role] ?? role;
}

export function statusLabel(status: string) {
  const map: Record<string, string> = {
    NORMAL: "正常",
    MUTED: "禁言",
    REMOVED: "已移除"
  };
  return map[status] ?? status;
}

export function errorMessage(error: unknown) {
  if (error instanceof APIError) return error.message;
  if (error instanceof Error) return error.message;
  return "操作失败，请稍后重试";
}

export function cx(...values: Array<string | false | null | undefined>) {
  return values.filter(Boolean).join(" ");
}
