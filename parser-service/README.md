# parser-service

独立文档解析服务（FastAPI）：
- 支持 `txt/md/pdf/docx/pptx`
- 对 `pdf/pptx` 支持图片计数
- 图片占比较高时可调用 Qwen 视觉模型生成描述文本

环境变量（可选）：
- `VISION_BASE_URL` / `VISION_API_KEY` / `VISION_MODEL`（默认 `qwen3.6-plus`）
- `VISION_TIMEOUT_SECONDS`
- `PARSER_MAX_DESCRIBE_IMAGES`
- `PARSER_IMAGE_HEAVY_MIN_IMAGES`
- `PARSER_IMAGE_HEAVY_TEXT_CHARS_PER_IMAGE`
