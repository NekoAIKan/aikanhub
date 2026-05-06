import { useMemo, useState } from 'react'
import axios from 'axios'
import { useMutation } from '@tanstack/react-query'
import { AlertTriangle, Download, FileJson, Upload } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  exportConfigBundle,
  importConfigBundle,
  previewConfigBundleImport,
} from '../api'
import { SettingsSection } from '../components/settings-section'
import { tryJsonParse } from '../utils/json-parser'

const SENSITIVE_KEY_PATTERNS = [
  'secret',
  'token',
  'key',
  'password',
  'private',
  'webhook',
  'cert',
]

function isRouteUnavailable(error: unknown) {
  if (!axios.isAxiosError(error)) return false
  return error.response?.status === 404 || error.response?.status === 405
}

function stringify(value: unknown) {
  return JSON.stringify(value, null, 2)
}

function collectKeys(value: unknown, prefix = '', result: string[] = []) {
  if (!value || typeof value !== 'object') return result
  if (Array.isArray(value)) {
    value.forEach((item, index) =>
      collectKeys(item, `${prefix}[${index}]`, result)
    )
    return result
  }
  Object.entries(value).forEach(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    result.push(path)
    collectKeys(child, path, result)
  })
  return result
}

function sensitiveKeys(value: unknown) {
  return collectKeys(value).filter((key) =>
    SENSITIVE_KEY_PATTERNS.some((pattern) =>
      key.toLowerCase().includes(pattern)
    )
  )
}

function downloadJson(data: unknown) {
  const blob = new Blob([stringify(data)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `config-bundle-${new Date().toISOString().slice(0, 10)}.json`
  document.body.append(anchor)
  anchor.click()
  anchor.remove()
  URL.revokeObjectURL(url)
}

function extractPreviewSummary(preview: unknown) {
  if (!preview || typeof preview !== 'object') return []
  const source = preview as Record<string, unknown>
  const candidateKeys = [
    'changed',
    'changes',
    'created',
    'updated',
    'removed',
    'dangerous',
    'sensitive',
  ]

  return candidateKeys.flatMap((key) => {
    const value = source[key]
    if (!value) return []
    if (Array.isArray(value)) {
      return value.slice(0, 8).map((item) => `${key}: ${String(item)}`)
    }
    if (typeof value === 'object') {
      return Object.keys(value as Record<string, unknown>)
        .slice(0, 8)
        .map((item) => `${key}: ${item}`)
    }
    return [`${key}: ${String(value)}`]
  })
}

export function ConfigBundlesSection() {
  const { t } = useTranslation()
  const [bundleText, setBundleText] = useState('')
  const [bundlePayload, setBundlePayload] = useState<unknown>(null)
  const [preview, setPreview] = useState<unknown>(null)

  const parsed = useMemo(() => tryJsonParse(bundleText), [bundleText])
  const warnings = useMemo(
    () => (parsed.success ? sensitiveKeys(parsed.data).slice(0, 12) : []),
    [parsed]
  )
  const previewSummary = useMemo(
    () => extractPreviewSummary(preview),
    [preview]
  )

  const exportMutation = useMutation({
    mutationFn: exportConfigBundle,
    onSuccess: (data) => {
      if (!data.success || data.data === undefined) {
        toast.error(data.message || t('Failed to export config bundle'))
        return
      }
      downloadJson(data.data)
      toast.success(t('Config bundle exported'))
    },
    onError: (error: Error) => {
      if (isRouteUnavailable(error)) {
        toast.error(t('Config bundle backend unavailable'))
        return
      }
      toast.error(error.message || t('Failed to export config bundle'))
    },
  })

  const previewMutation = useMutation({
    mutationFn: previewConfigBundleImport,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to preview import'))
        return
      }
      setPreview(data.data ?? {})
      toast.success(t('Import preview ready'))
    },
    onError: (error: Error) => {
      if (isRouteUnavailable(error)) {
        toast.error(t('Config bundle backend unavailable'))
        return
      }
      toast.error(error.message || t('Failed to preview import'))
    },
  })

  const importMutation = useMutation({
    mutationFn: importConfigBundle,
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to import config bundle'))
        return
      }
      toast.success(t('Config bundle imported'))
      setPreview(null)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to import config bundle'))
    },
  })

  const handleFile = (file: File | undefined) => {
    if (!file) return
    const reader = new FileReader()
    reader.onload = () => {
      setBundleText(String(reader.result ?? ''))
      setPreview(null)
    }
    reader.readAsText(file)
  }

  const handlePreview = () => {
    if (!parsed.success) {
      toast.error(t('Invalid JSON bundle'))
      return
    }
    setBundlePayload(parsed.data)
    previewMutation.mutate(parsed.data)
  }

  return (
    <SettingsSection
      title={t('Config Bundles')}
      description={t(
        'Export, preview, and import system configuration bundles.'
      )}
    >
      <div className='grid gap-6 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'>
        <div className='space-y-4'>
          <div className='rounded-lg border p-4'>
            <div className='mb-4 flex items-start justify-between gap-4'>
              <div>
                <div className='flex items-center gap-2 text-sm font-medium'>
                  <Download className='text-muted-foreground h-4 w-4' />
                  {t('Export bundle')}
                </div>
                <p className='text-muted-foreground mt-1 text-sm'>
                  {t(
                    'Create a backend-generated snapshot of supported options.'
                  )}
                </p>
              </div>
              <Button
                type='button'
                onClick={() => exportMutation.mutate()}
                disabled={exportMutation.isPending}
              >
                {exportMutation.isPending ? t('Exporting...') : t('Export')}
              </Button>
            </div>

            <div className='rounded-lg border px-3 py-2'>
              <div>
                <div className='text-sm font-medium'>
                  {t('Safe export only')}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {t('Sensitive values are excluded from exports.')}
                </div>
              </div>
            </div>
          </div>

          <Alert>
            <AlertTriangle className='h-4 w-4' />
            <AlertTitle>{t('Dangerous operation')}</AlertTitle>
            <AlertDescription>
              {t(
                'Always preview imports before applying them. Sensitive keys and provider credentials can affect live traffic.'
              )}
            </AlertDescription>
          </Alert>
        </div>

        <div className='space-y-4'>
          <div className='rounded-lg border p-4'>
            <div className='mb-4 flex items-center justify-between gap-3'>
              <div className='flex items-center gap-2 text-sm font-medium'>
                <Upload className='text-muted-foreground h-4 w-4' />
                {t('Import bundle')}
              </div>
              <Input
                type='file'
                accept='application/json,.json'
                className='max-w-64'
                onChange={(event) => handleFile(event.target.files?.[0])}
              />
            </div>

            <Textarea
              className='min-h-64 font-mono text-xs'
              spellCheck={false}
              value={bundleText}
              onChange={(event) => {
                setBundleText(event.target.value)
                setPreview(null)
              }}
              placeholder={t('Paste a config bundle JSON here.')}
            />

            <div className='mt-4 flex flex-wrap items-center gap-2'>
              <Button
                type='button'
                variant='outline'
                onClick={handlePreview}
                disabled={previewMutation.isPending || bundleText.trim() === ''}
              >
                <FileJson className='h-4 w-4' />
                {previewMutation.isPending
                  ? t('Previewing...')
                  : t('Preview import')}
              </Button>

              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button
                    type='button'
                    disabled={!preview || importMutation.isPending}
                  >
                    {importMutation.isPending
                      ? t('Importing...')
                      : t('Apply import')}
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      {t('Apply config bundle import?')}
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      {t(
                        'This will update persisted system options using the backend import endpoint.'
                      )}
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() => importMutation.mutate(bundlePayload)}
                    >
                      {t('Apply import')}
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </div>
          </div>

          {warnings.length > 0 && (
            <Alert variant='destructive'>
              <AlertTriangle className='h-4 w-4' />
              <AlertTitle>{t('Sensitive keys detected')}</AlertTitle>
              <AlertDescription>
                <div className='mt-2 flex flex-wrap gap-2'>
                  {warnings.map((key) => (
                    <Badge key={key} variant='secondary'>
                      {key}
                    </Badge>
                  ))}
                </div>
              </AlertDescription>
            </Alert>
          )}

          {preview !== null && (
            <div className='rounded-lg border p-4'>
              <div className='mb-3 text-sm font-medium'>
                {t('Import preview')}
              </div>
              {previewSummary.length > 0 && (
                <div className='mb-3 flex flex-wrap gap-2'>
                  {previewSummary.map((item) => (
                    <Badge key={item} variant='outline'>
                      {item}
                    </Badge>
                  ))}
                </div>
              )}
              <pre className='bg-muted max-h-80 overflow-auto rounded-md p-3 text-xs'>
                {stringify(preview)}
              </pre>
            </div>
          )}
        </div>
      </div>
    </SettingsSection>
  )
}
