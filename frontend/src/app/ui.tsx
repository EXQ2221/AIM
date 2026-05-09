import type React from "react";
import { Bot, MessageCircle, SendHorizontal, UserPlus, UserRound, Wifi, WifiOff } from "lucide-react";
import type { MessageInfo, MobilePane } from "../types";
import type { ToastState, WsStatus } from "./types";
import { cx, formatClock, handleAvatarMention, initials } from "./utils";

export function MessageBubble({
  message,
  mine,
  senderName,
  senderAvatar,
  mentionTarget,
  onMention
}: {
  message: MessageInfo;
  mine: boolean;
  senderName: string;
  senderAvatar?: string;
  mentionTarget?: string;
  onMention: (mentionTarget: string) => void;
}) {
  return (
    <article className={cx("message-row", mine && "mine")}>
      {!mine && (
        <Avatar
          name={senderName}
          src={senderAvatar}
          onContextMenu={(event) => handleAvatarMention(event, mentionTarget || senderName, onMention)}
        />
      )}
      <div className="message-bubble">
        <div className="message-meta">
          <span>{senderName}</span>
          <time>{formatClock(message.createdAt)}</time>
          {(message.pending || message.status === "FAILED") && (
            <span className={cx("message-state", message.pending && "pending", message.status === "FAILED" && "failed")}>
              {message.pending ? "发送中" : "发送失败"}
            </span>
          )}
        </div>
        <p>{message.status === "RECALLED" ? "消息已撤回" : message.content}</p>
      </div>
      {mine && (
        <Avatar
          name={senderName}
          src={senderAvatar}
          onContextMenu={(event) => handleAvatarMention(event, mentionTarget || senderName, onMention)}
        />
      )}
    </article>
  );
}

export function Field({ icon, label, children }: { icon: React.ReactNode; label: string; children: React.ReactNode }) {
  return (
    <label className="field">
      <span>
        {icon}
        {label}
      </span>
      {children}
    </label>
  );
}

export function Avatar({
  name,
  src,
  size = "normal",
  onContextMenu
}: {
  name: string;
  src?: string;
  size?: "normal" | "large";
  onContextMenu?: (event: React.MouseEvent<HTMLImageElement | HTMLSpanElement>) => void;
}) {
  if (src) {
    return <img className={cx("avatar", size === "large" && "large")} alt="" src={src} onContextMenu={onContextMenu} />;
  }
  return (
    <span className={cx("avatar", size === "large" && "large")} onContextMenu={onContextMenu}>
      {initials(name)}
    </span>
  );
}

export function IconButton({
  label,
  children,
  className,
  disabled,
  onClick
}: {
  label: string;
  children: React.ReactNode;
  className?: string;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      aria-label={label}
      className={cx("icon-button", className)}
      disabled={disabled}
      title={label}
      type="button"
      onClick={onClick}
    >
      {children}
    </button>
  );
}

export function WsBadge({ status }: { status: WsStatus }) {
  const open = status === "open";
  return (
    <span className={cx("ws-badge", open && "online")}>
      {open ? <Wifi size={15} /> : <WifiOff size={15} />}
      {open ? "已连接" : status === "connecting" ? "连接中" : "已断开"}
    </span>
  );
}

export function StatusPill({ label }: { label: string }) {
  return <span className="status-pill">{label}</span>;
}

export function Toast({ toast, onClose }: { toast: ToastState; onClose: () => void }) {
  if (!toast) return null;
  return (
    <button className={cx("toast", toast.tone)} type="button" onClick={onClose}>
      {toast.message}
    </button>
  );
}

export function MobileNav({
  active,
  hasConversation,
  onChange
}: {
  active: MobilePane;
  hasConversation: boolean;
  onChange: (pane: MobilePane) => void;
}) {
  return (
    <nav className="mobile-nav mobile-nav-five">
      <button className={active === "conversations" ? "active" : ""} type="button" onClick={() => onChange("conversations")}>
        <MessageCircle size={20} />
        会话
      </button>
      <button disabled={!hasConversation} className={active === "chat" ? "active" : ""} type="button" onClick={() => onChange("chat")}>
        <SendHorizontal size={20} />
        聊天
      </button>
      <button className={active === "friends" ? "active" : ""} type="button" onClick={() => onChange("friends")}>
        <UserPlus size={20} />
        好友
      </button>
      <button disabled={!hasConversation} className={active === "bots" ? "active" : ""} type="button" onClick={() => onChange("bots")}>
        <Bot size={20} />
        AI
      </button>
      <button className={active === "account" ? "active" : ""} type="button" onClick={() => onChange("account")}>
        <UserRound size={20} />
        我的
      </button>
    </nav>
  );
}
