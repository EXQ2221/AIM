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
import { FormEvent, useEffect, useMemo, useState } from "react";
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
  SessionInfo,
  UserInfo
} from "../../types";
import { AvatarUploader } from "../avatar-uploader";
import { Avatar, IconButton, StatusPill, WsBadge } from "../ui";
import type { BrowserNotificationStatus, DetailTab, WsStatus } from "../types";
import { cx, formatRelative, handleAvatarMention, parseGroupValue, roleLabel, statusLabel } from "../utils";

function isMemberMuted(member: Pick<MemberInfo, "muteUntil"> | null | undefined) {
  return Boolean(member?.muteUntil && member.muteUntil > Math.floor(Date.now() / 1000));
}

function formatMuteUntil(value?: number | null) {
  if (!value || value <= 0) return "";
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", { hour12: false });
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
