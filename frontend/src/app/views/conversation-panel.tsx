import { CheckCircle2, Loader2, MessageSquarePlus, RefreshCw, Search, UserPlus, UsersRound } from "lucide-react";
import { FormEvent, useState } from "react";
import { Avatar, IconButton } from "../ui";
import { joinPolicies } from "../types";
import { conversationPreview, cx, formatRelative } from "../utils";
import type { ConversationInfo, UserInfo } from "../../types";
export function ConversationPanel({
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


