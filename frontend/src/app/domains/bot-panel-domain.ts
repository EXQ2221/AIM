import { useCallback, useEffect } from "react";
import type { AICallLogInfo, AICallLogQuotaInfo, BotInfo } from "../../types";
import type { DetailTab } from "../types";
import { errorMessage } from "../utils";
import {
  addBotToConversationAction,
  createCustomBotAction,
  deleteCustomBotAction,
  type BotDomainDeps,
  type CreateCustomBotInput,
  type UpdateCustomBotInput,
  refreshAICallLogsAction,
  refreshAvailableBotsAction,
  refreshCustomBotsAction,
  refreshConversationBotsAction,
  removeBotFromConversationAction,
  updateCustomBotAction
} from "./bot-domain";

type UseBotPanelDomainDeps = {
  detailTab: DetailTab;
  selectedConversationId: string | null;
  selectedConversationType: string | null;
  botDomainDeps: BotDomainDeps;
  setAvailableBots: (value: BotInfo[]) => void;
  setCustomBots: (value: BotInfo[]) => void;
  setConversationBots: (value: BotInfo[]) => void;
  setAICallLogs: (value: AICallLogInfo[]) => void;
  setAICallLogQuota: (value: AICallLogQuotaInfo) => void;
  setLoadingAICallLogs: (value: boolean) => void;
  showToast: (message: string, tone?: "success" | "error" | "info") => void;
};

export function useBotPanelDomain(deps: UseBotPanelDomainDeps) {
  const {
    detailTab,
    selectedConversationId,
    selectedConversationType,
    botDomainDeps,
    setAvailableBots,
    setCustomBots,
    setConversationBots,
    setAICallLogs,
    setAICallLogQuota,
    setLoadingAICallLogs,
    showToast
  } = deps;

  const refreshAvailableBots = useCallback(
    async () => refreshAvailableBotsAction(botDomainDeps),
    [botDomainDeps]
  );

  const refreshConversationBots = useCallback(
    async () => refreshConversationBotsAction(botDomainDeps),
    [botDomainDeps]
  );

  const refreshCustomBots = useCallback(
    async () => refreshCustomBotsAction(botDomainDeps),
    [botDomainDeps]
  );

  const refreshAICallLogs = useCallback(
    async () => refreshAICallLogsAction(botDomainDeps),
    [botDomainDeps]
  );

  const handleAddBot = useCallback(
    async (input: {
      botId: number;
      displayNameOverride?: string;
      mentionNameOverride?: string;
      aliasesOverride?: string[];
      permissionScope?: string;
      modelNameOverride?: string;
    }) => addBotToConversationAction(input, botDomainDeps),
    [botDomainDeps]
  );

  const handleRemoveBot = useCallback(
    async (botId: number) => removeBotFromConversationAction(botId, botDomainDeps),
    [botDomainDeps]
  );

  const handleCreateCustomBot = useCallback(
    async (input: CreateCustomBotInput) => createCustomBotAction(input, botDomainDeps),
    [botDomainDeps]
  );

  const handleUpdateCustomBot = useCallback(
    async (input: UpdateCustomBotInput) => updateCustomBotAction(input, botDomainDeps),
    [botDomainDeps]
  );

  const handleDeleteCustomBot = useCallback(
    async (botId: number) => deleteCustomBotAction(botId, botDomainDeps),
    [botDomainDeps]
  );

  useEffect(() => {
    if (detailTab === "bots" && selectedConversationId && selectedConversationType === "GROUP") {
      void (async () => {
        try {
          await Promise.all([refreshAvailableBots(), refreshConversationBots(), refreshCustomBots()]);
        } catch (error) {
          showToast(errorMessage(error), "error");
        }
      })();
      return;
    }
    if (detailTab === "bots") {
      setAvailableBots([]);
      setCustomBots([]);
      setConversationBots([]);
    }
  }, [detailTab, refreshAvailableBots, refreshConversationBots, refreshCustomBots, selectedConversationId, selectedConversationType, setAvailableBots, setCustomBots, setConversationBots, showToast]);

  useEffect(() => {
    if (detailTab === "logs" && selectedConversationId) {
      void (async () => {
        try {
          await refreshAICallLogs();
        } catch (error) {
          showToast(errorMessage(error), "error");
        }
      })();
      return;
    }
    if (detailTab !== "logs") {
      setLoadingAICallLogs(false);
    }
  }, [detailTab, refreshAICallLogs, selectedConversationId, setLoadingAICallLogs, showToast]);

  useEffect(() => {
    if (!selectedConversationId) {
      setAICallLogs([]);
      setAICallLogQuota({
        dailyTotalTokens: 0,
        dailyTokenLimit: 50_000,
        remainingTokens: 50_000
      });
    }
  }, [selectedConversationId, setAICallLogs, setAICallLogQuota]);

  return {
    refreshAICallLogs,
    handleAddBot,
    handleRemoveBot,
    handleCreateCustomBot,
    handleUpdateCustomBot,
    handleDeleteCustomBot
  };
}
