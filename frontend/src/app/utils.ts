import type React from "react";
import { APIError } from "../api";
import type {
  ConversationInfo,
  FileMessageContent,
  FriendInfo,
  FriendRequestInfo,
  ImageMessageContent,
  MemberInfo,
  MessageInfo,
  ReplyPreviewInfo,
  SystemMessageContent,
  TextMessageContent,
  VoiceMessageContent
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

function parseJSONObject(value: string): Record<string, unknown> | null {
  const trimmed = value.trim();
  if (!trimmed) return null;
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
    // Compatible with double-encoded JSON payloads, e.g. "\"{\\\"text\\\":\\\"...\\\"}\"".
    if (typeof parsed === "string") {
      const nested = parsed.trim();
      if (!nested) return null;
      const parsedNested = JSON.parse(nested) as unknown;
      if (parsedNested && typeof parsedNested === "object" && !Array.isArray(parsedNested)) {
        return parsedNested as Record<string, unknown>;
      }
    }
  } catch {
    return null;
  }
  return null;
}

function readString(record: Record<string, unknown>, key: string) {
  const value = record[key];
  return typeof value === "string" ? value.trim() : "";
}

function readNumber(record: Record<string, unknown>, key: string) {
  const value = record[key];
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function parseTextMessageContent(content: string): TextMessageContent | null {
  const parsed = parseJSONObject(content);
  if (!parsed) return null;
  const text = readString(parsed, "text");
  return text ? { text } : null;
}

export function parseImageMessageContent(content: string): ImageMessageContent | null {
  const parsed = parseJSONObject(content);
  if (!parsed) return null;
  const url = readString(parsed, "url");
  const name = readString(parsed, "name");
  const mimeType = readString(parsed, "mimeType");
  if (!url || !name || !mimeType) return null;
  const text = readString(parsed, "text");
  return {
    url,
    name,
    mimeType,
    size: readNumber(parsed, "size"),
    width: readNumber(parsed, "width"),
    height: readNumber(parsed, "height"),
    text: text || undefined
  };
}

export function parseFileMessageContent(content: string): FileMessageContent | null {
  const parsed = parseJSONObject(content);
  if (!parsed) return null;
  const url = readString(parsed, "url");
  const name = readString(parsed, "name");
  const mimeType = readString(parsed, "mimeType");
  const size = readNumber(parsed, "size");
  if (!url || !name || !mimeType || !size || size <= 0) return null;
  return { url, name, mimeType, size };
}

export function parseVoiceMessageContent(content: string): VoiceMessageContent | null {
  const parsed = parseJSONObject(content);
  if (!parsed) return null;
  const url = readString(parsed, "url");
  const name = readString(parsed, "name");
  const mimeType = readString(parsed, "mimeType");
  const durationMs = readNumber(parsed, "durationMs");
  if (!url || !name || !mimeType || !durationMs || durationMs <= 0) return null;
  return {
    url,
    name,
    mimeType,
    durationMs,
    size: readNumber(parsed, "size")
  };
}

export function parseSystemMessageContent(content: string): SystemMessageContent | null {
  const parsed = parseJSONObject(content);
  if (!parsed) return null;
  const text = readString(parsed, "text");
  if (!text) return null;

  const eventType = readString(parsed, "eventType") || undefined;
  const actorUserId = readNumber(parsed, "actorUserId");
  const targetUserIds = Array.isArray(parsed.targetUserIds)
    ? parsed.targetUserIds.filter((value): value is number => typeof value === "number" && Number.isFinite(value))
    : undefined;

  return {
    text,
    eventType,
    actorUserId,
    targetUserIds
  };
}

export function messageText(message: Pick<MessageInfo, "messageType" | "content" | "status">) {
  if (message.status === "RECALLED") {
    return "消息已撤回";
  }

  switch (message.messageType) {
    case "TEXT":
      return parseTextMessageContent(message.content)?.text || message.content.trim();
    case "IMAGE":
      const imageText = parseImageMessageContent(message.content)?.text;
      return imageText ? `[图片] ${imageText}` : "[图片]";
    case "FILE":
      return parseFileMessageContent(message.content)?.name || "[文件]";
    case "VOICE":
      return "[语音]";
    case "SYSTEM":
      return parseSystemMessageContent(message.content)?.text || message.content.trim();
    case "BOT_REPLY":
      return parseTextMessageContent(message.content)?.text || message.content.trim();
    default:
      return message.content.trim();
  }
}

function inferConversationPreview(content: string) {
  const textPayload = parseTextMessageContent(content);
  if (textPayload) return textPayload.text;

  const voicePayload = parseVoiceMessageContent(content);
  if (voicePayload) return "[语音]";

  const imagePayload = parseImageMessageContent(content);
  if (imagePayload) return "[图片]";

  const filePayload = parseFileMessageContent(content);
  if (filePayload) return filePayload.name || "[文件]";

  const systemPayload = parseSystemMessageContent(content);
  if (systemPayload) return systemPayload.text;

  return content.trim();
}

export function messageContentPreview(
  messageType: MessageInfo["messageType"],
  content: string,
  options: { truncateText?: number } = {}
) {
  const truncateText = options.truncateText ?? 80;
  let preview = "";

  switch (messageType) {
    case "TEXT":
      preview = parseTextMessageContent(content)?.text || content.trim();
      break;
    case "IMAGE":
      {
        const image = parseImageMessageContent(content);
        preview = image?.text ? `[图片] ${image.text}` : "[图片]";
      }
      break;
    case "FILE":
      preview = parseFileMessageContent(content)?.name || "[文件]";
      break;
    case "VOICE":
      preview = "[语音]";
      break;
    case "SYSTEM":
      preview = parseSystemMessageContent(content)?.text || content.trim();
      break;
    case "BOT_REPLY":
      preview = parseTextMessageContent(content)?.text || content.trim();
      break;
    default:
      preview = inferConversationPreview(content);
      break;
  }

  const runes = Array.from(preview.trim());
  if (truncateText > 0 && runes.length > truncateText) {
    return `${runes.slice(0, truncateText).join("")}...`;
  }
  return runes.join("");
}

export function buildReplyPreview(message: Pick<MessageInfo, "id" | "senderId" | "senderType" | "messageType" | "content">): ReplyPreviewInfo {
  return {
    messageId: message.id,
    senderId: message.senderId,
    senderType: message.senderType,
    messageType: message.messageType,
    contentPreview: messageContentPreview(message.messageType, message.content, { truncateText: 80 })
  };
}

export function formatFileSize(size?: number) {
  if (!size || size <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  let value = size;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const digits = value >= 10 || unitIndex === 0 ? 0 : 1;
  return `${value.toFixed(digits)} ${units[unitIndex]}`;
}

export function formatVoiceDuration(durationMs?: number) {
  if (!durationMs || durationMs <= 0) return "";
  const totalSeconds = Math.ceil(durationMs / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${String(seconds).padStart(2, "0")}`;
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
  const preview = inferConversationPreview(conversation.lastMessageContent);
  const sender =
    conversation.lastMessageSenderName ||
    (conversation.lastMessageSenderId ? `用户${conversation.lastMessageSenderId}` : "");
  if (sender) {
    return `${sender}: ${preview}`;
  }
  return preview;
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

export function knowledgeSourceTypeLabel(sourceType: string) {
  const normalized = sourceType.trim().toUpperCase();
  const map: Record<string, string> = {
    TEXT: "文本",
    MARKDOWN: "Markdown"
  };
  return map[normalized] ?? (sourceType.trim() || "-");
}

export function knowledgeDocumentStatusLabel(status: string) {
  const normalized = status.trim().toUpperCase();
  const map: Record<string, string> = {
    PENDING: "排队中",
    PROCESSING: "处理中",
    READY: "已就绪",
    FAILED: "失败"
  };
  return map[normalized] ?? (status.trim() || "-");
}

export function knowledgeBaseStatusLabel(status: string) {
  const normalized = status.trim().toUpperCase();
  const map: Record<string, string> = {
    ACTIVE: "可用",
    DISABLED: "停用"
  };
  return map[normalized] ?? (status.trim() || "-");
}

export function errorMessage(error: unknown) {
  if (error instanceof APIError) return error.message;
  if (error instanceof Error) return error.message;
  return "操作失败，请稍后重试";
}

export function cx(...values: Array<string | false | null | undefined>) {
  return values.filter(Boolean).join(" ");
}
