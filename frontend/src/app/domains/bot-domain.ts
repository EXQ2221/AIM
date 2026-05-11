import { api } from "../../api";
import type { AICallLogInfo, AICallLogQuotaInfo, BotInfo } from "../../types";
import { errorMessage } from "../utils";

export type BotDomainDeps = {
  selectedConversationId: string | null;
  aiCallLogStatus: "" | "SUCCESS" | "FAILED";
  setBusyAction: (value: boolean) => void;
  setAvailableBots: (value: BotInfo[]) => void;
  setConversationBots: (value: BotInfo[]) => void;
  setAICallLogs: (value: AICallLogInfo[]) => void;
  setAICallLogQuota: (value: AICallLogQuotaInfo) => void;
  setLoadingAICallLogs: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
};

export async function refreshAvailableBotsAction(deps: BotDomainDeps) {
  const data = await api.bots();
  deps.setAvailableBots(data);
}

export async function refreshConversationBotsAction(deps: BotDomainDeps) {
  if (!deps.selectedConversationId) return;
  const data = await api.conversationBots(deps.selectedConversationId);
  deps.setConversationBots(data);
}

export async function refreshAICallLogsAction(deps: BotDomainDeps) {
  if (!deps.selectedConversationId) {
    deps.setAICallLogs([]);
    deps.setAICallLogQuota({ dailyTotalTokens: 0, dailyTokenLimit: 1_000_000, remainingTokens: 1_000_000 });
    return;
  }
  deps.setLoadingAICallLogs(true);
  try {
    const data = await api.aiCallLogs(deps.selectedConversationId, {
      limit: 50,
      status: deps.aiCallLogStatus || undefined
    });
    deps.setAICallLogs(data.logs);
    deps.setAICallLogQuota(data.quota);
  } finally {
    deps.setLoadingAICallLogs(false);
  }
}

export async function addBotToConversationAction(
  input: {
    botId: number;
    displayNameOverride?: string;
    mentionNameOverride?: string;
    aliasesOverride?: string[];
    permissionScope?: string;
    modelNameOverride?: string;
  },
  deps: BotDomainDeps
) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.addConversationBot(deps.selectedConversationId, input);
    await refreshConversationBotsAction(deps);
    deps.showToast("Bot added", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function removeBotFromConversationAction(botId: number, deps: BotDomainDeps) {
  if (!deps.selectedConversationId) return;
  deps.setBusyAction(true);
  try {
    await api.removeConversationBot(deps.selectedConversationId, botId);
    await refreshConversationBotsAction(deps);
    deps.showToast("Bot removed", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}
