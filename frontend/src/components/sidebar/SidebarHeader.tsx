import { PanelLeftClose } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { isMac } from '@/lib/utils'

interface SidebarHeaderProps {
  onCollapse: () => void
}

export function SidebarHeader({ onCollapse }: SidebarHeaderProps) {
  const { t } = useTranslation()
  return (
    <div className="desktop-sidebar-header flex h-12 items-center gap-2.5 px-4 select-none shrink-0">
      {!isMac && (
        <>
          <div className="group cursor-pointer">
            <img src="/brand/fire-poker.svg" alt="" width="32" height="32" className="brand-pixel-icon shrink-0" />
          </div>
          <span className="text-lg font-semibold tracking-tight text-foreground">
            Fire Poker
          </span>
        </>
      )}
      <button
        type="button"
        aria-label={t('sidebar.collapse')}
        onClick={onCollapse}
        className="ml-auto rounded-lg p-1.5 text-muted-foreground transition-colors hover:bg-accent hover:text-foreground"
      >
        <PanelLeftClose size={16} />
      </button>
    </div>
  )
}
