import type {
  AICallLogListResponse,
  APIResponse,
  AuthSessionResponse,
  BotInfo,
  ConversationKnowledgeBaseInfo,
  ConversationSummaryResponse,
  ConversationInfo,
  FriendGroupInfo,
  FriendInfo,
  FriendRequestInfo,
  FriendRequestResponse,
  GroupInfo,
  GroupJoinRequestInfo,
  KnowledgeBaseInfo,
  KnowledgeBaseQueryResponse,
  KnowledgeDocumentInfo,
  KnowledgeSearchChunkInfo,
  JoinGroupResponse,
  NotificationListResponse,
  MemberInfo,
  MessageInfo,
  PresenceSettings,
  HistorySearchMessageItem,
  SessionInfo,
  UploadAvatarResponse,
  UploadMediaResponse,
  UserMemoryInfo,
  UserMemorySettingInfo,
  UserInfo
} from "./types";

type RequestOptions = RequestInit & {
  retryOnUnauthorized?: boolean;
  timeoutMs?: number;
};

const JSON_HEADERS = {
  "Content-Type": "application/json"
};

let refreshInFlight: Promise<boolean> | null = null;

export class APIError extends Error {
  status: number;
  code: number;

  constructor(message: string, status: number, code = status) {
    super(message);
    this.name = "APIError";
    this.status = status;
    this.code = code;
  }
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { retryOnUnauthorized = true, headers, body, timeoutMs, ...rest } = options;
  const controller = new AbortController();
  const timeoutId =
    typeof timeoutMs === "number" && timeoutMs > 0 ? window.setTimeout(() => controller.abort(), timeoutMs) : null;
  let response: Response;
  try {
    response = await fetch(path, {
      credentials: "include",
      headers: body instanceof FormData ? headers : { ...JSON_HEADERS, ...headers },
      body,
      signal: controller.signal,
      ...rest
    });
  } catch (error) {
    const aborted = error instanceof DOMException && error.name === "AbortError";
    if (aborted) {
      throw new APIError("请求超时，请稍后重试或减小文件大小", 408, 408);
    }
    throw error;
  } finally {
    if (timeoutId !== null) {
      window.clearTimeout(timeoutId);
    }
  }

  if (response.status === 401 && retryOnUnauthorized && path !== "/api/v1/auth/refresh") {
    const refreshed = await refreshSession();
    if (refreshed) {
      return request<T>(path, { ...options, retryOnUnauthorized: false });
    }
  }

  const payload = (await response.json().catch(() => null)) as APIResponse<T> | null;
  if (!response.ok || !payload || payload.code !== 0) {
    throw new APIError(payload?.message || response.statusText || "请求失败", response.status, payload?.code);
  }

  return payload.data as T;
}

async function refreshSession(): Promise<boolean> {
  if (refreshInFlight) {
    return refreshInFlight;
  }

  refreshInFlight = (async () => {
    try {
      await request<AuthSessionResponse>("/api/v1/auth/refresh", {
        method: "POST",
        retryOnUnauthorized: false
      });
      return true;
    } catch {
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();

  return refreshInFlight;
}

export const api = {
  register(input: { aim_id: string; email: string; nickname: string; password: string }) {
    return request<UserInfo>("/api/v1/auth/register", {
      method: "POST",
      body: JSON.stringify(input),
      retryOnUnauthorized: false
    });
  },
  login(input: { email: string; password: string; device_name: string }) {
    return request<AuthSessionResponse>("/api/v1/auth/login", {
      method: "POST",
      body: JSON.stringify(input),
      retryOnUnauthorized: false
    });
  },
  logout() {
    return request<void>("/api/v1/auth/logout", { method: "POST" });
  },
  logoutAll(password: string) {
    return request<void>("/api/v1/auth/logout-all", {
      method: "POST",
      body: JSON.stringify({ password })
    });
  },
  revokeSession(session_id: string, password: string) {
    return request<void>("/api/v1/auth/sessions/revoke", {
      method: "POST",
      body: JSON.stringify({ session_id, password })
    });
  },
  sessions() {
    return request<SessionInfo[]>("/api/v1/auth/sessions");
  },
  me() {
    return request<UserInfo>("/api/v1/users/me");
  },
  listUserMemories(limit?: number) {
    const params = new URLSearchParams();
    if (typeof limit === "number" && limit > 0) {
      params.set("limit", String(limit));
    }
    const query = params.toString();
    return request<UserMemoryInfo[]>(`/api/v1/users/memory${query ? `?${query}` : ""}`);
  },
  writeUserMemory(content: string) {
    return request<UserMemoryInfo>("/api/v1/users/memory", {
      method: "POST",
      body: JSON.stringify({ content })
    });
  },
  updateUserMemory(memoryId: number, content: string) {
    return request<UserMemoryInfo>(`/api/v1/users/memory/${encodeURIComponent(String(memoryId))}`, {
      method: "PUT",
      body: JSON.stringify({ content })
    });
  },
  getUserMemorySetting() {
    return request<UserMemorySettingInfo>("/api/v1/users/memory/settings");
  },
  updateUserMemorySetting(input: {
    enabled?: boolean;
    scope?: "ALL_GROUPS" | "SELECTED_GROUPS" | string;
    conversationIds?: string[];
  }) {
    return request<UserMemorySettingInfo>("/api/v1/users/memory/settings", {
      method: "PUT",
      body: JSON.stringify(input)
    });
  },
  uploadAvatar(file: Blob) {
    const body = new FormData();
    body.append("file", file, "avatar.png");
    return request<UploadAvatarResponse>("/api/v1/users/me/avatar", {
      method: "POST",
      body
    });
  },
  uploadImage(file: File) {
    const body = new FormData();
    body.append("file", file, file.name);
    return request<UploadMediaResponse>("/api/v1/uploads/images", {
      method: "POST",
      body
    });
  },
  uploadFile(file: File) {
    const body = new FormData();
    body.append("file", file, file.name);
    return request<UploadMediaResponse>("/api/v1/uploads/files", {
      method: "POST",
      body
    });
  },
  uploadVoice(file: File) {
    const body = new FormData();
    body.append("file", file, file.name);
    return request<UploadMediaResponse>("/api/v1/uploads/voices", {
      method: "POST",
      body
    });
  },
  friendGroups() {
    return request<FriendGroupInfo[]>("/api/v1/friends/groups");
  },
  createFriendGroup(name: string) {
    return request<FriendGroupInfo>("/api/v1/friends/groups", {
      method: "POST",
      body: JSON.stringify({ name })
    });
  },
  friends() {
    return request<FriendInfo[]>("/api/v1/friends");
  },
  addFriend(input: { target_aim_id: string; remark: string; group_id?: number | null }) {
    return request<FriendRequestInfo>("/api/v1/friends", {
      method: "POST",
      body: JSON.stringify(input)
    });
  },
  friendRequests() {
    return request<FriendRequestInfo[]>("/api/v1/friends/requests");
  },
  respondFriendRequest(requestId: number, action: "ACCEPTED" | "REJECTED") {
    return request<FriendRequestResponse>(`/api/v1/friends/requests/${requestId}/respond`, {
      method: "POST",
      body: JSON.stringify({ action })
    });
  },
  updateFriend(friendUserId: number, input: { remark: string; group_id?: number | null }) {
    return request<FriendInfo>(`/api/v1/friends/${friendUserId}`, {
      method: "PATCH",
      body: JSON.stringify(input)
    });
  },
  deleteFriend(friendUserId: number) {
    return request<void>(`/api/v1/friends/${friendUserId}`, {
      method: "DELETE"
    });
  },
  getPresenceSettings() {
    return request<PresenceSettings>("/api/v1/friends/presence/settings");
  },
  updatePresenceSettings(invisible: boolean) {
    return request<PresenceSettings>("/api/v1/friends/presence/settings", {
      method: "PUT",
      body: JSON.stringify({ invisible })
    });
  },
  conversations() {
    return request<ConversationInfo[]>("/api/v1/conversations");
  },
  notifications(options: { unreadOnly?: boolean; limit?: number } = {}) {
    const params = new URLSearchParams();
    if (options.unreadOnly) {
      params.set("unreadOnly", "true");
    }
    if (options.limit && options.limit > 0) {
      params.set("limit", String(options.limit));
    }
    return request<NotificationListResponse>(`/api/v1/notifications?${params.toString()}`);
  },
  markNotificationRead(notificationId: number) {
    return request<void>(`/api/v1/notifications/${encodeURIComponent(String(notificationId))}/read`, {
      method: "POST"
    });
  },
  markAllNotificationsRead() {
    return request<void>("/api/v1/notifications/read-all", {
      method: "POST"
    });
  },
  findSingleConversation(targetUserId: number) {
    return request<ConversationInfo | null>(`/api/v1/conversations/single?targetUserId=${targetUserId}`);
  },
  createGroup(input: { name: string; avatar: string; announcement: string; joinPolicy: string }) {
    return request<GroupInfo>("/api/v1/conversations/group", {
      method: "POST",
      body: JSON.stringify(input)
    });
  },
  joinGroup(conversationId: string) {
    return request<JoinGroupResponse>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members`, {
      method: "POST"
    });
  },
  listGroupJoinRequests(conversationId: string, limit = 50) {
    const params = new URLSearchParams();
    if (limit > 0) {
      params.set("limit", String(limit));
    }
    return request<GroupJoinRequestInfo[]>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/join-requests${params.toString() ? `?${params.toString()}` : ""}`
    );
  },
  reviewGroupJoinRequest(conversationId: string, requestId: number, action: "APPROVE" | "REJECT") {
    return request<{ message: string }>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/join-requests/${encodeURIComponent(String(requestId))}/review`,
      {
        method: "POST",
        body: JSON.stringify({ action })
      }
    );
  },
  inviteMember(conversationId: string, targetUserId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members/invite`, {
      method: "POST",
      body: JSON.stringify({ targetUserId })
    });
  },
  transferOwner(conversationId: string, targetUserId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/owner/transfer`, {
      method: "POST",
      body: JSON.stringify({ targetUserId })
    });
  },
  setAdmin(conversationId: string, targetUserId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/admins`, {
      method: "POST",
      body: JSON.stringify({ targetUserId })
    });
  },
  removeAdmin(conversationId: string, targetUserId: number) {
    return request<void>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/admins/${encodeURIComponent(String(targetUserId))}`,
      {
        method: "DELETE"
      }
    );
  },
  muteMember(conversationId: string, targetUserId: number, muteUntil: number) {
    return request<void>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/members/${encodeURIComponent(String(targetUserId))}/mute`,
      {
        method: "POST",
        body: JSON.stringify({ muteUntil })
      }
    );
  },
  unmuteMember(conversationId: string, targetUserId: number) {
    return request<void>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/members/${encodeURIComponent(String(targetUserId))}/mute`,
      {
        method: "DELETE"
      }
    );
  },
  removeMember(conversationId: string, targetUserId: number) {
    return request<void>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/members/${encodeURIComponent(String(targetUserId))}`,
      {
        method: "DELETE"
      }
    );
  },
  setGroupMuteAll(conversationId: string, muteAll: boolean) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/mute-all`, {
      method: muteAll ? "POST" : "DELETE"
    });
  },
  leaveGroup(conversationId: string) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members/me`, {
      method: "DELETE"
    });
  },
  groupInfo(conversationId: string) {
    return request<GroupInfo>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/group`);
  },
  updateGroupAnnouncement(conversationId: string, announcement: string) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/announcement`, {
      method: "PUT",
      body: JSON.stringify({ announcement })
    });
  },
  updateGroupAvatar(conversationId: string, avatar: string) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/avatar`, {
      method: "PUT",
      body: JSON.stringify({ avatar })
    });
  },
  disbandGroup(conversationId: string) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/group`, {
      method: "DELETE"
    });
  },
  members(conversationId: string) {
    return request<MemberInfo[]>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members`);
  },
  messages(conversationId: string, options: { beforeId?: number; limit?: number } = {}) {
    const params = new URLSearchParams();
    if (options.beforeId) params.set("beforeId", String(options.beforeId));
    params.set("limit", String(options.limit ?? 30));
    return request<MessageInfo[]>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/messages?${params}`);
  },
  summarizeConversation(conversationId: string, messageCount: number) {
    return request<ConversationSummaryResponse>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/summary`, {
      method: "POST",
      body: JSON.stringify({ messageCount }),
      timeoutMs: 45000
    });
  },
  searchHistoryMessages(options: {
    conversationId?: string;
    startAt: number;
    endAt: number;
    keyword?: string;
    conversationType?: "ALL" | "GROUP" | "SINGLE";
  }) {
    const params = new URLSearchParams();
    if (options.conversationId && options.conversationId.trim()) {
      params.set("conversationId", options.conversationId.trim());
    }
    if (options.conversationType && options.conversationType !== "ALL") {
      params.set("conversationType", options.conversationType);
    }
    params.set("startAt", String(options.startAt));
    params.set("endAt", String(options.endAt));
    if (options.keyword && options.keyword.trim()) {
      params.set("keyword", options.keyword.trim());
    }
    return request<HistorySearchMessageItem[]>(`/api/v1/conversations/history/search?${params.toString()}`);
  },
  markConversationRead(conversationId: string, lastReadMessageId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/read`, {
      method: "POST",
      body: JSON.stringify({ lastReadMessageId })
    });
  },
  recallMessage(conversationId: string, messageId: number) {
    return request<void>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/messages/${encodeURIComponent(String(messageId))}/recall`,
      {
        method: "POST"
      }
    );
  },
  bots() {
    return request<BotInfo[]>("/api/v1/bots");
  },
  customBots() {
    return request<BotInfo[]>("/api/v1/bots/custom");
  },
  createCustomBot(input: {
    name: string;
    mentionName: string;
    aliases?: string[];
    description?: string;
    apiBaseUrl: string;
    apiKey: string;
    modelName: string;
    supportedModels?: string[];
    systemPrompt?: string;
  }) {
    return request<BotInfo>("/api/v1/bots", {
      method: "POST",
      body: JSON.stringify(input)
    });
  },
  updateCustomBot(
    botId: number,
    input: {
      name: string;
      mentionName: string;
      aliases?: string[];
      description?: string;
      apiBaseUrl?: string;
      apiKey?: string;
      modelName: string;
      supportedModels?: string[];
      systemPrompt?: string;
    }
  ) {
    return request<BotInfo>(`/api/v1/bots/${encodeURIComponent(String(botId))}`, {
      method: "PUT",
      body: JSON.stringify(input)
    });
  },
  deleteCustomBot(botId: number) {
    return request<void>(`/api/v1/bots/${encodeURIComponent(String(botId))}`, {
      method: "DELETE"
    });
  },
  conversationBots(conversationId: string) {
    return request<BotInfo[]>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/bots`);
  },
  addConversationBot(
    conversationId: string,
    input: {
      botId: number;
      displayNameOverride?: string;
      mentionNameOverride?: string;
      aliasesOverride?: string[];
      permissionScope?: string;
      modelNameOverride?: string;
    }
  ) {
    return request<BotInfo>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/bots`, {
      method: "POST",
      body: JSON.stringify(input)
    });
  },
  removeConversationBot(conversationId: string, botId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/bots/${botId}`, {
      method: "DELETE"
    });
  },
  aiCallLogs(
    conversationId: string,
    options: { beforeId?: number; limit?: number; botId?: number; status?: string } = {}
  ) {
    const params = new URLSearchParams();
    if (options.beforeId) params.set("beforeId", String(options.beforeId));
    if (options.limit) params.set("limit", String(options.limit));
    if (options.botId) params.set("botId", String(options.botId));
    if (options.status) params.set("status", options.status);
    return request<AICallLogListResponse>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/ai-call-logs?${params.toString()}`
    );
  },
  createKnowledgeBase(input: { name: string; description: string }) {
    return request<KnowledgeBaseInfo>("/api/v1/knowledge-bases", {
      method: "POST",
      body: JSON.stringify(input)
    });
  },
  listKnowledgeBases() {
    return request<KnowledgeBaseInfo[]>("/api/v1/knowledge-bases");
  },
  addKnowledgeDocumentText(
    knowledgeBaseId: number,
    input: { title: string; sourceType: "TEXT" | "MARKDOWN"; content: string }
  ) {
    return request<KnowledgeDocumentInfo>(
      `/api/v1/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}/documents/text`,
      {
        method: "POST",
        body: JSON.stringify(input)
      }
    );
  },
  addKnowledgeDocumentFile(knowledgeBaseId: number, input: { title?: string; file: File }) {
    const body = new FormData();
    body.append("file", input.file, input.file.name);
    if (input.title?.trim()) {
      body.append("title", input.title.trim());
    }
    return request<KnowledgeDocumentInfo>(
      `/api/v1/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}/documents/file`,
      {
        method: "POST",
        body,
        timeoutMs: 600000
      }
    );
  },
  listKnowledgeDocuments(knowledgeBaseId: number) {
    return request<KnowledgeDocumentInfo[]>(
      `/api/v1/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}/documents`
    );
  },
  deleteKnowledgeDocument(knowledgeBaseId: number, documentId: number) {
    return request<void>(
      `/api/v1/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}/documents/${encodeURIComponent(String(documentId))}`,
      {
        method: "DELETE"
      }
    );
  },
  searchKnowledgeBase(knowledgeBaseId: number, input: { query: string; topK?: number }) {
    return request<KnowledgeSearchChunkInfo[]>(
      `/api/v1/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}/search`,
      {
        method: "POST",
        body: JSON.stringify(input),
        timeoutMs: 20000
      }
    );
  },
  queryKnowledgeBase(
    knowledgeBaseId: number,
    input: { query: string; topK?: number; conversationId?: string; botId?: number }
  ) {
    return request<KnowledgeBaseQueryResponse>(
      `/api/v1/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}/query`,
      {
        method: "POST",
        body: JSON.stringify(input),
        timeoutMs: 30000
      }
    );
  },
  bindConversationKnowledgeBase(conversationId: string, knowledgeBaseId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/knowledge-bases`, {
      method: "POST",
      body: JSON.stringify({ knowledgeBaseId })
    });
  },
  listConversationKnowledgeBases(conversationId: string) {
    return request<ConversationKnowledgeBaseInfo[]>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/knowledge-bases`
    );
  },
  unbindConversationKnowledgeBase(conversationId: string, knowledgeBaseId: number) {
    return request<void>(
      `/api/v1/conversations/${encodeURIComponent(conversationId)}/knowledge-bases/${encodeURIComponent(String(knowledgeBaseId))}`,
      {
        method: "DELETE"
      }
    );
  }
};
