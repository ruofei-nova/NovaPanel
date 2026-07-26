import { useEffect, useRef, useState } from 'react';

/**
 * Keeps a short in-memory history of a value that is already refreshed by the
 * query layer. The initial flat line is the first real sample, not mock data.
 */
export function useRollingSeries(value: number, maxPoints = 40): number[] {
  const initialValue = Number.isFinite(value) ? value : 0;
  const [series, setSeries] = useState<number[]>(() => Array(maxPoints).fill(initialValue));
  const lastValueRef = useRef(initialValue);

  useEffect(() => {
    const nextValue = Number.isFinite(value) ? value : 0;
    if (nextValue === lastValueRef.current && series.length === maxPoints) return;
    lastValueRef.current = nextValue;
    setSeries((current) => [...current.slice(-(maxPoints - 1)), nextValue]);
  }, [maxPoints, series.length, value]);

  return series;
}
