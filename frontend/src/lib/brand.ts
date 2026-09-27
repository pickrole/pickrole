// Shared by the opening screen and the About dialog.
import { t } from './i18n/index.svelte'

/** One entry per line: the break is fixed so nothing reflows while it is typed. */
export const tagline = (): string[] => [t('brand.tagline.1'), t('brand.tagline.2')]
