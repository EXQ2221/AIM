import type { ConversationInfo, MemberInfo, MessageInfo, MessageRecalledEventInfo } from "../../types";
import { mergeMessagesById, sortConversations } from "../utils";

export const NOTIFICATION_PREFERENCE_KEY = "aim:notifications:enabled";
export const RECALLED_MESSAGE_PLACEHOLDER = "消息已撤回";

export function loadNotificationPreference() {
  if (typeof window === "undefined") return true;
  return window.localStorage.getItem(NOTIFICATION_PREFERENCE_KEY) !== "off";
}

export function saveNotificationPreference(enabled: boolean) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(NOTIFICATION_PREFERENCE_KEY, enabled ? "on" : "off");
}

export function latestMessageId(messages: MessageInfo[]) {
  return messages.reduce((max, message) => (message.id > max && !message.pending ? message.id : max), 0);
}

export function readReceiptLabel(conversation: ConversationInfo | null, message: MessageInfo, mine: boolean) {
  if (!conversation || !mine || message.pending || message.status === "FAILED") return undefined;
  if (conversation.type === "SINGLE") return message.readByPeer ? "Read" : "Unread";
  if (conversation.type === "GROUP" && typeof message.readCount === "number") return `${message.readCount} read`;
  return undefined;
}

export function applyMessageRecalled(messages: MessageInfo[], event: MessageRecalledEventInfo) {
  return mergeMessagesById(
    [],
    messages.map((message) => {
      const next =
        message.id === event.messageId
          ? { ...message, content: "", pending: false, status: "RECALLED" as const }
          : message;
      if (next.replyTo?.messageId === event.messageId) {
        return {
          ...next,
          replyTo: {
            ...next.replyTo,
            contentPreview: RECALLED_MESSAGE_PLACEHOLDER
          }
        };
      }
      return next;
    })
  );
}

export function applyConversationRecalled(conversations: ConversationInfo[], event: MessageRecalledEventInfo) {
  return sortConversations(
    conversations.map((conversation) =>
      conversation.conversationId === event.conversationId && conversation.lastMessageId === event.messageId
        ? { ...conversation, lastMessageContent: RECALLED_MESSAGE_PLACEHOLDER }
        : conversation
    )
  );
}

export function isMemberMuted(member: Pick<MemberInfo, "muteUntil"> | null | undefined) {
  return typeof member?.muteUntil === "number" && member.muteUntil > Math.floor(Date.now() / 1000);
}

export function formatMuteUntil(value?: number | null) {
  if (!value) return "";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value * 1000));
}
