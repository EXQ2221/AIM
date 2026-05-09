DELETE FROM bots WHERE id = 100000;

INSERT INTO bots (id, name, mention_name, aliases, avatar, description, model_name, system_prompt, created_by, status, created_at, updated_at)
VALUES (
  100000,
  'AI 助手',
  'ai',
  '["AI助手","AI","助手"]',
  '',
  'AIM 内置 AI 助手，支持 @ai 触发对话',
  'deepseek-chat',
  '你是 AIM 平台的 AI 助手。你的职责是帮助用户解决问题、回答问题、提供有用的建议。请用简洁、友好的中文回复。',
  1,
  'ENABLED',
  NOW(),
  NOW()
);