import { api } from "../../api";
import type { AICallLogInfo, AICallLogQuotaInfo, BotInfo } from "../../types";
import { errorMessage } from "../utils";

export type BotDomainDeps = {
  selectedConversationId: string | null;
  aiCallLogStatus: "" | "SUCCESS" | "FAILED";
  setBusyAction: (value: boolean) => void;
  setAvailableBots: (value: BotInfo[]) => void;
  setCustomBots: (value: BotInfo[]) => void;
  setConversationBots: (value: BotInfo[]) => void;
  setAICallLogs: (value: AICallLogInfo[]) => void;
  setAICallLogQuota: (value: AICallLogQuotaInfo) => void;
  setLoadingAICallLogs: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
};

export type CreateCustomBotInput = {
  name: string;
  mentionName: string;
  aliases?: string[];
  description?: string;
  apiBaseUrl: string;
  apiKey: string;
  modelName: string;
  supportedModels?: string[];
  systemPrompt?: string;
};

export type UpdateCustomBotInput = {
  botId: number;
  name: string;
  mentionName: string;
  aliases?: string[];
  description?: string;
  apiBaseUrl?: string;
  apiKey?: string;
  modelName: string;
  supportedModels?: string[];
  systemPrompt?: string;
};

export async function refreshAvailableBotsAction(deps: BotDomainDeps) {
  const data = await api.bots();
  deps.setAvailableBots(data);
}

export async function refreshCustomBotsAction(deps: BotDomainDeps) {
  const data = await api.customBots();
  deps.setCustomBots(data);
}

export async function refreshConversationBotsAction(deps: BotDomainDeps) {
  if (!deps.selectedConversationId) return;
  const data = await api.conversationBots(deps.selectedConversationId);
  deps.setConversationBots(data);
}

export async function refreshAICallLogsAction(deps: BotDomainDeps) {
  if (!deps.selectedConversationId) {
    deps.setAICallLogs([]);
    deps.setAICallLogQuota({ dailyTotalTokens: 0, dailyTokenLimit: 50_000, remainingTokens: 50_000 });
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
    deps.showToast("Bot 已添加", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}

export async function createCustomBotAction(input: CreateCustomBotInput, deps: BotDomainDeps) {
  deps.setBusyAction(true);
  try {
    await api.createCustomBot(input);
    await Promise.all([refreshAvailableBotsAction(deps), refreshCustomBotsAction(deps)]);
    deps.showToast("自定义 Bot 已创建", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
    throw error;
  } finally {
    deps.setBusyAction(false);
  }
}

export async function updateCustomBotAction(input: UpdateCustomBotInput, deps: BotDomainDeps) {
  deps.setBusyAction(true);
  try {
    await api.updateCustomBot(input.botId, {
      name: input.name,
      mentionName: input.mentionName,
      aliases: input.aliases ?? [],
      description: input.description ?? "",
      apiBaseUrl: input.apiBaseUrl,
      apiKey: input.apiKey,
      modelName: input.modelName,
      supportedModels: input.supportedModels ?? [],
      systemPrompt: input.systemPrompt
    });
    await Promise.all([
      refreshAvailableBotsAction(deps),
      refreshCustomBotsAction(deps),
      refreshConversationBotsAction(deps)
    ]);
    deps.showToast("自定义 Bot 已更新", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
    throw error;
  } finally {
    deps.setBusyAction(false);
  }
}

export async function deleteCustomBotAction(botId: number, deps: BotDomainDeps) {
  deps.setBusyAction(true);
  try {
    await api.deleteCustomBot(botId);
    await Promise.all([
      refreshAvailableBotsAction(deps),
      refreshCustomBotsAction(deps),
      refreshConversationBotsAction(deps)
    ]);
    deps.showToast("自定义 Bot 已删除", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
    throw error;
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
    deps.showToast("Bot 已移除", "success");
  } catch (error) {
    deps.showToast(errorMessage(error), "error");
  } finally {
    deps.setBusyAction(false);
  }
}
