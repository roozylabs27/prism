import { describe, expect, it } from 'vitest';
import { toApiNormalizedWeights, toUiWeightPercents } from './weights';

describe('routing weight helpers', () => {
  it('converts backend decimal weights into 0..100 UI percentages', () => {
    expect(
      toUiWeightPercents({ task_match: 0.35, quality: 0.35, cost: 0.15, speed: 0.15 })
    ).toEqual({ task_match: 35, quality: 35, cost: 15, speed: 15 });
  });

  it('keeps legacy 0..100 weights readable and falls back on empty', () => {
    expect(toUiWeightPercents({ quality: 40, cost: 30, speed: 20 })).toEqual({
      task_match: 0,
      quality: 40,
      cost: 30,
      speed: 20,
    });
    expect(toUiWeightPercents(undefined)).toEqual({
      task_match: 35,
      quality: 35,
      cost: 15,
      speed: 15,
    });
  });

  it('normalizes UI percentages into 0..1 weights summing to 1.0', () => {
    const out = toApiNormalizedWeights({ task_match: 35, quality: 35, cost: 15, speed: 15 });
    expect(out).toEqual({ task_match: 0.35, quality: 0.35, cost: 0.15, speed: 0.15 });
    expect(out.task_match + out.quality + out.cost + out.speed).toBeCloseTo(1, 4);
  });
});
