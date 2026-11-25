import type { ReactNode } from 'react'

export interface DocArticle {
  key: string
  title: string
  slug: string
  description?: string
  icon?: ReactNode
  rawMarkdown: string
}

export interface DocCategory {
  key: string
  title: string
  articles: DocArticle[]
}
