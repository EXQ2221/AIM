import { useCallback, type Dispatch, type RefObject, type SetStateAction } from "react";
import { api } from "../../api";
import type { ConversationInfo, FriendInfo, MessageInfo, MessageRecalledEventInfo, MobilePane, ReplyPreviewInfo } from "../../types";
import { appendMentionToDraft, buildReplyPreview } from "../utils";
import { applyConversationRecalled, applyMessageRecalled, RECALLED_MESSAGE_PLACEHOLDER } from "../helpers/chat-runtime";
import type { ToastTone } from "../types";

type UseChatInteractionDeps = {
  setConversations: Dispatch<SetStateAction<ConversationInfo[]>>;
  setSelectedConversationId: (value: string | null) => void;
  setMobilePane: (value: MobilePane) => void;
  showToast: (message: string, tone?: ToastTone) => void;
  setMessageDraft: Dispatch<SetStateAction<string>>;
  composerRef: RefObject<HTMLTextAreaElement | null>;
  setReplyingTo: Dispatch<SetStateAction<ReplyPreviewInfo | null>>;
  setMessages: Dispatch<SetStateAction<MessageInfo[]>>;
};

export function useChatInteractionDomain(deps: UseChatInteractionDeps) {
  const {
    setConversations,
    setSelectedConversationId,
    setMobilePane,
    showToast,
    setMessageDraft,
    composerRef,
    setReplyingTo,
    setMessages
  } = deps;

  const handleOpenChatWithFriend = useCallback(async (friend: FriendInfo) => {
    try {
      const result = await api.findSingleConversation(friend.user_id);
      if (result) {
        setConversations((prev) => {
          const exists = prev.some((c) => c.conversationId === result.conversationId);
          if (!exists) {
            return [...prev, result];
          }
          return prev;
        });
        setSelectedConversationId(result.conversationId);
        setMobilePane("chat");
      }
    } catch (err) {
      showToast(err instanceof Error ? err.message : "查找私聊会话失败", "error");
    }
  }, [setConversations, setMobilePane, setSelectedConversationId, showToast]);

  const handleMention = useCallback((mentionTarget: string) => {
    const normalized = mentionTarget.trim().replace(/^@+/, "");
    if (!normalized) {
      return;
    }
    setMessageDraft((current) => appendMentionToDraft(current, normalized));
    window.requestAnimationFrame(() => {
      composerRef.current?.focus();
    });
  }, [composerRef, setMessageDraft]);

  const handleReplyMessage = useCallback((message: MessageInfo) => {
    if (message.pending || message.id <= 0 || message.status !== "NORMAL") {
      return;
    }
    setReplyingTo(buildReplyPreview(message));
    window.requestAnimationFrame(() => {
      composerRef.current?.focus();
    });
  }, [composerRef, setReplyingTo]);

  const applyRecalledMessageEvent = useCallback((event: MessageRecalledEventInfo) => {
    setMessages((current) => applyMessageRecalled(current, event));
    setConversations((current) => applyConversationRecalled(current, event));
    setReplyingTo((current) =>
      current?.messageId === event.messageId
        ? {
            ...current,
            contentPreview: RECALLED_MESSAGE_PLACEHOLDER
          }
        : current
    );
  }, [setConversations, setMessages, setReplyingTo]);

  return {
    handleOpenChatWithFriend,
    handleMention,
    handleReplyMessage,
    applyRecalledMessageEvent
  };
}
