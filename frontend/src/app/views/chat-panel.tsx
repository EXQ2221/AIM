import { ChevronLeft, DoorOpen, FileImage, Loader2, LockKeyhole, MessageCircle, Mic, PanelRightOpen, Paperclip, RefreshCw, SendHorizontal, UserPlus, X } from "lucide-react";
import { ChangeEvent, KeyboardEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../../api";
import { Avatar, IconButton, MessageBubble, WsBadge } from "../ui";
import { cx, roleLabel } from "../utils";
import type { ConversationInfo, FriendInfo, MemberInfo, MessageInfo, OutgoingMessagePayload, ReplyPreviewInfo, UserInfo } from "../../types";
import type { WsStatus } from "../types";

function readReceiptLabel(conversation: ConversationInfo | null, message: MessageInfo, mine: boolean) {
  if (!conversation || !mine || message.pending || message.status === "FAILED") {
    return undefined;
  }
  if (conversation.type === "SINGLE") {
    return message.readByPeer ? "已读" : "未读";
  }
  if (conversation.type === "GROUP") {
    return `已读 ${message.readCount ?? 0}`;
  }
  return undefined;
}

function isMemberMuted(member: Pick<MemberInfo, "muteUntil"> | null | undefined) {
  return Boolean(member?.muteUntil && member.muteUntil > Math.floor(Date.now() / 1000));
}

function formatMuteUntil(value?: number | null) {
  if (!value || value <= 0) return "";
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return "";
  return date.toLocaleString("zh-CN", { hour12: false });
}
export function ChatPanel({
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
        contentPayload: payload
      });
      resetMediaComposer();
      return;
    }

    if (mediaMode === "FILE") {
      const size = Number(mediaSize);
      if (!Number.isFinite(size) || size <= 0) return;
      onSend({
        messageType: "FILE",
        contentPayload: {
          url,
          name,
          mimeType,
          size
        }
      });
      resetMediaComposer();
      return;
    }

    const durationMs = Number(voiceDurationMs);
    if (!Number.isFinite(durationMs) || durationMs <= 0) return;
    onSend({
      messageType: "VOICE",
      contentPayload: {
        url,
        name,
        mimeType,
        size: mediaSize.trim() ? Number(mediaSize) : undefined,
        durationMs
      }
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

