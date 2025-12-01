import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useRiskStore } from '../useRiskStore'
import { api } from '../../lib/api'

describe('useRiskStore', () => {
  beforeEach(() => {
    // reset store state
    const s = useRiskStore.getState()
    s.risks = []
    s.total = 0
    s.page = 1
    s.pageSize = 10
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('fetchRisks sets risks and total for paginated response', async () => {
    const fakeData = { items: [{ id: 1, title: 'Risk A' }], total: 1 }
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: fakeData })

    await useRiskStore.getState().fetchRisks({ page: 1, limit: 10 })

    const state = useRiskStore.getState()
    expect(state.risks).toEqual(fakeData.items)
    expect(state.total).toBe(1)
    expect(api.get).toHaveBeenCalledWith('/risks', { params: { page: 1, limit: 10 } })
  })

  it('fetchRisks handles legacy array response', async () => {
    const legacy = [{ id: 2, title: 'Risk B' }]
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: legacy })

    await useRiskStore.getState().fetchRisks()

    const state = useRiskStore.getState()
    expect(state.risks).toEqual(legacy)
    expect(state.total).toBe(1)
  })

  it('createRisk posts payload and refreshes list', async () => {
    const newRisk = { title: 'New' }
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { id: 3, ...newRisk } })
    // after create, fetchRisks will be called; mock its response
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [{ id: 3, title: 'New' }], total: 1 } })

    await useRiskStore.getState().createRisk(newRisk)

    const state = useRiskStore.getState()
    expect(api.post).toHaveBeenCalledWith('/risks', newRisk)
    expect(state.risks).toEqual([{ id: 3, title: 'New' }])
    expect(state.total).toBe(1)
  })

  it('updateRisk patches payload and refreshes list', async () => {
    const patch = { title: 'Updated' }
    const id = 4
    vi.spyOn(api, 'patch').mockResolvedValueOnce({ data: { id, ...patch } })
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [{ id, title: 'Updated' }], total: 1 } })

    await useRiskStore.getState().updateRisk(id, patch)

    const state = useRiskStore.getState()
    expect(api.patch).toHaveBeenCalledWith(`/risks/${id}`, patch)
    expect(state.risks).toEqual([{ id, title: 'Updated' }])
  })

  it('deleteRisk calls delete and refreshes list', async () => {
    const id = 5
    vi.spyOn(api, 'delete').mockResolvedValueOnce({ data: {} })
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [], total: 0 } })

    await useRiskStore.getState().deleteRisk(id)

    const state = useRiskStore.getState()
    expect(api.delete).toHaveBeenCalledWith(`/risks/${id}`)
    expect(state.risks).toEqual([])
    expect(state.total).toBe(0)
  })
})
