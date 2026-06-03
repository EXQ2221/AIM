package handler

import (
	"strings"

	"example.com/aim/gateway/internal/llmprofile"
	"example.com/aim/gateway/internal/middleware"
	"example.com/aim/gateway/internal/model/dto"
	"example.com/aim/gateway/internal/queryrouter"
	"github.com/gin-gonic/gin"
)

func PlanQueryRoute(ctx *gin.Context) {
	if _, ok := middleware.GetAuthContext(ctx); !ok {
		writeError(ctx, 401, "missing auth context")
		return
	}

	var req dto.QueryRoutePlanRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		writeError(ctx, 400, "invalid request body")
		return
	}
	if strings.TrimSpace(req.UserQuery) == "" {
		writeError(ctx, 400, "userQuery is required")
		return
	}

	planner, err := newQueryRouterPlanner(ctx, req.ConversationID, req.BotID)
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	plan, err := planner.Plan(ctx.Request.Context(), toQueryRouterInput(req))
	if err != nil {
		writeError(ctx, 500, err.Error())
		return
	}

	writeJSON(ctx, 200, dto.APIResponse{
		Code:    0,
		Message: "success",
		Data:    toQueryRoutePlanDTO(*plan),
	})
}

func toQueryRouterInput(req dto.QueryRoutePlanRequest) queryrouter.PlanningInput {
	targets := make([]queryrouter.Target, 0, len(req.SelectedTargets))
	for _, item := range req.SelectedTargets {
		targets = append(targets, queryrouter.Target{
			ID:    item.ID,
			Type:  item.Type,
			Title: item.Title,
		})
	}
	return queryrouter.PlanningInput{
		UserQuery:       req.UserQuery,
		SelectedTargets: targets,
		AvailableSpaces: queryrouter.AvailableSpaces{
			Conversation:      req.AvailableSpaces.Conversation,
			KnowledgeBase:     req.AvailableSpaces.KnowledgeBase,
			SelectedDocuments: req.AvailableSpaces.SelectedDocuments,
			AllDocuments:      req.AvailableSpaces.AllDocuments,
			Metadata:          req.AvailableSpaces.Metadata,
			Mixed:             req.AvailableSpaces.Mixed,
		},
		Capabilities: queryrouter.Capabilities{
			CanLookup:                  req.Capabilities.CanLookup,
			CanFullReadDocument:        req.Capabilities.CanFullReadDocument,
			CanSynthesizeMultiDocument: req.Capabilities.CanSynthesizeMultiDocument,
			CanExtractExactQuote:       req.Capabilities.CanExtractExactQuote,
			CanControlBindings:         req.Capabilities.CanControlBindings,
			CanUseExternalWeb:          req.Capabilities.CanUseExternalWeb,
		},
		ContextHints: queryrouter.ContextHints{
			ConversationID:     req.ContextHints.ConversationID,
			CurrentDocumentIDs: req.ContextHints.CurrentDocumentIDs,
			CurrentKBIDs:       req.ContextHints.CurrentKBIDs,
		},
	}
}

func newQueryRouterPlanner(ctx *gin.Context, conversationID string, botID *int64) (queryrouter.Planner, error) {
	authCtx, _ := middleware.GetAuthContext(ctx)
	profile, err := llmprofile.ResolveFromConversationBotOrEnv(ctx.Request.Context(), authCtx.UserID, conversationID, botID)
	if err != nil {
		return nil, err
	}
	return queryrouter.NewHTTPPlanner(queryrouter.Config{
		BaseURL:            profile.BaseURL,
		APIKey:             profile.APIKey,
		Model:              profile.Model,
		Timeout:            profile.Timeout,
		InsecureSkipVerify: profile.InsecureSkipVerify,
	})
}

func toQueryRoutePlanDTO(plan queryrouter.Plan) dto.QueryRoutePlanInfo {
	return dto.QueryRoutePlanInfo{
		PlanVersion:   plan.PlanVersion,
		Family:        string(plan.Family),
		SourceSpace:   string(plan.SourceSpace),
		Scope:         string(plan.Scope),
		ReadDepth:     string(plan.ReadDepth),
		OutputMode:    string(plan.OutputMode),
		EvidenceMode:  string(plan.EvidenceMode),
		Targets:       plan.Targets,
		Constraints: dto.QueryRouteConstraints{
			MustGroundInSources: plan.Constraints.MustGroundInSources,
			AllowExternalWeb:    plan.Constraints.AllowExternalWeb,
			StrictQuoteRequired: plan.Constraints.StrictQuoteRequired,
		},
		Confidence:     plan.Confidence,
		FallbackFamily: string(plan.FallbackFamily),
		Reason:         plan.Reason,
	}
}
