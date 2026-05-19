package llm

import (
	llmbiz "example.com/aim/chat-service/llm-internal/biz"
	llmconf "example.com/aim/chat-service/llm-internal/conf"
	llmdal "example.com/aim/chat-service/llm-internal/dal"
	llmmodel "example.com/aim/chat-service/llm-internal/model"
)

type ChatMessage = llmmodel.ChatMessage

type ChatMessagePart = llmmodel.ChatMessagePart

type GenerateRequest = llmmodel.GenerateRequest

type StreamChunk = llmmodel.StreamChunk

type GenerateResponse = llmmodel.GenerateResponse

type Client = llmmodel.Client

type StreamingClient = llmmodel.StreamingClient

type Config = llmmodel.Config

type MultiConfig = llmmodel.MultiConfig

type Registry = llmbiz.Registry

type HTTPStatusError = llmdal.HTTPStatusError

type OpenAICompatibleClient = llmdal.OpenAICompatibleClient

func LoadConfigFromEnv() (Config, error) {
	return llmconf.LoadConfigFromEnv()
}

func LoadMultiConfigFromEnv() (MultiConfig, error) {
	return llmconf.LoadMultiConfigFromEnv()
}

func NewOpenAICompatibleClient(cfg Config) (*OpenAICompatibleClient, error) {
	return llmdal.NewOpenAICompatibleClient(cfg)
}

func NewOpenAICompatibleClientFromEnv() (*OpenAICompatibleClient, error) {
	return llmdal.NewOpenAICompatibleClientFromEnv()
}

func NewRegistry(multiCfg MultiConfig) (*Registry, error) {
	return llmbiz.NewRegistry(multiCfg)
}
