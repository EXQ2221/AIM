import type { Dispatch, MutableRefObject, SetStateAction } from "react";
import type {
  AICallLogInfo,
  AICallLogQuotaInfo,
  BotInfo,
  ConversationInfo,
  FriendInfo,
  MemberInfo,
  MobilePane,
  UserInfo
} from "../../types";
import type { DetailTab, ToastTone } from "../types";
import type { AuthDomainDeps } from "../domains/auth-domain";
import type { BotDomainDeps } from "../domains/bot-domain";
import type { ConversationDomainDeps } from "../domains/conversation-domain";
import type { FriendDomainDeps } from "../domains/friend-domain";

type CommonToast = (message: string, tone?: ToastTone) => void;

export function createConversationDomainDeps(params: {
  selectedConversationId: string | null;
  selectedConversationType: string | null;
  selectedConversationIdRef: MutableRefObject<string | null>;
  setBusyAction: (value: boolean) => void;
  refreshConversations: () => Promise<ConversationInfo[]>;
  refreshCurrentConversationMessages: () => Promise<void>;
  refreshSelectedGroupInfo: (conversationId: string) => Promise<unknown>;
  setSelectedConversationId: (value: string | null) => void;
  setMobilePane: (value: MobilePane) => void;
  setMembers: Dispatch<SetStateAction<MemberInfo[]>>;
  setSelectedGroupInfo: Dispatch<SetStateAction<any>>;
  showToast: CommonToast;
}): ConversationDomainDeps {
  return {
    selectedConversationId: params.selectedConversationId,
    selectedConversationType: params.selectedConversationType,
    selectedConversationIdRef: params.selectedConversationIdRef,
    setBusyAction: params.setBusyAction,
    refreshConversations: params.refreshConversations,
    refreshCurrentConversationMessages: params.refreshCurrentConversationMessages,
    refreshSelectedGroupInfo: params.refreshSelectedGroupInfo,
    setSelectedConversationId: params.setSelectedConversationId,
    setMobilePane: params.setMobilePane,
    setMembers: (value) => params.setMembers(value),
    setSelectedGroupInfo: (value) => params.setSelectedGroupInfo(value),
    showToast: params.showToast
  };
}

export function createAuthDomainDeps(params: {
  setBusyAction: (value: boolean) => void;
  refreshSessions: () => Promise<void>;
  handleLogout: () => Promise<void>;
  setUser: Dispatch<SetStateAction<UserInfo | null>>;
  setMembers: Dispatch<SetStateAction<MemberInfo[]>>;
  showToast: CommonToast;
}): AuthDomainDeps {
  return {
    setBusyAction: params.setBusyAction,
    refreshSessions: params.refreshSessions,
    handleLogout: params.handleLogout,
    setUser: (user) => params.setUser(user),
    setMembers: (updater) => params.setMembers((current) => updater(current)),
    showToast: params.showToast
  };
}

export function createFriendDomainDeps(params: {
  setBusyAction: (value: boolean) => void;
  refreshFriends: () => Promise<{ groups: unknown[]; friends: FriendInfo[]; requests: unknown[] }>;
  refreshConversations: () => Promise<ConversationInfo[]>;
  conversations: ConversationInfo[];
  setSelectedConversationId: (value: string | null) => void;
  setMobilePane: (value: MobilePane) => void;
  setDetailTab: (value: DetailTab) => void;
  setFriends: Dispatch<SetStateAction<FriendInfo[]>>;
  showToast: CommonToast;
}): FriendDomainDeps {
  return {
    setBusyAction: params.setBusyAction,
    refreshFriends: params.refreshFriends,
    refreshConversations: params.refreshConversations,
    conversations: params.conversations,
    setSelectedConversationId: params.setSelectedConversationId,
    setMobilePane: params.setMobilePane,
    setDetailTab: params.setDetailTab,
    setFriends: (updater) => params.setFriends((current) => updater(current)),
    showToast: params.showToast
  };
}

export function createBotDomainDeps(params: {
  selectedConversationId: string | null;
  aiCallLogStatus: "" | "SUCCESS" | "FAILED";
  setBusyAction: (value: boolean) => void;
  setAvailableBots: Dispatch<SetStateAction<BotInfo[]>>;
  setCustomBots: Dispatch<SetStateAction<BotInfo[]>>;
  setConversationBots: Dispatch<SetStateAction<BotInfo[]>>;
  setAICallLogs: Dispatch<SetStateAction<AICallLogInfo[]>>;
  setAICallLogQuota: Dispatch<SetStateAction<AICallLogQuotaInfo>>;
  setLoadingAICallLogs: Dispatch<SetStateAction<boolean>>;
  showToast: CommonToast;
}): BotDomainDeps {
  return {
    selectedConversationId: params.selectedConversationId,
    aiCallLogStatus: params.aiCallLogStatus,
    setBusyAction: params.setBusyAction,
    setAvailableBots: (value) => params.setAvailableBots(value),
    setCustomBots: (value) => params.setCustomBots(value),
    setConversationBots: (value) => params.setConversationBots(value),
    setAICallLogs: (value) => params.setAICallLogs(value),
    setAICallLogQuota: (value) => params.setAICallLogQuota(value),
    setLoadingAICallLogs: (value) => params.setLoadingAICallLogs(value),
    showToast: params.showToast
  };
}
