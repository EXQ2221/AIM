# parser-service

独立文档解析服务（FastAPI）：
- 支持 `txt/md/pdf/docx/pptx`
- 对 `pdf/pptx` 支持图片计数
- 图片占比高时可调用视觉模型生成描述文本
- 支持调用轻量 LLM 对解析文本做语义切分，返回结构化 `chunks`

环境变量（可选）：
- 视觉描述：
  - `VISION_BASE_URL` / `VISION_API_KEY` / `VISION_MODEL`（默认 `qwen3.6-plus`）
  - `VISION_TIMEOUT_SECONDS`
  - `PARSER_MAX_DESCRIBE_IMAGES`
  - `PARSER_IMAGE_HEAVY_MIN_IMAGES`
  - `PARSER_IMAGE_HEAVY_TEXT_CHARS_PER_IMAGE`
- 语义切分：
  - `PARSER_ENABLE_LLM_CHUNKING`
  - `CHUNKER_BASE_URL` / `CHUNKER_API_KEY` / `CHUNKER_MODEL`（默认 `deepseek-v4-flash`）
  - `CHUNKER_TIMEOUT_SECONDS`
  - `CHUNKER_MAX_CONTENT_CHARS`
  - `CHUNKER_FALLBACK_CHARS`
