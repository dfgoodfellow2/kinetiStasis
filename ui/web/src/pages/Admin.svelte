<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { copyToClipboard } from '../lib/utils.js'
  import { store } from '../lib/stores.svelte.js'
  import Alert from '../components/Alert.svelte'

  // ── Registration mode ───────────────────────────────────
  const MODES = [
    { value: 'invite_only', label: 'Invite only', hint: 'Default. A single-use invite code is required to register.' },
    { value: 'public_approval', label: 'Public with approval', hint: 'Anyone can request an account; you approve each one before sign-in.' },
    { value: 'public', label: 'Public', hint: 'Anyone can register and sign in immediately, no approval.' },
  ]

  let mode = $state('invite_only')
  let modeSaving = $state(false)
  let modeError = $state('')

  async function changeMode(event) {
    const next = event.target.value
    if (next === mode) return
    const copy = {
      invite_only: 'Only people with an invite code will be able to register. Continue?',
      public_approval: 'Anyone will be able to request an account, and you approve each one before they can sign in. Continue?',
      public: 'Anyone will be able to register and sign in immediately. This opens the instance to the public. Continue?',
    }[next]
    if (!confirm(copy)) {
      // Force the select back to the saved value; mode itself did not change.
      event.target.value = mode
      return
    }
    modeError = ''
    modeSaving = true
    try {
      const res = await api.setRegistrationMode(next)
      mode = res?.mode || next
    } catch (e) {
      modeError = e.message || String(e)
      event.target.value = mode
    } finally {
      modeSaving = false
    }
  }

  // ── Invite codes ────────────────────────────────────────
  let invites = $state([])
  let invitesLoading = $state(false)
  let inviteError = $state('')
  let inviteBusy = $state(false)
  let newCode = $state('')
  let copied = $state(false)
  let notice = $state('')

  async function loadInvites() {
    invitesLoading = true
    inviteError = ''
    try {
      invites = await api.listRegistrationInvites()
    } catch (e) {
      inviteError = e.message || String(e)
    } finally {
      invitesLoading = false
    }
  }

  async function generateInvite() {
    inviteError = ''
    notice = ''
    inviteBusy = true
    try {
      const inv = await api.createRegistrationInvite()
      newCode = inv?.code || ''
      copied = false
      if (newCode) {
        const ok = await copyToClipboard(newCode)
        if (ok) {
          copied = true
          setTimeout(() => { copied = false }, 2500)
        }
      }
      await loadInvites()
    } catch (e) {
      inviteError = e.message || String(e)
    } finally {
      inviteBusy = false
    }
  }

  async function copyNewCode() {
    if (!newCode) return
    const ok = await copyToClipboard(newCode)
    copied = ok
  }

  async function revokeInvite(inv) {
    if (!confirm(`Revoke invite ${inv.code}? Anyone holding it will no longer be able to register.`)) return
    inviteError = ''
    notice = ''
    try {
      await api.revokeRegistrationInvite(inv.code)
      notice = `Invite ${inv.code} revoked.`
      await loadInvites()
    } catch (e) {
      inviteError = e.message || String(e)
    }
  }

  function inviteState(inv) {
    if (inv.usedBy) return { label: 'Used', cls: 'text-gray-500' }
    if (inv.expiresAt && new Date(inv.expiresAt).getTime() < Date.now()) {
      return { label: 'Expired', cls: 'text-amber-400' }
    }
    return { label: 'Active', cls: 'text-emerald-400' }
  }

  function shortID(id) {
    return id ? id.slice(0, 8) : '—'
  }

  function fmtDate(v) {
    if (!v) return '—'
    const d = new Date(v)
    return isNaN(d.getTime()) ? v : d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
  }

  // ── Pending users ───────────────────────────────────────
  let pending = $state([])
  let pendingLoading = $state(false)
  let pendingError = $state('')
  let pendingBusy = $state(null)

  async function loadPending() {
    pendingLoading = true
    pendingError = ''
    try {
      pending = await api.listPendingUsers()
    } catch (e) {
      pendingError = e.message || String(e)
    } finally {
      pendingLoading = false
    }
  }

  async function approveUser(user) {
    pendingError = ''
    notice = ''
    pendingBusy = user.id
    try {
      await api.approveUser(user.id)
      notice = `${user.username} approved — they can now sign in.`
      await loadPending()
    } catch (e) {
      pendingError = e.message || String(e)
    } finally {
      pendingBusy = null
    }
  }

  async function denyUser(user) {
    if (!confirm(`Deny ${user.username} and permanently delete their pending account?`)) return
    pendingError = ''
    notice = ''
    pendingBusy = user.id
    try {
      await api.denyUser(user.id)
      notice = `${user.username} denied and removed.`
      await loadPending()
    } catch (e) {
      pendingError = e.message || String(e)
    } finally {
      pendingBusy = null
    }
  }

  onMount(async () => {
    if (store.user && !store.user.isAdmin) return
    try {
      const res = await api.getRegistrationMode()
      if (res?.mode) mode = res.mode
    } catch (e) {
      modeError = e.message || String(e)
    }
    await Promise.all([loadInvites(), loadPending()])
  })
</script>

<div class="max-w-screen-xl mx-auto space-y-4">
  <div class="flex items-center justify-between">
    <h2 class="text-emerald-400 font-bold text-lg">🛠 Admin</h2>
  </div>

  {#if store.user && !store.user.isAdmin}
    <Alert type="error" message="Admin access required." />
  {:else}
    {#if notice}<Alert type="success" message={notice} />{/if}

    <!-- Registration mode -->
    <section class="bg-gray-800 p-4 rounded border border-gray-700 space-y-3">
      <div>
        <h3 class="text-emerald-400 font-semibold text-sm">Registration mode</h3>
        <p class="text-xs text-gray-500">
          Controls who can create an account on this instance. The household default is invite only.
        </p>
      </div>
      {#if modeError}<Alert type="error" message={modeError} />{/if}
      <div class="grid grid-cols-1 md:grid-cols-3 gap-3">
        <select
          class="input"
          id="registration-mode"
          aria-label="Registration mode"
          value={mode}
          onchange={changeMode}
          disabled={modeSaving}
        >
          {#each MODES as m}
            <option value={m.value}>{m.label}</option>
          {/each}
        </select>
        <p class="md:col-span-2 text-xs text-gray-400 self-center">
          {MODES.find((m) => m.value === mode)?.hint || ''}
          {#if modeSaving}<span class="text-gray-500"> Saving…</span>{/if}
        </p>
      </div>
    </section>

    <!-- Invite codes -->
    <section class="bg-gray-800 p-4 rounded border border-gray-700 space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 class="text-emerald-400 font-semibold text-sm">Invite codes</h3>
          <p class="text-xs text-gray-500">Single-use, valid for 7 days. Generate one, then send it to the person you want to invite.</p>
        </div>
        <button class="btn-primary text-sm" onclick={generateInvite} disabled={inviteBusy} type="button">
          {inviteBusy ? 'Generating…' : '＋ Generate invite'}
        </button>
      </div>

      {#if newCode}
        <div class="bg-gray-900 border border-emerald-700 rounded-lg p-3 flex flex-wrap items-center gap-3">
          <span class="text-xs text-gray-400">New code</span>
          <span class="font-mono text-lg tracking-widest text-emerald-300">{newCode}</span>
          <button class="text-xs text-emerald-400 underline" onclick={copyNewCode} type="button">
            {copied ? 'Copied ✓' : 'Copy'}
          </button>
        </div>
      {/if}

      {#if inviteError}<Alert type="error" message={inviteError} />{/if}

      {#if invitesLoading}
        <p class="text-xs text-gray-500">Loading invites…</p>
      {:else if invites.length === 0}
        <p class="text-xs text-gray-500">No invite codes yet.</p>
      {:else}
        <div class="overflow-x-auto">
          <table class="w-full text-sm">
            <thead>
              <tr class="text-left text-xs text-gray-500 border-b border-gray-700">
                <th class="py-2 pr-3 font-medium">Code</th>
                <th class="py-2 pr-3 font-medium">Expires</th>
                <th class="py-2 pr-3 font-medium">Status</th>
                <th class="py-2 pr-3 font-medium">Used by</th>
                <th class="py-2 pr-3 font-medium">Created by</th>
                <th class="py-2 font-medium"></th>
              </tr>
            </thead>
            <tbody>
              {#each invites as inv (inv.code)}
                {@const state = inviteState(inv)}
                <tr class="border-b border-gray-800">
                  <td class="py-2 pr-3 font-mono tracking-wide text-gray-200">{inv.code}</td>
                  <td class="py-2 pr-3 text-gray-400">{fmtDate(inv.expiresAt)}</td>
                  <td class={`py-2 pr-3 ${state.cls}`}>{state.label}</td>
                  <td class="py-2 pr-3 font-mono text-xs text-gray-500">{shortID(inv.usedBy)}</td>
                  <td class="py-2 pr-3 font-mono text-xs text-gray-500">{shortID(inv.createdBy)}</td>
                  <td class="py-2 text-right">
                    {#if !inv.usedBy}
                      <button
                        class="text-xs text-red-400 underline"
                        onclick={() => revokeInvite(inv)}
                        type="button"
                      >Revoke</button>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>

    <!-- Pending approvals -->
    <section class="bg-gray-800 p-4 rounded border border-gray-700 space-y-3">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 class="text-emerald-400 font-semibold text-sm">Pending approvals</h3>
          <p class="text-xs text-gray-500">Accounts created while the instance is public-with-approval wait here.</p>
        </div>
        <button class="bg-gray-700 px-3 py-2 rounded-lg text-sm hover:bg-gray-600" onclick={loadPending} disabled={pendingLoading} type="button">
          Refresh
        </button>
      </div>

      {#if pendingError}<Alert type="error" message={pendingError} />{/if}

      {#if pendingLoading}
        <p class="text-xs text-gray-500">Loading pending users…</p>
      {:else if pending.length === 0}
        <p class="text-xs text-gray-500">No one is waiting for approval.</p>
      {:else}
        <div class="divide-y divide-gray-800">
          {#each pending as user (user.id)}
            <div class="flex flex-wrap items-center justify-between gap-3 py-2">
              <div>
                <div class="text-sm text-gray-200">{user.username}</div>
                <div class="text-xs text-gray-500">{user.email}</div>
              </div>
              <div class="flex items-center gap-2">
                <button
                  class="btn-primary text-sm"
                  onclick={() => approveUser(user)}
                  disabled={pendingBusy === user.id}
                  type="button"
                >{pendingBusy === user.id ? 'Working…' : 'Approve'}</button>
                <button
                  class="bg-red-800 hover:bg-red-700 px-3 py-2 rounded-lg text-sm"
                  onclick={() => denyUser(user)}
                  disabled={pendingBusy === user.id}
                  type="button"
                >Deny</button>
              </div>
            </div>
          {/each}
        </div>
      {/if}
    </section>
  {/if}
</div>
