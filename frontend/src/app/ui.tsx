import type React from "react";
import { Bot, FileText, MessageCircle, Mic, SendHorizontal, UserPlus, UserRound, Wifi, WifiOff } from "lucide-react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { MessageInfo, MobilePane } from "../types";
import type { ToastState, WsStatus } from "./types";
import {
  cx,
  formatClock,
  formatFileSize,
  formatVoiceDuration,
  handleAvatarMention,
  initials,
  messageText,
  parseFileMessageContent,
  parseImageMessageContent,
  parseSystemMessageContent,
  parseVoiceMessageContent
} from "./utils";

export function MessageBubble({
  message,
  mine,
  senderName,
  senderAvatar,
  highlighted,
  readReceiptLabel,
  mentionTarget,
  onMention,
  replySummaryLabel,
  replyThumbnailURL,
  onJumpToReply,
  onReply,
  onRecall
}: {
  message: MessageInfo;
  mine: boolean;
  senderName: string;
  senderAvatar?: string;
  highlighted?: boolean;
  readReceiptLabel?: string;
  mentionTarget?: string;
  onMention: (mentionTarget: string) => void;
  replySummaryLabel?: string;
  replyThumbnailURL?: string;
  onJumpToReply?: () => void;
  onReply?: () => void;
  onRecall?: () => void;
}) {
  const imageContent = message.messageType === "IMAGE" ? parseImageMessageContent(message.content) : null;
  const fileContent = message.messageType === "FILE" ? parseFileMessageContent(message.content) : null;
  const systemContent = message.messageType === "SYSTEM" ? parseSystemMessageContent(message.content) : null;
  const voiceContent = message.messageType === "VOICE" ? parseVoiceMessageContent(message.content) : null;
  const recalled = message.status === "RECALLED";

  if (message.messageType === "SYSTEM") {
    return (
      <article className="message-row system">
        <div className="system-message-row">
          <span>{systemContent?.text || messageText(message)}</span>
          <time>{formatClock(message.createdAt)}</time>
        </div>
      </article>
    );
  }

  const markdownText = messageText(message);
  const enableMarkdown = !recalled && (message.messageType === "TEXT" || message.messageType === "BOT_REPLY");

  return (
    <article data-message-id={String(message.id)} className={cx("message-row", mine && "mine", highlighted && "highlighted")}>
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
          {onReply && !message.pending && message.status === "NORMAL" && (
            <button className="message-reply-button" type="button" onClick={onReply}>
              回复
            </button>
          )}
          {onRecall && mine && !message.pending && message.status === "NORMAL" && (
            <button className="message-reply-button" type="button" onClick={onRecall}>
              撤回
            </button>
          )}
          {(message.pending || message.status === "FAILED") && (
            <span className={cx("message-state", message.pending && "pending", message.status === "FAILED" && "failed")}>
              {message.pending ? "发送中" : "发送失败"}
            </span>
          )}
        </div>
        {(message.replyTo || message.replyToId) && (
          <div className="message-reply-preview">
            {replyThumbnailURL && (
              <img
                className="message-reply-thumbnail"
                src={replyThumbnailURL}
                alt=""
              />
            )}
            <strong>{replySummaryLabel || "原消息不可用"}</strong>
            <span>{message.replyTo?.contentPreview || "原消息不可用"}</span>
            {onJumpToReply && message.replyToId && (
              <button className="message-reply-jump" type="button" onClick={onJumpToReply}>
                跳转
              </button>
            )}
          </div>
        )}
        {recalled ? (
          <p>{messageText(message)}</p>
        ) : message.messageType === "IMAGE" && imageContent ? (
          <div className="message-media message-image">
            <a href={imageContent.url} target="_blank" rel="noreferrer">
              <img alt={imageContent.name} src={imageContent.url} />
            </a>
            <span>{imageContent.name}</span>
            {imageContent.text && <p>{imageContent.text}</p>}
          </div>
        ) : message.messageType === "FILE" && fileContent ? (
          <div className="message-media message-file">
            <span className="message-file-icon">
              <FileText size={18} />
            </span>
            <div className="message-file-copy">
              <strong>{fileContent.name}</strong>
              <span>{[formatFileSize(fileContent.size), fileContent.mimeType].filter(Boolean).join(" · ")}</span>
            </div>
            <a href={fileContent.url} target="_blank" rel="noreferrer">
              下载
            </a>
          </div>
        ) : message.messageType === "VOICE" && voiceContent ? (
          <div className="message-media message-voice">
            <div className="message-voice-meta">
              <Mic size={18} />
              <span>{voiceContent.name}</span>
              <span>{formatVoiceDuration(voiceContent.durationMs)}</span>
            </div>
            <audio controls preload="none" src={voiceContent.url} />
          </div>
        ) : enableMarkdown ? (
          <div className="message-markdown">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>{markdownText}</ReactMarkdown>
          </div>
        ) : (
          <p>{markdownText}</p>
        )}
        {mine && !message.pending && message.status !== "FAILED" && readReceiptLabel && (
          <div className="message-read-receipt">{readReceiptLabel}</div>
        )}
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
    <nav className="mobile-nav">
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
      <button className={active === "account" ? "active" : ""} type="button" onClick={() => onChange("account")}>
        <UserRound size={20} />
        我的
      </button>
    </nav>
  );
}
