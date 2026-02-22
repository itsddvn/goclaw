import { RichContent } from "./rich-content";

interface MessageContentProps {
  content: string;
  role: string;
}

export function MessageContent({ content, role }: MessageContentProps) {
  return <RichContent content={content} role={role} />;
}
