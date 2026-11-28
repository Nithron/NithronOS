/**
 * Motion: a loop only for something live (a lamp for a running task, a
 * cursor while loading), in steps so the mark is always whole. Under the
 * system's reduced motion everything stands still and nothing is missing.
 */
import {useEffect, useState} from 'react';
import {AccessibilityInfo} from 'react-native';

/** Whether the system asks for reduced motion. */
export function useReducedMotion(): boolean {
  const [reduced, setReduced] = useState(false);

  useEffect(() => {
    let live = true;
    AccessibilityInfo.isReduceMotionEnabled()
      .then(value => {
        if (live) {
          setReduced(value);
        }
      })
      .catch(() => undefined);
    const subscription = AccessibilityInfo.addEventListener(
      'reduceMotionChanged',
      setReduced,
    );
    return () => {
      live = false;
      subscription.remove();
    };
  }, []);

  return reduced;
}

/**
 * Whether a blinking thing is in its lit half: it changes every half
 * `period` ms, in one step. Always lit under reduced motion, or when
 * `enabled` is false.
 */
export function useBlink(period: number, enabled = true): boolean {
  const reduced = useReducedMotion();
  const [lit, setLit] = useState(true);

  useEffect(() => {
    if (reduced || !enabled) {
      setLit(true);
      return;
    }
    const timer = setInterval(() => setLit(value => !value), period / 2);
    return () => clearInterval(timer);
  }, [period, reduced, enabled]);

  return lit;
}
