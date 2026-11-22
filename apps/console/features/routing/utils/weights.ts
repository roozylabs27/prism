export interface PolicyUiWeights {
  task_match: number;
  quality: number;
  cost: number;
  speed: number;
}

export const DEFAULT_POLICY_UI_WEIGHTS: PolicyUiWeights = {
  task_match: 35,
  quality: 35,
  cost: 15,
  speed: 15,
};

function finiteNonNegative(val: number | undefined): number {
  if (typeof val !== 'number' || !Number.isFinite(val) || val <= 0) {
    return 0;
  }
  return val;
}

/**
 * Converts backend routing weights (normalized 0..1 decimals or legacy 0..100
 * percentages) into integer 0..100 percentages for UI sliders and display.
 */
export function toUiWeightPercents(weights?: Record<string, number | undefined>): PolicyUiWeights {
  if (!weights) {
    return { ...DEFAULT_POLICY_UI_WEIGHTS };
  }

  const taskMatch = finiteNonNegative(weights.task_match);
  const quality = finiteNonNegative(weights.quality);
  const cost = finiteNonNegative(weights.cost);
  const speed = finiteNonNegative(weights.speed);
  const sum = taskMatch + quality + cost + speed;

  if (sum <= 0) {
    return { ...DEFAULT_POLICY_UI_WEIGHTS };
  }

  const isDecimalScale =
    sum <= 1.05 && Math.max(taskMatch, quality, cost, speed) <= 1.0;

  if (isDecimalScale) {
    return {
      task_match: Math.round((taskMatch / sum) * 100),
      quality: Math.round((quality / sum) * 100),
      cost: Math.round((cost / sum) * 100),
      speed: Math.round((speed / sum) * 100),
    };
  }

  return {
    task_match: Math.round(taskMatch),
    quality: Math.round(quality),
    cost: Math.round(cost),
    speed: Math.round(speed),
  };
}

/**
 * Normalizes UI slider values (0..100) into 0..1 decimal weights summing to 1.0
 * for storage and smart-router scoring.
 */
export function toApiNormalizedWeights(weights: PolicyUiWeights): PolicyUiWeights {
  const taskMatch = finiteNonNegative(weights.task_match);
  const quality = finiteNonNegative(weights.quality);
  const cost = finiteNonNegative(weights.cost);
  const speed = finiteNonNegative(weights.speed);
  const sum = taskMatch + quality + cost + speed;

  if (sum <= 0) {
    return {
      task_match: 0.35,
      quality: 0.35,
      cost: 0.15,
      speed: 0.15,
    };
  }

  const round4 = (n: number) => Math.round((n / sum) * 10000) / 10000;
  return {
    task_match: round4(taskMatch),
    quality: round4(quality),
    cost: round4(cost),
    speed: round4(speed),
  };
}
