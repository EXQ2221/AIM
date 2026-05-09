package biz

import (
	"errors"
	"testing"

	"example.com/aim/chat-service/internal/dal/model"
)

func TestValidateBaseBotTokensRejectsReservedMentionName(t *testing.T) {
	err := validateBaseBotTokens(model.Bot{
		MentionName: "system",
		Aliases:     "[\"helper\"]",
	})
	if !errors.Is(err, ErrBotReservedName) {
		t.Fatalf("expected ErrBotReservedName, got %v", err)
	}
}

func TestValidateBaseBotTokensRejectsInvalidAlias(t *testing.T) {
	err := validateBaseBotTokens(model.Bot{
		MentionName: "aim",
		Aliases:     "[\"@helper\"]",
	})
	if !errors.Is(err, ErrBotAliasInvalid) {
		t.Fatalf("expected ErrBotAliasInvalid, got %v", err)
	}
}
