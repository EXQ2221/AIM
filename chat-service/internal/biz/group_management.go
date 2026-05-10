package biz

import (
	"context"
	"errors"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"gorm.io/gorm"
)

var (
	ErrGroupOwnerRequired  = errors.New("forbidden: only owner can perform this action")
	ErrCannotTransferOwner = errors.New("forbidden: cannot transfer ownership to target member")
	ErrCannotSetAdmin      = errors.New("forbidden: cannot set admin for target member")
	ErrCannotRemoveAdmin   = errors.New("forbidden: cannot remove admin from target member")
	ErrCannotManageTarget  = errors.New("forbidden: cannot manage target member")
	ErrMuteUntilInvalid    = errors.New("bad_request: invalid mute deadline")
)

func (s *ChatService) TransferOwner(ctx context.Context, input TransferOwnerInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 || input.OperatorID == input.TargetUserID {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	target, err := s.requireMember(ctx, conversation.ID, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	if !canTransferOwner(operator, target) {
		return nil, ErrCannotTransferOwner
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		groupRepo := s.GroupRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, currentTarget, group, loadErr := s.loadGroupMembersForManagement(ctx, memberRepo, groupRepo, conversation.ID, input.OperatorID, input.TargetUserID)
		if loadErr != nil {
			return loadErr
		}
		if !canTransferOwner(currentOperator, currentTarget) {
			return ErrCannotTransferOwner
		}
		if currentTarget.Role == model.MemberRoleOwner && group.OwnerID == input.TargetUserID {
			return nil
		}

		currentOperator.Role = model.MemberRoleMember
		currentTarget.Role = model.MemberRoleOwner
		group.OwnerID = input.TargetUserID

		if err := memberRepo.Update(ctx, currentOperator); err != nil {
			return err
		}
		if err := memberRepo.Update(ctx, currentTarget); err != nil {
			return err
		}
		if err := groupRepo.Update(ctx, group); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			model.SystemEventOwnerTransferred,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) SetAdmin(ctx context.Context, input SetAdminInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 || input.OperatorID == input.TargetUserID {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	target, err := s.requireMember(ctx, conversation.ID, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	if operator.Role != model.MemberRoleOwner {
		return nil, ErrGroupOwnerRequired
	}
	if target.Role == model.MemberRoleAdmin {
		return nil, nil
	}
	if !canSetAdmin(operator, target) {
		return nil, ErrCannotSetAdmin
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, currentTarget, loadErr := s.loadManagedMembersTx(ctx, memberRepo, conversation.ID, input.OperatorID, input.TargetUserID)
		if loadErr != nil {
			return loadErr
		}
		if currentOperator.Role != model.MemberRoleOwner {
			return ErrGroupOwnerRequired
		}
		if currentTarget.Role == model.MemberRoleAdmin {
			return nil
		}
		if !canSetAdmin(currentOperator, currentTarget) {
			return ErrCannotSetAdmin
		}

		currentTarget.Role = model.MemberRoleAdmin
		if err := memberRepo.Update(ctx, currentTarget); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			model.SystemEventAdminAdded,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) RemoveAdmin(ctx context.Context, input RemoveAdminInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 || input.OperatorID == input.TargetUserID {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	target, err := s.requireMember(ctx, conversation.ID, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	if operator.Role != model.MemberRoleOwner {
		return nil, ErrGroupOwnerRequired
	}
	if target.Role != model.MemberRoleAdmin {
		return nil, nil
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, currentTarget, loadErr := s.loadManagedMembersTx(ctx, memberRepo, conversation.ID, input.OperatorID, input.TargetUserID)
		if loadErr != nil {
			return loadErr
		}
		if currentOperator.Role != model.MemberRoleOwner {
			return ErrGroupOwnerRequired
		}
		if currentTarget.Role != model.MemberRoleAdmin {
			return nil
		}
		if currentTarget.Role == model.MemberRoleOwner {
			return ErrCannotRemoveAdmin
		}

		currentTarget.Role = model.MemberRoleMember
		if err := memberRepo.Update(ctx, currentTarget); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			model.SystemEventAdminRemoved,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) MuteMember(ctx context.Context, input MuteMemberInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 || input.OperatorID == input.TargetUserID {
		return nil, ErrBadRequest
	}
	muteUntil := time.Unix(input.MuteUntil, 0)
	if input.MuteUntil <= 0 || !muteUntil.After(time.Now()) {
		return nil, ErrMuteUntilInvalid
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	target, err := s.requireMember(ctx, conversation.ID, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	if !canMuteMember(operator, target) {
		return nil, ErrCannotManageTarget
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, currentTarget, loadErr := s.loadManagedMembersTx(ctx, memberRepo, conversation.ID, input.OperatorID, input.TargetUserID)
		if loadErr != nil {
			return loadErr
		}
		if !canMuteMember(currentOperator, currentTarget) {
			return ErrCannotManageTarget
		}

		currentTarget.MuteUntil = &muteUntil
		currentTarget.Status = model.MemberStatusNormal
		if err := memberRepo.Update(ctx, currentTarget); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			model.SystemEventMemberMuted,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) UnmuteMember(ctx context.Context, input UnmuteMemberInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 || input.OperatorID == input.TargetUserID {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	target, err := s.requireMember(ctx, conversation.ID, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	if !canMuteMember(operator, target) {
		return nil, ErrCannotManageTarget
	}
	if !isMutedNow(target) && target.MuteUntil == nil {
		return nil, nil
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, currentTarget, loadErr := s.loadManagedMembersTx(ctx, memberRepo, conversation.ID, input.OperatorID, input.TargetUserID)
		if loadErr != nil {
			return loadErr
		}
		if !canMuteMember(currentOperator, currentTarget) {
			return ErrCannotManageTarget
		}
		if currentTarget.MuteUntil == nil && currentTarget.Status != model.MemberStatusMuted {
			return nil
		}

		currentTarget.MuteUntil = nil
		currentTarget.Status = model.MemberStatusNormal
		if err := memberRepo.Update(ctx, currentTarget); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			model.SystemEventMemberUnmuted,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) RemoveMember(ctx context.Context, input RemoveMemberInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 || input.TargetUserID == 0 || input.OperatorID == input.TargetUserID {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	target, err := s.requireMember(ctx, conversation.ID, input.TargetUserID)
	if err != nil {
		return nil, err
	}
	if !canManageMember(operator, target) {
		return nil, ErrCannotManageTarget
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, currentTarget, loadErr := s.loadManagedMembersTx(ctx, memberRepo, conversation.ID, input.OperatorID, input.TargetUserID)
		if loadErr != nil {
			return loadErr
		}
		if !canManageMember(currentOperator, currentTarget) {
			return ErrCannotManageTarget
		}

		currentTarget.Status = model.MemberStatusRemoved
		currentTarget.MuteUntil = nil
		if err := memberRepo.Update(ctx, currentTarget); err != nil {
			return err
		}

		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			model.SystemEventMemberRemoved,
			input.OperatorID,
			[]uint64{input.TargetUserID},
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func (s *ChatService) SetGroupMuteAll(ctx context.Context, input SetGroupMuteAllInput) (*ConversationEventView, error) {
	if input.OperatorID == 0 {
		return nil, ErrBadRequest
	}

	conversation, err := s.requireGroupConversation(ctx, input.ConversationID)
	if err != nil {
		return nil, err
	}
	operator, err := s.requireMember(ctx, conversation.ID, input.OperatorID)
	if err != nil {
		return nil, err
	}
	if operator.Role != model.MemberRoleOwner {
		return nil, ErrGroupOwnerRequired
	}

	var event *ConversationEventView
	err = s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		groupRepo := s.GroupRepo.WithTx(tx)
		messageRepo := s.MessageRepo.WithTx(tx)
		conversationRepo := s.ConversationRepo.WithTx(tx)

		currentOperator, group, loadErr := s.loadOperatorAndGroupTx(ctx, memberRepo, groupRepo, conversation.ID, input.OperatorID)
		if loadErr != nil {
			return loadErr
		}
		if currentOperator.Role != model.MemberRoleOwner {
			return ErrGroupOwnerRequired
		}
		if group.MuteAll == input.MuteAll {
			return nil
		}

		now := time.Now()
		group.MuteAll = input.MuteAll
		group.MuteAllUpdatedBy = &input.OperatorID
		group.MuteAllUpdatedAt = &now
		if err := groupRepo.Update(ctx, group); err != nil {
			return err
		}

		eventType := model.SystemEventGroupMuted
		if !input.MuteAll {
			eventType = model.SystemEventGroupUnmuted
		}
		systemMessage, recipientUserIDs, eventErr := s.createSystemMessageTx(
			ctx,
			conversation,
			conversationRepo,
			memberRepo,
			messageRepo,
			eventType,
			input.OperatorID,
			nil,
		)
		if eventErr != nil {
			return eventErr
		}
		view := messageToView(systemMessage, conversation.ConversationID)
		event = &ConversationEventView{Message: &view, RecipientUserIDs: recipientUserIDs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return event, nil
}

func canTransferOwner(actor *model.ConversationMember, target *model.ConversationMember) bool {
	if actor == nil || target == nil {
		return false
	}
	return actor.Role == model.MemberRoleOwner &&
		actor.MemberID != target.MemberID &&
		(target.Role == model.MemberRoleMember || target.Role == model.MemberRoleAdmin)
}

func canSetAdmin(actor *model.ConversationMember, target *model.ConversationMember) bool {
	if actor == nil || target == nil {
		return false
	}
	return actor.Role == model.MemberRoleOwner &&
		actor.MemberID != target.MemberID &&
		target.Role == model.MemberRoleMember
}

func canManageMember(actor *model.ConversationMember, target *model.ConversationMember) bool {
	if actor == nil || target == nil || actor.MemberID == target.MemberID {
		return false
	}
	switch actor.Role {
	case model.MemberRoleOwner:
		return target.Role == model.MemberRoleMember || target.Role == model.MemberRoleAdmin
	case model.MemberRoleAdmin:
		return target.Role == model.MemberRoleMember
	default:
		return false
	}
}

func canMuteMember(actor *model.ConversationMember, target *model.ConversationMember) bool {
	return canManageMember(actor, target)
}

func isMutedNow(member *model.ConversationMember) bool {
	if member == nil {
		return false
	}
	return member.Status == model.MemberStatusMuted || (member.MuteUntil != nil && member.MuteUntil.After(time.Now()))
}

func (s *ChatService) loadManagedMembersTx(
	ctx context.Context,
	memberRepo interface {
		GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error)
	},
	conversationID uint64,
	operatorID uint64,
	targetUserID uint64,
) (*model.ConversationMember, *model.ConversationMember, error) {
	operator, err := memberRepo.GetUserMember(ctx, conversationID, operatorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrNotMember
		}
		return nil, nil, err
	}
	if operator.Status == model.MemberStatusRemoved {
		return nil, nil, ErrNotMember
	}

	target, err := memberRepo.GetUserMember(ctx, conversationID, targetUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrNotMember
		}
		return nil, nil, err
	}
	if target.Status == model.MemberStatusRemoved {
		return nil, nil, ErrNotMember
	}
	return operator, target, nil
}

func (s *ChatService) loadOperatorAndGroupTx(
	ctx context.Context,
	memberRepo interface {
		GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error)
	},
	groupRepo interface {
		GetByConversationID(ctx context.Context, conversationID uint64) (*model.GroupInfo, error)
	},
	conversationID uint64,
	operatorID uint64,
) (*model.ConversationMember, *model.GroupInfo, error) {
	operator, err := memberRepo.GetUserMember(ctx, conversationID, operatorID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrNotMember
		}
		return nil, nil, err
	}
	if operator.Status == model.MemberStatusRemoved {
		return nil, nil, ErrNotMember
	}

	group, err := groupRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrGroupNotFound
		}
		return nil, nil, err
	}
	return operator, group, nil
}

func (s *ChatService) loadGroupMembersForManagement(
	ctx context.Context,
	memberRepo interface {
		GetUserMember(ctx context.Context, conversationID, userID uint64) (*model.ConversationMember, error)
	},
	groupRepo interface {
		GetByConversationID(ctx context.Context, conversationID uint64) (*model.GroupInfo, error)
	},
	conversationID uint64,
	operatorID uint64,
	targetUserID uint64,
) (*model.ConversationMember, *model.ConversationMember, *model.GroupInfo, error) {
	operator, target, err := s.loadManagedMembersTx(ctx, memberRepo, conversationID, operatorID, targetUserID)
	if err != nil {
		return nil, nil, nil, err
	}
	group, err := groupRepo.GetByConversationID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, nil, ErrGroupNotFound
		}
		return nil, nil, nil, err
	}
	return operator, target, group, nil
}
