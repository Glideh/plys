import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'

/** Maps an API error to a translated message, falling back to the server message for validation errors. */
export function useErrorMessage() {
  const { t, te } = useI18n()
  return (err: unknown): string => {
    if (!(err instanceof ApiError)) return t('errors.generic')
    if (err.code === 'validation') return err.message
    const key = `errors.${err.code}`
    return te(key) ? t(key, { detail: err.message }) : t('errors.generic')
  }
}
