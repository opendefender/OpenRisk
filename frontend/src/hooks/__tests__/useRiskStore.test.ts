import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { useRiskStore } from '../useRiskStore'
import { api } from '../../lib/api'

describe('useRiskStore', () => {
  beforeEach(() => {
    // reset store state
    const s = useRiskStore.getState()
    s.risks = []
    s.total = 
    s.page = 
    s.pageSize = 
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('fetchRisks sets risks and total for paginated response', async () => {
    const fakeData = { items: [{ id: , title: 'Risk A' }], total:  }
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: fakeData })

    await useRiskStore.getState().fetchRisks({ page: , limit:  })

    const state = useRiskStore.getState()
    expect(state.risks).toEqual(fakeData.items)
    expect(state.total).toBe()
    expect(api.get).toHaveBeenCalledWith('/risks', { params: { page: , limit:  } })
  })

  it('fetchRisks handles legacy array response', async () => {
    const legacy = [{ id: , title: 'Risk B' }]
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: legacy })

    await useRiskStore.getState().fetchRisks()

    const state = useRiskStore.getState()
    expect(state.risks).toEqual(legacy)
    expect(state.total).toBe()
  })

  it('createRisk posts payload and refreshes list', async () => {
    const newRisk = { title: 'New' }
    vi.spyOn(api, 'post').mockResolvedValueOnce({ data: { id: , ...newRisk } })
    // after create, fetchRisks will be called; mock its response
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [{ id: , title: 'New' }], total:  } })

    await useRiskStore.getState().createRisk(newRisk)

    const state = useRiskStore.getState()
    expect(api.post).toHaveBeenCalledWith('/risks', newRisk)
    expect(state.risks).toEqual([{ id: , title: 'New' }])
    expect(state.total).toBe()
  })

  it('updateRisk patches payload and refreshes list', async () => {
    const patch = { title: 'Updated' }
    const id = ''
    vi.spyOn(api, 'patch').mockResolvedValueOnce({ data: { id, ...patch } })
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [{ id, title: 'Updated' }], total:  } })

    await useRiskStore.getState().updateRisk(id, patch)

    const state = useRiskStore.getState()
    expect(api.patch).toHaveBeenCalledWith(/risks/${id}, patch)
    expect(state.risks).toEqual([{ id, title: 'Updated' }])
  })

  it('deleteRisk calls delete and refreshes list', async () => {
    const id = ''
    vi.spyOn(api, 'delete').mockResolvedValueOnce({ data: {} })
    vi.spyOn(api, 'get').mockResolvedValueOnce({ data: { items: [], total:  } })

    await useRiskStore.getState().deleteRisk(id)

    const state = useRiskStore.getState()
    expect(api.delete).toHaveBeenCalledWith(/risks/${id})
    expect(state.risks).toEqual([])
    expect(state.total).toBe()
  })
})
