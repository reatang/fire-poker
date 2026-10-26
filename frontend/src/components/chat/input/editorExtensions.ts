import { Extension, InputRule } from '@tiptap/core'
import type { Editor, JSONContent, Range } from '@tiptap/core'
import type { Node as ProseMirrorNode } from '@tiptap/pm/model'
import Suggestion from '@tiptap/suggestion'
import { buildSlashSuggestion } from '../slashSuggestion'

// Editor extensions for the chat composer: "/" skill suggestions and Markdown-style code fences.
export const SlashCommand = Extension.create({
  name: 'slashCommand',
  addProseMirrorPlugins() {
    return [
      Suggestion({
        editor: this.editor,
        ...buildSlashSuggestion(),
        command: ({ editor, range, props }: { editor: Editor; range: Range; props: { name: string } }) => {
          editor.chain().focus().deleteRange(range).insertContent(`/${props.name} `).run()
        },
      }),
    ]
  },
})

export const SingleLineCodeBlock = Extension.create({
  name: 'singleLineCodeBlock',

  addInputRules() {
    return [
      new InputRule({
        find: /^```\s?([^`]+?)\s?```$/,
        handler: ({ range, match, commands }) => {
          const code = match[1]?.trim() ?? ''

          commands.insertContentAt(range, {
            type: 'codeBlock',
            content: code ? [{ type: 'text', text: code }] : [],
          })
        },
      }),
    ]
  },
})

const createCodeBlockContent = (code: string): JSONContent => ({
  type: 'codeBlock',
  content: code ? [{ type: 'text', text: code }] : [],
})

const getNodeTextWithBreaks = (node: ProseMirrorNode) => (
  node.textBetween(0, node.content.size, '\n', '\n')
)

export const convertFencedCodeBlock = (editor: Editor) => {
  let converted = false

  editor.state.doc.descendants((node, pos) => {
    if (converted || node.type.name !== 'paragraph') return false

    const match = getNodeTextWithBreaks(node).match(/^```[^\n]*\n([\s\S]*?)\n```$/)
    if (!match) return true

    editor.commands.insertContentAt(
      { from: pos, to: pos + node.nodeSize },
      createCodeBlockContent(match[1].trim())
    )
    converted = true

    return false
  })

  if (converted) return true

  const topLevelNodes: Array<{
    node: ProseMirrorNode
    pos: number
  }> = []

  editor.state.doc.forEach((node, offset) => {
    topLevelNodes.push({ node, pos: offset + 1 })
  })

  for (let start = 0; start < topLevelNodes.length; start += 1) {
    const startText = getNodeTextWithBreaks(topLevelNodes[start].node).trim()
    if (topLevelNodes[start].node.type.name !== 'paragraph' || !/^```[^\n]*$/.test(startText)) {
      continue
    }

    for (let end = start + 1; end < topLevelNodes.length; end += 1) {
      const endText = getNodeTextWithBreaks(topLevelNodes[end].node).trim()

      if (topLevelNodes[end].node.type.name !== 'paragraph' || endText !== '```') continue

      const code = topLevelNodes
        .slice(start + 1, end)
        .map(item => getNodeTextWithBreaks(item.node))
        .join('\n')
        .trim()

      editor.commands.insertContentAt(
        { from: topLevelNodes[start].pos, to: topLevelNodes[end].pos + topLevelNodes[end].node.nodeSize },
        createCodeBlockContent(code)
      )

      return true
    }
  }

  return false
}
