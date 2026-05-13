import { useCallback, useEffect } from "react";
import type { AICallLogInfo, AICallLogQuotaInfo, BotInfo } from "../../types";
import type { DetailTab } from "../types";
import { errorMessage } from "../utils";
import { addBotToConversationAction, type BotDomainDeps, refreshAICallLogsAction, refreshAvailableBotsAction, refreshConversationBotsAction, removeBotFromConversationAction } from "./bot-domain";

type UseBotPanelDomainDeps = {
  detailTab: DetailTab;
  selectedConversationId: string | null;
  selectedConversationType: string | null;
  botDomainDeps: BotDomainDeps;
  setAvailableBots: (value: BotInfo[]) => void;
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

  useEffect(() => {
    if (detailTab === "bots" && selectedConversationId && selectedConversationType === "GROUP") {
      void (async () => {
        try {
          await Promise.all([refreshAvailableBots(), refreshConversationBots()]);
        } catch (error) {
          showToast(errorMessage(error), "error");
        }
      })();
      return;
    }
    if (detailTab === "bots") {
      setAvailableBots([]);
      setConversationBots([]);
    }
  }, [detailTab, refreshAvailableBots, refreshConversationBots, selectedConversationId, selectedConversationType, setAvailableBots, setConversationBots, showToast]);

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
        dailyTokenLimit: 1_000_000,
        remainingTokens: 1_000_000
      });
    }
  }, [selectedConversationId, setAICallLogs, setAICallLogQuota]);

  return {
    refreshAICallLogs,
    handleAddBot,
    handleRemoveBot
  };
}
