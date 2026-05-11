import { useCallback, useMemo, useState } from "react";
import type { MessageInfo, UserInfo } from "../../types";
import type { BrowserNotificationStatus, ToastTone } from "../types";
import { getNotificationStatus, messageText, truncateNotificationBody } from "../utils";
import { loadNotificationPreference, saveNotificationPreference } from "../helpers/chat-runtime";

type UseNotificationDomainDeps = {
  user: UserInfo | null;
  showToast: (message: string, tone?: ToastTone) => void;
};

export function useNotificationDomain(deps: UseNotificationDomainDeps) {
  const { user, showToast } = deps;

  const [notificationStatus, setNotificationStatus] = useState<BrowserNotificationStatus>(() => getNotificationStatus());
  const [notificationsEnabled, setNotificationsEnabled] = useState(() => loadNotificationPreference());

  const showMessageNotification = useCallback(
    (message: MessageInfo) => {
      if (!user || message.senderId === user.user_id || document.visibilityState === "visible") return;
      if (!notificationsEnabled) return;
      if (getNotificationStatus() !== "granted") return;
      if (message.messageType === "SYSTEM") return;

      const title = message.senderType === "BOT" || message.messageType === "BOT_REPLY" ? "AIM Bot reply" : "AIM new message";
      const content = messageText(message).trim();
      try {
        new Notification(title, {
          body: content ? truncateNotificationBody(content) : "You received a new message"
        });
      } catch {
        // Notification support can disappear in restricted browser contexts.
      }
    },
    [notificationsEnabled, user]
  );

  const requestNotifications = useCallback(async () => {
    if (typeof Notification === "undefined") {
      setNotificationStatus("unsupported");
      showToast("当前浏览器不支持通知", "error");
      return;
    }
    if (Notification.permission === "granted") {
      setNotificationStatus("granted");
      setNotificationsEnabled(true);
      saveNotificationPreference(true);
      showToast("Browser notifications enabled", "success");
      return;
    }
    if (Notification.permission === "denied") {
      setNotificationStatus("denied");
      showToast("浏览器已阻止通知权限", "error");
      return;
    }

    const permission = await Notification.requestPermission();
    setNotificationStatus(permission);
    if (permission === "granted") {
      setNotificationsEnabled(true);
      saveNotificationPreference(true);
    }
    showToast(permission === "granted" ? "Browser notifications enabled" : "Browser notifications not enabled", permission === "granted" ? "success" : "info");
  }, [showToast]);

  const toggleNotifications = useCallback(async () => {
    if (notificationStatus === "granted") {
      const next = !notificationsEnabled;
      setNotificationsEnabled(next);
      saveNotificationPreference(next);
      showToast(next ? "Browser notifications enabled" : "Browser notifications disabled", next ? "success" : "info");
      return;
    }
    await requestNotifications();
  }, [notificationStatus, notificationsEnabled, requestNotifications, showToast]);

  return useMemo(
    () => ({
      notificationStatus,
      notificationsEnabled,
      showMessageNotification,
      handleToggleNotifications: toggleNotifications
    }),
    [notificationStatus, notificationsEnabled, showMessageNotification, toggleNotifications]
  );
}
