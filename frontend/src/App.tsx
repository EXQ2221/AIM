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
  mergeMessagesById,
  parseSystemMessageContent,
  parseGroupValue,
  roleLabel,
  sortConversations,
  sortFriendRequests,
  sortFriends,
  statusLabel
} from "./app/utils";
import { Avatar, Field, IconButton, MessageBubble, MobileNav, StatusPill, Toast, WsBadge } from "./app/ui";
import type {
  AICallLogInfo,
  AICallLogQuotaInfo,
  BotInfo,
  ConversationKnowledgeBaseInfo,
  ConversationInfo,
  FriendGroupInfo,
  FriendInfo,
  FriendRequestInfo,
  GroupInfo,
  KnowledgeBaseInfo,
  KnowledgeDocumentInfo,
  KnowledgeSearchChunkInfo,
  MemberInfo,
  MessageInfo,
  MobilePane,
  ReplyPreviewInfo,
  SessionInfo,
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

  const { markConversationRead, refreshCurrentConversationMessages, recoverRealtimeState } = useRealtimeState({
    selectedConversationIdRef,
    messageListRef,
    realtimeRecoveringRef,
    lastMarkedReadRef,
    refreshConversations,
    showToast,
    setMessages,
    setUnreadCounts
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

  const handleSocketEvent = useCallback(
    buildRealtimeEventHandler({
      user,
      selectedConversationIdRef,
      pendingMessagesRef,
      messageListRef,
      markConversationRead,
      refreshSelectedGroupInfo,
      showMessageNotification,
      showToast,
      syncFriendStateFromRealtime,
      applyRecalledMessageEvent,
      setMessages,
      setUnreadCounts,
      setConversations
    }),
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

  const { wsStatus, socketRef } = useRealtimeConnection({
    user,
    wsReconnectDelays,
    onMessage: handleSocketEvent,
    onRecover: recoverRealtimeState
  });

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
    setLoadingMessages
  });

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
    setLoadingOlder
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
    setBusyAction(true);
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
      setBusyAction(false);
    }
  };

  const handleAddKnowledgeDocument = async (input: {
    knowledgeBaseId: number;
    title: string;
    sourceType: "TEXT" | "MARKDOWN";
    content: string;
  }) => {
    setBusyAction(true);
    try {
      await api.addKnowledgeDocumentText(input.knowledgeBaseId, {
        title: input.title,
        sourceType: input.sourceType,
        content: input.content
      });
      if (selectedKnowledgeBaseId === input.knowledgeBaseId) {
        await refreshKnowledgeDocuments();
      }
      showToast("文档导入成功", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setBusyAction(false);
    }
  };

  const handleSearchKnowledgeBase = async (input: { knowledgeBaseId: number; query: string; topK: number }) => {
    setBusyAction(true);
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
      setBusyAction(false);
    }
  };

  const handleBindConversationKnowledgeBase = async (knowledgeBaseId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.bindConversationKnowledgeBase(selectedConversationId, knowledgeBaseId);
      await refreshConversationKnowledgeBases();
      showToast("已绑定知识库", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setBusyAction(false);
    }
  };

  const handleUnbindConversationKnowledgeBase = async (knowledgeBaseId: number) => {
    if (!selectedConversationId) return;
    setBusyAction(true);
    try {
      await api.unbindConversationKnowledgeBase(selectedConversationId, knowledgeBaseId);
      await refreshConversationKnowledgeBases();
      showToast("已解绑知识库", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
      throw error;
    } finally {
      setBusyAction(false);
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












