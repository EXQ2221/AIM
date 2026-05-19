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

type PendingImageDraft = {
  id: string;
  file: File;
  previewURL: string;
};

type PendingFileDraft = {
  id: string;
  file: File;
};

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
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
  peerTypingText,
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
  onDeleteLocalMessage,
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
  peerTypingText?: string;
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
  onDeleteLocalMessage: (message: MessageInfo) => void;
  onCancelReply: () => void;
  onSend: (payload?: OutgoingMessagePayload) => void;
}) {
  const [inviteOpen, setInviteOpen] = useState(false);
  const [inviteFriends, setInviteFriends] = useState<FriendInfo[]>([]);
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteInvitingId, setInviteInvitingId] = useState<number | null>(null);
  const [uploadingKind, setUploadingKind] = useState<"IMAGE" | "FILE" | "VOICE" | null>(null);
  const [voicePanelOpen, setVoicePanelOpen] = useState(false);
  const [voiceRecording, setVoiceRecording] = useState(false);
  const [voiceSeconds, setVoiceSeconds] = useState(0);
  const [mediaStatus, setMediaStatus] = useState("");
  const [imageDrafts, setImageDrafts] = useState<PendingImageDraft[]>([]);
  const [fileDrafts, setFileDrafts] = useState<PendingFileDraft[]>([]);
  const imageInputRef = useRef<HTMLInputElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const mediaStreamRef = useRef<MediaStream | null>(null);
  const voiceChunksRef = useRef<BlobPart[]>([]);
  const voiceStartedAtRef = useRef(0);
  const voiceTimerRef = useRef<number | null>(null);
  const discardVoiceUploadRef = useRef(false);

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

  const isRealtimeReady = Boolean(conversation && canSend && wsStatus === "open");
  const mediaBusy = uploadingKind !== null || voiceRecording;

  const ensureMediaReady = useCallback(() => {
    if (!isRealtimeReady) {
      alert("当前连接不可用，请等待实时连接恢复后再发送。");
      return false;
    }
    return true;
  }, [isRealtimeReady]);

  const stopVoiceTimer = useCallback(() => {
    if (voiceTimerRef.current !== null) {
      window.clearInterval(voiceTimerRef.current);
      voiceTimerRef.current = null;
    }
  }, []);

  const stopVoiceStream = useCallback(() => {
    if (mediaStreamRef.current) {
      mediaStreamRef.current.getTracks().forEach((track) => track.stop());
      mediaStreamRef.current = null;
    }
  }, []);

  const clearImageDrafts = useCallback(() => {
    setImageDrafts((current) => {
      current.forEach((item) => URL.revokeObjectURL(item.previewURL));
      return [];
    });
  }, []);

  const clearFileDrafts = useCallback(() => {
    setFileDrafts([]);
  }, []);

  const removeImageDraft = useCallback((id: string) => {
    setImageDrafts((current) => {
      const target = current.find((item) => item.id === id);
      if (target) {
        URL.revokeObjectURL(target.previewURL);
      }
      return current.filter((item) => item.id !== id);
    });
  }, []);

  const removeFileDraft = useCallback((id: string) => {
    setFileDrafts((current) => current.filter((item) => item.id !== id));
  }, []);

  const triggerPickImage = useCallback(() => {
    if (mediaBusy) {
      return;
    }
    imageInputRef.current?.click();
  }, [mediaBusy]);

  const triggerPickFile = useCallback(() => {
    if (mediaBusy) {
      return;
    }
    fileInputRef.current?.click();
  }, [mediaBusy]);

  const handleImageChosen = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(event.target.files ?? []);
      event.target.value = "";
      if (files.length === 0) {
        return;
      }
      const created = files.map((file, index) => ({
        id: `${Date.now()}-${index}-${Math.random().toString(16).slice(2)}`,
        file,
        previewURL: URL.createObjectURL(file)
      }));
      setImageDrafts((current) => [...current, ...created]);
    },
    []
  );

  const handleFileChosen = useCallback(
    (event: ChangeEvent<HTMLInputElement>) => {
      const files = Array.from(event.target.files ?? []);
      event.target.value = "";
      if (files.length === 0) {
        return;
      }
      const created = files.map((file, index) => ({
        id: `${Date.now()}-${index}-${Math.random().toString(16).slice(2)}`,
        file
      }));
      setFileDrafts((current) => [...current, ...created]);
    },
    []
  );

  const handleSendMediaDrafts = useCallback(async () => {
    if (!ensureMediaReady() || mediaBusy) {
      return;
    }
    if (imageDrafts.length === 0 && fileDrafts.length === 0) {
      return;
    }

    const failedImages: PendingImageDraft[] = [];
    const failedFiles: PendingFileDraft[] = [];
    const failedNames: string[] = [];

    try {
      if (imageDrafts.length > 0) {
        setUploadingKind("IMAGE");
        for (let index = 0; index < imageDrafts.length; index += 1) {
          const draft = imageDrafts[index];
          setMediaStatus(`图片上传中 (${index + 1}/${imageDrafts.length})：${draft.file.name}`);
          try {
            const uploaded = await api.uploadImage(draft.file);
            onSend({
              messageType: "IMAGE",
              contentPayload: {
                url: uploaded.file.url,
                name: uploaded.file.filename || draft.file.name,
                mimeType: uploaded.file.content_type || draft.file.type || "image/*",
                size: uploaded.file.size
              }
            });
            URL.revokeObjectURL(draft.previewURL);
          } catch {
            failedImages.push(draft);
            failedNames.push(draft.file.name);
          }
        }
      }

      if (fileDrafts.length > 0) {
        setUploadingKind("FILE");
        for (let index = 0; index < fileDrafts.length; index += 1) {
          const draft = fileDrafts[index];
          setMediaStatus(`文件上传中 (${index + 1}/${fileDrafts.length})：${draft.file.name}`);
          try {
            const uploaded = await api.uploadFile(draft.file);
            onSend({
              messageType: "FILE",
              contentPayload: {
                url: uploaded.file.url,
                name: uploaded.file.filename || draft.file.name,
                mimeType: uploaded.file.content_type || draft.file.type || "application/octet-stream",
                size: uploaded.file.size
              }
            });
          } catch {
            failedFiles.push(draft);
            failedNames.push(draft.file.name);
          }
        }
      }
    } finally {
      setImageDrafts(failedImages);
      setFileDrafts(failedFiles);
      setUploadingKind(null);
      setMediaStatus("");
    }

    if (failedNames.length > 0) {
      alert(`以下文件发送失败，请重试：\n${failedNames.join("\n")}`);
    }
  }, [ensureMediaReady, fileDrafts, imageDrafts, mediaBusy, onSend]);

  const stopVoiceRecording = useCallback(() => {
    const recorder = mediaRecorderRef.current;
    if (!recorder || recorder.state === "inactive") {
      return;
    }
    recorder.stop();
  }, []);

  const startVoiceRecording = useCallback(async () => {
    if (!ensureMediaReady() || mediaBusy || voiceRecording) {
      return;
    }
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== "inactive") {
      return;
    }
    if (!navigator.mediaDevices?.getUserMedia || typeof MediaRecorder === "undefined") {
      alert("当前浏览器不支持录音，请使用文件发送语音。");
      return;
    }

    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      mediaStreamRef.current = stream;
      voiceChunksRef.current = [];
      discardVoiceUploadRef.current = false;
      voiceStartedAtRef.current = Date.now();
      setVoiceSeconds(0);
      setMediaStatus("录音中，松开发送");

      const preferredMimeTypes = ["audio/webm;codecs=opus", "audio/webm", "audio/mp4"];
      const selectedMimeType = preferredMimeTypes.find((item) => MediaRecorder.isTypeSupported(item));
      const recorder = selectedMimeType ? new MediaRecorder(stream, { mimeType: selectedMimeType }) : new MediaRecorder(stream);
      mediaRecorderRef.current = recorder;

      recorder.ondataavailable = (dataEvent: BlobEvent) => {
        if (dataEvent.data.size > 0) {
          voiceChunksRef.current.push(dataEvent.data);
        }
      };

      recorder.onstop = async () => {
        setVoiceRecording(false);
        stopVoiceTimer();
        stopVoiceStream();

        const durationMs = Math.max(1, Date.now() - voiceStartedAtRef.current);
        const blob = new Blob(voiceChunksRef.current, { type: recorder.mimeType || "audio/webm" });
        mediaRecorderRef.current = null;
        voiceChunksRef.current = [];
        if (discardVoiceUploadRef.current) {
          discardVoiceUploadRef.current = false;
          setMediaStatus("");
          return;
        }

        if (blob.size <= 0) {
          setMediaStatus("未录到声音，请重试");
          return;
        }

        setUploadingKind("VOICE");
        setMediaStatus("语音上传中...");
        try {
          const ext = recorder.mimeType.includes("mp4") ? "m4a" : "webm";
          const file = new File([blob], `voice-${Date.now()}.${ext}`, {
            type: recorder.mimeType || "audio/webm"
          });
          const uploaded = await api.uploadVoice(file);
          onSend({
            messageType: "VOICE",
            contentPayload: {
              url: uploaded.file.url,
              name: uploaded.file.filename || file.name,
              mimeType: uploaded.file.content_type || file.type || "audio/webm",
              size: uploaded.file.size,
              durationMs
            }
          });
          setMediaStatus("");
        } catch (error) {
          setMediaStatus("");
          alert(error instanceof Error ? error.message : "语音上传失败");
        } finally {
          setUploadingKind(null);
        }
      };

      recorder.onerror = () => {
        setMediaStatus("录音失败，请重试");
      };

      recorder.start();
      setVoiceRecording(true);
      voiceTimerRef.current = window.setInterval(() => {
        setVoiceSeconds(Math.floor((Date.now() - voiceStartedAtRef.current) / 1000));
      }, 250);
    } catch (error) {
      setVoiceRecording(false);
      stopVoiceTimer();
      stopVoiceStream();
      setMediaStatus("");
      alert(error instanceof Error ? error.message : "无法启用麦克风");
    }
  }, [ensureMediaReady, mediaBusy, onSend, stopVoiceStream, stopVoiceTimer, voiceRecording]);

  const sendDisabled = !conversation || !canSend || !messageDraft.trim() || wsStatus !== "open";
  const composerDisabled = !conversation || !canSend || wsStatus !== "open";
  const hasMediaDrafts = imageDrafts.length > 0 || fileDrafts.length > 0;
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

  useEffect(
    () => () => {
      discardVoiceUploadRef.current = true;
      clearImageDrafts();
      clearFileDrafts();
      stopVoiceRecording();
      stopVoiceTimer();
      stopVoiceStream();
    },
    [clearFileDrafts, clearImageDrafts, stopVoiceRecording, stopVoiceStream, stopVoiceTimer]
  );

  useEffect(() => {
    setMediaStatus("");
    setVoicePanelOpen(false);
    setVoiceSeconds(0);
    clearImageDrafts();
    clearFileDrafts();
    discardVoiceUploadRef.current = true;
    stopVoiceRecording();
    stopVoiceTimer();
    stopVoiceStream();
  }, [clearFileDrafts, clearImageDrafts, conversation?.conversationId, stopVoiceRecording, stopVoiceStream, stopVoiceTimer]);

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
              {peerTypingText && <span className="chat-typing-status">{peerTypingText}</span>}
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
                    onDeleteLocal={() => onDeleteLocalMessage(message)}
                    mentionTarget={message.senderType === "BOT" ? sender?.mentionName || sender?.nickname || "ai" : sender?.nickname}
                    senderName={
                      message.senderType === "BOT" ? sender?.nickname || sender?.mentionName || "AI 助手" : sender?.nickname || `用户 ${message.senderId}`
                    }
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
              <input ref={imageInputRef} className="visually-hidden" type="file" accept="image/*" multiple onChange={handleImageChosen} />
              <input ref={fileInputRef} className="visually-hidden" type="file" multiple onChange={handleFileChosen} />

              {hasMediaDrafts && (
                <div className="media-composer">
                  <div className="composer-draft-header">
                    <strong>待发送附件</strong>
                    <span>{`图片 ${imageDrafts.length} · 文件 ${fileDrafts.length}`}</span>
                  </div>
                  {imageDrafts.length > 0 && (
                    <div className="composer-image-draft-list">
                      {imageDrafts.map((draft) => (
                        <div key={draft.id} className="composer-image-draft-item">
                          <img alt={draft.file.name} src={draft.previewURL} />
                          <div className="composer-image-draft-meta">
                            <span title={draft.file.name}>{draft.file.name}</span>
                            <small>{formatBytes(draft.file.size)}</small>
                          </div>
                          <button disabled={mediaBusy} type="button" onClick={() => removeImageDraft(draft.id)}>
                            <X size={14} />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                  {fileDrafts.length > 0 && (
                    <div className="composer-file-draft-list">
                      {fileDrafts.map((draft) => (
                        <div key={draft.id} className="composer-file-draft-item">
                          <div className="composer-file-draft-meta">
                            <span title={draft.file.name}>{draft.file.name}</span>
                            <small>{formatBytes(draft.file.size)}</small>
                          </div>
                          <button disabled={mediaBusy} type="button" onClick={() => removeFileDraft(draft.id)}>
                            <X size={14} />
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                  <div className="media-composer-actions">
                    <button disabled={mediaBusy || composerDisabled} type="button" onClick={() => void handleSendMediaDrafts()}>
                      发送全部
                    </button>
                    <button
                      disabled={mediaBusy}
                      type="button"
                      onClick={() => {
                        clearImageDrafts();
                        clearFileDrafts();
                      }}
                    >
                      清空
                    </button>
                  </div>
                </div>
              )}

              <div className="composer-tools">
                <button disabled={mediaBusy || composerDisabled} type="button" onClick={triggerPickImage}>
                  <FileImage size={16} />
                  图片
                </button>
                <button disabled={mediaBusy || composerDisabled} type="button" onClick={triggerPickFile}>
                  <Paperclip size={16} />
                  文件
                </button>
                <button
                  className={cx("voice-toggle-button", voiceRecording && "recording", voicePanelOpen && "active")}
                  disabled={uploadingKind !== null || composerDisabled}
                  type="button"
                  onClick={() => setVoicePanelOpen((current) => !current)}
                >
                  <Mic size={16} />
                  {voiceRecording ? `录音中 ${voiceSeconds}s` : "语音"}
                </button>
              </div>
              {voicePanelOpen && (
                <div className="media-composer">
                  <div className="voice-preview-meta">
                    <strong>{voiceRecording ? `录音中 ${voiceSeconds}s` : "按住说话，松开发送"}</strong>
                    <span>和 QQ 一样：按住按钮录音，松开发送。</span>
                  </div>
                  <div className="media-composer-actions">
                    <button
                      className={cx("voice-toggle-button", voiceRecording && "recording")}
                      disabled={uploadingKind !== null || composerDisabled}
                      type="button"
                      onMouseDown={() => void startVoiceRecording()}
                      onMouseUp={stopVoiceRecording}
                      onMouseLeave={stopVoiceRecording}
                      onTouchStart={() => void startVoiceRecording()}
                      onTouchEnd={stopVoiceRecording}
                      onTouchCancel={stopVoiceRecording}
                      onContextMenu={(event) => event.preventDefault()}
                    >
                      {voiceRecording ? "松开发送" : "按住说话"}
                    </button>
                  </div>
                </div>
              )}
              {mediaStatus && <div className="composer-media-status">{mediaStatus}</div>}

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

