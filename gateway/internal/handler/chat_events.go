package handler

import (
	"context"

	gatewayws "example.com/aim/gateway/internal/websocket"
	chatpb "example.com/aim/gateway/kitex_gen/chat"
	"example.com/aim/gateway/kitex_gen/chat/chatservice"
)

func broadcastConversationEvent(ctx context.Context, client chatservice.Client, resp *chatpb.ConversationEventResponse) {
	if resp == nil || resp.EventMessage == nil || len(resp.RecipientUserIds) == 0 {
		return
	}
	chatHub.SendToUsers(resp.RecipientUserIds, gatewayws.OutgoingEvent{
		Type: gatewayws.EventNewMessage,
		Data: gatewayws.ToMessageInfo(resp.EventMessage),
	})
	broadcastEventNotifications(ctx, client, resp)
}

func broadcastEventNotifications(ctx context.Context, client chatservice.Client, resp *chatpb.ConversationEventResponse) {
	if client == nil || resp == nil || resp.EventMessage == nil || len(resp.RecipientUserIds) == 0 {
		return
	}

	limit := int32(5)
	unreadOnly := true
	relatedMessageID := resp.EventMessage.Id
	for _, userID := range resp.RecipientUserIds {
		notificationResp, err := client.ListNotifications(ctx, &chatpb.ListNotificationsRequest{
			OperatorId: userID,
			UnreadOnly: &unreadOnly,
			Limit:      &limit,
		})
		if err != nil || notificationResp == nil {
			continue
		}
		for _, item := range notificationResp.Notifications {
			if item == nil || item.RelatedMessageId == nil || *item.RelatedMessageId != relatedMessageID {
				continue
			}
			chatHub.SendToUsers([]int64{userID}, gatewayws.OutgoingEvent{
				Type: gatewayws.EventNotificationCreated,
				Data: gatewayws.NotificationCreatedData{
					Notification: gatewayws.ToNotificationInfo(item),
					UnreadCount:  &notificationResp.UnreadCount,
				},
			})
			break
		}
	}
}

func broadcastMessageRecalledEvent(resp *chatpb.MessageRecalledEventResponse) {
	if resp == nil || resp.Event == nil || len(resp.RecipientUserIds) == 0 {
		return
	}
	chatHub.SendToUsers(resp.RecipientUserIds, gatewayws.OutgoingEvent{
		Type: gatewayws.EventMessageRecalled,
		Data: gatewayws.MessageRecalledInfo{
			MessageID:      resp.Event.MessageId,
			ConversationID: resp.Event.ConversationId,
		},
	})
}
