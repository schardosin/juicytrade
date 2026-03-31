import { describe, it, expect, vi, beforeEach } from 'vitest';

const mockGet = vi.fn();

vi.mock('../src/services/api.js', async () => {
  const { api } = await import('../src/services/api.js?bypass');
  return { default: api, api };
});

vi.mock('axios', () => {
  const mockAxiosInstance = {
    get: mockGet,
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
    interceptors: {
      request: { use: vi.fn() },
      response: { use: vi.fn() },
    },
  };
  return {
    default: {
      create: vi.fn(() => mockAxiosInstance),
      get: vi.fn(),
      post: vi.fn(),
      put: vi.fn(),
      delete: vi.fn(),
    },
  };
});

function makeResponse(symbol, priceData) {
  return { data: { data: { [symbol]: priceData } } };
}

describe('getUnderlyingPrice', () => {
  let api;

  beforeEach(async () => {
    vi.clearAllMocks();
    const mod = await import('../src/services/api.js');
    api = mod.default || mod.api;
  });

  it('returns midpoint when both bid and ask are present', async () => {
    mockGet.mockResolvedValue(makeResponse('SPY', { bid: 100.0, ask: 102.0, last: 101.5 }));
    const result = await api.getUnderlyingPrice('SPY');
    expect(result).toBeCloseTo(101.0, 6);
  });

  it('returns ask when only ask is present', async () => {
    mockGet.mockResolvedValue(makeResponse('SPY', { bid: null, ask: 102.0, last: null }));
    const result = await api.getUnderlyingPrice('SPY');
    expect(result).toBe(102.0);
  });

  it('returns bid when only bid is present', async () => {
    mockGet.mockResolvedValue(makeResponse('SPY', { bid: 100.0, ask: null, last: null }));
    const result = await api.getUnderlyingPrice('SPY');
    expect(result).toBe(100.0);
  });

  it('returns last when only last is present and bid/ask are null (NDX case)', async () => {
    mockGet.mockResolvedValue(makeResponse('NDX', { bid: null, ask: null, last: 18542.50 }));
    const result = await api.getUnderlyingPrice('NDX');
    expect(result).toBe(18542.50);
  });

  it('returns last when only last is present and bid/ask are missing from object', async () => {
    mockGet.mockResolvedValue(makeResponse('NDX', { last: 18542.50 }));
    const result = await api.getUnderlyingPrice('NDX');
    expect(result).toBe(18542.50);
  });

  it('returns null when no price fields are available', async () => {
    mockGet.mockResolvedValue(makeResponse('NDX', { bid: null, ask: null, last: null }));
    const result = await api.getUnderlyingPrice('NDX');
    expect(result).toBeNull();
  });

  it('returns priceData directly when it is a plain number', async () => {
    mockGet.mockResolvedValue(makeResponse('SPY', 500.25));
    const result = await api.getUnderlyingPrice('SPY');
    expect(result).toBe(500.25);
  });
});
