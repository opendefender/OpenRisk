// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
//
// Framework JSON export and import service functions (#307).

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

vi.mock('../../lib/api', () => ({ api: { get: vi.fn(), post: vi.fn() } }));

import { api } from '../../lib/api';
import { complianceService } from '../complianceService';

const get = vi.mocked(api.get);
const post = vi.mocked(api.post);

describe('complianceService.exportFramework', () => {
  let click: ReturnType<typeof vi.spyOn>;
  const clicked = (): HTMLAnchorElement | undefined =>
    click.mock.contexts[0] as HTMLAnchorElement | undefined;

  beforeEach(() => {
    URL.createObjectURL = vi.fn(() => 'blob:x');
    URL.revokeObjectURL = vi.fn();
    click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
  });
  afterEach(() => vi.restoreAllMocks());

  it('requests the export as a blob and saves it under the server filename', async () => {
    get.mockResolvedValue({
      data: new Blob(['{}']),
      headers: { 'content-disposition': 'attachment; filename="circulaire-1.0.json"' },
    });
    await complianceService.exportFramework('fw-1');
    expect(get).toHaveBeenCalledWith('/compliance/frameworks/fw-1/export', {
      responseType: 'blob',
    });
    expect(clicked()?.download).toBe('circulaire-1.0.json');
    expect(URL.revokeObjectURL).toHaveBeenCalled();
  });

  it('falls back to a default filename without Content-Disposition', async () => {
    get.mockResolvedValue({ data: new Blob(['{}']), headers: {} });
    await complianceService.exportFramework('fw-1');
    expect(clicked()?.download).toBe('framework.json');
  });

  it('propagates a 404 and downloads nothing', async () => {
    get.mockRejectedValue(new Error('404'));
    await expect(complianceService.exportFramework('nope')).rejects.toThrow('404');
    expect(clicked()).toBeUndefined();
  });
});

describe('complianceService.importFramework', () => {
  it('posts the document as the raw JSON body', async () => {
    const doc = {
      schema_version: 1 as const,
      framework: { name: 'F', version: '1', description: '' },
      controls: [{ reference_code: 'A', name: 'a' }],
    };
    post.mockResolvedValue({ data: { id: 'x', controls_created: 1 } });
    const res = await complianceService.importFramework(doc);
    expect(post).toHaveBeenCalledWith('/compliance/frameworks/import', doc);
    expect(res.controls_created).toBe(1);
  });
});
