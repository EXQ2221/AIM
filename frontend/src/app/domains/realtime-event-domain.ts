import type { Dispatch, RefObject, SetStateAction } from "react";
import type {
  ConversationInfo,
  MessageInfo,
  MessageRecalledEventInfo,
  NotificationInfo,
  UserInfo,
  WebSocketEvent
} from "../../types";
import type { PendingMessageEntry, ToastTone } from "../types";
import {
  mergeMessagesById,
  parseSystemMessageContent,
  reconcilePendingMessage,
  scrollMessagesToBottom,
  sortConversations,
  sortMessages
} from "../utils";

type RealtimeHandlerDeps = {
  user: UserInfo | null;
  selectedConversationIdRef: RefObject<string | null>;
  pendingMessagesRef: RefObject<Map<string, PendingMessageEntry>>;
  messageListRef: RefObject<HTMLDivElement | null>;
  markConversationRead: (conversationID: string, lastReadMessageId: number) => void | Promise<void>;
  refreshSelectedGroupInfo: (conversationId: string) => void | Promise<unknown>;
  refreshSelectedConversationMembers: (conversationId: string) => void | Promise<void>;
  showMessageNotification: (message: MessageInfo) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  refreshConversations: () => Promise<ConversationInfo[]>;
  syncFriendStateFromRealtime: (options?: { refreshConversations?: boolean }) => void | Promise<void>;
  applyRecalledMessageEvent: (event: MessageRecalledEventInfo) => void;
  setMessages: Dispatch<SetStateAction<MessageInfo[]>>;
  setUnreadCounts: Dispatch<SetStateAction<Record<string, number>>>;
  setConversations: Dispatch<SetStateAction<ConversationInfo[]>>;
  setNotifications: Dispatch<SetStateAction<NotificationInfo[]>>;
  setNotificationUnreadCount: Dispatch<SetStateAction<number>>;
};

function replaceBotGeneratingMessage(messages: MessageInfo[], incoming: MessageInfo) {
  if (incoming.senderType !== "BOT" && incoming.messageType !== "BOT_REPLY") {
    return mergeMessagesById(messages, [incoming]);
  }
  let replaced = false;
  const updated = messages.map((message) => {
    if (!replaced && message.conversationId === incoming.conversationId && message.isBotGenerating) {
      replaced = true;
      return incoming;
    }
    return message;
  });
  return replaced ? sortMessages(updated) : mergeMessagesById(messages, [incoming]);
}

export function buildRealtimeEventHandler(deps: RealtimeHandlerDeps) {
  const memberMutationEvents = new Set([
    "MEMBER_JOINED",
    "MEMBER_LEFT",
    "MEMBER_INVITED",
    "MEMBER_REMOVED",
    "ADMIN_ADDED",
    "ADMIN_REMOVED",
    "OWNER_TRANSFERRED"
  ]);

  return (raw: string) => {
    let event: WebSocketEvent;
    try {
      event = JSON.parse(raw) as WebSocketEvent;
    } catch {
      return;
    }

    if (event.type === "CONNECTED") {
      return;
    }

    if (event.type === "MESSAGE_ACK") {
      const data = event.data as {
        messageId?: number;
        status: "SUCCESS" | "FAILED";
        errorMessage?: string;
      };
      let failedConversationID = "";
      const clientMsgID = event.clientMsgId?.trim();
      if (clientMsgID) {
        const pendingEntry = deps.pendingMessagesRef.current.get(clientMsgID);
        if (pendingEntry) {
          deps.setMessages((current) =>
            reconcilePendingMessage(current, clientMsgID, {
              id: data.messageId ?? pendingEntry.tempId,
              pending: false,
              status: data.status === "FAILED" ? "FAILED" : "NORMAL"
            })
          );
          deps.pendingMessagesRef.current.delete(clientMsgID);
          if (data.status === "SUCCESS" && pendingEntry.conversationId === deps.selectedConversationIdRef.current && data.messageId) {
            void deps.markConversationRead(pendingEntry.conversationId, data.messageId);
          }
          if (data.status === "FAILED") {
            failedConversationID = pendingEntry.conversationId;
          }
        }
      }
      if (data.status === "FAILED") {
        if (failedConversationID) {
          deps.setMessages((current) =>
            current.filter((item) => !(item.conversationId === failedConversationID && item.isBotGenerating))
          );
        }
        deps.showToast(data.errorMessage || "消息发送失败", "error");
      }
      return;
    }

    if (event.type === "NEW_MESSAGE") {
      const incoming = event.data as MessageInfo;
      const systemContent = incoming.messageType === "SYSTEM" ? parseSystemMessageContent(incoming.content) : null;
      deps.showMessageNotification(incoming);

      const activeConversationID = deps.selectedConversationIdRef.current;
      if (activeConversationID === incoming.conversationId) {
        deps.setMessages((current) => replaceBotGeneratingMessage(current, incoming));
        window.setTimeout(() => scrollMessagesToBottom(deps.messageListRef), 20);
        void deps.markConversationRead(incoming.conversationId, incoming.id);
        if (systemContent?.eventType === "ANNOUNCEMENT_UPDATED") {
          void deps.refreshSelectedGroupInfo(incoming.conversationId);
        } else if (systemContent?.eventType && memberMutationEvents.has(systemContent.eventType)) {
          void deps.refreshSelectedConversationMembers(incoming.conversationId);
        }
      } else if (incoming.messageType !== "SYSTEM") {
        deps.setUnreadCounts((current) => ({
          ...current,
          [incoming.conversationId]: (current[incoming.conversationId] ?? 0) + 1
        }));
      }

      deps.setConversations((current) =>
        sortConversations(
          current.map((item) =>
            item.conversationId === incoming.conversationId
              ? {
                  ...item,
                  lastMessageId: incoming.id,
                  lastMessageAt: incoming.createdAt,
                  lastMessageSenderId: incoming.senderId,
                  lastMessageSenderName:
                    incoming.senderType === "SYSTEM" || incoming.messageType === "SYSTEM"
                      ? ""
                      : incoming.senderType === "BOT" || incoming.messageType === "BOT_REPLY"
                      ? "Bot"
                      : incoming.senderId === deps.user?.user_id
                        ? deps.user.nickname || deps.user.aim_id
                        : item.lastMessageSenderName,
                  lastMessageContent: incoming.content,
                  updatedAt: incoming.createdAt
                }
              : item
          )
        )
      );
      return;
    }

    if (event.type === "NOTIFICATION_CREATED") {
      const data = event.data as { notification?: NotificationInfo; unreadCount?: number } | undefined;
      const notification = data?.notification;
      if (!notification || typeof notification.id !== "number" || notification.id <= 0) {
        return;
      }

      deps.setNotifications((current) => [notification, ...current.filter((item) => item.id !== notification.id)].slice(0, 8));
      if (typeof data?.unreadCount === "number") {
        deps.setNotificationUnreadCount(data.unreadCount);
      } else if (!notification.isRead) {
        deps.setNotificationUnreadCount((current) => current + 1);
      }
      if (notification.conversationId) {
        void deps.refreshConversations();
      }
      const summary = (notification.summary || notification.title || "").trim();
      deps.showToast(summary || "你有一条新通知", "info");
      return;
    }

    if (event.type === "MESSAGE_RECALLED") {
      const recalled = event.data as Partial<MessageRecalledEventInfo> | undefined;
      if (
        !recalled ||
        typeof recalled.messageId !== "number" ||
        recalled.messageId <= 0 ||
        typeof recalled.conversationId !== "string" ||
        !recalled.conversationId
      ) {
        return;
      }
      deps.applyRecalledMessageEvent({
        messageId: recalled.messageId,
        conversationId: recalled.conversationId
      });
      return;
    }

    if (event.type === "FRIEND_SYNC") {
      const data = event.data as {
        reason: string;
        status?: string;
        conversationId?: string;
      };
      if (data.reason === "PRESENCE_CHANGED") {
        void deps.syncFriendStateFromRealtime();
        return;
      }
      void deps.syncFriendStateFromRealtime({
        refreshConversations: Boolean(data.conversationId) || (data.reason === "REQUEST_RESPONDED" && data.status === "ACCEPTED")
      });
    }
  };
}
