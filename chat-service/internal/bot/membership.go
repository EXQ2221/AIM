package bot

import (
	"context"
	"errors"
	"time"

	"example.com/aim/chat-service/internal/dal/model"
	"example.com/aim/chat-service/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrConversationIDRequired       = errors.New("conversation id is required")
	ErrBotIDRequired                = errors.New("bot id is required")
	ErrConversationNotFound         = errors.New("conversation not found")
	ErrConversationTypeNotSupported = errors.New("bot membership only supports group conversations")
	ErrBotNotFound                  = errors.New("bot not found")
	ErrBotNotInConversation         = errors.New("bot is not attached to conversation")
)

type ConversationBotConfig struct {
	ModelNameOverride   string
	DisplayNameOverride string
	MentionNameOverride string
	AliasesOverride     string
	PermissionScope     model.BotPermissionScope
}

type MembershipService struct {
	TxManager           repository.TxManager
	ConversationRepo    repository.ConversationRepository
	MemberRepo          repository.MemberRepository
	BotRepo             repository.BotRepository
	ConversationBotRepo repository.ConversationBotRepository
}

func NewMembershipService(
	txManager repository.TxManager,
	conversationRepo repository.ConversationRepository,
	memberRepo repository.MemberRepository,
	botRepo repository.BotRepository,
	conversationBotRepo repository.ConversationBotRepository,
) *MembershipService {
	return &MembershipService{
		TxManager:           txManager,
		ConversationRepo:    conversationRepo,
		MemberRepo:          memberRepo,
		BotRepo:             botRepo,
		ConversationBotRepo: conversationBotRepo,
	}
}

func (s *MembershipService) AddBotToConversation(ctx context.Context, conversationID, botID uint64) error {
	return s.AddBotToConversationWithConfig(ctx, conversationID, botID, ConversationBotConfig{
		PermissionScope: model.BotScopeConversationOnly,
	})
}

func (s *MembershipService) AddBotToConversationWithConfig(ctx context.Context, conversationID, botID uint64, cfg ConversationBotConfig) error {
	if s == nil {
		return errors.New("membership service is nil")
	}
	if conversationID == 0 {
		return ErrConversationIDRequired
	}
	if botID == 0 {
		return ErrBotIDRequired
	}
	if s.TxManager == nil || s.ConversationRepo == nil || s.MemberRepo == nil || s.BotRepo == nil || s.ConversationBotRepo == nil {
		return errors.New("membership service dependencies are not complete")
	}

	conversation, err := s.ConversationRepo.GetByID(ctx, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConversationNotFound
		}
		return err
	}
	if conversation.Type != model.ConversationTypeGroup {
		return ErrConversationTypeNotSupported
	}
	if _, err := s.BotRepo.GetByID(ctx, botID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrBotNotFound
		}
		return err
	}

	return s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		conversationBotRepo := s.ConversationBotRepo.WithTx(tx)
		now := time.Now()
		permissionScope := cfg.PermissionScope
		if permissionScope == "" {
			permissionScope = model.BotScopeConversationOnly
		}

		member, err := memberRepo.GetBotMember(ctx, conversationID, botID)
		switch {
		case err == nil:
			member.Role = model.MemberRoleBot
			member.Status = model.MemberStatusNormal
			member.JoinedAt = now
			if err := memberRepo.Update(ctx, member); err != nil {
				return err
			}
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := memberRepo.Create(ctx, &model.ConversationMember{
				ConversationID: conversationID,
				MemberType:     model.MemberTypeBot,
				MemberID:       botID,
				Role:           model.MemberRoleBot,
				Status:         model.MemberStatusNormal,
				JoinedAt:       now,
			}); err != nil {
				return err
			}
		default:
			return err
		}

		conversationBot, err := conversationBotRepo.GetByConversationAndBotID(ctx, conversationID, botID)
		switch {
		case err == nil:
			conversationBot.Enabled = true
			conversationBot.PermissionScope = permissionScope
			conversationBot.ModelNameOverride = cfg.ModelNameOverride
			conversationBot.DisplayNameOverride = cfg.DisplayNameOverride
			conversationBot.MentionNameOverride = cfg.MentionNameOverride
			conversationBot.AliasesOverride = cfg.AliasesOverride
			return conversationBotRepo.Update(ctx, conversationBot)
		case errors.Is(err, gorm.ErrRecordNotFound):
			return conversationBotRepo.Create(ctx, &model.ConversationBot{
				ConversationID:      conversationID,
				BotID:               botID,
				Enabled:             true,
				PermissionScope:     permissionScope,
				ModelNameOverride:   cfg.ModelNameOverride,
				DisplayNameOverride: cfg.DisplayNameOverride,
				MentionNameOverride: cfg.MentionNameOverride,
				AliasesOverride:     cfg.AliasesOverride,
			})
		default:
			return err
		}
	})
}

func (s *MembershipService) RemoveBotFromConversation(ctx context.Context, conversationID, botID uint64) error {
	if s == nil {
		return errors.New("membership service is nil")
	}
	if conversationID == 0 {
		return ErrConversationIDRequired
	}
	if botID == 0 {
		return ErrBotIDRequired
	}
	if s.TxManager == nil || s.MemberRepo == nil || s.ConversationBotRepo == nil {
		return errors.New("membership service dependencies are not complete")
	}

	return s.TxManager.WithinTransaction(ctx, func(tx *gorm.DB) error {
		memberRepo := s.MemberRepo.WithTx(tx)
		conversationBotRepo := s.ConversationBotRepo.WithTx(tx)
		updated := false

		member, err := memberRepo.GetBotMember(ctx, conversationID, botID)
		switch {
		case err == nil:
			if member.Status == model.MemberStatusNormal {
				updated = true
			}
			member.Status = model.MemberStatusRemoved
			if err := memberRepo.Update(ctx, member); err != nil {
				return err
			}
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		conversationBot, err := conversationBotRepo.GetByConversationAndBotID(ctx, conversationID, botID)
		switch {
		case err == nil:
			if conversationBot.Enabled {
				updated = true
			}
			if !updated && !conversationBot.Enabled {
				return ErrBotNotInConversation
			}
			conversationBot.Enabled = false
			return conversationBotRepo.Update(ctx, conversationBot)
		case errors.Is(err, gorm.ErrRecordNotFound):
			if updated {
				return nil
			}
			return ErrBotNotInConversation
		default:
			return err
		}
	})
}
