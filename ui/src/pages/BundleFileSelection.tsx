import { useNavigate, useParams } from '@solidjs/router'
import { For, Show, createResource, createSignal } from 'solid-js'
import { reconcile } from 'solid-js/store'

import { BundleResponse } from '../types/Bundle'
import { useData } from '../components/DataContext'

import FileArchive from '../assets/file-archive.svg'
import { backend_url } from '../utils/backend_url'

export const BundleFileSelection = () => {
  const { setNewBundle } = useData()
  const params = useParams()
  const navigate = useNavigate()
  async function fetchRevisions() {
    const res = await fetch(`${backend_url}/api/revisions/package/${params.package_id}`)
    return (await res.json()) as string[]
  }

  async function fetchPackageData(filename: string) {
    const res = await fetch(`${backend_url}/api/revisions/package/${params.package_id}/${filename}`)
    return (await res.json()) as BundleResponse
  }

  const [files] = createResource(fetchRevisions)

  return (
    <div class="flex flex-col justify-center items-center h-full w-full">
      <h1 class="text-4xl text-gray-600 font-bold text mb-12 max-w-lg text-center">
        Select a bundle file from the selected package
      </h1>
      <div class="w-96 space-y-2">
        <For each={files()} fallback={<li>Loading...</li>}>
          {(file) => {
            const [loading, setLoading] = createSignal(false)

            return (
              <button
                class="flex justify-between items-center w-full bg-gray-200 hover:bg-gray-300 text-gray-600 py-2 px-4 rounded"
                onClick={async () => {
                  setLoading(true)

                  const data = await fetchPackageData(file)
                  setNewBundle(data.files)

                  navigate(`/play?revision_id=${JSON.parse(data.files['/.manifest']).revision}`)
                }}
              >
                <div class="flex items-center">
                  <FileArchive class="w-8 h-8 mr-4" />
                  {file}
                </div>
                <Show when={loading()}>
                  <svg
                    aria-hidden="true"
                    role="status"
                    class="inline mr-2 w-4 h-4 text-gray-200 animate-spin dark:text-gray-600"
                    viewBox="0 0 100 101"
                    fill="none"
                    xmlns="http://www.w3.org/2000/svg"
                  >
                    <path
                      d="M100 50.5908C100 78.2051 77.6142 100.591 50 100.591C22.3858 100.591 0 78.2051 0 50.5908C0 22.9766 22.3858 0.59082 50 0.59082C77.6142 0.59082 100 22.9766 100 50.5908ZM9.08144 50.5908C9.08144 73.1895 27.4013 91.5094 50 91.5094C72.5987 91.5094 90.9186 73.1895 90.9186 50.5908C90.9186 27.9921 72.5987 9.67226 50 9.67226C27.4013 9.67226 9.08144 27.9921 9.08144 50.5908Z"
                      fill="currentColor"
                    ></path>
                    <path
                      d="M93.9676 39.0409C96.393 38.4038 97.8624 35.9116 97.0079 33.5539C95.2932 28.8227 92.871 24.3692 89.8167 20.348C85.8452 15.1192 80.8826 10.7238 75.2124 7.41289C69.5422 4.10194 63.2754 1.94025 56.7698 1.05124C51.7666 0.367541 46.6976 0.446843 41.7345 1.27873C39.2613 1.69328 37.813 4.19778 38.4501 6.62326C39.0873 9.04874 41.5694 10.4717 44.0505 10.1071C47.8511 9.54855 51.7191 9.52689 55.5402 10.0491C60.8642 10.7766 65.9928 12.5457 70.6331 15.2552C75.2735 17.9648 79.3347 21.5619 82.5849 25.841C84.9175 28.9121 86.7997 32.2913 88.1811 35.8758C89.083 38.2158 91.5421 39.6781 93.9676 39.0409Z"
                      fill="rgb(96 165 250)"
                    ></path>
                  </svg>
                </Show>
              </button>
            )
          }}
        </For>
      </div>
    </div>
  )
}
