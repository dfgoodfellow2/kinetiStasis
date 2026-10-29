<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import { copyToClipboard } from '../lib/utils.js'
  import Alert from '../components/Alert.svelte'
  import Card from '../components/Card.svelte'
  import InfoTip from '../components/InfoTip.svelte'
  import Spinner from '../components/Spinner.svelte'

  let groups = $state([])
  let loading = $state(true)
  let error = $state('')
  let notice = $state('')

  // List / detail navigation is in-page so a group feels like a workspace.
  let selectedId = $state(null)
  let selected = $state(null)
  let summary = $state(null)
  let roster = $state([])
  let detailLoading = $state(false)

  // My sharing toggles for the selected group.
  let mySharing = $state({ clock: false, streak: false, adherence: false })
  let sharingSaving = $state(false)

  // Owner invite tooling.
  let invite = $state(null)
  let inviteBusy = $state(false)
  let inviteCopied = $state(false)

  // Forms.
  let newName = $state('')
  let joinCode = $state('')
  let formBusy = $state(false)

  function badgeClass(role) {
    return role === 'owner'
      ? 'inline-flex items-center rounded-full border border-emerald-700 bg-emerald-900/60 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-emerald-300'
      : 'inline-flex items-center rounded-full border border-gray-600 bg-gray-700 px-2 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-gray-300'
  }

  function roleLabel(role) {
    return role === 'owner' ? 'Owner' : 'Member'
  }

  function clearMessages() {
    error = ''
    notice = ''
  }

  async function loadGroups() {
    try {
      const list = await api.listGroups()
      groups = Array.isArray(list) ? list : []
      if (selectedId) {
        const refreshed = groups.find((g) => g.id === selectedId)
        if (refreshed) selected = refreshed
      }
    } catch (e) {
      error = e.message || String(e)
    } finally {
      loading = false
    }
  }

  async function loadDetail(id) {
    detailLoading = true
    try {
      const s = await api.getGroupSummary(id)
      summary = s
      mySharing = {
        clock: !!s?.mySharing?.clock,
        streak: !!s?.mySharing?.streak,
        adherence: !!s?.mySharing?.adherence,
      }
      if (s?.myRole === 'owner') {
        try {
          roster = (await api.getGroupMembers(id)) || []
        } catch {
          // The privacy-safe summary still works without the owner roster;
          // removal simply stays unavailable until the roster loads.
          roster = []
        }
      } else {
        roster = []
      }
    } catch (e) {
      error = e.message || String(e)
      summary = null
    } finally {
      detailLoading = false
    }
  }

  async function openGroup(g) {
    clearMessages()
    selected = g
    selectedId = g.id
    summary = null
    roster = []
    invite = null
    inviteCopied = false
    await loadDetail(g.id)
    if (typeof window !== 'undefined') window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  function backToList() {
    clearMessages()
    selectedId = null
    selected = null
    summary = null
    roster = []
    invite = null
    inviteCopied = false
  }

  async function createGroup(event) {
    event?.preventDefault?.()
    const name = newName.trim()
    if (!name) {
      error = 'Give the group a name first.'
      return
    }
    clearMessages()
    formBusy = true
    try {
      const g = await api.createGroup(name)
      newName = ''
      notice = `Created "${g.name}".`
      await loadGroups()
      await openGroup(g)
      notice = `Created "${g.name}".`
    } catch (e) {
      error = e.message || String(e)
    } finally {
      formBusy = false
    }
  }

  async function joinGroup(event) {
    event?.preventDefault?.()
    const code = joinCode.trim().toUpperCase()
    if (!code) {
      error = 'Enter an invite code first.'
      return
    }
    clearMessages()
    formBusy = true
    try {
      const g = await api.joinGroup(code)
      joinCode = ''
      await loadGroups()
      await openGroup(g)
      notice = `Joined "${g.name}".`
    } catch (e) {
      error = e.message || String(e)
    } finally {
      formBusy = false
    }
  }

  async function toggleSharing(key, checked) {
    if (!selectedId) return
    const previous = { ...mySharing }
    mySharing = { ...mySharing, [key]: checked }
    sharingSaving = true
    clearMessages()
    try {
      const names = { clock: 'shareClock', streak: 'shareStreak', adherence: 'shareAdherence' }
      await api.updateMySharing(selectedId, { [names[key]]: checked })
      // Re-read so the member cards reflect what others now see.
      const s = await api.getGroupSummary(selectedId)
      summary = s
      mySharing = {
        clock: !!s?.mySharing?.clock,
        streak: !!s?.mySharing?.streak,
        adherence: !!s?.mySharing?.adherence,
      }
    } catch (e) {
      mySharing = previous
      error = e.message || String(e)
    } finally {
      sharingSaving = false
    }
  }

  async function generateInvite() {
    if (!selectedId) return
    clearMessages()
    inviteBusy = true
    try {
      invite = await api.createGroupInvite(selectedId)
      inviteCopied = false
    } catch (e) {
      error = e.message || String(e)
    } finally {
      inviteBusy = false
    }
  }

  async function copyInvite() {
    if (!invite?.code) return
    const ok = await copyToClipboard(invite.code)
    if (ok) {
      inviteCopied = true
      setTimeout(() => { inviteCopied = false }, 2000)
    } else {
      error = 'Copy failed'
      setTimeout(() => { error = '' }, 2000)
    }
  }

  async function removeMember(member) {
    if (!selectedId) return
    if (!confirm(`Remove ${member.name} from "${summary?.groupName || 'this group'}"?`)) return
    clearMessages()
    try {
      await api.removeGroupMember(selectedId, member.userId)
      notice = `${member.name} removed.`
      await loadDetail(selectedId)
      await loadGroups()
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function leaveGroup() {
    if (!selectedId) return
    const name = summary?.groupName || selected?.name || 'this group'
    const owner = summary?.myRole === 'owner'
    const message = owner
      ? `Leave "${name}"? You're the owner — ownership transfers to another member automatically.`
      : `Leave "${name}"?`
    if (!confirm(message)) return
    clearMessages()
    try {
      await api.leaveGroup(selectedId)
      backToList()
      loading = true
      await loadGroups()
      notice = `Left "${name}".`
    } catch (e) {
      error = e.message || String(e)
    }
  }

  async function deleteGroup() {
    if (!selectedId) return
    const name = summary?.groupName || selected?.name || 'this group'
    if (!confirm(`DESTRUCTIVE: permanently delete "${name}" for every member? All memberships and invites are removed. This cannot be undone.`)) return
    clearMessages()
    try {
      await api.deleteGroup(selectedId)
      backToList()
      loading = true
      await loadGroups()
      notice = `Deleted "${name}".`
    } catch (e) {
      error = e.message || String(e)
    }
  }

  onMount(loadGroups)
</script>

{#if error}<Alert type="error" message={error} />{/if}
{#if notice}<Alert type="success" message={notice} />{/if}

{#if !selectedId}
  <div class="max-w-screen-xl mx-auto space-y-4">
    <div class="flex items-center justify-between">
      <h1 class="text-emerald-400 font-bold text-xl">Groups</h1>
      <span class="text-xs text-gray-500">Share the journey, not your body data</span>
    </div>

    {#if loading}
      <div class="flex justify-center py-10"><Spinner /></div>
    {:else if groups.length === 0}
      <Card>
        <p class="text-sm text-gray-400">No groups yet — create one or join with a code.</p>
      </Card>
    {:else}
      <div class="grid grid-cols-1 md:grid-cols-2 gap-3">
        {#each groups as g (g.id)}
          <button
            type="button"
            class="text-left bg-gray-800 hover:bg-gray-700/60 border border-gray-700 hover:border-emerald-700 rounded-xl p-4 transition-colors"
            onclick={() => openGroup(g)}
            aria-label={`Open group ${g.name}`}
          >
            <div class="flex items-start justify-between gap-2">
              <span class="font-semibold text-white break-words">{g.name}</span>
              <span class={badgeClass(g.myRole)}>{roleLabel(g.myRole)}</span>
            </div>
            <div class="mt-2 text-xs text-gray-400">
              {g.memberCount ?? 1} {g.memberCount === 1 ? 'member' : 'members'}
            </div>
          </button>
        {/each}
      </div>
    {/if}

    <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
      <Card title="Create a group">
        <form class="space-y-3" onsubmit={createGroup}>
          <div>
            <label class="text-xs text-gray-400" for="group-name">Group name</label>
            <input
              class="input"
              id="group-name"
              placeholder="e.g. Tuesday Crew"
              maxlength="64"
              bind:value={newName}
            />
          </div>
          <button class="btn-primary" type="submit" disabled={formBusy}>
            {formBusy ? 'Creating…' : 'Create group'}
          </button>
        </form>
      </Card>

      <Card title="Join with a code">
        <form class="space-y-3" onsubmit={joinGroup}>
          <div>
            <label class="text-xs text-gray-400" for="group-code">Invite code</label>
            <input
              class="input font-mono tracking-widest uppercase"
              id="group-code"
              placeholder="ABC234XYZ9"
              maxlength="10"
              autocomplete="off"
              autocapitalize="characters"
              spellcheck="false"
              bind:value={joinCode}
            />
          </div>
          <button class="btn-primary" type="submit" disabled={formBusy}>
            {formBusy ? 'Joining…' : 'Join group'}
          </button>
        </form>
      </Card>
    </div>
  </div>
{:else}
  <div class="max-w-screen-xl mx-auto space-y-4">
    <button class="text-sm text-emerald-400 underline" type="button" onclick={backToList}>
      ← All groups
    </button>

    <div class="flex flex-wrap items-center gap-2">
      <h1 class="text-emerald-400 font-bold text-xl break-words">{summary?.groupName || selected?.name || 'Group'}</h1>
      {#if summary?.myRole}
        <span class={badgeClass(summary.myRole)}>{roleLabel(summary.myRole)}</span>
      {/if}
      <span class="text-xs text-gray-400">
        {summary?.members?.length ?? selected?.memberCount ?? 1}
        {(summary?.members?.length ?? selected?.memberCount ?? 1) === 1 ? 'member' : 'members'}
      </span>
    </div>

    {#if detailLoading}
      <div class="flex justify-center py-10"><Spinner /></div>
    {:else if !summary}
      <Card>
        <p class="text-sm text-gray-400">This group could not be loaded.</p>
      </Card>
    {:else}
      <Card title="Members">
        <div class="space-y-2">
          {#each summary.members || [] as m, i (i)}
            <div class="rounded-lg border border-gray-700 bg-gray-900/40 p-3">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium text-white break-words">{m.name}</span>
                <span class={badgeClass(m.role)}>{roleLabel(m.role)}</span>
              </div>
              <div class="mt-2 flex flex-wrap items-center gap-2 text-xs">
                {#if m.presence}
                  {#if m.presence.loggedWeight}
                    <span class="rounded bg-cyan-900/60 px-2 py-0.5 text-cyan-300">✓ Weight</span>
                  {/if}
                  {#if m.presence.loggedFood}
                    <span class="rounded bg-emerald-900/60 px-2 py-0.5 text-emerald-300">✓ Food</span>
                  {/if}
                  {#if m.presence.loggedWorkout}
                    <span class="rounded bg-amber-900/60 px-2 py-0.5 text-amber-300">✓ Workout</span>
                  {/if}
                  {#if !m.presence.loggedWeight && !m.presence.loggedFood && !m.presence.loggedWorkout}
                    <span class="text-gray-500">Nothing logged today</span>
                  {/if}
                {/if}
                {#if m.streak !== undefined && m.streak !== null}
                  <span class="rounded bg-orange-900/50 px-2 py-0.5 text-orange-300">🔥 {m.streak}-day streak</span>
                {/if}
                {#if m.adherencePct !== undefined && m.adherencePct !== null}
                  <span class="rounded bg-purple-900/50 px-2 py-0.5 text-purple-300">{Math.round(m.adherencePct)}% adherence</span>
                {/if}
                {#if !m.presence && (m.streak === undefined || m.streak === null) && (m.adherencePct === undefined || m.adherencePct === null)}
                  <span class="text-gray-500">Not sharing anything yet</span>
                {/if}
              </div>
            </div>
          {/each}
        </div>
      </Card>

      <Card>
        <div class="flex items-center gap-1 mb-3">
          <h2 class="text-lg font-bold text-emerald-400">My sharing</h2>
          <InfoTip
            label="my sharing"
            text="Controls what other members can see about you. Body data is never shareable — by design."
          />
        </div>
        <div class="space-y-2 text-sm">
          <label class="flex items-start gap-3 rounded-lg border border-gray-700 bg-gray-900/40 p-3">
            <input
              type="checkbox"
              class="mt-0.5"
              checked={mySharing.clock}
              disabled={sharingSaving}
              onchange={(e) => toggleSharing('clock', e.currentTarget.checked)}
            />
            <span>
              <span class="block text-gray-100">Today's logging clock</span>
              <span class="block text-xs text-gray-500">Shows which of food, weight and workouts you logged today.</span>
            </span>
          </label>
          <label class="flex items-start gap-3 rounded-lg border border-gray-700 bg-gray-900/40 p-3">
            <input
              type="checkbox"
              class="mt-0.5"
              checked={mySharing.streak}
              disabled={sharingSaving}
              onchange={(e) => toggleSharing('streak', e.currentTarget.checked)}
            />
            <span>
              <span class="block text-gray-100">Logging streak</span>
              <span class="block text-xs text-gray-500">Your consecutive logged days.</span>
            </span>
          </label>
          <label class="flex items-start gap-3 rounded-lg border border-gray-700 bg-gray-900/40 p-3">
            <input
              type="checkbox"
              class="mt-0.5"
              checked={mySharing.adherence}
              disabled={sharingSaving}
              onchange={(e) => toggleSharing('adherence', e.currentTarget.checked)}
            />
            <span>
              <span class="block text-gray-100">7-day adherence</span>
              <span class="block text-xs text-gray-500">How closely you hit your calorie target over the past week.</span>
            </span>
          </label>
        </div>
      </Card>

      {#if summary.myRole === 'owner'}
        <Card title="Owner tools">
          <div class="space-y-5">
            <div>
              <h3 class="text-sm font-semibold text-gray-200 mb-2">Invite someone</h3>
              <div class="flex flex-wrap items-center gap-3">
                <button class="btn-primary" type="button" onclick={generateInvite} disabled={inviteBusy}>
                  {inviteBusy ? 'Generating…' : 'Generate invite code'}
                </button>
                {#if invite}
                  <span class="font-mono text-2xl tracking-[0.3em] text-emerald-300 break-all">{invite.code}</span>
                  <button
                    class="btn-secondary"
                    class:btn-success={inviteCopied}
                    type="button"
                    onclick={copyInvite}
                  >
                    {inviteCopied ? '✓ Copied!' : 'Copy'}
                  </button>
                {/if}
              </div>
              {#if invite}
                <p class="mt-2 text-xs text-gray-500">Works once · expires in 7 days</p>
              {/if}
            </div>

            <div class="border-t border-gray-700 pt-4">
              <h3 class="text-sm font-semibold text-gray-200 mb-2">Members</h3>
              {#if roster.length === 0}
                <p class="text-xs text-gray-500">No member roster available.</p>
              {:else}
                <div class="space-y-2">
                  {#each roster as m (m.userId)}
                    <div class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-gray-700 bg-gray-900/40 p-3">
                      <div class="flex items-center gap-2">
                        <span class="text-sm text-white break-words">{m.name}</span>
                        <span class={badgeClass(m.role)}>{roleLabel(m.role)}</span>
                      </div>
                      {#if m.role !== 'owner'}
                        <button
                          class="bg-red-800 hover:bg-red-700 text-white px-3 py-1.5 rounded-lg text-xs"
                          type="button"
                          onclick={() => removeMember(m)}
                        >
                          Remove
                        </button>
                      {/if}
                    </div>
                  {/each}
                </div>
              {/if}
            </div>

            <div class="border-t border-gray-700 pt-4 flex flex-wrap gap-2">
              <button class="btn-secondary" type="button" onclick={leaveGroup}>Leave group</button>
              <button class="bg-red-800 hover:bg-red-700 text-white px-3 py-2 rounded-lg" type="button" onclick={deleteGroup}>
                Delete group
              </button>
            </div>
            <p class="text-xs text-gray-500">
              Leaving transfers ownership automatically if you're the owner. Deleting removes the group for everyone.
            </p>
          </div>
        </Card>
      {:else}
        <Card>
          <button class="btn-secondary" type="button" onclick={leaveGroup}>Leave group</button>
        </Card>
      {/if}
    {/if}
  </div>
{/if}
