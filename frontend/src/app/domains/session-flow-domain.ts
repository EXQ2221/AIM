import { useCallback, type Dispatch, type RefObject, type SetStateAction } from "react";
import { api } from "../../api";
import type { ConversationInfo, FriendGroupInfo, FriendInfo, FriendRequestInfo, MemberInfo, MessageInfo, SessionInfo, UserInfo } from "../../types";
import type { ToastTone } from "../types";
import { errorMessage } from "../utils";

type UseSessionFlowDeps = {
  socketRef: RefObject<WebSocket | null>;
  setBusyAction: Dispatch<SetStateAction<boolean>>;
  setBooting: Dispatch<SetStateAction<boolean>>;
  setUser: Dispatch<SetStateAction<UserInfo | null>>;
  setFriendGroups: Dispatch<SetStateAction<FriendGroupInfo[]>>;
  setFriends: Dispatch<SetStateAction<FriendInfo[]>>;
  setFriendRequests: Dispatch<SetStateAction<FriendRequestInfo[]>>;
  setMessages: Dispatch<SetStateAction<MessageInfo[]>>;
  setMembers: Dispatch<SetStateAction<MemberInfo[]>>;
  setConversations: Dispatch<SetStateAction<ConversationInfo[]>>;
  setUnreadCounts: Dispatch<SetStateAction<Record<string, number>>>;
  setSelectedConversationId: Dispatch<SetStateAction<string | null>>;
  setSessions: Dispatch<SetStateAction<SessionInfo[]>>;
  refreshConversations: () => Promise<ConversationInfo[]>;
  refreshFriends: () => Promise<{ groups: unknown[]; friends: FriendInfo[]; requests: unknown[] }>;
  refreshSessions: () => Promise<void>;
  showToast: (message: string, tone?: ToastTone) => void;
};

export function useSessionFlow(deps: UseSessionFlowDeps) {
  const {
    socketRef,
    setBusyAction,
    setBooting,
    setUser,
    setFriendGroups,
    setFriends,
    setFriendRequests,
    setMessages,
    setMembers,
    setConversations,
    setUnreadCounts,
    setSelectedConversationId,
    setSessions,
    refreshConversations,
    refreshFriends,
    refreshSessions,
    showToast
  } = deps;

  const bootstrap = useCallback(async () => {
    try {
      const me = await api.me();
      setUser(me);
      await Promise.all([refreshConversations(), refreshFriends(), refreshSessions()]);
    } catch {
      setUser(null);
      setFriendGroups([]);
      setFriends([]);
      setFriendRequests([]);
      setConversations([]);
      setUnreadCounts({});
      setSessions([]);
      setSelectedConversationId(null);
    } finally {
      setBooting(false);
    }
  }, [
    refreshConversations,
    refreshFriends,
    refreshSessions,
    setBooting,
    setConversations,
    setFriendGroups,
    setFriendRequests,
    setFriends,
    setSelectedConversationId,
    setSessions,
    setUnreadCounts,
    setUser
  ]);

  const handleLogin = useCallback(async (input: { email: string; password: string }) => {
    setBusyAction(true);
    try {
      await api.login({
        email: input.email,
        password: input.password,
        device_name: "AIM Web"
      });
      await bootstrap();
      showToast("登录成功", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  }, [bootstrap, setBusyAction, showToast]);

  const handleRegister = useCallback(async (input: { aim_id: string; email: string; nickname: string; password: string }) => {
    setBusyAction(true);
    try {
      await api.register(input);
      showToast("注册完成，可以登录了", "success");
    } catch (error) {
      showToast(errorMessage(error), "error");
    } finally {
      setBusyAction(false);
    }
  }, [setBusyAction, showToast]);

  const handleLogout = useCallback(async () => {
    setBusyAction(true);
    try {
      await api.logout();
    } catch {
      // Local cleanup still applies when the session has already expired.
    } finally {
      socketRef.current?.close();
      setUser(null);
      setFriendGroups([]);
      setFriends([]);
      setFriendRequests([]);
      setMessages([]);
      setMembers([]);
      setConversations([]);
      setUnreadCounts({});
      setSelectedConversationId(null);
      setBusyAction(false);
    }
  }, [
    setBusyAction,
    setConversations,
    setFriendGroups,
    setFriendRequests,
    setFriends,
    setMembers,
    setMessages,
    setSelectedConversationId,
    setUnreadCounts,
    setUser,
    socketRef
  ]);

  return {
    bootstrap,
    handleLogin,
    handleRegister,
    handleLogout
  };
}
