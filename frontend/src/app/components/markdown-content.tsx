import { useMemo } from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import "katex/dist/katex.min.css";

type MarkdownContentProps = {
  content: string;
};

export function MarkdownContent({ content }: MarkdownContentProps) {
  const normalizedContent = useMemo(() => normalizeMarkdownMath(content), [content]);

  return (
    <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
      {normalizedContent}
    </ReactMarkdown>
  );
}

function normalizeMarkdownMath(content: string) {
  if (!content) {
    return "";
  }

  let normalized = content.replace(/\r\n/g, "\n");
  if (normalized.includes("\\[") && normalized.includes("\\]")) {
    normalized = normalized.replace(/\\\[((?:[\s\S]*?))\\\]/g, (_, body: string) => `$$\n${body.trim()}\n$$`);
  }
  if (normalized.includes("\\(") && normalized.includes("\\)")) {
    normalized = normalized.replace(/\\\(((?:[\s\S]*?))\\\)/g, (_, body: string) => `$${body.trim()}$`);
  }
  normalized = wrapStandaloneMathLines(normalized);
  return normalized;
}

function wrapStandaloneMathLines(content: string) {
  const lines = content.split("\n");
  const queue = [...lines];
  const result: string[] = [];
  let inCodeFence = false;
  let inDisplayMath = false;

  while (queue.length > 0) {
    const rawLine = queue.shift() as string;
    const line = stripUnwantedIndentation(rawLine);
    const trimmed = line.trim();

    if (trimmed.startsWith("```")) {
      inCodeFence = !inCodeFence;
      result.push(line);
      continue;
    }

    if (inCodeFence) {
      result.push(rawLine);
      continue;
    }

    if (inDisplayMath && trimmed.includes("$$")) {
      const markerIndex = trimmed.indexOf("$$");
      const before = trimmed.slice(0, markerIndex).trim();
      const after = trimmed.slice(markerIndex + 2).trim();
      if (before) {
        result.push(before);
      }
      result.push("$$");
      inDisplayMath = false;
      if (after) {
        queue.unshift(after);
      }
      continue;
    }

    if (inDisplayMath) {
      if (!shouldContinueDisplayMath(trimmed)) {
        result.push("$$");
        inDisplayMath = false;
        queue.unshift(rawLine);
        continue;
      }
      result.push(trimmed);
      continue;
    }

    const markerIndex = line.indexOf("$$");
    if (markerIndex >= 0) {
      const prefix = line.slice(0, markerIndex).replace(/\s+$/, "");
      const suffix = line.slice(markerIndex + 2).trim();
      if (prefix.trim()) {
        result.push(prefix);
      }
      result.push("$$");
      if (suffix) {
        const closingIndex = suffix.indexOf("$$");
        if (closingIndex >= 0) {
          const body = suffix.slice(0, closingIndex).trim();
          if (body) {
            result.push(body);
          }
          result.push("$$");
          const trailing = suffix.slice(closingIndex + 2).trim();
          if (trailing) {
            queue.unshift(trailing);
          }
        } else {
          result.push(suffix);
          inDisplayMath = true;
        }
      } else {
        inDisplayMath = true;
      }
      continue;
    }

    if (trimmed.startsWith("$") && trimmed.endsWith("$") && trimmed.length > 1) {
      result.push(trimmed);
      continue;
    }

    if (shouldWrapStandaloneMathLine(trimmed)) {
      result.push("$$");
      result.push(trimmed);
      result.push("$$");
      continue;
    }

    result.push(line);
  }

  if (inDisplayMath) {
    result.push("$$");
  }

  return result.join("\n");
}

function stripUnwantedIndentation(line: string) {
  if (!line || !/^[ \t]+/.test(line)) {
    return line;
  }

  const trimmed = line.trimStart();
  if (!trimmed) {
    return "";
  }

  if (
    trimmed.startsWith("```") ||
    trimmed.startsWith("- ") ||
    trimmed.startsWith("* ") ||
    trimmed.startsWith("> ") ||
    /^\d+\.\s/.test(trimmed) ||
    /^#{1,6}\s/.test(trimmed) ||
    /^\|/.test(trimmed)
  ) {
    return line;
  }

  return trimmed;
}

function shouldContinueDisplayMath(line: string) {
  if (!line) {
    return true;
  }
  if (line === "$$" || line.includes("$$")) {
    return true;
  }
  if (line.startsWith("- ") || line.startsWith("* ") || line.startsWith("> ")) {
    return false;
  }
  if (/^\d+\.\s/.test(line)) {
    return false;
  }
  if (line.includes("$")) {
    return false;
  }
  if (/[\p{Script=Han}]/u.test(line)) {
    return false;
  }
  return /\\|[_^=+\-*/<>()[\]{}]|[0-9A-Za-z]/.test(line);
}

function shouldWrapStandaloneMathLine(line: string) {
  if (!line) {
    return false;
  }
  if (line.includes("$")) {
    return false;
  }
  if (line.startsWith("- ") || line.startsWith("* ") || line.startsWith("> ")) {
    return false;
  }
  if (/[，。；：、]/.test(line)) {
    return false;
  }
  if (!/\\(frac|partial|lim|sum|int|sqrt|alpha|beta|gamma|delta|theta|lambda|mu|pi|rho|sigma|cos|sin|tan|cdot|to|times|begin|end|left|right)|[_^=]/i.test(line)) {
    return false;
  }
  if (/[\p{Script=Han}]/u.test(line) && !/\\(frac|partial|lim|sum|int|sqrt|alpha|beta|gamma|delta|theta|lambda|mu|pi|rho|sigma|cos|sin|tan|cdot|to|times|begin|end|left|right)/i.test(line)) {
    return false;
  }
  return true;
}
