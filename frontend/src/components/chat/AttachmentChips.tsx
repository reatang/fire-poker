import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { File as FileBinding } from '@bindings/github.com/reatang/fire-poker/backend/service/file'
import { FileText, Image as ImageIcon, Paperclip, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { isImageMime, isPdfMime } from '@/lib/attachments'
import type { Attachment } from '@/types'

interface Props {
  items: Attachment[]
  variant: 'input' | 'message'
  onRemove?: (index: number) => void
  onOpen?: (item: Attachment) => void
}

function chipIcon(att: Attachment) {
  if (isImageMime(att.mime)) return <ImageIcon size={12} />
  if (isPdfMime(att.mime)) return <FileText size={12} />
  return <Paperclip size={12} />
}

function AttachmentImage({ attachment, variant }: { attachment: Attachment; variant: Props['variant'] }) {
  const [preview, setPreview] = useState<{ path: string; src: string } | null>(null)

  useEffect(() => {
    let active = true
    setPreview(null)
    void FileBinding.ImagePreview({ path: attachment.path }).then((result) => {
      if (active && result?.dataURL) setPreview({ path: attachment.path, src: result.dataURL })
    }).catch(() => { /* Missing files remain accessible as filename chips. */ })
    return () => { active = false }
  }, [attachment.path])

  if (!preview || preview.path !== attachment.path) return null
  return (
    <img
      src={preview.src}
      alt={attachment.name}
      className={variant === 'message' ? 'max-h-64 max-w-full rounded object-contain' : 'h-16 w-20 rounded object-contain'}
      onError={() => setPreview(null)}
    />
  )
}

export function AttachmentChips({ items, variant, onRemove, onOpen }: Props) {
  const { t } = useTranslation()

  if (items.length === 0) return null

  return (
    <div className="flex flex-wrap gap-1.5 px-3 py-2">
      {items.map((att, index) => (
        variant === 'message' ? (
          <button
            key={`${att.path}-${index}`}
            type="button"
            onClick={() => onOpen?.(att)}
            className={cn(
              'inline-flex flex-col items-start gap-1 max-w-[18rem] rounded-md border border-border bg-muted/60 px-2 py-1 text-xs text-foreground',
              'bg-background/60 hover:bg-background/80 transition-colors'
            )}
            title={att.name}
          >
            {isImageMime(att.mime) && <AttachmentImage attachment={att} variant={variant} />}
            <span className="inline-flex max-w-full items-center gap-1">
              {chipIcon(att)}
              <span className="truncate">{att.name}</span>
            </span>
          </button>
        ) : (
          <span
            key={`${att.path}-${index}`}
            className="inline-flex items-center gap-1 max-w-[18rem] rounded-md border border-border bg-muted/60 px-2 py-1 text-xs text-foreground"
          >
            {isImageMime(att.mime) && <AttachmentImage attachment={att} variant={variant} />}
            {chipIcon(att)}
            <span className="truncate" title={att.name}>{att.name}</span>
            {onRemove && (
            <button
              type="button"
              aria-label={t('chat.attachmentRemove')}
              onClick={() => onRemove(index)}
              className="ml-0.5 rounded text-muted-foreground hover:text-foreground hover:bg-accent p-0.5"
            >
              <X size={10} />
            </button>
            )}
          </span>
        )
      ))}
    </div>
  )
}
