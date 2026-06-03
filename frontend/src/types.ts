export type APIResponse<T> = {
  code: number;
  message: string;
  data?: T;
};

export type UserInfo = {
  user_id: number;
  aim_id: string;
  email: string;
  nickname: string;
  avatar: string;
  status: string;
  role: string;
  token_version: number;
  created_at: number;
  updated_at: number;
};

export type AuthSessionResponse = {
  session_id: string;
  device_id: string;
  access_expires_at: number;
  refresh_expires_at: number;
};

export type SessionInfo = {
  session_id: string;
  device_id: string;
  device_name: string;
  user_agent: string;
  login_ip: string;
  last_ip: string;
  status: string;
  current: boolean;
  created_at: number;
  last_seen_at: number;
};

export type UploadedFileInfo = {
  url: string;
  filename: string;
  content_type: string;
  size: number;
};

export type UploadAvatarResponse = {
  avatar: string;
  file: UploadedFileInfo;
  user: UserInfo;
};

export type UploadMediaResponse = {
  file: UploadedFileInfo;
};

export type FriendGroupInfo = {
  id: number;
  name: string;
  sort_order: number;
  created_at: number;
  updated_at: number;
};

export type FriendInfo = {
  user_id: number;
  aim_id: string;
  nickname: string;
  avatar: string;
  remark: string;
  group_id?: number | null;
  status: string;
  is_online?: boolean;
  presence?: "ONLINE" | "OFFLINE" | string;
  created_at: number;
  updated_at: number;
};

export type PresenceSettings = {
  invisible: boolean;
};

export type FriendRequestInfo = {
  id: number;
  user_id: number;
  aim_id: string;
  nickname: string;
  avatar: string;
  direction: "INCOMING" | "OUTGOING" | string;
  status: "PENDING" | "ACCEPTED" | "REJECTED" | string;
  remark: string;
  group_id?: number | null;
  created_at: number;
  updated_at: number;
};

export type FriendRequestResponse = {
  request: FriendRequestInfo;
  friend?: FriendInfo | null;
};

export type ConversationInfo = {
  conversationId: string;
  type: "SINGLE" | "GROUP" | "BOT" | "SYSTEM" | string;
  title: string;
  avatar: string;
  lastMessageId?: number | null;
  lastMessageAt?: number | null;
  lastMessageSenderId?: number | null;
  lastMessageSenderName: string;
  lastMessageContent: string;
  muteAll?: boolean;
  role: string;
  isPinned: boolean;
  isMuted: boolean;
  updatedAt: number;
};

export type NotificationInfo = {
  id: number;
  type: string;
  category?: "GROUP_SYSTEM" | "USER_CENTER" | "SYSTEM" | string;
  title: string;
  summary?: string;
  content: string;
  detail?: string;
  conversationId: string;
  relatedMessageId?: number | null;
  isRead: boolean;
  createdAt: number;
  persistent?: boolean;
};

export type NotificationListResponse = {
  notifications: NotificationInfo[];
  unreadCount: number;
};

export type GroupInfo = {
  conversationId: string;
  type: string;
  name: string;
  avatar: string;
  announcement: string;
  announcementUpdatedBy?: number | null;
  announcementUpdatedAt?: number | null;
  ownerId: number;
  joinPolicy: string;
  createdAt: number;
};

export type JoinGroupResponse = {
  message: string;
  pending: boolean;
};

export type GroupJoinRequestInfo = {
  requestId: number;
  conversationId: string;
  applicantUserId: number;
  applicantName: string;
  applicantAvatar: string;
  reason: string;
  status: string;
  reviewedBy?: number | null;
  reviewedAt?: number | null;
  createdAt: number;
  updatedAt: number;
};

export type MessageType = "TEXT" | "IMAGE" | "FILE" | "VOICE" | "SYSTEM" | "BOT_REPLY" | string;

export type TextMessageContent = {
  text: string;
};

export type ImageMessageContent = {
  url: string;
  name: string;
  size?: number;
  mimeType: string;
  width?: number;
  height?: number;
  text?: string;
};

export type FileMessageContent = {
  url: string;
  name: string;
  size: number;
  mimeType: string;
};

export type VoiceMessageContent = {
  url: string;
  name: string;
  size?: number;
  mimeType: string;
  durationMs: number;
};

export type SystemMessageContent = {
  eventType?: string;
  actorUserId?: number;
  targetUserIds?: number[];
  text: string;
};

export type MemberInfo = {
  userId: number;
  nickname: string;
  avatar: string;
  role: string;
  status: string;
  joinedAt: number;
  memberType?: string;
  botId?: number;
  mentionName?: string;
  aliases?: string[];
  enabled?: boolean;
  permissionScope?: string;
  muteUntil?: number | null;
};

export type ReplyPreviewInfo = {
  messageId: number;
  senderId: number;
  senderType: string;
  messageType: MessageType;
  contentPreview: string;
};

export type MessageInfo = {
  id: number;
  conversationId: string;
  senderId: number;
  senderType: string;
  messageType: MessageType;
  content: string;
  replyToId?: number | null;
  replyTo?: ReplyPreviewInfo | null;
  status: string;
  createdAt: number;
  readByPeer?: boolean;
  readCount?: number;
  pending?: boolean;
  clientMsgId?: string;
  isBotGenerating?: boolean;
};

export type HistorySearchMessageItem = {
  conversationId: string;
  conversationType: string;
  conversationTitle: string;
  message: MessageInfo;
};

export type MessageRecalledEventInfo = {
  messageId: number;
  conversationId: string;
};

export type TypingEventData = {
  conversationId: string;
  isTyping: boolean;
  userId: number;
  at?: number;
};

export type BotReplyStreamData = {
  conversationId: string;
  senderId: number;
  senderType: string;
  messageType: MessageType;
  content: string;
  done: boolean;
};

export type OutgoingMessagePayload = {
  messageType: "TEXT" | "IMAGE" | "FILE" | "VOICE";
  contentPayload: Record<string, unknown>;
};

export type WebSocketEvent =
  | {
      type: "CONNECTED";
      data: { userId: number };
    }
  | {
      type: "MESSAGE_ACK";
      clientMsgId?: string;
      data: {
        messageId?: number;
        status: "SUCCESS" | "FAILED";
        errorCode?: string;
        errorMessage?: string;
      };
    }
  | {
      type: "NEW_MESSAGE";
      data: MessageInfo;
    }
  | {
      type: "MESSAGE_RECALLED";
      data: MessageRecalledEventInfo;
    }
  | {
      type: "TYPING";
      data: TypingEventData;
    }
  | {
      type: "BOT_REPLY_STREAM";
      data: BotReplyStreamData;
    }
  | {
      type: "NOTIFICATION_CREATED";
      data: {
        notification: NotificationInfo;
        unreadCount?: number;
      };
    }
  | {
      type: "FRIEND_SYNC";
      data: {
        reason: "REQUEST_CREATED" | "REQUEST_RESPONDED" | "PRESENCE_CHANGED" | string;
        requestId?: number;
        status?: "PENDING" | "ACCEPTED" | "REJECTED" | "ONLINE" | "OFFLINE" | string;
        actorUserId?: number;
        friendUserId?: number;
        conversationId?: string;
      };
    }
  | {
      type: string;
      clientMsgId?: string;
      data?: unknown;
    };

export type MobilePane = "conversations" | "chat" | "friends" | "members" | "knowledge" | "account" | "bots";

export type BotInfo = {
  botId: number;
  memberType: string;
  memberId: number;
  name: string;
  displayName: string;
  mentionName: string;
  aliases: string[];
  avatar: string;
  description: string;
  enabled: boolean;
  permissionScope: string;
  memberStatus: string;
  modelName: string;
  supportedModels: string[];
};

export type AICallLogInfo = {
  id: number;
  conversationId: string;
  userId: number;
  botId: number;
  botName: string;
  requestMessageId?: number | null;
  responseMessageId?: number | null;
  modelName: string;
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
  latencyMs: number;
  status: string;
  errorMessage: string;
  createdAt: number;
};

export type AICallLogQuotaInfo = {
  dailyTotalTokens: number;
  dailyTokenLimit: number;
  remainingTokens: number;
};

export type AICallLogListResponse = {
  logs: AICallLogInfo[];
  quota: AICallLogQuotaInfo;
};

export type KnowledgeBaseInfo = {
  knowledgeBaseId: number;
  name: string;
  description: string;
  status: string;
};

export type KnowledgeDocumentInfo = {
  documentId: number;
  knowledgeBaseId: number;
  title: string;
  sourceType: string;
  status: string;
  errorMessage: string;
  createdAt: number;
};

export type KnowledgeSearchChunkInfo = {
  chunkId: number;
  documentId: number;
  score: number;
  content: string;
};

export type QueryRouteConstraints = {
  must_ground_in_sources: boolean;
  allow_external_web: boolean;
  strict_quote_required: boolean;
};

export type QueryRoutePlanInfo = {
  plan_version: string;
  family: string;
  source_space: string;
  scope: string;
  read_depth: string;
  output_mode: string;
  evidence_mode: string;
  targets: string[];
  constraints: QueryRouteConstraints;
  confidence: number;
  fallback_family: string;
  reason: string;
};

export type KnowledgeBaseQueryCitationInfo = {
  index: number;
  chunkId: number;
  documentId: number;
  documentTitle: string;
  score: number;
  excerpt: string;
};

export type KnowledgeBaseQueryQuoteInfo = {
  quoteId: string;
  documentId: number;
  documentTitle: string;
  chunkId: number;
  sentenceIndex: number;
  pageStart: number;
  pageEnd: number;
  charStart: number;
  charEnd: number;
  text: string;
};

export type KnowledgeBaseQueryResponse = {
  status: string;
  answer: string;
  model?: string;
  plan: QueryRoutePlanInfo;
  citations: KnowledgeBaseQueryCitationInfo[];
  quotes: KnowledgeBaseQueryQuoteInfo[];
  chunks: KnowledgeSearchChunkInfo[];
};

export type ConversationKnowledgeBaseInfo = {
  id: number;
  conversationId: string;
  knowledgeBaseId: number;
  name: string;
  description: string;
  status: string;
  enabled: boolean;
};

export type UserMemoryInfo = {
  id: number;
  userId: number;
  content: string;
  sourceConversationId: string;
  sourceMessageId?: number | null;
  lastUsedAt: number;
  createdAt: number;
  updatedAt: number;
};

export type UserMemorySettingInfo = {
  enabled: boolean;
  scope: "ALL_GROUPS" | "SELECTED_GROUPS" | string;
  conversationIds: string[];
  updatedAt: number;
};

export type ConversationSummaryResponse = {
  summary: string;
  messageCountUsed: number;
  usedCount: number;
  remainingCount: number;
  model: string;
};
