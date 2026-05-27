import {
  BadgePlus,
  Bell,
  Bot,
  CheckCircle2,
  ChevronDown,
  KeyRound,
  Loader2,
  LockKeyhole,
  LogOut,
  Mail,
  RefreshCw,
  ShieldCheck,
  Smartphone,
  Trash2,
  UserRound,
  UserPlus,
  UsersRound,
  X
} from "lucide-react";
import { ChangeEvent, FormEvent, useEffect, useMemo, useState } from "react";
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
  SessionInfo,
  UserMemoryInfo,
  UserInfo
} from "../../types";
import { AvatarUploader } from "../avatar-uploader";
import { Avatar, IconButton, StatusPill, WsBadge } from "../ui";
import type { BrowserNotificationStatus, DetailTab, WsStatus } from "../types";
import {
  cx,
  formatRelative,
  handleAvatarMention,
  knowledgeBaseStatusLabel,
  knowledgeDocumentStatusLabel,
  knowledgeSourceTypeLabel,
  parseGroupValue,
  roleLabel,
  statusLabel
} from "../utils";

function isMemberMuted(member: Pick<MemberInfo, "muteUntil"> | null | undefined) {
  return Boolean(member?.muteUntil && member.muteUntil > Math.floor(Date.now() / 1000));
}

function formatMuteUntil(value?: number | null) {
  if (!value || value <= 0) return "";
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", { hour12: false });
}

function isTextImportFile(file: File | null) {
  if (!file) return false;
  const lowerName = (file.name || "").toLowerCase();
  return lowerName.endsWith(".txt") || lowerName.endsWith(".md") || lowerName.endsWith(".markdown");
}

export function DetailPanel({
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
  invisibleMode,
  selectedConversationId,
  selectedConversation,
  selectedConversationType,
  selectedGroupInfo,
  currentMember,
  availableBots,
  customBots,
  conversationBots,
  aiCallLogs,
  aiCallLogQuota,
  loadingAICallLogs,
  aiCallLogStatus,
  knowledgeBases,
  selectedKnowledgeBaseId,
  knowledgeDocuments,
  knowledgeSearchChunks,
  conversationKnowledgeBases,
  loadingKnowledge,
  knowledgeBusy,
  userMemories,
  loadingUserMemories,
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
  onToggleInvisibleMode,
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
  onCreateCustomBot,
  onUpdateCustomBot,
  onDeleteCustomBot,
  onAICallLogStatusChange,
  onRefreshAICallLogs,
  onSelectKnowledgeBase,
  onCreateKnowledgeBase,
  onAddKnowledgeDocument,
  onSearchKnowledgeBase,
  onDeleteKnowledgeDocument,
  onBindConversationKnowledgeBase,
  onUnbindConversationKnowledgeBase,
  onRefreshKnowledgePanelData,
  onRefreshUserMemories,
  onWriteUserMemory,
  onUpdateUserMemory,
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
  invisibleMode: boolean;
  selectedConversationId: string | null;
  selectedConversation: ConversationInfo | null;
  selectedConversationType: ConversationInfo["type"] | null;
  selectedGroupInfo: GroupInfo | null;
  currentMember: MemberInfo | null;
  availableBots: BotInfo[];
  customBots: BotInfo[];
  conversationBots: BotInfo[];
  aiCallLogs: AICallLogInfo[];
  aiCallLogQuota: AICallLogQuotaInfo;
  loadingAICallLogs: boolean;
  aiCallLogStatus: "" | "SUCCESS" | "FAILED";
  knowledgeBases: KnowledgeBaseInfo[];
  selectedKnowledgeBaseId: number | null;
  knowledgeDocuments: KnowledgeDocumentInfo[];
  knowledgeSearchChunks: KnowledgeSearchChunkInfo[];
  conversationKnowledgeBases: ConversationKnowledgeBaseInfo[];
  loadingKnowledge: boolean;
  knowledgeBusy: boolean;
  userMemories: UserMemoryInfo[];
  loadingUserMemories: boolean;
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
  onToggleInvisibleMode: (invisible: boolean) => Promise<void>;
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
  onCreateCustomBot: (input: {
    name: string;
    mentionName: string;
    aliases?: string[];
    description?: string;
    apiBaseUrl: string;
    apiKey: string;
    modelName: string;
    supportedModels?: string[];
    systemPrompt?: string;
  }) => Promise<void>;
  onUpdateCustomBot: (input: {
    botId: number;
    name: string;
    mentionName: string;
    aliases?: string[];
    description?: string;
    apiBaseUrl?: string;
    apiKey?: string;
    modelName: string;
    supportedModels?: string[];
    systemPrompt?: string;
  }) => Promise<void>;
  onDeleteCustomBot: (botId: number) => Promise<void>;
  onAICallLogStatusChange: (status: "" | "SUCCESS" | "FAILED") => void;
  onRefreshAICallLogs: () => Promise<void>;
  onSelectKnowledgeBase: (knowledgeBaseId: number | null) => void;
  onCreateKnowledgeBase: (input: { name: string; description: string }) => Promise<void>;
  onAddKnowledgeDocument: (input: {
    knowledgeBaseId: number;
    title: string;
    sourceType?: "TEXT" | "MARKDOWN";
    content?: string;
    file?: File | null;
  }) => Promise<void>;
  onSearchKnowledgeBase: (input: { knowledgeBaseId: number; query: string; topK: number }) => Promise<void>;
  onDeleteKnowledgeDocument: (knowledgeBaseId: number, documentId: number) => Promise<void>;
  onBindConversationKnowledgeBase: (knowledgeBaseId: number) => Promise<void>;
  onUnbindConversationKnowledgeBase: (knowledgeBaseId: number) => Promise<void>;
  onRefreshKnowledgePanelData: () => Promise<void>;
  onRefreshUserMemories: () => Promise<void>;
  onWriteUserMemory: (content: string) => Promise<void>;
  onUpdateUserMemory: (memoryId: number, content: string) => Promise<void>;
  onMention: (mentionTarget: string) => void;
  onClose: () => void;
}) {
  return (
    <aside className={cx("pane detail-pane", active && "mobile-active")}>
      <header className="detail-header">
        <div className="segmented small-tabs tabs-six">
          <button className={tab === "friends" ? "active" : ""} type="button" onClick={() => onTabChange("friends")}>
            好友
          </button>
          <button className={tab === "members" ? "active" : ""} type="button" onClick={() => onTabChange("members")}>
            成员
          </button>
          <button className={tab === "bots" ? "active" : ""} type="button" onClick={() => onTabChange("bots")}>
            AI 助手
          </button>
          <button className={tab === "knowledge" ? "active" : ""} type="button" onClick={() => onTabChange("knowledge")}>
            知识库
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
          customBots={customBots}
          conversationBots={conversationBots}
          busy={busy}
          onAddBot={onAddBot}
          onRemoveBot={onRemoveBot}
          onCreateCustomBot={onCreateCustomBot}
          onUpdateCustomBot={onUpdateCustomBot}
          onDeleteCustomBot={onDeleteCustomBot}
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
      ) : tab === "knowledge" ? (
        <KnowledgeBasePanel
          selectedConversationId={selectedConversationId}
          selectedConversationType={selectedConversationType}
          currentMember={currentMember}
          knowledgeBases={knowledgeBases}
          selectedKnowledgeBaseId={selectedKnowledgeBaseId}
          knowledgeDocuments={knowledgeDocuments}
          knowledgeSearchChunks={knowledgeSearchChunks}
          conversationKnowledgeBases={conversationKnowledgeBases}
          loading={loadingKnowledge}
          busy={knowledgeBusy}
          onSelectKnowledgeBase={onSelectKnowledgeBase}
          onCreateKnowledgeBase={onCreateKnowledgeBase}
          onAddKnowledgeDocument={onAddKnowledgeDocument}
          onSearchKnowledgeBase={onSearchKnowledgeBase}
          onDeleteKnowledgeDocument={onDeleteKnowledgeDocument}
          onBindConversationKnowledgeBase={onBindConversationKnowledgeBase}
          onUnbindConversationKnowledgeBase={onUnbindConversationKnowledgeBase}
          onRefresh={onRefreshKnowledgePanelData}
        />
      ) : (
        <AccountView
          user={user}
          sessions={sessions}
          busy={busy}
          wsStatus={wsStatus}
          notificationStatus={notificationStatus}
          notificationsEnabled={notificationsEnabled}
          invisibleMode={invisibleMode}
          userMemories={userMemories}
          loadingUserMemories={loadingUserMemories}
          onRefreshSessions={onRefreshSessions}
          onLogout={onLogout}
          onLogoutAll={onLogoutAll}
          onAvatarUpload={onAvatarUpload}
          onRevokeSession={onRevokeSession}
          onToggleNotifications={onToggleNotifications}
          onToggleInvisibleMode={onToggleInvisibleMode}
          onRefreshUserMemories={onRefreshUserMemories}
          onWriteUserMemory={onWriteUserMemory}
          onUpdateUserMemory={onUpdateUserMemory}
        />
      )}
    </aside>
  );
}

function KnowledgeBasePanel({
  selectedConversationId,
  selectedConversationType,
  currentMember,
  knowledgeBases,
  selectedKnowledgeBaseId,
  knowledgeDocuments,
  knowledgeSearchChunks,
  conversationKnowledgeBases,
  loading,
  busy,
  onSelectKnowledgeBase,
  onCreateKnowledgeBase,
  onAddKnowledgeDocument,
  onSearchKnowledgeBase,
  onDeleteKnowledgeDocument,
  onBindConversationKnowledgeBase,
  onUnbindConversationKnowledgeBase,
  onRefresh
}: {
  selectedConversationId: string | null;
  selectedConversationType: ConversationInfo["type"] | null;
  currentMember: MemberInfo | null;
  knowledgeBases: KnowledgeBaseInfo[];
  selectedKnowledgeBaseId: number | null;
  knowledgeDocuments: KnowledgeDocumentInfo[];
  knowledgeSearchChunks: KnowledgeSearchChunkInfo[];
  conversationKnowledgeBases: ConversationKnowledgeBaseInfo[];
  loading: boolean;
  busy: boolean;
  onSelectKnowledgeBase: (knowledgeBaseId: number | null) => void;
  onCreateKnowledgeBase: (input: { name: string; description: string }) => Promise<void>;
  onAddKnowledgeDocument: (input: {
    knowledgeBaseId: number;
    title: string;
    sourceType?: "TEXT" | "MARKDOWN";
    content?: string;
    file?: File | null;
  }) => Promise<void>;
  onSearchKnowledgeBase: (input: { knowledgeBaseId: number; query: string; topK: number }) => Promise<void>;
  onDeleteKnowledgeDocument: (knowledgeBaseId: number, documentId: number) => Promise<void>;
  onBindConversationKnowledgeBase: (knowledgeBaseId: number) => Promise<void>;
  onUnbindConversationKnowledgeBase: (knowledgeBaseId: number) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [title, setTitle] = useState("");
  const [content, setContent] = useState("");
  const [selectedDocumentFile, setSelectedDocumentFile] = useState<File | null>(null);
  const [query, setQuery] = useState("");
  const [topK, setTopK] = useState(5);
  const [bindKnowledgeBaseId, setBindKnowledgeBaseId] = useState<number | "">("");

  const canManageBinding =
    selectedConversationType === "GROUP" && (currentMember?.role === "OWNER" || currentMember?.role === "ADMIN");
  const visibleKnowledgeDocuments = knowledgeDocuments.filter((item) => (item.status || "").toUpperCase() !== "FAILED");
  const failedKnowledgeDocuments = knowledgeDocuments.filter((item) => (item.status || "").toUpperCase() === "FAILED");
  const enabledBindings = conversationKnowledgeBases.filter((item) => item.enabled);
  const enabledBindingIds = new Set(enabledBindings.map((item) => item.knowledgeBaseId));
  const bindCandidates = knowledgeBases.filter((item) => !enabledBindingIds.has(item.knowledgeBaseId));

  useEffect(() => {
    if (bindCandidates.length === 0) {
      setBindKnowledgeBaseId("");
      return;
    }
    if (bindKnowledgeBaseId === "" || !bindCandidates.some((item) => item.knowledgeBaseId === bindKnowledgeBaseId)) {
      setBindKnowledgeBaseId(bindCandidates[0].knowledgeBaseId);
    }
  }, [bindCandidates, bindKnowledgeBaseId]);

  const submitCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextName = name.trim();
    if (!nextName) return;
    await onCreateKnowledgeBase({
      name: nextName,
      description: description.trim()
    });
    setName("");
    setDescription("");
  };

  const submitDocument = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selectedKnowledgeBaseId) return;
    const nextTitle = title.trim();
    if (!nextTitle) return;
    if (selectedDocumentFile) {
      await onAddKnowledgeDocument({
        knowledgeBaseId: selectedKnowledgeBaseId,
        title: nextTitle,
        file: selectedDocumentFile
      });
    } else {
      const nextContent = content.trim();
      if (!nextContent) return;
      await onAddKnowledgeDocument({
        knowledgeBaseId: selectedKnowledgeBaseId,
        title: nextTitle,
        sourceType: "TEXT",
        content: nextContent
      });
    }
    setTitle("");
    setContent("");
    setSelectedDocumentFile(null);
  };

  const handleDocumentFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0] ?? null;
    if (!file) {
      setSelectedDocumentFile(null);
      setContent("");
      return;
    }
    const fileName = file.name || "";
    const lowerName = fileName.toLowerCase();
    setSelectedDocumentFile(file);
    if (!title.trim()) {
      const dotIndex = fileName.lastIndexOf(".");
      setTitle(dotIndex > 0 ? fileName.slice(0, dotIndex) : fileName);
    }

    if (!isTextImportFile(file)) {
      setContent("");
      return;
    }

    const reader = new FileReader();
    reader.onload = () => {
      const raw = typeof reader.result === "string" ? reader.result : "";
      setContent(raw);
    };
    reader.onerror = () => {
      setContent("");
      setSelectedDocumentFile(null);
      alert("读取文件失败，请重试");
    };
    reader.readAsText(file, "utf-8");
  };

  const submitSearch = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selectedKnowledgeBaseId) return;
    const nextQuery = query.trim();
    if (!nextQuery) return;
    await onSearchKnowledgeBase({
      knowledgeBaseId: selectedKnowledgeBaseId,
      query: nextQuery,
      topK
    });
  };

  const submitBind = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selectedConversationId || typeof bindKnowledgeBaseId !== "number") return;
    await onBindConversationKnowledgeBase(bindKnowledgeBaseId);
  };

  return (
    <div className="detail-body knowledge-body">
      <div className="section-title">
        <span>知识库管理</span>
        <IconButton label="刷新知识库数据" onClick={() => void onRefresh()}>
          <RefreshCw size={16} />
        </IconButton>
      </div>

      <form className="drawer-form" onSubmit={submitCreate}>
        <label className="field">
          <span>新建知识库</span>
          <input value={name} onChange={(event) => setName(event.target.value)} placeholder="知识库名称" required />
        </label>
        <textarea
          value={description}
          onChange={(event) => setDescription(event.target.value)}
          placeholder="知识库描述（可选）"
          rows={2}
        />
        <button disabled={busy} type="submit">
          {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
          创建知识库
        </button>
      </form>

      <div className="kb-card">
        <label className="field">
          <span>当前知识库</span>
          <select
            value={selectedKnowledgeBaseId ?? ""}
            onChange={(event) => onSelectKnowledgeBase(event.target.value ? Number(event.target.value) : null)}
          >
            <option value="">请选择知识库</option>
            {knowledgeBases.map((item) => (
              <option key={item.knowledgeBaseId} value={item.knowledgeBaseId}>
                {item.name} (#{item.knowledgeBaseId})
              </option>
            ))}
          </select>
        </label>
        {loading && (
          <span className="kb-hint">
            <Loader2 className="spin" size={14} />
            加载中
          </span>
        )}
      </div>

      <form className="drawer-form" onSubmit={submitDocument}>
        <label className="field">
          <span>导入文档</span>
          <input
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            placeholder="文档标题"
            disabled={!selectedKnowledgeBaseId}
            required
          />
        </label>
        <span className="form-hint">系统会自动识别并处理文档类型，无需手动选择。</span>
        <label className="field">
          <span>选择文件</span>
          <input
            type="file"
            accept=".txt,.md,.markdown,.pdf,.docx,.pptx,.ppt,text/plain,text/markdown,application/pdf,application/vnd.openxmlformats-officedocument.wordprocessingml.document,application/vnd.openxmlformats-officedocument.presentationml.presentation,application/vnd.ms-powerpoint"
            onChange={handleDocumentFileChange}
            disabled={!selectedKnowledgeBaseId}
          />
        </label>
        {selectedDocumentFile && <span className="form-hint">已选择：{selectedDocumentFile.name}</span>}
        {selectedDocumentFile && !isTextImportFile(selectedDocumentFile) && (
          <span className="form-hint">PDF / DOCX / PPTX 文件会在服务端解析后再进入 RAG 链路。</span>
        )}
        <textarea
          value={content}
          onChange={(event) => setContent(event.target.value)}
          placeholder={
            selectedDocumentFile && !isTextImportFile(selectedDocumentFile)
              ? "PDF / DOCX / PPTX 会由后端直接解析，无需在这里粘贴内容"
              : "文件内容会自动填充，也可手动补充编辑"
          }
          rows={5}
          disabled={!selectedKnowledgeBaseId}
        />
        <span className="form-hint">
          支持直接粘贴，或上传 txt / md / pdf / docx / pptx 文件。文件导入时会自动识别类型。
        </span>
        <button
          disabled={
            busy ||
            !selectedKnowledgeBaseId ||
            !title.trim() ||
            (!selectedDocumentFile && !content.trim())
          }
          type="submit"
        >
          {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
          导入文档
        </button>
      </form>

      <div className="kb-card">
        <div className="section-title">
          <span>文档状态</span>
          <strong>{visibleKnowledgeDocuments.length}</strong>
        </div>
        {failedKnowledgeDocuments.length > 0 && (
          <span className="form-hint">有 {failedKnowledgeDocuments.length} 条导入失败，失败原因已推送到通知中心。</span>
        )}
        <div className="kb-list">
          {visibleKnowledgeDocuments.map((item) => (
            <div className="kb-row" key={item.documentId}>
              <strong>{item.title}</strong>
              <span>
                {knowledgeSourceTypeLabel(item.sourceType)} · {knowledgeDocumentStatusLabel(item.status)}
              </span>
              <button
                className="danger-button compact-button"
                disabled={busy || !selectedKnowledgeBaseId}
                type="button"
                onClick={() => {
                  if (!selectedKnowledgeBaseId) return;
                  if (!window.confirm(`确认删除文档「${item.title}」吗？`)) return;
                  void onDeleteKnowledgeDocument(selectedKnowledgeBaseId, item.documentId);
                }}
              >
                <Trash2 size={14} />
                删除
              </button>
            </div>
          ))}
          {visibleKnowledgeDocuments.length === 0 && <span className="kb-empty">暂无文档</span>}
        </div>
      </div>

      <form className="drawer-form" onSubmit={submitSearch}>
        <label className="field">
          <span>检索测试</span>
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="输入检索问题"
            disabled={!selectedKnowledgeBaseId}
          />
        </label>
        <label className="field">
          <span>TopK</span>
          <input
            type="number"
            min={1}
            max={10}
            value={topK}
            onChange={(event) => setTopK(Math.max(1, Math.min(10, Number(event.target.value) || 1)))}
            disabled={!selectedKnowledgeBaseId}
          />
        </label>
        <button disabled={busy || !selectedKnowledgeBaseId} type="submit">
          {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
          执行检索
        </button>
      </form>

      <div className="kb-card">
        <div className="section-title">
          <span>检索结果</span>
          <strong>{knowledgeSearchChunks.length}</strong>
        </div>
        <div className="kb-list">
          {knowledgeSearchChunks.map((item) => (
            <div className="kb-row" key={item.chunkId}>
              <strong>Chunk #{item.chunkId}</strong>
              <span>
                文档 #{item.documentId} · score {item.score.toFixed(4)}
              </span>
              <p>{item.content}</p>
            </div>
          ))}
          {knowledgeSearchChunks.length === 0 && <span className="kb-empty">暂无检索结果</span>}
        </div>
      </div>

      <div className="kb-card">
        <div className="section-title">
          <span>会话绑定知识库</span>
          <strong>{enabledBindings.length}</strong>
        </div>
        {!selectedConversationId || selectedConversationType !== "GROUP" ? (
          <span className="kb-empty">仅群聊支持知识库绑定</span>
        ) : (
          <>
            <form className="kb-bind-form" onSubmit={submitBind}>
              <select
                value={bindKnowledgeBaseId}
                onChange={(event) => setBindKnowledgeBaseId(event.target.value ? Number(event.target.value) : "")}
                disabled={!canManageBinding || bindCandidates.length === 0}
              >
                <option value="">请选择待绑定知识库</option>
                {bindCandidates.map((item) => (
                  <option key={item.knowledgeBaseId} value={item.knowledgeBaseId}>
                    {item.name} (#{item.knowledgeBaseId})
                  </option>
                ))}
              </select>
              <button disabled={!canManageBinding || busy || typeof bindKnowledgeBaseId !== "number"} type="submit">
                绑定
              </button>
            </form>
            {!canManageBinding && <span className="kb-empty">当前角色为只读（仅 OWNER/ADMIN 可绑定或解绑）</span>}
          </>
        )}
        <div className="kb-list">
          {enabledBindings.map((item) => (
            <div className="kb-row" key={item.id}>
              <strong>
                {item.name} (#{item.knowledgeBaseId})
              </strong>
              <span>{knowledgeBaseStatusLabel(item.status)}</span>
              {canManageBinding && selectedConversationType === "GROUP" && (
                <button
                  className="danger-button compact-button"
                  disabled={busy}
                  type="button"
                  onClick={() => void onUnbindConversationKnowledgeBase(item.knowledgeBaseId)}
                >
                  解绑
                </button>
              )}
            </div>
          ))}
          {enabledBindings.length === 0 && <span className="kb-empty">当前会话暂无绑定知识库</span>}
        </div>
      </div>
    </div>
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
            <option value="">未分组</option>
            {friendGroups.map((group) => (
              <option key={group.id} value={group.id}>
                {group.name}
              </option>
            ))}
          </select>
          <span className="form-hint">对方同意后会自动创建会话。</span>
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
            <span>可切换分组筛选，或先把好友移动到该分组。</span>
          </div>
        )}
        {friends.length === 0 && (
          <div className="empty-block">
            <UserRound size={28} />
            <strong>暂无好友</strong>
            <span>可以先通过 AIM ID 添加好友。</span>
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
          <span>{request.aim_id} {"  "} {incoming ? "收到的申请" : "发出的申请"}</span>
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
  const presenceLabel = (friend.presence || "").toUpperCase() === "ONLINE" ? "在线" : "离线";
  const presenceBadgeClass = (friend.presence || "").toUpperCase() === "ONLINE" ? "presence-online" : "presence-offline";
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
          <span>{friend.aim_id} {"  "} {assignedGroup?.name || "未分组"}</span>
        </div>
        <div className="friend-card-side">
          <span className={cx("friend-presence-badge", presenceBadgeClass)}>{presenceLabel}</span>
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
              <option value="">未分组</option>
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
  const todayLogs = useMemo(() => logs.filter((item) => isSameLocalDay(item.createdAt)), [logs]);
  const quotaItems = useMemo(() => {
    const usageMap = new Map<string, { provider: string; model: string; used: number; limit: number }>();
    for (const item of todayLogs) {
      const model = (item.modelName || "unknown").trim() || "unknown";
      const provider = resolveProviderNameFromModel(model);
      const key = `${provider}::${model}`;
      const current = usageMap.get(key) ?? {
        provider,
        model,
        used: 0,
        limit: dailyLimitByModelName(model)
      };
      current.used += Math.max(0, Number(item.totalTokens) || 0);
      usageMap.set(key, current);
    }
    return Array.from(usageMap.values())
      .map((entry) => {
        const remaining = Math.max(0, entry.limit - entry.used);
        const percent = entry.limit > 0 ? Math.min(100, Math.round((entry.used / entry.limit) * 100)) : 0;
        return { ...entry, remaining, percent };
      })
      .sort((a, b) => b.used - a.used);
  }, [todayLogs]);
  const quotaDailyLimit = useMemo(() => quotaItems.reduce((sum, item) => sum + item.limit, 0), [quotaItems]);
  const quotaDailyUsed = useMemo(() => quotaItems.reduce((sum, item) => sum + item.used, 0), [quotaItems]);
  const quotaRemaining = Math.max(0, quotaDailyLimit - quotaDailyUsed);
  const usagePercent = quotaDailyLimit > 0 ? Math.min(100, Math.round((quotaDailyUsed / quotaDailyLimit) * 100)) : 0;

  if (!selectedConversationId) {
    return (
      <div className="detail-body log-body">
        <div className="empty-block">
          <Bot size={28} />
          <strong>请选择一个群聊会话</strong>
          <span>进入群聊后可查看 AI 调用日志。</span>
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
          <span>{quotaDailyUsed.toLocaleString()} / {quotaDailyLimit.toLocaleString()} tokens</span>
        </div>
        <div aria-hidden="true" className="quota-bar">
          <span className="quota-bar-fill" style={{ width: `${usagePercent}%` }} />
        </div>
        <div className="quota-card-meta">
          <span>剩余 {quotaRemaining.toLocaleString()} tokens</span>
          <span>{usagePercent}%</span>
        </div>
        <details className="quota-breakdown" open>
          <summary>按厂家 / 模型查看额度明细</summary>
          {quotaItems.length === 0 ? (
            <p className="quota-empty">暂无可统计的模型调用记录</p>
          ) : (
            <div className="quota-breakdown-list">
              {quotaItems.map((item) => (
                <article className="quota-breakdown-item" key={`${item.provider}-${item.model}`}>
                  <div className="quota-breakdown-head">
                    <strong>{item.model}</strong>
                    <span>{item.provider}</span>
                  </div>
                  <div className="quota-breakdown-meta">
                    <span>
                      {item.used.toLocaleString()} / {item.limit.toLocaleString()} tokens
                    </span>
                    <span>剩余 {item.remaining.toLocaleString()}</span>
                  </div>
                  <div aria-hidden="true" className="quota-bar quota-bar-small">
                    <span className="quota-bar-fill" style={{ width: `${item.percent}%` }} />
                  </div>
                </article>
              ))}
            </div>
          )}
        </details>
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
            <span>该群触发 AI 助手后，这里会出现调用日志。</span>
          </div>
        ) : (
          logs.map((log) => (
            <article className="log-card" key={log.id}>
              <div className="log-card-head">
                <div className="log-card-meta">
                  <strong>{log.botName || `机器人 ${log.botId}`}</strong>
                  <span>{log.modelName || "未知模型"}</span>
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
      ? `更新于 ${formatRelative(groupInfo.announcementUpdatedAt)}${
          typeof groupInfo.announcementUpdatedBy === "number"
            ? `，操作人：${updater?.nickname || `用户 ${groupInfo.announcementUpdatedBy}`}`
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
          <strong>群公告</strong>
          <span>会展示在群详情中，所有成员可见。</span>
        </div>
        {canEdit && !editing && (
          <button className="secondary-button compact-button" disabled={busy} type="button" onClick={() => setEditing(true)}>
            编辑
          </button>
        )}
      </div>

      {groupInfo === null ? (
        <p className="group-announcement-empty">正在加载群公告...</p>
      ) : editing ? (
        <form className="group-announcement-form" onSubmit={handleSubmit}>
          <textarea
            maxLength={2000}
            placeholder="请输入群公告内容"
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
              保存
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
              取消
            </button>
          </div>
        </form>
      ) : announcement ? (
        <p className="group-announcement-content">{announcement}</p>
      ) : (
        <p className="group-announcement-empty">{canEdit ? "暂无公告，点击编辑可新增。" : "暂无公告。"}</p>
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
    { label: "10分钟", value: 10 * 60 },
    { label: "1小时", value: 60 * 60 },
    { label: "24小时", value: 24 * 60 * 60 }
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
        <span>成员</span>
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
                name={member.nickname || "机器人"}
                src={member.avatar}
                onContextMenu={(event) =>
                  handleAvatarMention(event, member.aliases?.[0] || member.mentionName || member.nickname, onMention)
                }
              />
              <div>
                <strong>{member.nickname || `机器人 ${member.botId ?? member.userId}`}</strong>
                <span>
                  <StatusPill label="AI" /> {"  "}@{member.mentionName ?? "bot"}
                  {member.enabled === false && <StatusPill label="已禁用" />}
                </span>
                {member.aliases && member.aliases.length > 0 && (
                  <span className="bot-member-aliases">别名：{member.aliases.join(", ")}</span>
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
                <span>
                  {roleLabel(member.role)} {"  "} {statusLabel(member.status)}
                </span>
                {isMemberMuted(member) && (
                  <span className="member-extra-note">已禁言至 {formatMuteUntil(member.muteUntil)}</span>
                )}
                {member.userId === currentMember?.userId &&
                  muteAllEnabled &&
                  currentMember.role !== "OWNER" &&
                  currentMember.role !== "ADMIN" && (
                    <span className="member-extra-note">当前群聊已开启全员禁言。</span>
                  )}
                {hasManagementActions(member) && (
                  <button
                    className={cx("member-expand-toggle", expandedMemberId === member.userId && "is-open")}
                    disabled={busy}
                    type="button"
                    onClick={() => setExpandedMemberId((current) => (current === member.userId ? null : member.userId))}
                  >
                    <span>管理</span>
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
                        设为管理员
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
                        取消管理员
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
                            if (window.confirm(`确认将 ${member.nickname || `用户 ${member.userId}`} 移出群聊吗？`)) {
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
function BotPanelClean({
  selectedConversationId,
  selectedConversationType,
  currentMember,
  availableBots,
  customBots,
  conversationBots,
  busy,
  onAddBot,
  onRemoveBot,
  onCreateCustomBot,
  onUpdateCustomBot,
  onDeleteCustomBot,
  onMention
}: {
  selectedConversationId: string | null;
  selectedConversationType: ConversationInfo["type"] | null;
  currentMember: MemberInfo | null;
  availableBots: BotInfo[];
  customBots: BotInfo[];
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
  onCreateCustomBot: (input: {
    name: string;
    mentionName: string;
    aliases?: string[];
    description?: string;
    apiBaseUrl: string;
    apiKey: string;
    modelName: string;
    supportedModels?: string[];
    systemPrompt?: string;
  }) => Promise<void>;
  onUpdateCustomBot: (input: {
    botId: number;
    name: string;
    mentionName: string;
    aliases?: string[];
    description?: string;
    apiBaseUrl?: string;
    apiKey?: string;
    modelName: string;
    supportedModels?: string[];
    systemPrompt?: string;
  }) => Promise<void>;
  onDeleteCustomBot: (botId: number) => Promise<void>;
  onMention: (mentionTarget: string) => void;
}) {
  const providerByModel = (modelName?: string) => {
    const value = (modelName || "").trim().toLowerCase();
    if (!value) return "其他";
    if (value.startsWith("deepseek")) return "DeepSeek";
    if (value.startsWith("qwen") || value.includes("tongyi")) return "通义";
    return "其他";
  };

  const [addOpen, setAddOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedBotId, setSelectedBotId] = useState<number | "">("");
  const [selectedModelName, setSelectedModelName] = useState("");
  const [selectedPermissionScope, setSelectedPermissionScope] = useState("CONVERSATION_ONLY");
  const [selectedDisplayNameOverride, setSelectedDisplayNameOverride] = useState("");
  const [customName, setCustomName] = useState("");
  const [customMentionName, setCustomMentionName] = useState("");
  const [customAliases, setCustomAliases] = useState("");
  const [customDescription, setCustomDescription] = useState("");
  const [customAPIBaseURL, setCustomAPIBaseURL] = useState("");
  const [customAPIKey, setCustomAPIKey] = useState("");
  const [customModelName, setCustomModelName] = useState("");
  const [customSupportedModels, setCustomSupportedModels] = useState("");
  const [customSystemPrompt, setCustomSystemPrompt] = useState("");
  const customBaseURLLooksLikeEndpoint = /\/chat\/completions\/?$/i.test(customAPIBaseURL.trim());

  const canManage = currentMember?.role === "OWNER" || currentMember?.role === "ADMIN";
  const addedBotIds = new Set(conversationBots.map((item) => item.botId));
  const candidateBots = availableBots.filter((item) => !addedBotIds.has(item.botId));
  const candidateBotGroups = candidateBots.reduce<Record<string, BotInfo[]>>((acc, item) => {
    const key = providerByModel(item.modelName);
    if (!acc[key]) acc[key] = [];
    acc[key].push(item);
    return acc;
  }, {});
  const candidateBotGroupOrder = ["DeepSeek", "通义", "其他"];
  const selectedBot =
    typeof selectedBotId === "number" ? candidateBots.find((item) => item.botId === selectedBotId) ?? null : null;

  useEffect(() => {
    if (!selectedBot) {
      setSelectedModelName("");
      setSelectedDisplayNameOverride("");
      return;
    }
    setSelectedModelName(selectedBot.modelName || selectedBot.supportedModels?.[0] || "");
    setSelectedDisplayNameOverride("");
  }, [selectedBot]);

  const handleAdd = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (typeof selectedBotId !== "number" || selectedBotId <= 0 || !selectedModelName) return;
    await onAddBot({
      botId: selectedBotId,
      displayNameOverride: selectedDisplayNameOverride.trim() || undefined,
      permissionScope: selectedPermissionScope,
      modelNameOverride: selectedModelName
    });
    setSelectedBotId("");
    setSelectedModelName("");
    setSelectedPermissionScope("CONVERSATION_ONLY");
    setSelectedDisplayNameOverride("");
    setAddOpen(false);
  };

  const parseCSVValues = (raw: string) =>
    raw
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);

  const handleCreateCustom = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = customName.trim();
    const mentionName = customMentionName.trim();
    const apiBaseUrl = customAPIBaseURL.trim();
    const apiKey = customAPIKey.trim();
    const modelName = customModelName.trim();
    if (!name || !mentionName || !apiBaseUrl || !apiKey || !modelName) {
      return;
    }
    if (mentionName.includes("@")) {
      window.alert("提及名不需要包含 @");
      return;
    }
    const aliases = parseCSVValues(customAliases);
    const supportedModelsInput = parseCSVValues(customSupportedModels);
    const supportedModels = supportedModelsInput.length > 0 ? supportedModelsInput : [modelName];
    await onCreateCustomBot({
      name,
      mentionName,
      aliases,
      description: customDescription.trim(),
      apiBaseUrl,
      apiKey,
      modelName,
      supportedModels,
      systemPrompt: customSystemPrompt.trim()
    });
    setCustomName("");
    setCustomMentionName("");
    setCustomAliases("");
    setCustomDescription("");
    setCustomAPIBaseURL("");
    setCustomAPIKey("");
    setCustomModelName("");
    setCustomSupportedModels("");
    setCustomSystemPrompt("");
    setCreateOpen(false);
    setAddOpen(true);
  };

  return (
    <div className="detail-body bot-body">
      {!selectedConversationId ? (
        <div className="empty-block">
          <Bot size={30} />
          <strong>请选择一个群聊会话</strong>
          <span>进入会话后可管理 AI 助手</span>
        </div>
      ) : selectedConversationType !== "GROUP" ? (
        <div className="empty-block">
          <Bot size={30} />
          <strong>单聊不支持添加 AI 助手</strong>
          <span>当前仅支持在群聊中添加 AI 助手。</span>
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
                <button type="button" onClick={() => setCreateOpen((value) => !value)}>
                  <Bot size={18} />
                  创建自定义 Bot
                </button>
              </div>

              {addOpen && (
                <form className="drawer-form" onSubmit={handleAdd}>
                  <label className="field">
                    <span>选择 Bot</span>
                    <select value={selectedBotId} onChange={(event) => setSelectedBotId(event.target.value ? Number(event.target.value) : "")}>
                      <option value="">请选择</option>
                      {candidateBotGroupOrder.map((groupName) =>
                        (candidateBotGroups[groupName] || []).length > 0 ? (
                          <optgroup key={groupName} label={groupName}>
                            {(candidateBotGroups[groupName] || []).map((item) => (
                              <option key={item.botId} value={item.botId}>
                                {(item.displayName || item.name) + " · " + (item.modelName || "-")}
                              </option>
                            ))}
                          </optgroup>
                        ) : null
                      )}
                    </select>
                  </label>

                  <label className="field">
                    <span>权限范围</span>
                    <select value={selectedPermissionScope} onChange={(event) => setSelectedPermissionScope(event.target.value)}>
                      <option value="CONVERSATION_ONLY">群聊上下文</option>
                      <option value="KNOWLEDGE_BASE_ONLY">仅知识库</option>
                      <option value="CONVERSATION_AND_KB">群聊 + 知识库</option>
                    </select>
                  </label>

                  <label className="field">
                    <span>群内显示昵称</span>
                    <select
                      value={selectedDisplayNameOverride}
                      onChange={(event) => setSelectedDisplayNameOverride(event.target.value)}
                      disabled={!selectedBot}
                    >
                      <option value="">使用默认名称（{selectedBot?.displayName || selectedBot?.name || "-"})</option>
                      {selectedBot?.aliases?.map((alias) => (
                        <option key={`alias-${alias}`} value={alias}>
                          使用别名：{alias}
                        </option>
                      ))}
                      {selectedBot?.mentionName && (
                        <option value={selectedBot.mentionName}>使用提及名：{selectedBot.mentionName}</option>
                      )}
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

                  {candidateBots.length === 0 && <span className="form-hint">当前可用 Bot 已全部加入本群。</span>}
                  <span className="form-hint">建议按 Bot 用途选择权限范围，避免越权读取。</span>
                  <button disabled={busy || typeof selectedBotId !== "number" || !selectedModelName} type="submit">
                    {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
                    添加到会话
                  </button>
                </form>
              )}

              {createOpen && (
                <form className="drawer-form" onSubmit={handleCreateCustom}>
                  <label className="field">
                    <span>Bot 名称</span>
                    <input value={customName} onChange={(event) => setCustomName(event.target.value)} placeholder="例如：我的助手" required />
                  </label>
                  <label className="field">
                    <span>提及名（不带 @）</span>
                    <input value={customMentionName} onChange={(event) => setCustomMentionName(event.target.value)} placeholder="例如：mybot" required />
                  </label>
                  <label className="field">
                    <span>别名（逗号分隔）</span>
                    <input value={customAliases} onChange={(event) => setCustomAliases(event.target.value)} placeholder="例如：小助手,客服助手" />
                  </label>
                  <label className="field">
                    <span>描述（可选）</span>
                    <input value={customDescription} onChange={(event) => setCustomDescription(event.target.value)} placeholder="Bot 的用途说明" />
                  </label>
                  <label className="field">
                    <span>API Base URL</span>
                    <input
                      value={customAPIBaseURL}
                      onChange={(event) => setCustomAPIBaseURL(event.target.value)}
                      placeholder="例如：https://dashscope.aliyuncs.com/compatible-mode/v1"
                      required
                    />
                    <small className="form-hint">请填写到 `/v1`，不要包含 `/chat/completions`（系统会自动拼接）。</small>
                    {customBaseURLLooksLikeEndpoint && (
                      <small className="form-hint form-hint-warning">
                        你当前填的是完整接口路径。建议只填 Base URL（如 `.../v1`），不要带 `/chat/completions`。
                      </small>
                    )}
                  </label>
                  <label className="field">
                    <span>API Key</span>
                    <input value={customAPIKey} onChange={(event) => setCustomAPIKey(event.target.value)} placeholder="sk-***" required />
                  </label>
                  <label className="field">
                    <span>模型名</span>
                    <input value={customModelName} onChange={(event) => setCustomModelName(event.target.value)} placeholder="例如：qwen-plus-latest" required />
                  </label>
                  <label className="field">
                    <span>支持模型（逗号分隔，可选）</span>
                    <input value={customSupportedModels} onChange={(event) => setCustomSupportedModels(event.target.value)} placeholder="不填则默认使用模型名" />
                  </label>
                  <label className="field">
                    <span>系统提示词（可选）</span>
                    <textarea
                      value={customSystemPrompt}
                      onChange={(event) => setCustomSystemPrompt(event.target.value)}
                      rows={3}
                      placeholder="例如：你是一个专业且简洁的群聊助手。"
                    />
                  </label>
                  <span className="form-hint">创建后会出现在“选择 Bot”列表中，再添加到当前群聊即可生效。</span>
                  <button
                    disabled={
                      busy ||
                      !customName.trim() ||
                      !customMentionName.trim() ||
                      !customAPIBaseURL.trim() ||
                      !customAPIKey.trim() ||
                      !customModelName.trim()
                    }
                    type="submit"
                  >
                    {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
                    创建 Bot
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
                onSaveProfile={(input) =>
                  onAddBot({
                    botId: item.botId,
                    displayNameOverride: input.displayNameOverride,
                    aliasesOverride: input.aliasesOverride
                  })
                }
              />
            ))}
            {conversationBots.length === 0 && (
              <div className="empty-block compact-empty">
                <Bot size={24} />
                <strong>暂无 AI 助手</strong>
                <span>{canManage ? "可使用上方按钮添加助手。" : "请等待管理员添加助手。"}</span>
              </div>
            )}
          </div>

          <div className="section-title">
            <span>我的自建 Bot</span>
            <strong>{customBots.length}</strong>
          </div>
          <div className="bot-list">
            {customBots.map((item) => (
              <CustomBotManageCard
                key={`custom-${item.botId}`}
                bot={item}
                canManage={canManage}
                busy={busy}
                onMention={onMention}
                onUpdate={onUpdateCustomBot}
                onDelete={onDeleteCustomBot}
              />
            ))}
            {customBots.length === 0 && (
              <div className="empty-block compact-empty">
                <Bot size={24} />
                <strong>暂无自建 Bot</strong>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}

function CustomBotManageCard({
  bot,
  canManage,
  busy,
  onMention,
  onUpdate,
  onDelete
}: {
  bot: BotInfo;
  canManage: boolean;
  busy: boolean;
  onMention: (mentionTarget: string) => void;
  onUpdate: (input: {
    botId: number;
    name: string;
    mentionName: string;
    aliases?: string[];
    description?: string;
    apiBaseUrl?: string;
    apiKey?: string;
    modelName: string;
    supportedModels?: string[];
    systemPrompt?: string;
  }) => Promise<void>;
  onDelete: (botId: number) => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState(bot.name || "");
  const [mentionName, setMentionName] = useState(bot.mentionName || "");
  const [aliases, setAliases] = useState((bot.aliases ?? []).join(", "));
  const [description, setDescription] = useState(bot.description || "");
  const [modelName, setModelName] = useState(bot.modelName || "");
  const [supportedModels, setSupportedModels] = useState((bot.supportedModels ?? []).join(", "));
  const [apiBaseUrl, setAPIBaseURL] = useState("");
  const [apiKey, setAPIKey] = useState("");
  const [systemPrompt, setSystemPrompt] = useState("");

  useEffect(() => {
    setName(bot.name || "");
    setMentionName(bot.mentionName || "");
    setAliases((bot.aliases ?? []).join(", "));
    setDescription(bot.description || "");
    setModelName(bot.modelName || "");
    setSupportedModels((bot.supportedModels ?? []).join(", "));
    setAPIBaseURL("");
    setAPIKey("");
    setSystemPrompt("");
  }, [bot.aliases, bot.description, bot.mentionName, bot.modelName, bot.name, bot.supportedModels]);

  const parseCSVValues = (raw: string) =>
    raw
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);

  const handleSave = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmedName = name.trim();
    const trimmedMentionName = mentionName.trim();
    const trimmedModelName = modelName.trim();
    if (!trimmedName || !trimmedMentionName || !trimmedModelName) {
      return;
    }
    await onUpdate({
      botId: bot.botId,
      name: trimmedName,
      mentionName: trimmedMentionName,
      aliases: parseCSVValues(aliases),
      description: description.trim(),
      apiBaseUrl: apiBaseUrl.trim() || undefined,
      apiKey: apiKey.trim() || undefined,
      modelName: trimmedModelName,
      supportedModels: parseCSVValues(supportedModels),
      systemPrompt: systemPrompt.trim() || undefined
    });
    setEditing(false);
  };

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
          <span>@{bot.mentionName} {" · "} {bot.modelName || "-"}</span>
        </div>
        <StatusPill label="自建" />
      </div>
      {bot.description && <p className="request-remark">{bot.description}</p>}
      {canManage && (
        <div className="friend-row-actions">
          <button className="secondary-button" disabled={busy} type="button" onClick={() => setEditing((value) => !value)}>
            {editing ? "取消编辑" : "编辑"}
          </button>
          <button
            className="danger-button"
            disabled={busy}
            type="button"
            onClick={() => {
              if (window.confirm(`确认删除自建 Bot「${bot.name}」吗？删除后将从会话中失效。`)) {
                void onDelete(bot.botId);
              }
            }}
          >
            <Trash2 size={16} />
            删除
          </button>
        </div>
      )}
      {canManage && editing && (
        <form className="drawer-form" onSubmit={handleSave}>
          <label className="field">
            <span>Bot 名称</span>
            <input value={name} onChange={(event) => setName(event.target.value)} required />
          </label>
          <label className="field">
            <span>提及名（不带 @）</span>
            <input value={mentionName} onChange={(event) => setMentionName(event.target.value)} required />
          </label>
          <label className="field">
            <span>别名（逗号分隔）</span>
            <input value={aliases} onChange={(event) => setAliases(event.target.value)} />
          </label>
          <label className="field">
            <span>描述</span>
            <input value={description} onChange={(event) => setDescription(event.target.value)} />
          </label>
          <label className="field">
            <span>模型名</span>
            <input value={modelName} onChange={(event) => setModelName(event.target.value)} required />
          </label>
          <label className="field">
            <span>支持模型（逗号分隔）</span>
            <input value={supportedModels} onChange={(event) => setSupportedModels(event.target.value)} />
          </label>
          <label className="field">
            <span>API Base URL（留空不改）</span>
            <input value={apiBaseUrl} onChange={(event) => setAPIBaseURL(event.target.value)} placeholder="例如：https://xxx/v1" />
          </label>
          <label className="field">
            <span>API Key（留空不改）</span>
            <input value={apiKey} onChange={(event) => setAPIKey(event.target.value)} placeholder="sk-***" />
          </label>
          <label className="field">
            <span>系统提示词（留空不改）</span>
            <textarea value={systemPrompt} onChange={(event) => setSystemPrompt(event.target.value)} rows={3} />
          </label>
          <button disabled={busy || !name.trim() || !mentionName.trim() || !modelName.trim()} type="submit">
            {busy ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
            保存自建 Bot
          </button>
        </form>
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
  onSaveModel,
  onSaveProfile
}: {
  bot: BotInfo;
  canManage: boolean;
  busy: boolean;
  onRemove: () => void;
  onMention: (mentionTarget: string) => void;
  onSaveModel: (modelName: string) => Promise<void>;
  onSaveProfile: (input: { displayNameOverride?: string; aliasesOverride?: string[] }) => Promise<void>;
}) {
  const [modelName, setModelName] = useState(bot.modelName || bot.supportedModels?.[0] || "");
  const [aliasesDraft, setAliasesDraft] = useState((bot.aliases ?? []).join(", "));
  const [displayNameDraft, setDisplayNameDraft] = useState(bot.displayName || bot.name || "");
  const permissionScopeLabel = toPermissionScopeLabel(bot.permissionScope);

  useEffect(() => {
    setModelName(bot.modelName || bot.supportedModels?.[0] || "");
    setAliasesDraft((bot.aliases ?? []).join(", "));
    setDisplayNameDraft(bot.displayName || bot.name || "");
  }, [bot.modelName, bot.supportedModels]);

  useEffect(() => {
    setAliasesDraft((bot.aliases ?? []).join(", "));
    setDisplayNameDraft(bot.displayName || bot.name || "");
  }, [bot.aliases, bot.displayName, bot.name]);

  const aliasOptions = aliasesDraft
    .split(",")
    .map((item) => item.trim())
    .filter(Boolean);

  const canSaveProfile = canManage && (aliasesDraft.trim() !== (bot.aliases ?? []).join(", ") || displayNameDraft.trim() !== (bot.displayName || bot.name || ""));

  const handleSaveProfile = async () => {
    const normalizedAliases = aliasesDraft
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
    const displayName = displayNameDraft.trim();
    await onSaveProfile({
      displayNameOverride: displayName || undefined,
      aliasesOverride: normalizedAliases.length > 0 ? normalizedAliases : undefined
    });
  };

  return (
    <div className="friend-card bot-card">
      <div className="friend-card-head">
        <Avatar
          name={bot.displayName || bot.name}
          src={bot.avatar}
          onContextMenu={(event) => handleAvatarMention(event, bot.aliases?.[0] || bot.mentionName, onMention)}
        />
        <div className="friend-card-meta">
          <strong>{bot.displayName || bot.name}</strong>
          <span>@{bot.mentionName} {" · "} {bot.memberType === "BOT" ? "机器人" : bot.memberType}</span>
        </div>
        <StatusPill label={bot.enabled ? "启用" : "禁用"} />
      </div>

      {bot.description && <p className="request-remark">{bot.description}</p>}

      <div className="bot-detail-fields">
        <div className="bot-field-row">
          <span className="bot-field-label">提及名</span>
          <span className="bot-field-value">@{bot.mentionName}</span>
        </div>
        <div className="bot-field-row">
          <span className="bot-field-label">别名</span>
          {canManage ? (
            <input
              className="bot-model-select"
              value={aliasesDraft}
              onChange={(event) => setAliasesDraft(event.target.value)}
              placeholder="用逗号分隔，例如：小助手,客服助手"
            />
          ) : (
            <span className="bot-field-value">{bot.aliases.join(", ") || "-"}</span>
          )}
        </div>
        <div className="bot-field-row">
          <span className="bot-field-label">显示昵称</span>
          {canManage ? (
            <div className="bot-model-editor">
              <select className="bot-model-select" value={displayNameDraft} onChange={(event) => setDisplayNameDraft(event.target.value)}>
                <option value={bot.name}>默认名称：{bot.name}</option>
                {aliasOptions.map((alias) => (
                  <option key={`display-${alias}`} value={alias}>
                    使用别名：{alias}
                  </option>
                ))}
              </select>
              <button className="secondary-button compact-button" disabled={busy || !canSaveProfile} type="button" onClick={() => void handleSaveProfile()}>
                保存资料
              </button>
            </div>
          ) : (
            <span className="bot-field-value">{bot.displayName || bot.name}</span>
          )}
        </div>
        <div className="bot-field-row">
          <span className="bot-field-label">权限范围</span>
          <span className="bot-field-value">{permissionScopeLabel}</span>
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
          <span className="bot-field-label">状态</span>
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

function isSameLocalDay(unixSeconds: number) {
  const now = new Date();
  const value = new Date(unixSeconds * 1000);
  return (
    now.getFullYear() === value.getFullYear() &&
    now.getMonth() === value.getMonth() &&
    now.getDate() === value.getDate()
  );
}

function dailyLimitByModelName(modelName: string) {
  const value = modelName.toLowerCase();
  if (value.includes("vl") || value.includes("vision")) {
    return 25_000;
  }
  return 50_000;
}

function resolveProviderNameFromModel(modelName: string) {
  const value = modelName.toLowerCase();
  if (value.includes("qwen") || value.includes("tongyi")) {
    return "aliyun";
  }
  if (value.includes("deepseek") || value.includes("dsv4")) {
    return "deepseek";
  }
  return "other";
}

function toPermissionScopeLabel(scope: string) {
  if (scope === "KNOWLEDGE_BASE_ONLY") return "仅知识库";
  if (scope === "CONVERSATION_AND_KB") return "群聊 + 知识库";
  return "群聊上下文";
}

function AccountView({
  user,
  sessions,
  busy,
  wsStatus,
  notificationStatus,
  notificationsEnabled,
  invisibleMode,
  userMemories,
  loadingUserMemories,
  onRefreshSessions,
  onLogout,
  onLogoutAll,
  onAvatarUpload,
  onRevokeSession,
  onToggleNotifications,
  onToggleInvisibleMode,
  onRefreshUserMemories,
  onWriteUserMemory,
  onUpdateUserMemory
}: {
  user: UserInfo;
  sessions: SessionInfo[];
  busy: boolean;
  wsStatus: WsStatus;
  notificationStatus: BrowserNotificationStatus;
  notificationsEnabled: boolean;
  invisibleMode: boolean;
  userMemories: UserMemoryInfo[];
  loadingUserMemories: boolean;
  onRefreshSessions: () => Promise<void>;
  onLogout: () => Promise<void>;
  onLogoutAll: (password: string) => Promise<void>;
  onAvatarUpload: (avatar: Blob) => Promise<void>;
  onRevokeSession: (sessionId: string, password: string) => Promise<void>;
  onToggleNotifications: () => Promise<void>;
  onToggleInvisibleMode: (invisible: boolean) => Promise<void>;
  onRefreshUserMemories: () => Promise<void>;
  onWriteUserMemory: (content: string) => Promise<void>;
  onUpdateUserMemory: (memoryId: number, content: string) => Promise<void>;
}) {
  const [password, setPassword] = useState("");
  const [newMemoryContent, setNewMemoryContent] = useState("");
  const [memoryDrafts, setMemoryDrafts] = useState<Record<number, string>>({});
  const [creatingMemory, setCreatingMemory] = useState(false);
  const [savingMemoryId, setSavingMemoryId] = useState<number | null>(null);
  const notificationLabel =
    notificationStatus === "unsupported"
      ? "当前浏览器不支持通知"
      : notificationStatus === "denied"
        ? "浏览器通知权限已被阻止"
        : notificationStatus === "granted"
          ? notificationsEnabled
            ? "浏览器通知已开启"
            : "浏览器通知已关闭"
          : "浏览器通知尚未授权";

  useEffect(() => {
    setMemoryDrafts((current) => {
      const next: Record<number, string> = {};
      for (const memory of userMemories) {
        const existing = current[memory.id];
        next[memory.id] = typeof existing === "string" ? existing : memory.content || "";
      }
      return next;
    });
  }, [userMemories]);

  const handleCreateMemory = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const value = newMemoryContent.trim();
    if (!value) {
      return;
    }
    if (value.length > 512) {
      window.alert("记忆内容不能超过 512 字");
      return;
    }
    setCreatingMemory(true);
    try {
      await onWriteUserMemory(value);
      setNewMemoryContent("");
    } finally {
      setCreatingMemory(false);
    }
  };

  const handleSaveMemory = async (memoryId: number) => {
    const value = memoryDrafts[memoryId] ?? "";
    if (value.length > 512) {
      window.alert("记忆内容不能超过 512 字");
      return;
    }
    setSavingMemoryId(memoryId);
    try {
      await onUpdateUserMemory(memoryId, value);
    } finally {
      setSavingMemoryId(null);
    }
  };

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
          {notificationStatus === "granted" ? (notificationsEnabled ? "关闭通知" : "开启通知") : "开启通知"}
        </button>
      </div>

      <div className="notification-card">
        <div>
          <ShieldCheck size={18} />
          <span>{invisibleMode ? "当前为隐身模式（好友侧显示离线）" : "当前为可见在线状态"}</span>
        </div>
        <button className={cx(invisibleMode && "is-on")} disabled={busy} type="button" onClick={() => void onToggleInvisibleMode(!invisibleMode)}>
          {invisibleMode ? "关闭隐身" : "开启隐身"}
        </button>
      </div>

      <div className="memory-card">
        <div className="memory-card-head">
          <strong>个人记忆</strong>
          <div className="memory-card-tools">
            <span className="memory-count">{userMemories.length} 条</span>
            <IconButton label="刷新记忆" onClick={() => void onRefreshUserMemories()}>
              <RefreshCw size={16} />
            </IconButton>
          </div>
        </div>
        <p className="memory-hint">最多 512 字。支持修改为空，用于清空该条记忆。</p>

        <form className="memory-create-form" onSubmit={handleCreateMemory}>
          <textarea
            value={newMemoryContent}
            onChange={(event) => setNewMemoryContent(event.target.value)}
            rows={3}
            maxLength={512}
            placeholder="新增一条个人记忆，例如：我偏好简洁回答，优先给结论。"
          />
          <button disabled={busy || creatingMemory || !newMemoryContent.trim()} type="submit">
            {creatingMemory ? <Loader2 className="spin" size={16} /> : <BadgePlus size={16} />}
            新增记忆
          </button>
        </form>

        {loadingUserMemories ? (
          <div className="memory-empty">
            <Loader2 className="spin" size={16} />
            <span>记忆加载中...</span>
          </div>
        ) : userMemories.length === 0 ? (
          <div className="memory-empty">
            <span>暂无记忆，新增后可在 AI 对话中优先参考。</span>
          </div>
        ) : (
          <div className="memory-list">
            {userMemories.map((memory) => (
              <div className="memory-row" key={memory.id}>
                <div className="memory-row-meta">
                  <span># {memory.id}</span>
                  <span>更新于 {formatRelative(memory.updatedAt)}</span>
                </div>
                <textarea
                  value={memoryDrafts[memory.id] ?? ""}
                  onChange={(event) =>
                    setMemoryDrafts((current) => ({
                      ...current,
                      [memory.id]: event.target.value
                    }))
                  }
                  rows={3}
                  maxLength={512}
                />
                <div className="memory-row-actions">
                  <span>{(memoryDrafts[memory.id] ?? "").length}/512</span>
                  <button
                    className="secondary-button compact-button"
                    disabled={busy || savingMemoryId === memory.id}
                    type="button"
                    onClick={() => void handleSaveMemory(memory.id)}
                  >
                    {savingMemoryId === memory.id ? <Loader2 className="spin" size={14} /> : <CheckCircle2 size={14} />}
                    保存修改
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
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
              <span>{session.last_ip || session.login_ip || "未知 IP"}</span>
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
