import { api } from "../../api";
import type { ConversationInfo, FriendInfo } from "../../types";
import type { DetailTab } from "../types";
import { errorMessage, sortFriends } from "../utils";

export type FriendDomainDeps = {
  setBusyAction: (value: boolean) => void;
  refreshFriends: () => Promise<{ groups: unknown[]; friends: FriendInfo[]; requests: unknown[] }>;
  refreshConversations: () => Promise<ConversationInfo[]>;
  conversations: ConversationInfo[];
  setSelectedConversationId: (value: string | null) => void;
  setMobilePane: (value: "conversations" | "chat" | "friends" | "members" | "account" | "bots") => void;
  setDetailTab: (value: DetailTab) => void;
  setFriends: (updater: (current: FriendInfo[]) => FriendInfo[]) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
};

export async function createFriendGroupAction(name: string, deps: FriendDomainDeps) {
  deps.setBusyAction(true);
  try {
    const group = await api.createFriendGroup(name);
    await deps.refreshFriends();
    deps.setDetailTab("friends");
    deps.setMobilePane("friends");
    deps.showToast(`好友分组已创建：${group.name}`, "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function addFriendAction(
  input: { targetAimId: string; remark: string; groupId: number | null },
  deps: FriendDomainDeps
) {
  deps.setBusyAction(true);
  try {
    const request = await api.addFriend({
      target_aim_id: input.targetAimId,
      remark: input.remark,
      group_id: input.groupId
    });
    await deps.refreshFriends();
    deps.setDetailTab("friends");
    deps.setMobilePane("friends");
    deps.showToast(`好友申请已发送给${request.nickname || request.aim_id}`, "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function respondFriendRequestAction(
  requestId: number,
  action: "ACCEPTED" | "REJECTED",
  deps: FriendDomainDeps
) {
  deps.setBusyAction(true);
  try {
    const existingConversationIds = new Set(deps.conversations.map((item) => item.conversationId));
    const response = await api.respondFriendRequest(requestId, action);
    await deps.refreshFriends();

    if (action === "ACCEPTED") {
      const nextConversations = await deps.refreshConversations();
      const singleConversation = nextConversations.find(
        (item) => item.type === "SINGLE" && !existingConversationIds.has(item.conversationId)
      );
      if (singleConversation) {
        deps.setSelectedConversationId(singleConversation.conversationId);
        deps.setMobilePane("chat");
      } else {
        deps.setDetailTab("friends");
        deps.setMobilePane("friends");
      }
      deps.showToast(`Accepted ${response.friend?.nickname || response.request.nickname}'s friend request`, "success");
      return;
    }

    deps.setDetailTab("friends");
    deps.setMobilePane("friends");
    deps.showToast(`Rejected ${response.request.nickname}'s friend request`, "info");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function updateFriendAction(
  friendUserId: number,
  input: { remark: string; groupId: number | null },
  deps: FriendDomainDeps
) {
  deps.setBusyAction(true);
  try {
    const updated = await api.updateFriend(friendUserId, {
      remark: input.remark,
      group_id: input.groupId
    });
    deps.setFriends((current) => sortFriends(current.map((item) => (item.user_id === friendUserId ? updated : item))));
    deps.showToast("Friend updated", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function deleteFriendAction(friendUserId: number, deps: FriendDomainDeps) {
  deps.setBusyAction(true);
  try {
    await api.deleteFriend(friendUserId);
    deps.setFriends((current) => current.filter((item) => item.user_id !== friendUserId));
    deps.showToast("Friend deleted", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}
