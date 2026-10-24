import { useTranslation } from 'react-i18next'
import { ExternalLink } from 'lucide-react'
import { SettingsContentLayout } from '@/components/settings/common/SettingsContentLayout'
import { SettingsPanelHeader } from '@/components/settings/common/SettingsPanelHeader'

export function AboutSettingsView(props: { version: string }) {
  const { t } = useTranslation()

  return (
    <SettingsContentLayout
      header={
        <SettingsPanelHeader
          title="Fire Poker"
          description={t('settingsPage.about.description')}
          aside={<span className="rounded-full bg-primary/10 px-3 py-1 text-sm text-primary">{props.version}</span>}
        />
      }
    >
      <section className="my-8 rounded-xl border border-border bg-card p-6" aria-labelledby="original-project-title">
        <h2 id="original-project-title" className="text-base font-semibold text-foreground">
          {t('settingsPage.about.originalProjectTitle')}
        </h2>
        <p className="mt-3 text-sm leading-7 text-muted-foreground">
          {t('settingsPage.about.originalProjectThanks')}
        </p>
        <a
          href="https://github.com/MikeLINGxZ/lemontea"
          target="_blank"
          rel="noopener noreferrer"
          className="mt-4 inline-flex items-center gap-2 rounded-sm text-sm font-medium text-primary underline-offset-4 hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          Lemontea · MikeLINGxZ/lemontea
          <ExternalLink className="h-4 w-4" aria-hidden="true" />
        </a>
      </section>
    </SettingsContentLayout>
  )
}
