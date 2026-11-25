import { useMemo } from 'react'
import { marked } from 'marked'
import './markdown.css'

interface MarkdownRendererProps {
  content: string
}

export function MarkdownRenderer({ content }: MarkdownRendererProps) {
  const html = useMemo(() => {
    return marked.parse(content, { gfm: true, breaks: true }) as string
  }, [content])

  return (
    <div
      className="markdown-body"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}
