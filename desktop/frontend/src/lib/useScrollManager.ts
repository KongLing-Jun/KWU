import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import gsap from "gsap";
import { DUR_FAST, EASE_OUT, prefersReducedMotion } from "./gsapAnimations";
import { recordTranscriptTrace } from "./transcriptPerformanceTrace";

const BOTTOM_THRESHOLD_PX = 80;

type GsapCompat = typeof gsap & {
  default?: typeof gsap;
  gsap?: typeof gsap;
};

function killScrollTweens(target: HTMLElement) {
  const api = gsap as GsapCompat;
  const killTweensOf = api.killTweensOf ?? api.gsap?.killTweensOf ?? api.default?.killTweensOf;
  if (typeof killTweensOf === "function") killTweensOf(target);
}

function isNearBottom(el: HTMLElement): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight < BOTTOM_THRESHOLD_PX;
}

/**
 * useScrollManager — GSAP-driven auto-scroll for the transcript container.
 *
 * - Auto-pins to the bottom when content is near the edge.
 * - Smooth scroll for jump-to-question navigation.
 * - Uses gsap.scrollTo for layout-safe scrolling (avoids layout thrashing).
 * - Batches ResizeObserver callbacks into a single GSAP tween.
 */
export function useScrollManager(traceView = "", viewKey = traceView) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const stick = useRef(true);
  const gsapCtx = useRef<gsap.Context | null>(null);
  const prevQuestionsLen = useRef(0);
  const resizeFrame = useRef<number | null>(null);
  const layoutScrollFrames = useRef<number[]>([]);
  const lastClientHeight = useRef<number | null>(null);
  const lastFooterHeight = useRef<number | null>(null);
  const [isAtBottom, setIsAtBottom] = useState(true);
  const traceViewRef = useRef(traceView);
  traceViewRef.current = traceView;

  // Transcript reuses its scroll element across tasks. Retire the previous
  // task's forced scrolling before the next task restores its own position.
  useLayoutEffect(() => {
    const element = scrollRef.current;
    return () => {
      if (resizeFrame.current !== null) cancelAnimationFrame(resizeFrame.current);
      resizeFrame.current = null;
      for (const frame of layoutScrollFrames.current) cancelAnimationFrame(frame);
      layoutScrollFrames.current = [];
      if (element) killScrollTweens(element);
    };
  }, [viewKey]);

  // Kill any lingering tweens on unmount.
  useEffect(() => {
    recordTranscriptTrace("scroll-manager.mount", { view: traceViewRef.current }, { persist: true });
    return () => {
      recordTranscriptTrace("scroll-manager.unmount", {
        view: traceViewRef.current,
        resizePending: resizeFrame.current !== null,
        layoutPending: layoutScrollFrames.current.length,
      }, { persist: true });
      gsapCtx.current?.revert();
      if (resizeFrame.current !== null) cancelAnimationFrame(resizeFrame.current);
      for (const frame of layoutScrollFrames.current) cancelAnimationFrame(frame);
      layoutScrollFrames.current = [];
    };
  }, []);

  const updateBottomState = useCallback((el: HTMLElement) => {
    const atBottom = isNearBottom(el);
    stick.current = atBottom;
    setIsAtBottom(atBottom);
    return atBottom;
  }, []);

  const onScroll = useCallback(() => {
    const el = scrollRef.current;
    if (el) updateBottomState(el);
  }, [updateBottomState]);

  /** Scroll smoothly to a specific element.  Used by the JumpBar. */
  const smoothScrollTo = useCallback((element: HTMLElement, offset = 12) => {
    const el = scrollRef.current;
    if (!el) return;
    stick.current = false;
    setIsAtBottom(false);
    if (resizeFrame.current !== null) {
      cancelAnimationFrame(resizeFrame.current);
      recordTranscriptTrace("raf.cancel", { view: traceViewRef.current, owner: "scroll-bottom", reason: "smooth-scroll" });
      resizeFrame.current = null;
    }
    const rect = element.getBoundingClientRect();
    const containerRect = el.getBoundingClientRect();
    const top = el.scrollTop + rect.top - containerRect.top - offset;
    recordTranscriptTrace("scroll.smooth", {
      view: traceViewRef.current,
      scrollTop: el.scrollTop,
      targetTop: Math.max(0, top),
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    });
    const reduced = prefersReducedMotion();
    gsap.to(el, {
      scrollTo: { y: Math.max(0, top) },
      duration: reduced ? 0.001 : DUR_FAST * 2,
      ease: EASE_OUT,
      onComplete: () => updateBottomState(el),
    });
  }, [updateBottomState]);

  /** Force-scroll to the bottom — used when a new question is sent. */
  const scrollToBottom = useCallback((force = false) => {
    const el = scrollRef.current;
    if (!el) return;
    if (force) {
      stick.current = true;
      setIsAtBottom(true);
    }
    if (!stick.current && !force) return;
    if (resizeFrame.current !== null) {
      cancelAnimationFrame(resizeFrame.current);
      recordTranscriptTrace("raf.cancel", { view: traceViewRef.current, owner: "scroll-bottom", reason: "reschedule" });
      resizeFrame.current = null;
    }
    recordTranscriptTrace("raf.schedule", { view: traceViewRef.current, owner: "scroll-bottom" });
    resizeFrame.current = requestAnimationFrame(() => {
      resizeFrame.current = null;
      recordTranscriptTrace("raf.fire", { view: traceViewRef.current, owner: "scroll-bottom" });
      if (!stick.current && !force) return;
      if (force) {
        stick.current = true;
        setIsAtBottom(true);
      }
      const reduced = prefersReducedMotion();
      gsap.to(el, {
        scrollTo: { y: "max" },
        duration: reduced ? 0.001 : DUR_FAST,
        ease: "none",
        overwrite: "auto",
        onComplete: () => {
          stick.current = true;
          setIsAtBottom(true);
        },
      });
    });
  }, []);

  const snapToBottom = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    if (resizeFrame.current !== null) {
      cancelAnimationFrame(resizeFrame.current);
      recordTranscriptTrace("raf.cancel", { view: traceViewRef.current, owner: "scroll-bottom", reason: "force-snap" });
      resizeFrame.current = null;
    }
    killScrollTweens(el);
    stick.current = true;
    el.scrollTop = el.scrollHeight;
    recordTranscriptTrace("scroll.snap", {
      view: traceViewRef.current,
      owner: "force-bottom",
      scrollTop: el.scrollTop,
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    });
    setIsAtBottom(true);
  }, []);

  const snapPinnedToBottom = useCallback(() => {
    const el = scrollRef.current;
    if (!el || !stick.current) return false;
    if (resizeFrame.current !== null) {
      cancelAnimationFrame(resizeFrame.current);
      recordTranscriptTrace("raf.cancel", { view: traceViewRef.current, owner: "scroll-bottom", reason: "pinned-snap" });
      resizeFrame.current = null;
    }
    killScrollTweens(el);
    el.scrollTop = el.scrollHeight;
    recordTranscriptTrace("scroll.snap", {
      view: traceViewRef.current,
      owner: "pinned-bottom",
      scrollTop: el.scrollTop,
      scrollHeight: el.scrollHeight,
      clientHeight: el.clientHeight,
    });
    setIsAtBottom(true);
    return true;
  }, []);

  const pinToBottomAfterLayout = useCallback((frames = 3) => {
    const cancelled = layoutScrollFrames.current.length;
    for (const frame of layoutScrollFrames.current) cancelAnimationFrame(frame);
    layoutScrollFrames.current = [];
    if (cancelled > 0) recordTranscriptTrace("raf.cancel", { view: traceViewRef.current, owner: "pin-layout", count: cancelled });
    if (!snapPinnedToBottom()) return;
    let remaining = Math.max(0, frames);
    const tick = () => {
      if (remaining <= 0) return;
      recordTranscriptTrace("raf.schedule", { view: traceViewRef.current, owner: "pin-layout", framesLeft: remaining });
      const frame = requestAnimationFrame(() => {
        layoutScrollFrames.current = layoutScrollFrames.current.filter((id) => id !== frame);
        recordTranscriptTrace("raf.fire", { view: traceViewRef.current, owner: "pin-layout", framesLeft: remaining });
        if (!snapPinnedToBottom()) return;
        remaining -= 1;
        tick();
      });
      layoutScrollFrames.current.push(frame);
    };
    tick();
  }, [snapPinnedToBottom]);

  const scrollToBottomAfterLayout = useCallback((frames = 4) => {
    const cancelled = layoutScrollFrames.current.length;
    for (const frame of layoutScrollFrames.current) cancelAnimationFrame(frame);
    layoutScrollFrames.current = [];
    if (cancelled > 0) recordTranscriptTrace("raf.cancel", { view: traceViewRef.current, owner: "force-layout", count: cancelled });
    snapToBottom();
    let remaining = Math.max(0, frames);
    const tick = () => {
      if (remaining <= 0) return;
      recordTranscriptTrace("raf.schedule", { view: traceViewRef.current, owner: "force-layout", framesLeft: remaining });
      const frame = requestAnimationFrame(() => {
        layoutScrollFrames.current = layoutScrollFrames.current.filter((id) => id !== frame);
        recordTranscriptTrace("raf.fire", { view: traceViewRef.current, owner: "force-layout", framesLeft: remaining });
        snapToBottom();
        remaining -= 1;
        tick();
      });
      layoutScrollFrames.current.push(frame);
    };
    tick();
  }, [snapToBottom]);

  /** Call when a new question is submitted — overrides stick state. */
  const onNewQuestion = useCallback(() => {
    stick.current = true;
    scrollToBottom(true);
  }, [scrollToBottom]);

  /**
   * Refresh pin state on resize — call from a ResizeObserver on the container.
   */
  const repinIfWasPinned = useCallback(
    (containerHeightDelta: number) => {
      const el = scrollRef.current;
      if (!el) return;
      const bottomDistance = el.scrollHeight - el.scrollTop - el.clientHeight;
      if (!stick.current && bottomDistance + containerHeightDelta >= BOTTOM_THRESHOLD_PX) return;
      stick.current = true;
      setIsAtBottom(true);
      scrollToBottom();
    },
    [scrollToBottom],
  );

  /**
   * Track question count changes to call onNewQuestion.
   * Returns the previous length ref for useEffect comparison.
   */
  const trackQuestions = useCallback(
    (questionsLen: number, autoScroll = true) => {
      if (autoScroll && questionsLen > prevQuestionsLen.current) {
        onNewQuestion();
      }
      prevQuestionsLen.current = questionsLen;
    },
    [onNewQuestion],
  );

  return {
    scrollRef,
    stick,
    onScroll,
    isAtBottom,
    smoothScrollTo,
    scrollToBottom,
    scrollToBottomAfterLayout,
    pinToBottomAfterLayout,
    onNewQuestion,
    repinIfWasPinned,
    trackQuestions,
    resizeFrame,
    lastClientHeight,
    lastFooterHeight,
  };
}
