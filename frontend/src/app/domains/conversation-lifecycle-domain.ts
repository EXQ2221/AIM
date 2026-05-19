import { useEffect, type Dispatch, type RefObject, type SetStateAction } from "react";
import { api } from "../../api";
import type { GroupInfo, MemberInfo, MessageInfo, ReplyPreviewInfo, UserInfo } from "../../types";
import { errorMessage, mergeMessagesById, scrollMessagesToBottom } from "../utils";
import { latestMessageId } from "../helpers/chat-runtime";
import type { ToastTone } from "../types";

type UseConversationLifecycleDeps = {
  bootstrap: () => Promise<void>;
  selectedConversationId: string | null;
  selectedConversationType: string | null;
  selectedConversationIdRef: RefObject<string | null>;
  messages: MessageInfo[];
  user: UserInfo | null;
  messageListRef: RefObject<HTMLDivElement | null>;
  markConversationRead: (conversationID: string, lastReadMessageId: number) => Promise<void> | void;
  showToast: (message: string, tone?: ToastTone) => void;
  setUnreadCounts: Dispatch<SetStateAction<Record<string, number>>>;
  setReplyingTo: Dispatch<SetStateAction<ReplyPreviewInfo | null>>;
  setMessages: Dispatch<SetStateAction<MessageInfo[]>>;
  setMembers: Dispatch<SetStateAction<MemberInfo[]>>;
  setSelectedGroupInfo: Dispatch<SetStateAction<GroupInfo | null>>;
  setLoadingMessages: Dispatch<SetStateAction<boolean>>;
  filterVisibleMessages?: (conversationID: string, nextMessages: MessageInfo[]) => MessageInfo[];
  shouldAutoScrollOnMessagesChange?: () => boolean;
};

export function useConversationLifecycle(deps: UseConversationLifecycleDeps) {
  const {
    bootstrap,
    selectedConversationId,
    selectedConversationType,
    selectedConversationIdRef,
    messages,
    user,
    messageListRef,
    markConversationRead,
    showToast,
    setUnreadCounts,
    setReplyingTo,
    setMessages,
    setMembers,
    setSelectedGroupInfo,
    setLoadingMessages,
    filterVisibleMessages,
    shouldAutoScrollOnMessagesChange
  } = deps;

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  useEffect(() => {
    selectedConversationIdRef.current = selectedConversationId;
    if (selectedConversationId) {
      setUnreadCounts((current) => ({ ...current, [selectedConversationId]: 0 }));
    }
    setReplyingTo(null);
  }, [selectedConversationId, selectedConversationIdRef, setReplyingTo, setUnreadCounts]);

  useEffect(() => {
    if (!selectedConversationId || messages.length === 0 || !user) {
      return;
    }
    const lastMessage = messages[messages.length - 1];
    if (lastMessage.conversationId !== selectedConversationId) {
      return;
    }
    if (lastMessage.pending || lastMessage.senderId === user.user_id) {
      if (shouldAutoScrollOnMessagesChange && !shouldAutoScrollOnMessagesChange()) {
        return;
      }
      scrollMessagesToBottom(messageListRef);
    }
  }, [messages, messageListRef, selectedConversationId, shouldAutoScrollOnMessagesChange, user]);

  useEffect(() => {
    if (!selectedConversationId) {
      setMessages([]);
      setMembers([]);
      setSelectedGroupInfo(null);
      return;
    }

    let active = true;
    setLoadingMessages(true);
    const shouldLoadGroupInfo = selectedConversationType === "GROUP";
    if (!shouldLoadGroupInfo) {
      setSelectedGroupInfo(null);
    }
    Promise.all([
      api.messages(selectedConversationId, { limit: 30 }),
      api.members(selectedConversationId),
      shouldLoadGroupInfo ? api.groupInfo(selectedConversationId) : Promise.resolve(null)
    ])
      .then(([nextMessages, nextMembers, nextGroupInfo]) => {
        if (!active) return;
        const filteredMessages = filterVisibleMessages ? filterVisibleMessages(selectedConversationId, nextMessages) : nextMessages;
        setMessages(mergeMessagesById([], filteredMessages));
        setMembers(nextMembers);
        setSelectedGroupInfo(nextGroupInfo);
        window.setTimeout(() => scrollMessagesToBottom(messageListRef), 20);
        const lastReadMessageId = latestMessageId(nextMessages);
        if (lastReadMessageId > 0) {
          void markConversationRead(selectedConversationId, lastReadMessageId);
        }
      })
      .catch((error: unknown) => {
        if (!active) return;
        showToast(errorMessage(error), "error");
      })
      .finally(() => {
        if (active) setLoadingMessages(false);
      });

    return () => {
      active = false;
    };
  }, [
    markConversationRead,
    messageListRef,
    selectedConversationId,
    selectedConversationType,
    setLoadingMessages,
    setMembers,
    setMessages,
    setSelectedGroupInfo,
    showToast,
    filterVisibleMessages
  ]);
}
