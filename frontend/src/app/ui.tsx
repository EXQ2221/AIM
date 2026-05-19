import type React from "react";
import { useEffect, useRef, useState } from "react";
import { FileText, Loader2, MessageCircle, Mic, SendHorizontal, UserPlus, UserRound, Wifi, WifiOff } from "lucide-react";
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

type ContextMenuState = {
  x: number;
  y: number;
};

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
  onRecall,
  onDeleteLocal
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
  onDeleteLocal?: () => void;
}) {
  const imageContent = message.messageType === "IMAGE" ? parseImageMessageContent(message.content) : null;
  const fileContent = message.messageType === "FILE" ? parseFileMessageContent(message.content) : null;
  const systemContent = message.messageType === "SYSTEM" ? parseSystemMessageContent(message.content) : null;
  const voiceContent = message.messageType === "VOICE" ? parseVoiceMessageContent(message.content) : null;
  const recalled = message.status === "RECALLED";
  const [contextMenu, setContextMenu] = useState<ContextMenuState | null>(null);
  const canDeleteLocally = Boolean(onDeleteLocal && !message.pending && !message.isBotGenerating);
  const contextMenuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!contextMenu) {
      return;
    }
    const handlePointerDown = (event: PointerEvent) => {
      const target = event.target as Node | null;
      if (target && contextMenuRef.current?.contains(target)) {
        return;
      }
      setContextMenu(null);
    };
    const handleScroll = () => {
      setContextMenu(null);
    };
    const handleEsc = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setContextMenu(null);
      }
    };
    window.addEventListener("pointerdown", handlePointerDown);
    window.addEventListener("scroll", handleScroll, true);
    window.addEventListener("keydown", handleEsc);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
      window.removeEventListener("scroll", handleScroll, true);
      window.removeEventListener("keydown", handleEsc);
    };
  }, [contextMenu]);

  const handleContextMenu = (event: React.MouseEvent<HTMLDivElement>) => {
    if (!canDeleteLocally) {
      return;
    }
    event.preventDefault();
    setContextMenu({ x: event.clientX, y: event.clientY });
  };

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
  const streamMarkdownText = message.isBotGenerating ? message.content : markdownText;

  return (
    <article data-message-id={String(message.id)} className={cx("message-row", mine && "mine", highlighted && "highlighted")}>
      {!mine && (
        <Avatar
          name={senderName}
          src={senderAvatar}
          onContextMenu={(event) => handleAvatarMention(event, mentionTarget || senderName, onMention)}
        />
      )}
      <div className="message-bubble" onContextMenu={handleContextMenu}>
        <div className="message-meta">
          <span>{senderName}</span>
          <time>{formatClock(message.createdAt)}</time>
          {onReply && !message.pending && message.status === "NORMAL" && !message.isBotGenerating && (
            <button className="message-reply-button" type="button" onClick={onReply}>
              {"\u56de\u590d"}
            </button>
          )}
          {onRecall && mine && !message.pending && message.status === "NORMAL" && !message.isBotGenerating && (
            <button className="message-reply-button" type="button" onClick={onRecall}>
              {"\u64a4\u56de"}
            </button>
          )}
          {!message.isBotGenerating && (message.pending || message.status === "FAILED") && (
            <span className={cx("message-state", message.pending && "pending", message.status === "FAILED" && "failed")}>
              {message.pending ? "\u53d1\u9001\u4e2d" : "\u53d1\u9001\u5931\u8d25"}
            </span>
          )}
        </div>
        {(message.replyTo || message.replyToId) && (
          <div className="message-reply-preview">
            {replyThumbnailURL && <img className="message-reply-thumbnail" src={replyThumbnailURL} alt="" />}
            <strong>{replySummaryLabel || "\u539f\u6d88\u606f\u4e0d\u53ef\u7528"}</strong>
            <span>{message.replyTo?.contentPreview || "\u539f\u6d88\u606f\u4e0d\u53ef\u7528"}</span>
            {onJumpToReply && message.replyToId && (
              <button className="message-reply-jump" type="button" onClick={onJumpToReply}>
                {"\u8df3\u8f6c"}
              </button>
            )}
          </div>
        )}
        {recalled ? (
          <p>{messageText(message)}</p>
        ) : message.isBotGenerating ? (
          <div className="message-bot-generating">
            <div className="message-markdown">
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{streamMarkdownText || "..."}</ReactMarkdown>
            </div>
            <span className="message-state pending">
              <Loader2 className="spin" size={14} />
              {"\u751f\u6210\u4e2d"}
            </span>
          </div>
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
              <span>{[formatFileSize(fileContent.size), fileContent.mimeType].filter(Boolean).join(" \u00b7 ")}</span>
            </div>
            <a href={fileContent.url} target="_blank" rel="noreferrer">
              {"\u4e0b\u8f7d"}
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
      {contextMenu && canDeleteLocally && (
        <div ref={contextMenuRef} className="message-context-menu" style={{ left: contextMenu.x, top: contextMenu.y }}>
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              setContextMenu(null);
              onDeleteLocal?.();
            }}
          >
            {"\u4ec5\u6211\u5220\u9664"}
          </button>
        </div>
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
      {open ? "\u5df2\u8fde\u63a5" : status === "connecting" ? "\u8fde\u63a5\u4e2d" : "\u5df2\u65ad\u5f00"}
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
        {"\u4f1a\u8bdd"}
      </button>
      <button disabled={!hasConversation} className={active === "chat" ? "active" : ""} type="button" onClick={() => onChange("chat")}>
        <SendHorizontal size={20} />
        {"\u804a\u5929"}
      </button>
      <button className={active === "friends" ? "active" : ""} type="button" onClick={() => onChange("friends")}>
        <UserPlus size={20} />
        {"\u597d\u53cb"}
      </button>
      <button className={active === "account" ? "active" : ""} type="button" onClick={() => onChange("account")}>
        <UserRound size={20} />
        {"\u6211\u7684"}
      </button>
    </nav>
  );
}
