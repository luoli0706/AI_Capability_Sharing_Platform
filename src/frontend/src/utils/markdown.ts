function escapeHtml(raw: string): string {
  return raw
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function sanitizeUrl(url: string): string {
  const trimmed = url.trim()
  if (/^(https?:\/\/|\/|#)/i.test(trimmed)) {
    return trimmed
  }
  return '#'
}

function renderInline(escaped: string): string {
  // Extract inline code spans first so formatting inside code isn't altered
  const codePlaceholders: string[] = []
  let text = escaped.replace(/`([^`]+)`/g, (_match, code: string) => {
    const idx = codePlaceholders.length
    codePlaceholders.push(`<code>${code}</code>`)
    return `\u0000CODE${idx}\u0000`
  })

  // Links: [label](url)
  text = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, (_m, label: string, url: string) => {
    const safe = sanitizeUrl(url.replace(/&amp;/g, '&'))
    return `<a href="${escapeHtml(safe)}" target="_blank" rel="noopener noreferrer">${label}</a>`
  })

  // Bold & Italic
  text = text.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
  text = text.replace(/__([^_]+)__/g, '<strong>$1</strong>')
  text = text.replace(/\*([^*]+)\*/g, '<em>$1</em>')
  text = text.replace(/_([^_]+)_/g, '<em>$1</em>')
  text = text.replace(/~~([^~]+)~~/g, '<del>$1</del>')

  // Restore inline code
  text = text.replace(/\u0000CODE(\d+)\u0000/g, (_m, idx: string) => codePlaceholders[Number(idx)] ?? '')
  return text
}

function isTableSeparator(line: string): boolean {
  return /^\s*\|?(\s*:?-+:?\s*\|)+\s*:?-+:?\s*\|?\s*$/.test(line)
}

function splitTableRow(line: string): string[] {
  const trimmed = line.trim().replace(/^\|/, '').replace(/\|$/, '')
  return trimmed.split('|').map((cell) => cell.trim())
}

export function renderMarkdown(source: string): string {
  if (!source) return ''
  const normalized = source.replace(/\r\n/g, '\n')
  const lines = normalized.split('\n')
  const out: string[] = []

  let i = 0
  while (i < lines.length) {
    const line = lines[i]

    // Fenced code block
    const fenceMatch = line.match(/^```([\w-]*)\s*$/)
    if (fenceMatch) {
      const lang = escapeHtml(fenceMatch[1] || '')
      const codeLines: string[] = []
      i++
      while (i < lines.length && !/^```\s*$/.test(lines[i])) {
        codeLines.push(escapeHtml(lines[i]))
        i++
      }
      i++ // skip closing ```
      const langClass = lang ? ` class="language-${lang}"` : ''
      out.push(`<pre><code${langClass}>${codeLines.join('\n')}</code></pre>`)
      continue
    }

    // Blank line
    if (!line.trim()) {
      i++
      continue
    }

    // Horizontal rule
    if (/^\s*([-*_])\1{2,}\s*$/.test(line)) {
      out.push('<hr />')
      i++
      continue
    }

    // Heading
    const headingMatch = line.match(/^(#{1,6})\s+(.*)$/)
    if (headingMatch) {
      const level = headingMatch[1].length
      out.push(`<h${level}>${renderInline(escapeHtml(headingMatch[2]))}</h${level}>`)
      i++
      continue
    }

    // Markdown table
    if (line.includes('|') && i + 1 < lines.length && isTableSeparator(lines[i + 1])) {
      const headers = splitTableRow(line)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && lines[i].includes('|') && lines[i].trim()) {
        rows.push(splitTableRow(lines[i]))
        i++
      }
      const thead = `<thead><tr>${headers.map((h) => `<th>${renderInline(escapeHtml(h))}</th>`).join('')}</tr></thead>`
      const tbody = rows.length
        ? `<tbody>${rows
            .map(
              (r) =>
                `<tr>${headers
                  .map((_, idx) => `<td>${renderInline(escapeHtml(r[idx] ?? ''))}</td>`)
                  .join('')}</tr>`
            )
            .join('')}</tbody>`
        : ''
      out.push(`<table>${thead}${tbody}</table>`)
      continue
    }

    // Blockquote
    if (/^\s*>\s?/.test(line)) {
      const quoteLines: string[] = []
      while (i < lines.length && /^\s*>\s?/.test(lines[i])) {
        quoteLines.push(renderInline(escapeHtml(lines[i].replace(/^\s*>\s?/, ''))))
        i++
      }
      out.push(`<blockquote><p>${quoteLines.join('<br />')}</p></blockquote>`)
      continue
    }

    // Unordered list
    if (/^\s*[-*+]\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*[-*+]\s+/.test(lines[i])) {
        items.push(`<li>${renderInline(escapeHtml(lines[i].replace(/^\s*[-*+]\s+/, '')))}</li>`)
        i++
      }
      out.push(`<ul>${items.join('')}</ul>`)
      continue
    }

    // Ordered list
    if (/^\s*\d+\.\s+/.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        items.push(`<li>${renderInline(escapeHtml(lines[i].replace(/^\s*\d+\.\s+/, '')))}</li>`)
        i++
      }
      out.push(`<ol>${items.join('')}</ol>`)
      continue
    }

    // Paragraph
    const paraLines: string[] = []
    while (
      i < lines.length &&
      lines[i].trim() &&
      !/^```/.test(lines[i]) &&
      !/^#{1,6}\s+/.test(lines[i]) &&
      !/^\s*>\s?/.test(lines[i]) &&
      !/^\s*[-*+]\s+/.test(lines[i]) &&
      !/^\s*\d+\.\s+/.test(lines[i]) &&
      !/^\s*([-*_])\1{2,}\s*$/.test(lines[i])
    ) {
      paraLines.push(renderInline(escapeHtml(lines[i])))
      i++
    }
    out.push(`<p>${paraLines.join('<br />')}</p>`)
  }

  return out.join('\n')
}
