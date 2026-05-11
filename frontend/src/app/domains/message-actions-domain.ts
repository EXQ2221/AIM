import { useCallback, type Dispatch, type RefObject, type SetStateAction } from "react";
import { api } from "../../api";
import type {
  ConversationInfo,
  MessageInfo,
  MessageRecalledEventInfo,
  OutgoingMessagePayload,
  ReplyPreviewInfo,
  UserInfo
} from "../../types";
import type { PendingMessageEntry, ToastTone } from "../types";
import { errorMessage, mergeMessagesById, reconcilePendingMessage, scrollMessagesToBottom, sortConversations, sortMessages } from "../utils";

type UseMessageActionsDeps = {
  user: UserInfo | null;
  selectedConversationId: string | null;
  selectedConversationType: ConversationInfo["type"] | null;
  selectedConversation: ConversationInfo | null;
  canSendCurrentConversation: boolean;
  messageDraft: string;
  replyingTo: ReplyPreviewInfo | null;
  messages: MessageInfo[];
  socketRef: RefObject<WebSocket | null>;
  messageListRef: RefObject<HTMLDivElement | null>;
  pendingMessagesRef: RefObject<Map<string, PendingMessageEntry>>;
  showToast: (message: string, tone?: ToastTone) => void;
  applyRecalledMessageEvent: (event: MessageRecalledEventInfo) => void;
  setMessageDraft: Dispatch<SetStateAction<string>>;
  setReplyingTo: Dispatch<SetStateAction<ReplyPreviewInfo | null>>;
  setMessages: Dispatch<SetStateAction<MessageInfo[]>>;
  setConversations: Dispatch<SetStateAction<ConversationInfo[]>>;
  setLoadingOlder: Dispatch<SetStateAction<boolean>>;
};

export function useMessageActions(deps: UseMessageActionsDeps) {
  const {
    user,
    selectedConversationId,
    selectedConversation,
    canSendCurrentConversation,
    messageDraft,
    replyingTo,
    messages,
    socketRef,
    messageListRef,
    pendingMessagesRef,
    showToast,
    applyRecalledMessageEvent,
    setMessageDraft,
    setReplyingTo,
    setMessages,
    setConversations,
    setLoadingOlder
  } = deps;

  const handleRecallMessage = useCallback(
    async (message: MessageInfo) => {
      if (!message || message.pending || message.id <= 0 || message.status !== "NORMAL") {
        return;
      }
      if (!window.confirm("确认撤回这条消息吗？")) {
        return;
      }
      try {
        await api.recallMessage(message.conversationId, message.id);
        applyRecalledMessageEvent({
          messageId: message.id,
          conversationId: message.conversationId
        });
      } catch (error) {
        showToast(errorMessage(error), "error");
      }
    },
    [applyRecalledMessageEvent, showToast]
  );

  const handleSendMessage = useCallback(
    (payload?: OutgoingMessagePayload) => {
      const content = messageDraft.trim();
      if (!selectedConversationId || !user) return;
      if (!canSendCurrentConversation) {
        showToast("You cannot continue sending messages in this conversation.", "error");
        return;
      }
      const socket = socketRef.current;
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        showToast("Realtime connection is not ready", "error");
        return;
      }

      const outgoing =
        payload ??
        (content
          ? {
              messageType: "TEXT" as const,
              contentPayload: { text: content }
            }
          : null);
      if (!outgoing) {
        return;
      }

      const clientMsgId = `web-${Date.now()}-${Math.random().toString(16).slice(2)}`;
      const tempId = -1 * (Date.now() + Math.floor(Math.random() * 1000));
      const createdAt = Math.floor(Date.now() / 1000);
      const replyToId = replyingTo?.messageId && replyingTo.messageId > 0 ? replyingTo.messageId : undefined;
      const pendingMessage: MessageInfo = {
        id: tempId,
        clientMsgId,
        conversationId: selectedConversationId,
        senderId: user.user_id,
        senderType: "USER",
        messageType: outgoing.messageType,
        content: JSON.stringify(outgoing.contentPayload),
        replyToId,
        replyTo: replyingTo,
        status: "NORMAL",
        createdAt,
        readByPeer: selectedConversation?.type === "SINGLE" ? false : undefined,
        readCount: selectedConversation?.type === "GROUP" ? 0 : undefined,
        pending: true
      };

      pendingMessagesRef.current.set(clientMsgId, {
        tempId,
        conversationId: selectedConversationId
      });
      setMessages((current) => mergeMessagesById(current, [pendingMessage]));
      setConversations((current) =>
        sortConversations(
          current.map((item) =>
            item.conversationId === selectedConversationId
              ? {
                  ...item,
                  lastMessageContent: JSON.stringify(outgoing.contentPayload),
                  lastMessageSenderId: user.user_id,
                  lastMessageSenderName: user.nickname || user.aim_id,
                  updatedAt: createdAt,
                  lastMessageAt: createdAt
                }
              : item
          )
        )
      );
      window.setTimeout(() => scrollMessagesToBottom(messageListRef), 20);

      try {
        socket.send(
          JSON.stringify({
            type: "SEND_MESSAGE",
            clientMsgId,
            data: {
              conversationId: selectedConversationId,
              messageType: outgoing.messageType,
              contentPayload: outgoing.contentPayload,
              replyToId
            }
          })
        );
        setReplyingTo(null);
      } catch {
        pendingMessagesRef.current.delete(clientMsgId);
        setMessages((current) =>
          reconcilePendingMessage(current, clientMsgId, {
            pending: false,
            status: "FAILED"
          })
        );
        showToast("Send failed", "error");
        return;
      }
      if (outgoing.messageType === "TEXT") {
        setMessageDraft("");
      }
    },
    [
      canSendCurrentConversation,
      messageDraft,
      messageListRef,
      pendingMessagesRef,
      replyingTo,
      selectedConversation,
      selectedConversationId,
      setConversations,
      setMessageDraft,
      setMessages,
      setReplyingTo,
      showToast,
      socketRef,
      user
    ]
  );

  const handleLoadOlder = useCallback(async () => {
    if (!selectedConversationId || messages.length === 0) return;
    setLoadingOlder(true);
    try {
      const oldest = messages[0];
      const older = sortMessages(await api.messages(selectedConversationId, { beforeId: oldest.id, limit: 30 }));
      setMessages((current) => mergeMessagesById(current, older));
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setLoadingOlder(false);
    }
  }, [messages, selectedConversationId, setLoadingOlder, setMessages, showToast]);

  return {
    handleRecallMessage,
    handleSendMessage,
    handleLoadOlder
  };
}
