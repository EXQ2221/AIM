import { api } from "../../api";
import type { UserInfo } from "../../types";
import { errorMessage } from "../utils";

export type AuthDomainDeps = {
  setBusyAction: (value: boolean) => void;
  refreshSessions: () => Promise<void>;
  handleLogout: () => Promise<void>;
  setUser: (user: UserInfo) => void;
  setMembers: (updater: (current: any[]) => any[]) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
};

export async function revokeSessionAction(sessionId: string, password: string, deps: AuthDomainDeps) {
  if (!password.trim()) {
    deps.showToast("Please enter your password", "error");
    return;
  }
  deps.setBusyAction(true);
  try {
    await api.revokeSession(sessionId, password);
    await deps.refreshSessions();
    deps.showToast("会话已注销", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function logoutAllAction(password: string, deps: AuthDomainDeps) {
  if (!password.trim()) {
    deps.showToast("Please enter your password", "error");
    return;
  }
  deps.setBusyAction(true);
  try {
    await api.logoutAll(password);
    await deps.handleLogout();
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
    deps.setBusyAction(false);
  }
}

export async function uploadAvatarAction(avatar: Blob, deps: AuthDomainDeps) {
  deps.setBusyAction(true);
  try {
    const response = await api.uploadAvatar(avatar);
    deps.setUser(response.user);
    deps.setMembers((current) =>
      current.map((member: any) =>
        member.userId === response.user.user_id
          ? { ...member, avatar: response.user.avatar, nickname: response.user.nickname }
          : member
      )
    );
    deps.showToast("Avatar updated", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}
