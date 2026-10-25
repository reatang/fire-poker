import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Events } from '@wailsio/runtime'
import { Runtime } from '@bindings/github.com/reatang/fire-poker/backend/service/runtime'
import { useAlertStore } from '@/alert/store'

type RuntimePhase = 'download' | 'extract' | 'verify'

type RuntimeProgressEvent = {
  phase: RuntimePhase
  received: number
  total: number
  percent: number
}

const RUNTIME_TOAST_KEY = 'plugins-runtime-download-toast'

// useRuntimeDownloadReminder offers the Node runtime download while the plugins tab is open and
// mirrors download progress into a single replaceable toast.
export function useRuntimeDownloadReminder(active: boolean) {
  const { t } = useTranslation()
  const pushAlert = useAlertStore((state) => state.pushAlert)

  useEffect(() => {
    let cancelled = false

    if (!active) {
      return
    }

    Runtime.GetStatus({})
      .then((result) => {
        if (cancelled || !result || !shouldShowRuntimeReminder(result.state)) {
          return
        }

        pushAlert({
          kind: 'info',
          placement: 'toast',
          title: t('onboarding.runtime.title'),
          message: t('onboarding.runtime.intro'),
          dedupeKey: RUNTIME_TOAST_KEY,
          dedupeBehavior: 'replace',
          showCount: false,
          autoClose: false,
          actions: [
            {
              id: 'download-runtime-later',
              label: t('onboarding.actions.downloadLater'),
              style: 'secondary',
              closeOnClick: true,
            },
            {
              id: 'download-runtime-now',
              label: t('onboarding.actions.downloadNow'),
              style: 'primary',
              closeOnClick: false,
              onClick: () => {
                pushAlert({
                  kind: 'info',
                  placement: 'toast',
                  title: t('onboarding.runtime.title'),
                  message: buildRuntimeProgressMessage(t, {
                    phase: 'download',
                    received: 0,
                    total: 0,
                    percent: 0,
                  }),
                  detail: null,
                  dedupeKey: RUNTIME_TOAST_KEY,
                  dedupeBehavior: 'replace',
                  showCount: false,
                  autoClose: false,
                  actions: [],
                })
                void Runtime.DownloadNode({})
              },
            },
          ],
        })
      })
      .catch(() => undefined)

    return () => {
      cancelled = true
    }
  }, [active, pushAlert, t])

  useEffect(() => {
    if (!active) {
      return
    }

    const off = Events.On('runtime.node.progress', (event: { data: RuntimeProgressEvent }) => {
      const progress = event.data
      pushAlert({
        kind: 'info',
        placement: 'toast',
        title: t('onboarding.runtime.title'),
        message: buildRuntimeProgressMessage(t, progress),
        detail: null,
        dedupeKey: RUNTIME_TOAST_KEY,
        dedupeBehavior: 'replace',
        showCount: false,
        autoClose: false,
        actions: [],
      })

      if (progress.phase === 'verify' && progress.percent === 100) {
        Runtime.GetStatus({})
          .then((result) => {
            if (!result || result.state !== 'ready') {
              return
            }

            pushAlert({
              kind: 'success',
              placement: 'toast',
              title: t('onboarding.runtime.title'),
              message: t('onboarding.runtime.stateReady'),
              detail: result.version || null,
              dedupeKey: RUNTIME_TOAST_KEY,
              dedupeBehavior: 'replace',
              showCount: false,
              autoClose: true,
              durationMs: 4000,
              actions: [],
            })
          })
          .catch(() => undefined)
      }
    })

    return () => {
      off()
    }
  }, [active, pushAlert, t])
}

function buildRuntimeProgressDetail(
  t: (key: string) => string,
  progress: RuntimeProgressEvent,
): string {
  return [
    t(`onboarding.runtime.phase${capitalize(progress.phase)}`),
    `${formatBytes(progress.received)} / ${progress.total > 0 ? formatBytes(progress.total) : '…'}`,
  ].join(' · ')
}

function buildRuntimeProgressMessage(
  t: (key: string) => string,
  progress: RuntimeProgressEvent,
): string {
  return `${t('onboarding.runtime.stateDownloading')} ${progress.percent}% · ${buildRuntimeProgressDetail(t, progress)}`
}

function shouldShowRuntimeReminder(state: string): boolean {
  return state === 'pending_later' || state === 'missing'
}

function capitalize(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1)
}

function formatBytes(value: number): string {
  if (value <= 0) {
    return '0 B'
  }

  const units = ['B', 'KB', 'MB', 'GB']
  let unitIndex = 0
  let current = value

  while (current >= 1024 && unitIndex < units.length - 1) {
    current /= 1024
    unitIndex += 1
  }

  return `${current.toFixed(1)} ${units[unitIndex]}`
}
