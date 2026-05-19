import { Bell, CheckCircle2, ChevronDown, Loader2, MessageSquarePlus, RefreshCw, Search, UserPlus, UsersRound } from "lucide-react";
import { FormEvent, useState } from "react";
import type { ConversationInfo, NotificationInfo, UserInfo } from "../../types";
import { Avatar, IconButton } from "../ui";
import { joinPolicies } from "../types";
import { conversationPreview, cx, formatRelative } from "../utils";

export function ConversationPanel({
  active,
  busy,
  conversations,
  notifications,
  notificationUnreadCount,
  unreadCounts,
  rawConversationCount,
  currentUser,
  selectedConversationId,
  search,
  onSearch,
  onCreateGroup,
  onJoinGroup,
  onRefresh,
  onMarkNotificationRead,
  onMarkAllNotificationsRead,
  onSelect
}: {
  active: boolean;
  busy: boolean;
  conversations: ConversationInfo[];
  notifications: NotificationInfo[];
  notificationUnreadCount: number;
  unreadCounts: Record<string, number>;
  rawConversationCount: number;
  currentUser: UserInfo;
  selectedConversationId: string | null;
  search: string;
  onSearch: (value: string) => void;
  onCreateGroup: (input: { name: string; announcement: string; joinPolicy: string }) => Promise<void>;
  onJoinGroup: (conversationId: string) => Promise<void>;
  onRefresh: () => Promise<ConversationInfo[]>;
  onMarkNotificationRead: (notificationId: number) => Promise<void>;
  onMarkAllNotificationsRead: () => Promise<void>;
  onSelect: (conversationId: string) => void;
}) {
  const [createOpen, setCreateOpen] = useState(false);
  const [joinOpen, setJoinOpen] = useState(false);
  const [groupName, setGroupName] = useState("");
  const [announcement, setAnnouncement] = useState("");
  const [joinPolicy, setJoinPolicy] = useState("FREE");
  const [joinID, setJoinID] = useState("");
  const [expandedNotificationId, setExpandedNotificationId] = useState<number | null>(null);
  const [notificationCollapsed, setNotificationCollapsed] = useState(true);

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

      <div className="conversation-notification-card">
        <div className="conversation-notification-header">
          <button
            className="conversation-notification-toggle"
            type="button"
            onClick={() => setNotificationCollapsed((current) => !current)}
          >
            <Bell size={14} />
            通知中心
          </button>
          <div className="conversation-notification-header-actions">
            {notificationUnreadCount > 0 && <span className="notification-unread-pill">{notificationUnreadCount > 99 ? "99+" : notificationUnreadCount}</span>}
            <button disabled={notificationUnreadCount === 0} type="button" onClick={() => void onMarkAllNotificationsRead()}>
              全部已读
            </button>
            <button
              aria-label={notificationCollapsed ? "展开通知中心" : "收起通知中心"}
              className="conversation-notification-collapse-btn"
              type="button"
              onClick={() => setNotificationCollapsed((current) => !current)}
            >
              <ChevronDown className={cx(notificationCollapsed && "open")} size={14} />
            </button>
          </div>
        </div>
        <div className={cx("conversation-notification-list", notificationCollapsed && "collapsed")}>
          {notifications.length === 0 && <span className="empty">暂无通知</span>}
          {notifications.map((item) => (
            <button
              key={item.id}
              className={cx("notification-item", !item.isRead && "unread")}
              type="button"
              onClick={() => {
                if (!item.isRead) {
                  void onMarkNotificationRead(item.id);
                }
                if (item.conversationId && item.category === "GROUP_SYSTEM") {
                  onSelect(item.conversationId);
                  return;
                }
                setExpandedNotificationId((current) => (current === item.id ? null : item.id));
              }}
            >
              <span className="notification-copy">
                <strong>{item.summary || item.title}</strong>
                {(item.detail || item.content) && <small>{item.detail || item.content}</small>}
                {expandedNotificationId === item.id && (item.detail || item.content) && (
                  <small className="notification-detail">{item.detail || item.content}</small>
                )}
                <time>{formatRelative(item.createdAt)}</time>
              </span>
              {!(item.conversationId && item.category === "GROUP_SYSTEM") && (
                <ChevronDown className={cx("notification-expand", expandedNotificationId === item.id && "open")} size={14} />
              )}
              {!item.isRead && <i />}
            </button>
          ))}
        </div>
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
                <span className="conversation-preview">{preview}</span>
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
