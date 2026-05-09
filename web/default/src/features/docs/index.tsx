import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { CopyButton } from '@/components/copy-button'
import { PublicLayout } from '@/components/layout'

const ENDPOINT_BASE =
  typeof window !== 'undefined'
    ? window.location.origin
    : 'https://your.kittyvibe.com'

// ============================================================================
// Code samples (literal — code stays in English; example prompts kept in
// Chinese to preserve the model's authentic reference behavior)
// ============================================================================

const CURL_T2V = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "A ginger cat strolls down a Tokyo street at sunset, 4K, cinematic",
    "size": "720p",
    "duration": 5,
    "metadata": { "ratio": "16:9", "generate_audio": true }
  }'`

const CURL_I2V_FIRST = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "Slow camera push-in, subject animates from still to motion",
    "images": ["https://example.com/first.jpg"],
    "size": "720p",
    "duration": 5,
    "metadata": { "ratio": "16:9" }
  }'`

const CURL_I2V_FIRSTLAST = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "Smooth transition from first frame to last frame",
    "metadata": {
      "content": [
        { "type": "image_url", "image_url": { "url": "https://example.com/first.jpg" }, "role": "first_frame" },
        { "type": "image_url", "image_url": { "url": "https://example.com/last.jpg"  }, "role": "last_frame"  }
      ],
      "ratio": "16:9"
    },
    "duration": 5
  }'`

const CURL_MULTIMODAL = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "Use video1 first-person framing throughout, audio1 as the BGM. A POV beverage commercial...",
    "metadata": {
      "content": [
        { "type": "image_url", "image_url": { "url": "https://example.com/pic1.jpg" }, "role": "reference_image" },
        { "type": "image_url", "image_url": { "url": "https://example.com/pic2.jpg" }, "role": "reference_image" },
        { "type": "video_url", "video_url": { "url": "https://example.com/v1.mp4"  }, "role": "reference_video" },
        { "type": "audio_url", "audio_url": { "url": "https://example.com/a1.mp3"  }, "role": "reference_audio" }
      ],
      "ratio": "16:9",
      "generate_audio": true
    },
    "duration": 11
  }'`

const CURL_EDIT = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "Replace the perfume in video1 with the cream from image1; keep camera motion intact",
    "metadata": {
      "content": [
        { "type": "image_url", "image_url": { "url": "https://example.com/cream.jpg" }, "role": "reference_image" },
        { "type": "video_url", "video_url": { "url": "https://example.com/edit.mp4" }, "role": "reference_video" }
      ],
      "ratio": "16:9",
      "generate_audio": true
    },
    "duration": 5
  }'`

const CURL_EXTEND = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "Arched window in video1 opens, enter the gallery, then video2; camera zooms into the painting, then video3",
    "metadata": {
      "content": [
        { "type": "video_url", "video_url": { "url": "https://example.com/v1.mp4" }, "role": "reference_video" },
        { "type": "video_url", "video_url": { "url": "https://example.com/v2.mp4" }, "role": "reference_video" },
        { "type": "video_url", "video_url": { "url": "https://example.com/v3.mp4" }, "role": "reference_video" }
      ],
      "ratio": "16:9",
      "generate_audio": true
    },
    "duration": 8
  }'`

const CURL_WEB_SEARCH = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "doubao-seedance-2-0-260128",
    "prompt": "Macro shot of a glass frog on an emerald leaf; focus shifts from skin to its transparent belly",
    "metadata": {
      "ratio": "16:9",
      "generate_audio": true,
      "tools": [{ "type": "web_search" }]
    },
    "duration": 11
  }'`

// ----------------------------------------------------------------------------
// Pixverse curl examples.
// Same /v1/video/generations endpoint — only the model id and request shape
// differ. Aspect ratio, quality, and duration come either from the model SKU
// suffix (e.g. pixverse-v4.5-720p locks 720p) or from the request body /
// metadata.
// ----------------------------------------------------------------------------

const CURL_PIXVERSE_T2V = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "pixverse-v4.5",
    "prompt": "a corgi wearing sunglasses surfing on a sunset wave, cinematic 4K"
  }'`

const CURL_PIXVERSE_T2V_PINNED = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "pixverse-v4.5-720p",
    "prompt": "a fluffy white cat napping in a sunlit window"
  }'`

const CURL_PIXVERSE_I2V = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "pixverse-v4.5",
    "prompt": "gently animate the dog turning its head",
    "images": ["https://example.com/dog.jpg"]
  }'`

const CURL_PIXVERSE_TRANSITION = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "pixverse-v4.5",
    "prompt": "morph the dog into the cat smoothly",
    "images": [
      "https://example.com/start.jpg",
      "https://example.com/end.jpg"
    ]
  }'`

const CURL_PIXVERSE_FUSION = `curl -X POST ${ENDPOINT_BASE}/v1/video/generations \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "pixverse-v4.5",
    "prompt": "@cat plays with @dog in a sunlit garden",
    "images": [
      "https://example.com/cat.jpg",
      "https://example.com/dog.jpg"
    ],
    "metadata": {
      "image_references": [
        {"type": "subject", "img_id": 0, "ref_name": "cat"},
        {"type": "subject", "img_id": 0, "ref_name": "dog"}
      ]
    }
  }'`

const PY_PIXVERSE_T2V = `# pip install openai
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/v1",
)

video = client.videos.create(
    model="pixverse-v4.5",
    prompt="a corgi wearing sunglasses surfing on a sunset wave",
)
print(video.id)`

const PY_PIXVERSE_I2V = `# pip install openai
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/v1",
)

# images[] accepts URLs or data:image/...;base64,... — the gateway
# uploads them to Pixverse and substitutes img_id automatically.
video = client.videos.create(
    model="pixverse-v4.5",
    prompt="gently animate the dog turning its head",
    extra_body={"images": ["https://example.com/dog.jpg"]},
)
print(video.id)`

const CURL_POLL = `curl ${ENDPOINT_BASE}/v1/video/generations/$TASK_ID \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN"`

const CURL_DOWNLOAD = `curl -o video.mp4 ${ENDPOINT_BASE}/v1/videos/$TASK_ID/content \\
  -H "Authorization: Bearer $KITTYVIBE_TOKEN"`

// ----------------------------------------------------------------------------
// Python SDK examples.
//   - Simple endpoints (text-to-video, image-to-video first-frame, polling)
//     use the official OpenAI SDK pointed at /v1.
//       pip install openai
//   - Endpoints that need the full Volcano Ark `content[]` shape — first/last
//     frame, multi-modal reference, edit, extend, web search — use the
//     official Volcano SDK pointed at /api/v3.
//       pip install volcengine-python-sdk[ark]
// ----------------------------------------------------------------------------

const PY_T2V = `# pip install openai
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/v1",
)

video = client.videos.create(
    model="doubao-seedance-2-0-260128",
    prompt="A ginger cat strolls down a Tokyo street at sunset, 4K, cinematic",
    seconds="5",
    size="720p",
    extra_body={"metadata": {"ratio": "16:9", "generate_audio": True}},
)
print(video.id)`

const PY_I2V_FIRST = `# pip install openai
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/v1",
)

video = client.videos.create(
    model="doubao-seedance-2-0-260128",
    prompt="Slow camera push-in, subject animates from still to motion",
    seconds="5",
    size="720p",
    extra_body={
        "images": ["https://example.com/first.jpg"],
        "metadata": {"ratio": "16:9"},
    },
)
print(video.id)`

const PY_I2V_FIRSTLAST = `# pip install volcengine-python-sdk[ark]
import os
from volcenginesdkarkruntime import Ark

client = Ark(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/api/v3",
)

task = client.content_generation.tasks.create(
    model="doubao-seedance-2-0-260128",
    content=[
        {"type": "text", "text": "Smooth transition from first frame to last frame"},
        {"type": "image_url", "image_url": {"url": "https://example.com/first.jpg"}, "role": "first_frame"},
        {"type": "image_url", "image_url": {"url": "https://example.com/last.jpg"},  "role": "last_frame"},
    ],
    duration=5,
    ratio="16:9",
)
print(task.id)`

const PY_MULTIMODAL = `# pip install volcengine-python-sdk[ark]
import os
from volcenginesdkarkruntime import Ark

client = Ark(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/api/v3",
)

task = client.content_generation.tasks.create(
    model="doubao-seedance-2-0-260128",
    content=[
        {"type": "text", "text": "Use video1 first-person framing throughout, audio1 as the BGM. A POV beverage commercial..."},
        {"type": "image_url", "image_url": {"url": "https://example.com/pic1.jpg"}, "role": "reference_image"},
        {"type": "image_url", "image_url": {"url": "https://example.com/pic2.jpg"}, "role": "reference_image"},
        {"type": "video_url", "video_url": {"url": "https://example.com/v1.mp4"},   "role": "reference_video"},
        {"type": "audio_url", "audio_url": {"url": "https://example.com/a1.mp3"},   "role": "reference_audio"},
    ],
    duration=11,
    ratio="16:9",
    generate_audio=True,
)
print(task.id)`

const PY_EDIT = `# pip install volcengine-python-sdk[ark]
import os
from volcenginesdkarkruntime import Ark

client = Ark(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/api/v3",
)

task = client.content_generation.tasks.create(
    model="doubao-seedance-2-0-260128",
    content=[
        {"type": "text", "text": "Replace the perfume in video1 with the cream from image1; keep camera motion intact"},
        {"type": "image_url", "image_url": {"url": "https://example.com/cream.jpg"}, "role": "reference_image"},
        {"type": "video_url", "video_url": {"url": "https://example.com/edit.mp4"},  "role": "reference_video"},
    ],
    duration=5,
    ratio="16:9",
    generate_audio=True,
)
print(task.id)`

const PY_EXTEND = `# pip install volcengine-python-sdk[ark]
import os
from volcenginesdkarkruntime import Ark

client = Ark(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/api/v3",
)

task = client.content_generation.tasks.create(
    model="doubao-seedance-2-0-260128",
    content=[
        {"type": "text", "text": "Arched window in video1 opens, enter the gallery, then video2; camera zooms into the painting, then video3"},
        {"type": "video_url", "video_url": {"url": "https://example.com/v1.mp4"}, "role": "reference_video"},
        {"type": "video_url", "video_url": {"url": "https://example.com/v2.mp4"}, "role": "reference_video"},
        {"type": "video_url", "video_url": {"url": "https://example.com/v3.mp4"}, "role": "reference_video"},
    ],
    duration=8,
    ratio="16:9",
    generate_audio=True,
)
print(task.id)`

const PY_WEB_SEARCH = `# pip install volcengine-python-sdk[ark]
import os
from volcenginesdkarkruntime import Ark

client = Ark(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/api/v3",
)

task = client.content_generation.tasks.create(
    model="doubao-seedance-2-0-260128",
    content=[
        {"type": "text", "text": "Macro shot of a glass frog on an emerald leaf; focus shifts from skin to its transparent belly"},
    ],
    duration=11,
    ratio="16:9",
    generate_audio=True,
    tools=[{"type": "web_search"}],
)
print(task.id)`

const PY_POLL = `# pip install openai
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["KITTYVIBE_TOKEN"],
    base_url="${ENDPOINT_BASE}/v1",
)

video = client.videos.retrieve("your-task-id-here")
print(video.status, video.model_extra)

# Volcano SDK alternative — exposes content.video_url directly:
# from volcenginesdkarkruntime import Ark
# ark = Ark(api_key=os.environ["KITTYVIBE_TOKEN"], base_url="${ENDPOINT_BASE}/api/v3")
# task = ark.content_generation.tasks.get(task_id="your-task-id-here")
# print(task.status, task.content.video_url)`

const PY_DOWNLOAD = `# pip install httpx
import os, httpx
from pathlib import Path

TASK_ID = "your-task-id-here"
r = httpx.get(
    f"${ENDPOINT_BASE}/v1/videos/{TASK_ID}/content",
    headers={"Authorization": f"Bearer {os.environ['KITTYVIBE_TOKEN']}"},
)
r.raise_for_status()
Path("video.mp4").write_bytes(r.content)
print("saved to video.mp4")`

const PYTHON_FULL = `"""End-to-end example: submit → poll → download (OpenAI SDK + httpx)."""
# pip install openai httpx
import os, time, httpx
from pathlib import Path
from openai import OpenAI

BASE  = "${ENDPOINT_BASE}"
TOKEN = os.environ["KITTYVIBE_TOKEN"]
client = OpenAI(api_key=TOKEN, base_url=f"{BASE}/v1")


def submit_video_task(prompt, *, model="doubao-seedance-2-0-fast-260128",
                      size="720p", seconds="5", ratio="16:9"):
    video = client.videos.create(
        model=model,
        prompt=prompt,
        seconds=seconds,
        size=size,
        extra_body={"metadata": {"ratio": ratio}},
    )
    return video.id


def wait_for_video(task_id, *, interval=5, timeout=600):
    deadline = time.time() + timeout
    while time.time() < deadline:
        time.sleep(interval)
        video = client.videos.retrieve(task_id)
        status = video.status
        print(f"  [{task_id[:12]}] status={status}")
        if status in ("succeeded", "completed"):
            # The video file lives behind /v1/videos/{id}/content; the URL is
            # also stashed on video.metadata for convenience.
            return (video.metadata or {}).get("url", "")
        if status == "failed":
            err = getattr(video, "error", None)
            raise RuntimeError(f"task failed: {err}")
    raise TimeoutError(f"task {task_id} did not complete within {timeout}s")


def download_video(task_id, out_path):
    r = httpx.get(f"{BASE}/v1/videos/{task_id}/content",
                  headers={"Authorization": f"Bearer {TOKEN}"})
    r.raise_for_status()
    Path(out_path).write_bytes(r.content)


if __name__ == "__main__":
    task_id = submit_video_task("A ginger cat strolls down a Tokyo street at sunset, 4K, cinematic")
    print(f"submitted: {task_id}")
    video_url = wait_for_video(task_id)
    print(f"video URL: {video_url}")
    download_video(task_id, "out.mp4")
    print("saved to out.mp4")`

const NODE_FULL = `// End-to-end example: submit -> poll -> download
import { writeFile } from "node:fs/promises";

const BASE  = "${ENDPOINT_BASE}";
const TOKEN = process.env.KITTYVIBE_TOKEN;
const H     = { Authorization: \`Bearer \${TOKEN}\`, "Content-Type": "application/json" };

async function submitVideoTask(prompt, opts = {}) {
  const r = await fetch(\`\${BASE}/v1/video/generations\`, {
    method: "POST",
    headers: H,
    body: JSON.stringify({
      model:    opts.model    ?? "doubao-seedance-2-0-fast-260128",
      prompt,
      size:     opts.size     ?? "720p",
      duration: opts.duration ?? 5,
      metadata: { ratio: opts.ratio ?? "16:9" },
    }),
  });
  if (!r.ok) throw new Error(\`submit failed: \${r.status} \${await r.text()}\`);
  return (await r.json()).task_id;
}

async function waitForVideo(taskId, { interval = 5000, timeout = 600_000 } = {}) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    await new Promise((r) => setTimeout(r, interval));
    const r = await fetch(\`\${BASE}/v1/video/generations/\${taskId}\`, { headers: H });
    const body = await r.json();
    const status = body.status ?? body.data?.status;
    console.log(\`  [\${taskId.slice(0, 12)}] status=\${status}\`);
    if (status === "succeeded" || status === "SUCCESS")
      return body.data.data.content.video_url;
    if (status === "failed" || status === "FAILED")
      throw new Error(\`task failed: \${body.data?.fail_reason ?? "unknown"}\`);
  }
  throw new Error(\`timeout after \${timeout}ms\`);
}

async function downloadVideo(taskId, outPath) {
  const r = await fetch(\`\${BASE}/v1/videos/\${taskId}/content\`, {
    headers: { Authorization: \`Bearer \${TOKEN}\` },
  });
  if (!r.ok) throw new Error(\`download failed: \${r.status}\`);
  await writeFile(outPath, Buffer.from(await r.arrayBuffer()));
}

const taskId = await submitVideoTask("A ginger cat strolls down a Tokyo street at sunset, 4K, cinematic");
console.log("submitted:", taskId);
const videoUrl = await waitForVideo(taskId);
console.log("video URL:", videoUrl);
await downloadVideo(taskId, "out.mp4");
console.log("saved to out.mp4");`

// ============================================================================
// Reusable components
// ============================================================================

function CodeBlock({ code, lang }: { code: string; lang: string }) {
  return (
    <div className='relative'>
      <div className='bg-muted text-muted-foreground border-b px-4 py-2 text-xs font-medium tracking-wider uppercase'>
        {lang}
      </div>
      <pre className='bg-card overflow-x-auto p-4 text-sm leading-relaxed'>
        <code>{code}</code>
      </pre>
      <div className='absolute top-10 right-2'>
        <CopyButton value={code} variant='ghost' size='sm' />
      </div>
    </div>
  )
}

function CodeTabs({ shell, python }: { shell: string; python: string }) {
  return (
    <Tabs defaultValue='shell' className='overflow-hidden rounded-lg border'>
      <TabsList className='bg-muted h-10 w-full justify-start rounded-none border-b px-2'>
        <TabsTrigger value='shell'>Shell</TabsTrigger>
        <TabsTrigger value='python'>Python</TabsTrigger>
      </TabsList>
      <TabsContent value='shell' className='m-0'>
        <CodeBlock lang='shell' code={shell} />
      </TabsContent>
      <TabsContent value='python' className='m-0'>
        <CodeBlock lang='python' code={python} />
      </TabsContent>
    </Tabs>
  )
}

function Section({
  id,
  title,
  description,
  children,
}: {
  id: string
  title: React.ReactNode
  description?: React.ReactNode
  children: React.ReactNode
}) {
  return (
    <section id={id} className='scroll-mt-24 space-y-4'>
      <div className='space-y-1'>
        <h2 className='text-2xl font-semibold tracking-tight'>{title}</h2>
        {description && (
          <div className='text-muted-foreground text-sm'>{description}</div>
        )}
      </div>
      {children}
    </section>
  )
}

function EndpointCard({
  method,
  path,
  description,
}: {
  method: 'GET' | 'POST'
  path: string
  description: React.ReactNode
}) {
  const methodColor =
    method === 'POST'
      ? 'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300'
      : 'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300'
  return (
    <div className='bg-card flex items-start gap-3 rounded-md border p-3'>
      <span
        className={`shrink-0 rounded px-2 py-0.5 font-mono text-xs font-semibold ${methodColor}`}
      >
        {method}
      </span>
      <div className='space-y-1'>
        <code className='font-mono text-sm'>{path}</code>
        <p className='text-muted-foreground text-xs'>{description}</p>
      </div>
    </div>
  )
}

interface Param {
  name: string
  type: string
  required?: boolean
  desc: React.ReactNode
}

function ParamTable({
  params,
  fieldLabel,
  typeLabel,
  requiredLabel,
  descLabel,
  yes,
  no,
}: {
  params: Param[]
  fieldLabel: string
  typeLabel: string
  requiredLabel: string
  descLabel: string
  yes: string
  no: string
}) {
  return (
    <div className='overflow-hidden rounded-lg border'>
      <table className='w-full text-sm'>
        <thead className='bg-muted'>
          <tr className='text-left'>
            <th className='w-44 px-4 py-2 font-medium'>{fieldLabel}</th>
            <th className='w-28 px-4 py-2 font-medium'>{typeLabel}</th>
            <th className='w-20 px-4 py-2 font-medium'>{requiredLabel}</th>
            <th className='px-4 py-2 font-medium'>{descLabel}</th>
          </tr>
        </thead>
        <tbody className='divide-y'>
          {params.map((p) => (
            <tr key={p.name}>
              <td className='px-4 py-2 font-mono text-xs'>{p.name}</td>
              <td className='px-4 py-2 text-xs'>{p.type}</td>
              <td className='px-4 py-2 text-xs'>
                {p.required ? (
                  <span className='font-medium text-red-600 dark:text-red-400'>
                    {yes}
                  </span>
                ) : (
                  <span className='text-muted-foreground'>{no}</span>
                )}
              </td>
              <td className='px-4 py-2 text-xs'>{p.desc}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

function MethodBadge({ method }: { method: 'GET' | 'POST' }) {
  const color =
    method === 'POST'
      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
      : 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-300'
  return (
    <span
      className={`inline-block rounded px-1.5 py-0.5 font-mono text-[10px] font-semibold ${color}`}
    >
      {method}
    </span>
  )
}

function NewBadge() {
  return (
    <span className='ml-2 inline-block rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-300'>
      NEW
    </span>
  )
}

function Callout({
  type = 'info',
  children,
}: {
  type?: 'info' | 'warn' | 'tip'
  children: React.ReactNode
}) {
  const styles = {
    info: 'bg-blue-50 text-blue-900 dark:bg-blue-900/20 dark:text-blue-200',
    warn: 'bg-amber-50 text-amber-900 dark:bg-amber-900/20 dark:text-amber-200',
    tip: 'bg-emerald-50 text-emerald-900 dark:bg-emerald-900/20 dark:text-emerald-200',
  }
  return (
    <div className={`rounded-md p-3 text-xs leading-relaxed ${styles[type]}`}>
      {children}
    </div>
  )
}

function K({ children }: { children: React.ReactNode }) {
  return <code className='bg-muted rounded px-1'>{children}</code>
}

// ============================================================================
// Capability matrix
// ============================================================================

const SEEDANCE_MODELS = [
  { id: 'doubao-seedance-2-0-260128', name: 'Seedance 2.0' },
  { id: 'doubao-seedance-2-0-fast-260128', name: 'Seedance 2.0 fast' },
  { id: 'doubao-seedance-1-5-pro-251215', name: 'Seedance 1.5 pro' },
  { id: 'doubao-seedance-1-0-pro-250528', name: 'Seedance 1.0 pro' },
  { id: 'doubao-seedance-1-0-pro-fast-251015', name: 'Seedance 1.0 pro fast' },
  { id: 'doubao-seedance-1-0-lite-i2v-250428', name: 'Seedance 1.0 lite i2v' },
  { id: 'doubao-seedance-1-0-lite-t2v-250428', name: 'Seedance 1.0 lite t2v' },
] as const

type Cap = '✅' | '❌'
const Y: Cap = '✅'
const N: Cap = '❌'

// ============================================================================
// Scroll-spy: track which section is currently in view, then determine which
// nav group it belongs to.
// ============================================================================

function useActiveSection(sectionIds: string[]): string | null {
  const [active, setActive] = useState<string | null>(sectionIds[0] ?? null)

  useEffect(() => {
    if (typeof window === 'undefined') return
    const elements = sectionIds
      .map((id) => document.getElementById(id))
      .filter((el): el is HTMLElement => !!el)

    if (elements.length === 0) return

    // Track visible sections; pick the topmost one above the fold.
    const visible = new Map<string, number>()

    const observer = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) {
            visible.set(e.target.id, e.boundingClientRect.top)
          } else {
            visible.delete(e.target.id)
          }
        }
        if (visible.size > 0) {
          // Section closest to (but above) the top of viewport is active.
          let best: string | null = null
          let bestTop = -Infinity
          for (const [id, top] of visible) {
            if (top <= 120 && top > bestTop) {
              best = id
              bestTop = top
            }
          }
          // Fallback: smallest positive top
          if (!best) {
            let smallest = Infinity
            for (const [id, top] of visible) {
              if (top >= 0 && top < smallest) {
                best = id
                smallest = top
              }
            }
          }
          if (best) setActive(best)
        }
      },
      { rootMargin: '-80px 0px -50% 0px', threshold: [0, 0.25, 0.5, 1] }
    )

    elements.forEach((el) => observer.observe(el))
    return () => observer.disconnect()
  }, [sectionIds])

  return active
}

// ============================================================================
// Page
// ============================================================================

export function Docs() {
  const { t } = useTranslation()

  const NAV_GROUPS: Array<{
    label: string
    items: Array<{
      id: string
      label: string
      method?: 'GET' | 'POST'
      isNew?: boolean
    }>
  }> = useMemo(
    () => [
      {
        label: t('Product basics'),
        items: [
          { id: 'overview', label: t('Introduction') },
          { id: 'quick-start', label: t('Quick start') },
          { id: 'auth', label: t('Authentication') },
          { id: 'endpoints', label: t('Endpoint reference') },
        ],
      },
      {
        label: t('Video API · Seedance'),
        items: [
          { id: 'seedance-overview', label: t('Seedance overview') },
          { id: 'models', label: t('Models & capabilities') },
          { id: 'mode-text', label: t('Text-to-video'), method: 'POST' },
          {
            id: 'mode-image-first',
            label: t('Image-to-video · first frame'),
            method: 'POST',
          },
          {
            id: 'mode-image-firstlast',
            label: t('Image-to-video · first/last frame'),
            method: 'POST',
          },
          {
            id: 'mode-multimodal',
            label: t('Multi-modal reference'),
            method: 'POST',
            isNew: true,
          },
          {
            id: 'mode-edit',
            label: t('Edit video'),
            method: 'POST',
            isNew: true,
          },
          {
            id: 'mode-extend',
            label: t('Extend video'),
            method: 'POST',
            isNew: true,
          },
          {
            id: 'mode-web-search',
            label: t('Web search augmented'),
            method: 'POST',
            isNew: true,
          },
          { id: 'params', label: t('Full request parameters') },
          { id: 'media-limits', label: t('Input file limits') },
        ],
      },
      {
        label: t('Video API · Pixverse'),
        items: [
          { id: 'pixverse-overview', label: t('Pixverse overview') },
          { id: 'pixverse-models', label: t('Pixverse models & pricing') },
          {
            id: 'pixverse-mode-text',
            label: t('Text-to-video'),
            method: 'POST',
          },
          {
            id: 'pixverse-mode-image',
            label: t('Image-to-video'),
            method: 'POST',
          },
          {
            id: 'pixverse-mode-transition',
            label: t('First/last frame'),
            method: 'POST',
          },
          {
            id: 'pixverse-mode-fusion',
            label: t('Multi-reference (fusion)'),
            method: 'POST',
            isNew: true,
          },
          { id: 'pixverse-params', label: t('Pixverse parameters') },
          { id: 'pixverse-errors', label: t('Pixverse error codes') },
        ],
      },
      {
        label: t('Tasks & results'),
        items: [
          { id: 'poll', label: t('Query a task'), method: 'GET' },
          { id: 'download', label: t('Download a video'), method: 'GET' },
          { id: 'full-example', label: t('End-to-end example') },
        ],
      },
      {
        label: t('Appendix'),
        items: [
          { id: 'pricing', label: t('Pricing') },
          { id: 'limits', label: t('Rate limits & quotas') },
          { id: 'errors', label: t('Error codes') },
          { id: 'best-practices', label: t('Best practices') },
          { id: 'faq', label: t('FAQ') },
        ],
      },
    ],
    [t]
  )

  const allIds = useMemo(
    () => NAV_GROUPS.flatMap((g) => g.items.map((i) => i.id)),
    [NAV_GROUPS]
  )
  const activeId = useActiveSection(allIds)
  const activeGroup =
    NAV_GROUPS.find((g) => g.items.some((i) => i.id === activeId)) ??
    NAV_GROUPS[0]

  // ---- Capability matrix rows (built inside component so labels can use t()) ----
  const CAPABILITY_ROWS: Array<{ label: React.ReactNode; values: Cap[] }> = [
    { label: t('Text to video'), values: [Y, Y, Y, Y, Y, N, Y] },
    { label: t('Image to video · first frame'), values: [Y, Y, Y, Y, Y, Y, N] },
    {
      label: t('Image to video · first/last frame'),
      values: [Y, Y, Y, Y, N, Y, N],
    },
    {
      label: (
        <>
          {t('Multi-modal · image reference')}
          <NewBadge />
        </>
      ),
      values: [Y, Y, N, N, N, Y, N],
    },
    {
      label: (
        <>
          {t('Multi-modal · video reference')}
          <NewBadge />
        </>
      ),
      values: [Y, Y, N, N, N, N, N],
    },
    {
      label: (
        <>
          {t('Multi-modal · combined reference')}
          <NewBadge />
        </>
      ),
      values: [Y, Y, N, N, N, N, N],
    },
    {
      label: (
        <>
          {t('Edit video')}
          <NewBadge />
        </>
      ),
      values: [Y, N, N, N, N, N, N],
    },
    {
      label: (
        <>
          {t('Extend video')}
          <NewBadge />
        </>
      ),
      values: [Y, N, N, N, N, N, N],
    },
    { label: t('Audio generation'), values: [Y, Y, N, N, N, N, N] },
    {
      label: (
        <>
          {t('Web search augmented')}
          <NewBadge />
        </>
      ),
      values: [Y, N, N, N, N, N, N],
    },
    { label: t('Returns last frame'), values: [Y, Y, Y, Y, Y, Y, Y] },
  ]

  const SPEC_ROWS: Array<{ label: string; values: string[] }> = [
    {
      label: t('Output resolution'),
      values: [
        '480p / 720p',
        '480p / 720p / 1080p',
        '480p / 720p / 1080p',
        '480p / 720p / 1080p',
        '480p / 720p / 1080p',
        '480p / 720p / 1080p',
        '480p / 720p / 1080p',
      ],
    },
    {
      label: t('Aspect ratio'),
      values: Array(7).fill('21:9 / 16:9 / 4:3 / 1:1 / 3:4 / 9:16'),
    },
    {
      label: t('Output duration'),
      values: [
        t('4–15 s'),
        t('4–12 s'),
        t('2–12 s'),
        t('2–12 s'),
        t('2–12 s'),
        t('2–12 s'),
        t('2–12 s'),
      ],
    },
    {
      label: t('RPM (online)'),
      values: ['600', '600', '600', '600', '600', '300', '300'],
    },
    {
      label: t('Concurrency (online)'),
      values: ['10', '10', '10', '10', '10', '5', '5'],
    },
  ]

  return (
    <PublicLayout showMainContainer={false}>
      <div className='docs-font'>
        {/* Outer wrapper provides padding-top equal to the fixed PublicHeader height. */}
        <div className='pt-16'>
          <div className='mx-auto flex max-w-[1400px] gap-6 px-4 lg:px-6'>
            {/* ===================== Left sidebar (page nav) ===================== */}
            <aside className='sticky top-16 hidden h-[calc(100vh-4rem)] w-60 shrink-0 overflow-y-auto py-8 pr-2 lg:block'>
              <nav className='space-y-5'>
                {NAV_GROUPS.map((group) => (
                  <div key={group.label} className='space-y-1.5'>
                    <a
                      href={`#${group.items[0].id}`}
                      className='text-foreground hover:text-primary block text-xs font-semibold tracking-wider uppercase transition-colors'
                    >
                      {group.label}
                    </a>
                    <div className='flex flex-col gap-0.5'>
                      {group.items.map((item) => {
                        const isActive = item.id === activeId
                        return (
                          <a
                            key={item.id}
                            href={`#${item.id}`}
                            className={`group flex items-center gap-1.5 rounded px-2 py-1 text-sm transition-colors ${
                              isActive
                                ? 'bg-muted text-primary font-medium'
                                : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                            }`}
                          >
                            {item.method && (
                              <MethodBadge method={item.method} />
                            )}
                            <span className='truncate'>{item.label}</span>
                            {item.isNew && (
                              <span className='ml-auto rounded bg-amber-100 px-1 text-[9px] font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-300'>
                                NEW
                              </span>
                            )}
                          </a>
                        )
                      })}
                    </div>
                  </div>
                ))}
              </nav>
            </aside>

            {/* ===================== Main content ===================== */}
            <main className='min-w-0 flex-1 py-8'>
              <header className='mb-10 space-y-3 border-b pb-6'>
                <h1 className='text-3xl font-semibold tracking-tight'>
                  {t('API Documentation')}
                </h1>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'KittyVibe video generation API — call mainstream video models with one token, OpenAI-style async tasks.'
                  )}
                </p>
              </header>

              <article className='space-y-12'>
                {/* ============================ Product basics ============================ */}
                <Section
                  id='overview'
                  title={t('Introduction')}
                  description={t(
                    'In one sentence: use a single sk- key to drive Seedance and other mainstream video generation models.'
                  )}
                >
                  <p className='text-sm leading-relaxed'>
                    {t(
                      'KittyVibe is a unified video generation API gateway. All supported models are called through the same set of endpoints — one token, no per-vendor signups, no SDK juggling, no fragmented invoices.'
                    )}
                  </p>
                  <ul className='text-muted-foreground list-disc space-y-1 pl-6 text-sm'>
                    <li>
                      {t('Async task model: submit and receive a')}{' '}
                      <K>task_id</K>, {t('then poll until it finishes.')}
                    </li>
                    <li>
                      {t('OpenAI-style auth (')}
                      <K>Authorization: Bearer sk-...</K>
                      {t(').')}
                    </li>
                    <li>
                      {t(
                        'Videos are served from KittyVibe-signed URLs so storage links never leak to clients.'
                      )}
                    </li>
                    <li>
                      {t('Unified billing per video — see')}{' '}
                      <a
                        href='#pricing'
                        className='text-primary hover:underline'
                      >
                        {t('Pricing')}
                      </a>
                      .
                    </li>
                  </ul>
                </Section>

                <Section
                  id='quick-start'
                  title={t('Quick start')}
                  description={t(
                    'Three steps to your first video generation request.'
                  )}
                >
                  <ol className='space-y-5'>
                    <li className='space-y-2'>
                      <h3 className='font-medium'>
                        {t('1. Create an API key')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t('Sign in, open the')}{' '}
                        <a
                          href='/keys'
                          className='text-primary hover:underline'
                        >
                          {t('Tokens')}
                        </a>{' '}
                        {t(
                          'page, click "Create API key", then copy the string starting with'
                        )}{' '}
                        <K>sk-</K>.
                      </p>
                    </li>
                    <li className='space-y-2'>
                      <h3 className='font-medium'>
                        {t('2. Set environment variable')}
                      </h3>
                      <CodeBlock
                        lang='shell'
                        code={`export KITTYVIBE_TOKEN=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`}
                      />
                    </li>
                    <li className='space-y-2'>
                      <h3 className='font-medium'>
                        {t('3. Submit a text-to-video task')}
                      </h3>
                      <CodeTabs shell={CURL_T2V} python={PY_T2V} />
                      <p className='text-muted-foreground text-xs'>
                        {t('A successful response includes a')} <K>task_id</K>.{' '}
                        {t('Use it to')}{' '}
                        <a
                          href='#poll'
                          className='text-primary hover:underline'
                        >
                          {t('check progress')}
                        </a>
                        , {t('then')}{' '}
                        <a
                          href='#download'
                          className='text-primary hover:underline'
                        >
                          {t('download the video')}
                        </a>
                        . {t('The full code lives in')}{' '}
                        <a
                          href='#full-example'
                          className='text-primary hover:underline'
                        >
                          {t('End-to-end example')}
                        </a>
                        .
                      </p>
                    </li>
                  </ol>
                </Section>

                <Section
                  id='auth'
                  title={t('Authentication')}
                  description={t(
                    'All endpoints use Bearer Token authentication.'
                  )}
                >
                  <CodeBlock
                    lang='shell'
                    code={`Authorization: Bearer $KITTYVIBE_TOKEN`}
                  />
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'A single token can call every supported model. Per-token model scope, quota, and concurrency limits can be configured on the'
                    )}{' '}
                    <a href='/keys' className='text-primary hover:underline'>
                      {t('Tokens')}
                    </a>{' '}
                    {t('page.')}
                  </p>
                </Section>

                <Section id='endpoints' title={t('Endpoint reference')}>
                  <div className='space-y-2'>
                    <EndpointCard
                      method='POST'
                      path='/v1/video/generations'
                      description={t(
                        'Submit a video generation task; returns task_id.'
                      )}
                    />
                    <EndpointCard
                      method='GET'
                      path='/v1/video/generations/:task_id'
                      description={t(
                        'Query task status, progress, and final video URL.'
                      )}
                    />
                    <EndpointCard
                      method='GET'
                      path='/v1/videos/:task_id/content'
                      description={t(
                        'Download or preview the video file (valid for 24h).'
                      )}
                    />
                  </div>
                </Section>

                {/* ============================ Seedance ============================ */}
                <Section
                  id='seedance-overview'
                  title={t('Seedance overview')}
                  description={t(
                    'Seedance is a high-end video generation model series. KittyVibe supports every Seedance call mode: text-to-video, image-to-video (first / first+last frame), multi-modal reference, edit video, extend video — plus audio generation and web search augmentation.'
                  )}
                >
                  <Callout type='tip'>
                    <strong>{t('Choosing a model:')}</strong> {t('use')}{' '}
                    <K>doubao-seedance-2-0-260128</K>{' '}
                    {t('for the highest quality;')}{' '}
                    <K>doubao-seedance-2-0-fast-260128</K>{' '}
                    {t(
                      'when speed and cost matter most; the 1.x series for basic text/image-to-video only.'
                    )}
                  </Callout>
                  <Callout type='info'>
                    <strong>
                      {t('Three mutually exclusive input scenarios')}
                    </strong>{' '}
                    — {t("don't mix them:")}
                    <ul className='mt-2 list-disc space-y-1 pl-5'>
                      <li>
                        <strong>{t('Image-to-video · first frame')}</strong>:{' '}
                        {t('one image, one output video.')}
                      </li>
                      <li>
                        <strong>
                          {t('Image-to-video · first/last frame')}
                        </strong>
                        : {t('two images (first and last).')}
                      </li>
                      <li>
                        <strong>{t('Multi-modal reference')}</strong>:{' '}
                        {t(
                          'any combination of images (0–9), videos (0–3), audio (0–3), plus text prompt.'
                        )}
                      </li>
                    </ul>
                    {t(
                      'Note: audio cannot be the only input — it must accompany at least one reference video or image.'
                    )}
                  </Callout>
                </Section>

                <Section id='models' title={t('Models & capabilities')}>
                  <div className='overflow-x-auto rounded-lg border'>
                    <table className='min-w-full text-xs'>
                      <thead className='bg-muted'>
                        <tr>
                          <th className='bg-muted sticky left-0 px-3 py-2 text-left font-medium'>
                            {t('Capability / spec')}
                          </th>
                          {SEEDANCE_MODELS.map((m) => (
                            <th
                              key={m.id}
                              className='px-3 py-2 text-left font-medium whitespace-nowrap'
                            >
                              <div className='font-semibold'>{m.name}</div>
                              <div className='text-muted-foreground font-mono text-[10px]'>
                                {m.id}
                              </div>
                            </th>
                          ))}
                        </tr>
                      </thead>
                      <tbody className='divide-y'>
                        {CAPABILITY_ROWS.map((row, i) => (
                          <tr key={i}>
                            <td className='bg-card sticky left-0 px-3 py-2 text-left'>
                              {row.label}
                            </td>
                            {row.values.map((v, j) => (
                              <td key={j} className='px-3 py-2 text-center'>
                                {v}
                              </td>
                            ))}
                          </tr>
                        ))}
                        {SPEC_ROWS.map((row, i) => (
                          <tr key={`spec-${i}`} className='bg-muted/30'>
                            <td className='bg-muted/30 sticky left-0 px-3 py-2 text-left font-medium'>
                              {row.label}
                            </td>
                            {row.values.map((v, j) => (
                              <td
                                key={j}
                                className='px-3 py-2 whitespace-nowrap'
                              >
                                {v}
                              </td>
                            ))}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t('Output format is always mp4.')}
                  </p>
                </Section>

                {/* ============================ Call modes ============================ */}
                <Section
                  id='mode-text'
                  title={t('Text-to-video')}
                  description={t(
                    'POST /v1/video/generations · text prompt only. Supported by every Seedance model.'
                  )}
                >
                  <CodeTabs shell={CURL_T2V} python={PY_T2V} />
                  <p className='text-muted-foreground text-sm'>
                    {t('Top-level fields:')} <K>prompt</K> {t('is required;')}{' '}
                    <K>size</K> {t('controls resolution (')}
                    <K>480p</K>/<K>720p</K>/<K>1080p</K>
                    {t(');')} <K>duration</K>{' '}
                    {t('controls length in seconds. Full field list:')}{' '}
                    <a href='#params' className='text-primary hover:underline'>
                      {t('Full request parameters')}
                    </a>
                    .
                  </p>
                </Section>

                <Section
                  id='mode-image-first'
                  title={t('Image-to-video · first frame')}
                  description={t(
                    'Pass one reference image as the first frame, with an optional text prompt.'
                  )}
                >
                  <CodeTabs shell={CURL_I2V_FIRST} python={PY_I2V_FIRST} />
                  <Callout type='info'>
                    {t('Top-level')} <K>images</K>{' '}
                    {t(
                      'is sugar for the first frame — equivalent to passing a single image_url entry with role first_frame inside metadata.content. Use the full structure when you need finer control.'
                    )}
                  </Callout>
                  <Callout type='warn'>
                    <strong>{t('Real-person reference image?')}</strong>{' '}
                    {t('Add')} <K>"metadata": &#123; "audit_image": true &#125;</K>
                    {t(
                      ', otherwise the request will be rejected by the upstream privacy filter. Adds ~10–30 s before task_id is returned.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='mode-image-firstlast'
                  title={t('Image-to-video · first/last frame')}
                  description={t(
                    'Pass two images, marking each with role: first_frame / last_frame.'
                  )}
                >
                  <CodeTabs
                    shell={CURL_I2V_FIRSTLAST}
                    python={PY_I2V_FIRSTLAST}
                  />
                  <p className='text-muted-foreground text-sm'>
                    {t('Both images must declare a')} <K>role</K>{' '}
                    {t('— specifically')} <K>first_frame</K> {t('and')}{' '}
                    <K>last_frame</K>{' '}
                    {t("— otherwise the model can't infer the time order.")}
                  </p>
                  <Callout type='warn'>
                    <strong>{t('Real-person reference image?')}</strong>{' '}
                    {t('Add')} <K>"metadata": &#123; "audit_image": true &#125;</K>
                    {t(
                      ', otherwise the request will be rejected by the upstream privacy filter. Adds ~10–30 s before task_id is returned.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='mode-multimodal'
                  title={t('Multi-modal reference')}
                  description={
                    <>
                      {t(
                        'Seedance 2.0 / 2.0 fast only. Combine 0–9 reference images, 0–3 reference videos, 0–3 reference audios into a single output video.'
                      )}
                      <NewBadge />
                    </>
                  }
                >
                  <CodeTabs shell={CURL_MULTIMODAL} python={PY_MULTIMODAL} />
                  <p className='text-muted-foreground text-sm'>
                    {t('Reference images use')} <K>role: reference_image</K>,{' '}
                    {t('videos use')} <K>role: reference_video</K>,{' '}
                    {t('audios use')} <K>role: reference_audio</K>.
                  </p>
                  <Callout type='warn'>
                    {t(
                      'Audio cannot be passed alone — it must accompany at least one reference video or image. If you need strict first/last frame matching, prefer'
                    )}{' '}
                    <a
                      href='#mode-image-firstlast'
                      className='text-primary hover:underline'
                    >
                      {t('Image-to-video · first/last frame')}
                    </a>
                    .
                  </Callout>
                  <Callout type='warn'>
                    <strong>{t('Real-person reference image?')}</strong>{' '}
                    {t('Add')} <K>"metadata": &#123; "audit_image": true &#125;</K>
                    {t(
                      ', otherwise the request will be rejected by the upstream privacy filter. Adds ~10–30 s before task_id is returned.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='mode-edit'
                  title={t('Edit video')}
                  description={
                    <>
                      {t(
                        'Seedance 2.0 only. Locally replace or alter elements of an existing video while preserving camera motion and composition.'
                      )}
                      <NewBadge />
                    </>
                  }
                >
                  <CodeTabs shell={CURL_EDIT} python={PY_EDIT} />
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Combine a reference video with reference image(s) and a prompt that describes the edit, e.g. "Replace the perfume in video1 with the cream from image1; keep camera motion intact".'
                    )}
                  </p>
                  <Callout type='warn'>
                    <strong>{t('Real-person reference image?')}</strong>{' '}
                    {t('Add')} <K>"metadata": &#123; "audit_image": true &#125;</K>
                    {t(
                      ', otherwise the request will be rejected by the upstream privacy filter. Adds ~10–30 s before task_id is returned.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='mode-extend'
                  title={t('Extend video')}
                  description={
                    <>
                      {t(
                        'Seedance 2.0 only. Stitch multiple reference videos into one continuous clip; the model fills in the transitions.'
                      )}
                      <NewBadge />
                    </>
                  }
                >
                  <CodeTabs shell={CURL_EXTEND} python={PY_EXTEND} />
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Up to three reference videos, total duration ≤ 15 s. The prompt describes the transitions or camera moves between segments.'
                    )}
                  </p>
                </Section>

                <Section
                  id='mode-web-search'
                  title={t('Web search augmented')}
                  description={
                    <>
                      {t(
                        'Seedance 2.0 only, text-to-video only. The model retrieves real-time web information before generation, improving accuracy for time-sensitive content (products, weather, news).'
                      )}
                      <NewBadge />
                    </>
                  }
                >
                  <CodeTabs shell={CURL_WEB_SEARCH} python={PY_WEB_SEARCH} />
                  <p className='text-muted-foreground text-sm'>
                    {t('Configure via')}{' '}
                    <K>{`metadata.tools: [{"type":"web_search"}]`}</K>.{' '}
                    {t('The response field')} <K>usage.tool_usage.web_search</K>{' '}
                    {t(
                      'reports how many searches were actually triggered (0 means none). This adds slight latency.'
                    )}
                  </p>
                </Section>

                <Section
                  id='params'
                  title={t('Full request parameters')}
                  description={t(
                    'POST /v1/video/generations · top-level field reference.'
                  )}
                >
                  <h3 className='text-base font-medium'>
                    {t('Top-level fields')}
                  </h3>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'model',
                        type: 'string',
                        required: true,
                        desc: (
                          <>
                            {t('Model ID — see')}{' '}
                            <a
                              href='#models'
                              className='text-primary hover:underline'
                            >
                              {t('Models & capabilities')}
                            </a>
                            .
                          </>
                        ),
                      },
                      {
                        name: 'prompt',
                        type: 'string',
                        desc: t(
                          'Text prompt. Chinese ≤ 500 chars / English ≤ 1000 words. Overly long prompts get partially ignored.'
                        ),
                      },
                      {
                        name: 'size',
                        type: 'string',
                        desc: (
                          <>
                            {t('Resolution:')} <K>480p</K> · <K>720p</K> ·{' '}
                            <K>1080p</K>{' '}
                            {t('(default 720p; 2.0 does not support 1080p).')}
                          </>
                        ),
                      },
                      {
                        name: 'duration',
                        type: 'int',
                        desc: (
                          <>
                            {t(
                              'Duration in seconds; range depends on model. Set to'
                            )}{' '}
                            <K>-1</K> {t('to let the model decide.')}
                          </>
                        ),
                      },
                      {
                        name: 'images',
                        type: 'string[]',
                        desc: t(
                          'Sugar for image-to-video first frame; equivalent to a metadata.content entry with role first_frame.'
                        ),
                      },
                      {
                        name: 'metadata',
                        type: 'object',
                        desc: t('Carries non-top-level fields, see below.'),
                      },
                    ]}
                  />

                  <h3 className='pt-2 text-base font-medium'>
                    {t('metadata fields')}
                  </h3>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'metadata.content',
                        type: 'array',
                        desc: (
                          <>
                            {t(
                              'Content array for multi-modal / first-last / edit / extend modes. Each entry has'
                            )}{' '}
                            <K>
                              {'{type, image_url|video_url|audio_url, role}'}
                            </K>
                            .
                          </>
                        ),
                      },
                      {
                        name: 'metadata.ratio',
                        type: 'string',
                        desc: (
                          <>
                            {t('Aspect ratio:')} <K>16:9</K> · <K>9:16</K> ·{' '}
                            <K>1:1</K> · <K>4:3</K> · <K>3:4</K> · <K>21:9</K> ·{' '}
                            <K>adaptive</K> {t('(default).')}
                          </>
                        ),
                      },
                      {
                        name: 'metadata.generate_audio',
                        type: 'boolean',
                        desc: t(
                          'Whether to generate a synced audio track (Seedance 2.0 / 2.0 fast only; default true).'
                        ),
                      },
                      {
                        name: 'metadata.tools',
                        type: 'array',
                        desc: (
                          <>
                            {t('Tool list. Currently:')}{' '}
                            <K>{`[{"type":"web_search"}]`}</K>.
                          </>
                        ),
                      },
                      {
                        name: 'metadata.seed',
                        type: 'int',
                        desc: t(
                          'Random seed; the same seed + parameters produce similar outputs.'
                        ),
                      },
                      {
                        name: 'metadata.watermark',
                        type: 'boolean',
                        desc: t(
                          'Whether to overlay a watermark (default false).'
                        ),
                      },
                      {
                        name: 'metadata.camera_fixed',
                        type: 'boolean',
                        desc: t('1.x models only — locks the camera position.'),
                      },
                      {
                        name: 'metadata.return_last_frame',
                        type: 'boolean',
                        desc: t(
                          'Return a still frame from the end of the video in the response.'
                        ),
                      },
                    ]}
                  />

                  <h3 className='pt-2 text-base font-medium'>
                    {t('content[].role values')}
                  </h3>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'first_frame',
                        type: 'string',
                        desc: t(
                          'First frame image (optional in single-image mode).'
                        ),
                      },
                      {
                        name: 'last_frame',
                        type: 'string',
                        desc: t(
                          'Last frame image (required in first/last frame mode).'
                        ),
                      },
                      {
                        name: 'reference_image',
                        type: 'string',
                        desc: t(
                          'Multi-modal reference image; every reference image uses this role.'
                        ),
                      },
                      {
                        name: 'reference_video',
                        type: 'string',
                        desc: t(
                          'Multi-modal reference video; every video clip uses this role.'
                        ),
                      },
                      {
                        name: 'reference_audio',
                        type: 'string',
                        desc: t(
                          'Multi-modal reference audio; every audio segment uses this role.'
                        ),
                      },
                    ]}
                  />

                  <h3 className='pt-2 text-base font-medium'>
                    {t('Pixel dimensions per ratio')}
                  </h3>
                  <div className='overflow-hidden rounded-lg border'>
                    <table className='w-full text-xs'>
                      <thead className='bg-muted'>
                        <tr className='text-left'>
                          <th className='px-4 py-2 font-medium'>
                            {t('Resolution')}
                          </th>
                          <th className='px-4 py-2 font-medium'>16:9</th>
                          <th className='px-4 py-2 font-medium'>9:16</th>
                          <th className='px-4 py-2 font-medium'>1:1</th>
                          <th className='px-4 py-2 font-medium'>4:3</th>
                          <th className='px-4 py-2 font-medium'>21:9</th>
                        </tr>
                      </thead>
                      <tbody className='divide-y'>
                        <tr>
                          <td className='px-4 py-2 font-medium'>480p</td>
                          <td className='px-4 py-2'>864×496</td>
                          <td className='px-4 py-2'>496×864</td>
                          <td className='px-4 py-2'>640×640</td>
                          <td className='px-4 py-2'>752×560</td>
                          <td className='px-4 py-2'>992×432</td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-medium'>720p</td>
                          <td className='px-4 py-2'>1280×720</td>
                          <td className='px-4 py-2'>720×1280</td>
                          <td className='px-4 py-2'>960×960</td>
                          <td className='px-4 py-2'>1112×834</td>
                          <td className='px-4 py-2'>1470×630</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                  <Callout type='tip'>
                    <K>ratio: adaptive</K> {t('(default):')}{' '}
                    {t(
                      'the model picks the best ratio for the scene. Text-to-video infers from the prompt; first/last-frame matches the uploaded image; multi-modal follows the prompt intent (video > image).'
                    )}
                  </Callout>
                </Section>

                <Section id='media-limits' title={t('Input file limits')}>
                  <h3 className='text-base font-medium'>{t('Image')}</h3>
                  <ul className='text-muted-foreground list-disc space-y-1 pl-6 text-sm'>
                    <li>
                      {t('Formats:')} <K>jpeg</K> · <K>png</K> · <K>webp</K> ·{' '}
                      <K>bmp</K> · <K>tiff</K> · <K>gif</K>
                    </li>
                    <li>
                      {t('Aspect ratio (W/H):')} <K>(0.4, 2.5)</K>
                    </li>
                    <li>
                      {t('Side length (px):')} <K>(300, 6000)</K>
                    </li>
                    <li>
                      {t(
                        'Size: ≤ 30 MB per image; total request body ≤ 64 MB (avoid Base64 for large files).'
                      )}
                    </li>
                    <li>
                      {t(
                        'Counts: 1 for first frame; 2 for first/last frame; 1–9 for multi-modal reference.'
                      )}
                    </li>
                  </ul>
                  <h3 className='pt-2 text-base font-medium'>
                    {t('Video (Seedance 2.0 / 2.0 fast only)')}
                  </h3>
                  <ul className='text-muted-foreground list-disc space-y-1 pl-6 text-sm'>
                    <li>
                      {t('Formats:')} <K>mp4</K> · <K>mov</K>
                    </li>
                    <li>{t('Resolution: 480p / 720p')}</li>
                    <li>
                      {t(
                        'Duration: [2, 15] s per clip; up to 3 clips, total ≤ 15 s.'
                      )}
                    </li>
                    <li>
                      {t(
                        'Aspect ratio (W/H): [0.4, 2.5]; side length [300, 6000] px; pixel area [409600, 927408].'
                      )}
                    </li>
                    <li>{t('Size: ≤ 50 MB per clip; FPS [24, 60].')}</li>
                  </ul>
                  <h3 className='pt-2 text-base font-medium'>
                    {t('Audio (Seedance 2.0 / 2.0 fast only)')}
                  </h3>
                  <ul className='text-muted-foreground list-disc space-y-1 pl-6 text-sm'>
                    <li>
                      {t('Formats:')} <K>wav</K> · <K>mp3</K>
                    </li>
                    <li>
                      {t(
                        'Duration: [2, 15] s per clip; up to 3 clips, total ≤ 15 s.'
                      )}
                    </li>
                    <li>
                      {t('Size: ≤ 15 MB per clip; total request body ≤ 64 MB.')}
                    </li>
                    <li>
                      {t(
                        'Audio cannot be passed alone; it must accompany at least one reference video or image.'
                      )}
                    </li>
                  </ul>
                  <Callout type='info'>
                    {t(
                      'Media inputs accept either a public URL or Base64 (data:image/png;base64,...). For large files, prefer URL to avoid oversized request bodies.'
                    )}
                  </Callout>
                </Section>

                {/* ============================ Pixverse ============================ */}
                <Section
                  id='pixverse-overview'
                  title={t('Pixverse overview')}
                  description={t(
                    'Pixverse is a fast cinematic video generation model series. The gateway exposes its full API behind the same /v1/video/generations endpoint as Seedance — you only swap the model id.'
                  )}
                >
                  <Callout type='tip'>
                    <strong>{t('Choosing a model:')}</strong> {t('use')}{' '}
                    <K>pixverse-v4.5</K>{' '}
                    {t(
                      'as the default; v5.5 / v5.6 are newer; c1 / v6 are billed per-second (advanced). Append a quality / duration suffix (e.g.'
                    )}{' '}
                    <K>pixverse-v4.5-720p</K>, <K>pixverse-v4.5-1080p-8s</K>
                    {t(
                      ') to lock the output spec — handy for stable per-call pricing.'
                    )}
                  </Callout>
                  <Callout type='info'>
                    <strong>
                      {t(
                        'Four input modes (auto-detected from images[].length):'
                      )}
                    </strong>
                    <ul className='mt-2 list-disc space-y-1 pl-5'>
                      <li>
                        <strong>{t('Text-to-video')}</strong> —{' '}
                        {t('no images.')}
                      </li>
                      <li>
                        <strong>{t('Image-to-video')}</strong> —{' '}
                        {t('one image (animates the still).')}
                      </li>
                      <li>
                        <strong>{t('First/last frame (transition)')}</strong> —{' '}
                        {t('two images (start and end).')}
                      </li>
                      <li>
                        <strong>{t('Multi-reference (fusion)')}</strong> —{' '}
                        {t('three or more images. Use')} <K>@ref_name</K>{' '}
                        {t('in the prompt to bind a noun to a specific image.')}
                      </li>
                    </ul>
                  </Callout>
                  <Callout type='info'>
                    <strong>{t('Image input — URL or Base64.')}</strong>{' '}
                    {t(
                      'The gateway uploads each image to Pixverse on your behalf and substitutes img_id automatically. Pre-uploaded img_ids can also be passed via metadata.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='pixverse-models'
                  title={t('Pixverse models & pricing')}
                  description={t(
                    'Each priced SKU is its own model id. The suffix locks output quality / duration so billing always matches what was generated.'
                  )}
                >
                  <Callout type='warn'>
                    {t(
                      'When the model id pins quality/duration (e.g. pixverse-v4.5-720p), the adapter overrides any client-supplied size or duration. Bare ids (pixverse-v4.5) honour the client request and bill at the 540p / 5s tier — only use bare ids when you trust the caller.'
                    )}
                  </Callout>
                  <div className='overflow-x-auto rounded-lg border'>
                    <table className='w-full text-sm'>
                      <thead className='bg-muted'>
                        <tr className='text-left'>
                          <th className='px-4 py-2 font-medium'>
                            {t('Model id')}
                          </th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Quality')}
                          </th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Duration')}
                          </th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Price (USD/video)')}
                          </th>
                        </tr>
                      </thead>
                      <tbody className='divide-y text-xs'>
                        {[
                          [
                            'pixverse-v3.5 / v4 / v4.5 / v5',
                            '540p',
                            '5s',
                            '$0.99',
                          ],
                          ['  · -720p', '720p', '5s', '$1.32'],
                          ['  · -1080p', '1080p', '5s', '$2.64'],
                          ['  · -8s', '540p', '8s', '$1.98'],
                          ['  · -720p-8s', '720p', '8s', '$2.64'],
                          ['  · -1080p-8s', '1080p', '8s', '$5.28'],
                          ['pixverse-v5.5', '540p', '5s', '$0.99'],
                          ['  · -720p', '720p', '5s', '$1.32'],
                          ['  · -1080p', '1080p', '5s', '$2.64'],
                          ['pixverse-v5.6', '540p', '5s', '$0.77'],
                          ['  · -720p', '720p', '5s', '$0.99'],
                          ['  · -1080p', '1080p', '5s', '$1.65'],
                          [
                            'pixverse-c1 / pixverse-v6',
                            '—',
                            '—',
                            t('per-second; configure separately'),
                          ],
                        ].map((row, i) => (
                          <tr key={i}>
                            <td className='px-4 py-2 font-mono whitespace-pre'>
                              {row[0]}
                            </td>
                            <td className='px-4 py-2'>{row[1]}</td>
                            <td className='px-4 py-2'>{row[2]}</td>
                            <td className='px-4 py-2'>{row[3]}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t('Source:')}{' '}
                    <a
                      href='https://docs.platform.pixverse.ai/model-pricing-796039m0'
                      target='_blank'
                      rel='noreferrer noopener'
                      className='text-primary hover:underline'
                    >
                      docs.platform.pixverse.ai · model-pricing
                    </a>
                  </p>
                </Section>

                <Section
                  id='pixverse-mode-text'
                  title={t('Text-to-video')}
                  description={t(
                    'No images — pass model + prompt. Defaults: 540p, 5s, 16:9.'
                  )}
                >
                  <CodeTabs
                    shell={CURL_PIXVERSE_T2V}
                    python={PY_PIXVERSE_T2V}
                  />
                  <h3 className='pt-2 text-base font-medium'>
                    {t('Pinning the spec via the model id')}
                  </h3>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'A suffix on the model id locks both quality and billing — the caller cannot upgrade quality past what they paid for.'
                    )}
                  </p>
                  <CodeBlock lang='shell' code={CURL_PIXVERSE_T2V_PINNED} />
                </Section>

                <Section
                  id='pixverse-mode-image'
                  title={t('Image-to-video')}
                  description={t(
                    'Pass exactly one image in images[]. The gateway uploads it to Pixverse before submitting.'
                  )}
                >
                  <CodeTabs
                    shell={CURL_PIXVERSE_I2V}
                    python={PY_PIXVERSE_I2V}
                  />
                  <Callout type='info'>
                    {t('Each entry in')} <K>images</K>{' '}
                    {t(
                      'may be an HTTP(S) URL, a data:image/...;base64,... URI, or a numeric string of an already-uploaded Pixverse img_id (skips re-upload).'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='pixverse-mode-transition'
                  title={t('First/last frame (transition)')}
                  description={t(
                    'Two images: the first becomes the start frame, the second becomes the end frame. Pixverse interpolates between them.'
                  )}
                >
                  <CodeBlock lang='shell' code={CURL_PIXVERSE_TRANSITION} />
                </Section>

                <Section
                  id='pixverse-mode-fusion'
                  title={
                    <>
                      {t('Multi-reference (fusion)')}
                      <NewBadge />
                    </>
                  }
                  description={t(
                    'Three or more images. Each becomes a named reference; bind nouns in the prompt with @ref_name.'
                  )}
                >
                  <CodeBlock lang='shell' code={CURL_PIXVERSE_FUSION} />
                  <Callout type='tip'>
                    {t('If')} <K>metadata.image_references</K>{' '}
                    {t(
                      'is omitted, the gateway auto-fills it as ref_name=img1, img2, ... all with type=subject. To use specific @-references in your prompt, supply image_references explicitly with type/img_id/ref_name. Pixverse upload-then-bind pre-uploaded img_ids by passing the upload-response id directly.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='pixverse-params'
                  title={t('Pixverse parameters')}
                  description={t(
                    'POST /v1/video/generations — fields specific to Pixverse on top of the standard task body.'
                  )}
                >
                  <h3 className='text-base font-medium'>
                    {t('Top-level fields')}
                  </h3>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'model',
                        type: 'string',
                        required: true,
                        desc: t('A Pixverse model id (see Models & pricing).'),
                      },
                      {
                        name: 'prompt',
                        type: 'string',
                        required: true,
                        desc: t(
                          'Up to 2048 UTF-8 chars. Use @ref_name in fusion mode.'
                        ),
                      },
                      {
                        name: 'images',
                        type: 'string[]',
                        desc: t(
                          '0 → text-to-video, 1 → image-to-video, 2 → transition, 3+ → fusion. URLs, data URIs, or numeric img_ids.'
                        ),
                      },
                      {
                        name: 'size',
                        type: 'string',
                        desc: t(
                          'WxH hint, e.g. 1280x720. Mapped to quality + aspect_ratio. Ignored when the model id pins quality.'
                        ),
                      },
                      {
                        name: 'duration',
                        type: 'integer',
                        desc: t(
                          '1–15 (s). Defaults to 5. Ignored when the model id pins duration (e.g. -8s suffix).'
                        ),
                      },
                      {
                        name: 'metadata',
                        type: 'object',
                        desc: t('Pixverse-specific overrides — see below.'),
                      },
                    ]}
                  />
                  <h3 className='pt-4 text-base font-medium'>
                    {t('metadata fields')}
                  </h3>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'quality',
                        type: 'string',
                        desc: t(
                          '"360p" | "540p" | "720p" | "1080p". Overrides default if model id is bare.'
                        ),
                      },
                      {
                        name: 'aspect_ratio',
                        type: 'string',
                        desc: t(
                          '"16:9" | "9:16" | "1:1" | "4:3" | "3:4" | "2:3" | "3:2" | "21:9".'
                        ),
                      },
                      {
                        name: 'motion_mode',
                        type: 'string',
                        desc: t('"normal" | "fast" (only 5s + ≤720p).'),
                      },
                      {
                        name: 'negative_prompt',
                        type: 'string',
                        desc: t('Things to avoid in the output.'),
                      },
                      {
                        name: 'seed',
                        type: 'integer',
                        desc: t('0–2147483647. Reproducibility seed.'),
                      },
                      {
                        name: 'water_mark',
                        type: 'boolean',
                        desc: t(
                          'Include the Pixverse watermark on the output.'
                        ),
                      },
                      {
                        name: 'generate_audio_switch',
                        type: 'boolean',
                        desc: t(
                          'Add generated audio to the video (model-dependent).'
                        ),
                      },
                      {
                        name: 'style',
                        type: 'string',
                        desc: t(
                          'Pixverse style name (e.g. "anime", "3d_animation").'
                        ),
                      },
                      {
                        name: 'img_id',
                        type: 'integer',
                        desc: t(
                          'Image-to-video — skip the auto-upload by supplying a pre-uploaded Pixverse img_id directly.'
                        ),
                      },
                      {
                        name: 'first_frame_img / last_frame_img',
                        type: 'integer',
                        desc: t(
                          'Transition — pre-uploaded img_ids for explicit start/end frames.'
                        ),
                      },
                      {
                        name: 'image_references',
                        type: 'object[]',
                        desc: t(
                          'Fusion — array of {type, img_id, ref_name}. type is "subject" or "background"; ref_name is referenced as @name in the prompt.'
                        ),
                      },
                    ]}
                  />
                </Section>

                <Section
                  id='pixverse-errors'
                  title={t('Pixverse error codes')}
                  description={t(
                    'On failure the gateway forwards the upstream code/message. Some common ones:'
                  )}
                >
                  <div className='overflow-hidden rounded-lg border'>
                    <table className='w-full text-sm'>
                      <thead className='bg-muted'>
                        <tr className='text-left'>
                          <th className='px-4 py-2 font-medium'>ErrCode</th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Meaning')}
                          </th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Likely cause')}
                          </th>
                        </tr>
                      </thead>
                      <tbody className='divide-y text-xs'>
                        {[
                          [
                            '400013',
                            t('Invalid field type or value'),
                            t(
                              'Wrong shape for image_references / fusion (need objects, not ints).'
                            ),
                          ],
                          [
                            '400017',
                            t('Required field missing'),
                            t('e.g. image_references entry without img_id.'),
                          ],
                          [
                            '1004',
                            t('Auth failed'),
                            t('Channel API-KEY wrong or revoked.'),
                          ],
                          [
                            '429xx',
                            t('Rate limited'),
                            t(
                              'Pixverse account / IP throttled — back off and retry.'
                            ),
                          ],
                          [
                            '2013',
                            t('Parameter conflict'),
                            t(
                              'e.g. 1080p + 8s with a model that disallows it; fast motion + 1080p.'
                            ),
                          ],
                        ].map((row, i) => (
                          <tr key={i}>
                            <td className='px-4 py-2 font-mono'>{row[0]}</td>
                            <td className='px-4 py-2'>{row[1]}</td>
                            <td className='text-muted-foreground px-4 py-2'>
                              {row[2]}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t('Full list:')}{' '}
                    <a
                      href='https://docs.platform.pixverse.ai/error-codes-796041m0'
                      target='_blank'
                      rel='noreferrer noopener'
                      className='text-primary hover:underline'
                    >
                      docs.platform.pixverse.ai · error-codes
                    </a>
                  </p>
                </Section>

                {/* ============================ Tasks & results ============================ */}
                <Section
                  id='poll'
                  title={t('Query a task')}
                  description={t(
                    'GET /v1/video/generations/:task_id · recommended polling interval 5 s.'
                  )}
                >
                  <h3 className='text-base font-medium'>
                    {t('Request example')}
                  </h3>
                  <CodeTabs shell={CURL_POLL} python={PY_POLL} />
                  <h3 className='pt-2 text-base font-medium'>
                    {t('Status values')}
                  </h3>
                  <div className='overflow-hidden rounded-lg border'>
                    <table className='w-full text-sm'>
                      <thead className='bg-muted'>
                        <tr className='text-left'>
                          <th className='px-4 py-2 font-medium'>status</th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Meaning')}
                          </th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Terminal?')}
                          </th>
                        </tr>
                      </thead>
                      <tbody className='divide-y text-xs'>
                        <tr>
                          <td className='px-4 py-2 font-mono'>
                            queued / NOT_START
                          </td>
                          <td className='px-4 py-2'>{t('Queued')}</td>
                          <td className='text-muted-foreground px-4 py-2'>
                            {t('no')}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>
                            IN_PROGRESS / processing
                          </td>
                          <td className='px-4 py-2'>{t('Generating')}</td>
                          <td className='text-muted-foreground px-4 py-2'>
                            {t('no')}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>
                            SUCCESS / succeeded
                          </td>
                          <td className='px-4 py-2'>
                            {t('Completed; the video URL is at')}{' '}
                            <K>data.data.content.video_url</K>.
                          </td>
                          <td className='px-4 py-2'>✅</td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>
                            FAILED / failed
                          </td>
                          <td className='px-4 py-2'>
                            {t('Failed; the reason is in')} <K>fail_reason</K>.
                          </td>
                          <td className='px-4 py-2'>✅</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                  <h3 className='pt-2 text-base font-medium'>
                    {t('Response fields')}
                  </h3>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'data.status',
                        type: 'string',
                        desc: t('Any of the status values above.'),
                      },
                      {
                        name: 'data.progress',
                        type: 'string',
                        desc: t('Like "50%"; informational only.'),
                      },
                      {
                        name: 'data.data.content.video_url',
                        type: 'string',
                        desc: t(
                          'Video URL on success — signed link with limited validity, download promptly.'
                        ),
                      },
                      {
                        name: 'data.data.usage.completion_tokens',
                        type: 'int',
                        desc: t('Tokens spent on output.'),
                      },
                      {
                        name: 'data.data.usage.total_tokens',
                        type: 'int',
                        desc: t('Total tokens for this request.'),
                      },
                      {
                        name: 'data.data.usage.tool_usage.web_search',
                        type: 'int',
                        desc: t(
                          'Web-search invocations (returned only when tools=[web_search]).'
                        ),
                      },
                      {
                        name: 'data.data.duration',
                        type: 'int',
                        desc: t(
                          'Actual generated video duration in seconds — important when you submitted duration=-1.'
                        ),
                      },
                      {
                        name: 'data.data.ratio',
                        type: 'string',
                        desc: t(
                          'Actual aspect ratio (useful when you submitted ratio=adaptive).'
                        ),
                      },
                    ]}
                  />
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      '720p / 5 s tasks usually finish in 90–120 s. Do not poll faster than every 5 s — you may trigger rate limits.'
                    )}
                  </p>
                </Section>

                <Section
                  id='download'
                  title={t('Download a video')}
                  description={t(
                    'GET /v1/videos/:task_id/content · download the generated video file.'
                  )}
                >
                  <CodeTabs shell={CURL_DOWNLOAD} python={PY_DOWNLOAD} />
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'This endpoint streams the video file directly. Response'
                    )}{' '}
                    <K>Content-Type: video/mp4</K>
                    {t(', so you can embed directly with')}{' '}
                    <K>{'<video src=...>'}</K>.
                  </p>
                  <Callout type='warn'>
                    {t(
                      '⚠️ Videos remain available for 24 hours. Download or re-host to your own storage immediately after success — the endpoint will return 502 after this window. Long-term storage is on the roadmap.'
                    )}
                  </Callout>
                </Section>

                <Section
                  id='full-example'
                  title={t('End-to-end example')}
                  description={t(
                    'Submit → poll → download. Drop-in code with error handling.'
                  )}
                >
                  <Tabs
                    defaultValue='python'
                    className='overflow-hidden rounded-lg border'
                  >
                    <TabsList className='bg-muted h-10 w-full justify-start rounded-none border-b px-2'>
                      <TabsTrigger value='python'>Python</TabsTrigger>
                      <TabsTrigger value='node'>Node.js</TabsTrigger>
                    </TabsList>
                    <TabsContent value='python' className='m-0'>
                      <CodeBlock lang='python' code={PYTHON_FULL} />
                    </TabsContent>
                    <TabsContent value='node' className='m-0'>
                      <CodeBlock lang='javascript' code={NODE_FULL} />
                    </TabsContent>
                  </Tabs>
                </Section>

                {/* ============================ Appendix ============================ */}
                <Section
                  id='pricing'
                  title={t('Pricing')}
                  description={t('Charged per video, in USD.')}
                >
                  <div className='overflow-hidden rounded-lg border'>
                    <table className='w-full text-sm'>
                      <thead className='bg-muted'>
                        <tr className='text-left'>
                          <th className='px-4 py-2 font-medium'>
                            {t('Model')}
                          </th>
                          <th className='px-4 py-2 font-medium'>{t('Spec')}</th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Unit price')}
                          </th>
                        </tr>
                      </thead>
                      <tbody className='divide-y text-xs'>
                        <tr>
                          <td className='px-4 py-2 font-mono'>
                            doubao-seedance-2-0-260128
                          </td>
                          <td className='px-4 py-2'>720p / 5 s</td>
                          <td className='px-4 py-2'>$0.885 / video</td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>
                            doubao-seedance-2-0-fast-260128
                          </td>
                          <td className='px-4 py-2'>720p / 5 s</td>
                          <td className='px-4 py-2'>$0.712 / video</td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'Currently flat pricing (baseline 720p / 5 s / no video input). Per-resolution / per-duration / video-input pricing is in development. Multi-modal calls that include video inputs apply a discount multiplier. Failed tasks are not billed.'
                    )}
                  </p>
                </Section>

                <Section id='limits' title={t('Rate limits & quotas')}>
                  <ParamTable
                    fieldLabel={t('Field')}
                    typeLabel={t('Type')}
                    requiredLabel={t('Required')}
                    descLabel={t('Description')}
                    yes={t('yes')}
                    no={t('no')}
                    params={[
                      {
                        name: 'RPM (online)',
                        type: '600 / 300',
                        desc: t(
                          'Requests per minute. 600 for Seedance 2.x / 1.5 / 1.0 pro families; 300 for the 1.0 lite family. Excess returns 429.'
                        ),
                      },
                      {
                        name: 'Concurrency',
                        type: '10 / 5',
                        desc: t(
                          '10 in-flight tasks for 2.x / 1.5 / 1.0 pro families; 5 for the 1.0 lite family. Excess gets queued.'
                        ),
                      },
                      {
                        name: 'Per-task timeout',
                        type: '5 min',
                        desc: t(
                          'Typical completion 90–120 s. Tasks past 5 minutes are auto-marked FAILED.'
                        ),
                      },
                      {
                        name: 'Account balance',
                        type: '$',
                        desc: (
                          <>
                            {t(
                              'Deducted per success; depleting it returns 403. Top up on the'
                            )}{' '}
                            <a
                              href='/wallet'
                              className='text-primary hover:underline'
                            >
                              {t('Wallet')}
                            </a>{' '}
                            {t('page.')}
                          </>
                        ),
                      },
                    ]}
                  />
                </Section>

                <Section id='errors' title={t('Error codes')}>
                  <div className='overflow-hidden rounded-lg border'>
                    <table className='w-full text-sm'>
                      <thead className='bg-muted'>
                        <tr className='text-left'>
                          <th className='px-4 py-2 font-medium'>HTTP</th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Meaning')}
                          </th>
                          <th className='px-4 py-2 font-medium'>
                            {t('Action')}
                          </th>
                        </tr>
                      </thead>
                      <tbody className='divide-y text-xs'>
                        <tr>
                          <td className='px-4 py-2 font-mono'>400</td>
                          <td className='px-4 py-2'>{t('Bad parameter')}</td>
                          <td className='px-4 py-2'>
                            {t(
                              'Check model/prompt; read the message field for detail.'
                            )}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>401</td>
                          <td className='px-4 py-2'>{t('Unauthorized')}</td>
                          <td className='px-4 py-2'>
                            {t('Check Authorization header format.')}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>403</td>
                          <td className='px-4 py-2'>
                            {t('Insufficient balance or model not authorized')}
                          </td>
                          <td className='px-4 py-2'>
                            {t('Top up, or check token model scope.')}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>404</td>
                          <td className='px-4 py-2'>
                            {t('task_id not found or not owned by you')}
                          </td>
                          <td className='px-4 py-2'>{t('Verify the ID.')}</td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>429</td>
                          <td className='px-4 py-2'>{t('Rate limited')}</td>
                          <td className='px-4 py-2'>
                            {t(
                              'Lower RPM or concurrency; honor Retry-After header.'
                            )}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>502</td>
                          <td className='px-4 py-2'>
                            {t(
                              'Video file unavailable (usually expired — kept for 24 h)'
                            )}
                          </td>
                          <td className='px-4 py-2'>
                            {t(
                              'Retry within 24h, or migrate to your own storage.'
                            )}
                          </td>
                        </tr>
                        <tr>
                          <td className='px-4 py-2 font-mono'>500</td>
                          <td className='px-4 py-2'>{t('Server error')}</td>
                          <td className='px-4 py-2'>
                            {t('Retry; if persistent, contact support.')}
                          </td>
                        </tr>
                      </tbody>
                    </table>
                  </div>
                </Section>

                <Section id='best-practices' title={t('Best practices')}>
                  <ul className='space-y-3 text-sm leading-relaxed'>
                    <li>
                      <strong className='font-medium'>
                        {t('Polling cadence')}
                      </strong>
                      :{' '}
                      {t(
                        'a steady 5 s interval is enough. Polling faster only triggers rate limiting; processing time is fixed. Add exponential backoff: double the interval on errors, cap at 30 s.'
                      )}
                    </li>
                    <li>
                      <strong className='font-medium'>
                        {t('Download immediately')}
                      </strong>
                      : {t('right after success, GET')}{' '}
                      <K>/v1/videos/:task_id/content</K>{' '}
                      {t(
                        "and persist to your own object storage or CDN. Don't rely on the 24h window."
                      )}
                    </li>
                    <li>
                      <strong className='font-medium'>
                        {t('Concurrency control')}
                      </strong>
                      :{' '}
                      {t(
                        'cap is 10 (5 for lite). For batch jobs, use a semaphore — friendlier than firing requests until 429.'
                      )}
                    </li>
                    <li>
                      <strong className='font-medium'>
                        {t('Prompt engineering')}
                      </strong>
                      :{' '}
                      {t(
                        'keep Chinese prompts under 500 chars. Best with all four ingredients: subject / action / camera / style. For audio generation, wrap dialogue in double quotes for cleaner voice synthesis.'
                      )}
                    </li>
                    <li>
                      <strong className='font-medium'>
                        {t('Failure retry')}
                      </strong>
                      :{' '}
                      {t(
                        "FAILED tasks are not billed. Inspect fail_reason: content-moderation failures won't recover via retry; other reasons are worth up to 2 retries."
                      )}
                    </li>
                    <li>
                      <strong className='font-medium'>
                        {t('Pick the right mode')}
                      </strong>
                      :{' '}
                      {t(
                        'commit to your creative intent before choosing a mode. Strict first/last frame control → first/last frame; loose image anchors → multi-modal reference; pure text description → text-to-video.'
                      )}
                    </li>
                  </ul>
                </Section>

                <Section id='faq' title={t('FAQ')}>
                  <div className='space-y-5'>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t('How long does generation take?')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t(
                          '720p / 5 s usually 90–120 s. 1080p or longer videos take more. Concurrent tasks in the queue add waiting time.'
                        )}
                      </p>
                    </div>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t('Can I call this with the OpenAI SDK?')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t(
                          "Video tasks are an async task model, not OpenAI's chat/completion shape. The SDK's video.generate is not yet compatible. Use raw HTTP, or our official SDK (planned)."
                        )}
                      </p>
                    </div>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t('How long are videos retained?')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t('Videos pulled via')}{' '}
                        <K>/v1/videos/:task_id/content</K>{' '}
                        {t(
                          'are available for 24 hours. Long-term storage (in our own object store) is on the roadmap.'
                        )}
                      </p>
                    </div>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t('How do I check usage?')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t('Visit')}{' '}
                        <a
                          href='/usage-logs/task'
                          className='text-primary hover:underline'
                        >
                          {t('Task logs')}
                        </a>{' '}
                        {t('for per-task billing, and')}{' '}
                        <a
                          href='/wallet'
                          className='text-primary hover:underline'
                        >
                          {t('Wallet')}
                        </a>{' '}
                        {t('for balance changes.')}
                      </p>
                    </div>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t('Are failed tasks billed?')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t('No. Only SUCCESS tasks deduct balance.')}
                      </p>
                    </div>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t('Are webhook callbacks supported?')}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t(
                          'Not yet — clients poll today. Webhook callbacks are on the roadmap.'
                        )}
                      </p>
                    </div>
                    <div className='space-y-1'>
                      <h3 className='text-sm font-medium'>
                        {t(
                          'Can multi-modal reference replace first/last frame?'
                        )}
                      </h3>
                      <p className='text-muted-foreground text-sm'>
                        {t(
                          "Multi-modal can hint to the model via prompt that an image should serve as the first/last frame, but it's less strict than the explicit"
                        )}{' '}
                        <K>first_frame</K> / <K>last_frame</K>{' '}
                        {t(
                          'roles. Prefer the latter when exact alignment matters.'
                        )}
                      </p>
                    </div>
                  </div>
                </Section>

                <div className='space-y-2 border-t pt-6 text-sm'>
                  <h2 className='font-semibold'>{t('Related resources')}</h2>
                  <ul className='text-muted-foreground space-y-1'>
                    <li>
                      ·{' '}
                      <a href='/keys' className='text-primary hover:underline'>
                        {t('Tokens')}
                      </a>{' '}
                      · {t('Manage API keys')}
                    </li>
                    <li>
                      ·{' '}
                      <a
                        href='/wallet'
                        className='text-primary hover:underline'
                      >
                        {t('Wallet')}
                      </a>{' '}
                      · {t('Check balance and top up')}
                    </li>
                    <li>
                      ·{' '}
                      <a
                        href='/usage-logs/task'
                        className='text-primary hover:underline'
                      >
                        {t('Task logs')}
                      </a>{' '}
                      · {t('Past tasks and billing')}
                    </li>
                    <li>
                      ·{' '}
                      <a
                        href='https://github.com/NekoAIKan/aikanhub'
                        target='_blank'
                        rel='noreferrer noopener'
                        className='text-primary hover:underline'
                      >
                        {t('GitHub repository')}
                      </a>
                    </li>
                  </ul>
                </div>
              </article>
            </main>

            {/* ===================== Right TOC (current group only) ===================== */}
            <aside className='sticky top-16 hidden h-[calc(100vh-4rem)] w-44 shrink-0 overflow-y-auto py-8 pl-2 xl:block'>
              <div className='space-y-2'>
                <h3 className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
                  {activeGroup.label}
                </h3>
                <nav className='flex flex-col gap-1'>
                  {activeGroup.items.map((item) => {
                    const isActive = item.id === activeId
                    return (
                      <a
                        key={item.id}
                        href={`#${item.id}`}
                        className={`border-l-2 pl-3 text-xs leading-relaxed transition-colors ${
                          isActive
                            ? 'border-primary text-foreground font-medium'
                            : 'text-muted-foreground hover:text-foreground hover:border-primary border-transparent'
                        }`}
                      >
                        {item.label}
                      </a>
                    )
                  })}
                </nav>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </PublicLayout>
  )
}
