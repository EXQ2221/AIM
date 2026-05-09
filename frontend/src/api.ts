import type {
  AICallLogListResponse,
  APIResponse,
  AuthSessionResponse,
  BotInfo,
  ConversationInfo,
  FriendGroupInfo,
  FriendInfo,
  FriendRequestInfo,
  FriendRequestResponse,
  GroupInfo,
  MemberInfo,
  MessageInfo,
  SessionInfo,
  UploadAvatarResponse,
  UserInfo
} from "./types";

type RequestOptions = RequestInit & {
  retryOnUnauthorized?: boolean;
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
  const { retryOnUnauthorized = true, headers, body, ...rest } = options;
  const response = await fetch(path, {
    credentials: "include",
    headers: body instanceof FormData ? headers : { ...JSON_HEADERS, ...headers },
    body,
    ...rest
  });

  if (response.status === 401 && retryOnUnauthorized && path !== "/api/v1/auth/refresh") {
    const refreshed = await refreshSession();
    if (refreshed) {
      return request<T>(path, { ...options, retryOnUnauthorized: false });
    }
  }

  const payload = (await response.json().catch(() => null)) as APIResponse<T> | null;
  if (!response.ok || !payload || payload.code !== 0) {
    throw new APIError(payload?.message || response.statusText || "request failed", response.status, payload?.code);
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
  uploadAvatar(file: Blob) {
    const body = new FormData();
    body.append("file", file, "avatar.png");
    return request<UploadAvatarResponse>("/api/v1/users/me/avatar", {
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
  conversations() {
    return request<ConversationInfo[]>("/api/v1/conversations");
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
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members`, {
      method: "POST"
    });
  },
  inviteMember(conversationId: string, targetUserId: number) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members/invite`, {
      method: "POST",
      body: JSON.stringify({ targetUserId })
    });
  },
  leaveGroup(conversationId: string) {
    return request<void>(`/api/v1/conversations/${encodeURIComponent(conversationId)}/members/me`, {
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
  bots() {
    return request<BotInfo[]>("/api/v1/bots");
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
  }
};
