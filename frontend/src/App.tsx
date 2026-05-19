import {
  BadgePlus,
  Bell,
  Bot,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  DoorOpen,
  FileImage,
  KeyRound,
  Loader2,
  LockKeyhole,
  LogOut,
  Mail,
  MessageCircle,
  MessageSquarePlus,
  Mic,
  PanelRightOpen,
  Paperclip,
  RefreshCw,
  Search,
  SendHorizontal,
  ShieldCheck,
  Smartphone,
  Trash2,
  UserPlus,
  UserRound,
  UsersRound,
  X
} from "lucide-react";
import {
  FormEvent,
  KeyboardEvent,
  type ChangeEvent,
  type Dispatch,
  type SetStateAction,
  useEffect,
  useCallback,
  useMemo,
  useRef,
  useState
} from "react";
import { api } from "./api";
import { AvatarUploader } from "./app/avatar-uploader";
import {
  type AuthMode,
  type DetailTab,
  type PendingMessageEntry,
  type ToastState,
  type ToastTone,
  joinPolicies,
  wsReconnectDelays
} from "./app/types";
import {
  canSendToConversation,
  conversationPreview,
  cx,
  errorMessage,
  formatRelative,
  handleAvatarMention,
  knowledgeDocumentStatusLabel,
  mergeMessagesById,
  parseSystemMessageContent,
  parseGroupValue,
  roleLabel,
  scrollMessagesToBottom,
  sortConversations,
  sortFriendRequests,
  sortFriends,
  sortMessages,
  statusLabel
} from "./app/utils";
import { Avatar, Field, IconButton, MessageBubble, MobileNav, StatusPill, Toast, WsBadge } from "./app/ui";
import type {
  AICallLogInfo,
  AICallLogQuotaInfo,
  BotInfo,
  BotReplyStreamData,
  ConversationKnowledgeBaseInfo,
  ConversationInfo,
  FriendGroupInfo,
  FriendInfo,
  FriendRequestInfo,
  GroupInfo,
  KnowledgeBaseInfo,
  KnowledgeDocumentInfo,
  KnowledgeSearchChunkInfo,
  NotificationInfo,
  MemberInfo,
  MessageInfo,
  MobilePane,
  ReplyPreviewInfo,
  SessionInfo,
  TypingEventData,
  UserInfo
} from "./types";
import { AuthView } from "./app/views/auth-view";
import { ConversationPanel } from "./app/views/conversation-panel";
import { ChatPanel } from "./app/views/chat-panel";
import { DetailPanel } from "./app/views/detail-panel";
import { isMemberMuted } from "./app/helpers/chat-runtime";
import {
  addFriendAction,
  createFriendGroupAction,
  deleteFriendAction,
  respondFriendRequestAction,
  updateFriendAction
} from "./app/domains/friend-domain";
import { logoutAllAction, revokeSessionAction, uploadAvatarAction } from "./app/domains/auth-domain";
import {
  createGroupAction,
  inviteMemberAction,
  joinGroupAction,
  leaveGroupAction,
  muteMemberAction,
  removeAdminAction,
  removeMemberAction,
  setAdminAction,
  setGroupMuteAllAction,
  transferOwnerAction,
  unmuteMemberAction,
  updateGroupAnnouncementAction
} from "./app/domains/conversation-domain";
import { useRealtimeConnection } from "./app/domains/realtime-domain";
import { buildRealtimeEventHandler } from "./app/domains/realtime-event-domain";
import { useConversationLifecycle } from "./app/domains/conversation-lifecycle-domain";
import { useNotificationDomain } from "./app/domains/notification-domain";
import { useRealtimeState } from "./app/domains/realtime-state-domain";
import { useMessageActions } from "./app/domains/message-actions-domain";
import { useChatInteractionDomain } from "./app/domains/chat-interaction-domain";
import { useSessionFlow } from "./app/domains/session-flow-domain";
import { useBotPanelDomain } from "./app/domains/bot-panel-domain";
import { createAuthDomainDeps, createBotDomainDeps, createConversationDomainDeps, createFriendDomainDeps } from "./app/facades/domain-bindings";

const typingExpireMs = 6000;
const typingHeartbeatMs = 2000;
const typingIdleMs = 2500;
const maxHiddenMessageIdsPerConversation = 2000;

function App() {
  const [booting, setBooting] = useState(true);
  const [user, setUser] = useState<UserInfo | null>(null);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [friendGroups, setFriendGroups] = useState<FriendGroupInfo[]>([]);
  const [friends, setFriends] = useState<FriendInfo[]>([]);
  const [friendRequests, setFriendRequests] = useState<FriendRequestInfo[]>([]);
  const [conversations, setConversations] = useState<ConversationInfo[]>([]);
  const [notifications, setNotifications] = useState<NotificationInfo[]>([]);
  const [notificationUnreadCount, setNotificationUnreadCount] = useState(0);
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const [selectedGroupInfo, setSelectedGroupInfo] = useState<GroupInfo | null>(null);
  const [members, setMembers] = useState<MemberInfo[]>([]);
  const [messages, setMessages] = useState<MessageInfo[]>([]);
  const [hiddenMessageIdsByConversation, setHiddenMessageIdsByConversation] = useState<Record<string, number[]>>({});
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  const [messageDrafts, setMessageDrafts] = useState<Record<string, string>>({});
  const [typingByConversation, setTypingByConversation] = useState<Record<string, TypingEventData>>({});
  const [replyingTo, setReplyingTo] = useState<ReplyPreviewInfo | null>(null);
  const [search, setSearch] = useState("");
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [busyAction, setBusyAction] = useState(false);
  const [knowledgeBusy, setKnowledgeBusy] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const [mobilePane, setMobilePane] = useState<MobilePane>("conversations");
  const [detailTab, setDetailTab] = useState<DetailTab>("friends");
  const [availableBots, setAvailableBots] = useState<BotInfo[]>([]);
  const [conversationBots, setConversationBots] = useState<BotInfo[]>([]);
  const [aiCallLogs, setAICallLogs] = useState<AICallLogInfo[]>([]);
  const [aiCallLogQuota, setAICallLogQuota] = useState<AICallLogQuotaInfo>({
    dailyTotalTokens: 0,
    dailyTokenLimit: 1_000_000,
    remainingTokens: 1_000_000
  });
  const [loadingAICallLogs, setLoadingAICallLogs] = useState(false);
  const [aiCallLogStatus, setAICallLogStatus] = useState<"" | "SUCCESS" | "FAILED">("");
  const [knowledgeBases, setKnowledgeBases] = useState<KnowledgeBaseInfo[]>([]);
  const [selectedKnowledgeBaseId, setSelectedKnowledgeBaseId] = useState<number | null>(null);
  const [knowledgeDocuments, setKnowledgeDocuments] = useState<KnowledgeDocumentInfo[]>([]);
  const [knowledgeSearchChunks, setKnowledgeSearchChunks] = useState<KnowledgeSearchChunkInfo[]>([]);
  const [conversationKnowledgeBases, setConversationKnowledgeBases] = useState<ConversationKnowledgeBaseInfo[]>([]);
  const [loadingKnowledge, setLoadingKnowledge] = useState(false);

  const selectedConversationIdRef = useRef<string | null>(null);
  const messageListRef = useRef<HTMLDivElement | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const realtimeRecoveringRef = useRef(false);
  const pendingMessagesRef = useRef(new Map<string, PendingMessageEntry>());
  const lastMarkedReadRef = useRef(new Map<string, number>());
  const localTypingConversationRef = useRef<string | null>(null);
  const typingHeartbeatRef = useRef(0);
  const typingIdleTimerRef = useRef(0);
  const suppressAutoScrollOnceRef = useRef(false);

  const selectedConversation = useMemo(
    () => conversations.find((item) => item.conversationId === selectedConversationId) ?? null,
    [conversations, selectedConversationId]
  );
  const localHiddenMessagesStorageKey = useMemo(() => {
    if (!user) {
      return "";
    }
    return `aim:hidden-messages:v1:${user.user_id}`;
  }, [user]);
  useEffect(() => {
    if (!localHiddenMessagesStorageKey) {
      setHiddenMessageIdsByConversation({});
      return;
    }
    try {
      const raw = window.localStorage.getItem(localHiddenMessagesStorageKey);
      if (!raw) {
        setHiddenMessageIdsByConversation({});
        return;
      }
      const parsed = JSON.parse(raw) as Record<string, unknown>;
      const normalized: Record<string, number[]> = {};
      for (const [conversationId, messageIds] of Object.entries(parsed)) {
        if (!Array.isArray(messageIds)) {
          continue;
        }
        const ids = messageIds.filter((id): id is number => typeof id === "number" && Number.isFinite(id) && id > 0);
        if (ids.length > 0) {
          normalized[conversationId] = Array.from(new Set(ids)).slice(-maxHiddenMessageIdsPerConversation);
        }
      }
      setHiddenMessageIdsByConversation(normalized);
    } catch {
      setHiddenMessageIdsByConversation({});
    }
  }, [localHiddenMessagesStorageKey]);
  useEffect(() => {
    if (!localHiddenMessagesStorageKey) {
      return;
    }
    try {
      window.localStorage.setItem(localHiddenMessagesStorageKey, JSON.stringify(hiddenMessageIdsByConversation));
    } catch {
      // ignore storage write failures
    }
  }, [hiddenMessageIdsByConversation, localHiddenMessagesStorageKey]);
  const filterLocallyHiddenMessages = useCallback(
    (conversationId: string, nextMessages: MessageInfo[]) => {
      const hiddenMessageIds = hiddenMessageIdsByConversation[conversationId];
      if (!hiddenMessageIds || hiddenMessageIds.length === 0) {
        return nextMessages;
      }
      const hiddenSet = new Set(hiddenMessageIds);
      return nextMessages.filter((item) => !(item.conversationId === conversationId && hiddenSet.has(item.id)));
    },
    [hiddenMessageIdsByConversation]
  );
  const hiddenMessageIdSet = useMemo(() => {
    if (!selectedConversationId) {
      return new Set<number>();
    }
    return new Set(hiddenMessageIdsByConversation[selectedConversationId] ?? []);
  }, [hiddenMessageIdsByConversation, selectedConversationId]);
  const visibleMessages = useMemo(() => {
    if (!selectedConversationId || hiddenMessageIdSet.size === 0) {
      return messages;
    }
    return messages.filter((item) => item.conversationId !== selectedConversationId || !hiddenMessageIdSet.has(item.id));
  }, [hiddenMessageIdSet, messages, selectedConversationId]);
  const messageDraft = useMemo(() => {
    if (!selectedConversationId) {
      return "";
    }
    return messageDrafts[selectedConversationId] ?? "";
  }, [messageDrafts, selectedConversationId]);
  const setMessageDraft: Dispatch<SetStateAction<string>> = useCallback((value) => {
    const conversationId = selectedConversationIdRef.current;
    if (!conversationId) {
      return;
    }
    setMessageDrafts((current) => {
      const previous = current[conversationId] ?? "";
      const next = typeof value === "function" ? (value as (prevState: string) => string)(previous) : value;
      if (next === previous) {
        return current;
      }
      if (!next.trim()) {
        if (!(conversationId in current)) {
          return current;
        }
        const compact = { ...current };
        delete compact[conversationId];
        return compact;
      }
      return {
        ...current,
        [conversationId]: next
      };
    });
  }, []);
  const currentMember = useMemo(() => {
    if (!user) return null;
    return members.find((member) => member.userId === user.user_id) ?? null;
  }, [members, user]);
  const canSendCurrentConversation = useMemo(() => {
    if (!canSendToConversation(selectedConversation, members, friends)) {
      return false;
    }
    if (!selectedConversation) {
      return false;
    }
    if (isMemberMuted(currentMember)) {
      return false;
    }
    if (
      selectedConversation.type === "GROUP" &&
      selectedConversation.muteAll &&
      currentMember?.role !== "OWNER" &&
      currentMember?.role !== "ADMIN"
    ) {
      return false;
    }
    return true;
  }, [currentMember, friends, members, selectedConversation]);

  const filteredConversations = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return conversations;
    return conversations.filter((item) => {
      const target = `${item.title} ${item.conversationId} ${item.type}`.toLowerCase();
    return target.includes(keyword);
    });
  }, [conversations, search]);

  const showToast = useCallback((message: string, tone: ToastTone = "info") => {
    setToast({ tone, message });
    window.setTimeout(() => {
      setToast(null);
    }, 2800);
  }, []);
  const handleDeleteMessageLocal = useCallback(
    (message: MessageInfo) => {
      if (!message || message.id <= 0 || message.pending) {
        return;
      }
      const previousScrollTop = messageListRef.current?.scrollTop ?? null;
      const conversationId = message.conversationId;
      const messageId = message.id;
      setHiddenMessageIdsByConversation((current) => {
        const previous = current[conversationId] ?? [];
        if (previous.includes(messageId)) {
          return current;
        }
        const merged = [...previous, messageId];
        return {
          ...current,
          [conversationId]: merged.slice(-maxHiddenMessageIdsPerConversation)
        };
      });
      suppressAutoScrollOnceRef.current = true;
      setMessages((current) => current.filter((item) => !(item.conversationId === conversationId && item.id === messageId)));
      setReplyingTo((current) => (current?.messageId === messageId ? null : current));
      if (previousScrollTop !== null) {
        window.requestAnimationFrame(() => {
          window.requestAnimationFrame(() => {
            const scroller = messageListRef.current;
            if (!scroller) {
              return;
            }
            const maxScrollTop = Math.max(0, scroller.scrollHeight - scroller.clientHeight);
            scroller.scrollTop = Math.min(previousScrollTop, maxScrollTop);
          });
        });
      }
      showToast("已在本地隐藏该消息", "success");
    },
    [showToast]
  );
  const shouldAutoScrollOnMessagesChange = useCallback(() => {
    if (suppressAutoScrollOnceRef.current) {
      suppressAutoScrollOnceRef.current = false;
      return false;
    }
    return true;
  }, []);

  const refreshSessions = useCallback(async () => {
    const data = await api.sessions();
    setSessions(data);
  }, []);

  const refreshFriends = useCallback(async () => {
    const [groups, friendList, requests] = await Promise.all([api.friendGroups(), api.friends(), api.friendRequests()]);
    setFriendGroups(groups);
    setFriends(sortFriends(friendList));
    setFriendRequests(sortFriendRequests(requests));
    return { groups, friends: friendList, requests };
  }, []);

  const refreshConversations = useCallback(async () => {
    const data = await api.conversations();
    const sorted = sortConversations(data);
    setConversations(sorted);
    setSelectedConversationId((current) => {
      if (current && sorted.some((item) => item.conversationId === current)) {
        return current;
      }
      return sorted[0]?.conversationId ?? null;
    });
    return sorted;
  }, []);

  const refreshNotifications = useCallback(async () => {
    const data = await api.notifications({ limit: 8 });
    setNotifications(data.notifications);
    setNotificationUnreadCount(data.unreadCount);
    return data;
  }, []);

  const syncFriendStateFromRealtime = useCallback(
    async (options?: { refreshConversations?: boolean }) => {
      try {
        await refreshFriends();
        if (options?.refreshConversations) {
          await refreshConversations();
        }
      } catch (error) {
        showToast(errorMessage(error), "error");
      }
    },
    [refreshConversations, refreshFriends, showToast]
  );

  const { markConversationRead, refreshCurrentConversationMessages, recoverRealtimeState } = useRealtimeState({
    selectedConversationIdRef,
    messageListRef,
    realtimeRecoveringRef,
    lastMarkedReadRef,
    refreshConversations,
    showToast,
    setMessages,
    setUnreadCounts,
    filterVisibleMessages: filterLocallyHiddenMessages
  });

  const refreshSelectedGroupInfo = useCallback(async (conversationId: string) => {
    const data = await api.groupInfo(conversationId);
    if (selectedConversationIdRef.current === conversationId) {
      setSelectedGroupInfo(data);
    }
    return data;
  }, []);

  const { notificationStatus, notificationsEnabled, showMessageNotification, handleToggleNotifications } =
    useNotificationDomain({
      user,
      showToast
    });

  const { handleOpenChatWithFriend, handleMention, handleReplyMessage, applyRecalledMessageEvent } =
    useChatInteractionDomain({
      setConversations,
      setSelectedConversationId,
      setMobilePane,
      showToast,
      setMessageDraft,
      composerRef,
      setReplyingTo,
      setMessages
    });

  const baseSocketEventHandler = useMemo(
    () =>
      buildRealtimeEventHandler({
        user,
        selectedConversationIdRef,
        pendingMessagesRef,
        messageListRef,
        markConversationRead,
        refreshSelectedGroupInfo,
        showMessageNotification,
        showToast,
        refreshConversations,
        syncFriendStateFromRealtime,
        applyRecalledMessageEvent,
        setMessages,
        setUnreadCounts,
        setConversations,
        setNotifications,
        setNotificationUnreadCount
      }),
    [
      applyRecalledMessageEvent,
      markConversationRead,
      refreshSelectedGroupInfo,
      refreshConversations,
      showMessageNotification,
      showToast,
      syncFriendStateFromRealtime,
      user
    ]
  );

  const handleSocketEvent = useCallback(
    (raw: string) => {
      try {
        const event = JSON.parse(raw) as { type?: string; clientMsgId?: string; data?: unknown };
        const eventType = String(event.type ?? "").toUpperCase();
        if (eventType === "TYPING") {
          const data = event.data as Partial<TypingEventData> | undefined;
          const conversationId = typeof data?.conversationId === "string" ? data.conversationId : "";
          const userId = typeof data?.userId === "number" ? data.userId : 0;
          const isTyping = Boolean(data?.isTyping);
          if (!conversationId || userId <= 0 || userId === user?.user_id) {
            return;
          }
          setTypingByConversation((current) => {
            if (!isTyping) {
              if (!current[conversationId] || current[conversationId].userId !== userId) {
                return current;
              }
              const next = { ...current };
              delete next[conversationId];
              return next;
            }
            return {
              ...current,
              [conversationId]: {
                conversationId,
                userId,
                isTyping: true,
                at: Date.now()
              }
            };
          });
          return;
        }

        if (eventType === "NEW_MESSAGE") {
          const incoming = event.data as Partial<MessageInfo> | undefined;
          const conversationId = typeof incoming?.conversationId === "string" ? incoming.conversationId : "";
          const senderId = typeof incoming?.senderId === "number" ? incoming.senderId : 0;
          if (conversationId && senderId > 0) {
            setTypingByConversation((current) => {
              const typing = current[conversationId];
              if (!typing || typing.userId !== senderId) {
                return current;
              }
              const next = { ...current };
              delete next[conversationId];
              return next;
            });
          }
        }

        if (eventType === "BOT_REPLY_STREAM") {
          const data = event.data as Partial<BotReplyStreamData> | undefined;
          const conversationId = typeof data?.conversationId === "string" ? data.conversationId : "";
          const senderId = typeof data?.senderId === "number" ? data.senderId : 0;
          const messageType = typeof data?.messageType === "string" ? data.messageType : "BOT_REPLY";
          const senderType = typeof data?.senderType === "string" ? data.senderType : "BOT";
          const content = typeof data?.content === "string" ? data.content : "";
          const done = Boolean(data?.done);
          if (!conversationId || senderId <= 0) {
            return;
          }
          setMessages((current) => {
            const next = current.filter((item) => !(done && item.conversationId === conversationId && item.isBotGenerating));
            if (done) {
              return next;
            }
            const existingIndex = next.findIndex((item) => item.conversationId === conversationId && item.isBotGenerating);
            const streamMessage: MessageInfo = {
              id:
                existingIndex >= 0
                  ? next[existingIndex].id
                  : Math.max(
                      ...next.filter((item) => item.conversationId === conversationId).map((item) => item.id),
                      0
                    ) + 1,
              conversationId,
              senderId,
              senderType,
              messageType,
              content,
              status: "NORMAL",
              createdAt: Math.floor(Date.now() / 1000),
              isBotGenerating: true
            };
            if (existingIndex >= 0) {
              const updated = [...next];
              updated[existingIndex] = streamMessage;
              return sortMessages(updated);
            }
            return sortMessages([...next, streamMessage]);
          });
          if (selectedConversationIdRef.current === conversationId) {
            window.setTimeout(() => scrollMessagesToBottom(messageListRef), 20);
          }
          return;
        }
      } catch {
        // ignore parse errors and let base handler decide
      }
      baseSocketEventHandler(raw);
    },
    [baseSocketEventHandler, messageListRef, user?.user_id]
  );

  const { wsStatus, socketRef } = useRealtimeConnection({
    user,
    wsReconnectDelays,
    onMessage: handleSocketEvent,
    onRecover: recoverRealtimeState
  });

  const emitTypingEvent = useCallback(
    (conversationId: string, isTyping: boolean) => {
      if (!conversationId || !user) {
        return;
      }
      const socket = socketRef.current;
      if (!socket || socket.readyState !== WebSocket.OPEN) {
        return;
      }
      socket.send(
        JSON.stringify({
          type: "TYPING",
          data: {
            conversationId,
            isTyping,
            userId: user.user_id,
            at: Date.now()
          }
        })
      );
    },
    [socketRef, user]
  );

  const clearLocalTypingTimers = useCallback(() => {
    if (typingHeartbeatRef.current) {
      window.clearInterval(typingHeartbeatRef.current);
      typingHeartbeatRef.current = 0;
    }
    if (typingIdleTimerRef.current) {
      window.clearTimeout(typingIdleTimerRef.current);
      typingIdleTimerRef.current = 0;
    }
  }, []);

  const stopLocalTyping = useCallback(() => {
    const currentConversationId = localTypingConversationRef.current;
    if (currentConversationId) {
      emitTypingEvent(currentConversationId, false);
    }
    localTypingConversationRef.current = null;
    clearLocalTypingTimers();
  }, [clearLocalTypingTimers, emitTypingEvent]);

  const canEmitTypingInConversation = useCallback(
    (conversationId: string | null) => Boolean(conversationId && selectedConversation?.type === "SINGLE" && canSendCurrentConversation),
    [canSendCurrentConversation, selectedConversation?.type]
  );

  const keepLocalTyping = useCallback(
    (conversationId: string, draftText: string) => {
      if (!canEmitTypingInConversation(conversationId) || !draftText.trim()) {
        stopLocalTyping();
        return;
      }

      if (localTypingConversationRef.current !== conversationId) {
        stopLocalTyping();
        localTypingConversationRef.current = conversationId;
        emitTypingEvent(conversationId, true);
      }

      if (!typingHeartbeatRef.current) {
        typingHeartbeatRef.current = window.setInterval(() => {
          if (!canEmitTypingInConversation(conversationId) || !messageDraft.trim()) {
            stopLocalTyping();
            return;
          }
          if (localTypingConversationRef.current === conversationId) {
            emitTypingEvent(conversationId, true);
          }
        }, typingHeartbeatMs);
      }

      if (typingIdleTimerRef.current) {
        window.clearTimeout(typingIdleTimerRef.current);
      }
      typingIdleTimerRef.current = window.setTimeout(() => {
        if (localTypingConversationRef.current === conversationId) {
          stopLocalTyping();
        }
      }, typingIdleMs);
    },
    [canEmitTypingInConversation, emitTypingEvent, messageDraft, stopLocalTyping]
  );

  const handleDraftChange = useCallback(
    (next: string) => {
      setMessageDraft(next);
      const conversationId = selectedConversationIdRef.current;
      if (!conversationId || !next.trim()) {
        stopLocalTyping();
        return;
      }
      keepLocalTyping(conversationId, next);
    },
    [keepLocalTyping, stopLocalTyping]
  );

  useEffect(
    () => () => {
      clearLocalTypingTimers();
    },
    [clearLocalTypingTimers]
  );

  useEffect(() => {
    if (!user || wsStatus !== "open") {
      stopLocalTyping();
    }
  }, [stopLocalTyping, user, wsStatus]);

  useEffect(() => {
    const currentConversationId = selectedConversationIdRef.current;
    if (!currentConversationId || !canEmitTypingInConversation(currentConversationId) || !messageDraft.trim()) {
      stopLocalTyping();
      return;
    }
    keepLocalTyping(currentConversationId, messageDraft);
  }, [canEmitTypingInConversation, keepLocalTyping, messageDraft, selectedConversationId, stopLocalTyping]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      setTypingByConversation((current) => {
        const now = Date.now();
        let changed = false;
        const next: Record<string, TypingEventData> = {};
        for (const [conversationId, entry] of Object.entries(current)) {
          const at = typeof entry.at === "number" ? entry.at : 0;
          if (entry.isTyping && now-at <= typingExpireMs) {
            next[conversationId] = entry;
            continue;
          }
          changed = true;
        }
        return changed ? next : current;
      });
    }, 1000);
    return () => {
      window.clearInterval(timer);
    };
  }, []);

  const peerTypingText = useMemo(() => {
    if (!selectedConversationId || selectedConversation?.type !== "SINGLE" || !user) {
      return "";
    }
    const typing = typingByConversation[selectedConversationId];
    if (!typing || !typing.isTyping || typing.userId === user.user_id) {
      return "";
    }
    if (typeof typing.at !== "number" || Date.now() - typing.at > typingExpireMs) {
      return "";
    }
    return "对方正在输入中...";
  }, [selectedConversation?.type, selectedConversationId, typingByConversation, user]);

  const { bootstrap, handleLogin, handleRegister, handleLogout } = useSessionFlow({
    socketRef,
    setBusyAction,
    setBooting,
    setUser,
    setFriendGroups,
    setFriends,
    setFriendRequests,
    setMessages,
    setMembers,
    setConversations,
    setUnreadCounts,
    setSelectedConversationId,
    setSessions,
    refreshConversations,
    refreshFriends,
    refreshSessions,
    showToast
  });

  useConversationLifecycle({
    bootstrap,
    selectedConversationId,
    selectedConversationType: selectedConversation?.type ?? null,
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
    filterVisibleMessages: filterLocallyHiddenMessages,
    shouldAutoScrollOnMessagesChange
  });

  useEffect(() => {
    if (!user) {
      setMessageDrafts({});
      setNotifications([]);
      setNotificationUnreadCount(0);
      return;
    }
    void refreshNotifications().catch(() => undefined);
  }, [refreshNotifications, user]);

  const conversationDomainDeps = useMemo(
    () =>
      createConversationDomainDeps({
        selectedConversationId,
        selectedConversationType: selectedConversation?.type ?? null,
        selectedConversationIdRef,
        setBusyAction,
        refreshConversations,
        refreshCurrentConversationMessages,
        refreshSelectedGroupInfo,
        setSelectedConversationId,
        setMobilePane,
        setMembers,
        setSelectedGroupInfo,
        showToast
      }),
    [
      selectedConversationId,
      selectedConversation?.type,
      selectedConversationIdRef,
      refreshConversations,
      refreshCurrentConversationMessages,
      refreshSelectedGroupInfo,
      showToast
    ]
  );

  const authDomainDeps = useMemo(
    () =>
      createAuthDomainDeps({
        setBusyAction,
        refreshSessions,
        handleLogout,
        setUser,
        setMembers,
        showToast
      }),
    [refreshSessions, handleLogout, showToast]
  );

  const friendDomainDeps = useMemo(
    () =>
      createFriendDomainDeps({
        setBusyAction,
        refreshFriends,
        refreshConversations,
        conversations,
        setSelectedConversationId,
        setMobilePane,
        setDetailTab,
        setFriends,
        showToast
      }),
    [conversations, refreshConversations, refreshFriends, showToast]
  );

  const botDomainDeps = useMemo(
    () =>
      createBotDomainDeps({
        selectedConversationId,
        aiCallLogStatus,
        setBusyAction,
        setAvailableBots,
        setConversationBots,
        setAICallLogs,
        setAICallLogQuota,
        setLoadingAICallLogs,
        showToast
      }),
    [aiCallLogStatus, selectedConversationId, showToast]
  );

  const handleCreateGroup = async (input: { name: string; announcement: string; joinPolicy: string }) =>
    createGroupAction(input, conversationDomainDeps);

  const handleJoinGroup = async (conversationId: string) =>
    joinGroupAction(conversationId, conversationDomainDeps);

  const handleLeaveGroup = async () =>
    leaveGroupAction(conversationDomainDeps);

  const handleInviteMember = async (targetUserId: number) =>
    inviteMemberAction(targetUserId, conversationDomainDeps);

  const handleTransferOwner = async (targetUserId: number) =>
    transferOwnerAction(targetUserId, conversationDomainDeps);

  const handleSetAdmin = async (targetUserId: number) =>
    setAdminAction(targetUserId, conversationDomainDeps);

  const handleRemoveAdmin = async (targetUserId: number) =>
    removeAdminAction(targetUserId, conversationDomainDeps);

  const handleMuteMember = async (targetUserId: number, durationSeconds: number) =>
    muteMemberAction(targetUserId, durationSeconds, conversationDomainDeps);

  const handleUnmuteMember = async (targetUserId: number) =>
    unmuteMemberAction(targetUserId, conversationDomainDeps);

  const handleRemoveMember = async (targetUserId: number) =>
    removeMemberAction(targetUserId, conversationDomainDeps);

  const handleSetGroupMuteAll = async (muteAll: boolean) =>
    setGroupMuteAllAction(muteAll, conversationDomainDeps);

  const handleUpdateGroupAnnouncement = async (announcement: string) =>
    updateGroupAnnouncementAction(announcement, conversationDomainDeps);

  const { handleRecallMessage, handleSendMessage, handleLoadOlder } = useMessageActions({
    user,
    selectedConversationId,
    selectedConversationType: selectedConversation?.type ?? null,
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
    setLoadingOlder,
    filterVisibleMessages: filterLocallyHiddenMessages
  });

  const handleRevokeSession = async (sessionId: string, password: string) =>
    revokeSessionAction(sessionId, password, authDomainDeps);

  const handleLogoutAll = async (password: string) =>
    logoutAllAction(password, authDomainDeps);

  const handleAvatarUpload = async (avatar: Blob) =>
    uploadAvatarAction(avatar, authDomainDeps);

  const handleCreateFriendGroup = async (name: string) =>
    createFriendGroupAction(name, friendDomainDeps);

  const handleAddFriend = async (input: { targetAimId: string; remark: string; groupId: number | null }) =>
    addFriendAction(input, friendDomainDeps);

  const handleRespondFriendRequest = async (requestId: number, action: "ACCEPTED" | "REJECTED") =>
    respondFriendRequestAction(requestId, action, friendDomainDeps);

  const handleUpdateFriend = async (friendUserId: number, input: { remark: string; groupId: number | null }) =>
    updateFriendAction(friendUserId, input, friendDomainDeps);

  const handleDeleteFriend = async (friendUserId: number) =>
    deleteFriendAction(friendUserId, friendDomainDeps);

  const handleMarkNotificationRead = async (notificationId: number) => {
    const current = notifications.find((item) => item.id === notificationId);
    if (current?.persistent === false) {
      setNotifications((items) => items.map((item) => (item.id === notificationId ? { ...item, isRead: true } : item)));
      if (!current.isRead) {
        setNotificationUnreadCount((count) => Math.max(0, count - 1));
      }
      return;
    }
    await api.markNotificationRead(notificationId);
    await refreshNotifications();
  };

  const handleMarkAllNotificationsRead = async () => {
    setNotifications((items) => items.map((item) => (item.persistent === false ? { ...item, isRead: true } : item)));
    await api.markAllNotificationsRead();
    await refreshNotifications();
  };

  const { refreshAICallLogs, handleAddBot, handleRemoveBot } = useBotPanelDomain({
    detailTab,
    selectedConversationId,
    selectedConversationType: selectedConversation?.type ?? null,
    botDomainDeps,
    setAvailableBots,
    setConversationBots,
    setAICallLogs,
    setAICallLogQuota,
    setLoadingAICallLogs,
    showToast
  });

  const refreshConversationKnowledgeBases = useCallback(async () => {
    if (!selectedConversationId) {
      setConversationKnowledgeBases([]);
      return [];
    }
    const data = await api.listConversationKnowledgeBases(selectedConversationId);
    setConversationKnowledgeBases(data);
    setKnowledgeBases((current) => {
      const map = new Map<number, KnowledgeBaseInfo>();
      for (const item of current) {
        map.set(item.knowledgeBaseId, item);
      }
      for (const item of data) {
        map.set(item.knowledgeBaseId, {
          knowledgeBaseId: item.knowledgeBaseId,
          name: item.name,
          description: item.description,
          status: item.status
        });
      }
      return Array.from(map.values());
    });
    return data;
  }, [selectedConversationId]);

  const refreshKnowledgeDocuments = useCallback(async () => {
    if (!selectedKnowledgeBaseId) {
      setKnowledgeDocuments([]);
      return [];
    }
    const data = await api.listKnowledgeDocuments(selectedKnowledgeBaseId);
    setKnowledgeDocuments(data);
    return data;
  }, [selectedKnowledgeBaseId]);

  const loadKnowledgePanelData = useCallback(async () => {
    setLoadingKnowledge(true);
    try {
      const list = await api.listKnowledgeBases();
      setKnowledgeBases(list);
      const tasks: Array<Promise<unknown>> = [];
      if (selectedConversationId) {
        tasks.push(refreshConversationKnowledgeBases());
      } else {
        setConversationKnowledgeBases([]);
      }
      if (selectedKnowledgeBaseId) {
        tasks.push(refreshKnowledgeDocuments());
      } else {
        setKnowledgeDocuments([]);
      }
      await Promise.all(tasks);
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setLoadingKnowledge(false);
    }
  }, [refreshConversationKnowledgeBases, refreshKnowledgeDocuments, selectedConversationId, selectedKnowledgeBaseId, showToast]);

  useEffect(() => {
    if (detailTab !== "knowledge") {
      return;
    }
    void loadKnowledgePanelData();
  }, [detailTab, loadKnowledgePanelData]);

  useEffect(() => {
    if (!selectedConversationId) {
      setConversationKnowledgeBases([]);
      return;
    }
    if (detailTab === "knowledge") {
      void refreshConversationKnowledgeBases();
    }
  }, [detailTab, refreshConversationKnowledgeBases, selectedConversationId]);

  useEffect(() => {
    if (!selectedKnowledgeBaseId) {
      setKnowledgeDocuments([]);
      setKnowledgeSearchChunks([]);
      return;
    }
    if (detailTab === "knowledge") {
      void refreshKnowledgeDocuments();
    }
  }, [detailTab, refreshKnowledgeDocuments, selectedKnowledgeBaseId]);

  const handleCreateKnowledgeBase = async (input: { name: string; description: string }) => {
    setKnowledgeBusy(true);
    try {
      const created = await api.createKnowledgeBase(input);
      setKnowledgeBases((current) => {
        const map = new Map<number, KnowledgeBaseInfo>();
        for (const item of current) {
          map.set(item.knowledgeBaseId, item);
        }
        map.set(created.knowledgeBaseId, created);
        return Array.from(map.values());
      });
      setSelectedKnowledgeBaseId(created.knowledgeBaseId);
      showToast("知识库创建成功", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setKnowledgeBusy(false);
    }
  };

  const handleAddKnowledgeDocument = async (input: {
    knowledgeBaseId: number;
    title: string;
    sourceType?: "TEXT" | "MARKDOWN";
    content?: string;
    file?: File | null;
  }) => {
    setKnowledgeBusy(true);
    try {
      let created: KnowledgeDocumentInfo;
      if (input.file) {
        created = await api.addKnowledgeDocumentFile(input.knowledgeBaseId, {
          title: input.title,
          file: input.file
        });
      } else {
        created = await api.addKnowledgeDocumentText(input.knowledgeBaseId, {
          title: input.title,
          sourceType: input.sourceType ?? "TEXT",
          content: input.content ?? ""
        });
      }
      if (selectedKnowledgeBaseId === input.knowledgeBaseId) {
        await refreshKnowledgeDocuments();
      }
      if (created.status === "READY") {
        showToast("文档导入成功", "success");
      } else {
        showToast(`文档已提交，当前状态：${knowledgeDocumentStatusLabel(created.status)}`, "success");
      }
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setKnowledgeBusy(false);
    }
  };

  const handleSearchKnowledgeBase = async (input: { knowledgeBaseId: number; query: string; topK: number }) => {
    setKnowledgeBusy(true);
    try {
      const data = await api.searchKnowledgeBase(input.knowledgeBaseId, {
        query: input.query,
        topK: input.topK
      });
      setKnowledgeSearchChunks(data);
    } catch (error) {
      setKnowledgeSearchChunks([]);
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setKnowledgeBusy(false);
    }
  };

  const handleDeleteKnowledgeDocument = async (knowledgeBaseId: number, documentId: number) => {
    setKnowledgeBusy(true);
    try {
      await api.deleteKnowledgeDocument(knowledgeBaseId, documentId);
      if (selectedKnowledgeBaseId === knowledgeBaseId) {
        await refreshKnowledgeDocuments();
      }
      showToast("文档已删除", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setKnowledgeBusy(false);
    }
  };

  const handleBindConversationKnowledgeBase = async (knowledgeBaseId: number) => {
    if (!selectedConversationId) return;
    setKnowledgeBusy(true);
    try {
      await api.bindConversationKnowledgeBase(selectedConversationId, knowledgeBaseId);
      await refreshConversationKnowledgeBases();
      showToast("已绑定知识库", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setKnowledgeBusy(false);
    }
  };

  const handleUnbindConversationKnowledgeBase = async (knowledgeBaseId: number) => {
    if (!selectedConversationId) return;
    setKnowledgeBusy(true);
    try {
      await api.unbindConversationKnowledgeBase(selectedConversationId, knowledgeBaseId);
      await refreshConversationKnowledgeBases();
      showToast("已解绑知识库", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setKnowledgeBusy(false);
    }
  };

  useEffect(() => {
    if (selectedKnowledgeBaseId) {
      return;
    }
    const first = conversationKnowledgeBases[0];
    if (first) {
      setSelectedKnowledgeBaseId(first.knowledgeBaseId);
    }
  }, [conversationKnowledgeBases, selectedKnowledgeBaseId]);

  if (booting) {
    return (
      <div className="boot-screen">
        <div className="brand-mark">A</div>
        <Loader2 className="spin" size={24} />
      </div>
    );
  }

  if (!user) {
    return (
      <>
        <AuthView busy={busyAction} onLogin={handleLogin} onRegister={handleRegister} />
        <Toast toast={toast} onClose={() => setToast(null)} />
      </>
    );
  }

  return (
    <div className="app-shell">
      <ConversationPanel
        active={mobilePane === "conversations"}
        busy={busyAction}
        conversations={filteredConversations}
        notifications={notifications}
        notificationUnreadCount={notificationUnreadCount}
        unreadCounts={unreadCounts}
        rawConversationCount={conversations.length}
        currentUser={user}
        selectedConversationId={selectedConversationId}
        search={search}
        onSearch={setSearch}
        onCreateGroup={handleCreateGroup}
        onJoinGroup={handleJoinGroup}
        onRefresh={async () => {
          const data = await refreshConversations();
          await refreshNotifications();
          return data;
        }}
        onMarkNotificationRead={handleMarkNotificationRead}
        onMarkAllNotificationsRead={handleMarkAllNotificationsRead}
        onSelect={(id) => {
          setUnreadCounts((current) => ({ ...current, [id]: 0 }));
          setSelectedConversationId(id);
          setMobilePane("chat");
        }}
      />

      <ChatPanel
        active={mobilePane === "chat"}
        conversation={selectedConversation}
        currentUser={user}
        currentMember={currentMember}
        members={members}
        loading={loadingMessages}
        loadingOlder={loadingOlder}
        messages={visibleMessages}
        messageDraft={messageDraft}
        replyingTo={replyingTo}
        peerTypingText={peerTypingText}
        wsStatus={wsStatus}
        busy={busyAction}
        messageListRef={messageListRef}
        composerRef={composerRef}
        canSend={canSendCurrentConversation}
        onBack={() => setMobilePane("conversations")}
        onDraftChange={handleDraftChange}
        onLoadOlder={handleLoadOlder}
        onLeaveGroup={handleLeaveGroup}
        onInviteMember={handleInviteMember}
        onOpenMembers={() => {
          setDetailTab("members");
          setMobilePane("members");
        }}
        onMention={handleMention}
        onReplySelect={handleReplyMessage}
        onRecallMessage={handleRecallMessage}
        onDeleteLocalMessage={handleDeleteMessageLocal}
        onCancelReply={() => setReplyingTo(null)}
        onSend={handleSendMessage}
      />

      <DetailPanel
        active={
          mobilePane === "friends" ||
          mobilePane === "members" ||
          mobilePane === "bots" ||
          mobilePane === "account"
        }
        tab={detailTab}
        user={user}
        friendGroups={friendGroups}
        friends={friends}
        friendRequests={friendRequests}
        members={members}
        conversations={conversations}
        sessions={sessions}
        busy={busyAction}
        wsStatus={wsStatus}
        notificationStatus={notificationStatus}
        notificationsEnabled={notificationsEnabled}
        selectedConversationId={selectedConversationId}
        selectedConversation={selectedConversation}
        selectedConversationType={selectedConversation?.type ?? null}
        selectedGroupInfo={selectedGroupInfo}
        currentMember={currentMember}
        availableBots={availableBots}
        conversationBots={conversationBots}
        aiCallLogs={aiCallLogs}
        aiCallLogQuota={aiCallLogQuota}
        loadingAICallLogs={loadingAICallLogs}
        aiCallLogStatus={aiCallLogStatus}
        knowledgeBases={knowledgeBases}
        selectedKnowledgeBaseId={selectedKnowledgeBaseId}
        knowledgeDocuments={knowledgeDocuments}
        knowledgeSearchChunks={knowledgeSearchChunks}
        conversationKnowledgeBases={conversationKnowledgeBases}
        loadingKnowledge={loadingKnowledge}
        knowledgeBusy={knowledgeBusy}
        onTabChange={setDetailTab}
        onCreateFriendGroup={handleCreateFriendGroup}
        onAddFriend={handleAddFriend}
        onRespondFriendRequest={handleRespondFriendRequest}
        onUpdateFriend={handleUpdateFriend}
        onDeleteFriend={handleDeleteFriend}
        onOpenChatWithFriend={handleOpenChatWithFriend}
        onRefreshSessions={refreshSessions}
        onLogout={handleLogout}
        onLogoutAll={handleLogoutAll}
        onAvatarUpload={handleAvatarUpload}
        onRevokeSession={handleRevokeSession}
        onToggleNotifications={handleToggleNotifications}
        onTransferOwner={handleTransferOwner}
        onSetAdmin={handleSetAdmin}
        onRemoveAdmin={handleRemoveAdmin}
        onMuteMember={handleMuteMember}
        onUnmuteMember={handleUnmuteMember}
        onRemoveMember={handleRemoveMember}
        onSetGroupMuteAll={handleSetGroupMuteAll}
        onUpdateGroupAnnouncement={handleUpdateGroupAnnouncement}
        onAddBot={handleAddBot}
        onRemoveBot={handleRemoveBot}
        onAICallLogStatusChange={setAICallLogStatus}
        onRefreshAICallLogs={refreshAICallLogs}
        onSelectKnowledgeBase={setSelectedKnowledgeBaseId}
        onCreateKnowledgeBase={handleCreateKnowledgeBase}
        onAddKnowledgeDocument={handleAddKnowledgeDocument}
        onSearchKnowledgeBase={handleSearchKnowledgeBase}
        onDeleteKnowledgeDocument={handleDeleteKnowledgeDocument}
        onBindConversationKnowledgeBase={handleBindConversationKnowledgeBase}
        onUnbindConversationKnowledgeBase={handleUnbindConversationKnowledgeBase}
        onRefreshKnowledgePanelData={loadKnowledgePanelData}
        onMention={handleMention}
        onClose={() => setMobilePane(selectedConversation ? "chat" : "conversations")}
      />

      <MobileNav
        active={mobilePane}
        hasConversation={Boolean(selectedConversation)}
        onChange={(pane) => {
          if (pane === "friends" || pane === "members" || pane === "bots" || pane === "account") {
            setDetailTab(pane);
          }
          setMobilePane(pane);
        }}
      />

      <Toast toast={toast} onClose={() => setToast(null)} />
    </div>
  );
}





export default App;












