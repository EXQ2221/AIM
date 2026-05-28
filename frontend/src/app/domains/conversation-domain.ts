import { api } from "../../api";
import type { ConversationInfo, MemberInfo } from "../../types";
import { errorMessage } from "../utils";

export type ConversationDomainDeps = {
  selectedConversationId: string | null;
  selectedConversationType: string | null;
  selectedConversationIdRef: { current: string | null };
  setBusyAction: (value: boolean) => void;
  refreshConversations: () => Promise<ConversationInfo[]>;
  refreshCurrentConversationMessages: () => Promise<void>;
  refreshSelectedGroupInfo: (conversationId: string) => Promise<unknown>;
  setSelectedConversationId: (value: string | null) => void;
  setMobilePane: (value: "conversations" | "chat" | "friends" | "members" | "account" | "bots") => void;
  setMembers: (value: MemberInfo[]) => void;
  setSelectedGroupInfo: (value: any) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
};

export async function createGroupAction(
  input: { name: string; announcement: string; joinPolicy: string },
  deps: ConversationDomainDeps
) {
  deps.setBusyAction(true);
  try {
    const group = await api.createGroup({ name: input.name, avatar: "", announcement: input.announcement, joinPolicy: input.joinPolicy });
    await deps.refreshConversations();
    deps.setSelectedConversationId(group.conversationId);
    deps.setMobilePane("chat");
    deps.showToast(`会话已创建：conversationId: ${group.conversationId}`, "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function joinGroupAction(conversationId: string, deps: ConversationDomainDeps) {
  deps.setBusyAction(true);
  try {
    const result = await api.joinGroup(conversationId);
    await deps.refreshConversations();
    if (result.pending) {
      deps.showToast(result.message || "已提交入群申请，等待管理员审核", "info");
    } else {
      deps.setSelectedConversationId(conversationId);
      deps.setMobilePane("chat");
      deps.showToast(result.message || "已加入群聊", "success");
    }
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function leaveGroupAction(deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.leaveGroup(deps.selectedConversationId);
    await deps.refreshConversations();
    deps.showToast("Left group", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function inviteMemberAction(targetUserId: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  await api.inviteMember(deps.selectedConversationId, targetUserId);
  const nextMembers = await api.members(deps.selectedConversationId);
  deps.setMembers(nextMembers);
}

export async function refreshSelectedConversationStateAction(deps: ConversationDomainDeps) {
  const conversationId = deps.selectedConversationIdRef.current;
  if (!conversationId) return;
  const shouldRefreshGroupInfo = deps.selectedConversationType === "GROUP";
  const [nextMembers, nextGroupInfo] = await Promise.all([
    api.members(conversationId),
    shouldRefreshGroupInfo ? api.groupInfo(conversationId) : Promise.resolve(null),
    deps.refreshConversations()
  ]);
  if (deps.selectedConversationIdRef.current === conversationId) {
    deps.setMembers(nextMembers);
    deps.setSelectedGroupInfo(nextGroupInfo);
    await deps.refreshCurrentConversationMessages();
  }
}

export async function transferOwnerAction(targetUserId: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.transferOwner(deps.selectedConversationId, targetUserId);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast("Ownership transferred", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function setAdminAction(targetUserId: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.setAdmin(deps.selectedConversationId, targetUserId);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast("管理员已设置", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function removeAdminAction(targetUserId: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.removeAdmin(deps.selectedConversationId, targetUserId);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast("管理员已取消", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function muteMemberAction(targetUserId: number, durationSeconds: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId || durationSeconds <= 0) return;
  deps.setBusyAction(true);
  try {
    const muteUntil = Math.floor(Date.now() / 1000) + durationSeconds;
    await api.muteMember(deps.selectedConversationId, targetUserId, muteUntil);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast("成员已禁言", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function unmuteMemberAction(targetUserId: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.unmuteMember(deps.selectedConversationId, targetUserId);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast("已解除禁言", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function removeMemberAction(targetUserId: number, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.removeMember(deps.selectedConversationId, targetUserId);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast("Member removed from group", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function setGroupMuteAllAction(muteAll: boolean, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.setGroupMuteAll(deps.selectedConversationId, muteAll);
    await refreshSelectedConversationStateAction(deps);
    deps.showToast(muteAll ? "已开启全员禁言" : "已关闭全员禁言", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function updateGroupAnnouncementAction(announcement: string, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.updateGroupAnnouncement(deps.selectedConversationId, announcement);
    await Promise.all([
      deps.refreshSelectedGroupInfo(deps.selectedConversationId),
      refreshSelectedConversationStateAction(deps)
    ]);
    deps.showToast("Group announcement updated", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function updateGroupAvatarAction(avatar: string, deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.updateGroupAvatar(deps.selectedConversationId, avatar);
    await Promise.all([
      deps.refreshSelectedGroupInfo(deps.selectedConversationId),
      refreshSelectedConversationStateAction(deps)
    ]);
    deps.showToast("群头像已更新", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function disbandGroupAction(deps: ConversationDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.disbandGroup(deps.selectedConversationId);
    const removedConversationID = deps.selectedConversationId;
    const nextConversations = await deps.refreshConversations();
    if (deps.selectedConversationIdRef.current === removedConversationID) {
      const nextSelected = nextConversations[0]?.conversationId ?? null;
      deps.setSelectedConversationId(nextSelected);
      if (!nextSelected) {
        deps.setMobilePane("conversations");
      }
    }
    deps.showToast("群聊已解散", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}
