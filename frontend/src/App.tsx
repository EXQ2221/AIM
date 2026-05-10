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
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState
} from "react";
import { APIError, api } from "./api";
import { AvatarUploader } from "./app/avatar-uploader";
import {
  type AuthMode,
  type BrowserNotificationStatus,
  type DetailTab,
  type PendingMessageEntry,
  type ToastState,
  type ToastTone,
  type WsStatus,
  joinPolicies,
  wsReconnectDelays
} from "./app/types";
import {
  appendMentionToDraft,
  buildReplyPreview,
  canSendToConversation,
  conversationPreview,
  cx,
  errorMessage,
  formatRelative,
  getNotificationStatus,
  handleAvatarMention,
  messageText,
  mergeMessagesById,
  parseSystemMessageContent,
  parseGroupValue,
  reconcilePendingMessage,
  roleLabel,
  scrollMessagesToBottom,
  sortConversations,
  sortFriendRequests,
  sortFriends,
  sortMessages,
  statusLabel,
  truncateNotificationBody
} from "./app/utils";
import { Avatar, Field, IconButton, MessageBubble, MobileNav, StatusPill, Toast, WsBadge } from "./app/ui";
import type {
  AICallLogInfo,
  AICallLogQuotaInfo,
  BotInfo,
  ConversationInfo,
  FriendGroupInfo,
  FriendInfo,
  FriendRequestInfo,
  GroupInfo,
  MemberInfo,
  MessageInfo,
  MessageRecalledEventInfo,
  MobilePane,
  OutgoingMessagePayload,
  ReplyPreviewInfo,
  SessionInfo,
  UserInfo,
  WebSocketEvent
} from "./types";

const NOTIFICATION_PREFERENCE_KEY = "aim:notifications:enabled";
const RECALLED_MESSAGE_PLACEHOLDER = "消息已撤回";

function loadNotificationPreference() {
  if (typeof window === "undefined") return true;
  return window.localStorage.getItem(NOTIFICATION_PREFERENCE_KEY) !== "off";
}

function saveNotificationPreference(enabled: boolean) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(NOTIFICATION_PREFERENCE_KEY, enabled ? "on" : "off");
}

function latestMessageId(messages: MessageInfo[]) {
  return messages.reduce((max, message) => (message.id > max && !message.pending ? message.id : max), 0);
}

function readReceiptLabel(conversation: ConversationInfo | null, message: MessageInfo, mine: boolean) {
  if (!conversation || !mine || message.pending || message.status === "FAILED") {
    return undefined;
  }
  if (conversation.type === "SINGLE") {
    return message.readByPeer ? "Read" : "Unread";
  }
  if (conversation.type === "GROUP" && typeof message.readCount === "number") {
    return `${message.readCount} read`;
  }
  return undefined;
}

function applyMessageRecalled(messages: MessageInfo[], event: MessageRecalledEventInfo) {
  return mergeMessagesById(
    [],
    messages.map((message) => {
      const next =
        message.id === event.messageId
          ? {
              ...message,
              content: "",
              pending: false,
              status: "RECALLED"
            }
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

function applyConversationRecalled(conversations: ConversationInfo[], event: MessageRecalledEventInfo) {
  return sortConversations(
    conversations.map((conversation) =>
      conversation.conversationId === event.conversationId && conversation.lastMessageId === event.messageId
        ? {
            ...conversation,
            lastMessageContent: RECALLED_MESSAGE_PLACEHOLDER
          }
        : conversation
    )
  );
}

function isMemberMuted(member: Pick<MemberInfo, "muteUntil"> | null | undefined) {
  return typeof member?.muteUntil === "number" && member.muteUntil > Math.floor(Date.now() / 1000);
}

function formatMuteUntil(value?: number | null) {
  if (!value) return "";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value * 1000));
}

function App() {
  const [booting, setBooting] = useState(true);
  const [user, setUser] = useState<UserInfo | null>(null);
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [friendGroups, setFriendGroups] = useState<FriendGroupInfo[]>([]);
  const [friends, setFriends] = useState<FriendInfo[]>([]);
  const [friendRequests, setFriendRequests] = useState<FriendRequestInfo[]>([]);
  const [conversations, setConversations] = useState<ConversationInfo[]>([]);
  const [selectedConversationId, setSelectedConversationId] = useState<string | null>(null);
  const [selectedGroupInfo, setSelectedGroupInfo] = useState<GroupInfo | null>(null);
  const [members, setMembers] = useState<MemberInfo[]>([]);
  const [messages, setMessages] = useState<MessageInfo[]>([]);
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  const [messageDraft, setMessageDraft] = useState("");
  const [replyingTo, setReplyingTo] = useState<ReplyPreviewInfo | null>(null);
  const [search, setSearch] = useState("");
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [loadingOlder, setLoadingOlder] = useState(false);
  const [busyAction, setBusyAction] = useState(false);
  const [toast, setToast] = useState<ToastState>(null);
  const [wsStatus, setWsStatus] = useState<WsStatus>("closed");
  const [notificationStatus, setNotificationStatus] = useState<BrowserNotificationStatus>(() => getNotificationStatus());
  const [notificationsEnabled, setNotificationsEnabled] = useState(() => loadNotificationPreference());
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

  const socketRef = useRef<WebSocket | null>(null);
  const selectedConversationIdRef = useRef<string | null>(null);
  const messageListRef = useRef<HTMLDivElement | null>(null);
  const composerRef = useRef<HTMLTextAreaElement | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef(0);
  const connectNowRef = useRef<(() => void) | null>(null);
  const realtimeRecoveringRef = useRef(false);
  const pendingMessagesRef = useRef(new Map<string, PendingMessageEntry>());
  const lastMarkedReadRef = useRef(new Map<string, number>());
  const handleSocketEventRef = useRef<(raw: string) => void>(() => {});
  const recoverRealtimeStateRef = useRef<() => Promise<void>>(async () => {});

  const selectedConversation = useMemo(
    () => conversations.find((item) => item.conversationId === selectedConversationId) ?? null,
    [conversations, selectedConversationId]
  );
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
  }, []);

  const refreshCurrentConversationMessages = useCallback(async () => {
    const conversationID = selectedConversationIdRef.current;
    if (!conversationID) return;

    const nextMessages = await api.messages(conversationID, { limit: 50 });
    if (selectedConversationIdRef.current !== conversationID) return;

    setMessages((current) => mergeMessagesById(current, nextMessages));
    setUnreadCounts((current) => ({ ...current, [conversationID]: 0 }));
    window.setTimeout(() => scrollMessagesToBottom(messageListRef), 20);
    const lastReadMessageId = latestMessageId(nextMessages);
    if (lastReadMessageId > 0) {
      void markConversationRead(conversationID, lastReadMessageId);
    }
  }, [markConversationRead]);

  const refreshSelectedGroupInfo = useCallback(async (conversationId: string) => {
    const data = await api.groupInfo(conversationId);
    if (selectedConversationIdRef.current === conversationId) {
      setSelectedGroupInfo(data);
    }
    return data;
  }, []);

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
  }, [refreshConversations, refreshCurrentConversationMessages, showToast]);

  const showMessageNotification = useCallback(
    (message: MessageInfo) => {
      if (!user || message.senderId === user.user_id || document.visibilityState === "visible") return;
      if (!notificationsEnabled) return;
      if (getNotificationStatus() !== "granted") return;
      if (message.messageType === "SYSTEM") return;

      const title = message.senderType === "BOT" || message.messageType === "BOT_REPLY" ? "AIM Bot reply" : "AIM new message";
      const content = messageText(message).trim();
      try {
        new Notification(title, {
          body: content ? truncateNotificationBody(content) : "You received a new message"
        });
      } catch {
        // Notification support can disappear in restricted browser contexts.
      }
    },
    [notificationsEnabled, user]
  );

  const handleRequestNotifications = useCallback(async () => {
    if (typeof Notification === "undefined") {
      setNotificationStatus("unsupported");
      showToast("当前浏览器不支持通知", "error");
      return;
    }
    if (Notification.permission === "granted") {
      setNotificationStatus("granted");
      setNotificationsEnabled(true);
      saveNotificationPreference(true);
      showToast("Browser notifications enabled", "success");
      return;
    }
    if (Notification.permission === "denied") {
      setNotificationStatus("denied");
      showToast("浏览器已阻止通知权限", "error");
      return;
    }

    const permission = await Notification.requestPermission();
    setNotificationStatus(permission);
    if (permission === "granted") {
      setNotificationsEnabled(true);
      saveNotificationPreference(true);
    }
    showToast(permission === "granted" ? "Browser notifications enabled" : "Browser notifications not enabled", permission === "granted" ? "success" : "info");
  }, [showToast]);

  const handleToggleNotifications = useCallback(async () => {
    if (notificationStatus === "granted") {
      const next = !notificationsEnabled;
      setNotificationsEnabled(next);
      saveNotificationPreference(next);
      showToast(next ? "Browser notifications enabled" : "Browser notifications disabled", next ? "success" : "info");
      return;
    }
    await handleRequestNotifications();
  }, [handleRequestNotifications, notificationStatus, notificationsEnabled, showToast]);

  const bootstrap = useCallback(async () => {
    try {
      const me = await api.me();
      setUser(me);
      await Promise.all([refreshConversations(), refreshFriends(), refreshSessions()]);
    } catch {
      setUser(null);
      setFriendGroups([]);
      setFriends([]);
      setFriendRequests([]);
      setConversations([]);
      setUnreadCounts({});
      setSessions([]);
      setSelectedConversationId(null);
    } finally {
      setBooting(false);
    }
  }, [refreshConversations, refreshFriends, refreshSessions]);

  useEffect(() => {
    void bootstrap();
  }, [bootstrap]);

  useEffect(() => {
    selectedConversationIdRef.current = selectedConversationId;
    if (selectedConversationId) {
      setUnreadCounts((current) => ({ ...current, [selectedConversationId]: 0 }));
    }
    setReplyingTo(null);
  }, [selectedConversationId]);

  useEffect(() => {
    if (!selectedConversationId || messages.length === 0 || !user) {
      return;
    }
    const lastMessage = messages[messages.length - 1];
    if (lastMessage.conversationId !== selectedConversationId) {
      return;
    }
    if (lastMessage.pending || lastMessage.senderId === user.user_id) {
      scrollMessagesToBottom(messageListRef);
    }
  }, [messages, selectedConversationId, user]);

  useEffect(() => {
    if (!selectedConversationId) {
      setMessages([]);
      setMembers([]);
      setSelectedGroupInfo(null);
      return;
    }

    let active = true;
    setLoadingMessages(true);
    const shouldLoadGroupInfo = selectedConversation?.type === "GROUP";
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
        setMessages(mergeMessagesById([], nextMessages));
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
  }, [markConversationRead, selectedConversation?.type, selectedConversationId, showToast]);

  const handleSocketEvent = useCallback(
    (raw: string) => {
      let event: WebSocketEvent;
      try {
        event = JSON.parse(raw) as WebSocketEvent;
      } catch {
        return;
      }

      if (event.type === "CONNECTED") {
        setWsStatus("open");
        return;
      }

      if (event.type === "MESSAGE_ACK") {
        const data = event.data as {
          messageId?: number;
          status: "SUCCESS" | "FAILED";
          errorMessage?: string;
        };
        const clientMsgID = event.clientMsgId?.trim();
        if (clientMsgID) {
          const pendingEntry = pendingMessagesRef.current.get(clientMsgID);
          if (pendingEntry) {
            setMessages((current) =>
              reconcilePendingMessage(current, clientMsgID, {
                id: data.messageId ?? pendingEntry.tempId,
                pending: false,
                status: data.status === "FAILED" ? "FAILED" : "NORMAL"
              })
            );
            pendingMessagesRef.current.delete(clientMsgID);
            if (data.status === "SUCCESS" && pendingEntry.conversationId === selectedConversationIdRef.current && data.messageId) {
              void markConversationRead(pendingEntry.conversationId, data.messageId);
            }
          }
        }
        if (data.status === "FAILED") {
          showToast(data.errorMessage || "Message send failed", "error");
        }
        return;
      }

      if (event.type === "NEW_MESSAGE") {
        const incoming = event.data as MessageInfo;
        const systemContent = incoming.messageType === "SYSTEM" ? parseSystemMessageContent(incoming.content) : null;
        showMessageNotification(incoming);
        const activeConversationID = selectedConversationIdRef.current;
        if (activeConversationID === incoming.conversationId) {
          setMessages((current) => mergeMessagesById(current, [incoming]));
          window.setTimeout(() => scrollMessagesToBottom(messageListRef), 20);
          void markConversationRead(incoming.conversationId, incoming.id);
          if (systemContent?.eventType === "ANNOUNCEMENT_UPDATED") {
            void refreshSelectedGroupInfo(incoming.conversationId);
          }
        } else if (incoming.messageType !== "SYSTEM") {
          setUnreadCounts((current) => ({
            ...current,
            [incoming.conversationId]: (current[incoming.conversationId] ?? 0) + 1
          }));
        }

        setConversations((current) =>
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
                        : incoming.senderId === user?.user_id
                          ? user.nickname || user.aim_id
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
        applyRecalledMessageEvent({
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
        void syncFriendStateFromRealtime({
          refreshConversations:
            Boolean(data.conversationId) ||
            (data.reason === "REQUEST_RESPONDED" && data.status === "ACCEPTED")
        });
      }
    },
    [
      applyRecalledMessageEvent,
      markConversationRead,
      refreshSelectedGroupInfo,
      showMessageNotification,
      showToast,
      syncFriendStateFromRealtime,
      user
    ]
  );

  useEffect(() => {
    handleSocketEventRef.current = handleSocketEvent;
  }, [handleSocketEvent]);

  useEffect(() => {
    recoverRealtimeStateRef.current = recoverRealtimeState;
  }, [recoverRealtimeState]);

  useEffect(() => {
    if (!user) {
      window.clearTimeout(reconnectTimerRef.current);
      connectNowRef.current = null;
      reconnectAttemptRef.current = 0;
      socketRef.current?.close();
      socketRef.current = null;
      setWsStatus("closed");
      return;
    }

    let disposed = false;

    function clearReconnectTimer() {
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = 0;
    }

    function scheduleReconnect() {
      if (disposed) return;
      clearReconnectTimer();
      const index = Math.min(reconnectAttemptRef.current, wsReconnectDelays.length - 1);
      const delay = wsReconnectDelays[index];
      reconnectAttemptRef.current += 1;
      reconnectTimerRef.current = window.setTimeout(connect, delay);
    }

    function connect() {
      if (disposed) return;
      clearReconnectTimer();

      const currentSocket = socketRef.current;
      if (
        currentSocket &&
        (currentSocket.readyState === WebSocket.OPEN || currentSocket.readyState === WebSocket.CONNECTING)
      ) {
        return;
      }

      setWsStatus("connecting");
      const protocol = window.location.protocol === "https:" ? "wss" : "ws";
      const endpoint = import.meta.env.VITE_WS_URL || `${protocol}://${window.location.host}/ws/chat`;
      const socket = new WebSocket(endpoint);
      socketRef.current = socket;

      socket.onopen = () => {
        if (disposed) return;
        reconnectAttemptRef.current = 0;
        setWsStatus("open");
        void recoverRealtimeStateRef.current();
      };
      socket.onclose = () => {
        if (disposed) return;
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        setWsStatus("closed");
        scheduleReconnect();
      };
      socket.onerror = () => {
        if (disposed) return;
        setWsStatus("closed");
        socket.close();
      };
      socket.onmessage = (event) => {
        handleSocketEventRef.current(event.data);
      };
    }

    connectNowRef.current = connect;
    connect();

    return () => {
      disposed = true;
      clearReconnectTimer();
      connectNowRef.current = null;
      reconnectAttemptRef.current = 0;
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [user]);

  useEffect(() => {
    if (!user) return;

    const handleVisible = () => {
      if (document.visibilityState !== "visible") return;

      const socket = socketRef.current;
      if (!socket || socket.readyState === WebSocket.CLOSED || socket.readyState === WebSocket.CLOSING) {
        connectNowRef.current?.();
      }
      void recoverRealtimeState();
    };

    document.addEventListener("visibilitychange", handleVisible);
    return () => {
      document.removeEventListener("visibilitychange", handleVisible);
    };
  }, [recoverRealtimeState, user]);

  const handleLogin = async (input: { email: string; password: string }) => {
    setBusyAction(true);
    try {
      await api.login({
        email: input.email,
        password: input.password,
        device_name: "AIM Web"
      });
      await bootstrap();
      showToast("登录成功", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleRegister = async (input: { aim_id: string; email: string; nickname: string; password: string }) => {
    setBusyAction(true);
    try {
      await api.register(input);
      showToast("注册完成，可以登录了", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleLogout = async () => {
    setBusyAction(true);
    try {
      await api.logout();
    } catch {
      // Local cleanup still applies when the session has already expired.
    } finally {
      socketRef.current?.close();
      setUser(null);
      setFriendGroups([]);
      setFriends([]);
      setFriendRequests([]);
      setMessages([]);
      setMembers([]);
      setConversations([]);
      setUnreadCounts({});
      setSelectedConversationId(null);
      setBusyAction(false);
    }
  };

  const handleCreateGroup = async (input: { name: string; announcement: string; joinPolicy: string }) => {
    setBusyAction(true);
    try {
      const group = await api.createGroup({
        name: input.name,
        avatar: "",
        announcement: input.announcement,
        joinPolicy: input.joinPolicy
      });
      await refreshConversations();
      setSelectedConversationId(group.conversationId);
      setMobilePane("chat");
      showToast(`会话已创建：conversationId: ${group.conversationId}`, "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleJoinGroup = async (conversationId: string) => {
    setBusyAction(true);
    try {
      await api.joinGroup(conversationId);
      await refreshConversations();
      setSelectedConversationId(conversationId);
      setMobilePane("chat");
      showToast("Joined group", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleLeaveGroup = async () => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.leaveGroup(selectedConversationId);
      await refreshConversations();
      showToast("Left group", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleInviteMember = async (targetUserId: number) => {
    if (!selectedConversationId) return;
    try {
      await api.inviteMember(selectedConversationId, targetUserId);
      const nextMembers = await api.members(selectedConversationId);
      setMembers(nextMembers);
    } catch (error) {
      throw error;
    }
  };

  const refreshSelectedConversationState = useCallback(async () => {
    const conversationId = selectedConversationIdRef.current;
    if (!conversationId) return;

    const shouldRefreshGroupInfo = selectedConversation?.type === "GROUP";
    const [nextMembers, nextGroupInfo] = await Promise.all([
      api.members(conversationId),
      shouldRefreshGroupInfo ? api.groupInfo(conversationId) : Promise.resolve(null),
      refreshConversations()
    ]);
    if (selectedConversationIdRef.current === conversationId) {
      setMembers(nextMembers);
      setSelectedGroupInfo(nextGroupInfo);
      await refreshCurrentConversationMessages();
    }
  }, [refreshConversations, refreshCurrentConversationMessages, selectedConversation?.type]);

  const handleTransferOwner = async (targetUserId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.transferOwner(selectedConversationId, targetUserId);
      await refreshSelectedConversationState();
      showToast("Ownership transferred", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleSetAdmin = async (targetUserId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.setAdmin(selectedConversationId, targetUserId);
      await refreshSelectedConversationState();
      showToast("管理员已设置", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleRemoveAdmin = async (targetUserId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.removeAdmin(selectedConversationId, targetUserId);
      await refreshSelectedConversationState();
      showToast("管理员已取消", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleMuteMember = async (targetUserId: number, durationSeconds: number) => {
    if (!selectedConversationId || durationSeconds <= 0) return;
    setBusyAction(true);
    try {
      const muteUntil = Math.floor(Date.now() / 1000) + durationSeconds;
      await api.muteMember(selectedConversationId, targetUserId, muteUntil);
      await refreshSelectedConversationState();
      showToast("成员已禁言", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleUnmuteMember = async (targetUserId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.unmuteMember(selectedConversationId, targetUserId);
      await refreshSelectedConversationState();
      showToast("已解除禁言", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleRemoveMember = async (targetUserId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.removeMember(selectedConversationId, targetUserId);
      await refreshSelectedConversationState();
      showToast("Member removed from group", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleSetGroupMuteAll = async (muteAll: boolean) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.setGroupMuteAll(selectedConversationId, muteAll);
      await refreshSelectedConversationState();
      showToast(muteAll ? "已开启全员禁言" : "已关闭全员禁言", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleUpdateGroupAnnouncement = async (announcement: string) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.updateGroupAnnouncement(selectedConversationId, announcement);
      await Promise.all([refreshSelectedGroupInfo(selectedConversationId), refreshSelectedConversationState()]);
      showToast("Group announcement updated", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

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
  }, []);

  const handleMention = useCallback((mentionTarget: string) => {
    const normalized = mentionTarget.trim().replace(/^@+/, "");
    if (!normalized) {
      return;
    }
    setMessageDraft((current) => appendMentionToDraft(current, normalized));
    window.requestAnimationFrame(() => {
      composerRef.current?.focus();
    });
  }, []);

  const handleReplyMessage = useCallback((message: MessageInfo) => {
    if (message.pending || message.id <= 0 || message.status !== "NORMAL") {
      return;
    }
    setReplyingTo(buildReplyPreview(message));
    window.requestAnimationFrame(() => {
      composerRef.current?.focus();
    });
  }, []);

  function applyRecalledMessageEvent(event: MessageRecalledEventInfo) {
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
  }

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

  const handleSendMessage = (payload?: OutgoingMessagePayload) => {
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
            content: JSON.stringify({ text: content })
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
      content: outgoing.content,
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
                lastMessageContent: outgoing.content,
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
            content: outgoing.content,
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
  };

  const handleLoadOlder = async () => {
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
  };

  const handleRevokeSession = async (sessionId: string, password: string) => {
    if (!password.trim()) {
      showToast("Please enter your password", "error");
      return;
    }
    setBusyAction(true);
    try {
      await api.revokeSession(sessionId, password);
      await refreshSessions();
      showToast("会话已注销", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleLogoutAll = async (password: string) => {
    if (!password.trim()) {
      showToast("Please enter your password", "error");
      return;
    }
    setBusyAction(true);
    try {
      await api.logoutAll(password);
      await handleLogout();
    } catch (error) {
      showToast(errorMessage(error), "error");
      setBusyAction(false);
    }
  };

  const handleAvatarUpload = async (avatar: Blob) => {
    setBusyAction(true);
    try {
      const response = await api.uploadAvatar(avatar);
      setUser(response.user);
      setMembers((current) =>
        current.map((member) =>
          member.userId === response.user.user_id
            ? { ...member, avatar: response.user.avatar, nickname: response.user.nickname }
            : member
        )
      );
      showToast("Avatar updated", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleCreateFriendGroup = async (name: string) => {
    setBusyAction(true);
    try {
      const group = await api.createFriendGroup(name);
      await refreshFriends();
      setDetailTab("friends");
      setMobilePane("friends");
      showToast(`好友分组已创建：${group.name}`, "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleAddFriend = async (input: { targetAimId: string; remark: string; groupId: number | null }) => {
    setBusyAction(true);
    try {
      const request = await api.addFriend({
        target_aim_id: input.targetAimId,
        remark: input.remark,
        group_id: input.groupId
      });
      await refreshFriends();
      setDetailTab("friends");
      setMobilePane("friends");
      showToast(`好友申请已发送给${request.nickname || request.aim_id}`, "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleRespondFriendRequest = async (requestId: number, action: "ACCEPTED" | "REJECTED") => {
    setBusyAction(true);
    try {
      const existingConversationIds = new Set(conversations.map((item) => item.conversationId));
      const response = await api.respondFriendRequest(requestId, action);
      await refreshFriends();
      if (action === "ACCEPTED") {
        const nextConversations = await refreshConversations();
        const singleConversation = nextConversations.find(
          (item) => item.type === "SINGLE" && !existingConversationIds.has(item.conversationId)
        );
        if (singleConversation) {
          setSelectedConversationId(singleConversation.conversationId);
          setMobilePane("chat");
        } else {
          setDetailTab("friends");
          setMobilePane("friends");
        }
        showToast(`Accepted ${response.friend?.nickname || response.request.nickname}` + "'s friend request", "success");
      } else {
        setDetailTab("friends");
        setMobilePane("friends");
        showToast(`Rejected ${response.request.nickname}` + "'s friend request", "info");
      }
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleUpdateFriend = async (friendUserId: number, input: { remark: string; groupId: number | null }) => {
    setBusyAction(true);
    try {
      const updated = await api.updateFriend(friendUserId, {
        remark: input.remark,
        group_id: input.groupId
      });
      setFriends((current) =>
        sortFriends(current.map((item) => (item.user_id === friendUserId ? updated : item)))
      );
      showToast("Friend updated", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleDeleteFriend = async (friendUserId: number) => {
    setBusyAction(true);
    try {
      await api.deleteFriend(friendUserId);
      setFriends((current) => current.filter((item) => item.user_id !== friendUserId));
      showToast("Friend deleted", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const refreshAvailableBots = useCallback(async () => {
    const data = await api.bots();
    setAvailableBots(data);
  }, []);

  const refreshConversationBots = useCallback(async () => {
    if (!selectedConversationId) return;
    const data = await api.conversationBots(selectedConversationId);
    setConversationBots(data);
  }, [selectedConversationId]);

  const refreshAICallLogs = useCallback(async () => {
    if (!selectedConversationId) {
      setAICallLogs([]);
      setAICallLogQuota({
        dailyTotalTokens: 0,
        dailyTokenLimit: 1_000_000,
        remainingTokens: 1_000_000
      });
      return;
    }
    setLoadingAICallLogs(true);
    try {
      const data = await api.aiCallLogs(selectedConversationId, {
        limit: 50,
        status: aiCallLogStatus || undefined
      });
      setAICallLogs(data.logs);
      setAICallLogQuota(data.quota);
    } finally {
      setLoadingAICallLogs(false);
    }
  }, [aiCallLogStatus, selectedConversationId]);

  const handleAddBot = async (input: {
    botId: number;
    displayNameOverride?: string;
    mentionNameOverride?: string;
    aliasesOverride?: string[];
    permissionScope?: string;
    modelNameOverride?: string;
  }) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.addConversationBot(selectedConversationId, input);
      await refreshConversationBots();
      showToast("Bot added", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  const handleRemoveBot = async (botId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.removeConversationBot(selectedConversationId, botId);
      await refreshConversationBots();
      showToast("Bot removed", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  };

  useEffect(() => {
    if (detailTab === "bots" && selectedConversationId && selectedConversation?.type === "GROUP") {
      void (async () => {
        try {
          await Promise.all([refreshAvailableBots(), refreshConversationBots()]);
        } catch (error) {
          showToast(errorMessage(error), "error");
        }
      })();
      return;
    }
    if (detailTab === "bots") {
      setAvailableBots([]);
      setConversationBots([]);
    }
  }, [detailTab, refreshAvailableBots, refreshConversationBots, selectedConversation?.type, selectedConversationId, showToast]);

  useEffect(() => {
    if (detailTab === "logs" && selectedConversationId) {
      void (async () => {
        try {
          await refreshAICallLogs();
        } catch (error) {
          showToast(errorMessage(error), "error");
        }
      })();
      return;
    }
    if (detailTab !== "logs") {
      setLoadingAICallLogs(false);
    }
  }, [detailTab, selectedConversationId, refreshAICallLogs, showToast]);

  useEffect(() => {
    if (!selectedConversationId) {
      setAICallLogs([]);
      setAICallLogQuota({
        dailyTotalTokens: 0,
        dailyTokenLimit: 1_000_000,
        remainingTokens: 1_000_000
      });
    }
  }, [selectedConversationId]);

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
        unreadCounts={unreadCounts}
        rawConversationCount={conversations.length}
        currentUser={user}
        selectedConversationId={selectedConversationId}
        search={search}
        onSearch={setSearch}
        onCreateGroup={handleCreateGroup}
        onJoinGroup={handleJoinGroup}
        onRefresh={refreshConversations}
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
        messages={messages}
        messageDraft={messageDraft}
        replyingTo={replyingTo}
        wsStatus={wsStatus}
        busy={busyAction}
        messageListRef={messageListRef}
        composerRef={composerRef}
        canSend={canSendCurrentConversation}
        onBack={() => setMobilePane("conversations")}
        onDraftChange={setMessageDraft}
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
        onCancelReply={() => setReplyingTo(null)}
        onSend={handleSendMessage}
      />

      <DetailPanel
        active={mobilePane === "friends" || mobilePane === "members" || mobilePane === "bots" || mobilePane === "account"}
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

function AuthView({
  busy,
  onLogin,
  onRegister
}: {
  busy: boolean;
  onLogin: (input: { email: string; password: string }) => Promise<void>;
  onRegister: (input: { aim_id: string; email: string; nickname: string; password: string }) => Promise<void>;
}) {
  const [mode, setMode] = useState<AuthMode>("login");
  const [email, setEmail] = useState("demo@example.com");
  const [password, setPassword] = useState("Password123!");
  const [aimID, setAimID] = useState("");
  const [nickname, setNickname] = useState("");

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (mode === "login") {
      await onLogin({ email, password });
      return;
    }
    await onRegister({ aim_id: aimID, email, nickname, password });
    setMode("login");
  };

  return (
    <main className="auth-screen">
      <section className="auth-copy" aria-label="AIM">
        <div className="brand-row">
          <div className="brand-mark">A</div>
          <div>
            <h1>AIM</h1>
            <p>P0/P1 Chat Console</p>
          </div>
        </div>
        <div className="auth-signal">
          <div className="signal-line">
            <ShieldCheck size={18} />
            <span>Gateway Cookie Auth</span>
          </div>
          <div className="signal-line">
            <UsersRound size={18} />
            <span>Group Conversations</span>
          </div>
          <div className="signal-line">
            <MessageCircle size={18} />
            <span>Text Message History</span>
          </div>
        </div>
      </section>

      <section className="auth-card">
        <div className="segmented">
          <button className={mode === "login" ? "active" : ""} type="button" onClick={() => setMode("login")}>
            登录
          </button>
          <button className={mode === "register" ? "active" : ""} type="button" onClick={() => setMode("register")}>
            注册
          </button>
        </div>

        <form className="stack-form" onSubmit={submit}>
          {mode === "register" && (
            <>
              <Field icon={<BadgePlus size={18}></BadgePlus>} label="AIM ID">
                <input required value={aimID} onChange={(event) => setAimID(event.target.value)} placeholder="xqe_0422" />
              </Field>
              <Field icon={<UserRound size={18}></UserRound>} label="昵称">
                <input required value={nickname} onChange={(event) => setNickname(event.target.value)} placeholder="小青" />
              </Field>
            </>
          )}
          <Field icon={<Mail size={18}></Mail>} label="邮箱">
            <input required type="email" value={email} onChange={(event) => setEmail(event.target.value)} placeholder="demo@example.com" />
          </Field>
          <Field icon={<LockKeyhole size={18}></LockKeyhole>} label="密码">
            <input required type="password" value={password} onChange={(event) => setPassword(event.target.value)} placeholder="Password123!" />
          </Field>
          <button className="primary-action" disabled={busy} type="submit">
            {busy ? <Loader2 className="spin" size={18} /> : <CheckCircle2 size={18} />}
            {mode === "login" ? "登录 AIM" : "创建账号"}
          </button>
        </form>
      </section>
    </main>
  );
}

function ConversationPanel({
  active,
  busy,
  conversations,
  unreadCounts,
  rawConversationCount,
  currentUser,
  selectedConversationId,
  search,
  onSearch,
  onCreateGroup,
  onJoinGroup,
  onRefresh,
  onSelect
}: {
  active: boolean;
  busy: boolean;
  conversations: ConversationInfo[];
  unreadCounts: Record<string, number>;
  rawConversationCount: number;
  currentUser: UserInfo;
  selectedConversationId: string | null;
  search: string;
  onSearch: (value: string) => void;
  onCreateGroup: (input: { name: string; announcement: string; joinPolicy: string }) => Promise<void>;
  onJoinGroup: (conversationId: string) => Promise<void>;
  onRefresh: () => Promise<ConversationInfo[]>;
  onSelect: (conversationId: string) => void;
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const [joinOpen, setJoinOpen] = useState(false);
  const [groupName, setGroupName] = useState("");
  const [announcement, setAnnouncement] = useState("");
  const [joinPolicy, setJoinPolicy] = useState("FREE");
  const [joinID, setJoinID] = useState("");

  const create = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onCreateGroup({ name: groupName, announcement, joinPolicy });
    setGroupName("");
    setAnnouncement("");
    setCreateOpen(false);
  };

  const join = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const conversationId = joinID.trim();
    if (!conversationId) return;
    await onJoinGroup(conversationId);
    setJoinID("");
    setJoinOpen(false);
  };

  return (
    <aside className={cx("pane conversation-pane", active && "mobile-active")}>
      <div className="pane-header">
        <div className="brand-row compact">
          <div className="brand-mark small">A</div>
          <div>
            <strong>AIM</strong>
            <span>加密即时通讯</span>
          </div>
        </div>
        <IconButton label="刷新会话" onClick={() => void onRefresh()}>
          <RefreshCw size={18} />
        </IconButton>
      </div>

      <div className="profile-strip">
        <Avatar name={currentUser.nickname || currentUser.aim_id} src={currentUser.avatar} />
        <div>
          <strong>{currentUser.nickname || currentUser.aim_id}</strong>
          <span>{currentUser.aim_id}</span>
        </div>
      </div>

      <div className="action-grid">
        <button type="button" onClick={() => setCreateOpen((value) => !value)}>
          <MessageSquarePlus size={18} />
          建群
        </button>
        <button type="button" onClick={() => setJoinOpen((value) => !value)}>
          <UserPlus size={18} />
          入群
        </button>
      </div>

      {createOpen && (
        <form className="drawer-form" onSubmit={create}>
          <input required value={groupName} onChange={(event) => setGroupName(event.target.value)} placeholder="Group name" />
          <textarea value={announcement} onChange={(event) => setAnnouncement(event.target.value)} rows={3} placeholder="Announcement" />
          <select value={joinPolicy} onChange={(event) => setJoinPolicy(event.target.value)}>
            {joinPolicies.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
          <button disabled={busy} type="submit">
            {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
            创建
          </button>
        </form>
      )}

      {joinOpen && (
        <form className="drawer-form" onSubmit={join}>
          <input required value={joinID} onChange={(event) => setJoinID(event.target.value)} placeholder="conversationId (for example c_xxxxx)" />
          <button disabled={busy} type="submit">
            {busy ? <Loader2 className="spin" size={16} /> : <UserPlus size={16} />}
            加入
          </button>
        </form>
      )}

      <label className="search-box">
        <Search size={17} />
        <input value={search} onChange={(event) => onSearch(event.target.value)} placeholder="Search conversations or IDs" />
      </label>

      <div className="list-meta">
        <span>会话</span>
        <strong>{rawConversationCount}</strong>
      </div>

      <div className="conversation-list">
        {conversations.map((conversation) => {
          const unread = unreadCounts[conversation.conversationId] ?? 0;
          const preview = conversationPreview(conversation);
          const title = conversation.title || conversation.type;
          return (
          <button
            key={conversation.conversationId}
            className={cx("conversation-item", selectedConversationId === conversation.conversationId && "active")}
            type="button"
            onClick={() => onSelect(conversation.conversationId)}
          >
            <Avatar name={title} src={conversation.avatar} />
            <span className="conversation-text">
              <strong>{conversation.title || `会话 ${conversation.conversationId}`}</strong>
              <span className="conversation-preview">
                {preview}
              </span>
            </span>
            <span className="conversation-side">
              <time>{formatRelative(conversation.lastMessageAt ?? conversation.updatedAt)}</time>
              {unread > 0 && <span className="unread-badge">{unread > 99 ? "99+" : unread}</span>}
            </span>
          </button>
          );
        })}

        {conversations.length === 0 && (
          <div className="empty-block">
            <UsersRound size={30} />
            <strong>暂无会话</strong>
            <span>创建会话或输入：会话 ID 加入</span>
          </div>
        )}
      </div>
    </aside>
  );
}

function ChatPanel({
  active,
  conversation,
  currentUser,
  currentMember,
  members,
  loading,
  loadingOlder,
  messages,
  messageDraft,
  replyingTo,
  wsStatus,
  busy,
  messageListRef,
  composerRef,
  canSend,
  onBack,
  onDraftChange,
  onLoadOlder,
  onLeaveGroup,
  onOpenMembers,
  onInviteMember,
  onMention,
  onReplySelect,
  onRecallMessage,
  onCancelReply,
  onSend
}: {
  active: boolean;
  conversation: ConversationInfo | null;
  currentUser: UserInfo;
  currentMember: MemberInfo | null;
  members: MemberInfo[];
  loading: boolean;
  loadingOlder: boolean;
  messages: MessageInfo[];
  messageDraft: string;
  replyingTo: ReplyPreviewInfo | null;
  wsStatus: WsStatus;
  busy: boolean;
  messageListRef: React.RefObject<HTMLDivElement | null>;
  composerRef: React.RefObject<HTMLTextAreaElement | null>;
  canSend: boolean;
  onBack: () => void;
  onDraftChange: (value: string) => void;
  onLoadOlder: () => Promise<void>;
  onLeaveGroup: () => Promise<void>;
  onOpenMembers: () => void;
  onInviteMember?: (targetUserId: number) => Promise<void>;
  onMention: (mentionTarget: string) => void;
  onReplySelect: (message: MessageInfo) => void;
  onRecallMessage: (message: MessageInfo) => Promise<void>;
  onCancelReply: () => void;
  onSend: (payload?: OutgoingMessagePayload) => void;
}) {
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteFriends, setInviteFriends] = useState<FriendInfo[]>([]);
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteInvitingId, setInviteInvitingId] = useState<number | null>(null);
  const [mediaMode, setMediaMode] = useState<"IMAGE" | "FILE" | "VOICE" | null>(null);
  const [mediaURL, setMediaURL] = useState("");
  const [mediaName, setMediaName] = useState("");
  const [mediaMimeType, setMediaMimeType] = useState("");
  const [mediaSize, setMediaSize] = useState("");
  const [imageWidth, setImageWidth] = useState("");
  const [imageHeight, setImageHeight] = useState("");
  const [voiceDurationMs, setVoiceDurationMs] = useState("");

  const isGroupChat = conversation?.type === "GROUP";
  const memberUserIds = useMemo(() => new Set(members.filter((m) => m.memberType !== "BOT").map((m) => m.userId)), [members]);

  const openInviteDialog = useCallback(async () => {
    if (!conversation) return;
    setInviteOpen(true);
    setInviteLoading(true);
    try {
      const list = await api.friends();
      setInviteFriends(list);
    } catch {
      // silently ignore
    } finally {
      setInviteLoading(false);
    }
  }, [conversation]);

  const handleInviteFriend = async (friend: FriendInfo) => {
    if (!onInviteMember || !conversation) return;
    setInviteInvitingId(friend.user_id);
    try {
      await onInviteMember(friend.user_id);
      setInviteFriends((prev) => prev.filter((f) => f.user_id !== friend.user_id));
      setInviteOpen(false);
    } catch (err) {
      alert(err instanceof Error ? err.message : "Invite failed");
    } finally {
      setInviteInvitingId(null);
    }
  };

  const resetMediaComposer = useCallback(() => {
    setMediaMode(null);
    setMediaURL("");
    setMediaName("");
    setMediaMimeType("");
    setMediaSize("");
    setImageWidth("");
    setImageHeight("");
    setVoiceDurationMs("");
  }, []);

  const toggleMediaMode = useCallback(
    (mode: "IMAGE" | "FILE" | "VOICE") => {
      if (mediaMode === mode) {
        resetMediaComposer();
        return;
      }
      setMediaMode(mode);
      setMediaURL("");
      setMediaName("");
      setMediaMimeType("");
      setMediaSize("");
      setImageWidth("");
      setImageHeight("");
      setVoiceDurationMs("");
    },
    [mediaMode, resetMediaComposer]
  );

  const handleSendMedia = () => {
    if (!mediaMode) return;
    const url = mediaURL.trim();
    const name = mediaName.trim();
    const mimeType = mediaMimeType.trim();
    if (!url || !name || !mimeType) return;

    if (mediaMode === "IMAGE") {
      const payload = {
        url,
        name,
        mimeType,
        size: mediaSize.trim() ? Number(mediaSize) : undefined,
        width: imageWidth.trim() ? Number(imageWidth) : undefined,
        height: imageHeight.trim() ? Number(imageHeight) : undefined
      };
      onSend({
        messageType: "IMAGE",
        content: JSON.stringify(payload)
      });
      resetMediaComposer();
      return;
    }

    if (mediaMode === "FILE") {
      const size = Number(mediaSize);
      if (!Number.isFinite(size) || size <= 0) return;
      onSend({
        messageType: "FILE",
        content: JSON.stringify({
          url,
          name,
          mimeType,
          size
        })
      });
      resetMediaComposer();
      return;
    }

    const durationMs = Number(voiceDurationMs);
    if (!Number.isFinite(durationMs) || durationMs <= 0) return;
    onSend({
      messageType: "VOICE",
      content: JSON.stringify({
        url,
        name,
        mimeType,
        size: mediaSize.trim() ? Number(mediaSize) : undefined,
        durationMs
      })
    });
    resetMediaComposer();
  };

  const sendDisabled = !conversation || !canSend || !messageDraft.trim() || wsStatus !== "open";
  const composerDisabled = !conversation || !canSend || wsStatus !== "open";
  const mediaSendDisabled =
    !conversation ||
    !canSend ||
    wsStatus !== "open" ||
    !mediaMode ||
    !mediaURL.trim() ||
    !mediaName.trim() ||
    !mediaMimeType.trim() ||
    (mediaMode === "FILE" && (!mediaSize.trim() || Number(mediaSize) <= 0)) ||
    (mediaMode === "VOICE" && (!voiceDurationMs.trim() || Number(voiceDurationMs) <= 0));
  const memberMap = useMemo(() => new Map(members.map((member) => [member.userId, member])), [members]);
  const resolveReplySenderLabel = useCallback(
    (replyPreview: ReplyPreviewInfo | null | undefined) => {
      if (!replyPreview) {
        return "Original message unavailable";
      }
      if (replyPreview.senderType === "SYSTEM") {
        return "System";
      }
      if (replyPreview.senderType === "BOT") {
        const botMember = memberMap.get(replyPreview.senderId);
        return botMember?.nickname || botMember?.mentionName || `Bot ${replyPreview.senderId}`;
      }
      if (replyPreview.senderId === currentUser.user_id) {
        return currentUser.nickname || currentUser.aim_id;
      }
      return memberMap.get(replyPreview.senderId)?.nickname || `User ${replyPreview.senderId}`;
    },
    [currentUser.aim_id, currentUser.nickname, currentUser.user_id, memberMap]
  );
  /* const lockedReason = !conversation
    ? ""
    : conversation.type === "SINGLE"
      ? "已不是好友，不能继续发送单聊消息"
      : isMemberMuted(currentMember)
        ? `你已被禁言?${formatMuteUntil(currentMember?.muteUntil)}`
        : conversation.muteAll && currentMember?.role !== "OWNER" && currentMember?.role !== "ADMIN"
          ? "当前群已开启全员禁言"
          : "当前无法发送消息";

  */
  const lockedReason = !conversation
    ? ""
    : conversation.type === "SINGLE"
      ? "You can no longer send direct messages in this conversation."
      : isMemberMuted(currentMember)
        ? `You are muted until ${formatMuteUntil(currentMember?.muteUntil)}`
        : conversation.muteAll && currentMember?.role !== "OWNER" && currentMember?.role !== "ADMIN"
          ? "This group is muted for regular members."
          : "You cannot send messages right now.";

  const onComposerKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (!canSend) {
      return;
    }
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      onSend();
    }
  };

  useEffect(() => {
    resetMediaComposer();
  }, [conversation?.conversationId, resetMediaComposer]);

  return (
    <main className={cx("pane chat-pane", active && "mobile-active")}>
      {conversation ? (
        <>
          <header className="chat-header">
            <IconButton className="mobile-only" label="返回会话" onClick={onBack}>
              <ChevronLeft size={20} />
            </IconButton>
            <Avatar name={conversation.title || `#${conversation.conversationId}`} src={conversation.avatar} />
            <div className="chat-title">
              <strong>{conversation.title || `会话 ${conversation.conversationId}`}</strong>
              <span>
                conversationId: {conversation.conversationId} · {currentMember ? roleLabel(currentMember.role) : roleLabel(conversation.role)}
              </span>
            </div>
            <span className="chat-id-badge">ID {conversation.conversationId}</span>
            <WsBadge status={wsStatus} />
            <IconButton label="成员" onClick={onOpenMembers}>
              <PanelRightOpen size={19} />
            </IconButton>
            {isGroupChat && onInviteMember && (
              <IconButton label="Invite friend" onClick={openInviteDialog}>
                <UserPlus size={19} />
              </IconButton>
            )}
            <IconButton label="Leave group" disabled={busy} onClick={() => void onLeaveGroup()}>
              <DoorOpen size={19} />
            </IconButton>
          </header>

          <div className="message-scroll" ref={messageListRef}>
            <div className="history-row">
              <button disabled={loadingOlder || messages.length === 0} type="button" onClick={() => void onLoadOlder()}>
                {loadingOlder ? <Loader2 className="spin" size={16} /> : <RefreshCw size={16} />}
                加载更多消息
              </button>
            </div>

            {loading ? (
              <div className="center-state">
                <Loader2 className="spin" size={28} />
              </div>
            ) : messages.length > 0 ? (
              messages.map((message) => {
                const mine = message.senderId === currentUser.user_id;
                const sender =
                  memberMap.get(message.senderId) ??
                  (mine
                    ? {
                        userId: currentUser.user_id,
                        nickname: currentUser.nickname || currentUser.aim_id,
                        avatar: currentUser.avatar,
                        role: currentMember?.role ?? "MEMBER",
                        status: currentMember?.status ?? "NORMAL",
                        joinedAt: 0
                      }
                    : null);

                return (
                  <MessageBubble
                    key={message.id}
                    message={message}
                    mine={mine}
                    readReceiptLabel={readReceiptLabel(conversation, message, mine)}
                    replySummaryLabel={resolveReplySenderLabel(message.replyTo)}
                    senderAvatar={sender?.avatar}
                    onMention={onMention}
                    onReply={() => onReplySelect(message)}
                    onRecall={() => void onRecallMessage(message)}
                    mentionTarget={message.senderType === "BOT" ? sender?.mentionName || sender?.nickname : sender?.nickname}
                    senderName={sender?.nickname || `用户 ${message.senderId}`}
                  />
                );
              })
            ) : (
              <div className="empty-thread">
                <MessageCircle size={36} />
                <strong>No messages yet</strong>
              </div>
            )}
          </div>

          {canSend ? (
            <footer className="composer">
              {replyingTo && (
                <div className="replying-banner">
                  <div className="replying-banner-copy">
                    <strong>Replying to {resolveReplySenderLabel(replyingTo)}</strong>
                    <span>{replyingTo.contentPreview}</span>
                  </div>
                  <button type="button" onClick={onCancelReply}>
                    Cancel
                  </button>
                </div>
              )}
              {mediaMode && (
                <div className="media-composer">
                  <div className="media-composer-grid">
                    <input value={mediaURL} onChange={(event) => setMediaURL(event.target.value)} placeholder="Media URL" />
                    <input value={mediaName} onChange={(event) => setMediaName(event.target.value)} placeholder="Name" />
                    <input value={mediaMimeType} onChange={(event) => setMediaMimeType(event.target.value)} placeholder="MIME type" />
                    <input
                      value={mediaSize}
                      onChange={(event) => setMediaSize(event.target.value)}
                      inputMode="numeric"
                      placeholder={mediaMode === "FILE" ? "Size in bytes (required)" : "Size in bytes (optional)"}
                    />
                    {mediaMode === "IMAGE" && (
                      <>
                        <input value={imageWidth} onChange={(event) => setImageWidth(event.target.value)} inputMode="numeric" placeholder="Width (optional)" />
                        <input value={imageHeight} onChange={(event) => setImageHeight(event.target.value)} inputMode="numeric" placeholder="Height (optional)" />
                      </>
                    )}
                    {mediaMode === "VOICE" && (
                      <input
                        value={voiceDurationMs}
                        onChange={(event) => setVoiceDurationMs(event.target.value)}
                        inputMode="numeric"
                        placeholder="Duration ms (required)"
                      />
                    )}
                  </div>
                  <div className="media-composer-actions">
                    <button disabled={mediaSendDisabled} type="button" onClick={handleSendMedia}>
                      {mediaMode === "IMAGE" ? "Send image" : mediaMode === "FILE" ? "Send file" : "Send voice"}
                    </button>
                    <button type="button" onClick={resetMediaComposer}>
                      Cancel
                    </button>
                  </div>
                </div>
              )}

              <div className="composer-tools">
                <button className={cx(mediaMode === "IMAGE" && "active")} type="button" onClick={() => toggleMediaMode("IMAGE")}>
                  <FileImage size={16} />
                  Image
                </button>
                <button className={cx(mediaMode === "FILE" && "active")} type="button" onClick={() => toggleMediaMode("FILE")}>
                  <Paperclip size={16} />
                  File
                </button>
                <button className={cx(mediaMode === "VOICE" && "active")} type="button" onClick={() => toggleMediaMode("VOICE")}>
                  <Mic size={16} />
                  Voice
                </button>
              </div>

              <div className="composer-input-row">
                <textarea
                  ref={composerRef}
                  value={messageDraft}
                  onChange={(event) => onDraftChange(event.target.value)}
                  onKeyDown={onComposerKeyDown}
                  rows={1}
                  disabled={composerDisabled}
                  placeholder={wsStatus === "open" ? "Send a message" : "Connecting to realtime channel"}
                />
                <button aria-label="Send" disabled={sendDisabled} type="button" onClick={() => onSend()}>
                  <SendHorizontal size={21} />
                </button>
              </div>
            </footer>
          ) : (
            <footer className="composer composer-locked" title={lockedReason}>
              <LockKeyhole size={18} />
              <span>{lockedReason}</span>
            </footer>
          )}
        </>
      ) : (
        <div className="no-selection">
          <div className="brand-mark">A</div>
          <h2>AIM</h2>
          <p>Pick a conversation to start chatting</p>
        </div>
      )}

      {inviteOpen && (
        <div className="modal-overlay" onClick={() => setInviteOpen(false)}>
          <div className="modal-box" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <strong>Invite a friend</strong>
              <button type="button" onClick={() => setInviteOpen(false)}>
                <X size={18} />
              </button>
            </div>
            {inviteLoading ? (
              <div className="center-state" style={{ padding: "2rem 0" }}>
                <Loader2 className="spin" size={24} />
              </div>
            ) : inviteFriends.length === 0 ? (
              <div className="center-state" style={{ padding: "2rem 0" }}>
                <p>No inviteable friends</p>
              </div>
            ) : (
              <div className="invite-friend-list">
                {inviteFriends.map((friend) => {
                  const alreadyIn = memberUserIds.has(friend.user_id);
                  const inviting = inviteInvitingId === friend.user_id;
                  return (
                    <div key={friend.user_id} className={`invite-friend-item ${alreadyIn ? "disabled" : ""}`}>
                      <Avatar name={friend.nickname || friend.aim_id} src={friend.avatar} />
                      <div className="invite-friend-info">
                        <strong>{friend.remark || friend.nickname || friend.aim_id}</strong>
                        <span>{alreadyIn ? "Already in group" : friend.nickname || friend.aim_id}</span>
                      </div>
                      {alreadyIn ? (
                        <span className="invite-badge in-group">Already in group</span>
                      ) : (
                        <button
                          className="btn btn-sm"
                          disabled={inviting}
                          type="button"
                          onClick={() => void handleInviteFriend(friend)}
                        >
                          {inviting ? <Loader2 className="spin" size={14} /> : "Invite"}
                        </button>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </main>
  );
}

function DetailPanel({
  active,
  tab,
  user,
  friendGroups,
  friends,
  friendRequests,
  members,
  conversations,
  sessions,
  busy,
  wsStatus,
  notificationStatus,
  notificationsEnabled,
  selectedConversationId,
  selectedConversation,
  selectedConversationType,
  selectedGroupInfo,
  currentMember,
  availableBots,
  conversationBots,
  aiCallLogs,
  aiCallLogQuota,
  loadingAICallLogs,
  aiCallLogStatus,
  onTabChange,
  onCreateFriendGroup,
  onAddFriend,
  onRespondFriendRequest,
  onUpdateFriend,
  onDeleteFriend,
  onOpenChatWithFriend,
  onRefreshSessions,
  onLogout,
  onLogoutAll,
  onAvatarUpload,
  onRevokeSession,
  onToggleNotifications,
  onTransferOwner,
  onSetAdmin,
  onRemoveAdmin,
  onMuteMember,
  onUnmuteMember,
  onRemoveMember,
  onSetGroupMuteAll,
  onUpdateGroupAnnouncement,
  onAddBot,
  onRemoveBot,
  onAICallLogStatusChange,
  onRefreshAICallLogs,
  onMention,
  onClose
}: {
  active: boolean;
  tab: DetailTab;
  user: UserInfo;
  friendGroups: FriendGroupInfo[];
  friends: FriendInfo[];
  friendRequests: FriendRequestInfo[];
  members: MemberInfo[];
  conversations: ConversationInfo[];
  sessions: SessionInfo[];
  busy: boolean;
  wsStatus: WsStatus;
  notificationStatus: BrowserNotificationStatus;
  notificationsEnabled: boolean;
  selectedConversationId: string | null;
  selectedConversation: ConversationInfo | null;
  selectedConversationType: ConversationInfo["type"] | null;
  selectedGroupInfo: GroupInfo | null;
  currentMember: MemberInfo | null;
  availableBots: BotInfo[];
  conversationBots: BotInfo[];
  aiCallLogs: AICallLogInfo[];
  aiCallLogQuota: AICallLogQuotaInfo;
  loadingAICallLogs: boolean;
  aiCallLogStatus: "" | "SUCCESS" | "FAILED";
  onTabChange: (tab: DetailTab) => void;
  onCreateFriendGroup: (name: string) => Promise<void>;
  onAddFriend: (input: { targetAimId: string; remark: string; groupId: number | null }) => Promise<void>;
  onRespondFriendRequest: (requestId: number, action: "ACCEPTED" | "REJECTED") => Promise<void>;
  onUpdateFriend: (friendUserId: number, input: { remark: string; groupId: number | null }) => Promise<void>;
  onDeleteFriend: (friendUserId: number) => Promise<void>;
  onOpenChatWithFriend?: (friend: FriendInfo) => void;
  onRefreshSessions: () => Promise<void>;
  onLogout: () => Promise<void>;
  onLogoutAll: (password: string) => Promise<void>;
  onAvatarUpload: (avatar: Blob) => Promise<void>;
  onRevokeSession: (sessionId: string, password: string) => Promise<void>;
  onToggleNotifications: () => Promise<void>;
  onTransferOwner: (targetUserId: number) => Promise<void>;
  onSetAdmin: (targetUserId: number) => Promise<void>;
  onRemoveAdmin: (targetUserId: number) => Promise<void>;
  onMuteMember: (targetUserId: number, durationSeconds: number) => Promise<void>;
  onUnmuteMember: (targetUserId: number) => Promise<void>;
  onRemoveMember: (targetUserId: number) => Promise<void>;
  onSetGroupMuteAll: (muteAll: boolean) => Promise<void>;
  onUpdateGroupAnnouncement: (announcement: string) => Promise<void>;
  onAddBot: (input: {
    botId: number;
    displayNameOverride?: string;
    mentionNameOverride?: string;
    aliasesOverride?: string[];
    permissionScope?: string;
    modelNameOverride?: string;
  }) => Promise<void>;
  onRemoveBot: (botId: number) => Promise<void>;
  onAICallLogStatusChange: (status: "" | "SUCCESS" | "FAILED") => void;
  onRefreshAICallLogs: () => Promise<void>;
  onMention: (mentionTarget: string) => void;
  onClose: () => void;
}) {
  return (
    <aside className={cx("pane detail-pane", active && "mobile-active")}>
      <header className="detail-header">
        <div className="segmented small-tabs tabs-five">
          <button className={tab === "friends" ? "active" : ""} type="button" onClick={() => onTabChange("friends")}>
            好友
          </button>
          <button className={tab === "members" ? "active" : ""} type="button" onClick={() => onTabChange("members")}>
            成员
          </button>
          <button className={tab === "bots" ? "active" : ""} type="button" onClick={() => onTabChange("bots")}>
            AI 助手
          </button>
          <button disabled={!selectedConversationId} className={tab === "logs" ? "active" : ""} type="button" onClick={() => onTabChange("logs")}>
            日志
          </button>
          <button className={tab === "account" ? "active" : ""} type="button" onClick={() => onTabChange("account")}>
账号
          </button>
        </div>
        <IconButton className="mobile-only" label="返回" onClick={onClose}>
          <X size={18} />
        </IconButton>
      </header>

      {tab === "friends" ? (
        <FriendsView
          busy={busy}
          friendGroups={friendGroups}
          friends={friends}
          friendRequests={friendRequests}
          onOpenChatWithFriend={onOpenChatWithFriend}
          onCreateFriendGroup={onCreateFriendGroup}
          onAddFriend={onAddFriend}
          onRespondFriendRequest={onRespondFriendRequest}
          onUpdateFriend={onUpdateFriend}
          onDeleteFriend={onDeleteFriend}
        />
      ) : tab === "members" ? (
        <MembersViewClean
          members={members}
          selectedConversation={selectedConversation}
          groupInfo={selectedGroupInfo}
          currentMember={currentMember}
          busy={busy}
          onMention={onMention}
          onTransferOwner={onTransferOwner}
          onSetAdmin={onSetAdmin}
          onRemoveAdmin={onRemoveAdmin}
          onMuteMember={onMuteMember}
          onUnmuteMember={onUnmuteMember}
          onRemoveMember={onRemoveMember}
          onSetGroupMuteAll={onSetGroupMuteAll}
          onUpdateGroupAnnouncement={onUpdateGroupAnnouncement}
        />
      ) : tab === "bots" ? (
        <BotPanelClean
          selectedConversationId={selectedConversationId}
          selectedConversationType={selectedConversationType}
          currentMember={currentMember}
          availableBots={availableBots}
          conversationBots={conversationBots}
          busy={busy}
          onAddBot={onAddBot}
          onRemoveBot={onRemoveBot}
          onMention={onMention}
        />
      ) : tab === "logs" ? (
        <AICallLogsPanel
          selectedConversationId={selectedConversationId}
          logs={aiCallLogs}
          quota={aiCallLogQuota}
          loading={loadingAICallLogs}
          statusFilter={aiCallLogStatus}
          onStatusFilterChange={onAICallLogStatusChange}
          onRefresh={onRefreshAICallLogs}
        />
      ) : (
        <AccountView
          user={user}
          sessions={sessions}
          busy={busy}
          wsStatus={wsStatus}
          notificationStatus={notificationStatus}
          notificationsEnabled={notificationsEnabled}
          onRefreshSessions={onRefreshSessions}
          onLogout={onLogout}
          onLogoutAll={onLogoutAll}
          onAvatarUpload={onAvatarUpload}
          onRevokeSession={onRevokeSession}
          onToggleNotifications={onToggleNotifications}
        />
      )}
    </aside>
  );
}

function FriendsView({
  busy,
  friendGroups,
  friends,
  friendRequests,
  onOpenChatWithFriend,
  onCreateFriendGroup,
  onAddFriend,
  onRespondFriendRequest,
  onUpdateFriend,
  onDeleteFriend
}: {
  busy: boolean;
  friendGroups: FriendGroupInfo[];
  friends: FriendInfo[];
  friendRequests: FriendRequestInfo[];
  onOpenChatWithFriend?: (friend: FriendInfo) => void;
  onCreateFriendGroup: (name: string) => Promise<void>;
  onAddFriend: (input: { targetAimId: string; remark: string; groupId: number | null }) => Promise<void>;
  onRespondFriendRequest: (requestId: number, action: "ACCEPTED" | "REJECTED") => Promise<void>;
  onUpdateFriend: (friendUserId: number, input: { remark: string; groupId: number | null }) => Promise<void>;
  onDeleteFriend: (friendUserId: number) => Promise<void>;
}) {
  const [addOpen, setAddOpen] = useState(false);
  const [groupOpen, setGroupOpen] = useState(false);
  const [targetAimId, setTargetAimId] = useState("");
  const [remark, setRemark] = useState("");
  const [groupId, setGroupId] = useState<number | null>(null);
  const [groupName, setGroupName] = useState("");
  const [selectedGroupId, setSelectedGroupId] = useState<number | null>(null);

  const filteredFriends = useMemo(() => {
    if (selectedGroupId === null) return friends;
    return friends.filter((friend) => friend.group_id === selectedGroupId);
  }, [friends, selectedGroupId]);

  const createGroup = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = groupName.trim();
    if (!name) return;
    await onCreateFriendGroup(name);
    setGroupName("");
    setGroupOpen(false);
  };

  const addFriend = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextAimId = targetAimId.trim();
    if (!nextAimId) return;
    await onAddFriend({ targetAimId: nextAimId, remark: remark.trim(), groupId });
    setTargetAimId("");
    setRemark("");
    setGroupId(null);
    setAddOpen(false);
  };

  return (
    <div className="detail-body friend-body">
      <div className="action-grid detail-actions">
        <button type="button" onClick={() => setAddOpen((value) => !value)}>
          <UserPlus size={18} />
          添加好友
        </button>
        <button type="button" onClick={() => setGroupOpen((value) => !value)}>
          <BadgePlus size={18} />
          新建分组
        </button>
      </div>

      {addOpen && (
        <form className="drawer-form" onSubmit={addFriend}>
          <input required value={targetAimId} onChange={(event) => setTargetAimId(event.target.value)} placeholder="目标 AIM ID" />
          <input value={remark} onChange={(event) => setRemark(event.target.value)} placeholder="备注（可选）" />
          <select value={groupId ?? ""} onChange={(event) => setGroupId(parseGroupValue(event.target.value))}>
            <option value="">Ungrouped</option>
            {friendGroups.map((group) => (
              <option key={group.id} value={group.id}>
                {group.name}
              </option>
            ))}
          </select>
          <span className="form-hint">A chat will be created after the request is accepted.</span>
          <button disabled={busy} type="submit">
            {busy ? <Loader2 className="spin" size={16} /> : <UserPlus size={16} />}
            发送申请
          </button>
        </form>
      )}

      {groupOpen && (
        <form className="drawer-form" onSubmit={createGroup}>
          <input required value={groupName} onChange={(event) => setGroupName(event.target.value)} placeholder="分组名称" />
          <button disabled={busy} type="submit">
            {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
            创建
          </button>
        </form>
      )}

      <div className="section-title">
        <span>好友申请</span>
        <strong>{friendRequests.length}</strong>
      </div>

      <div className="request-list">
        {friendRequests.map((request) => (
          <FriendRequestRow
            busy={busy}
            key={request.id}
            request={request}
            onRespond={onRespondFriendRequest}
          />
        ))}
        {friendRequests.length === 0 && (
          <div className="empty-block compact-empty">
            <Mail size={24} />
            <strong>暂无好友申请</strong>
          </div>
        )}
      </div>

      <div className="section-title">
        <span>好友分组</span>
        <strong>{friendGroups.length}</strong>
      </div>

      <div className="section-title">
        <span>好友</span>
        <strong>{filteredFriends.length}</strong>
      </div>

      <div className="friend-group-list">
        <button className={cx("friend-group-chip", selectedGroupId === null && "active")} type="button" onClick={() => setSelectedGroupId(null)}>
          全部
        </button>
        {friendGroups.map((group) => (
          <button
            className={cx("friend-group-chip", selectedGroupId === group.id && "active")}
            key={group.id}
            type="button"
            onClick={() => setSelectedGroupId(group.id)}
          >
            {group.name}
          </button>
        ))}
      </div>

      <div className="friend-list">
        {filteredFriends.map((friend) => (
          <FriendRow
            busy={busy}
            friend={friend}
            friendGroups={friendGroups}
            key={friend.user_id}
            onOpenChat={onOpenChatWithFriend}
            onDelete={onDeleteFriend}
            onSave={onUpdateFriend}
          />
        ))}
        {filteredFriends.length === 0 && friends.length > 0 && (
          <div className="empty-block">
            <UserRound size={28} />
            <strong>当前分组暂无好友</strong>
            <span>Move friends to another group or switch filters.</span>
          </div>
        )}
        {friends.length === 0 && (
          <div className="empty-block">
            <UserRound size={28} />
            <strong>No friends yet</strong>
            <span>You can add one by AIM ID first.</span>
          </div>
        )}
      </div>
    </div>
  );
}

function FriendRequestRow({
  busy,
  request,
  onRespond
}: {
  busy: boolean;
  request: FriendRequestInfo;
  onRespond: (requestId: number, action: "ACCEPTED" | "REJECTED") => Promise<void>;
}) {
  const incoming = request.direction === "INCOMING";
  const pending = request.status === "PENDING";

  return (
    <div className="request-card">
      <div className="friend-card-head">
        <Avatar name={request.nickname || request.aim_id} src={request.avatar} />
        <div className="friend-card-meta">
          <strong>{request.nickname || request.aim_id}</strong>
          <span>{request.aim_id} {"  "} {incoming ? "Incoming request" : "Outgoing request"}</span>
        </div>
        <StatusPill label={request.status} />
      </div>

      {request.remark && <p className="request-remark">{request.remark}</p>}

      {incoming && pending ? (
        <div className="friend-row-actions">
          <button className="secondary-button" disabled={busy} type="button" onClick={() => void onRespond(request.id, "ACCEPTED")}>
            {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
            同意
          </button>
          <button className="danger-button" disabled={busy} type="button" onClick={() => void onRespond(request.id, "REJECTED")}>
            <X size={16} />
            拒绝
          </button>
        </div>
      ) : (
        <div className="request-meta">
          <time>{formatRelative(request.updated_at || request.created_at)}</time>
        </div>
      )}
    </div>
  );
}

function FriendRow({
  busy,
  friend,
  friendGroups,
  onOpenChat,
  onSave,
  onDelete
}: {
  busy: boolean;
  friend: FriendInfo;
  friendGroups: FriendGroupInfo[];
  onOpenChat?: (friend: FriendInfo) => void;
  onSave: (friendUserId: number, input: { remark: string; groupId: number | null }) => Promise<void>;
  onDelete: (friendUserId: number) => Promise<void>;
}) {
  const [remark, setRemark] = useState(friend.remark ?? "");
  const [groupId, setGroupId] = useState<number | null>(friend.group_id ?? null);
  const [expanded, setExpanded] = useState(false);

  useEffect(() => {
    setRemark(friend.remark ?? "");
    setGroupId(friend.group_id ?? null);
  }, [friend.group_id, friend.remark, friend.user_id]);

  const assignedGroup = friendGroups.find((group) => group.id === friend.group_id);
  const handleSave = async () => {
    await onSave(friend.user_id, { remark: remark.trim(), groupId });
    setExpanded(false);
  };

  return (
    <div className="friend-card">
      <div className="friend-card-head" onClick={() => onOpenChat?.(friend)} style={{ cursor: onOpenChat ? "pointer" : "default" }}>
        <Avatar name={friend.nickname || friend.aim_id} src={friend.avatar} />
        <div className="friend-card-meta">
          <strong>{friend.remark || friend.nickname || friend.aim_id}</strong>
          <span>{friend.aim_id} {"  "} {assignedGroup?.name || "Ungrouped"}</span>
        </div>
        <div className="friend-card-side">
          <StatusPill label={friend.status} />
          <button className="friend-expand-button" type="button" onClick={(e) => { e.stopPropagation(); setExpanded((value) => !value); }}>
            {expanded ? "收起" : "编辑"}
          </button>
        </div>
      </div>

      {expanded && (
        <div className="friend-card-form">
          <label className="field compact-field">
            <span>备注</span>
            <input value={remark} onChange={(event) => setRemark(event.target.value)} placeholder="好友备注" />
          </label>

          <label className="field compact-field">
            <span>分组</span>
            <select value={groupId ?? ""} onChange={(event) => setGroupId(parseGroupValue(event.target.value))}>
              <option value="">Ungrouped</option>
              {friendGroups.map((group) => (
                <option key={group.id} value={group.id}>
                  {group.name}
                </option>
              ))}
            </select>
          </label>

          <div className="friend-row-actions">
            <button className="secondary-button" disabled={busy} type="button" onClick={() => void handleSave()}>
              {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
              保存
            </button>
            <button className="danger-button" disabled={busy} type="button" onClick={() => void onDelete(friend.user_id)}>
              <Trash2 size={16} />
              删除
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function BotPanel({
  selectedConversationId,
  currentMember,
  availableBots,
  conversationBots,
  busy,
  onAddBot,
  onRemoveBot,
  onMention
}: {
  selectedConversationId: string | null;
  currentMember: MemberInfo | null;
  availableBots: BotInfo[];
  conversationBots: BotInfo[];
  busy: boolean;
  onAddBot: (input: { botId: number; displayNameOverride?: string; mentionNameOverride?: string; aliasesOverride?: string[]; permissionScope?: string }) => Promise<void>;
  onRemoveBot: (botId: number) => Promise<void>;
  onMention: (mentionTarget: string) => void;
}) {
  const [addOpen, setAddOpen] = useState(false);
  const [selectedBotId, setSelectedBotId] = useState<number | "">("");
  const [displayNameOverride, setDisplayNameOverride] = useState("");
  const [mentionNameOverride, setMentionNameOverride] = useState("");
  const [aliasesOverrideText, setAliasesOverrideText] = useState("");

  const canManage = currentMember?.role === "OWNER" || currentMember?.role === "ADMIN";
  const addedBotIds = new Set(conversationBots.map((b) => b.botId));
  const candidateBots = availableBots.filter((b) => !addedBotIds.has(b.botId));

  const handleAdd = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (typeof selectedBotId !== "number" || selectedBotId <= 0) return;
    const aliases = aliasesOverrideText.split(",").map((a) => a.trim()).filter(Boolean);
    await onAddBot({
      botId: selectedBotId,
      displayNameOverride: displayNameOverride.trim() || undefined,
      mentionNameOverride: mentionNameOverride.trim() || undefined,
      aliasesOverride: aliases.length > 0 ? aliases : undefined
    });
    setSelectedBotId("");
    setDisplayNameOverride("");
    setMentionNameOverride("");
    setAliasesOverrideText("");
    setAddOpen(false);
  };

  return (
    <div className="detail-body bot-body">
      {!selectedConversationId ? (
        <div className="empty-block">
          <Bot size={30} />
          <strong>Select a group conversation</strong>
          <span>进入会话后可管理 AI 助手</span>
        </div>
      ) : (
        <>
          {canManage && (
            <>
              <div className="action-grid detail-actions">
                <button type="button" onClick={() => setAddOpen((value) => !value)}>
                  <BadgePlus size={18} />
                  添加助手
                </button>
              </div>

              {addOpen && (
                <form className="drawer-form" onSubmit={handleAdd}>
                  <label className="field">
                    <span>选择 Bot</span>
                    <select value={selectedBotId} onChange={(e) => setSelectedBotId(e.target.value ? Number(e.target.value) : "")}>
                      <option value="">请选择</option>
                      {candidateBots.map((bot) => (
                        <option key={bot.botId} value={bot.botId}>
                          {bot.displayName || bot.name}
                        </option>
                      ))}
                    </select>
                  </label>

                  {candidateBots.length === 0 && <span className="form-hint">All available bots are already added to this conversation.</span>}

                  <input value={displayNameOverride} onChange={(e) => setDisplayNameOverride(e.target.value)} placeholder="显示名称覆盖（可选）" />
                  <input value={mentionNameOverride} onChange={(e) => setMentionNameOverride(e.target.value)} placeholder="@提及名覆盖（可选）" />
                  <input value={aliasesOverrideText} onChange={(e) => setAliasesOverrideText(e.target.value)} placeholder="别名覆盖，逗号分隔（可选）" />
                  <span className="form-hint">Only OWNER and ADMIN can override these fields.</span>
                  <button disabled={busy || typeof selectedBotId !== "number"} type="submit">
                    {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
                    添加到会话?
                  </button>
                </form>
              )}
            </>
          )}

          <div className="section-title">
            <span>会话中的 AI 助手</span>
            <strong>{conversationBots.length}</strong>
          </div>

          <div className="bot-list">
            {conversationBots.map((bot) => (
              <BotCard key={bot.botId} bot={bot} canManage={canManage} busy={busy} onRemove={() => void onRemoveBot(bot.botId)} onMention={onMention} />
            ))}
            {conversationBots.length === 0 && (
              <div className="empty-block compact-empty">
                <Bot size={24} />
                <strong>暂无 AI 助手</strong>
                <span>{canManage ? "Use the add action above to add an assistant." : "Wait for an admin to add an assistant."}</span>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

function BotCard({
  bot,
  canManage,
  busy,
  onRemove,
  onMention
}: {
  bot: BotInfo;
  canManage: boolean;
  busy: boolean;
  onRemove: () => void;
  onMention: (mentionTarget: string) => void;
}) {
  return (
    <div className="friend-card bot-card">
      <div className="friend-card-head">
        <Avatar name={bot.displayName || bot.name} src={bot.avatar} onContextMenu={(event) => handleAvatarMention(event, bot.mentionName, onMention)} />
        <div className="friend-card-meta">
          <strong>{bot.displayName || bot.name}</strong>
          <span>@{bot.mentionName} {" · "} {bot.memberType === "BOT" ? "Bot" : bot.memberType}</span>
        </div>
        <StatusPill label={bot.enabled ? "启用" : "禁用"} />
      </div>

      {bot.description && <p className="request-remark">{bot.description}</p>}

      <div className="bot-detail-fields">
        <div className="bot-field-row">
          <span className="bot-field-label">Mention</span>
          <span className="bot-field-value">@{bot.mentionName}</span>
        </div>
        {bot.aliases.length > 0 && (
          <div className="bot-field-row">
            <span className="bot-field-label">别名</span>
            <span className="bot-field-value">{bot.aliases.join(", ")}</span>
          </div>
        )}
        <div className="bot-field-row">
          <span className="bot-field-label">权限范围</span>
          <span className="bot-field-value">{bot.permissionScope}</span>
        </div>
        <div className="bot-field-row">
          <span className="bot-field-label">Status</span>
          <StatusPill label={bot.memberStatus} />
        </div>
      </div>

      {canManage && (
        <div className="friend-row-actions">
          <button className="danger-button" disabled={busy} type="button" onClick={onRemove}>
            <Trash2 size={16} />
            移除
          </button>
        </div>
      )}
    </div>
  );
}

function MembersView({ members, onMention }: { members: MemberInfo[]; onMention: (mentionTarget: string) => void }) {
  const isBot = (m: MemberInfo) => m.memberType === "BOT";

  return (
    <div className="detail-body">
      <div className="section-title">
        <span>Members</span>
        <strong>{members.length}</strong>
      </div>
      <div className="member-list">
        {members.map((member) => isBot(member) ? (
          <div className="member-row member-bot-row" key={`bot-${member.botId ?? member.userId}`}>
            <Avatar name={member.nickname || "Bot"} src={member.avatar} onContextMenu={(event) => handleAvatarMention(event, member.mentionName || member.nickname, onMention)} />
            <div>
              <strong>{member.nickname || `Bot ${member.botId ?? member.userId}`}</strong>
              <span><StatusPill label="AI" /> {"  "}@{member.mentionName ?? "bot"}{member.enabled === false && <StatusPill label="Disabled" />}</span>
              {member.aliases && member.aliases.length > 0 && <span className="bot-member-aliases">Aliases: {member.aliases.join(", ")}</span>}
            </div>
          </div>
        ) : (
          <div className="member-row" key={member.userId}>
            <Avatar name={member.nickname || String(member.userId)} src={member.avatar} onContextMenu={(event) => handleAvatarMention(event, member.nickname, onMention)} />
            <div>
              <strong>{member.nickname || `用户 ${member.userId}`}</strong>
              <span>{roleLabel(member.role)} {" · "} {statusLabel(member.status)}</span>
            </div>
          </div>
        ))}
        {members.length === 0 && (
          <div className="empty-block">
            <UsersRound size={28} />
            <strong>暂无成员数据</strong>
          </div>
        )}
      </div>
    </div>
  );
}

function AICallLogsPanel({
  selectedConversationId,
  logs,
  quota,
  loading,
  statusFilter,
  onStatusFilterChange,
  onRefresh
}: {
  selectedConversationId: string | null;
  logs: AICallLogInfo[];
  quota: AICallLogQuotaInfo;
  loading: boolean;
  statusFilter: "" | "SUCCESS" | "FAILED";
  onStatusFilterChange: (status: "" | "SUCCESS" | "FAILED") => void;
  onRefresh: () => Promise<void>;
}) {
  const usagePercent = quota.dailyTokenLimit > 0 ? Math.min(100, Math.round((quota.dailyTotalTokens / quota.dailyTokenLimit) * 100)) : 0;

  if (!selectedConversationId) {
    return (
      <div className="detail-body log-body">
        <div className="empty-block">
          <Bot size={28} />
          <strong>Select a group conversation</strong>
          <span>Open a group chat to inspect AI call logs.</span>
        </div>
      </div>
    );
  }

  return (
    <div className="detail-body log-body">
      <div className="section-title">
        <span>AI 调用记录</span>
        <IconButton label="刷新日志" onClick={() => void onRefresh()}>
          <RefreshCw size={16} />
        </IconButton>
      </div>

      <section className="quota-card">
        <div className="quota-card-head">
          <strong>今日额度</strong>
          <span>{quota.dailyTotalTokens.toLocaleString()} / {quota.dailyTokenLimit.toLocaleString()} tokens</span>
        </div>
        <div aria-hidden="true" className="quota-bar">
          <span className="quota-bar-fill" style={{ width: `${usagePercent}%` }} />
        </div>
        <div className="quota-card-meta">
          <span>剩余 {quota.remainingTokens.toLocaleString()} tokens</span>
          <span>{usagePercent}%</span>
        </div>
      </section>

      <div className="log-filter-list">
        <button className={cx("friend-group-chip", statusFilter === "" && "active")} type="button" onClick={() => onStatusFilterChange("")}>
          全部
        </button>
        <button className={cx("friend-group-chip", statusFilter === "SUCCESS" && "active")} type="button" onClick={() => onStatusFilterChange("SUCCESS")}>
          成功
        </button>
        <button className={cx("friend-group-chip", statusFilter === "FAILED" && "active")} type="button" onClick={() => onStatusFilterChange("FAILED")}>
          失败
        </button>
      </div>

      <div className="log-list">
        {loading ? (
          <div className="empty-block compact-empty">
            <Loader2 className="spin" size={24} />
            <strong>正在加载调用记录</strong>
          </div>
        ) : logs.length === 0 ? (
          <div className="empty-block compact-empty">
            <Bot size={24} />
            <strong>暂无调用记录</strong>
            <span>AI call logs will appear here after the assistant is triggered in this group.</span>
          </div>
        ) : (
          logs.map((log) => (
            <article className="log-card" key={log.id}>
              <div className="log-card-head">
                <div className="log-card-meta">
                  <strong>{log.botName || `Bot ${log.botId}`}</strong>
                  <span>{log.modelName || "unknown model"}</span>
                </div>
                <span className={cx("log-status", log.status === "FAILED" && "failed")}>{log.status}</span>
              </div>

              <div className="log-card-stats">
                <span>用户 {log.userId}</span>
                <span>{log.totalTokens} tokens</span>
                <span>{log.latencyMs} ms</span>
                <time>{formatRelative(log.createdAt)}</time>
              </div>

              <div className="log-id-row">
                <span>请求 #{log.requestMessageId ?? "-"}</span>
                <span>回复 #{log.responseMessageId ?? "-"}</span>
              </div>

              {log.errorMessage && <p className="log-error">{log.errorMessage}</p>}
            </article>
          ))
        )}
      </div>
    </div>
  );
}

function MembersViewLegacy({ members, onMention }: { members: MemberInfo[]; onMention: (mentionTarget: string) => void }) {
  return (
    <div className="detail-body">
      <div className="section-title">
        <span>Members</span>
        <strong>{members.length}</strong>
      </div>
      <div className="member-list">
        {members.map((member) =>
          member.memberType === "BOT" ? (
            <div className="member-row member-bot-row" key={`bot-${member.botId ?? member.userId}`}>
              <Avatar
                name={member.nickname || "Bot"}
                src={member.avatar}
                onContextMenu={(event) => handleAvatarMention(event, member.mentionName || member.nickname, onMention)}
              />
              <div>
                <strong>{member.nickname || `Bot ${member.botId ?? member.userId}`}</strong>
                <span>
                  <StatusPill label="AI" /> {"  "}@{member.mentionName ?? "bot"}
                  {member.enabled === false && <StatusPill label="Disabled" />}
                </span>
                {member.aliases && member.aliases.length > 0 && (
                  <span className="bot-member-aliases">Aliases: {member.aliases.join(", ")}</span>
                )}
              </div>
            </div>
          ) : (
            <div className="member-row" key={member.userId}>
              <Avatar
                name={member.nickname || String(member.userId)}
                src={member.avatar}
                onContextMenu={(event) => handleAvatarMention(event, member.nickname, onMention)}
              />
              <div>
                <strong>{member.nickname || `用户 ${member.userId}`}</strong>
                <span>{roleLabel(member.role)} {" · "} {statusLabel(member.status)}</span>
              </div>
            </div>
          )
        )}
        {members.length === 0 && (
          <div className="empty-block">
            <UsersRound size={28} />
            <strong>暂无成员数据</strong>
          </div>
        )}
      </div>
    </div>
  );
}

/* function MembersViewClean({
  members,
  selectedConversation,
  currentMember,
  busy,
  onMention,
  onTransferOwner,
  onSetAdmin,
  onRemoveAdmin,
  onMuteMember,
  onUnmuteMember,
  onRemoveMember,
  onSetGroupMuteAll
}: {
  members: MemberInfo[];
  selectedConversation: ConversationInfo | null;
  currentMember: MemberInfo | null;
  busy: boolean;
  onMention: (mentionTarget: string) => void;
  onTransferOwner: (targetUserId: number) => Promise<void>;
  onSetAdmin: (targetUserId: number) => Promise<void>;
  onRemoveAdmin: (targetUserId: number) => Promise<void>;
  onMuteMember: (targetUserId: number, durationSeconds: number) => Promise<void>;
  onUnmuteMember: (targetUserId: number) => Promise<void>;
  onRemoveMember: (targetUserId: number) => Promise<void>;
  onSetGroupMuteAll: (muteAll: boolean) => Promise<void>;
}) {
  const [muteDurations, setMuteDurations] = useState<Record<number, number>>({});
  const [expandedMemberId, setExpandedMemberId] = useState<number | null>(null);
  const isGroup = selectedConversation?.type === "GROUP";
  const canToggleMuteAll = isGroup && currentMember?.role === "OWNER";
  const muteAllEnabled = Boolean(selectedConversation?.muteAll);
  const durationOptions = [
    { label: "10分钟", value: 10 * 60 },
    { label: "1小时", value: 60 * 60 },
    { label: "24小时", value: 24 * 60 * 60 }
  ];

  const getMuteDuration = (userId: number) => muteDurations[userId] ?? durationOptions[0].value;

  useEffect(() => {
    setExpandedMemberId(null);
  }, [selectedConversation?.conversationId]);

  const canTransfer = (member: MemberInfo) =>
    isGroup &&
    currentMember?.role === "OWNER" &&
    member.memberType !== "BOT" &&
    member.userId !== currentMember.userId &&
    (member.role === "MEMBER" || member.role === "ADMIN");

  const canSetAdminFor = (member: MemberInfo) =>
    isGroup &&
    currentMember?.role === "OWNER" &&
    member.memberType !== "BOT" &&
    member.userId !== currentMember.userId &&
    member.role === "MEMBER";

  const canRemoveAdminFor = (member: MemberInfo) =>
    isGroup &&
    currentMember?.role === "OWNER" &&
    member.memberType !== "BOT" &&
    member.userId !== currentMember.userId &&
    member.role === "ADMIN";

  const canManageMember = (member: MemberInfo) => {
    if (!isGroup || member.memberType === "BOT" || !currentMember || member.userId === currentMember.userId) {
      return false;
    }
    if (currentMember.role === "OWNER") {
      return member.role === "MEMBER" || member.role === "ADMIN";
    }
    if (currentMember.role === "ADMIN") {
      return member.role === "MEMBER";
    }
    return false;
  };

  const hasManagementActions = (member: MemberInfo) =>
    canTransfer(member) || canSetAdminFor(member) || canRemoveAdminFor(member) || canManageMember(member);

  return (
    <div className="detail-body">
      <div className="section-title">
        <span>Members</span>
        <strong>{members.length}</strong>
      </div>
      {canToggleMuteAll && (
        <div className="member-toolbar">
          <button
            className={cx("secondary-button", "compact-button", muteAllEnabled && "member-toolbar-active")}
            disabled={busy}
            type="button"
            onClick={() => void onSetGroupMuteAll(!muteAllEnabled)}
          >
            <LockKeyhole size={14} />
            {muteAllEnabled ? "关闭全员禁言" : "开启全员禁言"}
          </button>
        </div>
      )}
      <div className="member-list">
        {members.map((member) =>
          member.memberType === "BOT" ? (
            <div className="member-row member-bot-row" key={`bot-${member.botId ?? member.userId}`}>
              <Avatar
                name={member.nickname || "Bot"}
                src={member.avatar}
                onContextMenu={(event) => handleAvatarMention(event, member.mentionName || member.nickname, onMention)}
              />
              <div>
                <strong>{member.nickname || `Bot ${member.botId ?? member.userId}`}</strong>
                <span>
                  <StatusPill label="AI 助手" /> {" ?"}@{member.mentionName ?? "?"}
                  {member.enabled === false && <StatusPill label="已禁用" />}
                </span>
                {member.aliases && member.aliases.length > 0 && (
                  <span className="bot-member-aliases">Aliases: {member.aliases.join(", ")}</span>
                )}
              </div>
            </div>
          ) : (
            <div className="member-row" key={member.userId}>
              <Avatar
                name={member.nickname || String(member.userId)}
                src={member.avatar}
                onContextMenu={(event) => handleAvatarMention(event, member.nickname, onMention)}
              />
              <div className="member-meta-stack">
                <strong>{member.nickname || `用户 ${member.userId}`}</strong>
                <span>{roleLabel(member.role)} {" ?"} {statusLabel(member.status)}</span>
                {isMemberMuted(member) && (
                  <span className="member-extra-note">已禁言?{formatMuteUntil(member.muteUntil)}</span>
                )}
                {member.userId === currentMember?.userId && muteAllEnabled && currentMember.role !== "OWNER" && currentMember.role !== "ADMIN" && (
                  <span className="member-extra-note">当前群已开启全员禁言</span>
                )}
                {hasManagementActions(member) && (
                  <>
                    <button
                      className={cx("member-expand-toggle", expandedMemberId === member.userId && "is-open")}
                      disabled={busy}
                      type="button"
                      onClick={() =>
                        setExpandedMemberId((current) => (current === member.userId ? null : member.userId))
                      }
                    >
                      <span>Manage</span>
                      <ChevronDown size={16} />
                    </button>
                  </>
                )}
                {hasManagementActions(member) && expandedMemberId === member.userId && (
                  <div className="member-actions">
                    {canTransfer(member) && (
                      <button
                        className="secondary-button compact-button"
                        disabled={busy}
                        type="button"
                        onClick={() => {
                          if (window.confirm(`确认将群主转让给 ${member.nickname || `用户 ${member.userId}`} 吗？`)) {
                            void onTransferOwner(member.userId);
                          }
                        }}
                      >
                        <ShieldCheck size={14} />
                        转让群主
                      </button>
                    )}
                    {canSetAdminFor(member) && (
                      <button
                        className="secondary-button compact-button"
                        disabled={busy}
                        type="button"
                        onClick={() => void onSetAdmin(member.userId)}
                      >
                        <ShieldCheck size={14} />
                        设为管理�?                      </button>
                    )}
                    {canRemoveAdminFor(member) && (
                      <button
                        className="secondary-button compact-button"
                        disabled={busy}
                        type="button"
                        onClick={() => void onRemoveAdmin(member.userId)}
                      >
                        <ShieldCheck size={14} />
                        取消管理�?                      </button>
                    )}
                    {canManageMember(member) && (
                      <div className="member-action-row">
                        {isMemberMuted(member) ? (
                          <button
                            className="secondary-button compact-button"
                            disabled={busy}
                            type="button"
                            onClick={() => void onUnmuteMember(member.userId)}
                          >
                            <LockKeyhole size={14} />
                            解除禁言
                          </button>
                        ) : (
                          <>
                            <select
                              className="member-action-select"
                              value={getMuteDuration(member.userId)}
                              onChange={(event) =>
                                setMuteDurations((current) => ({
                                  ...current,
                                  [member.userId]: Number(event.target.value)
                                }))
                              }
                            >
                              {durationOptions.map((option) => (
                                <option key={option.value} value={option.value}>
                                  {option.label}
                                </option>
                              ))}
                            </select>
                            <button
                              className="secondary-button compact-button"
                              disabled={busy}
                              type="button"
                              onClick={() => void onMuteMember(member.userId, getMuteDuration(member.userId))}
                            >
                              <LockKeyhole size={14} />
                              禁言
                            </button>
                          </>
                        )}
                        <button
                          className="danger-button compact-button"
                          disabled={busy}
                          type="button"
                          onClick={() => {
                            if (window.confirm(`确认�?${member.nickname || `用户 ${member.userId}`} 移出群聊吗？`)) {
                              void onRemoveMember(member.userId);
                            }
                          }}
                        >
                          <Trash2 size={14} />
                          移出群聊
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          )
        )}
        {members.length === 0 && (
          <div className="empty-block">
            <UsersRound size={28} />
            <strong>暂无成员数据</strong>
          </div>
        )}
      </div>
    </div>
  );
}

*/
function GroupAnnouncementCard({
  groupInfo,
  members,
  currentMember,
  busy,
  onSave
}: {
  groupInfo: GroupInfo | null;
  members: MemberInfo[];
  currentMember: MemberInfo | null;
  busy: boolean;
  onSave: (announcement: string) => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(groupInfo?.announcement ?? "");

  useEffect(() => {
    setDraft(groupInfo?.announcement ?? "");
    setEditing(false);
  }, [groupInfo?.announcement, groupInfo?.announcementUpdatedAt, groupInfo?.announcementUpdatedBy]);

  const canEdit = Boolean(groupInfo) && (currentMember?.role === "OWNER" || currentMember?.role === "ADMIN");
  const announcement = groupInfo?.announcement?.trim() ?? "";
  const nextAnnouncement = draft.trim();
  const updater =
    typeof groupInfo?.announcementUpdatedBy === "number"
      ? members.find((member) => member.userId === groupInfo.announcementUpdatedBy)
      : null;
  const updatedMeta =
    typeof groupInfo?.announcementUpdatedAt === "number"
      ? `Updated ${formatRelative(groupInfo.announcementUpdatedAt)}${
          typeof groupInfo.announcementUpdatedBy === "number"
            ? ` by ${updater?.nickname || `User ${groupInfo.announcementUpdatedBy}`}`
            : ""
        }`
      : "";

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    await onSave(draft);
  };

  return (
    <section className="group-announcement-card">
      <div className="group-announcement-header">
        <div>
          <strong>Announcement</strong>
          <span>Shown in the group details area for all members.</span>
        </div>
        {canEdit && !editing && (
          <button className="secondary-button compact-button" disabled={busy} type="button" onClick={() => setEditing(true)}>
            Edit
          </button>
        )}
      </div>

      {groupInfo === null ? (
        <p className="group-announcement-empty">Loading group announcement...</p>
      ) : editing ? (
        <form className="group-announcement-form" onSubmit={handleSubmit}>
          <textarea
            maxLength={2000}
            placeholder="Write a group announcement"
            rows={5}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
          <div className="group-announcement-actions">
            <span className="form-hint">{draft.trim().length}/2000</span>
            <button
              className="secondary-button compact-button"
              disabled={busy || nextAnnouncement === announcement}
              type="submit"
            >
              Save
            </button>
            <button
              className="secondary-button compact-button"
              disabled={busy}
              type="button"
              onClick={() => {
                setDraft(groupInfo?.announcement ?? "");
                setEditing(false);
              }}
            >
              Cancel
            </button>
          </div>
        </form>
      ) : announcement ? (
        <p className="group-announcement-content">{announcement}</p>
      ) : (
        <p className="group-announcement-empty">{canEdit ? "No announcement yet. Add one for the group." : "No announcement yet."}</p>
      )}

      {updatedMeta && <div className="group-announcement-meta">{updatedMeta}</div>}
    </section>
  );
}

function MembersViewClean({
  members,
  selectedConversation,
  groupInfo,
  currentMember,
  busy,
  onMention,
  onTransferOwner,
  onSetAdmin,
  onRemoveAdmin,
  onMuteMember,
  onUnmuteMember,
  onRemoveMember,
  onSetGroupMuteAll,
  onUpdateGroupAnnouncement
}: {
  members: MemberInfo[];
  selectedConversation: ConversationInfo | null;
  groupInfo: GroupInfo | null;
  currentMember: MemberInfo | null;
  busy: boolean;
  onMention: (mentionTarget: string) => void;
  onTransferOwner: (targetUserId: number) => Promise<void>;
  onSetAdmin: (targetUserId: number) => Promise<void>;
  onRemoveAdmin: (targetUserId: number) => Promise<void>;
  onMuteMember: (targetUserId: number, durationSeconds: number) => Promise<void>;
  onUnmuteMember: (targetUserId: number) => Promise<void>;
  onRemoveMember: (targetUserId: number) => Promise<void>;
  onSetGroupMuteAll: (muteAll: boolean) => Promise<void>;
  onUpdateGroupAnnouncement: (announcement: string) => Promise<void>;
}) {
  const [muteDurations, setMuteDurations] = useState<Record<number, number>>({});
  const [expandedMemberId, setExpandedMemberId] = useState<number | null>(null);
  const isGroup = selectedConversation?.type === "GROUP";
  const canToggleMuteAll = isGroup && currentMember?.role === "OWNER";
  const muteAllEnabled = Boolean(selectedConversation?.muteAll);
  const durationOptions = [
    { label: "10 min", value: 10 * 60 },
    { label: "1 hour", value: 60 * 60 },
    { label: "24 hours", value: 24 * 60 * 60 }
  ];

  const getMuteDuration = (userId: number) => muteDurations[userId] ?? durationOptions[0].value;

  const canTransfer = (member: MemberInfo) =>
    isGroup &&
    currentMember?.role === "OWNER" &&
    member.memberType !== "BOT" &&
    member.userId !== currentMember.userId &&
    (member.role === "MEMBER" || member.role === "ADMIN");

  const canSetAdminFor = (member: MemberInfo) =>
    isGroup &&
    currentMember?.role === "OWNER" &&
    member.memberType !== "BOT" &&
    member.userId !== currentMember.userId &&
    member.role === "MEMBER";

  const canRemoveAdminFor = (member: MemberInfo) =>
    isGroup &&
    currentMember?.role === "OWNER" &&
    member.memberType !== "BOT" &&
    member.userId !== currentMember.userId &&
    member.role === "ADMIN";

  const canManageMember = (member: MemberInfo) => {
    if (!isGroup || member.memberType === "BOT" || !currentMember || member.userId === currentMember.userId) {
      return false;
    }
    if (currentMember.role === "OWNER") {
      return member.role === "MEMBER" || member.role === "ADMIN";
    }
    if (currentMember.role === "ADMIN") {
      return member.role === "MEMBER";
    }
    return false;
  };

  const hasManagementActions = (member: MemberInfo) =>
    canTransfer(member) || canSetAdminFor(member) || canRemoveAdminFor(member) || canManageMember(member);

  useEffect(() => {
    setExpandedMemberId(null);
  }, [selectedConversation?.conversationId]);

  return (
    <div className="detail-body">
      {isGroup && (
        <GroupAnnouncementCard
          groupInfo={groupInfo}
          members={members}
          currentMember={currentMember}
          busy={busy}
          onSave={onUpdateGroupAnnouncement}
        />
      )}
      <div className="section-title">
        <span>Members</span>
        <strong>{members.length}</strong>
      </div>
      {canToggleMuteAll && (
        <div className="member-toolbar">
          <button
            className={cx("secondary-button", "compact-button", muteAllEnabled && "member-toolbar-active")}
            disabled={busy}
            type="button"
            onClick={() => void onSetGroupMuteAll(!muteAllEnabled)}
          >
            <LockKeyhole size={14} />
            {muteAllEnabled ? "Disable mute all" : "Enable mute all"}
          </button>
        </div>
      )}
      <div className="member-list">
        {members.map((member) =>
          member.memberType === "BOT" ? (
            <div className="member-row member-bot-row" key={`bot-${member.botId ?? member.userId}`}>
              <Avatar
                name={member.nickname || "Bot"}
                src={member.avatar}
                onContextMenu={(event) => handleAvatarMention(event, member.mentionName || member.nickname, onMention)}
              />
              <div>
                <strong>{member.nickname || `Bot ${member.botId ?? member.userId}`}</strong>
                <span>
                  <StatusPill label="AI" /> {"  "}@{member.mentionName ?? "bot"}
                  {member.enabled === false && <StatusPill label="Disabled" />}
                </span>
                {member.aliases && member.aliases.length > 0 && (
                  <span className="bot-member-aliases">Aliases: {member.aliases.join(", ")}</span>
                )}
              </div>
            </div>
          ) : (
            <div className="member-row" key={member.userId}>
              <Avatar
                name={member.nickname || String(member.userId)}
                src={member.avatar}
                onContextMenu={(event) => handleAvatarMention(event, member.nickname, onMention)}
              />
              <div className="member-meta-stack">
                <strong>{member.nickname || `User ${member.userId}`}</strong>
                <span>
                  {roleLabel(member.role)} {"  "} {statusLabel(member.status)}
                </span>
                {isMemberMuted(member) && (
                  <span className="member-extra-note">Muted until {formatMuteUntil(member.muteUntil)}</span>
                )}
                {member.userId === currentMember?.userId &&
                  muteAllEnabled &&
                  currentMember.role !== "OWNER" &&
                  currentMember.role !== "ADMIN" && (
                    <span className="member-extra-note">This group is currently muted for members.</span>
                  )}
                {hasManagementActions(member) && (
                  <button
                    className={cx("member-expand-toggle", expandedMemberId === member.userId && "is-open")}
                    disabled={busy}
                    type="button"
                    onClick={() => setExpandedMemberId((current) => (current === member.userId ? null : member.userId))}
                  >
                    <span>Manage</span>
                    <ChevronDown size={16} />
                  </button>
                )}
                {hasManagementActions(member) && expandedMemberId === member.userId && (
                  <div className="member-actions">
                    {canTransfer(member) && (
                      <button
                        className="secondary-button compact-button"
                        disabled={busy}
                        type="button"
                        onClick={() => {
                          if (window.confirm(`Transfer ownership to ${member.nickname || `User ${member.userId}`}?`)) {
                            void onTransferOwner(member.userId);
                          }
                        }}
                      >
                        <ShieldCheck size={14} />
                        Transfer owner
                      </button>
                    )}
                    {canSetAdminFor(member) && (
                      <button
                        className="secondary-button compact-button"
                        disabled={busy}
                        type="button"
                        onClick={() => void onSetAdmin(member.userId)}
                      >
                        <ShieldCheck size={14} />
                        Set admin
                      </button>
                    )}
                    {canRemoveAdminFor(member) && (
                      <button
                        className="secondary-button compact-button"
                        disabled={busy}
                        type="button"
                        onClick={() => void onRemoveAdmin(member.userId)}
                      >
                        <ShieldCheck size={14} />
                        Remove admin
                      </button>
                    )}
                    {canManageMember(member) && (
                      <div className="member-action-row">
                        {isMemberMuted(member) ? (
                          <button
                            className="secondary-button compact-button"
                            disabled={busy}
                            type="button"
                            onClick={() => void onUnmuteMember(member.userId)}
                          >
                            <LockKeyhole size={14} />
                            Unmute
                          </button>
                        ) : (
                          <>
                            <select
                              className="member-action-select"
                              value={getMuteDuration(member.userId)}
                              onChange={(event) =>
                                setMuteDurations((current) => ({
                                  ...current,
                                  [member.userId]: Number(event.target.value)
                                }))
                              }
                            >
                              {durationOptions.map((option) => (
                                <option key={option.value} value={option.value}>
                                  {option.label}
                                </option>
                              ))}
                            </select>
                            <button
                              className="secondary-button compact-button"
                              disabled={busy}
                              type="button"
                              onClick={() => void onMuteMember(member.userId, getMuteDuration(member.userId))}
                            >
                              <LockKeyhole size={14} />
                              Mute
                            </button>
                          </>
                        )}
                        <button
                          className="danger-button compact-button"
                          disabled={busy}
                          type="button"
                          onClick={() => {
                            if (window.confirm(`Remove ${member.nickname || `User ${member.userId}`} from the group?`)) {
                              void onRemoveMember(member.userId);
                            }
                          }}
                        >
                          <Trash2 size={14} />
                          Remove member
                        </button>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          )
        )}
        {members.length === 0 && (
          <div className="empty-block">
            <UsersRound size={28} />
            <strong>No member data</strong>
          </div>
        )}
      </div>
    </div>
  );
}
function BotPanelClean({
  selectedConversationId,
  selectedConversationType,
  currentMember,
  availableBots,
  conversationBots,
  busy,
  onAddBot,
  onRemoveBot,
  onMention
}: {
  selectedConversationId: string | null;
  selectedConversationType: ConversationInfo["type"] | null;
  currentMember: MemberInfo | null;
  availableBots: BotInfo[];
  conversationBots: BotInfo[];
  busy: boolean;
  onAddBot: (input: {
    botId: number;
    displayNameOverride?: string;
    mentionNameOverride?: string;
    aliasesOverride?: string[];
    permissionScope?: string;
    modelNameOverride?: string;
  }) => Promise<void>;
  onRemoveBot: (botId: number) => Promise<void>;
  onMention: (mentionTarget: string) => void;
}) {
  const [addOpen, setAddOpen] = useState(false);
  const [selectedBotId, setSelectedBotId] = useState<number | "">("");
  const [selectedModelName, setSelectedModelName] = useState("");

  const canManage = currentMember?.role === "OWNER" || currentMember?.role === "ADMIN";
  const addedBotIds = new Set(conversationBots.map((item) => item.botId));
  const candidateBots = availableBots.filter((item) => !addedBotIds.has(item.botId));
  const selectedBot =
    typeof selectedBotId === "number" ? candidateBots.find((item) => item.botId === selectedBotId) ?? null : null;

  useEffect(() => {
    if (!selectedBot) {
      setSelectedModelName("");
      return;
    }
    setSelectedModelName(selectedBot.modelName || selectedBot.supportedModels?.[0] || "");
  }, [selectedBot]);

  const handleAdd = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (typeof selectedBotId !== "number" || selectedBotId <= 0 || !selectedModelName) return;
    await onAddBot({
      botId: selectedBotId,
      modelNameOverride: selectedModelName
    });
    setSelectedBotId("");
    setSelectedModelName("");
    setAddOpen(false);
  };

  return (
    <div className="detail-body bot-body">
      {!selectedConversationId ? (
        <div className="empty-block">
          <Bot size={30} />
          <strong>Select a group conversation</strong>
          <span>进入会话后可管理 AI 助手</span>
        </div>
      ) : selectedConversationType !== "GROUP" ? (
        <div className="empty-block">
          <Bot size={30} />
          <strong>单聊不支持添加 AI 助手</strong>
          <span>Bots can currently be added only in group conversations.</span>
        </div>
      ) : (
        <>
          {canManage && (
            <>
              <div className="action-grid detail-actions">
                <button type="button" onClick={() => setAddOpen((value) => !value)}>
                  <BadgePlus size={18} />
                  添加助手
                </button>
              </div>

              {addOpen && (
                <form className="drawer-form" onSubmit={handleAdd}>
                  <label className="field">
                    <span>选择 Bot</span>
                    <select value={selectedBotId} onChange={(event) => setSelectedBotId(event.target.value ? Number(event.target.value) : "")}>
                      <option value="">请选择</option>
                      {candidateBots.map((item) => (
                        <option key={item.botId} value={item.botId}>
                          {item.displayName || item.name}
                        </option>
                      ))}
                    </select>
                  </label>

                  <label className="field">
                    <span>选择模型</span>
                    <select
                      value={selectedModelName}
                      onChange={(event) => setSelectedModelName(event.target.value)}
                      disabled={!selectedBot || (selectedBot.supportedModels?.length ?? 0) === 0}
                    >
                      <option value="">请选择模型</option>
                      {(selectedBot?.supportedModels ?? []).map((modelName) => (
                        <option key={modelName} value={modelName}>
                          {modelName}
                        </option>
                      ))}
                    </select>
                  </label>

                  {candidateBots.length === 0 && <span className="form-hint">All available bots are already in this group.</span>}
                  <span className="form-hint">DeepSeek platform bots are currently available here.</span>
                  <button disabled={busy || typeof selectedBotId !== "number" || !selectedModelName} type="submit">
                    {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
                    添加到会话?
                  </button>
                </form>
              )}
            </>
          )}

          <div className="section-title">
            <span>会话中的 AI 助手</span>
            <strong>{conversationBots.length}</strong>
          </div>

          <div className="bot-list">
            {conversationBots.map((item) => (
              <BotCardClean
                key={item.botId}
                bot={item}
                canManage={canManage}
                busy={busy}
                onRemove={() => void onRemoveBot(item.botId)}
                onMention={onMention}
                onSaveModel={(modelName) =>
                  onAddBot({
                    botId: item.botId,
                    modelNameOverride: modelName
                  })
                }
              />
            ))}
            {conversationBots.length === 0 && (
              <div className="empty-block compact-empty">
                <Bot size={24} />
                <strong>暂无 AI 助手</strong>
                <span>{canManage ? "Use the add action above to add an assistant." : "Wait for an admin to add an assistant."}</span>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

function BotCardClean({
  bot,
  canManage,
  busy,
  onRemove,
  onMention,
  onSaveModel
}: {
  bot: BotInfo;
  canManage: boolean;
  busy: boolean;
  onRemove: () => void;
  onMention: (mentionTarget: string) => void;
  onSaveModel: (modelName: string) => Promise<void>;
}) {
  const [modelName, setModelName] = useState(bot.modelName || bot.supportedModels?.[0] || "");

  useEffect(() => {
    setModelName(bot.modelName || bot.supportedModels?.[0] || "");
  }, [bot.modelName, bot.supportedModels]);

  return (
    <div className="friend-card bot-card">
      <div className="friend-card-head">
        <Avatar
          name={bot.displayName || bot.name}
          src={bot.avatar}
          onContextMenu={(event) => handleAvatarMention(event, bot.mentionName, onMention)}
        />
        <div className="friend-card-meta">
          <strong>{bot.displayName || bot.name}</strong>
          <span>@{bot.mentionName} {" · "} {bot.memberType === "BOT" ? "Bot" : bot.memberType}</span>
        </div>
        <StatusPill label={bot.enabled ? "启用" : "禁用"} />
      </div>

      {bot.description && <p className="request-remark">{bot.description}</p>}

      <div className="bot-detail-fields">
        <div className="bot-field-row">
          <span className="bot-field-label">Mention</span>
          <span className="bot-field-value">@{bot.mentionName}</span>
        </div>
        {bot.aliases.length > 0 && (
          <div className="bot-field-row">
            <span className="bot-field-label">别名</span>
            <span className="bot-field-value">{bot.aliases.join(", ")}</span>
          </div>
        )}
        <div className="bot-field-row">
          <span className="bot-field-label">权限范围</span>
          <span className="bot-field-value">{bot.permissionScope}</span>
        </div>
        <div className="bot-field-row">
          <span className="bot-field-label">模型</span>
          {canManage ? (
            <div className="bot-model-editor">
              <select className="bot-model-select" value={modelName} onChange={(event) => setModelName(event.target.value)}>
                {(bot.supportedModels ?? []).map((supportedModel) => (
                  <option key={supportedModel} value={supportedModel}>
                    {supportedModel}
                  </option>
                ))}
              </select>
              <button
                className="secondary-button compact-button"
                disabled={busy || !modelName || modelName === bot.modelName}
                type="button"
                onClick={() => void onSaveModel(modelName)}
              >
                保存模型
              </button>
            </div>
          ) : (
            <span className="bot-field-value">{bot.modelName}</span>
          )}
        </div>
        <div className="bot-field-row">
          <span className="bot-field-label">Status</span>
          <StatusPill label={bot.memberStatus} />
        </div>
      </div>

      {canManage && (
        <div className="friend-row-actions">
          <button className="danger-button" disabled={busy} type="button" onClick={onRemove}>
            <Trash2 size={16} />
            移除
          </button>
        </div>
      )}
    </div>
  );
}

function AccountView({
  user,
  sessions,
  busy,
  wsStatus,
  notificationStatus,
  notificationsEnabled,
  onRefreshSessions,
  onLogout,
  onLogoutAll,
  onAvatarUpload,
  onRevokeSession,
  onToggleNotifications
}: {
  user: UserInfo;
  sessions: SessionInfo[];
  busy: boolean;
  wsStatus: WsStatus;
  notificationStatus: BrowserNotificationStatus;
  notificationsEnabled: boolean;
  onRefreshSessions: () => Promise<void>;
  onLogout: () => Promise<void>;
  onLogoutAll: (password: string) => Promise<void>;
  onAvatarUpload: (avatar: Blob) => Promise<void>;
  onRevokeSession: (sessionId: string, password: string) => Promise<void>;
  onToggleNotifications: () => Promise<void>;
}) {
  const [password, setPassword] = useState("");
  const notificationLabel =
    notificationStatus === "unsupported"
      ? "Browser notifications are not supported"
      : notificationStatus === "denied"
        ? "Browser notifications are blocked"
        : notificationStatus === "granted"
          ? notificationsEnabled
            ? "Browser notifications are enabled"
            : "Browser notifications are disabled"
          : "Browser notifications are not granted";

  return (
    <div className="detail-body account-body">
      <div className="account-card">
        <Avatar name={user.nickname || user.aim_id} src={user.avatar} size="large" />
        <strong>{user.nickname || user.aim_id}</strong>
        <span>AIM ID: {user.aim_id}</span>
        <span>{user.email}</span>
        <AvatarUploader busy={busy} onUpload={onAvatarUpload} />
        <div className="account-badges">
          <StatusPill label={user.status} />
          <StatusPill label={user.role} />
          <WsBadge status={wsStatus} />
        </div>
      </div>

      <label className="password-box">
        <KeyRound size={17} />
        <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" placeholder="会话管理密码" />
      </label>

      <div className="account-actions">
        <button disabled={busy} type="button" onClick={() => void onLogout()}>
          <LogOut size={17} />
          退出
        </button>
        <button disabled={busy} type="button" onClick={() => void onLogoutAll(password)}>
          <ShieldCheck size={17} />
          全部下线
        </button>
      </div>

      <div className="notification-card">
        <div>
          <Bell size={18} />
          <span>{notificationLabel}</span>
        </div>
        <button className={cx(notificationStatus === "granted" && notificationsEnabled && "is-on")} disabled={notificationStatus === "unsupported"} type="button" onClick={() => void onToggleNotifications()}>
          {notificationStatus === "granted" ? (notificationsEnabled ? "Disable" : "Enable") : "Enable notifications"}
        </button>
      </div>

      <div className="section-title">
        <span>登录会话</span>
        <IconButton label="刷新会话" onClick={() => void onRefreshSessions()}>
          <RefreshCw size={16} />
        </IconButton>
      </div>

      <div className="session-list">
        {sessions.map((session) => (
          <div className="session-row" key={session.session_id}>
            <Smartphone size={20} />
            <div>
              <strong>
                {session.device_name || "AIM Web"}
                {session.current && <span className="inline-tag">当前</span>}
              </strong>
              <span>{session.last_ip || session.login_ip || "unknown ip"}</span>
              <time>{formatRelative(session.last_seen_at || session.created_at)}</time>
            </div>
            <button disabled={busy} type="button" onClick={() => void onRevokeSession(session.session_id, password)}>
              撤销
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}

export default App;
