import { useCallback, type Dispatch, type RefObject, type SetStateAction } from "react";
import { api } from "../../api";
import { errorMessage, mergeMessagesById, scrollMessagesToBottom } from "../utils";
import { latestMessageId } from "../helpers/chat-runtime";
import type { MessageInfo } from "../../types";
import type { ToastTone } from "../types";

type UseRealtimeStateDeps = {
  selectedConversationIdRef: RefObject<string | null>;
  messageListRef: RefObject<HTMLDivElement | null>;
  realtimeRecoveringRef: RefObject<boolean>;
  lastMarkedReadRef: RefObject<Map<string, number>>;
  refreshConversations: () => Promise<unknown>;
  showToast: (message: string, tone?: ToastTone) => void;
  setMessages: Dispatch<SetStateAction<MessageInfo[]>>;
  setUnreadCounts: Dispatch<SetStateAction<Record<string, number>>>;
  filterVisibleMessages?: (conversationID: string, nextMessages: MessageInfo[]) => MessageInfo[];
};

export function useRealtimeState(deps: UseRealtimeStateDeps) {
  const {
    selectedConversationIdRef,
    messageListRef,
    realtimeRecoveringRef,
    lastMarkedReadRef,
    refreshConversations,
    showToast,
    setMessages,
    setUnreadCounts,
    filterVisibleMessages
  } = deps;

  const markConversationRead = useCallback(async (conversationID: string, lastReadMessageId: number) => {
    if (!conversationID || lastReadMessageId <= 0) {
      return;
    }
    const previous = lastMarkedReadRef.current.get(conversationID) ?? 0;
    if (previous >= lastReadMessageId) {
      return;
    }
    lastMarkedReadRef.current.set(conversationID, lastReadMessageId);
    try {
      await api.markConversationRead(conversationID, lastReadMessageId);
    } catch {
      if ((lastMarkedReadRef.current.get(conversationID) ?? 0) === lastReadMessageId) {
        lastMarkedReadRef.current.delete(conversationID);
      }
    }
  }, [lastMarkedReadRef]);

  const refreshCurrentConversationMessages = useCallback(async () => {
    const conversationID = selectedConversationIdRef.current;
    if (!conversationID) return;

    const nextMessages = await api.messages(conversationID, { limit: 50 });
    if (selectedConversationIdRef.current !== conversationID) return;
    const filteredMessages = filterVisibleMessages ? filterVisibleMessages(conversationID, nextMessages) : nextMessages;

    setMessages((current) => mergeMessagesById(current, filteredMessages));
    setUnreadCounts((current) => ({ ...current, [conversationID]: 0 }));
    window.setTimeout(() => scrollMessagesToBottom(messageListRef), 20);
    const lastReadMessageId = latestMessageId(nextMessages);
    if (lastReadMessageId > 0) {
      void markConversationRead(conversationID, lastReadMessageId);
    }
  }, [filterVisibleMessages, markConversationRead, messageListRef, selectedConversationIdRef, setMessages, setUnreadCounts]);

  const recoverRealtimeState = useCallback(async () => {
    if (realtimeRecoveringRef.current) return;
    realtimeRecoveringRef.current = true;
    try {
      await Promise.all([refreshConversations(), refreshCurrentConversationMessages()]);
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      realtimeRecoveringRef.current = false;
    }
  }, [realtimeRecoveringRef, refreshConversations, refreshCurrentConversationMessages, showToast]);

  return {
    markConversationRead,
    refreshCurrentConversationMessages,
    recoverRealtimeState
  };
}
